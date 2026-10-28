// Author: Sunil Jain, 2026-10-28
// Worker-facing tabular inputs accept spreadsheets and keep editable review controls.
package web

import (
	"strings"
	"testing"
)

func TestWorkerSpreadsheetInputsAndTemplates(t *testing.T) {
	for _, page := range []string{"order_intake.html", "material_planning.html"} {
		b, e := staticFS.ReadFile("static/" + page)
		if e != nil {
			t.Fatal(e)
		}
		if !strings.Contains(string(b), `accept=".xlsx,.csv`) {
			t.Errorf("%s: Excel input missing", page)
		}
	}
	cutting, e := staticFS.ReadFile("static/cutting_view.html")
	if e != nil {
		t.Fatal(e)
	}
	for _, needle := range []string{`data-cut-sheet="marker_positions"`, `data-cut-sheet="pattern_parts"`, `data-cut-sheet="issue_allocations"`, `/static/cutting_sheet_upload.js`, `textarea name="positions"`, `textarea name="parts"`, `textarea name="issue_allocations"`} {
		if !strings.Contains(string(cutting), needle) {
			t.Errorf("cutting input missing: %s", needle)
		}
	}
	js, e := staticFS.ReadFile("static/cutting_sheet_upload.js")
	if e != nil {
		t.Fatal(e)
	}
	for _, needle := range []string{"/api/v1/tabular-preview", "textarea.value = data.lines.join", "Review them before saving"} {
		if !strings.Contains(string(js), needle) {
			t.Errorf("worksheet loader missing %s", needle)
		}
	}
	for _, name := range []string{"agiparel_buyer_po_template.xlsx", "agiparel_style_bom_template.xlsx", "agiparel_stock_check_template.xlsx", "agiparel_material_rates_template.xlsx", "agiparel_marker_positions_template.xlsx", "agiparel_pattern_parts_template.xlsx", "agiparel_issue_allocations_template.xlsx"} {
		if _, e = staticFS.ReadFile("static/" + name); e != nil {
			t.Errorf("missing spreadsheet template: %s", name)
		}
	}
}
