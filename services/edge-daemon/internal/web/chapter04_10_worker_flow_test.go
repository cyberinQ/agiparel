// Author: Sunil Jain, 2026-10-28
// Regression contract for the simple ten-chapter application: actionable flows,
// preserved operational controls, and an unchanged shared header and Aggie.
package web

import (
	"strings"
	"testing"
)

func TestRemainingChaptersUseProgressiveWorkerFlow(t *testing.T) {
	pages := []struct {
		file, phase, chapter string
		critical             []string
	}{
		{"stores.html", "stores", "04 · FABRIC STORE", []string{"passport-form", "roll-list", "pick-form", "reconciliation-form", "admission-queue", "secure-reservations"}},
		{"cutting_view.html", "cutting", "05 · CUTTING", []string{"cut-form", "cut-register", "cut-bundles", "spread-form", "cut-release-evidence", "cut-source-facts"}},
		{"sewing_execution.html", "sewing", "06 · SEWING", []string{"lay", "group", "op", "scanButtons", "claim", "complete", "cycles", "feedback"}},
		{"quality_execution.html", "quality", "07 · QUALITY", []string{"serial", "pass", "fail", "repair", "repass", "refail", "finish", "bind", "units"}},
		{"packing_execution.html", "packing", "08 · PACKING", []string{"serial", "carton", "manifest", "cartons", "manifests", "feedback"}},
		{"factory_intelligence.html", "reports", "09 · FACTORY REPORT", []string{"lay", "timezone", "refresh", "capture", "stages", "residuals", "intervals", "alerts", "snapshots"}},
		{"documents.html", "documents", "Step 1 — Add files", []string{"files", "upload", "doc-list", "review-submit", "review-file", "source-details"}},
	}
	for _, p := range pages {
		t.Run(p.file, func(t *testing.T) {
			raw, err := staticFS.ReadFile("static/" + p.file)
			if err != nil {
				t.Fatal(err)
			}
			html := string(raw)
			for _, required := range []string{
				`data-ag-workflow="simple"`, `data-ag-phase="` + p.phase + `"`,
				`/static/workspace_nav.js`, `/static/ux_unified.css`,
				`/static/chapter_workflow.css`, p.chapter,
			} {
				if !strings.Contains(html, required) {
					t.Errorf("missing shared workflow contract %q", required)
				}
			}
			for _, id := range p.critical {
				needle := `id="` + id + `"`
				if strings.Count(html, needle) != 1 {
					t.Errorf("expected exactly one %s", needle)
				}
			}
			if strings.Contains(html, `href="/cutting" class="chapter-next"`) {
				t.Error("do not duplicate shared chapter navigation")
			}
		})
	}
	css, err := staticFS.ReadFile("static/chapter_workflow.css")
	if err != nil {
		t.Fatal(err)
	}
	// Scope the layout to the chapter main area; do not resize Aggie's shared chat.
	for _, required := range []string{`data-ag-workflow="simple"`, `main button`, `ag-work-extra`, `var(--ag-accent)`} {
		if !strings.Contains(string(css), required) {
			t.Errorf("missing CSS contract %s", required)
		}
	}
}
