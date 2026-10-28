// Author: Sunil Jain, CyberinQ LLC
// Package extract performs source-preserving, bounded document text segmentation.
// It does not assert that extracted text is an approved order, BOM, or factory fact.
package extract

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

const MaxBytes = 20 << 20
const MaxSegments = 5000
const maxDecodedXML = 24 << 20

type Segment struct {
	Ordinal int             `json:"ordinal"`
	Kind    string          `json:"kind"`
	Locator json.RawMessage `json:"locator"`
	Content string          `json:"content"`
}

type Result struct {
	Format      string    `json:"format"`
	Status      string    `json:"status"`
	Explanation string    `json:"explanation"`
	Segments    []Segment `json:"segments"`
}

func Detect(filename string, raw []byte) (format, media string) {
	if len(raw) >= 5 && string(raw[:5]) == "%PDF-" {
		return "PDF", "application/pdf"
	}
	if len(raw) >= 8 && bytes.Equal(raw[:8], []byte{0xd0, 0xcf, 0x11, 0xe0, 0xa1, 0xb1, 0x1a, 0xe1}) {
		return "XLS_LEGACY", "application/vnd.ms-excel"
	}
	if len(raw) >= 4 && bytes.Equal(raw[:4], []byte("PK\x03\x04")) {
		zr, e := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
		if e != nil {
			return "UNKNOWN", "application/octet-stream"
		}
		has := func(path string) bool {
			for _, f := range zr.File {
				if f.Name == path {
					return true
				}
			}
			return false
		}
		if has("xl/workbook.xml") {
			return "XLSX", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
		}
		if has("word/document.xml") {
			return "DOCX", "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
		}
		return "UNKNOWN", "application/octet-stream"
	}
	mt := http.DetectContentType(raw)
	if strings.HasPrefix(mt, "image/") {
		return "IMAGE", strings.Split(mt, ";")[0]
	}
	if !utf8.Valid(raw) || bytes.IndexByte(raw, 0) >= 0 {
		return "UNKNOWN", "application/octet-stream"
	}
	lower := strings.ToLower(filepath.Ext(filename))
	if lower == ".csv" {
		return "CSV", "text/csv"
	}
	if lower == ".tsv" {
		return "TSV", "text/tab-separated-values"
	}
	if lower == ".txt" || lower == ".md" || mt == "text/plain; charset=utf-8" {
		return "TEXT", "text/plain"
	}
	if m, _, e := mime.ParseMediaType(mt); e == nil && strings.HasPrefix(m, "text/") {
		return "TEXT", "text/plain"
	}
	return "UNKNOWN", "application/octet-stream"
}

func (r *Result) add(kind string, locator any, content string) {
	if len(r.Segments) >= MaxSegments {
		return
	}
	content = strings.TrimSpace(content)
	if content == "" {
		return
	}
	loc, _ := json.Marshal(locator)
	// Split long segments without truncating entire pages/sheets silently.
	for len(content) > 0 && len(r.Segments) < MaxSegments {
		next := len(content)
		if next > 6000 {
			next = 6000
			for next > 0 && !utf8.RuneStart(content[next]) {
				next--
			}
		}
		r.Segments = append(r.Segments, Segment{len(r.Segments) + 1, kind, loc, content[:next]})
		content = content[next:]
	}
}

