package main

import (
	"encoding/json"
	"testing"
)

func hintSeg(ord int, kind, content string, scope any) factoryEvidenceSegment {
	loc, _ := json.Marshal(scope)
	return factoryEvidenceSegment{Ordinal: ord, Kind: kind, Content: content, Locator: loc}
}
func TestDocumentTriageFromEvidenceAndNotFilename(t *testing.T) {
	cases := []struct {
		name     string
		segments []factoryEvidenceSegment
		want     string
	}{
		{"factory machine headers", []factoryEvidenceSegment{hintSeg(1, "ROW", `["machine_id","machine_type","status"]`, map[string]int{"row": 1})}, "MACHINE_REGISTER"},
		{"purchase order", []factoryEvidenceSegment{hintSeg(1, "PAGE", "Purchase Order # 7751 Style 8803 Ship Date", map[string]int{"page": 1})}, "BUYER_ORDER"},
		{"techpack", []factoryEvidenceSegment{hintSeg(1, "PAGE", "TECH PACK / SIZE SPEC / Measurement chart for style 876", map[string]int{"page": 1})}, "TECH_PACK"},
		{"inventory", []factoryEvidenceSegment{hintSeg(1, "SHEET_ROW", "A1: Material Code | B1: Stock Qty | C1: UOM", map[string]any{"sheet": "Stock", "row": 1})}, "FACTORY_INVENTORY"},
		{"adjacent PDF", []factoryEvidenceSegment{hintSeg(1, "PAGE", "Employee ID list", map[string]int{"page": 1}), hintSeg(2, "PAGE", "Skill and proficiency assessment", map[string]int{"page": 1})}, "SKILL_MATRIX"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out := routeDocumentEvidence(tc.segments)
			found := false
			for _, h := range out {
				if h.Category == tc.want && h.SourceOrdinal > 0 && h.Confidence == "POSSIBLE" {
					found = true
				}
			}
			if !found {
				t.Fatalf("missing %s in %+v", tc.want, out)
			}
		})
	}
}
func TestDocumentTriageNoUnsafeCrossPageLeak(t *testing.T) {
	out := routeDocumentEvidence([]factoryEvidenceSegment{hintSeg(1, "PAGE", "Machine ID", map[string]int{"page": 1}), hintSeg(2, "PAGE", "Machine Type", map[string]int{"page": 2})})
	if len(out) != 0 {
		t.Fatalf("cross-page context leaked %+v", out)
	}
	out = routeDocumentEvidence([]factoryEvidenceSegment{hintSeg(1, "PAGE", "Maybe a machine?", map[string]int{"page": 1})})
	if len(out) != 0 {
		t.Fatalf("guess from no signal %+v", out)
	}
}
