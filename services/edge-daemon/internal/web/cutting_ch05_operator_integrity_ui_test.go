// Author: Sunil Jain, 2026-10-28
// Chapter 05 operator controls and read-only, evidence-preserving safeguards.
package web

import (
	"strings"
	"testing"
)

func TestChapter05CuttingSourceAndEvidenceControls(t *testing.T) {
	html, err := staticFS.ReadFile("static/cutting_view.html")
	if err != nil {
		t.Fatal(err)
	}
	page := string(html)
	for _, token := range []string{"revise-cut", "cut-register-scope", "cut-planned-consumption", "spread-check", "NOT RELEASED", "PROPOSED / HOLD"} {
		if !strings.Contains(page, token) {
			t.Errorf("missing Chapter 05 operator control %q", token)
		}
	}
	checks := map[string][]string{
		"static/cutting_lay.js":            {"cut-draft-cleared", "cutPrepareRevision", "cutPendingSignature", "draft-planned", "data.has_more"},
		"static/cutting_spread.js":         {"cut-draft-cleared", "spreadReadInput", "spreadPendingSignature", "available_for_attribution_mm", "spreadCheck"},
		"static/cutting_release_status.js": {"cutReleaseGeneration", "cut-draft-cleared", "gateway_reported_bundles", "matched_bundles"},
	}
	for file, tokens := range checks {
		data, err := staticFS.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		for _, token := range tokens {
			if !strings.Contains(string(data), token) {
				t.Errorf("%s: missing %q", file, token)
			}
		}
	}
}
