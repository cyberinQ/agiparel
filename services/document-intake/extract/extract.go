// Author: Sunil Jain, CyberinQ LLC
// Package extract performs universal, source-preserving, bounded document text segmentation
// across PDF, XLSX, legacy binary XLS (OLE2/BIFF8), ODS, DOCX, ODT, RTF, CSV, TSV, JSON, XML, HTML, TXT, and Images.
// It does not assert that extracted text is an approved order, BOM, or factory fact.
package extract

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/binary"
	"encoding/csv"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"math"
	"mime"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf16"
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
		if has("content.xml") {
			ext := strings.ToLower(filepath.Ext(filename))
			if ext == ".ods" {
				return "ODS", "application/vnd.oasis.opendocument.spreadsheet"
			}
			return "ODT", "application/vnd.oasis.opendocument.text"
		}
		return "UNKNOWN", "application/octet-stream"
	}
	mt := http.DetectContentType(raw)
	if strings.HasPrefix(mt, "image/") {
		return "IMAGE", strings.Split(mt, ";")[0]
	}
	if len(raw) >= 5 && string(raw[:5]) == "{\\rtf" {
		return "RTF", "application/rtf"
	}
	if !utf8.Valid(raw) || bytes.IndexByte(raw, 0) >= 0 {
		return "UNKNOWN", "application/octet-stream"
	}
	lower := strings.ToLower(filepath.Ext(filename))
	switch lower {
	case ".csv":
		return "CSV", "text/csv"
	case ".tsv":
		return "TSV", "text/tab-separated-values"
	case ".json":
		return "JSON", "application/json"
	case ".xml":
		return "XML", "application/xml"
	case ".html", ".htm":
		return "HTML", "text/html"
	case ".rtf":
		return "RTF", "application/rtf"
	case ".txt", ".md":
		return "TEXT", "text/plain"
	}
	if mt == "text/plain; charset=utf-8" {
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
	case "TEXT", "JSON", "XML":
		for n, line := range strings.Split(string(raw), "\n") {
			if n >= MaxSegments {
				break
			}
			out.add("LINE", map[string]any{"line": n + 1}, line)
		}
	case "HTML", "RTF":
		cleaned := stripMarkupOrRTF(f, string(raw))
		for n, line := range strings.Split(cleaned, "\n") {
			if n >= MaxSegments {
				break
			}
			out.add("LINE", map[string]any{"line": n + 1}, line)
		}
	case "XLSX", "DOCX", "ODS", "ODT":
		if e := extractOffice(&out, raw); e != nil {
			out.Status = "PARTIAL"
			out.Explanation = "Office extraction incomplete: " + e.Error()
		}
	case "XLS_LEGACY":
		if len(raw) < 512 {
			out.Status = "UNSUPPORTED"
			out.Explanation = "Legacy binary .xls stored; conversion worker is planned for Gate 06.5-B"
			break
		}
		if e := parseLegacyXLS(&out, raw); e != nil || len(out.Segments) == 0 {
			out.Status = "UNSUPPORTED"
			out.Explanation = "Legacy binary .xls stored; conversion worker is planned for Gate 06.5-B"
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

var htmlTagRe = regexp.MustCompile(`<[^>]+>`)
var rtfCtrlRe = regexp.MustCompile(`\\[a-zA-Z]+-?[0-9]* ?|[{}]`)

func stripMarkupOrRTF(format, s string) string {
	if format == "RTF" {
		s = strings.ReplaceAll(s, "\\par", "\n")
		s = strings.ReplaceAll(s, "\\line", "\n")
		s = strings.ReplaceAll(s, "\\tab", "\t")
		return rtfCtrlRe.ReplaceAllString(s, "")
	}
	s = strings.ReplaceAll(s, "</tr>", "\n")
	s = strings.ReplaceAll(s, "</p>", "\n")
	s = strings.ReplaceAll(s, "<br>", "\n")
	s = strings.ReplaceAll(s, "<br/>", "\n")
	return htmlTagRe.ReplaceAllString(s, " ")
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
	switch out.Format {
	case "DOCX":
		return parseDocx(out, z)
	case "ODS", "ODT":
		return parseOpenDocument(out, z)
	default:
		return parseXlsx(out, z)
	}
}

func parseOpenDocument(out *Result, z *zip.Reader) error {
	raw, e := readZipEntry(z, "content.xml")
	if e != nil {
		return e
	}
	dec := xml.NewDecoder(bytes.NewReader(raw))
	var buf strings.Builder
	idx := 0
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		switch t := tok.(type) {
		case xml.CharData:
			buf.Write(t)
		case xml.EndElement:
			if t.Name.Local == "table-cell" {
				buf.WriteString(" | ")
			} else if t.Name.Local == "table-row" || t.Name.Local == "p" {
				idx++
				line := strings.Trim(strings.TrimSpace(buf.String()), "| ")
				if line != "" {
					kind := "PARAGRAPH"
					if out.Format == "ODS" {
						kind = "SHEET_ROW"
					}
					out.add(kind, map[string]any{"row": idx}, line)
				}
				buf.Reset()
			}
		}
	}
	return nil
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
						if er = dec.DecodeElement(&v, &t); err != nil {
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

func parseLegacyXLS(out *Result, raw []byte) error {
	wb, err := extractOLE2WorkbookStream(raw)
	if err != nil {
		return err
	}
	type xlsSheet struct {
		name   string
		offset int
	}
	var sstChunks [][]byte
	inSST := false
	sheets := []xlsSheet{}
	for pos := 0; pos+4 <= len(wb); {
		recID := binary.LittleEndian.Uint16(wb[pos : pos+2])
		recLen := int(binary.LittleEndian.Uint16(wb[pos+2 : pos+4]))
		pos += 4
		if pos+recLen > len(wb) {
			break
		}
		payload := wb[pos : pos+recLen]
		pos += recLen
		if recID == 0x00FC {
			sstChunks = [][]byte{payload}
			inSST = true
		} else if recID == 0x003C && inSST {
			sstChunks = append(sstChunks, payload)
		} else {
			inSST = false
			if recID == 0x0085 && len(payload) >= 8 {
				bofOff := int(binary.LittleEndian.Uint32(payload[0:4]))
				sLen := int(payload[6])
				sFlags := payload[7]
				name := decodeBIFFString(payload[8:], sLen, (sFlags&1) != 0)
				sheets = append(sheets, xlsSheet{name: name, offset: bofOff})
			} else if recID == 0x000A {
				break
			}
		}
	}
	if len(sheets) == 0 {
		return errors.New("no worksheets in .xls")
	}
	sst := parseBIFF8SST(sstChunks)
	for sIdx, sh := range sheets {
		if sIdx >= 100 || len(out.Segments) >= MaxSegments {
			break
		}
		if sh.offset < 0 || sh.offset >= len(wb) {
			continue
		}
		rowsMap := map[int]map[int]string{}
		lastFormulaRow, lastFormulaCol := -1, -1
		for p := sh.offset; p+4 <= len(wb); {
			rID := binary.LittleEndian.Uint16(wb[p : p+2])
			rLen := int(binary.LittleEndian.Uint16(wb[p+2 : p+4]))
			p += 4
			if p+rLen > len(wb) {
				break
			}
			pay := wb[p : p+rLen]
			p += rLen
			if rID == 0x000A {
				break
			}
			setCell := func(r, c int, val string) {
				val = strings.TrimSpace(val)
				if val == "" || r < 0 || c < 0 || r > 65535 || c > 512 {
					return
				}
				if rowsMap[r] == nil {
					rowsMap[r] = map[int]string{}
				}
				rowsMap[r][c] = val
			}
			switch rID {
			case 0x00FD:
				if len(pay) >= 10 {
					r := int(binary.LittleEndian.Uint16(pay[0:2]))
					c := int(binary.LittleEndian.Uint16(pay[2:4]))
					idx := int(binary.LittleEndian.Uint32(pay[6:10]))
					if idx >= 0 && idx < len(sst) {
						setCell(r, c, sst[idx])
					}
				}
			case 0x0204:
				if len(pay) >= 9 {
					r := int(binary.LittleEndian.Uint16(pay[0:2]))
					c := int(binary.LittleEndian.Uint16(pay[2:4]))
					sLen := int(binary.LittleEndian.Uint16(pay[6:8]))
					setCell(r, c, decodeBIFFString(pay[9:], sLen, (pay[8]&1) != 0))
				}
			case 0x027E:
				if len(pay) >= 10 {
					r := int(binary.LittleEndian.Uint16(pay[0:2]))
					c := int(binary.LittleEndian.Uint16(pay[2:4]))
					rk := binary.LittleEndian.Uint32(pay[6:10])
					setCell(r, c, formatXLSNum(decodeBIFFRK(rk)))
				}
			case 0x00BD:
				if len(pay) >= 6 {
					r := int(binary.LittleEndian.Uint16(pay[0:2]))
					firstC := int(binary.LittleEndian.Uint16(pay[2:4]))
					lastC := int(binary.LittleEndian.Uint16(pay[len(pay)-2:]))
					off := 4
					for c := firstC; c <= lastC && off+6 <= len(pay)-2; c++ {
						rk := binary.LittleEndian.Uint32(pay[off+2 : off+6])
						setCell(r, c, formatXLSNum(decodeBIFFRK(rk)))
						off += 6
					}
				}
			case 0x0203:
				if len(pay) >= 14 {
					r := int(binary.LittleEndian.Uint16(pay[0:2]))
					c := int(binary.LittleEndian.Uint16(pay[2:4]))
					bits := binary.LittleEndian.Uint64(pay[6:14])
					setCell(r, c, formatXLSNum(math.Float64frombits(bits)))
				}
			case 0x0006:
				if len(pay) >= 14 {
					r := int(binary.LittleEndian.Uint16(pay[0:2]))
					c := int(binary.LittleEndian.Uint16(pay[2:4]))
					if pay[12] == 0xFF && pay[13] == 0xFF {
						if pay[6] == 0 {
							lastFormulaRow, lastFormulaCol = r, c
						}
					} else {
						bits := binary.LittleEndian.Uint64(pay[6:14])
						setCell(r, c, formatXLSNum(math.Float64frombits(bits)))
					}
				}
			case 0x0207:
				if lastFormulaRow >= 0 && len(pay) >= 3 {
					sLen := int(binary.LittleEndian.Uint16(pay[0:2]))
					setCell(lastFormulaRow, lastFormulaCol, decodeBIFFString(pay[3:], sLen, (pay[2]&1) != 0))
					lastFormulaRow, lastFormulaCol = -1, -1
				}
			}
		}
		rowIndices := make([]int, 0, len(rowsMap))
		for r := range rowsMap {
			rowIndices = append(rowIndices, r)
		}
		sort.Ints(rowIndices)
		for _, r := range rowIndices {
			if len(out.Segments) >= MaxSegments {
				break
			}
			cols := rowsMap[r]
			colIndices := make([]int, 0, len(cols))
			for c := range cols {
				colIndices = append(colIndices, c)
			}
			sort.Ints(colIndices)
			parts := make([]string, 0, len(colIndices))
			for _, c := range colIndices {
				ref := xlsColName(c) + strconv.Itoa(r+1)
				parts = append(parts, ref+": "+cols[c])
			}
			out.add("SHEET_ROW", map[string]any{"sheet": sh.name, "row": r + 1, "sheet_index": sIdx + 1}, strings.Join(parts, " | "))
		}
	}
	return nil
}

func extractOLE2WorkbookStream(data []byte) ([]byte, error) {
	if len(data) < 512 {
		return nil, errors.New("truncated OLE2 header")
	}
	sectorShift := binary.LittleEndian.Uint16(data[30:32])
	if sectorShift != 9 && sectorShift != 12 {
		return nil, errors.New("unsupported OLE2 sector shift")
	}
	sectorSize := 1 << sectorShift
	numFAT := int(binary.LittleEndian.Uint32(data[44:48]))
	dirStart := binary.LittleEndian.Uint32(data[48:52])
	difatStart := binary.LittleEndian.Uint32(data[68:72])
	numDIFAT := int(binary.LittleEndian.Uint32(data[72:76]))

	difat := make([]uint32, 0, 109+numDIFAT*(sectorSize/4))
	for i := 0; i < 109; i++ {
		off := 76 + i*4
		difat = append(difat, binary.LittleEndian.Uint32(data[off:off+4]))
	}
	sec := difatStart
	for i := 0; i < numDIFAT && i < 1000 && sec < 0xFFFFFFFE; i++ {
		pos := int(sec+1) * sectorSize
		if pos < 0 || pos+sectorSize > len(data) {
			break
		}
		cnt := (sectorSize / 4) - 1
		for j := 0; j < cnt; j++ {
			off := pos + j*4
			difat = append(difat, binary.LittleEndian.Uint32(data[off:off+4]))
		}
		sec = binary.LittleEndian.Uint32(data[pos+cnt*4 : pos+(cnt+1)*4])
	}
	if numFAT > len(difat) {
		numFAT = len(difat)
	}
	fat := make([]uint32, 0, numFAT*(sectorSize/4))
	for _, s := range difat[:numFAT] {
		if s >= 0xFFFFFFFE {
			continue
		}
		pos := int(s+1) * sectorSize
		if pos < 0 || pos+sectorSize > len(data) {
			continue
		}
		for off := pos; off+4 <= pos+sectorSize; off += 4 {
			fat = append(fat, binary.LittleEndian.Uint32(data[off:off+4]))
		}
	}
	readChain := func(start uint32, maxBytes int) []byte {
		var buf bytes.Buffer
		cur := start
		for steps := 0; cur < 0xFFFFFFFE && int(cur) < len(fat) && buf.Len() < maxBytes && steps < 60000; steps++ {
			pos := int(cur+1) * sectorSize
			if pos < 0 || pos+sectorSize > len(data) {
				break
			}
			buf.Write(data[pos : pos+sectorSize])
			cur = fat[cur]
		}
		return buf.Bytes()
	}
	dirBytes := readChain(dirStart, 2<<20)
	for off := 0; off+128 <= len(dirBytes); off += 128 {
		nameLen := int(binary.LittleEndian.Uint16(dirBytes[off+64 : off+66]))
		if nameLen < 2 || nameLen > 64 {
			continue
		}
		name := decodeUTF16LE(dirBytes[off : off+nameLen-2])
		objType := dirBytes[off+66]
		startSec := binary.LittleEndian.Uint32(dirBytes[off+116 : off+120])
		streamSize := int(binary.LittleEndian.Uint32(dirBytes[off+120 : off+124]))
		if objType == 2 && (strings.EqualFold(name, "Workbook") || strings.EqualFold(name, "Book")) && streamSize > 0 {
			if streamSize > maxDecodedXML {
				streamSize = maxDecodedXML
			}
			stream := readChain(startSec, streamSize+sectorSize)
			if len(stream) > streamSize {
				stream = stream[:streamSize]
			}
			return stream, nil
		}
	}
	return nil, errors.New("no Workbook stream found in OLE2 container")
}

func parseBIFF8SST(chunks [][]byte) []string {
	if len(chunks) == 0 || len(chunks[0]) < 8 {
		return nil
	}
	uniqueCount := int(binary.LittleEndian.Uint32(chunks[0][4:8]))
	if uniqueCount <= 0 {
		return nil
	}
	if uniqueCount > 100000 {
		uniqueCount = 100000
	}
	cIdx, pos := 0, 8
	ensure := func() bool {
		for cIdx < len(chunks) && pos >= len(chunks[cIdx]) {
			cIdx++
			pos = 0
		}
		return cIdx < len(chunks)
	}
	readU8 := func() byte {
		if !ensure() {
			return 0
		}
		b := chunks[cIdx][pos]
		pos++
		return b
	}
	readU16 := func() int {
		b0 := int(readU8())
		b1 := int(readU8())
		return b0 | (b1 << 8)
	}
	readU32 := func() int {
		lo := readU16()
		hi := readU16()
		return lo | (hi << 16)
	}
	skip := func(n int) {
		for n > 0 && ensure() {
			avail := len(chunks[cIdx]) - pos
			take := n
			if take > avail {
				take = avail
			}
			pos += take
			n -= take
		}
	}
	out := make([]string, 0, uniqueCount)
	for i := 0; i < uniqueCount && ensure(); i++ {
		charLen := readU16()
		flags := readU8()
		isUTF16 := (flags & 0x01) != 0
		hasExt := (flags & 0x04) != 0
		hasRich := (flags & 0x08) != 0
		richRuns := 0
		if hasRich {
			richRuns = readU16()
		}
		extSize := 0
		if hasExt {
			extSize = readU32()
		}
		var sb strings.Builder
		rem := charLen
		started := false
		for rem > 0 && ensure() {
			if pos == 0 && started {
				contFlag := readU8()
				isUTF16 = (contFlag & 0x01) != 0
				if !ensure() {
					break
				}
			}
			started = true
			avail := len(chunks[cIdx]) - pos
			if isUTF16 {
				take := avail / 2
				if take > rem {
					take = rem
				}
				if take == 0 {
					_ = readU8()
					continue
				}
				sb.WriteString(decodeUTF16LE(chunks[cIdx][pos : pos+take*2]))
				pos += take * 2
				rem -= take
			} else {
				take := avail
				if take > rem {
					take = rem
				}
				sb.WriteString(decodeLatin1(chunks[cIdx][pos : pos+take]))
				pos += take
				rem -= take
			}
		}
		if richRuns > 0 {
			skip(richRuns * 4)
		}
		if extSize > 0 {
			skip(extSize)
		}
		out = append(out, sb.String())
	}
	return out
}

func decodeBIFFString(b []byte, charLen int, isUTF16 bool) string {
	if charLen <= 0 || len(b) == 0 {
		return ""
	}
	if isUTF16 {
		need := charLen * 2
		if need > len(b) {
			need = len(b) &^ 1
		}
		return decodeUTF16LE(b[:need])
	}
	if charLen > len(b) {
		charLen = len(b)
	}
	return decodeLatin1(b[:charLen])
}

func decodeUTF16LE(b []byte) string {
	u := make([]uint16, len(b)/2)
	for i := range u {
		u[i] = binary.LittleEndian.Uint16(b[i*2 : i*2+2])
	}
	return string(utf16.Decode(u))
}

func decodeLatin1(b []byte) string {
	runes := make([]rune, len(b))
	for i, c := range b {
		runes[i] = rune(c)
	}
	return string(runes)
}

func decodeBIFFRK(rk uint32) float64 {
	mul100 := (rk & 1) != 0
	isInt := (rk & 2) != 0
	var val float64
	if isInt {
		val = float64(int32(rk) >> 2)
	} else {
		bits := uint64(rk&^3) << 32
		val = math.Float64frombits(bits)
	}
	if mul100 {
		val /= 100.0
	}
	return val
}

func formatXLSNum(v float64) string {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return ""
	}
	if math.Abs(v-math.Round(v)) < 1e-9 {
		return strconv.FormatInt(int64(math.Round(v)), 10)
	}
	return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.4f", v), "0"), ".")
}

func xlsColName(col int) string {
	s := ""
	for c := col + 1; c > 0; {
		c--
		s = string(rune('A'+(c%26))) + s
		c /= 26
	}
	return s
}
