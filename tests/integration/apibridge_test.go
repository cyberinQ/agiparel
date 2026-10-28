package integration

import (
	"github.com/cyberinq/agiparel/services/edge-daemon/pkg/apibridge"
	"testing"
)

func TestAPIBridgeMapping(t *testing.T) {
	bridge := apibridge.NewAPIBridgeEngine("TENANT_NOBLE_OKHLA")

	row := apibridge.ImportedRow{
		RawAttributes: map[string]string{
			"Style Code":   "KK524867",
			"Shade":        "INDIGO",
			"Size Variant": "29/30",
			"Order Qty":    "600",
		},
	}

	sku, err := bridge.MapForeignRow(row)
	if err != nil {
		t.Fatalf("Failed to map foreign spreadsheet row: %v", err)
	}

	if sku.ItemTypeID != "KK524867" || sku.ColorID != "INDIGO" || sku.SizeID != "29/30" || sku.Quantity != 600 {
		t.Errorf("Mapped SKU mismatch: %+v", sku)
	}
}
