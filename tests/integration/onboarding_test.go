package integration

import (
	"strings"
	"testing"

	"github.com/cyberinq/agiparel/services/edge-daemon/pkg/onboarding"
)

func TestPreBoardingWorkbookTemplateAndParsing(t *testing.T) {
	csvTemplate := onboarding.GenerateTemplateCSV()
	if csvTemplate == "" {
		t.Fatal("Expected non-empty pre-boarding CSV template")
	}

	wb, err := onboarding.ParseWorkbookCSV(strings.NewReader(csvTemplate))
	if err != nil {
		t.Fatalf("Failed to parse generated pre-boarding template CSV: %v", err)
	}

	if wb.FactoryName != "Noble Garments" || wb.UnitCode != "OKHLA Unit 002/02" {
		t.Errorf("Unexpected factory profile: %+v", wb)
	}
	if len(wb.Lines) != 2 {
		t.Errorf("Expected 2 lines in template, got %d", len(wb.Lines))
	}
	if len(wb.Machines) != 5 {
		t.Errorf("Expected 5 machines in template, got %d", len(wb.Machines))
	}
	if len(wb.Workers) != 5 {
		t.Errorf("Expected 5 workers in template, got %d", len(wb.Workers))
	}
	if len(wb.StyleParts) != 4 || len(wb.OperationsDAG) != 3 {
		t.Errorf("Expected 4 style parts and 3 DAG operations, got %d parts and %d ops", len(wb.StyleParts), len(wb.OperationsDAG))
	}
}
