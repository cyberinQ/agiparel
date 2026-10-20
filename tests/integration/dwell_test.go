package integration

import (
	"testing"
	"time"
	"github.com/cyberinq/agiparel/services/edge-daemon/pkg/sewing"
)

func TestAntiGhostTapDwellGuard(t *testing.T) {
	now := time.Now()
	// Factory SAM = 0.60 mins -> 36 seconds total -> 40% = 14.4 seconds minimum dwell
	sub := sewing.CycleSubmission{
		StationID:      "LINE14_ST01",
		OperatorID:     "WORKER_A814",
		OperationID:    "OP_COLLAR_ATTACH",
		FactorySAMMin:  0.60,
		CycleStartedAt: now,
		SubmittedAt:    now.Add(8 * time.Second), // Rapid tap at 8 seconds (should fail)
	}

	result, err := sewing.ValidateCycleDwell(sub)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if result.IsValid {
		t.Errorf("Expected cycle submission to be rejected by anti-ghost tap guard")
	}
}