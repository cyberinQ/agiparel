package web

// Author: Sunil Jain, 2026-10-28
// Keep Chapter 04 visual contracts and the observation/stock-authority boundary stable.

import (
	"strings"
	"testing"
)

func TestChapter04OperatorIntegrityUI(t *testing.T) {
	raw, err := GetStoresHTML()
	if err != nil {
		t.Fatal(err)
	}
	html := string(raw)
	for _, required := range []string{
		`id="new-roll"`, `id="roll-count-note"`, `id="passport-form"`,
		`id="source-document"`, `id="pick-order"`, `id="reconciliation-form"`,
		`data-ag-phase="stores"`, `id="secure-reservations"`,
		`two separate external-IdP signatures`,
	} {
		if strings.Count(html, required) != 1 {
			t.Errorf("expected exactly one %s", required)
		}
	}
	css, err := staticFS.ReadFile("static/stores.css")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(css), "#a87a25") {
		t.Fatal("original Chapter 04 theme changed")
	}
	for _, filename := range []string{"stores.js", "stores_pick_proposals.js", "stores_reconciliation.js"} {
		js, err := staticFS.ReadFile("static/" + filename)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(js), "method:'DELETE'") || strings.Contains(string(js), "fetch('/api/v1/material/") {
			t.Errorf("%s attempted bypass of signed material authority", filename)
		}
	}
}
