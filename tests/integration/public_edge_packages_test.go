// Author: Sunil Jain, 2026-10-28
// Verify OCR and packing are importable and usable outside the edge-daemon subtree.
package integration

import (
	"testing"

	"github.com/cyberinq/agiparel/services/edge-daemon/pkg/ocr"
	"github.com/cyberinq/agiparel/services/edge-daemon/pkg/packing"
)

func TestPublicOCRPackage(t *testing.T) {
	report, err := ocr.EvaluateConservation("ORDER-101", 1, ocr.StageBreakdown{
		CutQty: 10, ShippedQty: 8, LostQty: 1, LeftOverQty: 1,
	}, "")
	if err != nil {
		t.Fatalf("OCR conservation failed: %v", err)
	}
	if !report.IsConserved || report.Variance != 0 {
		t.Fatalf("unexpected OCR reconciliation: %+v", report)
	}
	for _, dept := range ocr.RequiredDepartments {
		if err := report.SignOff(dept); err != nil {
			t.Fatalf("OCR signoff for %s failed: %v", dept, err)
		}
	}
	if !report.CanSeal() {
		t.Fatal("conserved and signed report should be sealable")
	}
}

func TestPublicPackingPackage(t *testing.T) {
	session := packing.NewCartonPackingSession(1, "RATIO", []*packing.RatioSlot{
		{UPC: "UPC-101", TargetQty: 2},
	}, 2.0, 0.1)
	for range 2 {
		if err := session.ScanItemUPC("UPC-101"); err != nil {
			t.Fatalf("packing scan failed: %v", err)
		}
	}
	if err := session.VerifyAndSeal(2.0); err != nil {
		t.Fatalf("packing verification failed: %v", err)
	}
	if !session.IsSealed {
		t.Fatal("verified carton should be sealed")
	}
	if err := session.ScanItemUPC("UPC-101"); err == nil {
		t.Fatal("sealed cartons must reject subsequent scans")
	}
}
