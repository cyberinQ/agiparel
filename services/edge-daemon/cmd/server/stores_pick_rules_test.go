package main

import (
	"crypto/sha256"
	"encoding/json"
	"math"
	"testing"
)

func pickFixture() (pickOrder, []pickDemand, []pickRoll) {
	o := pickOrder{IntakeID: "ORD-600", PONumber: "PO-600", StyleCode: "S1", Quantity: 600, SourceSHA256: "abcd", Status: "PROPOSED"}
	d := []pickDemand{{MaterialSKU: "FAB-1", Unit: "M", PerGarmentMilli: 1850}}
	r := []pickRoll{
		{ID: "R-A", MaterialSKU: "FAB-1", Shade: "BLUE-A", State: "AVAILABLE", Inspection: "PASS", RemainingMM: 500000, WidthMM: 1480},
		{ID: "R-B", MaterialSKU: "FAB-1", Shade: "BLUE-A", State: "AVAILABLE", Inspection: "PASS", RemainingMM: 200000, OpenReservationMM: 90000, WidthMM: 1490},
		{ID: "R-C", MaterialSKU: "FAB-1", Shade: "BLUE-B", State: "AVAILABLE", Inspection: "PASS", RemainingMM: 400000, WidthMM: 1480},
		{ID: "R-D", MaterialSKU: "FAB-1", Shade: "BLUE-A", State: "RESERVED", Inspection: "PASS", RemainingMM: 100000, OwnerOrder: "ANOTHER-PO", WidthMM: 1480},
		{ID: "R-E", MaterialSKU: "FAB-1", Shade: "BLUE-A", State: "QUARANTINE", Inspection: "PASS", RemainingMM: 100000, WidthMM: 1480},
		{ID: "R-F", MaterialSKU: "FAB-1", Shade: "BLUE-A", State: "AVAILABLE", Inspection: "HOLD", RemainingMM: 100000, WidthMM: 1480},
		{ID: "R-G", MaterialSKU: "FAB-1", Shade: "BLUE-A", State: "AVAILABLE", Inspection: "PASS", RemainingMM: 100000, ForeignReservationMM: 10000, WidthMM: 1480},
		{ID: "R-H", MaterialSKU: "FAB-1", Shade: "BLUE-A", State: "AVAILABLE", Inspection: "PASS", RemainingMM: 100000, ForeignIssuedMM: 1000, WidthMM: 1480},
	}
	return o, d, r
}
func TestPickPlansCannotBorrowOrBlend(t *testing.T) {
	o, d, r := pickFixture()
	p, e := buildPickReport(o, d, r)
	if e != nil {
		t.Fatal(e)
	}
	if p.Status != "PROPOSED_HOLD" || p.Authority != "NONE" {
		t.Fatalf("unauthorized status %+v", p)
	}
	if len(p.Materials) != 1 {
		t.Fatalf("materials=%d", len(p.Materials))
	}
	m := p.Materials[0]
	if m.DemandMM != 1110000 || m.TentativeMM != 610000 || m.UncoveredMM != 500000 || m.ChosenShade != "BLUE-A" {
		t.Fatalf("wrong quantity or shade %+v", m)
	}
	if len(m.Picks) != 2 {
		t.Fatalf("picks=%+v", m.Picks)
	}
	expected := map[string]string{"R-C": "ALTERNATE_SHADE_NOT_COMBINED", "R-D": "HELD_FOR_ANOTHER_ORDER", "R-E": "LEDGER_STOCK_NOT_AVAILABLE", "R-F": "LEDGER_INSPECTION_NOT_PASSED", "R-G": "OPEN_RESERVATION_FOR_ANOTHER_ORDER", "R-H": "PREVIOUSLY_ISSUED_TO_ANOTHER_ORDER"}
	for _, x := range m.Excluded {
		if expected[x.RollID] != x.Reason {
			t.Fatalf("bad exclusion %s %s", x.RollID, x.Reason)
		}
		delete(expected, x.RollID)
	}
	if len(expected) != 0 {
		t.Fatalf("missing excluded rolls: %v", expected)
	}
}
func TestPickPlansNeverInventAvailableStock(t *testing.T) {
	o, d, _ := pickFixture()
	p, e := buildPickReport(o, d, nil)
	if e != nil {
		t.Fatal(e)
	}
	if p.Materials[0].TentativeMM != 0 || p.Materials[0].UncoveredMM != 1110000 {
		t.Fatalf("invented stock %+v", p.Materials[0])
	}
}
func TestPickUnitAndBoundsFailClosed(t *testing.T) {
	o, d, r := pickFixture()
	d[0].Unit = "YARD"
	p, e := buildPickReport(o, d, r)
	if e != nil {
		t.Fatal(e)
	}
	if len(p.Materials[0].Picks) != 0 || p.Materials[0].DemandMM != 0 {
		t.Fatal("yard used as metre")
	}
	d[0].Unit = "M"
	d[0].PerGarmentMilli = math.MaxInt64
	if _, e = buildPickReport(o, d, r); e == nil {
		t.Fatal("overflow not rejected")
	}
	d[0].PerGarmentMilli = 1850
	d = append(d, d[0])
	if _, e = buildPickReport(o, d, r); e == nil {
		t.Fatal("duplicate fabric demand not rejected")
	}
	o.Status = "APPROVED"
	if _, e = buildPickReport(o, d[:1], r); e == nil {
		t.Fatal("non-proposed order accepted")
	}
}
func TestPickEligibility(t *testing.T) {
	o, _, r := pickFixture()
	for _, x := range r {
		_, reason := pickEligibility(x, o)
		switch x.ID {
		case "R-A", "R-B", "R-C":
			if reason != "" {
				t.Fatalf("good roll rejected %s %s", x.ID, reason)
			}
		default:
			if reason == "" {
				t.Fatalf("bad roll accepted %s", x.ID)
			}
		}
	}
	x := r[0]
	x.OpenReservationMM = x.RemainingMM + 1
	if _, reason := pickEligibility(x, o); reason != "INVALID_OR_OVERCOMMITTED_STOCK" {
		t.Fatal(reason)
	}
}
func TestPickRequestValidation(t *testing.T) {
	valid := pickInput{IntakeID: "ORDER-123", RequestKey: "my_key_123", Note: "Check buyer shade and ownership before picking"}
	if e := validatePickInput(valid); e != nil {
		t.Fatal(e)
	}
	for _, bad := range []pickInput{{IntakeID: "../../x", RequestKey: valid.RequestKey, Note: valid.Note}, {IntakeID: valid.IntakeID, RequestKey: "aaa", Note: valid.Note}, {IntakeID: valid.IntakeID, RequestKey: valid.RequestKey, Note: "too short"}} {
		if e := validatePickInput(bad); e == nil {
			t.Fatalf("bad request accepted %+v", bad)
		}
	}
}

func TestPickSnapshotJSONStableForDatabaseRoundtrip(t *testing.T) {
	o, d, r := pickFixture()
	p, err := buildPickReport(o, d, r)
	if err != nil {
		t.Fatal(err)
	}
	original, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	var decoded pickReport
	if err = json.Unmarshal(original, &decoded); err != nil {
		t.Fatal(err)
	}
	canonical, err := json.Marshal(decoded)
	if err != nil {
		t.Fatal(err)
	}
	a, b := sha256.Sum256(original), sha256.Sum256(canonical)
	if a != b {
		t.Fatalf("snapshot canonical hash changed across JSON roundtrip")
	}
}
