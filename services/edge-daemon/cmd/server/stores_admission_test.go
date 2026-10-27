package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

func readyAdmissionFixture() admissionCandidate {
	return admissionCandidate{
		RequestID: "12345678-1234-1234-1234-123456789abc", RollID: "ROLL-501", PassportRevision: 3, AssessmentHash: strings.Repeat("a", 64),
		Inspection: "PASS", MaterialSKU: "FAB-TWILL", SupplierLot: "LOT-10", ShadeBand: "A", BinLocation: "R-03", EPC: "300833B2DDD9014000000001", SourceDocumentID: "DOC-00000000000000000000000000000001",
		MeasuredMM: 99500, WidthMM: 1480, LatestRevision: 3, Defects: "none observed",
		Assessment: storesRollAssessment{RollID: "ROLL-501", PassportRevision: 3, PassportInspection: "PASS", ObservedMaterialSKU: "FAB-TWILL", ObservedShade: "A", MeasuredMM: 99500, ObservedWidthMM: 1480, EPC: "300833B2DDD9014000000001", SourceDocumentID: "DOC-00000000000000000000000000000001"},
	}
}
func TestRollAdmissionRequiresEvidenceAndIndependentSignature(t *testing.T) {
	good := readyAdmissionFixture()
	if reasons := admissionReasons(good); len(reasons) > 0 {
		t.Fatalf("unexpected prerequisites: %v", reasons)
	}
	cases := []struct {
		name   string
		modify func(*admissionCandidate)
		want   string
	}{
		{"hold", func(c *admissionCandidate) { c.Inspection = "HOLD" }, "INSPECTION_NOT_PASSED"},
		{"without RFID", func(c *admissionCandidate) { c.EPC = "" }, "RFID_EPC_NOT_BOUND"},
		{"without original", func(c *admissionCandidate) { c.SourceDocumentID = "" }, "SOURCE_DOCUMENT_REQUIRED"},
		{"stale", func(c *admissionCandidate) { c.LatestRevision++ }, "PASSPORT_REVISION_STALE"},
		{"already ledger", func(c *admissionCandidate) { c.LedgerExists = true }, "ROLL_ALREADY_IN_LEDGER"},
		{"assessment lied", func(c *admissionCandidate) { c.Assessment.ObservedWidthMM = 1600 }, "REVIEW_SNAPSHOT_CONFLICT"},
		{"bad width", func(c *admissionCandidate) { c.WidthMM = 0 }, "INVALID_MEASUREMENTS"},
		{"admitted", func(c *admissionCandidate) { c.IsAdmitted = true }, "ALREADY_ADMITTED"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			v := good
			c.modify(&v)
			if !containsAdmissionReason(admissionReasons(v), c.want) {
				t.Fatalf("expected %s: %v", c.want, admissionReasons(v))
			}
		})
	}
}
func containsAdmissionReason(ss []string, v string) bool {
	for _, s := range ss {
		if s == v {
			return true
		}
	}
	return false
}
func TestAdmissionChallengeBindsEveryAuthoritativeField(t *testing.T) {
	c := readyAdmissionFixture()
	orig := storesAdmissionChallenge("tenant-A", "plant-A", c)
	if len(orig) != 64 || orig != storesAdmissionChallenge("tenant-A", "plant-A", c) {
		t.Fatal("non-deterministic challenge")
	}
	changes := []struct {
		name   string
		change func(*admissionCandidate)
	}{
		{"roll", func(v *admissionCandidate) { v.RollID += "-1" }},
		{"length", func(v *admissionCandidate) { v.MeasuredMM++ }},
		{"width", func(v *admissionCandidate) { v.WidthMM++ }},
		{"epc", func(v *admissionCandidate) { v.EPC = "300833B2DDD9014000000002" }},
		{"source", func(v *admissionCandidate) { v.SourceDocumentID += "-other" }},
		{"review", func(v *admissionCandidate) { v.RequestID = "22222222-1234-1234-1234-123456789abc" }},
		{"snapshot", func(v *admissionCandidate) { v.AssessmentHash = strings.Repeat("b", 64) }},
		{"revision", func(v *admissionCandidate) { v.PassportRevision++ }},
		{"lot", func(v *admissionCandidate) { v.SupplierLot += "1" }},
		{"shade", func(v *admissionCandidate) { v.ShadeBand = "B" }},
		{"bin", func(v *admissionCandidate) { v.BinLocation = "OTHER" }},
		{"material", func(v *admissionCandidate) { v.MaterialSKU = "OTHER" }},
		{"defects", func(v *admissionCandidate) { v.Defects = "some" }},
	}
	for _, x := range changes {
		t.Run(x.name, func(t *testing.T) {
			v := c
			x.change(&v)
			if orig == storesAdmissionChallenge("tenant-A", "plant-A", v) {
				t.Fatal("challenge not bound to " + x.name)
			}
		})
	}
	if orig == storesAdmissionChallenge("tenant-B", "plant-A", c) || orig == storesAdmissionChallenge("tenant-A", "plant-B", c) {
		t.Fatal("tenant or plant not bound")
	}
}
func TestSignedStockPromotionFailsClosedWithoutIdentity(t *testing.T) {
	t.Setenv("AGIPAREL_MATERIAL_API_ENABLED", "false")
	mux := http.NewServeMux()
	registerMaterialRoutes(mux, nil)
	for _, p := range []struct{ method, path string }{{"GET", "/api/v1/material/roll-admissions/challenge?request_id=12345678-1234-1234-1234-123456789abc"}, {"POST", "/api/v1/material/roll-admissions"}} {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(p.method, p.path, bytes.NewReader([]byte(`{"request_id":"12345678-1234-1234-1234-123456789abc"}`)))
		mux.ServeHTTP(w, r)
		if w.Code != 503 {
			t.Fatalf("%s returned %d, expected secure fail-closed", p.path, w.Code)
		}
	}
}
func TestAdmissionFieldsRoundTrip(t *testing.T) {
	c := readyAdmissionFixture()
	b, e := json.Marshal(c)
	if e != nil {
		t.Fatal(e)
	}
	var v admissionCandidate
	if e = json.Unmarshal(b, &v); e != nil {
		t.Fatal(e)
	}
	if !reflect.DeepEqual(c, v) {
		t.Fatalf("round trip mismatch %+v", v)
	}
}
