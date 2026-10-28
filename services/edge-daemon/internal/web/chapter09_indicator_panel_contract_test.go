// Author: Sunil Jain, 2026-10-28. Guard objective 8-chapter indicator navigation and evidence transparency.
package web

import (
	"strings"
	"testing"
)

func TestChapter09IndicatorsAreReachableAndNoFabricatedCharts(t *testing.T) {
	page, err := staticFS.ReadFile("static/factory_intelligence.html")
	if err != nil {
		t.Fatal(err)
	}
	html := string(page)
	for _, marker := range []string{
		`id="indicator-grid"`, `id="indicator-view"`, `role="dialog"`,
		`id="indicator-close"`, `id="indicator-plot"`, `id="indicator-source-link"`,
		`/static/factory_indicator_catalog.js`, `/static/factory_indicator_panel.js`,
		`Trend over time`, `not people's worth`,
	} {
		if !strings.Contains(html, marker) {
			t.Errorf("missing Chapter 09 indicator feature: %q", marker)
		}
	}
	catalog, err := staticFS.ReadFile("static/factory_indicator_catalog.js")
	if err != nil {
		t.Fatal(err)
	}
	for _, marker := range []string{`'Factory setup'`, `'Buyer order'`, `'Plan material'`, `'Fabric store'`, `'Cutting'`, `'Sewing'`, `'Quality'`, `'Packing'`, `REHEARSAL_ONLY_NOT_PRODUCTION_CLOSURE`, `NOT_MEASURED`} {
		if !strings.Contains(string(catalog), marker) {
			t.Errorf("missing catalog authority/source: %s", marker)
		}
	}
	panel, err := staticFS.ReadFile("static/factory_indicator_panel.js")
	if err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{`innerHTML`, `eval(`, `fetch(`, `localStorage`, `window.location=`} {
		if strings.Contains(string(panel), bad) {
			t.Errorf("unsafe indicator panel implementation: %s", bad)
		}
	}
}
