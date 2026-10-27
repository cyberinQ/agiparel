package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func samplePassport() rollPassportInput {
	return rollPassportInput{RollID: "ROLL-1001", EPC: "300833B2DDD9014000000001", MaterialSKU: "FAB-100", SupplierLot: "LOT-A", ShadeBand: "A", BinLocation: "RACK-01", DeclaredLengthM: "100.000", MeasuredLengthM: "98.750", UsableWidthMM: 1480, InspectionResult: "HOLD", SourceNote: "Measured in stores; awaiting separate physical authority check", ExpectedRevision: 0}
}
func TestPassportExactMillimeters(t *testing.T) {
	for _, tt := range []struct {
		input string
		want  int64
		valid bool
	}{
		{"98.750", 98750, true}, {"0.001", 1, true}, {"100", 100000, true}, {"1.010", 1010, true},
		{"0", 0, false}, {"-2.000", 0, false}, {"2.0001", 0, false}, {"NaN", 0, false}, {"1e4", 0, false}, {"", 0, false}, {" 1.000", 0, false},
	} {
		got, err := passportMillimeters(tt.input)
		if (err == nil) != tt.valid || (tt.valid && got != tt.want) {
			t.Errorf("%q got=%d err=%v", tt.input, got, err)
		}
	}
}
func TestPassportVarianceExact(t *testing.T) {
	for _, tc := range []struct {
		mm   int64
		want string
	}{{-1250, "-1.250"}, {0, "+0.000"}, {150, "+0.150"}} {
		if got := passportVariance(tc.mm); got != tc.want {
			t.Errorf("variance %d = %s, want %s", tc.mm, got, tc.want)
		}
	}
}
func TestPassportRequiredEvidenceAndTags(t *testing.T) {
	good := samplePassport()
	declared, measured, err := validatePassport(good)
	if err != nil || declared != 100000 || measured != 98750 {
		t.Fatalf("valid observation %d %d %v", declared, measured, err)
	}
	cases := []struct {
		name   string
		mutate func(*rollPassportInput)
	}{
		{"missing roll ID", func(v *rollPassportInput) { v.RollID = "" }},
		{"identifier injection", func(v *rollPassportInput) { v.RollID = "a'; DROP TABLE tenants;--" }},
		{"bad EPC", func(v *rollPassportInput) { v.EPC = "abcdef" }},
		{"odd EPC", func(v *rollPassportInput) { v.EPC = "300833B2DDD90140000000011" }},
		{"no sku", func(v *rollPassportInput) { v.MaterialSKU = "" }},
		{"no lot", func(v *rollPassportInput) { v.SupplierLot = "" }},
		{"no shade", func(v *rollPassportInput) { v.ShadeBand = "" }},
		{"no location", func(v *rollPassportInput) { v.BinLocation = "" }},
		{"no declared length", func(v *rollPassportInput) { v.DeclaredLengthM = "0" }},
		{"non-numeric measured", func(v *rollPassportInput) { v.MeasuredLengthM = "invalid" }},
		{"zero width", func(v *rollPassportInput) { v.UsableWidthMM = 0 }},
		{"unapproved status", func(v *rollPassportInput) { v.InspectionResult = "AUTHORISED" }},
		{"no evidence explanation", func(v *rollPassportInput) { v.SourceNote = "yes" }},
		{"bad document", func(v *rollPassportInput) { v.SourceDocumentID = "DOC-never-trust-client" }},
		{"negative expected revision", func(v *rollPassportInput) { v.ExpectedRevision = -1 }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := good
			tc.mutate(&p)
			if _, _, err := validatePassport(p); err == nil {
				t.Fatalf("invalid passport accepted: %+v", p)
			}
		})
	}
	good.EPC = ""
	if _, _, err := validatePassport(good); err != nil {
		t.Fatalf("roll without physical tag must be permitted: %v", err)
	}
}
func TestStoresAPIFailClosedAndMethodGuards(t *testing.T) {
	t.Setenv("ENV", "development")
	t.Setenv("AGIPAREL_OPERATOR_TENANT", "dev")
	s := &EdgeServer{}
	v := samplePassport()
	raw, _ := json.Marshal(v)
	for _, tc := range []struct {
		method, contentType string
		want                int
	}{
		{"POST", "application/json", 503}, {"GET", "", 503}, {"POST", "text/plain", 503},
	} {
		r := httptest.NewRequest(tc.method, "/api/v1/stores/passports", bytes.NewReader(raw))
		r.Header.Set("Content-Type", tc.contentType)
		w := httptest.NewRecorder()
		s.handleStoresPassports(w, r)
		if w.Code != tc.want {
			t.Errorf("%s got=%d want=%d", tc.method, w.Code, tc.want)
		}
	}
	for _, method := range []string{"DELETE", "PUT"} {
		r := httptest.NewRequest(method, "/stores", nil)
		w := httptest.NewRecorder()
		s.handleStoresWorkspace(w, r)
		if w.Code != 405 {
			t.Errorf("stores page %s=%d", method, w.Code)
		}
	}
	r := httptest.NewRequest(http.MethodGet, "/stores", nil)
	w := httptest.NewRecorder()
	s.handleStoresWorkspace(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("stores page status=%d", w.Code)
	}
	// Saving an observed roll is not an inventory-admission action.
	for _, required := range []string{`id="passport-form"`, `id="roll-admission-workbench"`, "does not create available stock"} {
		if !strings.Contains(w.Body.String(), required) {
			t.Errorf("stores page missing %q", required)
		}
	}
}
