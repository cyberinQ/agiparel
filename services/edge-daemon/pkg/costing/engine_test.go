// Author: Sunil Jain, CyberinQ LLC (Generated 2026-11-24)
package costing

import (
	"testing"
	"time"
)

func TestSampleWorkbookCalculationsAndRules(t *testing.T) {
	now := time.Date(2026, 11, 24, 0, 0, 0, 0, time.UTC)

	// 1. V2: Reject mismatched colour subtotals from Sample Workbook defect (§2.2)
	badMatrix := OrderMatrixInput{
		BuyerID: "PERY", StyleID: "KK524867", PORef: "MK6574J9", Currency: "USD",
		OrderDate: now, RequestedShipDate: now.AddDate(0, 2, 0),
		StatedColourTotals: map[string]int{"BLUE": 4000}, // actual rows sum to 3000
		Lines: []OrderMatrixLine{
			{Colour: "BLUE", Size: "30/30", UPC: "111", Qty: 3000},
			{Colour: "GREEN", Size: "32/30", UPC: "222", Qty: 6000},
			{Colour: "BLACK", Size: "34/30", UPC: "333", Qty: 3000},
		},
	}
	if _, err := ValidateOrderMatrix(badMatrix, now); err == nil {
		t.Fatalf("expected ERR_V2_COLOUR_TOTAL_MISMATCH, got nil")
	}

	// Valid 12,000 pc matrix
	badMatrix.StatedColourTotals["BLUE"] = 3000
	sum, err := ValidateOrderMatrix(badMatrix, now)
	if err != nil || sum.TotalQty != 12000 {
		t.Fatalf("expected 12000 qty, got %d, err=%v", sum.TotalQty, err)
	}

	// 2. §8.1 BOM Netting: Inner waistband 1.34 * 12000 * 1.03 = 16562.4 -> ceiling 16563
	wbLine, err := ComputeBOMLine(BOMLineInput{
		LineNo: 3, ItemID: "INNER-WAISTBAND", Category: "TRIM", SupplierID: "SUP-1",
		RawConsumption: "1.34", UOM: "YD", PurchaseUnitMultiple: 1,
		WastagePct: 3.0, UnitPrice: 0.45, PriceCurrency: "USD",
		AvailUnreservedStock: 0, StockSnapshotAt: now, LeadTimeDays: 20,
	}, 12000)
	if err != nil {
		t.Fatalf("unexpected BOM error: %v", err)
	}
	if wbLine.GrossQty != 16562.4 || wbLine.NetQty != 16563 {
		t.Fatalf("expected gross=16562.4 net=16563, got gross=%v net=%v", wbLine.GrossQty, wbLine.NetQty)
	}

	// V4: Reject text consumption like "1.5 yd /pc"
	if _, err := ComputeBOMLine(BOMLineInput{
		LineNo: 1, ItemID: "SHELL", RawConsumption: "1.5 yd /pc", UOM: "YD", StockSnapshotAt: now,
	}, 12000); err == nil {
		t.Fatalf("expected V4 error on text consumption")
	}

	// 3. D3/D3a CPM using paid work minutes
	cpm, err := ComputeCPM(180000.0, 1200000.0)
	if err != nil || cpm != 0.15 {
		t.Fatalf("expected CPM 0.15, got %v (err=%v)", cpm, err)
	}

	// 4. §8.3 Cut allowance: 400 -> 408, 850 -> 865 (capped at 15)
	if got := ComputeCutAllowance(400, 0.02, 1, 15); got != 408 {
		t.Fatalf("expected 408, got %d", got)
	}
	if got := ComputeCutAllowance(850, 0.02, 1, 15); got != 865 {
		t.Fatalf("expected 865, got %d", got)
	}

	// 5. D5 Zero-override gate
	if err := ValidateSubmitAndConfirmGate(false, "RELEASED", VerdictAmber, false, ""); err == nil {
		t.Fatalf("expected D5 error when submitting Amber without joint replan")
	}
	if err := ValidateSubmitAndConfirmGate(false, "RELEASED", VerdictAmber, true, "Jointly shifted ship date by 5 days with IE and Planning"); err != nil {
		t.Fatalf("expected valid submit after joint replan, got %v", err)
	}
}
