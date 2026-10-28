// Author: Sunil Jain, 2026-10-28.
// Chapter 08: strict hold commands, challenge binding and fail-closed scope.
package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func samplePackProdCommand() packProdCommand {
	return packProdCommand{CommandID: "PACK-STAGE-20261028-00001", CompletionID: "12345678-1234-1234-1234-123456789abc", PlantID: "FACTORY_X", StationID: "PACK_QA_7", ExpectedQCRevision: 2, Note: "Hold for individual garment verification"}
}
func TestPackProdChallengeIntegrity(t *testing.T) {
	c := samplePackProdCommand()
	if err := packProdValid(c); err != nil {
		t.Fatal(err)
	}
	base := packProdChallenge(c)
	for _, change := range []func(*packProdCommand){
		func(v *packProdCommand) { v.CommandID = "PACK-STAGE-20261028-00002" },
		func(v *packProdCommand) { v.CompletionID = "87654321-4321-4321-4321-123456789abc" },
		func(v *packProdCommand) { v.PlantID = "FACTORY_Y" },
		func(v *packProdCommand) { v.StationID = "PACK_QA_8" },
		func(v *packProdCommand) { v.ExpectedQCRevision = 3 },
		func(v *packProdCommand) { v.Note = "altered" },
	} {
		next := c
		change(&next)
		if packProdChallenge(next) == base {
			t.Fatal("mutation unbound", next)
		}
	}
	for _, change := range []func(*packProdCommand){
		func(v *packProdCommand) { v.CommandID = "bad" },
		func(v *packProdCommand) { v.CompletionID = "bad" },
		func(v *packProdCommand) { v.StationID = "" },
		func(v *packProdCommand) { v.ExpectedQCRevision = 0 },
		func(v *packProdCommand) { v.Note = strings.Repeat("z", 1001) },
		func(v *packProdCommand) { v.Note = "\x00" },
	} {
		next := c
		change(&next)
		if err := packProdValid(next); err == nil {
			t.Fatal("accepted invalid command", next)
		}
	}
}
func TestPackProdFeatureGates(t *testing.T) {
	t.Setenv("ENV", "development")
	t.Setenv("AGIPAREL_SEWING_PRODUCTION_ENABLED", "true")
	t.Setenv("AGIPAREL_QUALITY_PRODUCTION_ENABLED", "true")
	t.Setenv("AGIPAREL_PACKING_PRODUCTION_ENABLED", "true")
	p := newPackProdAPI(&EdgeServer{})
	if p.enabled {
		t.Fatal("development must fail closed")
	}
	w := httptest.NewRecorder()
	p.readiness(w, httptest.NewRequest(http.MethodGet, "/api/v1/packing/production/readiness", nil))
	if w.Code != 503 {
		t.Fatal("unexpected readiness status", w.Code)
	}
	p.stage(w, httptest.NewRequest(http.MethodPost, "/api/v1/packing/production/holds", nil))
	if w.Code != 503 {
		t.Fatal("unexpected stage status", w.Code)
	}
}
func TestPackProdRequiresAuthenticatedRead(t *testing.T) {
	p := &packProdAPI{enabled: true, quality: &qcProdAPI{sewing: &sewProdAPI{enabled: true}}}
	w := httptest.NewRecorder()
	p.readiness(w, httptest.NewRequest(http.MethodGet, "/api/v1/packing/production/readiness", nil))
	if w.Code != 401 {
		t.Fatalf("anonymous read was allowed: %d", w.Code)
	}
}
func TestPackProdRouteExemptionsAreExact(t *testing.T) {
	t.Setenv("ENV", "production")
	gate := configuredOperatorGate(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) }))
	for _, path := range []string{"/api/v1/packing/production/readiness", "/api/v1/packing/production/holds/challenge", "/api/v1/packing/production/holds"} {
		w := httptest.NewRecorder()
		gate.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		if w.Code != 204 {
			t.Fatalf("secured route blocked %s: %d", path, w.Code)
		}
	}
	for _, path := range []string{"/api/v1/packing/production/readiness/", "/api/v1/packing/production/unknown", "/api/v1/packing/sim/event", "/api/v1/packing/session/seal"} {
		w := httptest.NewRecorder()
		gate.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		if w.Code != 503 {
			t.Fatalf("untrusted route bypassed gate %s: %d", path, w.Code)
		}
	}
}
func TestPackProdReadinessCannotImplyRelease(t *testing.T) {
	if !strings.Contains(packProdEvidence, "NO_GARMENT_CARTON_OR_DISPATCH_RELEASE") {
		t.Fatal("release contract changed")
	}
}
