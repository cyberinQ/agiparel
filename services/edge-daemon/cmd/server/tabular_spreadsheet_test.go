// Author: Sunil Jain, 2026-10-28
// Spreadsheet uploads must preserve originals, reject active content and validate identical CSV semantics.
package main

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"mime/multipart"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func testXLSX(rows [][]string, withFormula, duplicateSheet, activeContent bool) []byte {
	var buf bytes.Buffer
	writer := zip.NewWriter(&buf)
	put := func(name, data string) {
		f, e := writer.Create(name)
		if e != nil {
			panic(e)
		}
		if _, e = f.Write([]byte(data)); e != nil {
			panic(e)
		}
	}
	put("[Content_Types].xml", "<Types/>")
	put("xl/workbook.xml", "<workbook/>")
	var sheet strings.Builder
	sheet.WriteString(`<worksheet><sheetData>`)
	for i, row := range rows {
		sheet.WriteString(fmt.Sprintf(`<row r="%d">`, i+1))
		for j, cell := range row {
			col := string(rune('A' + j))
			var encoded bytes.Buffer
			_ = xml.EscapeText(&encoded, []byte(cell))
			sheet.WriteString(fmt.Sprintf(`<c r="%s%d" t="inlineStr">`, col, i+1))
			if withFormula && i == 1 && j == 0 {
				sheet.WriteString(`<f>HYPERLINK("http://bad")</f>`)
			}
			sheet.WriteString(`<is><t>` + encoded.String() + `</t></is></c>`)
		}
		sheet.WriteString(`</row>`)
	}
	sheet.WriteString(`</sheetData></worksheet>`)
	put("xl/worksheets/sheet1.xml", sheet.String())
	if duplicateSheet {
		put("xl/worksheets/sheet2.xml", sheet.String())
	}
	if activeContent {
		put("xl/vbaProject.bin", "fake binary")
	}
	_ = writer.Close()
	return buf.Bytes()
}
func testUpload(t *testing.T, filename string, content []byte) *multipart.FileHeader {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	f, e := mw.CreateFormFile("file", filename)
	if e != nil {
		t.Fatal(e)
	}
	_, _ = f.Write(content)
	_ = mw.Close()
	req := httptest.NewRequest("POST", "/", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	if e = req.ParseMultipartForm(8 << 20); e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() {
		if req.MultipartForm != nil {
			req.MultipartForm.RemoveAll()
		}
	})
	return req.MultipartForm.File["file"][0]
}
func TestStructuredExcelWithoutUserCSV(t *testing.T) {
	rows := [][]string{{"po_number", "style_code", "color_code", "size_code", "ordered_qty", "upc"}, {"PO-501", "ST-501", "BLUE", "M", "15", "012345"}, {"PO-501", "ST-501", "BLUE", "L", "25", "012346"}}
	workbook := testXLSX(rows, false, false, false)
	src, e := readSourceTable(testUpload(t, "buyer.xlsx", workbook), []string{"po_number", "style_code", "color_code", "size_code", "ordered_qty", "upc"}, false)
	if e != nil {
		t.Fatal(e)
	}
	if !bytes.Equal(src.Original, workbook) || !strings.Contains(string(src.CSV), "012345") {
		t.Fatal("source bytes or leading-zero UPC lost")
	}
	parsed, e := parseCSV(src.CSV, []string{"po_number", "style_code", "color_code", "size_code", "ordered_qty", "upc"})
	if e != nil || len(parsed) != 2 {
		t.Fatalf("parsed rows: %v / %v", parsed, e)
	}
	for _, sample := range []struct {
		label string
		data  []byte
	}{
		{"formula", testXLSX(rows, true, false, false)},
		{"ambiguous sheets", testXLSX(rows, false, true, false)},
		{"macro", testXLSX(rows, false, false, true)},
	} {
		if _, e = readSourceTable(testUpload(t, "buyer.xlsx", sample.data), []string{"po_number", "style_code", "color_code", "size_code", "ordered_qty", "upc"}, false); e == nil {
			t.Fatalf("accepted %s", sample.label)
		}
	}
	if _, e = readSourceTable(testUpload(t, "buyer.xls", workbook), []string{"po_number"}, false); e == nil {
		t.Fatal("legacy .xls incorrectly interpreted as xlsx")
	}
}
func TestExcelBlankTemplatesAndHeaderOnlyStock(t *testing.T) {
	testCases := []struct {
		filename   string
		headers    []string
		allowEmpty bool
	}{
		{"agiparel_buyer_po_template.xlsx", []string{"po_number", "style_code", "color_code", "size_code", "ordered_qty", "upc"}, false},
		{"agiparel_style_bom_template.xlsx", []string{"style_code", "material_sku", "category", "qty_per_garment", "uom"}, false},
		{"agiparel_stock_check_template.xlsx", stockColumns, true},
		{"agiparel_material_rates_template.xlsx", priceColumns, false},
		{"agiparel_marker_positions_template.xlsx", cuttingSheetHeaders["marker_positions"], false},
		{"agiparel_pattern_parts_template.xlsx", cuttingSheetHeaders["pattern_parts"], false},
		{"agiparel_issue_allocations_template.xlsx", cuttingSheetHeaders["issue_allocations"], false},
	}
	for _, tc := range testCases {
		t.Run(tc.filename, func(t *testing.T) {
			b, e := os.ReadFile(filepath.Join("..", "..", "internal", "web", "static", tc.filename))
			if e != nil {
				t.Fatal(e)
			}
			csv, e := workbookCSV(b, tc.headers, tc.allowEmpty)
			if tc.allowEmpty {
				if e != nil {
					t.Fatalf("header-only stock template invalid: %v", e)
				}
			} else if e == nil || !strings.Contains(e.Error(), "no data rows") {
				t.Fatalf("template should be empty until filled: %v", e)
			}
			if tc.allowEmpty && !strings.HasPrefix(string(csv), strings.Join(tc.headers, ",")) {
				t.Fatal("missing expected heading")
			}
		})
	}
}

