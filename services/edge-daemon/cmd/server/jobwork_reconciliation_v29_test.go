// Author: Sunil Jain, CyberinQ LLC, 2026-10-29.
package main

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func f317bCommand() jobworkDiscrepancyCommand {
	return jobworkDiscrepancyCommand{
		CommandID: "DAMAGE_RPT_00001", PlantID: "PLANT01",
		OutwardID: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa",
		PartID:    "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb",
		Category:  "DAMAGED_ON_RETURN", Note: "Frayed seam discovered during internal inward review, needs investigation.",
		EvidenceRef: "VENDOR_REPORT_2026_001", EvidenceSHA256: strings.Repeat("a", 64),
	}
}
func TestF317BDiscrepancyValidation(t *testing.T) {
	c := f317bCommand()
	if !validJobworkDiscrepancy(c) {
		t.Fatal("valid scoped report rejected")
	}
	c.Category = "LOSS_CONFIRMED"
	if validJobworkDiscrepancy(c) {
		t.Fatal("accounting loss must not be asserted")
	}
	c = f317bCommand()
	c.Category = "MISSING_AT_RECONCILIATION"
	if !validJobworkDiscrepancy(c) {
		t.Fatal("missing report rejected")
	}
	c.EvidenceSHA256 = "not-a-sha256"
	if validJobworkDiscrepancy(c) {
		t.Fatal("unverifiable digest accepted")
	}
	c = f317bCommand()
	c.EvidenceRef = "a\nbad"
	if validJobworkDiscrepancy(c) {
		t.Fatal("control character in reference accepted")
	}
	c = f317bCommand()
	c.Note = "short"
	if validJobworkDiscrepancy(c) {
		t.Fatal("reason not specific enough")
	}
	c = f317bCommand()
	c.OutwardID = "not-a-uuid"
	if validJobworkDiscrepancy(c) {
		t.Fatal("outward scope not validated")
	}
}
func TestF317BExactChallenge(t *testing.T) {
	c := f317bCommand()
	s := jobworkDiscrepancyChallenge(c)
	if !strings.HasPrefix(s, "JOBWORK_DISCREPANCY_V29:") {
		t.Fatal("missing challenge domain separation")
	}
	c.EvidenceSHA256 = strings.Repeat("b", 64)
	if s == jobworkDiscrepancyChallenge(c) {
		t.Fatal("source hash not challenge-bound")
	}
	c = f317bCommand()
	c.OutwardID = "cccccccc-cccc-4ccc-8ccc-cccccccccccc"
	if s == jobworkDiscrepancyChallenge(c) {
		t.Fatal("outward identity not challenge-bound")
	}
}
func TestF317BRegisterQueryFailClosed(t *testing.T) {
	good := url.Values{"tenant_id": {"tenant01"}, "plant_id": {"plant01"}, "vendor_id": {"VENDOR01"},
		"job_reference": {"JOBWORK_001"}, "process": {"EMBROIDERY"}}
	if _, _, _, _, _, limit, ok := validJobworkRegister(good); !ok || limit != 50 {
		t.Fatal("legitimate report query rejected")
	}
	bad := make(url.Values)
	for k, v := range good {
		bad[k] = append([]string{}, v...)
	}
	bad.Add("tenant_id", "TENANT02")
	if _, _, _, _, _, _, ok := validJobworkRegister(bad); ok {
		t.Fatal("mixed tenant accepted")
	}
	bad.Del("tenant_id")
	bad.Set("tenant_id", "tenant01")
	bad.Set("limit", "5000")
	if _, _, _, _, _, _, ok := validJobworkRegister(bad); ok {
		t.Fatal("unbounded pages accepted")
	}
	bad.Del("limit")
	bad.Set("process", "OTHER")
	if _, _, _, _, _, _, ok := validJobworkRegister(bad); ok {
		t.Fatal("unsupported process accepted")
	}
	bad.Set("process", "EMBROIDERY")
	bad.Set("after", "bad")
	if _, _, _, _, _, _, ok := validJobworkRegister(bad); ok {
		t.Fatal("non-UUID continuation accepted")
	}
	bad.Del("after")
	bad.Add("extra", "1")
	if _, _, _, _, _, _, ok := validJobworkRegister(bad); ok {
		t.Fatal("unknown query key accepted")
	}
}
func TestF317BDisabledAndMethod(t *testing.T) {
	p := newJobworkAPI(nil)
	r := httptest.NewRequest(http.MethodGet, "/api/v1/jobwork/reconciliation", nil)
	w := httptest.NewRecorder()
	p.register(w, r)
	if w.Code != 503 || !strings.Contains(w.Body.String(), "ERR_F317_DISABLED") {
		t.Fatalf("fail closed: %d %s", w.Code, w.Body.String())
	}
	r = httptest.NewRequest(http.MethodPost, "/api/v1/jobwork/reconciliation", nil)
	w = httptest.NewRecorder()
	p.register(w, r)
	if w.Code != 405 {
		t.Fatal("write method allowed on report")
	}
}
