// Author: Sunil Jain, 2026-10-28. Validate cursor and size bounds.
package main

import (
	"strings"
	"testing"
	"time"
)

// Author: Sunil Jain, 2026-10-28. Safe pagination parameters are never
// interpreted as permissions or as a tenant identifier.
func TestDocumentCatalogCursorValidationV14(t *testing.T) {
	cases := []struct {
		raw   string
		valid bool
	}{
		{"", true}, {"bad%%%", false}, {strings.Repeat("a", 513), false},
	}
	for _, tc := range cases {
		_, err := docCursorDecode(tc.raw)
		if (err == nil) != tc.valid {
			t.Errorf("cursor %q valid=%t err=%v", tc.raw, tc.valid, err)
		}
	}
	at := time.Date(2026, 10, 28, 18, 0, 0, 123000000, time.UTC)
	id := "DOC-0123456789abcdef0123456789abcdef"
	token := docCursorEncode(at, id)
	got, err := docCursorDecode(token)
	if err != nil || !got.CapturedAt.Equal(at) || got.ID != id {
		t.Fatalf("roundtrip %+v %v", got, err)
	}
	for _, tc := range []struct {
		raw   string
		want  int
		valid bool
	}{{"", 100, true}, {"1", 1, true}, {"100", 100, true}, {"101", 0, false}, {"0", 0, false}, {"-3", 0, false}, {"abc", 0, false}} {
		actual, err := docPageSize(tc.raw)
		if actual != tc.want || (err == nil) != tc.valid {
			t.Errorf("limit=%q got=%d err=%v", tc.raw, actual, err)
		}
	}
}

func TestDocumentSourceCursorBoundsV14(t *testing.T) {
	for _, c := range []struct {
		raw   string
		valid bool
	}{{"", true}, {"0", true}, {"200", true}, {"10000", true}, {"10001", false}, {"-1", false}, {"NaN", false}} {
		_, err := docSegmentAfter(c.raw)
		if (err == nil) != c.valid {
			t.Errorf("segment cursor %q err=%v", c.raw, err)
		}
	}
}
