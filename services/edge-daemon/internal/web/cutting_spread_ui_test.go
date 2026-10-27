package web

import (
	"strings"
	"testing"
)

func TestCuttingSpreadWorkbenchBatch08B(t *testing.T) {
	html, e := GetCuttingViewHTML()
	if e != nil {
		t.Fatal(e)
	}
	for _, token := range []string{"PROPOSED / HOLD", "NOT RELEASED", "/static/cutting_spread.js", "cut-spread-workbench", "signed-issue reconciliation", "does not decrement stores", "Hardware RF presence"} {
		if !strings.Contains(string(html), token) {
			t.Errorf("missing 08B safeguard %q", token)
		}
	}
	js, e := staticFS.ReadFile("static/cutting_spread.js")
	if e != nil {
		t.Fatal(e)
	}
	for _, token := range []string{"/api/v1/cutting/spreads", "cut-draft-selected", "matching_signed_issues", "signed_bundle_witnesses"} {
		if !strings.Contains(string(js), token) {
			t.Errorf("08B script missing %q", token)
		}
	}
}
