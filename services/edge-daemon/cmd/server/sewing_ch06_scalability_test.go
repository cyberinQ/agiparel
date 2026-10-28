// Author: Sunil Jain, 2026-10-28
// Chapter 06 cursor and factory-independent sewing catalogue contract.
package main

import (
	"strings"
	"testing"
	"time"
)

func TestSew06LayCursorRoundTripAndRejectsInvalid(t *testing.T) {
	at := time.Date(2026, 10, 28, 20, 50, 25, 123456000, time.UTC)
	original := sewLayCursor{Created: at, ID: exampleLayID}
	wire := sewEncodeLayCursor(original.Created, original.ID)
	decoded, err := sewDecodeLayCursor(wire)
	if err != nil || !decoded.Created.Equal(original.Created) || decoded.ID != original.ID {
		t.Fatalf("keyset cursor round-trip: %+v, %v", decoded, err)
	}
	for _, bad := range []string{"!invalid!", "abc", strings.Repeat("Z", 2000)} {
		if _, err := sewDecodeLayCursor(bad); err == nil {
			t.Errorf("accepted malformed lay cursor %q", bad[:min(len(bad), 30)])
		}
	}
}

func TestSew06BundleGroupCursorRejectsMalformed(t *testing.T) {
	cases := []struct {
		wire               string
		wantPos, wantGroup int
		valid              bool
	}{
		{"", 0, 0, true}, {"123/45", 123, 45, true},
		{"-1/7", 0, 0, false}, {"1/0", 0, 0, false}, {"1/2/3", 0, 0, false}, {"wat/1", 0, 0, false},
	}
	for _, tc := range cases {
		pos, group, err := sewDecodeGroupCursor(tc.wire)
		if (err == nil) != tc.valid || (tc.valid && (pos != tc.wantPos || group != tc.wantGroup)) {
			t.Errorf("cursor %q yielded %d/%d: %v", tc.wire, pos, group, err)
		}
	}
}
