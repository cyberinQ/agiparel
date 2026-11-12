// Author: Sunil Jain, 2026-11-12.
// Verify the ten-chapter manual, exact navigation and plain-language question routing.
package workerhelp

import (
	"strings"
	"testing"
)

func TestManualComplete(t *testing.T) {
	m := All()
	if len(m.Chapters) != 10 {
		t.Fatalf("expected 10 chapters, got %d", len(m.Chapters))
	}
	for i, ch := range m.Chapters {
		if ch.Number != i+1 || ch.Path == "" || ch.Done == "" || ch.Stop == "" || len(ch.Steps) != 6 {
			t.Fatalf("chapter %d incomplete", i+1)
		}
		if _, ok := ForPath(ch.Path); !ok {
			t.Fatalf("unmapped chapter path %s", ch.Path)
		}
	}
}

func TestManualRouting(t *testing.T) {
	cases := []struct {
		question, page string
		number         int
	}{
		{"How do I upload a buyer document?", "/buyer-review", 10},
		{"How do I set up a cutting plan?", "/", 5},
		{"Walk me through chapter 8", "/sewing", 8},
		{"Help me with this page", "/stores", 4},
		{"How do I pack a carton?", "/cutting", 8},
		{"How do I record a sewing job?", "/cutting", 6},
	}
	for _, tc := range cases {
		c, ok := Match(tc.question, tc.page)
		if !ok || c.Number != tc.number {
			t.Errorf("%q: got chapter %d, ok %t; want %d", tc.question, c.Number, ok, tc.number)
		}
		if ok && !strings.Contains(Answer(c), "STOP IF:") {
			t.Errorf("chapter %d missing warning", c.Number)
		}
	}
	for _, q := range []string{"How many pieces were shipped today?", "What is the factory capacity?", "Which PO is in progress?", "Tell me today's worker attendance"} {
		if _, ok := Match(q, "/cutting"); ok {
			t.Errorf("factory-data question intercepted as manual: %s", q)
		}
	}
	if _, ok := ForPath("/api/v1/agie/manual"); ok {
		t.Fatal("untrusted path must not resolve to chapter")
	}
}

func TestFullManualContent(t *testing.T) {
	all := FullText()
	for _, ch := range All().Chapters {
		if !strings.Contains(all, ch.Title) || !strings.Contains(all, ch.Stop) {
			t.Fatalf("full manual missing chapter %d", ch.Number)
		}
	}
	if !IsFullManualRequest("Please show me the full manual") {
		t.Fatal("full-manual request not recognized")
	}
}
