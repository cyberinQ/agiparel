package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func evidenceCmdV32(kind string) factoryEvidenceCommandV32 {
	c := factoryEvidenceCommandV32{Kind: kind, CommandID: "factory-evidence-001", PlantID: "PLANT-01", Note: "Independently recorded factory evidence with source context"}
	switch kind {
	case "CALIBRATION_LINK":
		c.ObservationID = "11111111-1111-4111-8111-111111111111"
		c.InstrumentID = "CALIPER-01"
		c.CalibrationRevision = 1
	case "AQL_SAMPLE":
		c.PlanID = "22222222-2222-4222-8222-222222222222"
		c.GarmentID = "33333333-3333-4333-8333-333333333333"
		c.Finding = "PASS"
	case "NEEDLE_SAFETY":
		c.GarmentID = "44444444-4444-4444-8444-444444444444"
		c.EventKind = "NEEDLE_BREAK"
		c.ExpectedFragments = 2
		c.RecoveredFragments = 1
		c.EvidenceDocumentID = "EVIDENCE-01"
		c.EvidenceSHA256 = strings.Repeat("a", 64)
	}
	return c
}
func TestFactoryEvidenceV32BoundActions(t *testing.T) {
	cases := []struct{ kind, action, role string }{
		{"CALIBRATION_LINK", "stores.fabric.calibration.link", "stores_inspector"},
		{"AQL_SAMPLE", "quality.aql.inspect", "quality_inspector"},
		{"NEEDLE_SAFETY", "quality.needle.report", "quality_inspector"},
	}
	for _, tc := range cases {
		c := evidenceCmdV32(tc.kind)
		a, r, ok := factoryEvidencePolicyV32(c)
		if !ok || a != tc.action || r != tc.role {
			t.Fatalf("unexpected policy %s: %s %s %v", tc.kind, a, r, ok)
		}
		signature := factoryEvidenceChallengeV32(c)
		c.Note += " tampered"
		if factoryEvidenceChallengeV32(c) == signature {
			t.Fatal("challenge not bound to evidence payload")
		}
	}
}
func TestFactoryEvidenceV32RejectsCrossWorkflowAndUntrustedValues(t *testing.T) {
	for _, kind := range []string{"CALIBRATION_LINK", "AQL_SAMPLE", "NEEDLE_SAFETY"} {
		c := evidenceCmdV32(kind)
		c.CommandID = "x"
		if _, _, ok := factoryEvidencePolicyV32(c); ok {
			t.Errorf("accepted invalid command: %s", kind)
		}
	}
	c := evidenceCmdV32("AQL_SAMPLE")
	c.Finding = "CRITICAL"
	if _, _, ok := factoryEvidencePolicyV32(c); ok {
		t.Fatal("accepted defect with no defect code")
	}
	c = evidenceCmdV32("CALIBRATION_LINK")
	c.GarmentID = "FORGED"
	if _, _, ok := factoryEvidencePolicyV32(c); ok {
		t.Fatal("accepted unrelated garment field")
	}
	c = evidenceCmdV32("NEEDLE_SAFETY")
	c.RecoveredFragments = 3
	if _, _, ok := factoryEvidencePolicyV32(c); ok {
		t.Fatal("accepted recovered fragments exceeding expected")
	}
	c = evidenceCmdV32("NEEDLE_SAFETY")
	c.EventKind = "DETECTOR_CHALLENGE_PASS"
	if _, _, ok := factoryEvidencePolicyV32(c); ok {
		t.Fatal("accepted detector pass without instrument")
	}
	if _, _, ok := factoryEvidencePolicyV32(factoryEvidenceCommandV32{Kind: "UNRECOGNIZED"}); ok {
		t.Fatal("accepted unrecognized kind")
	}
}
func TestFactoryEvidenceV32FailClosedWithoutConfiguration(t *testing.T) {
	t.Setenv("AGIPAREL_FACTORY_EVIDENCE_V32_ENABLED", "false")
	p := newFactoryEvidenceAPIv32(&EdgeServer{})
	if p.enabled {
		t.Fatal("unconfigured factory evidence enabled")
	}
	for _, endpoint := range []func(http.ResponseWriter, *http.Request){p.challenge, p.event, p.aqlReport, p.needleReport} {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/", nil)
		endpoint(w, req)
		if w.Code < 400 {
			t.Fatalf("unconfigured request succeeded: %d", w.Code)
		}
	}
}
func TestFactoryEvidenceV32TablesAreFixed(t *testing.T) {
	for _, kind := range []string{"CALIBRATION_LINK", "AQL_SAMPLE", "NEEDLE_SAFETY"} {
		table, subject := factoryEvidenceTableV32(kind)
		if table == "" || subject == "" {
			t.Fatalf("missing table mapping %q", kind)
		}
	}
	table, _ := factoryEvidenceTableV32("fabric_calibration_links_v32;DROP TABLE tenants")
	if table != "" {
		t.Fatal("accepted SQL-injected table name")
	}
}
