// Author: Sunil Jain, 2026-10-28
// Shared chapter shell must not duplicate Home navigation; unsaved form recovery
// must not persist file selections, authentication material, or approvals.
package web

import (
	"strings"
	"testing"
)

func TestTenChaptersUseOneCleanBannerAndLocalWIP(t *testing.T) {
	nav, err := staticFS.ReadFile("static/workspace_nav.js")
	if err != nil {
		t.Fatal(err)
	}
	code := string(nav)
	for _, required := range []string{"ag-chapter-brand", "ag-chapter-pipe", "ag-chapter-home", "ag-chapter-history-group", "aggie-open", "workspace_draft.js", "'/documents'", "'/intelligence'"} {
		if !strings.Contains(code, required) {
			t.Errorf("shared banner missing %q", required)
		}
	}
	for _, forbidden := range []string{"Next →", "More tools", "attachShadow", "mobile-menu", "areas.append"} {
		if strings.Contains(code, forbidden) {
			t.Errorf("duplicated header navigation %q", forbidden)
		}
	}
	css, err := staticFS.ReadFile("static/workspace_nav.css")
	if err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{".ag-chapter-pipe", "var(--ag-accent", ".ag-chapter-actions", ".ag-wip-resume"} {
		if !strings.Contains(string(css), required) {
			t.Errorf("shared style missing %q", required)
		}
	}
	draft, err := staticFS.ReadFile("static/workspace_draft.js")
	if err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{"sessionStorage", "agiparel:draft-committed", "Resume work", "Discard", "file selection", "allowedTypes", "sensitive"} {
		if !strings.Contains(strings.ToLower(string(draft)), strings.ToLower(required)) {
			t.Errorf("draft contract missing %q", required)
		}
	}
	for _, page := range []string{"factory_setup", "buyer_review", "material_planning", "stores", "cutting_view", "sewing_execution", "quality_execution", "packing_execution", "factory_intelligence", "documents"} {
		body, err := staticFS.ReadFile("static/" + page + ".html")
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(body), `src="/static/workspace_nav.js"`) {
			t.Errorf("%s has no shared header", page)
		}
	}
}
