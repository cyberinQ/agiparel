// Author: Sunil Jain, 2026-10-25 — Regression tests for all seven Epic 1 staging documents and conflict detection.
package main

import (
	"archive/zip"
	"encoding/json"
	"github.com/cyberinq/agiparel/services/edge-daemon/pkg/onboarding"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSevenStagingDocumentsDeterministic(t *testing.T) {
	dir := filepath.Join("..", "tests", "fixtures", "intake")
	docs := []struct {
		name                                         string
		operations, machines, workers, rolls, orders int
	}{
		{"01_Operation_Bulletin_KK524867.xlsx", 14, 0, 0, 0, 0},
		{"02_Buyer_Purchase_Order_MK6574J9.csv", 0, 0, 0, 0, 1},
		{"03_Line14_Machine_Roster_Layout.xlsx", 0, 16, 0, 0, 0},
		{"04_Worker_Skill_Attendance_Register.csv", 0, 0, 17, 0, 0},
		{"05_Fabric_Store_GRN_Inspection.xlsx", 0, 0, 0, 10, 0},
		{"06_Buyer_Packing_Bulletin_Shipping_Spec.csv", 0, 0, 0, 0, 1},
		{"agiparel SAMPLE DATA .xlsx", 0, 0, 0, 0, 5},
	}
	workbook := newWorkbook()
	conflicts := []IngestionConflict{}
	for _, doc := range docs {
		t.Run(doc.name, func(t *testing.T) {
			p := filepath.Join(dir, doc.name)
			var parsed *RawExtractionResult
			var err error
			if filepath.Ext(doc.name) == ".csv" {
				parsed, err = parseDeterministicCSV(p)
			} else {
				parsed, err = parseDeterministicXLSX(p)
			}
			if err != nil {
				t.Fatal(err)
			}
			if !hasExtractedEntities(parsed) {
				t.Fatalf("parsed no entities")
			}
			if len(parsed.OperationsDAG) != doc.operations || len(parsed.Machines) != doc.machines || len(parsed.Workers) != doc.workers || len(parsed.StoreRolls) != doc.rolls || len(parsed.WorkOrders) != doc.orders {
				t.Fatalf("wrong entity counts: operations=%d machines=%d workers=%d rolls=%d orders=%d", len(parsed.OperationsDAG), len(parsed.Machines), len(parsed.Workers), len(parsed.StoreRolls), len(parsed.WorkOrders))
			}
			conflicts = append(conflicts, mergeIntoWorkbook(workbook, parsed)...)
		})
	}
	if len(workbook.OperationsDAG) != 14 || len(workbook.Machines) != 16 || len(workbook.Workers) != 17 || len(workbook.StoreRolls) != 10 || len(workbook.WorkOrders) != 1 {
		t.Fatalf("bad cumulative state")
	}
	if workbook.WorkOrders[0].OrderedQty != 600 {
		t.Fatalf("historical PO overwrote current order: %+v", workbook.WorkOrders[0])
	}
	hasPackingConflict := false
	hasHistoricalQty := false
	for _, c := range conflicts {
		if c.Field == "packing_piece_total" && c.Incoming == 840 {
			hasPackingConflict = true
		}
		if c.Field == "ordered_qty" && c.Incoming == 12000 {
			hasHistoricalQty = true
		}
	}
	if !hasPackingConflict || !hasHistoricalQty {
		t.Fatalf("missing conservation conflict(s): %+v", conflicts)
	}
	if workbook.Workers[0].AttendanceDate != "2026-10-21" {
		t.Fatalf("attendance date lost: %q", workbook.Workers[0].AttendanceDate)
	}
}

func TestPOConflictDoesNotOverwrite(t *testing.T) {
	wb := newWorkbook()
	base := &RawExtractionResult{WorkOrders: []onboarding.WorkOrderRow{{WorkOrderID: "WO-P-01", PONumber: "P", OrderedQty: 600, Status: "PROPOSED"}}}
	mergeIntoWorkbook(wb, base)
	inc := &RawExtractionResult{WorkOrders: []onboarding.WorkOrderRow{{WorkOrderID: "WO-P-01", PONumber: "P", OrderedQty: 12000}}}
	conflicts := mergeIntoWorkbook(wb, inc)
	if len(conflicts) != 1 || wb.WorkOrders[0].OrderedQty != 600 {
		t.Fatalf("conflicting PO not held for review")
	}
}

// Author: Sunil Jain, 2026-10-25 — Unknown Excel layouts must reach Ollama as worksheet text, never ZIP bytes.
func TestUnknownXLSXFallsBackAsReadableText(t *testing.T) {
	path := filepath.Join(t.TempDir(), "unknown_columns.xlsx")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	z := zip.NewWriter(f)
	w, err := z.Create("xl/worksheets/sheet1.xml")
	if err != nil {
		t.Fatal(err)
	}
	_, err = w.Write([]byte(`<worksheet><sheetData><row><c r="A1" t="inlineStr"><is><t>Fabric Batch</t></is></c><c r="B1" t="inlineStr"><is><t>Checked Qty</t></is></c></row><row><c r="A2" t="inlineStr"><is><t>ROLL-42</t></is></c><c r="B2"><v>12</v></c></row></sheetData></worksheet>`))
	if err != nil {
		t.Fatal(err)
	}
	if err := z.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	model := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request OllamaChatRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Errorf("decode request: %v", err)
		}
		if len(request.Messages) < 2 || !strings.Contains(request.Messages[1].Content, "Fabric Batch") || !strings.Contains(request.Messages[1].Content, "ROLL-42") {
			t.Errorf("XLSX cells were not provided as readable text: %+v", request.Messages)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"message":{"content":"{\"lines\":[{\"line_id\":\"Line 14\"}]}"}}`))
	}))
	defer model.Close()
	result, mode, err := processFileHybrid(model.Client(), model.URL, "test-installed-model", path)
	if err != nil {
		t.Fatal(err)
	}
	if mode != "AI_LLM_FALLBACK" || len(result.Lines) != 1 {
		t.Fatalf("wrong result %+v mode %s", result, mode)
	}
}
