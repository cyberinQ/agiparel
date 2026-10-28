// Author: Sunil Jain, 2026-10-28
// Chapter 10 filenames stay unfilled even when the generic chapter workflow stylesheet is loaded.
package web

import (
	"strings"
	"testing"
)

func TestChapter10FileNamesNeverInheritPrimaryButtonFill(t *testing.T) {
	workflow, err := staticFS.ReadFile("static/chapter_workflow.css")
	if err != nil {
		t.Fatal(err)
	}
	css := string(workflow)
	selectorStart := strings.Index(css, `body[data-ag-workflow="simple"] main button:not(.secondary)`)
	if selectorStart < 0 {
		t.Fatal("generic chapter button rule missing")
	}
	selectorEnd := strings.Index(css[selectorStart:], "{")
	if selectorEnd < 0 {
		t.Fatal("generic chapter button rule malformed")
	}
	if !strings.Contains(css[selectorStart:selectorStart+selectorEnd], ":not(.file-name)") {
		t.Fatal("file-name buttons inherit the dark primary-button background")
	}
	page, err := staticFS.ReadFile("static/documents.html")
	if err != nil {
		t.Fatal(err)
	}
	html := string(page)
	for _, token := range []string{
		`name="agiparel-upload-version" content="20261027.v9"`,
		`/static/documents.js?v=20261027.v9`,
		`/static/review_submit.js?v=20261027.v9`,
	} {
		if !strings.Contains(html, token) {
			t.Errorf("missing existing upload contract: %s", token)
		}
	}
	generic := strings.Index(html, `/static/chapter_workflow.css`)
	specific := strings.Index(html, `/static/documents.css`)
	if generic < 0 || specific < 0 || generic >= specific {
		t.Fatal("Chapter 10 stylesheet must follow the shared workflow stylesheet")
	}
	documentCSS, err := staticFS.ReadFile("static/documents.css")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(documentCSS), `main .file-table button.file-name`) ||
		!strings.Contains(string(documentCSS), `background:transparent;`) {
		t.Fatal("plain filename style missing")
	}
}
