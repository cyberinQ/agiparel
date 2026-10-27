package web

import (
	"strings"
	"testing"
)

func TestStoresPickProposalsUI(t *testing.T) {
	html, e := GetStoresHTML()
	if e != nil {
		t.Fatal(e)
	}
	for _, needle := range []string{`id="pick-form"`, `id="pick-history"`, `id="pick-results"`, "/static/stores_pick_proposals.js", "never borrowed", "No physical movement", "does not create available stock", `id="roll-admission-workbench"`} {
		if !strings.Contains(strings.ToLower(string(html)), strings.ToLower(needle)) {
			t.Errorf("missing %q", needle)
		}
	}
	js, e := staticFS.ReadFile("static/stores_pick_proposals.js")
	if e != nil {
		t.Fatal(e)
	}
	if !strings.Contains(string(js), "/api/v1/stores/pick-proposals") {
		t.Fatal("missing pick API")
	}
	if strings.Contains(string(js), "/api/v1/material/") {
		t.Fatal("pick UI must not call secure material writer")
	}
}