func TestIntakeExcelPreservesOriginalBinary(t *testing.T) {
	po := testXLSX([][]string{{"po_number", "style_code", "color_code", "size_code", "ordered_qty", "upc"}, {"PO-501", "ST-501", "BLUE", "M", "15", "012345"}}, false, false, false)
	bom := testXLSX([][]string{{"style_code", "material_sku", "category", "qty_per_garment", "uom"}, {"ST-501", "TWILL-501", "FABRIC", "1.850", "M"}}, false, false, false)
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	part, e := mw.CreateFormFile("po_file", "buyer.xlsx")
	if e != nil {
		t.Fatal(e)
	}
	_, _ = part.Write(po)
	part, e = mw.CreateFormFile("bom_file", "materials.xlsx")
	if e != nil {
		t.Fatal(e)
	}
	_, _ = part.Write(bom)
	_ = mw.Close()
	req := httptest.NewRequest("POST", "/api/v1/order-intake?action=preview", &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	sub, e := receiveIntake(httptest.NewRecorder(), req)
	if e != nil {
		t.Fatal(e)
	}
	if sub.Report.OrderedQty != 15 || sub.Report.Materials[0].Required != "27.750" {
		t.Fatalf("wrong Excel quantities: %+v", sub.Report)
	}
	if !bytes.Equal(sub.PO, po) || !bytes.Equal(sub.BOM, bom) {
		t.Fatal("Excel original evidence was modified")
	}
	if sub.Report.POFile != "buyer.xlsx" || sub.Report.BOMFile != "materials.xlsx" {
		t.Fatal("Excel source filename missing")
	}
}

func TestMaterialScenarioExcelSheetsKeepOriginalEvidence(t *testing.T) {
	stock := testXLSX([][]string{stockColumns}, false, false, false)
	priceRows := [][]string{priceColumns, {"TWILL-501", "FABRIC", "M", "2.5000", "USD", "FACTORY", "SRC-1"}}
	prices := testXLSX(priceRows, false, false, false)
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	for _, pair := range []struct {
		field, name string
		data        []byte
	}{{"stock_file", "stock.xlsx", stock}, {"prices_file", "rates.xlsx", prices}} {
		f, e := mw.CreateFormFile(pair.field, pair.name)
		if e != nil {
			t.Fatal(e)
		}
		_, _ = f.Write(pair.data)
	}
	_ = mw.WriteField("intake_id", "OI-"+strings.Repeat("a", 32))
	_ = mw.Close()
	req := httptest.NewRequest("POST", "/api/v1/material-scenario?action=preview", &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	id, s, p, e := receiveScenario(httptest.NewRecorder(), req)
	if e != nil {
		t.Fatal(e)
	}
	if id != "OI-"+strings.Repeat("a", 32) || !bytes.Equal(s.Original, stock) || !bytes.Equal(p.Original, prices) {
		t.Fatal("original workbook evidence was altered")
	}
	intake := intakeReport{ID: id, Status: "PROPOSED", SourceSHA256: strings.Repeat("b", 64), PONumber: "PO-501", StyleCode: "ST-501", OrderedQty: 10, Materials: []intakeMaterial{{SKU: "TWILL-501", Category: "FABRIC", UOM: "M", Required: "18.500"}}}
	report, e := buildScenario(intake, s.CSV, p.CSV, s.Name, p.Name)
	if e != nil {
		t.Fatal(e)
	}
	if len(report.Report.Materials) != 1 || report.Report.Materials[0].Gross != "18.500" || report.Report.StockFile != "stock.xlsx" {
		t.Fatalf("wrong material scenario: %+v", report.Report)
	}
}

func TestCuttingSpreadsheetPreviewDoesNotCommit(t *testing.T) {
	xlsx := testXLSX([][]string{{"position", "color_code", "size_code"}, {"1", "NAVY", "M"}}, false, false, false)
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	_ = writer.WriteField("kind", "marker_positions")
	f, e := writer.CreateFormFile("file", "marker.xlsx")
	if e != nil {
		t.Fatal(e)
	}
	_, _ = f.Write(xlsx)
	_ = writer.Close()
	req := httptest.NewRequest("POST", "/api/v1/tabular-preview", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	result := httptest.NewRecorder()
	new(EdgeServer).handleTabularPreview(result, req)
	if result.Code != 200 {
		t.Fatalf("worksheet rejected (%d): %s", result.Code, result.Body.String())
	}
	var parsed struct {
		Lines    []string `json:"lines"`
		Filename string   `json:"filename"`
	}
	if e = json.Unmarshal(result.Body.Bytes(), &parsed); e != nil {
		t.Fatal(e)
	}
	if len(parsed.Lines) != 1 || parsed.Lines[0] != "1,NAVY,M" || parsed.Filename != "marker.xlsx" {
		t.Fatalf("unexpected preview: %+v", parsed)
	}
	// Unknown worksheet types never get a writable endpoint or a silent fallback.
	var bad bytes.Buffer
	writer = multipart.NewWriter(&bad)
	_ = writer.WriteField("kind", "approve_production")
	f, _ = writer.CreateFormFile("file", "marker.xlsx")
	_, _ = f.Write(xlsx)
	_ = writer.Close()
	req = httptest.NewRequest("POST", "/api/v1/tabular-preview", &bad)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	result = httptest.NewRecorder()
	new(EdgeServer).handleTabularPreview(result, req)
	if result.Code != 400 {
		t.Fatalf("unexpected worksheet kind status: %d", result.Code)
	}
}
