// Author: Sunil Jain, CyberinQ LLC
package ocr

import (
	"errors"
	"fmt"
)

type StageBreakdown struct {
	CutQty      int `json:"cut_qty"`
	ShippedQty  int `json:"shipped_qty"`
	LostQty     int `json:"lost_qty"`
	LeftOverQty int `json:"left_over_qty"`
}

type ReconciliationReport struct {
	OrderID         string          `json:"order_id"`
	CutNo           int             `json:"cut_no"`
	Breakdown       StageBreakdown  `json:"breakdown"`
	Variance        int             `json:"variance"`
	IsConserved     bool            `json:"is_conserved"`
	VarianceComment string          `json:"variance_comment,omitempty"`
	Signatures      map[string]bool `json:"signatures"`
}

var RequiredDepartments = []string{
	"MERCHANDISING",
	"INDUSTRIAL_ENG",
	"SOURCING",
	"PRODUCTION",
	"QUALITY",
	"FINANCE",
}

func EvaluateConservation(orderID string, cutNo int, b StageBreakdown, comment string) (*ReconciliationReport, error) {
	if b.CutQty <= 0 {
		return nil, errors.New("ERR_INVALID_CUT: Cut quantity must be greater than zero")
	}

	accountedFor := b.ShippedQty + b.LostQty + b.LeftOverQty
	variance := b.CutQty - accountedFor

	report := &ReconciliationReport{
		OrderID:         orderID,
		CutNo:           cutNo,
		Breakdown:       b,
		Variance:        variance,
		IsConserved:     variance == 0,
		VarianceComment: comment,
		Signatures:      make(map[string]bool),
	}

	for _, dept := range RequiredDepartments {
		report.Signatures[dept] = false
	}

	if variance != 0 && comment == "" {
		return report, fmt.Errorf("ERR_VARIANCE_UNEXPLAINED: Cut (%d) != Accounted (%d). Variance of %d requires mandatory comment",
			b.CutQty, accountedFor, variance)
	}

	return report, nil
}

func (r *ReconciliationReport) SignOff(dept string) error {
	if _, exists := r.Signatures[dept]; !exists {
		return fmt.Errorf("ERR_INVALID_DEPT: Unknown sign-off department %s", dept)
	}
	r.Signatures[dept] = true
	return nil
}

func (r *ReconciliationReport) CanSeal() bool {
	if !r.IsConserved && r.VarianceComment == "" {
		return false
	}
	if len(r.Signatures) < len(RequiredDepartments) {
		return false
	}
	for _, dept := range RequiredDepartments {
		if signed, ok := r.Signatures[dept]; !ok || !signed {
			return false
		}
	}
	return true
}

// AQLSampleCheck calculates Normal Inspection Level II sample size and maximum
// allowable defects under standard garment export AQL 2.5.
func AQLSampleCheck(lotSize int, majorDefects, minorDefects int) (sampleSize int, maxMajor int, maxMinor int, passed bool) {
	switch {
	case lotSize <= 150:
		sampleSize, maxMajor, maxMinor = 20, 1, 2
	case lotSize <= 500:
		sampleSize, maxMajor, maxMinor = 50, 3, 5
	case lotSize <= 1200:
		sampleSize, maxMajor, maxMinor = 80, 5, 7
	case lotSize <= 3200:
		sampleSize, maxMajor, maxMinor = 125, 7, 10
	case lotSize <= 10000:
		sampleSize, maxMajor, maxMinor = 200, 10, 14
	case lotSize <= 35000:
		sampleSize, maxMajor, maxMinor = 315, 14, 21
	default:
		sampleSize, maxMajor, maxMinor = 500, 21, 21
	}
	passed = majorDefects <= maxMajor && minorDefects <= maxMinor
	return sampleSize, maxMajor, maxMinor, passed
}
