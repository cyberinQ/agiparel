package main

import (
	"strings"
	"testing"
)

func testCutInput() cutDraftInput {
	return cutDraftInput{
		IntakeID: "OI-1234567890abcdef", CutNo: 1, RequestKey: "cut_request_test_0001", MarkerCode: "MARK-ONE", FabricSKU: "FABRIC-101", ShadeBand: "A",
		MarkerLengthMM: 6500, UsableWidthMM: 1450, ReportedPlies: 30, BundlePlyGroupSize: 20,
		Positions: []cutPosition{{1, "BLACK", "32"}, {2, "BLACK", "34"}},
		Parts:     []cutPart{{"FRONT", 2}, {"BACK", 2}, {"WAIST", 1}}, Note: "Marker prepared from reviewed buyer tech pack.",
	}
}
func TestCutDraftValidationAndClassicGenealogy(t *testing.T) {
	v := testCutInput()
	b, e := buildCutBundles("dev", v, 1)
	if e != nil {
		t.Fatal(e)
	}
	if len(b) != 12 {
		t.Fatalf("bundle count %d", len(b))
	}
	total := 0
	for _, x := range b {
		if x.PlyEnd > 30 || x.PlyStart < 1 || x.PieceCount != (x.PlyEnd-x.PlyStart+1)*x.PiecesPerGarment || x.RFIDState != "UNBOUND_DRAFT_LABEL" {
			t.Fatalf("bad classic bundle %#v", x)
		}
		total += x.PieceCount
	}
	if total != 300 {
		t.Fatalf("pieces %d want 300", total)
	}
	reordered := testCutInput()
	reordered.Positions[0], reordered.Positions[1] = reordered.Positions[1], reordered.Positions[0]
	reordered.Parts[0], reordered.Parts[2] = reordered.Parts[2], reordered.Parts[0]
	bb, e := buildCutBundles("dev", reordered, 1)
	if e != nil {
		t.Fatal(e)
	}
	for i := range b {
		if b[i] != bb[i] {
			t.Fatalf("unstable draft label %d", i)
		}
	}
	if b[0].LabelCode == cutLabel("other", v.IntakeID, 1, 1, b[0].Position, b[0].PlyGroup, b[0].PartCode) {
		t.Fatal("tenant label collision")
	}
	if b[0].LabelCode == cutLabel("dev", v.IntakeID, 1, 2, b[0].Position, b[0].PlyGroup, b[0].PartCode) {
		t.Fatal("revision label collision")
	}
	if !strings.HasPrefix(b[0].LabelCode, "CB08-") {
		t.Fatal("label prefix")
	}
}
func TestCutDraftFailClosedInputs(t *testing.T) {
	checks := []struct {
		name   string
		change func(*cutDraftInput)
	}{
		{"no buyer source", func(v *cutDraftInput) { v.IntakeID = "" }},
		{"no marker", func(v *cutDraftInput) { v.MarkerCode = "" }},
		{"no shade", func(v *cutDraftInput) { v.ShadeBand = "" }},
		{"bad ply", func(v *cutDraftInput) { v.ReportedPlies = 211 }},
		{"zero mm", func(v *cutDraftInput) { v.MarkerLengthMM = 0 }},
		{"duplicate positions", func(v *cutDraftInput) { v.Positions[1].Position = 1 }},
		{"mixed lay fabric colors", func(v *cutDraftInput) { v.Positions[1].ColorCode = "RED" }},
		{"missing position", func(v *cutDraftInput) { v.Positions[1].Position = 3 }},
		{"duplicate parts", func(v *cutDraftInput) { v.Parts[1].PartCode = "FRONT" }},
		{"invalid pieces", func(v *cutDraftInput) { v.Parts[0].PiecesPerGarment = 0 }},
		{"note missing", func(v *cutDraftInput) { v.Note = "x" }},
		{"bad key", func(v *cutDraftInput) { v.RequestKey = "short" }},
	}
	for _, tc := range checks {
		t.Run(tc.name, func(t *testing.T) {
			v := testCutInput()
			tc.change(&v)
			if e := validateCutDraft(v); e == nil {
				t.Fatal("accepted invalid physical draft")
			}
		})
	}
}
func TestCutQuantitiesNeverExceedBuyerSKU(t *testing.T) {
	v := testCutInput()
	wanted := cutCounts(v.Positions, v.ReportedPlies)
	allowed := map[string]int64{"BLACK\x0032": 40, "BLACK\x0034": 50}
	if e := checkCutQuantities(allowed, nil, wanted); e != nil {
		t.Fatal(e)
	}
	if e := checkCutQuantities(allowed, map[string]int64{"BLACK\x0032": 11}, wanted); e == nil || e.Error() != "OVERPLANNED_BUYER_SKU" {
		t.Fatalf("did not stop overcut: %v", e)
	}
	if e := checkCutQuantities(map[string]int64{"BLACK\x0032": 50}, nil, wanted); e == nil || e.Error() != "SKU_NOT_IN_BUYER_ORDER" {
		t.Fatalf("unknown buyer SKU: %v", e)
	}
	if e := checkCutQuantities(nil, nil, wanted); e == nil {
		t.Fatal("missing matrix allowed")
	}
	if e := checkCutQuantities(allowed, map[string]int64{"BLUE\x00M": 1}, wanted); e == nil {
		t.Fatal("unrecognized previously planned SKU should fail closed")
	}
	if e := checkCutQuantities(allowed, map[string]int64{"BLACK\x0032": 41}, map[string]int64{"BLACK\x0034": 1}); e == nil {
		t.Fatal("preexisting overplanned SKU hidden by different proposed SKU")
	}
}
func TestCutSnapshotTamperAndSourceBinding(t *testing.T) {
	v := testCutInput()
	b, _ := buildCutBundles("dev", v, 1)
	s := cutDraftReport{IntakeID: v.IntakeID, SourceOrderSHA256: strings.Repeat("a", 64), Bundles: b, ReportedPlies: 30, BundlePlyGroupSize: 20}
	sha := cutSnapshotHash(s)
	s.SnapshotSHA256 = sha
	if cutSnapshotHash(s) != sha {
		t.Fatal("self reference hash")
	}
	if _, e := decodeCutSnapshot([]byte(`{"reported_plies":31}`), sha); e == nil {
		t.Fatal("forged cut snapshot passed")
	}
	old := cutInputHash(v)
	v.ReportedPlies++
	if cutInputHash(v) == old {
		t.Fatal("ply changes did not change idempotency digest")
	}
}

