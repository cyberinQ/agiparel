package web

import (
	"strings"
	"testing"
)

func TestFactoryDraftConflictUIAndDocumentTriage(t *testing.T) {
	pages := map[string][]string{
		"static/factory_setup.html":      {"/static/factory_draft_review.js", "factory-drafts", "draft-feedback", "draft-items", "/documents"},
		"static/documents.html":          {"/static/document_triage.js", "document-triage", "/static/documents.js"},
		"static/factory_draft_review.js": {"/api/v1/factory-onboarding/draft-masters", "expected_revision", "NEEDS_CLARIFICATION", "VERIFY_DRAFT", "/documents?document="},
		"static/document_triage.js":      {"/api/v1/document-triage", "Possible document purpose", "source-segment-"},
	}
	for name, required := range pages {
		raw, err := staticFS.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		body := string(raw)
		for _, needle := range required {
			if !strings.Contains(body, needle) {
				t.Errorf("%s missing %q", name, needle)
			}
		}
		if strings.HasSuffix(name, ".js") && strings.Contains(body, "innerHTML") {
			t.Errorf("%s uses unsafe dynamic HTML", name)
		}
	}
}
