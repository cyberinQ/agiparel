// Author: Sunil Jain, 2026-11-11 — SOP-2 executable acceptance tests and negative controls.
package sop2

import (
	"strings"
	"testing"
)

func TestTargetsExactOptimizerAndExtraBand(t *testing.T) {
	in := TargetRequest{SKUs: []SKU{{Key: "NAVY|M", Ordered: 1000, MinExtraBP: 200, MaxExtraBP: 400}}, POMinExtraBP: 200, POMaxExtraBP: 400, MaxStates: 100000, MaxCuts: 20,
		Markers: []Marker{{ID: "M06", FabricSKU: "FAB-1", Shade: "N1", LengthMM: 8500, WidthMM: 1700, TableLengthMM: 10000, MaxPlies: 60, MinutesPerCut: 80, Ratios: map[string]int64{"NAVY|M": 2}, Approved: true}}}
	got, err := SolveTargets(in)
	if err != nil {
		t.Fatal(err)
	}
	if !got.Feasible || !got.OptimalityProven || got.TargetBySKU["NAVY|M"] < 1020 || got.TargetBySKU["NAVY|M"] > 1040 || got.Cuts != 9 {
		t.Fatalf("wrong optimization: %+v", got)
	}
	if err := ValidateFrozenTargets(in.SKUs, got.TargetBySKU, 200, 400); err != nil {
		t.Fatal(err)
	}
	// No planning intent can silently count as a confirmed cut.
	remain, err := Remaining(got.TargetBySKU, map[string]int64{"NAVY|M": 1000})
	if err != nil || remain["NAVY|M"] <= 0 {
		t.Fatalf("balance %+v err=%v", remain, err)
	}
	if _, err := Remaining(got.TargetBySKU, map[string]int64{"NAVY|M": 1050}); err == nil {
		t.Fatal("overcut accepted")
	}
}

func TestTargetInfeasibilityAndSearchLimit(t *testing.T) {
	s := SKU{Key: "N|M", Ordered: 5, MinExtraBP: 0, MaxExtraBP: 0}
	in := TargetRequest{SKUs: []SKU{s}, POMinExtraBP: 0, POMaxExtraBP: 0, Markers: []Marker{{ID: "PAIR", LengthMM: 1000, WidthMM: 1000, TableLengthMM: 2000, MaxPlies: 3, MinutesPerCut: 1, Ratios: map[string]int64{"N|M": 2}, Approved: true}}}
	result, err := SolveTargets(in)
	if err != nil || result.Feasible {
		t.Fatalf("odd total must be impossible: %+v %v", result, err)
	}
	in.MaxStates = 1
	result, err = SolveTargets(in)
	if err != nil || result.OptimalityProven || !result.SearchTruncated {
		t.Fatalf("search truncation must not claim optimality: %+v %v", result, err)
	}
}

func TestRollPoolOptimizationAndSegregation(t *testing.T) {
	r := RollAllocationRequest{ReserveMM: 1000, Rolls: []Roll{
		{ID: "R52", PlantID: "P1", FabricSKU: "F", Shade: "A", LengthMM: 52000, WidthMM: 1800, Inspected: true, Accepted: true, Relaxed: true},
		{ID: "R30", PlantID: "P1", FabricSKU: "F", Shade: "A", LengthMM: 30400, WidthMM: 1800, Inspected: true, Accepted: true, Relaxed: true},
		{ID: "R-BAD", PlantID: "P1", FabricSKU: "F", Shade: "B", LengthMM: 52000, WidthMM: 1800, Inspected: true, Accepted: true, Relaxed: true},
	}, Lays: []LayDemand{{ID: "L1", PlantID: "P1", FabricSKU: "F", Shade: "A", MarkerMM: 9800, WidthMM: 1700, Plies: 3}}}
	got, err := AllocateRolls(r)
	if err != nil || !got.Feasible || len(got.Assignments) != 1 || got.Assignments[0].RollID != "R30" || got.Assignments[0].EndBitMM != 0 {
		t.Fatalf("bad allocation %+v %v", got, err)
	}
	r.Lays[0].Shade = "Z"
	got, err = AllocateRolls(r)
	if err != nil || got.Feasible {
		t.Fatal("wrong shade accepted")
	}
}

