package main

// Author: Sunil Jain, 2026-10-28
// Protect Chapter 04 demand uncertainty and backward-compatible saved proposal hashes.

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestChapter04UnknownFabricUnitNotZeroDemand(t *testing.T) {
	order, demands, rolls := pickFixture()
	demands[0].Unit = "YARD"
	report, err := buildPickReport(order, demands, rolls)
	if err != nil {
		t.Fatal(err)
	}
	m := report.Materials[0]
	if m.DemandKnown || m.DemandMM != 0 || m.UncoveredMM != 0 || len(m.Picks) != 0 {
		t.Fatalf("unconvertible unit must be unknown, not satisfied: %+v", m)
	}
	raw, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(raw, []byte(`"demand_known":false`)) {
		t.Fatal("false must be omitted for legacy snapshot hash compatibility")
	}
	demands[0].Unit = "M"
	report, err = buildPickReport(order, demands, rolls)
	if err != nil {
		t.Fatal(err)
	}
	if !report.Materials[0].DemandKnown {
		t.Fatal("metre demand must be known")
	}
	raw, err = json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(raw, []byte(`"demand_known":true`)) {
		t.Fatal("new snapshot must expose known demand")
	}
}

func TestChapter04LegacySavedProposalRoundTrip(t *testing.T) {
	// Older immutable snapshots predate the optional demand_known field.
	order, demands, rolls := pickFixture()
	report, err := buildPickReport(order, demands, rolls)
	if err != nil {
		t.Fatal(err)
	}
	report.Materials[0].DemandKnown = false
	old, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(old, []byte("demand_known")) {
		t.Fatal("legacy form must lack the field")
	}
	var decoded pickReport
	if err := json.Unmarshal(old, &decoded); err != nil {
		t.Fatal(err)
	}
	again, err := json.Marshal(decoded)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(old, again) {
		t.Fatal("older signed/hashed snapshot changed during roundtrip")
	}
}
