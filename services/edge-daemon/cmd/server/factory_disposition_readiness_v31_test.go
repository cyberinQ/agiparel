// Author: Sunil Jain, CyberinQ LLC, 2026-10-29.
package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func testDispositionV31() jobworkDispositionCommandV31 {
	return jobworkDispositionCommandV31{CommandID: "JOBWORK_DISP_00001", PlantID: "PLANT01",
		DiscrepancyID: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", OutwardID: "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb",
		PartID: "cccccccc-cccc-4ccc-8ccc-cccccccccccc", ResolutionKind: "RETURNED_TO_PIECE_HOLD",
		EvidenceRef: "SCANNER_RECEIPT_001", EvidenceSHA256: strings.Repeat("a", 64),
		Note: "The recorded inward scan was followed by a supervised garment hold."}
}
func TestF317DDispositionValidationAndChallenge(t *testing.T) {
	c := testDispositionV31()
	if !validJobworkDispositionV31(c) {
		t.Fatal("valid input rejected")
	}
	h := jobworkDispositionChallengeV31(c)
	c.OutwardID = "dddddddd-dddd-4ddd-8ddd-dddddddddddd"
	if h == jobworkDispositionChallengeV31(c) {
		t.Fatal("outward not challenge-bound")
	}
	c = testDispositionV31()
	c.ResolutionKind = "FINANCIAL_SETTLEMENT"
	if validJobworkDispositionV31(c) {
		t.Fatal("settlement accepted")
	}
	c = testDispositionV31()
	c.EvidenceSHA256 = "not_a_sha"
	if validJobworkDispositionV31(c) {
		t.Fatal("unverified evidence field")
	}
	c = testDispositionV31()
	c.Note = "short"
	if validJobworkDispositionV31(c) {
		t.Fatal("missing explanation")
	}
}
func TestF318BClassifications(t *testing.T) {
	if evaluateShrinkageReadinessV31(shrinkageReadinessEvidenceV31{}) != "NO_CURRENT_APPROVED_SPEC" {
		t.Fatal("missing spec")
	}
	e := shrinkageReadinessEvidenceV31{Revision: 1, MinimumSamples: 3, TotalSamples: 2}
	if evaluateShrinkageReadinessV31(e) != "INSUFFICIENT_REPORTED_SAMPLES" {
		t.Fatal("missing samples")
	}
	e.TotalSamples = 3
	e.OutsideSamples = 1
	if evaluateShrinkageReadinessV31(e) != "OUTSIDE_SPEC_HOLD_FOR_REVIEW" {
		t.Fatal("outside spec")
	}
	e.OutsideSamples = 0
	if evaluateShrinkageReadinessV31(e) != "WITHIN_LIMITS_REPORTED_UNCALIBRATED" {
		t.Fatal("unjustified compliance")
	}
}
func TestF318BStrictQueryAndDisabled(t *testing.T) {
	good := map[string][]string{"tenant_id": {"TENANT01"}, "plant_id": {"PLANT01"}, "roll_id": {"ROLL01"}, "treatment_code": {"WASH"}}
	_, _, _, _, ok := validShrinkageReadinessV31(good)
	if !ok {
		t.Fatal("valid query")
	}
	good["tenant_id"] = []string{"TENANT01", "OTHER"}
	_, _, _, _, ok = validShrinkageReadinessV31(good)
	if ok {
		t.Fatal("duplicate scope")
	}
	delete(good, "tenant_id")
	good["unused"] = []string{"x"}
	_, _, _, _, ok = validShrinkageReadinessV31(good)
	if ok {
		t.Fatal("unknown query")
	}
	p := newJobworkAPI(nil)
	w := httptest.NewRecorder()
	p.dispositionEventV31(w, httptest.NewRequest(http.MethodPost, "/api/v1/jobwork/dispositions", strings.NewReader("{}")))
	if w.Code != 503 {
		t.Fatalf("disabled writer %d", w.Code)
	}
	s := newFabricShrinkageAPIv30(nil)
	w = httptest.NewRecorder()
	s.readinessV31(w, httptest.NewRequest(http.MethodGet, "/api/v1/fabric/shrinkage/readiness", nil))
	if w.Code != 503 {
		t.Fatalf("disabled reader %d", w.Code)
	}
}
