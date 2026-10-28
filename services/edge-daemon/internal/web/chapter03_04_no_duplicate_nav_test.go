// Author: Sunil Jain, 2026-10-28 — Both chapters share the one clean header.
package web

import (
	"strings"
	"testing"
)

func TestSimpleHeadersOptOutOfDuplicateChapterNavigation(t *testing.T) {
	script, err := staticFS.ReadFile("static/workspace_nav.js")
	if err != nil {
		t.Fatal(err)
	}
	for _, marker := range []string{`ag-chapter-bar`, `ag-chapter-brand`, `ag-chapter-home`, `aggie-open`} {
		if !strings.Contains(string(script), marker) {
			t.Errorf("missing shared chapter header marker %q", marker)
		}
	}
	for _, forbidden := range []string{`Next →`, `More tools`, `mobile-menu`, `areas.append`, `addPager(`} {
		if strings.Contains(string(script), forbidden) {
			t.Errorf("unexpected chapter navigation %q", forbidden)
		}
	}
	for _, name := range []string{"material_planning.html", "stores.html"} {
		raw, err := staticFS.ReadFile("static/" + name)
		if err != nil {
			t.Fatal(err)
		}
		s := string(raw)
		for _, required := range []string{`data-ag-simple-header="true"`, `id="aggie-open"`, `src="/static/workspace_nav.js"`} {
			if !strings.Contains(s, required) {
				t.Errorf("%s missing %s", name, required)
			}
		}
		if strings.Count(s, `id="aggie-open"`) != 1 {
			t.Errorf("%s must have one Aggie trigger", name)
		}
	}
}
