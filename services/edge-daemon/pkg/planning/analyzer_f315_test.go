// Author: Sunil Jain, CyberinQ LLC, 2026-10-29.
package planning

import (
	"github.com/cyberinq/agiparel/services/edge-daemon/pkg/onboarding"
	"strings"
	"testing"
)

func TestF315WithoutBOMCannotInventClothRequirement(t *testing.T) {
	wb := &onboarding.FactoryWorkbook{Styles: []onboarding.StyleRow{{StyleCode: "SHIRT", FabricSKU: "COTTON"}}, StyleParts: []onboarding.StylePartRow{{StyleCode: "SHIRT", MaterialSKU: "COTTON"}}, StoreRolls: []onboarding.StoreRollRow{{FactorySKU: "COTTON", LengthM: 9999, Status: "AVAILABLE"}}}
	card := evaluateCloth(wb, "PO1", "SHIRT", 100)
	if card.Status != LightYellow || !strings.Contains(card.Headline, "UNKNOWN") {
		t.Fatalf("invented requirement: %+v", card)
	}
}
func TestF315BOMMultipleFabricSKUsAndReservations(t *testing.T) {
	wb := &onboarding.FactoryWorkbook{
		BOMRows:    []onboarding.BOMRow{{StyleCode: "SHIRT", MaterialSKU: "SHELL", Category: "FABRIC", QtyPerGarment: 1.8, UOM: "M"}, {StyleCode: "SHIRT", MaterialSKU: "INTERLINING", Category: "FABRIC", QtyPerGarment: .3, UOM: "M"}, {StyleCode: "SHIRT", MaterialSKU: "BUTTON", Category: "TRIM", QtyPerGarment: 6, UOM: "PCS"}},
		StoreRolls: []onboarding.StoreRollRow{{FactorySKU: "SHELL", LengthM: 180, Status: "AVAILABLE"}, {FactorySKU: "INTERLINING", LengthM: 10, Status: "AVAILABLE"}, {FactorySKU: "INTERLINING", LengthM: 100, Status: "RESERVED", ReservedOrder: "OTHER_PO"}},
	}
	card := evaluateCloth(wb, "PO1", "SHIRT", 100)
	if card.Status != LightRed || !strings.Contains(strings.Join(card.Details, " "), "INTERLINING") {
		t.Fatalf("did not detect per-material shortfall: %+v", card)
	}
	wb.StoreRolls[1].LengthM = 30
	card = evaluateCloth(wb, "PO1", "SHIRT", 100)
	if card.Status != LightYellow || strings.Contains(card.Headline, "UNKNOWN") {
		t.Fatalf("workbook supply must be provisional, not green: %+v", card)
	}
}
func TestF315RejectsNonMeterAndInvalidBOM(t *testing.T) {
	for _, b := range []onboarding.BOMRow{{StyleCode: "ST", MaterialSKU: "COTTON", Category: "FABRIC", QtyPerGarment: 1.25, UOM: "KG"}, {StyleCode: "ST", MaterialSKU: "COTTON", Category: "FABRIC", QtyPerGarment: -5, UOM: "M"}} {
		card := evaluateCloth(&onboarding.FactoryWorkbook{BOMRows: []onboarding.BOMRow{b}}, "PO1", "ST", 1)
		if card.Status != LightYellow || !strings.Contains(card.Headline, "UNKNOWN") {
			t.Fatal(card)
		}
	}
}
