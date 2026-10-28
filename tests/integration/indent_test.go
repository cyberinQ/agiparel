package integration

import (
	"github.com/cyberinq/agiparel/services/edge-daemon/pkg/stores"
	"testing"
)

func TestPrePOSurplusGate(t *testing.T) {
	// Ground truth test case: BOM required = 2608 yds, Unreserved stock = 665 yds -> Net = 1943 yds
	req := stores.IndentRequest{
		SKU:            "POCKETING-TWILL-WHITE",
		BOMRequiredQty: 2608.0,
		UnreservedQty:  665.0,
	}

	result, err := stores.EvaluateIndent(req)
	if err != nil {
		t.Fatalf("Failed to evaluate indent: %v", err)
	}

	expectedNet := 1943.0
	if result.NetIndentQty != expectedNet {
		t.Errorf("Expected net indent %.2f, got %.2f", expectedNet, result.NetIndentQty)
	}

	if !result.RequiresOverride {
		t.Errorf("Expected RequiresOverride to be true when unreserved stock offsets BOM requirements")
	}
}
