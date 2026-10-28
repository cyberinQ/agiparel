// Author: Sunil Jain, 2026-10-28
// Safely read small, non-executable Excel worksheets as tabular evidence.
// The caller always retains original workbook bytes; conversion is for validation only.
package main

import (
	"archive/zip"
	"bytes"
	"encoding/csv"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
)

const spreadsheetMaxUpload = 8 << 20
const spreadsheetMaxExpanded = 20 << 20

var sheetCellRef = regexp.MustCompile(`^([A-Z]{1,3})[1-9][0-9]*$`)

type sourceTable struct {
	Original, CSV []byte
	Name          string
	Spreadsheet   bool
}

func readSourceTable(h *multipart.FileHeader, header []string, allowEmpty bool) (sourceTable, error) {
	name := h.Filename
	if name == "" || len(name) > 255 || strings.ContainsAny(name, "/\\") || filepath.Base(name) != name {
		return sourceTable{}, errors.New("choose a file with a simple filename")
	}
	ext := strings.ToLower(filepath.Ext(name))
	if ext != ".csv" && ext != ".xlsx" {
		return sourceTable{}, errors.New("choose an Excel .xlsx or CSV file; use Upload Records for PDF, Word or pictures")
	}
	f, e := h.Open()
	if e != nil {
		return sourceTable{}, e
	}
	defer f.Close()
	limit := int64(intakeMaxFile)
	if ext == ".xlsx" {
		limit = spreadsheetMaxUpload
	}
	raw, e := io.ReadAll(io.LimitReader(f, limit+1))
	if e != nil {
		return sourceTable{}, e
	}
	if int64(len(raw)) > limit {
		return sourceTable{}, fmt.Errorf("%s exceeds file size limit", name)
	}
	src := sourceTable{Original: raw, CSV: raw, Name: name, Spreadsheet: ext == ".xlsx"}
	if src.Spreadsheet {
		src.CSV, e = workbookCSV(raw, header, allowEmpty)
		if e != nil {
			return sourceTable{}, fmt.Errorf("%s: %w", name, e)
		}
	}
	return src, nil
}

type excelCell struct {
	Ref     string  `xml:"r,attr"`
	Type    string  `xml:"t,attr"`
	Value   string  `xml:"v"`
	Formula *string `xml:"f"`
	Inline  struct {
		Text string `xml:"t"`
		Rich []struct {
			Text string `xml:"t"`
		} `xml:"r"`
	} `xml:"is"`
}
type excelRow struct {
	Cells []excelCell `xml:"c"`
}
type excelSI struct {
	Text string `xml:"t"`
	Rich []struct {
		Text string `xml:"t"`
	} `xml:"r"`
}

