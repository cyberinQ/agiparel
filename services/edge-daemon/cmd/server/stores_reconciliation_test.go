package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func exampleAssessment() storesRollAssessment {
	return storesRollAssessment{
		RollID: "ROLL-1001", PassportRevision: 2, PassportInspection: "PASS", ObservedMaterialSKU: "FAB-100", ObservedShade: "A", MeasuredMM: 98750, ObservedWidthMM: 1480,
		Ledger: storesLedgerSnapshot{Exists: true, MaterialSKU: "FAB-100", ShadeBand: "A", Status: "AVAILABLE", Inspection: "PASS", OriginalMM: 98750, RemainingMM: 75000, WidthMM: 1480, OpenReservationMM: 12000, IssuedMM: 23750},
	}
}

func hasFinding(v storesRollAssessment, needle string) bool {
	for _, f := range v.Findings {
		if f == needle {
			return true
		}
	}
	return false
}

func TestReconciliationEvidenceIsNotMaterialAuthority(t *testing.T) {
	a := compareStoresEvidence(exampleAssessment())
	if a.State != "AWAITING_SECURE_RECONCILIATION" || !hasFinding(a, "AWAITING_INDEPENDENT_VERIFICATION") {
		t.Fatalf("matching observations granted authority: %+v", a)
	}
	if a.UncommittedMM != 63000 {
		t.Errorf("ledger uncommitted expected 63000 got %d", a.UncommittedMM)
	}
	if strings.Contains(a.State, "APPROVED") {
		t.Fatal("cannot approve stock with basic operator")
	}
}

func TestReconciliationFlagsAndHistoricalLength(t *testing.T) {
	cases := []struct {
		name    string
		update  func(*storesRollAssessment)
		finding string
	}{
		{"missing ledger", func(a *storesRollAssessment) { a.Ledger = storesLedgerSnapshot{} }, "NOT_IN_STOCK_LEDGER"},
		{"reported hold", func(a *storesRollAssessment) { a.PassportInspection = "HOLD" }, "PASSPORT_NOT_PASSED"},
		{"sku", func(a *storesRollAssessment) { a.Ledger.MaterialSKU = "DIFFERENT" }, "MATERIAL_SKU_CONFLICT"},
		{"shade", func(a *storesRollAssessment) { a.Ledger.ShadeBand = "B" }, "SHADE_BAND_CONFLICT"},
		{"width", func(a *storesRollAssessment) { a.Ledger.WidthMM = 1440 }, "USABLE_WIDTH_CONFLICT"},
		{"inspection", func(a *storesRollAssessment) { a.Ledger.Inspection = "UNVERIFIED" }, "LEDGER_INSPECTION_NOT_PASSED"},
		{"quarantine", func(a *storesRollAssessment) { a.Ledger.Status = "QUARANTINE" }, "LEDGER_NOT_RELEASED"},
		{"historical original", func(a *storesRollAssessment) { a.Ledger.OriginalMM = 100000 }, "MEASURED_VS_ORIGINAL_REVIEW"},
		{"overcommitted", func(a *storesRollAssessment) { a.Ledger.OpenReservationMM = 99000 }, "LEDGER_OVERCOMMITTED"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			a := exampleAssessment()
			c.update(&a)
			got := compareStoresEvidence(a)
			if got.State != "REQUIRES_REVIEW" || !hasFinding(got, c.finding) {
				t.Errorf("wanted %s in %+v", c.finding, got)
			}
		})
	}
	a := exampleAssessment()
	a.Ledger.RemainingMM = 70000
	a.Ledger.IssuedMM = 28750
	got := compareStoresEvidence(a)
	if hasFinding(got, "MEASURED_VS_ORIGINAL_REVIEW") {
		t.Fatal("remaining stock must not be compared to original passport measurement")
	}
}

func TestStoresReconciliationInputContract(t *testing.T) {
	good := storesReconciliationInput{RollID: "ROLL-1001", ExpectedPassportRevision: 1, RequestKey: "review-12345", Note: "Roll original GRN differs from passport inspection"}
	if e := validateStoresReconciliation(good); e != nil {
		t.Fatal(e)
	}
	cases := []func(*storesReconciliationInput){
		func(v *storesReconciliationInput) { v.RollID = "DROP; TABLE" },
		func(v *storesReconciliationInput) { v.ExpectedPassportRevision = 0 },
		func(v *storesReconciliationInput) { v.RequestKey = "x" },
		func(v *storesReconciliationInput) { v.RequestKey = "bad; DROP" },
		func(v *storesReconciliationInput) { v.Note = "ok" },
		func(v *storesReconciliationInput) { v.Note = strings.Repeat("x", 1001) },
		func(v *storesReconciliationInput) { v.Note = "hello\x00malicious1234" },
	}
	for i, change := range cases {
		v := good
		change(&v)
		if e := validateStoresReconciliation(v); e == nil {
			t.Errorf("invalid request %d accepted: %+v", i, v)
		}
	}
	raw, _ := json.Marshal(compareStoresEvidence(exampleAssessment()))
	var out storesRollAssessment
	if e := json.Unmarshal(raw, &out); e != nil || len(out.Findings) == 0 {
		t.Errorf("snapshot JSON invalid %v %+v", e, out)
	}
}

func TestStoresReconciliationFailClosedWithoutDatabase(t *testing.T) {
	t.Setenv("ENV", "development")
	t.Setenv("AGIPAREL_OPERATOR_TENANT", "dev")
	s := &EdgeServer{}
	for _, handler := range []http.HandlerFunc{s.handleStoresReconciliation, s.handleStoresReconciliationRequests, s.handleStoresReservations} {
		w := httptest.NewRecorder()
		handler(w, httptest.NewRequest("GET", "/api/v1/stores/reconciliation", nil))
		if w.Code != 503 {
			t.Errorf("no database must fail closed, got %d", w.Code)
		}
	}
	t.Setenv("ENV", "production")
	w := httptest.NewRecorder()
	s.handleStoresReconciliation(w, httptest.NewRequest("GET", "/api/v1/stores/reconciliation", nil))
	if w.Code != 503 {
		t.Fatal("production must require real tenant identity")
	}
}