func Process(ctx context.Context, filename string, raw []byte) Result {
	f, _ := Detect(filename, raw)
	out := Result{Format: f, Status: "EXTRACTED", Segments: []Segment{}}
	if len(raw) == 0 || len(raw) > MaxBytes {
		out.Status = "ERROR"
		out.Explanation = "Invalid file size"
		return out
	}
	switch f {
	case "CSV", "TSV":
		comma := ','
		if f == "TSV" {
			comma = '\t'
		}
		rd := csv.NewReader(bytes.NewReader(bytes.TrimPrefix(raw, []byte{0xef, 0xbb, 0xbf})))
		rd.Comma = comma
		rd.FieldsPerRecord = -1
		rd.LazyQuotes = true
		for row := 1; row <= MaxSegments; row++ {
			cols, e := rd.Read()
			if e == io.EOF {
				break
			}
			if e != nil {
				out.Status = "PARTIAL"
				out.Explanation = "CSV parse error at row " + strconv.Itoa(row)
				break
			}
			b, _ := json.Marshal(cols)
			out.add("ROW", map[string]any{"row": row}, string(b))
		}
	case "TEXT":
		for n, line := range strings.Split(string(raw), "\n") {
			if n >= MaxSegments {
				break
			}
			out.add("LINE", map[string]any{"line": n + 1}, line)
		}
	case "XLSX", "DOCX":
		if e := extractOffice(&out, raw); e != nil {
			out.Status = "PARTIAL"
			out.Explanation = "Office extraction incomplete: " + e.Error()
		}
	case "PDF":
		c, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		dir, e := os.MkdirTemp("", "agiparel-pdf-")
		if e != nil {
			out.Status = "ERROR"
			out.Explanation = "Temporary PDF directory unavailable"
			break
		}
		defer os.RemoveAll(dir)
		path := filepath.Join(dir, "source.pdf")
		if e = os.WriteFile(path, raw, 0600); e != nil {
			out.Status = "ERROR"
			out.Explanation = "Temporary PDF write failed"
			break
		}
		// The pdf process reads the original locally; never passes user path or options.
		data, e := exec.CommandContext(c, "pdftotext", "-layout", "-enc", "UTF-8", path, "-").Output()
		if e != nil {
			out.Status = "PARTIAL"
			out.Explanation = "PDF text extraction unavailable or failed; original retained for OCR/review"
			break
		}
		if len(data) > 12<<20 {
			out.Status = "PARTIAL"
			out.Explanation = "PDF text exceeded safe extraction budget"
			data = data[:12<<20]
		}
		for p, page := range strings.Split(string(data), "\f") {
			if p >= 500 {
				out.Status = "PARTIAL"
				out.Explanation = "PDF exceeded 500 extracted pages"
				break
			}
			out.add("PAGE", map[string]any{"page": p + 1}, page)
		}
		if len(out.Segments) == 0 {
			out.Status = "PARTIAL"
			out.Explanation = "No embedded PDF text; scanned pages require OCR in Gate 06.5-B"
		}
	case "IMAGE":
		out.Status = "PARTIAL"
		out.Explanation = "Image stored; image OCR is not enabled yet (Gate 06.5-B)"
	case "XLS_LEGACY":
		out.Status = "UNSUPPORTED"
		out.Explanation = "Legacy binary .xls stored; conversion worker is planned for Gate 06.5-B"
	default:
		out.Status = "UNSUPPORTED"
		out.Explanation = "Original stored; no safe extractor configured for this format"
	}
	if len(out.Segments) == MaxSegments {
		out.Status = "PARTIAL"
		out.Explanation = "Extraction stopped at 5,000 segments; source preserved"
	}
	if out.Status == "EXTRACTED" {
		out.Explanation = "Text segments captured for review; semantic entities and approval pending"
	}
	return out
}

func readZipEntry(z *zip.Reader, name string) ([]byte, error) {
	for _, f := range z.File {
		if f.Name != name {
			continue
		}
		if f.UncompressedSize64 > maxDecodedXML {
			return nil, errors.New("office XML entry exceeds limit")
		}
		r, e := f.Open()
		if e != nil {
			return nil, e
		}
		defer r.Close()
		data, e := io.ReadAll(io.LimitReader(r, maxDecodedXML+1))
		if len(data) > maxDecodedXML {
			return nil, errors.New("office XML expansion exceeds limit")
		}
		return data, e
	}
	return nil, fmt.Errorf("missing office entry: %s", name)
}

type sharedStrings struct {
	SI []struct {
		T    string `xml:"t"`
		Runs []struct {
			T string `xml:"t"`
		} `xml:"r"`
	} `xml:"si"`
}

func extractOffice(out *Result, raw []byte) error {
	z, e := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if e != nil {
		return e
	}
	if out.Format == "DOCX" {
		return parseDocx(out, z)
	}
	return parseXlsx(out, z)
}

func parseDocx(out *Result, z *zip.Reader) error {
	raw, e := readZipEntry(z, "word/document.xml")
	if e != nil {
		return e
	}
	dec := xml.NewDecoder(bytes.NewReader(raw))
	var paragraph strings.Builder
	p := 0
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if t.Name.Local == "t" {
				var text string
				if err = dec.DecodeElement(&text, &t); err != nil {
					return err
				}
				paragraph.WriteString(text)
			} else if t.Name.Local == "tab" {
				paragraph.WriteString("\t")
			} else if t.Name.Local == "br" {
				paragraph.WriteString("\n")
			}
		case xml.EndElement:
			if t.Name.Local == "p" {
				p++
				out.add("PARAGRAPH", map[string]any{"paragraph": p}, paragraph.String())
				paragraph.Reset()
			}
		}
	}
	if len(out.Segments) == 0 {
		out.Status = "PARTIAL"
		out.Explanation = "No DOCX text extracted"
	}
	return nil
}

