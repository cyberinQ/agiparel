// Author: Sunil Jain, 2026-10-27
// Guard simple employee review and the retained advanced evidence/revision tools.
package web

import (
	"strings"
	"testing"
)

func TestUploadPageReviewSubmitProgressiveDisclosure(t *testing.T) {
	paths := map[string][]string{
		"static/documents.html": {
			`id="review-submit"`, `id="review-file"`, `id="review-scan"`,
			`id="review-pending"`, `id="review-card"`, `id="review-feedback"`,
			`/static/review_submit.js`, `Manager approval saves factory reference records`,
			`id="doc-list"`, `id="upload"`, `id="source-details"`,
		},
		"static/review_submit.js": {
			`/api/v1/factory-onboarding/approvals`, `/approvals/review`,
			`/candidates/scan`, `/approvals/submit`, `expected_revision`, `checked_fields`,
			`document_id`, `source_ordinal`, `source_locator`,
			`textContent`, `createElement`, `rate_basis`,
		},
		"static/factory_setup.js": {
			`/documents#review-submit`, `advanced-review`, `approvals.showModal()`,
		},
		"static/factory_setup.html": {
			`id="approval-dialog"`, `id="factory-drafts"`, `id="factory-references"`,
		},
	}
	for path, needles := range paths {
		t.Run(path, func(t *testing.T) {
			content, err := staticFS.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			for _, needle := range needles {
				if !strings.Contains(string(content), needle) {
					t.Errorf("missing %q", needle)
				}
			}
		})
	}
	doc, err := staticFS.ReadFile("static/documents.html")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(doc), "Check conflicting records and verified references") || strings.Contains(string(doc), "/factory-setup?advanced-review=1") {
		t.Fatal("advanced tools must not appear in the employee review workflow")
	}
	js, err := staticFS.ReadFile("static/review_submit.js")
	if err != nil {
		t.Fatal(err)
	}
	for _, unsafe := range []string{"innerHTML", "/api/v1/stores/issue", "/api/v1/material/issue", "/api/v1/factory-onboarding/reference-masters/revise"} {
		if strings.Contains(string(js), unsafe) {
			t.Errorf("reviewer contains disallowed operation %q", unsafe)
		}
	}
}
