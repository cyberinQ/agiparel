package integration

import (
	"github.com/cyberinq/agiparel/services/edge-daemon/pkg/logicweaver"
	"testing"
)

func TestLogicWeaverCompilation(t *testing.T) {
	weaver := logicweaver.NewLogicWeaverEngine("TENANT_NOBLE_OKHLA")

	rule, err := weaver.CompileRule(
		"RULE-DAG-01",
		"ATTACHMENT",
		"Prevent left pocket attaching to right front panel on Style KK524867",
		"KK524867",
		101, // Source Part ID
		102, // Target Part ID
	)
	if err != nil {
		t.Fatalf("Failed to compile rule: %v", err)
	}

	if rule.Category != "ATTACHMENT" || !rule.IsActive || rule.ParamA != 101 {
		t.Errorf("Compiled rule payload mismatch: %+v", rule)
	}
}
