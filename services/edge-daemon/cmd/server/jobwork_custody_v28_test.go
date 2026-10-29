// Author: Sunil Jain, CyberinQ LLC, 2026-10-29.
package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func f317TestCommand() jobworkCommand {
	return jobworkCommand{CommandID: "OUTBOUND_123456", PlantID: "PLANT01", StationID: "STATION01",
		PartID: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", ScanID: "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb",
		VendorID: "VENDOR01", JobReference: "JOBWORK_2026_001", Process: "EMBROIDERY",
		Note: "Twenty-four piece external embroidery batch recorded by issuer."}
}
func TestF317FieldScope(t *testing.T) {
	c := f317TestCommand()
	if !jobworkValid(c, true) || jobworkValid(c, false) {
		t.Fatal("outbound mode invalid")
	}
	c.Process = "LASER_PRINT"
	if jobworkValid(c, true) {
		t.Fatal("unsupported process accepted")
	}
	c = f317TestCommand()
	c.VendorID = ""
	if jobworkValid(c, true) {
		t.Fatal("unprovisioned vendor syntax")
	}
	c = f317TestCommand()
	c.OutwardID = "cccccccc-cccc-4ccc-8ccc-cccccccccccc"
	if jobworkValid(c, true) || jobworkValid(c, false) {
		t.Fatal("outward/inward modes mixed")
	}
	c.VendorID = ""
	c.Process = ""
	c.JobReference = ""
	if !jobworkValid(c, false) {
		t.Fatal("inward fields invalid")
	}
	c.Note = "bad"
	if jobworkValid(c, false) {
		t.Fatal("unjustified return")
	}
}
func TestF317ChallengeExact(t *testing.T) {
	c := f317TestCommand()
	start := jobworkChallenge(c, true)
	if start == jobworkChallenge(c, false) || !strings.HasPrefix(start, "JOBWORK_OUT_V28:") {
		t.Fatal("domain-separated challenge required")
	}
	c.Note += " Additional verified note."
	if start == jobworkChallenge(c, true) {
		t.Fatal("command tampering not bound")
	}
}
func TestF317FeatureDisabled(t *testing.T) {
	t.Setenv("AGIPAREL_JOBWORK_CUSTODY_ENABLED", "false")
	p := newJobworkAPI(nil)
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/api/v1/jobwork/parts/outward/challenge", strings.NewReader(`{"command":{}}`))
	p.challenge(w, r, true)
	if w.Code != 503 || !strings.Contains(w.Body.String(), "ERR_F317_DISABLED") {
		t.Fatalf("fail closed: %d %s", w.Code, w.Body.String())
	}
}
func TestF317AdmissionError(t *testing.T) {
	if got := jobworkAdmissionError(sewFail(422, "CUSTOM")); got.Error() != "CUSTOM" {
		t.Fatal("unrelated errors mutated")
	}
}
