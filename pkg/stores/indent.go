package stores

import (
	"errors"
)

type IndentRequest struct {
	SKU            string  `json:"sku"`
	BOMRequiredQty float64 `json:"bom_required_qty"`
	UnreservedQty  float64 `json:"unreserved_qty"`
}

type IndentResult struct {
	SKU              string  `json:"sku"`
	BOMRequiredQty   float64 `json:"bom_required_qty"`
	UnreservedQty    float64 `json:"unreserved_qty"`
	NetIndentQty     float64 `json:"net_indent_qty"`
	RequiresOverride bool    `json:"requires_override"`
}

// EvaluateIndent calculates net purchase indents against existing unreserved stock.
func EvaluateIndent(req IndentRequest) (IndentResult, error) {
	if req.BOMRequiredQty < 0 || req.UnreservedQty < 0 {
		return IndentResult{}, errors.New("ERR_INVALID_QUANTITY: Quantities cannot be negative")
	}

	net := req.BOMRequiredQty - req.UnreservedQty
	if net < 0 {
		net = 0
	}

	return IndentResult{
		SKU:              req.SKU,
		BOMRequiredQty:   req.BOMRequiredQty,
		UnreservedQty:    req.UnreservedQty,
		NetIndentQty:     net,
		RequiresOverride: req.UnreservedQty > 0 && net < req.BOMRequiredQty,
	}, nil
}
