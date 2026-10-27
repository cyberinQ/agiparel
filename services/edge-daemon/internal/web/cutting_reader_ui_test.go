package web

import (
	"regexp"
	"strings"
	"testing"
)

func TestCuttingReaderEvidenceVisibleAndNoReleaseShortcut(t *testing.T) {
	b, e := GetCuttingViewHTML()
	if e != nil {
		t.Fatal(e)
	}
	page := string(b)
	for _, token := range []string{"/static/cutting_release_status.js", "cut-release-workbench", "cut-release-evidence", "release-refresh", "NOT RELEASED", "PROPOSED / HOLD", "Hardware RF presence", "signed-issue reconciliation"} {
		if !strings.Contains(page, token) {
			t.Errorf("missing visible cutting safeguard %q", token)
		}
	}
	js, e := staticFS.ReadFile("static/cutting_release_status.js")
	if e != nil {
		t.Fatal(e)
	}
	for _, token := range []string{"/api/v1/cutting/release-status", "gateway_reported_bundles", "matched_bundles", "cut-draft-selected", "Basic development credentials"} {
		if !strings.Contains(string(js), token) && !strings.Contains(page, token) {
			t.Errorf("missing read-only status check %q", token)
		}
	}
	// This test covers public customer page copy, NOT internal engineering docs.
	internal := regexp.MustCompile(`(?i)\b(batch|gate)\s*0[4-9](?:\.[0-9]+)?(?:-[A-D][0-9]?)?\b`)
	for _, name := range []string{"cutting_view.html", "stores.html", "documents.html", "factory_setup.html", "buyer_review.html", "material_planning.html", "order_intake.html", "onboarding.html", "portal.html"} {
		data, e := staticFS.ReadFile("static/" + name)
		if e != nil {
			t.Fatal(e)
		}
		if match := internal.Find(data); match != nil {
			t.Errorf("customer page %s contains internal roadmap label %q", name, string(match))
		}
	}
}
