// Author: Sunil Jain, 2026-10-27
// Ensure buyer material names get explicit, one-tap code confirmation without relaxing server validation.
package web

import (
	"strings"
	"testing"
)

func TestBuyerMaterialCodeConfirmation(t *testing.T) {
	js, err := staticFS.ReadFile("static/buyer_review.js")
	if err != nil {
		t.Fatal(err)
	}
	html, err := staticFS.ReadFile("static/buyer_review.html")
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range []string{"suggestMaterialCode", "material-code-confirm", "checkMaterialRows()", "field.value=proposed", "review_note.length<8"} {
		if !strings.Contains(string(js), s) {
			t.Errorf("buyer code confirmation missing %s", s)
		}
	}
	if !strings.Contains(string(html), "we’ll suggest a code") {
		t.Error("buyer screen must explain human code confirmation")
	}
}
