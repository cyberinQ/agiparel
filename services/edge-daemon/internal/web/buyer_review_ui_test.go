package web

import (
	"strings"
	"testing"
)

func TestBuyerReviewNavigationAndSourceDisclosure(t *testing.T) {
	html, err := GetBuyerReviewHTML()
	if err != nil {
		t.Fatal(err)
	}
	for _, needed := range []string{`src="/static/workspace_nav.js"`, `id="po-source"`, `id="bom-source"`, `id="acknowledged"`, `id="review-note"`, "PROPOSED · HOLD", "/documents", "/material-planning"} {
		if !strings.Contains(string(html), needed) {
			t.Errorf("missing %s", needed)
		}
	}
	script, err := staticFS.ReadFile("static/workspace_nav.js")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(script), "'/buyer-review'") {
		t.Fatal("buyer review not in global navigation")
	}
}
