package main

import (
	"strings"
	"testing"
)

func sampleCutSpread() cutSpreadInput {
	return cutSpreadInput{DraftID: "123e4567-e89b-12d3-a456-426614174000", RequestKey: "cut08b_tests_unique", ActualPlies: 20, MeasuredMarkerLengthMM: 6000, ConsumptionMM: 125000, Allocations: []cutIssueAllocation{{IssueID: "123e4567-e89b-12d3-a456-426614174002", AllocatedMM: 5000}, {IssueID: "123e4567-e89b-12d3-a456-426614174001", AllocatedMM: 120000}}, Note: "Physical spread reported; verified issue IDs attached."}
}
func TestCutSpreadExactMMAndUniqueness(t *testing.T) {
	v := sampleCutSpread()
	if e := validateCutSpread(v); e != nil {
		t.Fatal(e)
	}
	reversed := v
	reversed.Allocations = []cutIssueAllocation{v.Allocations[1], v.Allocations[0]}
	if cutSpreadInputHash(v) != cutSpreadInputHash(reversed) {
		t.Fatal("allocation order changed hash")
	}
	for _, x := range []struct {
		name   string
		change func(*cutSpreadInput)
	}{
		{"missing coverage", func(v *cutSpreadInput) { v.Allocations[0].AllocatedMM-- }},
		{"overlap", func(v *cutSpreadInput) { v.Allocations[0].IssueID = v.Allocations[1].IssueID }},
		{"minimum marker consumption", func(v *cutSpreadInput) { v.ConsumptionMM = 119999 }},
		{"notes", func(v *cutSpreadInput) { v.Note = "short" }},
		{"zero plies", func(v *cutSpreadInput) { v.ActualPlies = 0 }},
	} {
		t.Run(x.name, func(t *testing.T) {
			w := sampleCutSpread()
			x.change(&w)
			if validateCutSpread(w) == nil {
				t.Fatal("bad spread accepted")
			}
		})
	}
}
func TestCutScanChallengeBindsEveryPhysicalReferent(t *testing.T) {
	v := cutScanChallengeFields{"CUT08B_SIGNED_SCAN_V1", "dev", "PLANT_DEFAULT", "spread-1", "draft-1", strings.Repeat("a", 64), "CB08-0123456789ABCDEF01234567", "300833B2DDD9014000000000"}
	base := cutScanChallenge(v)
	alternatives := []cutScanChallengeFields{}
	for i := 0; i < 7; i++ {
		w := v
		switch i {
		case 0:
			w.TenantID = "other"
		case 1:
			w.PlantID = "other"
		case 2:
			w.SpreadID = "other"
		case 3:
			w.DraftID = "other"
		case 4:
			w.SnapshotSHA = strings.Repeat("b", 64)
		case 5:
			w.LabelCode = "other"
		case 6:
			w.EPCHex = "300833B2DDD9014000000002"
		}
		alternatives = append(alternatives, w)
	}
	for _, w := range alternatives {
		if cutScanChallenge(w) == base {
			t.Fatalf("challenge collision: %+v", w)
		}
	}
	if epc, e := canonicalEPCHex(" 300833b2DDD9014000000000 "); e != nil || epc != "300833B2DDD9014000000000" {
		t.Fatalf("canonical EPC: %s %v", epc, e)
	}
	for _, bad := range []string{"", "ABC", "ZZZZ0000", strings.Repeat("A", 98)} {
		if _, e := canonicalEPCHex(bad); e == nil {
			t.Fatalf("accepted %q", bad)
		}
	}
}
