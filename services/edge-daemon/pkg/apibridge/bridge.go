package apibridge

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

var (
	ErrMissingStyleOrQty      = errors.New("ERR_MAPPING_FAILED: Missing mandatory style code or quantity in foreign row")
	ErrMissingSKUDimension    = errors.New("ERR_MISSING_SKU_DIMENSION: color and size codes must be explicitly defined")
	ErrEmptyMatrix            = errors.New("ERR_EMPTY_MATRIX: Grid matrix has no rows or columns")
	ErrMatrixQuantityMismatch = errors.New("ERR_MATRIX_QUANTITY_MISMATCH: Row sum does not match row total")
)

type ImportedRow struct {
	RawAttributes map[string]string `json:"raw_attributes"`
}

type StandardizedSKU struct {
	ItemTypeID string `json:"item_type_id"`
	ColorID    string `json:"color_id"`
	SizeID     string `json:"size_id"`
	Quantity   int    `json:"quantity"`
	UPC        string `json:"upc,omitempty"`
}

type MatrixGrid struct {
	StyleCode string           `json:"style_code"`
	Sizes     []string         `json:"sizes"`
	Rows      []MatrixColorRow `json:"rows"`
}

type MatrixColorRow struct {
	ColorCode     string         `json:"color_code"`
	Quantities    map[string]int `json:"quantities"` // Size -> Qty
	ExpectedTotal int            `json:"expected_total,omitempty"`
}

type APIBridgeEngine struct {
	tenantID string
}

func NewAPIBridgeEngine(tenantID string) *APIBridgeEngine {
	return &APIBridgeEngine{tenantID: tenantID}
}

// MapForeignRow normalizes external flat spreadsheet rows without fabricating missing dimensions.
func (b *APIBridgeEngine) MapForeignRow(row ImportedRow) (StandardizedSKU, error) {
	styleCode := findVal(row.RawAttributes, "style", "style_code", "item", "product")
	colorCode := findVal(row.RawAttributes, "color", "shade", "colour")
	sizeCode := findVal(row.RawAttributes, "size", "dimension", "variant")
	qtyStr := findVal(row.RawAttributes, "qty", "quantity", "order_qty", "pieces")
	upc := findVal(row.RawAttributes, "upc", "ean", "gtin", "barcode")

	if styleCode == "" || qtyStr == "" {
		return StandardizedSKU{}, ErrMissingStyleOrQty
	}

	var qty int
	_, err := fmt.Sscanf(qtyStr, "%d", &qty)
	if err != nil || qty <= 0 {
		return StandardizedSKU{}, fmt.Errorf("ERR_INVALID_QUANTITY: Unable to parse quantity value '%s'", qtyStr)
	}

	if strings.TrimSpace(colorCode) == "" || strings.TrimSpace(sizeCode) == "" {
		return StandardizedSKU{}, ErrMissingSKUDimension
	}

	return StandardizedSKU{
		ItemTypeID: strings.ToUpper(styleCode),
		ColorID:    strings.ToUpper(colorCode),
		SizeID:     strings.ToUpper(sizeCode),
		Quantity:   qty,
		UPC:        strings.TrimSpace(upc),
	}, nil
}

// ParseMatrixGrid decomposes an apparel color-by-size grid into canonical SKU allocations.
func (b *APIBridgeEngine) ParseMatrixGrid(grid MatrixGrid) ([]StandardizedSKU, int, error) {
	if strings.TrimSpace(grid.StyleCode) == "" {
		return nil, 0, errors.New("ERR_MISSING_STYLE: Matrix requires a valid style code")
	}
	if len(grid.Sizes) == 0 || len(grid.Rows) == 0 {
		return nil, 0, ErrEmptyMatrix
	}

	var result []StandardizedSKU
	totalConserved := 0

	for _, row := range grid.Rows {
		if strings.TrimSpace(row.ColorCode) == "" {
			return nil, 0, errors.New("ERR_EMPTY_COLOR: Grid row has no color code")
		}

		rowSum := 0
		for _, size := range grid.Sizes {
			qty := row.Quantities[size]
			if qty < 0 {
				return nil, 0, fmt.Errorf("ERR_NEGATIVE_QUANTITY: Negative quantity at Color %s, Size %s", row.ColorCode, size)
			}
			if qty > 0 {
				rowSum += qty
				totalConserved += qty
				result = append(result, StandardizedSKU{
					ItemTypeID: strings.ToUpper(grid.StyleCode),
					ColorID:    strings.ToUpper(row.ColorCode),
					SizeID:     strings.ToUpper(size),
					Quantity:   qty,
				})
			}
		}

		if row.ExpectedTotal > 0 && rowSum != row.ExpectedTotal {
			return nil, 0, fmt.Errorf("%w for color %s (got %d, expected %d)",
				ErrMatrixQuantityMismatch, row.ColorCode, rowSum, row.ExpectedTotal)
		}
	}

	if totalConserved == 0 {
		return nil, 0, errors.New("ERR_ZERO_QUANTITY: Matrix contains zero total pieces")
	}

	return result, totalConserved, nil
}

// ValidateUPCChecksum verifies standard GS1 UPC-A (12 digits) or EAN-13 (13 digits) check digits.
func ValidateUPCChecksum(barcode string) bool {
	barcode = strings.TrimSpace(barcode)
	n := len(barcode)
	if n != 12 && n != 13 {
		return false
	}

	for _, c := range barcode {
		if c < '0' || c > '9' {
			return false
		}
	}

	digits := make([]int, n)
	for i, c := range barcode {
		digits[i] = int(c - '0')
	}

	sum := 0
	if n == 12 { // UPC-A
		for i := 0; i < 11; i++ {
			if i%2 == 0 {
				sum += digits[i] * 3
			} else {
				sum += digits[i] * 1
			}
		}
	} else { // EAN-13
		for i := 0; i < 12; i++ {
			if i%2 == 0 {
				sum += digits[i] * 1
			} else {
				sum += digits[i] * 3
			}
		}
	}

	checkDigit := (10 - (sum % 10)) % 10
	return checkDigit == digits[n-1]
}

func findVal(m map[string]string, keys ...string) string {
	for _, target := range keys {
		for k, v := range m {
			if strings.EqualFold(strings.TrimSpace(k), target) {
				return strings.TrimSpace(v)
			}
		}
	}

	sortedKeys := make([]string, 0, len(m))
	for k := range m {
		sortedKeys = append(sortedKeys, k)
	}
	sort.Strings(sortedKeys)

	for _, target := range keys {
		for _, k := range sortedKeys {
			if strings.Contains(strings.ToLower(k), target) {
				return strings.TrimSpace(m[k])
			}
		}
	}
	return ""
}
