package sewing

import (
	"errors"
	"fmt"
	"time"
)

type CycleSubmission struct {
	StationID   string    `json:"station_id"`
	OperatorID  string    `json:"operator_id"`
	OperationID string    `json:"operation_id"`
	FactorySAMMin float64 `json:"factory_sam_min"` // Standard Allowed Minutes
	CycleStartedAt time.Time `json:"cycle_started_at"`
	SubmittedAt    time.Time `json:"submitted_at"`
}

type DwellValidationResult struct {
	IsValid     bool          `json:"is_valid"`
	ElapsedTime time.Duration `json:"elapsed_time"`
	MinRequired time.Duration `json:"min_required"`
	Reason      string        `json:"reason"`
}

// ValidateCycleDwell checks if an operator's CYCLE DONE tap honors the empirical dwell floor (40% of SAM).
func ValidateCycleDwell(sub CycleSubmission) (DwellValidationResult, error) {
	if sub.FactorySAMMin <= 0 {
		return DwellValidationResult{}, errors.New("ERR_INVALID_SAM: Factory SAM must be greater than zero")
	}

	elapsed := sub.SubmittedAt.Sub(sub.CycleStartedAt)
	// Min dwell floor: 40% of SAM converted to seconds
	minRequiredSeconds := sub.FactorySAMMin * 60.0 * 0.40
	minRequired := time.Duration(minRequiredSeconds * float64(time.Second))

	if elapsed < minRequired {
		return DwellValidationResult{
			IsValid:     false,
			ElapsedTime: elapsed,
			MinRequired: minRequired,
			Reason:      fmt.Sprintf("ERR_GHOST_TAP: Elapsed %.2fs is below minimum dwell floor of %.2fs (40%% of SAM)", elapsed.Seconds(), minRequired.Seconds()),
		}, nil
	}

	return DwellValidationResult{
			IsValid:     true,
			ElapsedTime: elapsed,
			MinRequired: minRequired,
			Reason:      "PASS",
	}, nil
}