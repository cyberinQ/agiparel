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
            if err != nil { t.Fatal(err) }
            body := string(raw)
            for _, needle := range []string{`/static/workspace_nav.js`, `/static/ux_unified.css`, `data-ag-phase="` + phase + `"`} {
                if !strings.Contains(body, needle) { t.Errorf("missing %q", needle) }
            }
            if token := roadmap.FindString(body); token != "" { t.Errorf("customer page leaks roadmap label %q", token) }
        })
    }
}

func TestPublicHomeShowsWorkAreasNotInventedOrders(t *testing.T) {
    html, err := GetPortalHTML(); if err != nil { t.Fatal(err) }
    js, err := GetPortalJS(); if err != nil { t.Fatal(err) }
    page := string(html)
    for _, route := range []string{"/factory-setup", "/documents", "/buyer-review", "/material-planning", "/stores", "/cutting", "/sewing", "/quality", "/packing", "/intelligence"} {
        if !strings.Contains(page, `href="` + route + `"`) { t.Errorf("home missing %s", route) }
    }
    for _, fake := range []string{"MK6574J9", "GP240812", "PO-4518", "order-select", "metric-evidence"} {
        if strings.Contains(page, fake) || strings.Contains(string(js), fake) { t.Errorf("home must not show fabricated live-looking data %q", fake) }
    }
    if !strings.Contains(page, "Button taps stand in for scans") && !strings.Contains(page, "button tap stands in for a scanner read") {
        t.Fatal("public home must describe simulated reader provenance")
    }
}

func TestWorkAreaNavigationHasOneJourneyAndAdvancedTools(t *testing.T) {
    code, err := staticFS.ReadFile("static/workspace_nav.js"); if err != nil { t.Fatal(err) }
    nav := string(code)
    for _, token := range []string{`'/factory-setup'`, `'/documents'`, `'/buyer-review'`, `'/order-intake'`, `'/material-planning'`, `'/planning'`, `'/plant'`, `'/stores'`, `'/cutting'`, `'/sewing'`, `'/quality'`, `'/packing'`, `'/intelligence'`, `'/onboarding'`, `attachShadow`, `More tools`, `Next →`} {
        if !strings.Contains(nav, token) { t.Errorf("navigation missing %q", token) }
    }
}
