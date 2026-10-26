// Author: Sunil Jain, 2026-10-26 — Validate pre-PO surplus eligibility and no cross-buyer allocation.
package stores

import (
	"reflect"
	"testing"
	"time"
)

func TestSurplusGatePreservesBuyerAndReservationBoundaries(t *testing.T) {
	now := time.Date(2026, 10, 26, 0, 0, 0, 0, time.UTC)
	policy := SurplusPolicy{MaxStockAgeDays: 365, GSMTolerancePPM: 30000}
	need := BOMMaterialNeed{TenantID: "A", BuyerID: "BUYER-A", OrderID: "PO-A", FactoryItemID: "FAB-1", Color: "NAVY", Construction: "TWILL", GSM: 220, ShadeBand: "S1", RequiredUsableWidthMM: 1500, RequiredLengthMM: 10000}
	base := AvailableRollSegment{TenantID: "A", FactoryItemID: "FAB-1", Color: "NAVY", Construction: "TWILL", GSM: 220, UsableWidthMM: 1520, ShadeBand: "S1", StockReceivedAt: now.AddDate(0, -1, 0), QualityStatus: "PASS", OwnerType: "FACTORY", ApprovedBuyerIDs: []string{"BUYER-A"}, AvailableUnreservedMM: 6000, SourceEvidenceID: "GRN-1"}
	x := base
	x.SegmentID = "1"
	y := base
	y.SegmentID = "2"
	y.OwnerType = "BUYER"
	y.OwnerBuyerID = "BUYER-B"
	y.AvailableUnreservedMM = 9000
	z := base
	z.SegmentID = "3"
	z.ReservationOrderID = "PO-B"
	got, err := EvaluateSurplusGate(now, policy, need, []AvailableRollSegment{z, y, x})
	if err != nil {
		t.Fatal(err)
	}
	if got.EligibleMM != 6000 || got.NetIndentMM != 4000 || !got.AdvisoryOnly || !reflect.DeepEqual(got.Allocations, []SurplusAllocation{{SegmentID: "1", AllocateMM: 6000, EvidenceID: "GRN-1"}}) {
		t.Fatalf("unexpected %+v", got)
	}
	if len(got.Rejected) != 2 {
		t.Fatalf("expected two blocked segments %+v", got)
	}
}

func TestSurplusGateNeverAssumesUnknownEvidencePasses(t *testing.T) {
	now := time.Now().UTC()
	n := BOMMaterialNeed{TenantID: "T", BuyerID: "B", OrderID: "O", FactoryItemID: "F", Color: "C", Construction: "W", GSM: 200, ShadeBand: "S", RequiredUsableWidthMM: 1300, RequiredLengthMM: 1000}
	bad := AvailableRollSegment{SegmentID: "roll", TenantID: "T", FactoryItemID: "F", Color: "C", Construction: "W", GSM: 200, UsableWidthMM: 1350, ShadeBand: "S", QualityStatus: "PASS", OwnerType: "FACTORY", ApprovedBuyerIDs: []string{"B"}, AvailableUnreservedMM: 1500}
	result, err := EvaluateSurplusGate(now, SurplusPolicy{MaxStockAgeDays: 90}, n, []AvailableRollSegment{bad})
	if err != nil {
		t.Fatal(err)
	}
	if result.NetIndentMM != 1000 || len(result.Rejected) != 1 {
		t.Fatalf("unknown stock age/evidence must block: %+v", result)
	}
}
