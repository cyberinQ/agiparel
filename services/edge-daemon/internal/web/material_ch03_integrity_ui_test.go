// Author: Sunil Jain, 2026-10-28 — Chapter 03 evidence and operator accuracy contract.
package web

import (
	"strings"
	"testing"
)

func TestMaterialChapter03IntegrityAndVisualContract(t *testing.T) {
	html, err := staticFS.ReadFile("static/material_planning.html")
	if err != nil {
		t.Fatal(err)
	}
	js, err := staticFS.ReadFile("static/material_planning.js")
	if err != nil {
		t.Fatal(err)
	}
	css, err := staticFS.ReadFile("static/material_planning.css")
	if err != nil {
		t.Fatal(err)
	}
	for _, token := range []string{`data-ag-phase="material"`, `id="stage"`, `id="preview"`, `Conditional gap`, `Possible factory purchases`, `buyer-supplied gaps are excluded`} {
		if !strings.Contains(string(html), token) {
			t.Errorf("missing Chapter 03 UI contract: %q", token)
		}
	}
	for _, token := range []string{`preview_sha256`, `generation`, `invalidatePreview()`, `result.intake_id !== id`, `Claims only`, `factoryGaps`, `buyerGaps`} {
		if !strings.Contains(string(js), token) {
			t.Errorf("missing Chapter 03 correctness guard: %q", token)
		}
	}
	for _, token := range []string{`#7759B2`, `@media(max-width:580px)`, `body[data-ag-phase="material"] #agiparel-workspace-navigation{display:block}`} {
		if !strings.Contains(string(css), token) {
			t.Errorf("visual theme must remain unchanged: %q", token)
		}
	}
}
