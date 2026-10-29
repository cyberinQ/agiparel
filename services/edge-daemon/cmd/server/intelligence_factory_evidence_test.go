// Author: Sunil Jain, 2026-10-28.
package main

import (
	"database/sql"
	"encoding/json"
	"testing"
)

func TestF36RatesDoNotFabricateMissingDenominator(t *testing.T) {
	m := factoryRate("fpy", "qc_garment_events_v21", "initial pass", 0, 0)
	if m.Value != nil || m.Status != "NO_DENOMINATOR" || *m.Denominator != 0 {
		t.Fatalf("false 0%%: %+v", m)
	}
	m = factoryRate("fpy", "qc_garment_events_v21", "initial pass", 7, 8)
	if m.Value == nil || *m.Value != 8750 || m.Status != "DERIVED_SIGNED_ATTESTATIONS" {
		t.Fatalf("wrong FPY: %+v", m)
	}
	m = factoryRate("fpy", "qc_garment_events_v21", "zero first pass", 0, 8)
	if m.Value == nil || *m.Value != 0 {
		t.Fatalf("missing observed zero: %+v", m)
	}
}
func TestF36RecordedTimeProvenanceAndNull(t *testing.T) {
	m := factoryDuration("p95", "wip_movements_v20", "handoff", sql.NullFloat64{})
	if m.Value != nil || m.Status != "NO_HANDOFF_PAIRS" {
		t.Fatalf("fabricated handoff interval: %+v", m)
	}
	m = factoryDuration("p95", "wip_movements_v20", "handoff", sql.NullFloat64{Float64: 12.345, Valid: true})
	if m.Value == nil || *m.Value != 12345 {
		t.Fatalf("invalid milliseconds %+v", m)
	}
}
func TestF36AuthorityAndJSONLabels(t *testing.T) {
	b, e := json.Marshal(factoryEvidenceReport{Authority: factoryEvidenceAuthority, ProductionRelease: false,
		Metrics: []factoryEvidenceMetric{factoryAbsent("oee_pct", "machine_telemetry", "cannot infer", "basis_points", "NOT_MEASURED")}, SlowestRoutes: []factoryRouteTiming{}})
	if e != nil {
		t.Fatal(e)
	}
	var x struct {
		Authority         string                  `json:"authority"`
		ProductionRelease bool                    `json:"production_release"`
		Metrics           []factoryEvidenceMetric `json:"metrics"`
	}
	if e = json.Unmarshal(b, &x); e != nil || x.Authority != factoryEvidenceAuthority || x.ProductionRelease || len(x.Metrics) != 1 || x.Metrics[0].Value != nil {
		t.Fatalf("unsafe JSON: %s: %v", b, e)
	}
}