func TestCutTotalRejectsOverproducedPO(t *testing.T) {
	if e := checkCutTotal(100, map[string]int64{"A": 40}, map[string]int64{"B": 60}); e != nil {
		t.Fatal(e)
	}
	if e := checkCutTotal(100, map[string]int64{"A": 40}, map[string]int64{"B": 61}); e == nil {
		t.Fatal("overall PO quantity exceeded")
	}
	if e := checkCutTotal(0, nil, map[string]int64{"B": 1}); e == nil {
		t.Fatal("zero PO quantity")
	}
}

func TestCutBundlePlyGroupsAndRemainder(t *testing.T) {
	v := testCutInput()
	v.ReportedPlies = 45
	v.BundlePlyGroupSize = 20
	b, e := buildCutBundles("dev", v, 1)
	if e != nil {
		t.Fatal(e)
	}
	if len(b) != 18 {
		t.Fatalf("45 plies × 20 group: wanted 18 subbundles, got %d", len(b))
	}
	groupByPart := map[string]map[int]cutBundle{}
	for _, x := range b {
		key := x.PartCode + "/" + x.ColorCode + "/" + x.SizeCode
		if groupByPart[key] == nil {
			groupByPart[key] = map[int]cutBundle{}
		}
		groupByPart[key][x.PlyGroup] = x
	}
	for _, groups := range groupByPart {
		for group, bounds := range map[int][2]int{1: {1, 20}, 2: {21, 40}, 3: {41, 45}} {
			got, ok := groups[group]
			if !ok || got.PlyStart != bounds[0] || got.PlyEnd != bounds[1] || got.PieceCount != (bounds[1]-bounds[0]+1)*got.PiecesPerGarment {
				t.Fatalf("missing or invalid ply group %d %#v", group, got)
			}
		}
	}
}
