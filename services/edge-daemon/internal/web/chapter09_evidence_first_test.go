// Author: Sunil Jain, 2026-10-28.
// Chapter 09 must show a defensible report, not make missing factory data look measured.
package web

import (
	"strings"
	"testing"
)

func TestChapter09EvidenceFirstWorkerFlow(t *testing.T) {
	page, err := staticFS.ReadFile("static/factory_intelligence.html")
	if err != nil {
		t.Fatal(err)
	}
	html := string(page)
	for _, required := range []string{
		`data-ag-phase="reports"`, `/static/workspace_nav.js`, `/static/factory_intelligence.css`,
		`/static/factory_report_metrics.js`, `09 · FACTORY REPORT`, `Step 1 — Choose work`,
		`Step 2 — See progress`, `Step 3 — What needs attention?`,
		`Practice records only`, `not measured`, `Ask Aggie to explain`,
		`id="lay"`, `id="timezone"`, `id="refresh"`, `id="capture"`,
		`id="metric-sewn"`, `id="metric-accepted"`, `id="metric-hold"`,
		`id="metric-packed"`, `id="stages"`, `id="residuals"`,
		`id="intervals"`, `id="operations"`, `id="alerts"`, `id="snapshots"`,
		`id="interval-exclusions"`, `id="method"`, `id="evidence"`,
	} {
		if !strings.Contains(html, required) {
			t.Errorf("missing worker or safety contract: %q", required)
		}
	}
	js, err := staticFS.ReadFile("static/factory_intelligence.js")
	if err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{
		`M.AUTHORITY`, `textContent`, `credentials:'same-origin'`,
		`report?lay_id=`, `snapshots?lay_id=`, `capture`, `M.question(last)`,
	} {
		if !strings.Contains(string(js), required) {
			t.Errorf("missing read-only report contract: %s", required)
		}
	}
	for _, forbidden := range []string{`innerHTML=`, `eval(`, `UPDATE hourly_line_metrics`, `DELETE FROM`} {
		if strings.Contains(string(js), forbidden) {
			t.Errorf("unsafe report display or SQL: %s", forbidden)
		}
	}
}
