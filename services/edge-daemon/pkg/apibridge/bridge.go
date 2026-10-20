package apibridge

import (
	"errors"
	"fmt"
	"strings"
)

type ImportedRow struct {
	RawAttributes map[string]string `json:"raw_attributes"`
}

type StandardizedSKU struct {
	ItemTypeID string `json:"item_type_id"`
	ColorID    string `json:"color_id"`
	SizeID     string `json:"size_id"`
	Quantity   int    `json:"quantity"`
}

type APIBridgeEngine struct {
	tenantID string
}

func NewAPIBridgeEngine(tenantID string) *APIBridgeEngine {
	return &APIBridgeEngine{tenantID: tenantID}
}

// MapForeignRow normalizes external spreadsheet headers into Agiparel's 3-tier SKU hierarchy.
func (b *APIBridgeEngine) MapForeignRow(row ImportedRow) (StandardizedSKU, error) {
	styleCode := findVal(row.RawAttributes, "style", "style_code", "item", "product")
	colorCode := findVal(row.RawAttributes, "color", "shade", "colour")
	sizeCode := findVal(row.RawAttributes, "size", "dimension", "variant")
	qtyStr := findVal(row.RawAttributes, "qty", "quantity", "order_qty", "pieces")

	if styleCode == "" || qtyStr == "" {
		return StandardizedSKU{}, errors.New("ERR_MAPPING_FAILED: Missing mandatory style code or quantity in foreign row")
	}

	var qty int
	_, err := fmt.Sscanf(qtyStr, "%d", &qty)
	if err != nil || qty <= 0 {
		return StandardizedSKU{}, fmt.Errorf("ERR_INVALID_QUANTITY: Unable to parse quantity value '%s'", qtyStr)
	}

	if colorCode == "" {
		colorCode = "DEFAULT"
	}
	if sizeCode == "" {
		sizeCode = "DEFAULT"
	}

	return StandardizedSKU{
		ItemTypeID: strings.ToUpper(styleCode),
		ColorID:    strings.ToUpper(colorCode),
		SizeID:     strings.ToUpper(sizeCode),
		Quantity:   qty,
	}, nil
}

func findVal(m map[string]string, keys ...string) string {
	for k, v := range m {
		kLower := strings.ToLower(k)
		for _, target := range keys {
			if kLower == target || strings.Contains(kLower, target) {
				return v
			}
		}
	}
	return ""
}