func parseXlsx(out *Result, z *zip.Reader) error {
	shared := []string{}
	if raw, e := readZipEntry(z, "xl/sharedStrings.xml"); e == nil {
		var v sharedStrings
		if xml.Unmarshal(raw, &v) != nil {
			return errors.New("invalid sharedStrings.xml")
		}
		for _, s := range v.SI {
			var b strings.Builder
			b.WriteString(s.T)
			for _, r := range s.Runs {
				b.WriteString(r.T)
			}
			shared = append(shared, b.String())
		}
	}
	// Parse workbook relationship map so non-sequential sheet paths are preserved.
	wbRaw, e := readZipEntry(z, "xl/workbook.xml")
	if e != nil {
		return e
	}
	type sheet struct {
		Name  string `xml:"name,attr"`
		RelID string `xml:"id,attr"`
	}
	var wb struct {
		Sheets []sheet `xml:"sheets>sheet"`
	}
	if e = xml.Unmarshal(wbRaw, &wb); e != nil {
		return e
	}
	rels := map[string]string{}
	if relRaw, er := readZipEntry(z, "xl/_rels/workbook.xml.rels"); er == nil {
		var rs struct {
			Items []struct {
				ID     string `xml:"Id,attr"`
				Target string `xml:"Target,attr"`
			} `xml:"Relationship"`
		}
		if xml.Unmarshal(relRaw, &rs) == nil {
			for _, r := range rs.Items {
				target := strings.TrimPrefix(r.Target, "/")
				if strings.HasPrefix(target, "xl/") {
					rels[r.ID] = target
				} else if !strings.Contains(target, "..") {
					rels[r.ID] = "xl/" + target
				}
			}
		}
	}
	for sn, s := range wb.Sheets {
		if sn >= 100 || len(out.Segments) >= MaxSegments {
			out.Status = "PARTIAL"
			out.Explanation = "Sheet or segment extraction limit reached"
			break
		}
		path := rels[s.RelID]
		if path == "" {
			path = fmt.Sprintf("xl/worksheets/sheet%d.xml", sn+1)
		}
		if strings.Contains(path, "..") || !strings.HasPrefix(path, "xl/") {
			return errors.New("invalid sheet path")
		}
		data, er := readZipEntry(z, path)
		if er != nil {
			return er
		}
		dec := xml.NewDecoder(bytes.NewReader(data))
		row := 0
		cellRef := ""
		cellType := ""
		cellValue := ""
		cellFormula := ""
		inCell := false
		cells := []string{}
		flushCell := func() {
			if !inCell {
				return
			}
			v := cellValue
			switch cellType {
			case "s":
				idx, err := strconv.Atoi(strings.TrimSpace(v))
				if err == nil && idx >= 0 && idx < len(shared) {
					v = shared[idx]
				} else {
					v = "[unresolved shared string: " + v + "]"
				}
			case "b":
				if strings.TrimSpace(v) == "1" || strings.EqualFold(strings.TrimSpace(v), "true") {
					v = "TRUE"
				} else if strings.TrimSpace(v) == "0" || strings.EqualFold(strings.TrimSpace(v), "false") {
					v = "FALSE"
				}
			}
			if cellFormula != "" {
				v = "=" + cellFormula + " [cached: " + v + "]"
			}
			if v != "" {
				if cellRef != "" {
					cells = append(cells, cellRef+": "+v)
				} else {
					cells = append(cells, v)
				}
			}
			inCell = false
			cellValue = ""
			cellFormula = ""
		}
		for {
			if len(out.Segments) >= MaxSegments {
				out.Status = "PARTIAL"
				out.Explanation = "Workbook row segment limit reached"
				break
			}
			tok, er := dec.Token()
			if er == io.EOF {
				break
			}
			if er != nil {
				return er
			}
			switch t := tok.(type) {
			case xml.StartElement:
				if t.Name.Local == "row" {
					row++
					for _, a := range t.Attr {
						if a.Name.Local == "r" {
							if v, e := strconv.Atoi(a.Value); e == nil {
								row = v
							}
						}
					}
					cells = []string{}
				}
				if t.Name.Local == "c" {
					inCell = true
					cellRef = ""
					cellType = ""
					cellValue = ""
					cellFormula = ""
					for _, a := range t.Attr {
						if a.Name.Local == "r" {
							cellRef = a.Value
						}
						if a.Name.Local == "t" {
							cellType = a.Value
						}
					}
				}
				if t.Name.Local == "v" || t.Name.Local == "f" || t.Name.Local == "t" {
					if inCell {
						var v string
						if er = dec.DecodeElement(&v, &t); er != nil {
							return er
						}
						if t.Name.Local == "f" {
							cellFormula = v
						} else {
							cellValue += v
						}
					}
				}
			case xml.EndElement:
				if t.Name.Local == "c" {
					flushCell()
				}
				if t.Name.Local == "row" {
					out.add("SHEET_ROW", map[string]any{"sheet": s.Name, "row": row, "sheet_index": sn + 1}, strings.Join(cells, " | "))
				}
			}
		}
	}
	if len(out.Segments) == 0 {
		out.Status = "PARTIAL"
		out.Explanation = "No spreadsheet cell text extracted"
	}
	return nil
}