func TestInspectionReconciliationAndRecut(t *testing.T) {
	i := Inspection{RollID: "R1", TicketLengthMM: 52000, ActualLengthMM: 52000, WidthSamplesMM: []int64{1800, 1750, 1780}, Shade: "A", RelaxationDone: true, RelaxationHours: 24, Defects: []Defect{{Type: "HOLE", AtMM: 12000, SizeMM: 10, Points: 2}}}
	policy := InspectionPolicy{MinUsableWidthMM: 1700, MaxDefectPoints: 4, MaxLengthLossBP: 100, RequiredSamples: 3, RequiredRelaxHours: 24}
	got, err := InspectRoll(i, policy)
	if err != nil || !got.Accepted || got.UsableWidthMM != 1750 {
		t.Fatalf("inspection %+v %v", got, err)
	}
	i.Defects[0].Points = 8
	got, _ = InspectRoll(i, policy)
	if got.Accepted {
		t.Fatal("defect rejection bypass")
	}
	b := FabricBalance{IssuedMM: 52000, PliesMM: 49000, ReserveReturnedMM: 1000, EndBitsMM: 2000}
	rec, err := ReconcileFabric(b)
	if err != nil || !rec.Balanced {
		t.Fatalf("fabric ledger %+v %v", rec, err)
	}
	b.EndBitsMM = 1990
	rec, _ = ReconcileFabric(b)
	if rec.Balanced || rec.DifferenceMM != 10 {
		t.Fatalf("imbalance hidden %+v", rec)
	}
	c := RecutRequest{OriginalRollID: "R1", ReserveRollID: "R1", OriginalPanelNumber: "P123", RecutPanelNumber: "P123", OriginalShade: "A", ReserveShade: "A", ReserveAvailableMM: 1000, NeededMM: 700}
	if err := ValidateRecut(c); err != nil {
		t.Fatal(err)
	}
	c.ReserveRollID = "R2"
	if err := ValidateRecut(c); err == nil {
		t.Fatal("wrong-roll recut accepted")
	}
}

func TestSewingSchedulingPriorityAndShortage(t *testing.T) {
	needs := []SewingNeed{
		{ID: "LATE", IntakeID: "OI-1", Style: "S", SKU: "N|M", LineID: "L2", SewingStart: "2026-11-14", DeliveryDate: "2026-11-18", LeadDays: 2, NeedPieces: 10, LineStock: 10, MinimumStock: 0, RemainingPO: 100, MinutesPerPiece: 1, Frozen: true},
		{ID: "RISK", IntakeID: "OI-2", Style: "S", SKU: "N|M", LineID: "L1", SewingStart: "2026-11-14", DeliveryDate: "2026-11-18", LeadDays: 2, NeedPieces: 10, LineStock: 0, MinimumStock: 2, RemainingPO: 100, MinutesPerPiece: 1, Frozen: true},
	}
	got, err := SuggestDailyCut(needs, []CapacityDay{{Date: "2026-11-12", TableID: "T1", AvailableMinutes: 6}})
	if err != nil || len(got.Proposed) != 1 || got.Proposed[0].NeedID != "RISK" || got.Shortages["RISK"] != 6 {
		t.Fatalf("priority failed %+v %v", got, err)
	}
	needs[0].Frozen = false
	if _, err := SuggestDailyCut(needs, []CapacityDay{{Date: "2026-11-12", TableID: "T1", AvailableMinutes: 6}}); err == nil {
		t.Fatal("unfrozen sewing plan accepted")
	}
}

func TestSubbundleMarriageAndOnePlyOneRoll(t *testing.T) {
	mk := func(label, part, tag string) SubBundle {
		return SubBundle{Label: label, FamilyID: "F1", PartCode: part, PlyStart: 1, PlyEnd: 2, PanelStart: 101, PanelEnd: 102, PiecesPerGarment: 1, NFCUID: tag, Bound: true, SourceRollByPly: map[int]string{1: "R1", 2: "R2"}}
	}
	family := []SubBundle{mk("B-A", "FRONT", "TAG1"), mk("B-B", "BACK", "TAG2")}
	links := []MarriagePair{{Left: "B-A", Right: "B-B"}}
	got, err := VerifyFamily(family, links, []string{"B-A", "B-B"})
	if err != nil || !got.Complete || got.AuthorizedRelease {
		t.Fatalf("family logic wrong %+v %v", got, err)
	}
	got, _ = VerifyFamily(family, links, []string{"B-A"})
	if got.Complete || len(got.Missing) != 1 {
		t.Fatalf("partial issue accepted %+v", got)
	}
	family[1].SourceRollByPly[1] = "R3"
	got, _ = VerifyFamily(family, links, []string{"B-A", "B-B"})
	if got.Complete || !strings.Contains(strings.Join(got.Problems, ","), "ROLL_MISMATCH") {
		t.Fatalf("cross-ply shade mismatch allowed %+v", got)
	}
}

func TestFrozenChangeRequiresAllApprovers(t *testing.T) {
	c := ChangeControl{ResourceID: "OI-1", ResourceType: "PO_TARGET", OldVersion: 1, NewVersion: 2, ReasonCode: "PO_REVISION", AffectedStakeholders: []string{"planning", "sewing", "stores"}, ApprovedBy: map[string]bool{"planning": true, "stores": true}}
	ready, missing, err := EvaluateChange(c)
	if err != nil || ready || len(missing) != 1 || missing[0] != "sewing" {
		t.Fatalf("bad approval gate %v %v %v", ready, missing, err)
	}
	c.ApprovedBy["sewing"] = true
	ready, _, err = EvaluateChange(c)
	if err != nil || !ready {
		t.Fatalf("approved not ready %v %v", ready, err)
	}
}
