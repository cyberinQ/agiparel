// Author: Sunil Jain, 2026-10-28 — Unified chapter banner and Home colors.
package web

// Author: Sunil Jain, 2026-10-28 — Guard shared ten-chapter colors, navigation, Aggie, and CSP-safe styles.
import (
	"regexp"
	"strings"
	"testing"
)

func TestChapterThreeFourShareHomeVisualContract(t *testing.T) {
	for _, sample := range []struct{ page, chapter, phase, color string }{
		{"material_planning", "03 · PLAN MATERIAL", "material", "#7759B2"},
		{"stores", "04 · FABRIC STORE", "stores", "#A87A25"},
	} {
		t.Run(sample.page, func(t *testing.T) {
			body, err := staticFS.ReadFile("static/" + sample.page + ".html")
			if err != nil {
				t.Fatal(err)
			}
			html := string(body)
			for _, marker := range []string{sample.chapter, `data-ag-phase="` + sample.phase + `"`, `src="/static/workspace_nav.js"`, `id="aggie-open"`, `id="aggie-chat"`, `src="/static/portal.js"`} {
				if !strings.Contains(html, marker) {
					t.Errorf("missing %q", marker)
				}
			}
			if strings.Count(html, `id="aggie-open"`) != 1 {
				t.Fatal("Aggie trigger must be unique")
			}
			if strings.Contains(html, `class="material-top"`) || strings.Contains(html, `class="store-top"`) {
				t.Error("duplicate chapter navigation")
			}
			css, err := staticFS.ReadFile("static/" + sample.page + ".css")
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(strings.ToLower(string(css)), strings.ToLower(sample.color)) {
				t.Error("chapter accent differs from Home")
			}
		})
	}
	nav, err := staticFS.ReadFile("static/workspace_nav.js")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(nav), "createElement('style')") {
		t.Error("CSP prohibits injected inline stylesheet")
	}
	if !strings.Contains(string(nav), "/static/workspace_nav.css") {
		t.Error("missing CSP-safe shared styles")
	}
	navCSS, err := staticFS.ReadFile("static/workspace_nav.css")
	if err != nil {
		t.Fatal(err)
	}
	for _, marker := range []string{`var(--ag-accent,#167e88)`, `ag-chapter-pipe`, `ag-chapter-home`, `.ag-chapter-actions`} {
		if !strings.Contains(string(navCSS), marker) {
			t.Errorf("missing shared color %s", marker)
		}
	}
	common, err := staticFS.ReadFile("static/ux_unified.css")
	if err != nil {
		t.Fatal(err)
	}
	for _, theme := range []struct{ phase, color string }{{"material", "#7759B2"}, {"stores", "#A87A25"}} {
		// CSS whitespace is not semantically meaningful; test the rule, not its formatting.
		pattern := `body\[data-ag-phase="` + theme.phase + `"\]\s*\{\s*--ag-accent\s*:\s*` + theme.color + `\s*[;}]`
		if !regexp.MustCompile(`(?i)` + pattern).Match(common) {
			t.Errorf("missing canonical theme for %s (%s)", theme.phase, theme.color)
		}
	}
	// The local Chapter 03 stylesheet must not hide the shared chapter banner.
	materialCSS, err := staticFS.ReadFile("static/material_planning.css")
	if err != nil { t.Fatal(err) }
	if regexp.MustCompile(`(?i)#agiparel-workspace-navigation\s*\{[^}]*display\s*:\s*none`).Match(materialCSS) {
		t.Error("Chapter 03 CSS hides shared header")
	}
}
