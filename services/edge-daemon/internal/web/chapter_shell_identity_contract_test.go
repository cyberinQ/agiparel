// Author: Sunil Jain, 2026-10-28
// Keep chapter branding, Aggie and footer consistent with the Home page.
package web

import (
	"regexp"
	"strings"
	"testing"
)

func TestSharedChapterBrandAndFooterContract(t *testing.T) {
	read := func(file string) string {
		t.Helper()
		b, err := staticFS.ReadFile("static/" + file)
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	js := read("workspace_nav.js")
	css := read("workspace_nav.css")
	home := read("portal.html")

	for _, expected := range []string{
		"ag-chapter-brand", "ag-chapter-pipe", "ag-chapter-home", "ag-chapter-history",
		"agiparel-workspace-footer", "footerBrand.textContent = 'agiparel'",
		"powered.textContent = 'Empowered by CyberinQ.'",
		"privacy.textContent = 'Customer Records are Private, Secured and Encrypted.'",
	} {
		if !strings.Contains(js, expected) {
			t.Errorf("shared chapter shell missing %q", expected)
		}
	}
	if !strings.Contains(home, "Public home · No customer records") {
		t.Error("Home privacy label must stay unchanged")
	}

	// Explicit pixel sizes prevent legacy chapter styles and mobile media queries from resizing the mark.
	for _, required := range []string{
		"font-size:27px", "height:13px", "width:10px", "background:var(--ag-accent,#167e88)",
		"color:var(--ag-accent,#167e88)", "width:41px", "height:41px", "font:400 28px/1",
		"animation:ag-chapter-aggie-wiggle 4.8s", "#agiparel-workspace-footer",
		"font:850 19px/1.5", "padding:24px max(22px,calc((100vw - 1240px)/2))",
	} {
		if !strings.Contains(css, required) {
			t.Errorf("shared stylesheet missing %q", required)
		}
	}
	if regexp.MustCompile(`(?i)@media\([^}]+\.ag-chapter-pipe\s*\{[^}]*height\s*:`).MatchString(css) {
		t.Error("mobile rules must not change the pipe height")
	}

	themes := map[string]string{
		"factory": "#16869A", "buyer": "#326DB3", "material": "#7759B2", "stores": "#A87A25",
		"cutting": "#C76B3D", "sewing": "#16847D", "quality": "#3C855E", "packing": "#B34E7A",
		"reports": "#566F91", "documents": "#28323B",
	}
	unified := read("ux_unified.css")
	for phase, color := range themes {
		pattern := `(?i)body\[data-ag-phase="` + phase + `"\]\s*\{\s*--ag-accent\s*:\s*` + color + `\s*[;}]`
		if !regexp.MustCompile(pattern).MatchString(unified) {
			t.Errorf("%s must match Home color %s", phase, color)
		}
	}
}
