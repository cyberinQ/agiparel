package stores

import (
	"errors"
	"fmt"
)

var ErrGrossIndentBlocked = errors.New("ERR_GROSS_INDENT_BLOCKED: Unreserved surplus exists; net calculation enforced")

type IndentRequest struct {
	SKU                 string  `json:"sku"`
	BOMRequiredQty      float64 `json:"bom_required_qty"`
	AvailableUnreserved float64 `json:"available_unreserved"`
	RequestedIndentQty  float64 `json:"requested_indent_qty"`
	ExecutiveOverride   string  `json:"executive_override_code,omitempty"`
}

type IndentResponse struct {
	SKU             string  `json:"sku"`
	BOMRequiredQty  float64 `json:"bom_required_qty"`
	DeductedSurplus float64 `json:"deducted_surplus"`
	NetIndentQty    float64 `json:"net_indent_qty"`
	Status          string  `json:"status"`
}

// EnforcePrePOGate enforces NetIndent = BOMRequired - AvailableUnreserved
func EnforcePrePOGate(req IndentRequest) (*IndentResponse, error) {
	if req.BOMRequiredQty <= 0 {
		return nil, errors.New("BOM requirement must be greater than zero")
	}

	netRequired := req.BOMRequiredQty - req.AvailableUnreserved
	if netRequired < 0 {
		netRequired = 0
	}

	// If gross purchase is requested without override when stock exists, block transaction
	if req.RequestedIndentQty > netRequired && req.ExecutiveOverride == "" {
		return nil, fmt.Errorf("%w: Max allowed purchase is %.2f, requested %.2f",
			ErrGrossIndentBlocked, netRequired, req.RequestedIndentQty)
	}

	return &IndentResponse{
		SKU:             req.SKU,
		BOMRequiredQty:  req.BOMRequiredQty,
		DeductedSurplus: req.AvailableUnreserved,
		NetIndentQty:    netRequired,
		Status:          "APPROVED_FOR_PO",
	}, nil
}