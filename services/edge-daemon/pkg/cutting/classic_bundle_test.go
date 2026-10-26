// Author: Sunil Jain, 2026-10-26 — Verify classic bundle arithmetic, physical boundaries and serial genealogy.
package cutting

import (
	"reflect"
	"testing"
)

func TestBalanceConsecutivePlies(t *testing.T) {
	cases := []struct {
		n, target, min, max int
		want                []int
		invalid             bool
	}{
		{101, 25, 13, 25, []int{25, 25, 25, 13, 13}, false},
		{102, 25, 13, 25, []int{25, 25, 25, 14, 13}, false},
		{26, 25, 13, 25, []int{13, 13}, false},
		{50, 25, 13, 25, []int{25, 25}, false},
		{12, 25, 13, 25, nil, true},
		{27, 25, 20, 25, nil, true},
		{0, 25, 13, 25, nil, true},
	}
	for _, c := range cases {
		got, err := BalanceConsecutivePlies(c.n, c.target, c.min, c.max)
		if (err != nil) != c.invalid {
			t.Errorf("n=%d: unexpected err %v", c.n, err)
			continue
		}
		if err == nil && !reflect.DeepEqual(got, c.want) {
			t.Errorf("n=%d: got %v want %v", c.n, got, c.want)
		}
	}
}

func TestBuildClassicBundlesUsesActualPliesAndNonOverlappingIdentity(t *testing.T) {
	req := ClassicBundleRequest{
		OrderID: "PO-A", CutNo: 3, ActualPlies: 4, NextBundleNumber: 47, NextGarmentSerial: 201,
		TargetBundleSize: 2, MinBundleSize: 1, MaxBundleSize: 2,
		MarkerPositions: []ClassicMarkerPosition{{Position: 1, Size: "M", Parts: []string{"POCKET", "FRONT"}}, {Position: 2, Size: "L", Parts: []string{"POCKET", "FRONT"}}},
		PlySegments:     []ClassicPlySegment{{First: 1, Last: 2, Lot: "LOT-A", Shade: "S1"}, {First: 3, Last: 4, Lot: "LOT-B", Shade: "S2"}},
	}
	got, err := BuildClassicBundles(req)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Cards) != 8 || got.GarmentCount != 8 || got.NextBundleNumber != 55 || got.NextGarmentSerial != 209 {
		t.Fatalf("wrong output %+v", got)
	}
	for _, card := range got.Cards {
		if card.PlyFirst <= 2 && card.Lot != "LOT-A" || card.PlyFirst > 2 && card.Lot != "LOT-B" {
			t.Errorf("boundary violation %+v", card)
		}
		if card.MarkerPosition == 1 && card.GarmentSerialLast > 204 || card.MarkerPosition == 2 && card.GarmentSerialFirst < 205 {
			t.Errorf("serial overlap %+v", card)
		}
	}
	if got.Cards[0].GarmentSerialFirst != got.Cards[1].GarmentSerialFirst {
		t.Fatal("different parts of the same garment must share garment serial")
	}
	req.PlySegments = []ClassicPlySegment{{First: 1, Last: 1, Lot: "LOT-A", Shade: "S1"}, {First: 3, Last: 4, Lot: "LOT-B", Shade: "S2"}}
	if _, err := BuildClassicBundles(req); err == nil {
		t.Fatal("missing ply must be rejected")
	}
}

func TestActual97PliesNeverCreatesPlanned100(t *testing.T) {
	req := ClassicBundleRequest{OrderID: "PO-97", CutNo: 1, ActualPlies: 97, NextBundleNumber: 1, NextGarmentSerial: 1,
		TargetBundleSize: 25, MinBundleSize: 10, MaxBundleSize: 25,
		MarkerPositions: []ClassicMarkerPosition{{Position: 1, Size: "M", Parts: []string{"FRONT"}}},
		PlySegments:     []ClassicPlySegment{{First: 1, Last: 97, Lot: "LOT-1", Shade: "S1"}}}
	got, err := BuildClassicBundles(req)
	if err != nil {
		t.Fatal(err)
	}
	var total int
	for _, card := range got.Cards {
		total += card.PieceCount
	}
	if got.GarmentCount != 97 || total != 97 || got.NextGarmentSerial != 98 {
		t.Fatalf("phantom planned pieces: %+v", got)
	}
}
