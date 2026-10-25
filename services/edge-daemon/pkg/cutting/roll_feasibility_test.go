// Author: Sunil Jain, 2026-10-25
// Validate read-only roll allocation, reservation isolation and shade boundaries.
package cutting

import "testing"

func TestRollFeasibilityDoesNotSpendReservedOrWrongShade(t *testing.T) {
	rolls := []RollCandidate{{RollID: "A", MaterialSKU: "COTTON", WidthCM: 150, UsableLengthM: 12, ShadeBand: "1"}, {RollID: "B", MaterialSKU: "COTTON", WidthCM: 150, UsableLengthM: 20, ShadeBand: "1", ReservedFor: "OTHER"}, {RollID: "C", MaterialSKU: "COTTON", WidthCM: 150, UsableLengthM: 20, ShadeBand: "2"}}
	marker := []MarkerCandidate{{MarkerID: "G-1", MaterialSKU: "COTTON", MinimumWidthCM: 145, MaximumWidthCM: 155, LengthPerPlyM: 2, PiecesPerPly: 3, MaxPlies: 4}}
	x, err := EvaluateRollAllocation("PO1", "COTTON", "1", 9, rolls, marker)
	if err != nil {
		t.Fatal(err)
	}
	if !x.AdvisoryOnly || len(x.Allocations) != 1 || x.Allocations[0].RollID != "A" || x.PlannedPieces != 9 || x.UnfilledPieces != 0 {
		t.Fatalf("unexpected allocation: %+v", x)
	}
	if rolls[0].UsableLengthM != 12 {
		t.Fatal("input stock mutated")
	}
}
func TestRollFeasibilityAsksMissingInformation(t *testing.T) {
	x, err := EvaluateRollAllocation("PO1", "COTTON", "", 100, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(x.Questions) < 2 || x.UnfilledPieces != 100 {
		t.Fatalf("expected manager questions: %+v", x)
	}
}
