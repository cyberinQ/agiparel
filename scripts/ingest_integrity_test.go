package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMalformedMachineRateAndRollNeverBecomeFactoryFacts(t *testing.T) {
	rows := [][]string{
		{"MACHINE_ID", "CANONICAL_CLASS", "RATED_UPH", "HARDWARE_ASSET_STATUS"},
		{"MC-101", "SNLS_LOCKSTITCH", "bad", "ACTIVE"},
		{"MC-102", "SNLS_LOCKSTITCH", "100", "ACTIVE"},
		{"WORKER_ID", "ATTENDANCE_STATUS", "HOURLY_RATE_INR"},
		{"W-101", "PRESENT", "unknown"},
		{"W-102", "PRESENT", "0"},
		{"ROLL_ID", "MEASURED_LM", "USABLE_WIDTH_CM"},
		{"ROLL-01", "garbage", "148"},
		{"ROLL-02", "59", "0"},
		{"ROLL-03", "59", "148"},
	}
	got := &RawExtractionResult{}
	parseTabularRowsAt(rows, got, 3)
	if len(got.Machines) != 1 || got.Machines[0].MachineID != "MC-102" || got.Machines[0].CapacityUPH != 100 {
		t.Fatalf("invalid machine capacity promoted: %+v", got.Machines)
	}
	if len(got.Workers) != 1 || got.Workers[0].WorkerID != "W-102" || got.Workers[0].HourlyRateINR != 0 {
		t.Fatalf("invalid rate was promoted or confirmed zero rejected: %+v", got.Workers)
	}
	if len(got.StoreRolls) != 1 || got.StoreRolls[0].RollID != "ROLL-03" {
		t.Fatalf("invalid stock dimensions promoted: %+v", got.StoreRolls)
	}
	if len(got.ValidationIssues) != 4 {
		t.Fatalf("expected 4 row issues, got %+v", got.ValidationIssues)
	}
	for _, v := range got.ValidationIssues {
		if v.Row < 2 || v.Sheet != 3 || v.Resolution == "" {
			t.Fatalf("missing source/next action: %+v", v)
		}
	}
	data, _ := json.Marshal(got)
	if !strings.Contains(string(data), "validation_issues") {
		t.Fatalf("validation issues absent from persisted JSON: %s", data)
	}
}
func TestBuyerPOCannotFabricateFactoryCapacityOrStyleSAM(t *testing.T) {
	got := &RawExtractionResult{}
	parseTabularRows([][]string{{"PURCHASE_ORDER", "STYLE_CODE", "TOTAL_ORDER_QTY", "FACTORY_LINE", "TARGET_DAILY_OUTPUT"}, {"REP3", "GM09Y094", "57600", "Line 02", "650"}}, got)
	if len(got.WorkOrders) != 1 || got.WorkOrders[0].OrderedQty != 57600 {
		t.Fatalf("lost valid order: %+v", got.WorkOrders)
	}
	if len(got.Lines) != 0 || len(got.Styles) != 0 {
		t.Fatalf("invented station count or total SAM: lines=%+v styles=%+v", got.Lines, got.Styles)
	}
	if len(got.AdditionalFacts) < 2 {
		t.Fatalf("source facts lost: %+v", got.AdditionalFacts)
	}
}
func TestInvalidNumericRowsNeverTriggerAIOverride(t *testing.T) {
	p := filepath.Join(t.TempDir(), "machines.csv")
	if err := os.WriteFile(p, []byte("MACHINE_ID,CANONICAL_CLASS,RATED_UPH\nM-1,SNLS,not-a-number\n"), 0600); err != nil {
		t.Fatal(err)
	}
	got, mode, err := processFileHybrid(nil, "", "", p)
	if err != nil || mode != "DETERMINISTIC_CSV" {
		t.Fatalf("invalid recognized row should be held locally: %s %v", mode, err)
	}
	if len(got.Machines) != 0 || len(got.ValidationIssues) != 1 {
		t.Fatalf("unexpected promoted fact or missing error: %+v", got)
	}
}
func TestManifestRejectsInvalidAndWritesDurably(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "manifest.json")
	if e := os.WriteFile(p, []byte("{"), 0600); e != nil {
		t.Fatal(e)
	}
	if _, e := loadManifest(p); e == nil {
		t.Fatal("corrupted manifest silently accepted")
	}
	m := &ProcessedManifest{Files: map[string]FileRecord{"x.pdf": {SHA256: "fingerprint", Status: "CAPTURED"}}}
	if e := saveManifest(p, m); e != nil {
		t.Fatal(e)
	}
	got, e := loadManifest(p)
	if e != nil || got.Files["x.pdf"].SHA256 != "fingerprint" {
		t.Fatalf("manifest roundtrip failed: %+v %v", got, e)
	}
	entries, e := os.ReadDir(dir)
	if e != nil {
		t.Fatal(e)
	}
	if len(entries) != 1 {
		t.Fatalf("temporary file left after atomic replace: %+v", entries)
	}
}
