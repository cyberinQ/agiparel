package integration

import (
	"github.com/cyberinq/agiparel/services/edge-daemon/pkg/docscribe"
	"testing"
)

func TestDocScribeGeneration(t *testing.T) {
	scribe := docscribe.NewDocScribeEngine("TENANT_NOBLE_OKHLA")

	manual, err := scribe.GeneratePlainEnglishManual("LINE14_ST04", "ATTACH_BACK_RISE", "HI")
	if err != nil {
		t.Fatalf("Failed to generate manual: %v", err)
	}

	if manual.ReadingLevel != "5TH_GRADE" || manual.Language != "HI" || len(manual.Steps) != 4 {
		t.Errorf("Generated manual mismatch: %+v", manual)
	}
}
