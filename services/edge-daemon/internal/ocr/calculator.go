package ocr

import (
	"errors"
	"fmt"
)

var (
	ErrConservationFailed = errors.New("ERR_CONSERVATION_BREACH: Stage inflow does not balance outflow")
	ErrMissingSignOffs    = errors.New("ERR_SIGN_OFF_INCOMPLETE: All 6 department approvals are mandatory")
)

type DepartmentSignOff struct {
	Department string `json:"department"` // Cutting, Sewing, Finishing, Packing, Quality, Accounts
	SignerID   string `json:"signer_id"`
	Signature  string `json:"signature"`
	Timestamp  string `json:"timestamp"`
}

type OrderClosureBalance struct {
	CutQuantity     int                 `json:"cut_quantity"`
	ShippedQuantity int                 `json:"shipped_quantity"`
	TotalLost       int                 `json:"total_lost"`
	TotalLeftOver   int                 `json:"total_left_over"`
	SignOffs        []DepartmentSignOff `json:"sign_offs"`
}

// EvaluateOrderConservation verifies Cut = Shipped + TotalLost + TotalLeftOver and enforces 6 sign-offs
func EvaluateOrderConservation(record OrderClosureBalance) error {
	expectedCut := record.ShippedQuantity + record.TotalLost + record.TotalLeftOver

	if record.CutQuantity != expectedCut {
		return fmt.Errorf("%w: Cut (%d) != Shipped (%d) + Lost (%d) + LeftOver (%d). Delta = %d",
			ErrConservationFailed, record.CutQuantity, record.ShippedQuantity, record.TotalLost, record.TotalLeftOver,
			record.CutQuantity-expectedCut)
	}

	// Validate 6 mandatory department sign-offs
	requiredDepts := map[string]bool{
		"Cutting":   false,
		"Sewing":    false,
		"Finishing": false,
		"Packing":   false,
		"Quality":   false,
		"Accounts":  false,
	}

	for _, so := range record.SignOffs {
		if _, exists := requiredDepts[so.Department]; exists && so.Signature != "" {
			requiredDepts[so.Department] = true
		}
	}

	for dept, signed := range requiredDepts {
		if !signed {
			return fmt.Errorf("%w: Missing sign-off for %s", ErrMissingSignOffs, dept)
		}
	}

	return nil
}