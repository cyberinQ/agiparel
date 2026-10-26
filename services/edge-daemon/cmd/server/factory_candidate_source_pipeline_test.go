package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/cyberinq/agiparel/services/document-intake/extract"
)

// Runs the actual universal document extractor over 5 source files, then the
// B2 proposal engine. Checks the UI demo's honest expected candidate counts.
func TestFactorySampleFilesExtractorToReviewCandidates(t *testing.T) {
	cases := []struct {
		filename, entity string
		expected         int
	}{
		{"01_machine_register.csv", "MACHINE", 3},
		{"02_skills.csv", "SKILL", 3},
		{"03_opening_stock.csv", "MATERIAL", 2},
		{"04_factory_rates.csv", "RATE", 2},
		{"05_factory_profile.txt", "FIELD", 4},
	}
	for _, tc := range cases {
		t.Run(tc.filename, func(t *testing.T) {
			path := filepath.Join("..", "..", "..", "..", "examples", "factory-onboarding", tc.filename)
			original, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			result := extract.Process(context.Background(), tc.filename, original)
			if result.Status != "EXTRACTED" {
				t.Fatalf("unexpected extractor status: %s %s", result.Status, result.Explanation)
			}
			segments := make([]factoryEvidenceSegment, len(result.Segments))
			for i, s := range result.Segments {
				segments[i] = factoryEvidenceSegment{Ordinal: s.Ordinal, Kind: s.Kind, Locator: json.RawMessage(s.Locator), Content: s.Content}
			}
			got := factoryProposalsFromSegments("DOC-0123456789abcdef0123456789abcdef", tc.filename, segments)
			if len(got) != tc.expected {
				t.Fatalf("source-to-candidate count got %d want %d; segments=%+v, candidates=%+v", len(got), tc.expected, segments, got)
			}
			for _, p := range got {
				if p.EntityType != tc.entity {
					t.Fatalf("incorrect entity class %+v", p)
				}
			}
		})
	}
}
