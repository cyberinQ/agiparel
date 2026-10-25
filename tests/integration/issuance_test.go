package integration

import (
	"testing"
	"github.com/cyberinq/agiparel/services/edge-daemon/pkg/stores"
)

func TestDoubleWitnessIssuanceCheckout(t *testing.T) {
	engine := stores.NewStoresEngine()

	// Register Pilot Roll #7 (Reserved for PO MK6574J9, Cut 1, 59.0 meters)
	engine.RegisterRoll(stores.RollStock{
		RollID:          "ROLL-007",
		OriginalLengthM: 59.0,
		RemainingLength: 59.0,
		ReservedOrderID: "MK6574J9",
		ReservedCutNo:   1,
		Status:          "RESERVED",
	})

	// Execute Double-Witness Checkout (<20s SLA simulation)
	record, err := engine.DoubleWitnessIssue(
		"ROLL-007",
		"MK6574J9",
		1,
		59.0,
		"STORE_USER_01",
		"CUT_HELPER_01",
		"SECRET_PLANT_KEY",
	)
	if err != nil {
		t.Fatalf("Double witness issuance failed: %v", err)
	}

	if record.RemainingLengthM != 0.0 {
		t.Errorf("Expected remaining length to be 0.0, got %.2f", record.RemainingLengthM)
	}

	if record.DigitalSigHash == "" {
		t.Errorf("Expected cryptographic signature hash on issuance record")
	}
}