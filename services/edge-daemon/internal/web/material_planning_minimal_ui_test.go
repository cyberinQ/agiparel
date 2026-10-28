package web

// Author: Sunil Jain, 2026-10-27 — Preserve Chapter 03 controls, evidence safeguards, and a minimal work area.
import (
	"strings"
	"testing"
)

func TestMaterialPlanningMinimalEvidenceAwareUI(t *testing.T) {
	raw, err := staticFS.ReadFile("static/material_planning.html")
	if err != nil {
		t.Fatal(err)
	}
	html := string(raw)
	for _, required := range []string{
		`data-ag-phase="material"`, `data-ag-simple-header="true"`,
		`src="/static/workspace_nav.js"`, `src="/static/portal.js"`,
		`id="aggie-open"`, `id="aggie-chat"`, `href="/"`,
		`03 · PLAN MATERIAL`, `Plan your materials.`, `id="intake"`,
		`id="need-state"`, `id="have-state"`, `id="short-state"`,
		`id="material-lines"`, `id="scenario-form"`, `id="stock"`, `id="prices"`,
		`id="preview"`, `id="stage"`, `id="results"`, `id="candidates"`,
		`id="holds"`, `id="hash"`, `id="history"`, `id="refresh"`,
		`href="/documents"`, `.csv`, `Not checked`, `Unknown`, `DRAFT · HOLD`,
	} {
		if !strings.Contains(html, required) {
			t.Errorf("missing required contract %q", required)
		}
	}
	for _, forbidden := range []string{`EXAMPLE-LOT-`, `ACCEPTED_PASS`, `4.2500`, `EXAMPLE-QUOTE-REPLACE`, `More tools`} {
		if strings.Contains(html, forbidden) {
			t.Errorf("clutter or fabricated example visible: %q", forbidden)
		}
	}
	js, err := staticFS.ReadFile("static/material_planning.js")
	if err != nil {
		t.Fatal(err)
	}
	for _, necessary := range []string{`/api/v1/order-intake`, `/api/v1/material-scenario`, `new FormData()`, `preview`, `stage`, `Not checked`, `Unknown`} {
		if !strings.Contains(string(js), necessary) {
			t.Errorf("missing workflow %q", necessary)
		}
	}
	if strings.Contains(string(js), `EXAMPLE-GRN-REPLACE`) || strings.Contains(string(js), `EXAMPLE-LOT-`) {
		t.Fatal("template must not invent available stock")
	}
	css, err := staticFS.ReadFile("static/material_planning.css")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(css), `#7759B2`) || !strings.Contains(string(css), `@media(max-width:580px)`) {
		t.Fatal("wrong chapter accent or missing mobile layout")
	}
}
