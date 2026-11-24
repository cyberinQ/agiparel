// Author: Sunil Jain, CyberinQ LLC (Generated 2026-11-24)
package costing

import (
	"testing"
	"time"
)

func TestP1P2Extensions(t *testing.T) {
	cutRes, err := ComputeCutAndTagPreview([]OrderMatrixLine{
		{Colour: "BLUE", Size: "30/30", UPC: "U1", Qty: 600},
	}, CutPreviewRule{
		AllowancePct: 0.02, MinExtraPcs: 1, MaxExtraPcs: 15, MaxPlies: 210, BundleSize: 20, CutPartsPerGarment: 16, RFIDTagUnitCost: 0.04,
	})
	if err != nil {
		t.Fatalf("unexpected FR-9 error: %v", err)
	}
	if cutRes.TotalCutQty != 612 || cutRes.EstimatedBundles != 31 || cutRes.TotalRFIDTags != (612+31*16) {
		t.Fatalf("unexpected cut/tag result: %+v", cutRes)
	}

	matrix := []OrderMatrixLine{
		{Colour: "BLACK", Size: "32/30", UPC: "U1", Qty: 4000},
		{Colour: "BLUE", Size: "32/30", UPC: "U2", Qty: 4000},
		{Colour: "GREEN", Size: "32/30", UPC: "U3", Qty: 4000},
	}
	incompleteCartons, err := ComputeCartonPlanPreview(matrix, 800, []CartonPlanLine{
		{
			BoxRangeLabel: "1-50", BoxCount: 50, LengthCM: 60, WidthCM: 40, HeightCM: 30, GrossWeightKG: 6.2, CartonCost: 1.2,
			Assortment: []CartonAssortmentRatio{
				{Colour: "BLACK", Size: "32/30", PcsPerBox: 2},
				{Colour: "BLUE", Size: "32/30", PcsPerBox: 2},
				{Colour: "GREEN", Size: "32/30", PcsPerBox: 2},
			},
		},
	})
	if err != nil {
		t.Fatalf("unexpected FR-10 error: %v", err)
	}
	if incompleteCartons.IsComplete || len(incompleteCartons.Gaps) == 0 {
		t.Fatalf("expected V11 carton gaps, got complete")
	}

	ord := time.Date(2026, 11, 20, 0, 0, 0, 0, time.UTC)
	cut := time.Date(2026, 12, 10, 0, 0, 0, 0, time.UTC)
	tna := ComputeSamplingTNA(ord, cut, ord, nil)
	if !tna.HasPastOrPreOrderFlags {
		t.Fatalf("expected FR-14 pre-order flag when cut is only 20 days after order date")
	}

	ocrVar, err := ComputeOCRCostedVsActual(OCRActualInput{
		POID: "MK6574J9", CostedQty: 12000, ActualShippedQty: 12000,
		CostedSAM: 1.45, ActualAvgSAM: 1.48,
		CostedCMTotal: 2610.0, ActualCMTotal: 2664.0,
		CostedMaterialTotal: 45000.0, ActualMaterialTotal: 44800.0,
		QuotedExFactory: cut.AddDate(0, 0, 20), ActualExFactory: cut.AddDate(0, 0, 19),
	})
	if err != nil || !ocrVar.WithinKPIBand {
		t.Fatalf("expected WithinKPIBand=true, got %+v (err=%v)", ocrVar, err)
	}
}
