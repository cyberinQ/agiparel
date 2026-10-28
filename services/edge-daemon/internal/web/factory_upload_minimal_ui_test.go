// Author: Sunil Jain, 2026-10-27
// Guard the minimal factory/upload screens and keep evidence links and document APIs reachable.
package web

import (
	"strings"
	"testing"
)

func TestFactoryAndUploadMinimalContracts(t *testing.T) {
	files := map[string][]string{
		"static/factory_setup.html": {
			"01 · FACTORY SETUP", "Review &amp; Submit", "approval-dialog",
			"/static/workspace_nav.js", "candidate-source", "factory-drafts", "factory-references",
		},
		"static/documents.html": {
			`id="files"`, `id="upload"`, `id="doc-list"`, `id="sort-by"`,
			`id="document-triage"`, `id="source-details"`, `id="aggie-chat"`,
			"/static/documents.js", "/static/document_triage.js", "/static/workspace_nav.js",
		},
		"static/documents.js": {
			"/api/v1/documents", "captured_at", "uploaded_by", "filename", "format",
			"source-segment-", "/source", "textContent",
		},
		"static/factory_setup.css": {".welcome .section-kicker{color:#16869A}"},
		"static/ux_unified.css":    {`body[data-ag-phase="factory"] {--ag-accent:#16869A}`},
	}
	for file, want := range files {
		t.Run(file, func(t *testing.T) {
			contents, err := staticFS.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			for _, s := range want {
				if !strings.Contains(string(contents), s) {
					t.Errorf("missing contract %q", s)
				}
			}
		})
	}
}
