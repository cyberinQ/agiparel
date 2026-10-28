// Author: Sunil Jain, 2026-10-28 — The Home chapter menu is not duplicated in chapter headers.
package web

import (
	"regexp"
	"strings"
	"testing"
)

// Keep every customer route in the same work-area navigation, without
// rewriting any of the existing business forms or scanner event handlers.
func TestCustomerPagesShareWorkAreaShell(t *testing.T) {
	pages := map[string]string{
		"factory_setup.html": "factory", "documents.html": "documents", "buyer_review.html": "buyer",
		"order_intake.html": "buyer", "material_planning.html": "material", "stores.html": "stores",
		"cutting_view.html": "cutting", "sewing_execution.html": "sewing", "quality_execution.html": "quality",
		"packing_execution.html": "packing", "factory_intelligence.html": "reports",
		"plant_view.html": "reports", "onboarding.html": "factory", "sew01.html": "sewing",
	}
	// This finds internal roadmap references, not the legitimate Step 1..10 numbers.
	roadmap := regexp.MustCompile(`(?i)\b(?:batch|gate|epic)\s*0?[4-9](?:\.[0-9]+)?(?:[- ]?[A-D][0-9]?)?\b|\b(?:batch|gate|epic)\s*1[0-9](?:[- ]?[A-D][0-9]?)?\b`)
	for file, phase := range pages {
		t.Run(file, func(t *testing.T) {
			raw, err := staticFS.ReadFile("static/" + file)
			if err != nil {
				t.Fatal(err)
			}
			body := string(raw)
			for _, needle := range []string{`/static/workspace_nav.js`, `/static/ux_unified.css`, `data-ag-phase="` + phase + `"`} {
				if !strings.Contains(body, needle) {
					t.Errorf("missing %q", needle)
				}
			}
			if token := roadmap.FindString(body); token != "" {
				t.Errorf("customer page leaks roadmap label %q", token)
			}
		})
	}
}

func TestPublicHomeShowsWorkAreasNotInventedOrders(t *testing.T) {
	html, err := GetPortalHTML()
	if err != nil {
		t.Fatal(err)
	}
	js, err := GetPortalJS()
	if err != nil {
		t.Fatal(err)
	}
	page := string(html)
	for _, route := range []string{"/factory-setup", "/documents", "/buyer-review", "/material-planning", "/stores", "/cutting", "/sewing", "/quality", "/packing", "/intelligence"} {
		if !strings.Contains(page, `href="`+route+`"`) {
			t.Errorf("home missing %s", route)
		}
	}
	for _, fake := range []string{"MK6574J9", "GP240812", "PO-4518", "order-select", "metric-evidence"} {
		if strings.Contains(page, fake) || strings.Contains(string(js), fake) {
			t.Errorf("home must not show fabricated live-looking data %q", fake)
		}
	}
	// Home is a chapter index. Scan provenance belongs where scanning actually occurs.
	for _, removed := range []string{"Work areas require sign-in", "Open area", "Practice now. Connect scanners later.", "CHOOSE YOUR WORK AREA", "01 <span>→</span> 10"} {
		if strings.Contains(page, removed) {
			t.Errorf("home still contains removed clutter %q", removed)
		}
	}
	if !strings.Contains(page, "Every garment") || !strings.Contains(page, "tells a story.") || !strings.Contains(page, "CHAPTERS") || !strings.Contains(page, "Empowered by CyberinQ.") {
		t.Error("home must contain the requested heading, chapters label and footer")
	}
	ordered := []string{"/factory-setup", "/buyer-review", "/material-planning", "/stores", "/cutting", "/sewing", "/quality", "/packing", "/intelligence", "/documents"}
	previous := -1
	for _, route := range ordered {
		pos := strings.Index(page, `href="`+route+`"`)
		if pos < previous {
			t.Errorf("chapter %s is out of order", route)
		}
		previous = pos
	}
}

func TestWorkAreaNavigationHasOneJourneyAndAdvancedTools(t *testing.T) {
	code, err := staticFS.ReadFile("static/workspace_nav.js")
	if err != nil {
		t.Fatal(err)
	}
	nav := string(code)
	for _, token := range []string{`'/factory-setup'`, `'/documents'`, `'/buyer-review'`, `'/order-intake'`, `'/material-planning'`, `'/planning'`, `'/plant'`, `'/stores'`, `'/cutting'`, `'/sewing'`, `'/quality'`, `'/packing'`, `'/intelligence'`, `'/onboarding'`, `ag-chapter-bar`, `aggie-open`, `workspace_draft.js`} {
		if !strings.Contains(nav, token) {
			t.Errorf("chapter shell missing %q", token)
		}
	}
}
