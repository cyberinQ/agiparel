// Author: Sunil Jain, 2026-10-27
// Guard Chapter 02's simple workflow, CSP-safe color and unchanged evidence/approval controls.
package web

import (
	"strings"
	"testing"
)

func TestBuyerChapterMinimalUXPreservesOrderReview(t *testing.T) {
	read := func(name string) string {
		t.Helper()
		data, err := staticFS.ReadFile("static/" + name)
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}
	html, css, js := read("buyer_review.html"), read("buyer_review.css"), read("buyer_review.js")
	for _, token := range []string{
		`data-ag-phase="buyer"`, `data-ag-simple-header="true"`,
		`src="/static/workspace_nav.js"`, `src="/static/portal.js"`,
		`id="aggie-open"`, `id="aggie-chat"`, `href="/"`,
		`02 · BUYER ORDER`, `Choose buyer files`, `Check order details`,
		`id="po-source"`, `id="bom-source"`, `id="analyze"`,
		`id="po-number"`, `id="style-code"`, `id="sku-rows"`, `id="bom-rows"`,
		`id="techpack-workbench"`, `id="po-evidence"`, `id="bom-evidence"`,
		`id="review-note"`, `id="acknowledged"`, `id="stage"`, `id="result"`,
		`0.000`, `PROPOSED · HOLD`,
	} {
		if !strings.Contains(html, token) {
			t.Errorf("buyer UI missing %q", token)
		}
	}
	for _, token := range []string{
		`--ag-accent:#326DB3`, `@media(max-width:850px)`, `data-label`,
		`prefers-reduced-motion`, `.aggie-face`,
	} {
		if !strings.Contains(css, token) {
			t.Errorf("buyer CSS missing %q", token)
		}
	}
	for _, token := range []string{
		`/api/v1/buyer-review`, `source_ordinal`, `review_note`, `acknowledged`,
		`techpack_material_hints`, `per_garment:''`, `uom:''`,
		`td.dataset.label=label`, `sources.length===1`,
	} {
		if !strings.Contains(js, token) {
			t.Errorf("buyer JS missing %q", token)
		}
	}
	for _, old := range []string{`DEVELOPMENT · RESTRICTED`, `<header class="top">`, `<ol class="ag-steps"`, `class="hero-art"`} {
		if strings.Contains(html, old) {
			t.Errorf("buyer page contains obsolete clutter %q", old)
		}
	}
}
