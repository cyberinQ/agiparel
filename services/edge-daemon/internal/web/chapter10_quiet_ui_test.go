// Author: Sunil Jain, 2026-10-28
// The upload page stays focused: file intake, a compact library, and optional review.
package web

import (
	"strings"
	"testing"
)

func TestChapter10QuietUploadWithoutRemovingControls(t *testing.T) {
	data, err := staticFS.ReadFile("static/documents.html")
	if err != nil {
		t.Fatal(err)
	}
	body := string(data)
	for _, needle := range []string{
		"10 · UPLOAD FILES", "Keep your factory papers together.", "Step 1 — Add files", "Step 2 — Find your files",
		`class="library-options"`, `Sort files or see earlier versions`,
		`id="show-history"`, `id="sort-by"`, `id="review-submit"`,
		`id="review-file"`, `id="source-details"`, `id="doc-list"`,
		`src="/static/workspace_nav.js"`,
	} {
		if !strings.Contains(body, needle) {
			t.Errorf("missing quiet upload control %q", needle)
		}
	}
	if strings.Contains(body, `<h1>Upload records.</h1>`) {
		t.Error("Chapter 10 repeats its page name rather than using the original helpful heading")
	}
	if strings.Count(body, `<th scope="col">`) != 3 {
		t.Error("default upload library must have exactly three visible columns")
	}
	js, err := staticFS.ReadFile("static/documents.js")
	if err != nil {
		t.Fatal(err)
	}
	for _, needle := range []string{"uploaded_by", "format", "showHistory", "source-segment-", "cell.colSpan=3"} {
		if !strings.Contains(string(js), needle) {
			t.Errorf("missing uploader behavior %q", needle)
		}
	}
}

// A filename is a selectable source-evidence action, never a primary dark badge.
func TestChapter10FileNamesHaveQuietReadableStyle(t *testing.T) {
	css, err := staticFS.ReadFile("static/documents.css")
	if err != nil {
		t.Fatal(err)
	}
	text := string(css)
	for _, needed := range []string{
		`main .file-table button.file-name`,
		`background:transparent;`,
		`color:#215c78;`,
		`overflow-wrap:anywhere;`,
		`text-decoration:underline;`,
	} {
		if !strings.Contains(text, needed) {
			t.Errorf("missing plain file-name appearance %q", needed)
		}
	}
	js, err := staticFS.ReadFile("static/documents.js")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(js), `create('button','file-name'`) {
		t.Error("file-name style must target the actual library file buttons")
	}
}
