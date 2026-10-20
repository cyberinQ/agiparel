package integration

import (
	"testing"
	"github.com/cyberinq/agiparel/services/edge-daemon/pkg/cutting"
)

func TestCutPlanBufferCalculation(t *testing.T) {
	target, err := cutting.CalculateCutTarget("MK6574J9", "29/30", 600)
	if err != nil {
		t.Fatalf("Failed to calculate cut target: %v", err)
	}

	if target.ClampedExtra != 12 {
		t.Errorf("Expected clamped extra to be 12, got %d", target.ClampedExtra)
	}

	if target.TargetCutVolume != 612 {
		t.Errorf("Expected target cut volume to be 612, got %d", target.TargetCutVolume)
	}

	if target.MaxAllowedLays != 210 {
		t.Errorf("Expected max allowed lays to be 210, got %d", target.MaxAllowedLays)
	}
}