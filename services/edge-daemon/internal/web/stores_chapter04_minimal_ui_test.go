package web

// Author: Sunil Jain, 2026-10-27 — Guard the Chapter 04 minimal UI and signed-stock boundary.

import (
	"strings"
	"testing"
)

func TestChapter04MinimalFabricStore(t *testing.T) {
	raw, err := GetStoresHTML()
	if err != nil {
		t.Fatal(err)
	}
	html := string(raw)
	for _, required := range []string{
		`data-ag-phase="stores"`, `data-ag-simple-header="true"`,
		`04 · FABRIC STORE`, `Know your fabric rolls.`, `id="passport-form"`,
		`id="roll-list"`, `id="passport-history"`, `id="message"`,
		`id="observed"`, `id="passed"`, `id="held"`,
		`id="verified"`, `id="reserved"`,
		`id="pick-form"`, `id="pick-refresh"`, `id="pick-results"`,
		`id="reconciliation-form"`, `id="admission-queue"`, `id="secure-reservations"`,
		`id="aggie-open"`, `id="aggie-chat"`, `src="/static/portal.js"`,
		`href="/material-planning"`,
		`two separate external-IdP signatures`, `does not create available stock`,
		`No physical movement`,
	} {
		if !strings.Contains(strings.ToLower(html), strings.ToLower(required)) {
			t.Errorf("missing UI contract %q", required)
		}
	}
	if strings.Contains(html, `class="chapter-next"`) || strings.Contains(html, `href="/cutting"`) {
		t.Error("Chapter 04 must not show previous/next chapter navigation")
	}
	for _, singular := range []string{`id="pick-refresh"`, `id="passport-form"`, `id="roll-list"`, `id="aggie-open"`} {
		if strings.Count(html, singular) != 1 {
			t.Errorf("expected exactly one %s", singular)
		}
	}
	css, err := staticFS.ReadFile("static/stores.css")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(css), "#a87a25") || !strings.Contains(string(css), "@media(max-width:610px)") {
		t.Fatal("Chapter 04 color and responsive contract missing")
	}
	for _, src := range []string{"stores.js", "stores_reconciliation.js", "stores_admission.js", "stores_pick_proposals.js"} {
		js, err := staticFS.ReadFile("static/" + src)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(js), `fetch('/api/v1/material/`) || strings.Contains(string(js), `method:'DELETE'`) {
			t.Errorf("%s must not bypass material authority", src)
		}
	}
}