func excelText(t string, rich []struct {
	Text string `xml:"t"`
}) string {
	for _, r := range rich {
		t += r.Text
	}
	return t
}
func colIndex(ref string) (int, error) {
	groups := sheetCellRef.FindStringSubmatch(strings.ToUpper(ref))
	if groups == nil {
		return 0, fmt.Errorf("invalid worksheet cell address %q", ref)
	}
	n := 0
	for _, c := range groups[1] {
		n = n*26 + int(c-'A') + 1
	}
	return n - 1, nil
}
func workbookCSV(raw []byte, header []string, allowEmpty bool) ([]byte, error) {
	z, e := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if e != nil {
		return nil, errors.New("not a valid .xlsx workbook")
	}
	if len(z.File) > 200 {
		return nil, errors.New("workbook contains too many parts")
	}
	files := map[string]*zip.File{}
	var expanded uint64
	for _, f := range z.File {
		name := strings.ToLower(f.Name)
		if strings.Contains(f.Name, "..") || strings.HasPrefix(f.Name, "/") || strings.Contains(f.Name, "\\") {
			return nil, errors.New("unsafe workbook paths")
		}
		if f.UncompressedSize64 > spreadsheetMaxExpanded {
			return nil, errors.New("worksheet is too large")
		}
		expanded += f.UncompressedSize64
		if expanded > spreadsheetMaxExpanded {
			return nil, errors.New("workbook expands beyond safe limit")
		}
		if strings.Contains(name, "vbaproject") || strings.Contains(name, "embeddings/") || strings.Contains(name, "activex/") || strings.Contains(name, "connections.xml") || strings.Contains(name, "querytables/") {
			return nil, errors.New("workbook includes executable or external content; save a values-only .xlsx copy")
		}
		files[name] = f
	}
	if files["[content_types].xml"] == nil || files["xl/workbook.xml"] == nil {
		return nil, errors.New("not an Excel .xlsx workbook")
	}
	shared := []string{}
	if sf := files["xl/sharedstrings.xml"]; sf != nil {
		b, e := boundedZipRead(sf)
		if e != nil {
			return nil, e
		}
		var ss struct {
			Items []excelSI `xml:"si"`
		}
		if e = xml.Unmarshal(b, &ss); e != nil {
			return nil, errors.New("invalid shared strings")
		}
		if len(ss.Items) > 50000 {
			return nil, errors.New("too many shared strings")
		}
		for _, s := range ss.Items {
			shared = append(shared, excelText(s.Text, s.Rich))
		}
	}
	sheets := []string{}
	for name := range files {
		if strings.HasPrefix(name, "xl/worksheets/") && strings.HasSuffix(name, ".xml") && !strings.Contains(name, "/_rels/") {
			sheets = append(sheets, name)
		}
	}
	sort.Strings(sheets)
	if len(sheets) == 0 || len(sheets) > 40 {
		return nil, errors.New("expected 1–40 worksheets")
	}
	var found []byte
	matches := 0
	for _, name := range sheets {
		file, e := boundedZipRead(files[name])
		if e != nil {
			return nil, e
		}
		rows, formulas, e := readXLSXRows(file, shared, len(header))
		if e != nil {
			return nil, fmt.Errorf("worksheet %s: %w", name, e)
		}
		start := -1
		for i, row := range rows {
			if i >= 30 {
				break
			}
			if len(row) != len(header) {
				continue
			}
			ok := true
			for j, h := range header {
				if strings.TrimSpace(row[j]) != h {
					ok = false
					break
				}
			}
			if ok {
				start = i
				break
			}
		}
		if start < 0 {
			continue
		}
		for i := start; i < len(rows); i++ {
			if formulas[i] {
				return nil, fmt.Errorf("worksheet %s row %d: formula cells require a values-only copy", name, i+1)
			}
		}
		matches++
		if matches > 1 {
			return nil, errors.New("more than one worksheet has the expected columns; keep one relevant sheet")
		}
		var buf bytes.Buffer
		w := csv.NewWriter(&buf)
		if e = w.Write(header); e != nil {
			return nil, e
		}
		count := 0
		for i := start + 1; i < len(rows); i++ {
			row := rows[i]
			blank := true
			for _, c := range row {
				if strings.TrimSpace(c) != "" {
					blank = false
					break
				}
			}
			if blank {
				continue
			}
			if len(row) != len(header) {
				return nil, fmt.Errorf("row %d: expected %d columns", i+1, len(header))
			}
			count++
			if count > intakeMaxRows {
				return nil, errors.New("more than 2,000 rows")
			}
			if e = w.Write(row); e != nil {
				return nil, e
			}
		}
		if count == 0 && !allowEmpty {
			return nil, errors.New("worksheet has headings but no data rows")
		}
		w.Flush()
		if e = w.Error(); e != nil {
			return nil, e
		}
		if buf.Len() > intakeMaxFile {
			return nil, errors.New("table exceeds 2 MiB")
		}
		found = buf.Bytes()
	}
	if matches == 0 {
		return nil, fmt.Errorf("no worksheet has the required columns: %s", strings.Join(header, ", "))
	}
	return found, nil
}
func boundedZipRead(f *zip.File) ([]byte, error) {
	rc, e := f.Open()
	if e != nil {
		return nil, e
	}
	defer rc.Close()
	data, e := io.ReadAll(io.LimitReader(rc, spreadsheetMaxExpanded+1))
	if e != nil {
		return nil, e
	}
	if len(data) > spreadsheetMaxExpanded {
		return nil, errors.New("workbook content too large")
	}
	return data, nil
}
func readXLSXRows(data []byte, shared []string, width int) ([][]string, []bool, error) {
	decoder := xml.NewDecoder(bytes.NewReader(data))
	decoder.Strict = true
	result := [][]string{}
	formulaRows := []bool{}
	rowCount := 0
	for {
		token, e := decoder.Token()
		if e == io.EOF {
			break
		}
		if e != nil {
			return nil, nil, errors.New("malformed worksheet XML")
		}
		start, ok := token.(xml.StartElement)
		if !ok || start.Name.Local != "row" {
			continue
		}
		rowCount++
		if rowCount > intakeMaxRows+40 {
			return nil, nil, errors.New("worksheet contains too many rows")
		}
		var row excelRow
		if e = decoder.DecodeElement(&row, &start); e != nil {
			return nil, nil, errors.New("malformed worksheet row")
		}
		values := make([]string, width)
		seen := map[int]bool{}
		formulaPresent := false
		last := -1
		for _, cell := range row.Cells {
			if cell.Formula != nil {
				return nil, nil, errors.New("formula cells are not accepted; save a values-only workbook")
			}
			col := last + 1
			if cell.Ref != "" {
				col, e = colIndex(cell.Ref)
				if e != nil {
					return nil, nil, e
				}
			}
			last = col
			if col > 16383 {
				return nil, nil, errors.New("too many worksheet columns")
			}
			if seen[col] {
				return nil, nil, errors.New("duplicate worksheet cell")
			}
			seen[col] = true
			value := cell.Value
			switch cell.Type {
			case "s":
				idx, pe := strconv.Atoi(value)
				if pe != nil || idx < 0 || idx >= len(shared) {
					return nil, nil, errors.New("invalid shared string reference")
				}
				value = shared[idx]
			case "inlineStr":
				value = excelText(cell.Inline.Text, cell.Inline.Rich)
			case "str", "n", "", "b":
			case "e":
				return nil, nil, errors.New("worksheet includes error cells")
			default:
				return nil, nil, fmt.Errorf("unsupported cell type %q", cell.Type)
			}
			if !utf8.ValidString(value) || strings.ContainsRune(value, '\x00') {
				return nil, nil, errors.New("invalid cell text")
			}
			if col >= width {
				if strings.TrimSpace(value) != "" {
					return nil, nil, fmt.Errorf("unexpected data beyond column %d", width)
				}
				continue
			}
			values[col] = strings.TrimSpace(value)
		}
		result = append(result, values)
		formulaRows = append(formulaRows, formulaPresent)
	}
	return result, formulaRows, nil
}
