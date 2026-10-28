// Author: Sunil Jain, 2026-10-28
// Chapter 01 must expose evidence readiness and avoid privacy or approval confusion.
package web

import (
	"strings"
	"testing"
)

func TestChapter01ReadinessIsVisibleAndReviewIsNavigation(t *testing.T) {
	b, err := staticFS.ReadFile("static/factory_setup.html")
	if err != nil {
		t.Fatal(err)
	}
	page := string(b)
	for _, required := range []string{
		`id="decision-readiness"`, `id="readiness" class="readiness"`,
		`id="skip-next"`, `href="/documents#review-submit"`,
		`id="start-review"`, `id="fact-form"`, `id="approval-dialog"`,
		"Do not upload payroll", "Production release", "separate checks and sign-offs",
	} {
		if !strings.Contains(page, required) {
			t.Errorf("missing %s", required)
		}
	}
	for _, forbidden := range []string{
		`id="readiness" class="sr-only"`,
		`id="start-review" type="button"`,
		`id="start-review" aria-haspopup="dialog"`,
	} {
		if strings.Contains(page, forbidden) {
			t.Errorf("misleading element %s", forbidden)
		}
	}
	js, err := staticFS.ReadFile("static/factory_setup.js")
	if err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{
		`model.next_questions`, `skippedQuestions`, `factory-onboarding/facts`,
		`activeQuestion.field_key`, `Review skipped questions`, `model.readiness`,
	} {
		if !strings.Contains(string(js), required) {
			t.Errorf("missing guidance %s", required)
		}
	}
}
