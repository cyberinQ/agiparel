// Author: Sunil Jain, 2026-10-28.
// Chapter 07 contract regression: fail-closed, exact QC attestations, scoped reads.
package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

const qcSampleCompletion = "12345678-1234-1234-1234-123456789abc"

func qcSampleCommand() qcProdCommand {
	return qcProdCommand{CommandID: "QUALITY-20261028-TEST-001", CompletionID: qcSampleCompletion, PlantID: "PLANT-A", StationID: "QC-A", Action: "INSPECT_HOLD", InspectedUnits: 12, DefectiveUnits: 2, DefectCode: "BROKEN_STITCH", Note: "Group held for repair"}
}
func TestQCProdValidationAndChallengeBinding(t *testing.T) {
	c := qcSampleCommand()
	if e := qcProdValid(c); e != nil {
		t.Fatal(e)
	}
	base := qcProdChallenge(c)
	for _, mutate := range []func(*qcProdCommand){
		func(c *qcProdCommand) { c.PlantID = "PLANT-B" },
		func(c *qcProdCommand) { c.StationID = "QC-B" },
		func(c *qcProdCommand) { c.CompletionID = "87654321-4321-4321-4321-123456789abc" },
		func(c *qcProdCommand) { c.DefectiveUnits = 3 },
		func(c *qcProdCommand) { c.Note = "changed" },
		func(c *qcProdCommand) { c.Action = "REINSPECT_HOLD" },
	} {
		x := c
		mutate(&x)
		if qcProdChallenge(x) == base {
			t.Fatal("unsigned QC change", x)
		}
	}
	for _, mutate := range []func(*qcProdCommand){
		func(c *qcProdCommand) { c.Action = "REPAIR_COMPLETE" },
		func(c *qcProdCommand) { c.DefectiveUnits = 13 },
		func(c *qcProdCommand) { c.DefectCode = "" },
		func(c *qcProdCommand) { c.InspectedUnits = 0 },
		func(c *qcProdCommand) { c.CompletionID = "bad" },
		func(c *qcProdCommand) { c.Note = strings.Repeat("x", 1001) },
	} {
		x := c
		mutate(&x)
		if qcProdValid(x) == nil {
			t.Fatal("invalid QC action accepted", x)
		}
	}
	c.Action = "INSPECT_PASS"
	c.DefectiveUnits = 0
	c.DefectCode = ""
	if qcProdValid(c) != nil {
		t.Fatal("valid full-pass request rejected")
	}
	c.Action = "REPAIR_COMPLETE"
	c.InspectedUnits = 0
	if qcProdValid(c) != nil {
		t.Fatal("valid repair note rejected")
	}
	c.Note = ""
	if qcProdValid(c) == nil {
		t.Fatal("repair without observation note accepted")
	}
}
func TestQCProdRolesAndPages(t *testing.T) {
	if action, role := qcProdRole("REPAIR_COMPLETE"); action != "quality.repair" || role != "quality_repairer" {
		t.Fatal(action, role)
	}
	if action, role := qcProdRole("INSPECT_HOLD"); action != "quality.inspect" || role != "quality_inspector" {
		t.Fatal(action, role)
	}
	when := time.Date(2026, 10, 28, 12, 34, 56, 123456789, time.UTC)
	c := qcProdCursor(when, qcSampleCompletion)
	at, id, e := qcProdParseCursor(c)
	if e != nil || !at.Equal(when) || id != qcSampleCompletion {
		t.Fatal(at, id, e)
	}
	for _, bad := range []string{"bad", "!!!!", strings.Repeat("a", 193)} {
		if _, _, e := qcProdParseCursor(bad); e == nil {
			t.Fatal("bad cursor accepted", bad)
		}
	}
}
func TestQCProdCannotRunInDevelopmentOrWithoutSewing(t *testing.T) {
	t.Setenv("ENV", "development")
	t.Setenv("AGIPAREL_QUALITY_PRODUCTION_ENABLED", "true")
	t.Setenv("AGIPAREL_SEWING_PRODUCTION_ENABLED", "true")
	q := newQCProdAPI(&EdgeServer{})
	if q.enabled {
		t.Fatal("production QC should fail closed in development")
	}
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/api/v1/quality/production/events", strings.NewReader(`{}`))
	q.event(w, r)
	if w.Code != 503 {
		t.Fatalf("development QC write must fail closed: %d", w.Code)
	}
	t.Setenv("AGIPAREL_QUALITY_PRODUCTION_ENABLED", "false")
	q = newQCProdAPI(&EdgeServer{})
	if q.enabled {
		t.Fatal("explicit QC feature gate ignored")
	}
}
func TestQCProdReadRequiresAuthenticatedIdentity(t *testing.T) {
	q := &qcProdAPI{enabled: true, sewing: &sewProdAPI{enabled: true}}
	w := httptest.NewRecorder()
	q.catalog(w, httptest.NewRequest(http.MethodGet, "/api/v1/quality/production/catalog", nil))
	if w.Code != 401 {
		t.Fatalf("catalog must require bearer identity: %d", w.Code)
	}
}
