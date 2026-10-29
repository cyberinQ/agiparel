// Author: Sunil Jain, CyberinQ LLC, 2026-10-29.
package main

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestF316SignedPieceChallenge(t *testing.T) {
	base := pieceHoldCommand{CommandID: "PIECE_HOLD_0123456", PlantID: "plant_01", GarmentID: "c6e63b80-e150-4364-90be-759755e3ec46", StationID: "sewing_02", Action: "HOLD", Reason: "NEEDLE_DAMAGE", Note: "Needle pierced individual panel"}
	if !pieceHoldValid(base) {
		t.Fatal("valid one-piece hold rejected")
	}
	x := pieceHoldChallenge(base)
	if !strings.HasPrefix(x, "WIP_PIECE_V26:") || len(x) != len("WIP_PIECE_V26:")+64 {
		t.Fatalf("invalid challenge %q", x)
	}
	perm, role := pieceHoldPermission("HOLD")
	if perm != "sewing.piece.hold" || role != "sewing_operator" {
		t.Fatal("incorrect operator permission")
	}
	for _, reason := range []string{"SEWING_DEFECT", "MISSING_COMPONENT", "OIL_STAIN", "NEEDLE_DAMAGE", "REWORK_REQUIRED"} {
		c := base
		c.Reason = reason
		if !pieceHoldValid(c) {
			t.Errorf("reason %s rejected", reason)
		}
	}
	for i, mutate := range []func(*pieceHoldCommand){
		func(c *pieceHoldCommand) { c.GarmentID = "g-01" },
		func(c *pieceHoldCommand) { c.CommandID = "short" },
		func(c *pieceHoldCommand) { c.Reason = "AUTOMATIC_REPAIR" },
		func(c *pieceHoldCommand) { c.HoldID = "c6e63b80-e150-4364-90be-759755e3ec46" },
		func(c *pieceHoldCommand) { c.ScanID = "c6e63b80-e150-4364-90be-759755e3ec46" },
		func(c *pieceHoldCommand) { c.Note = "short" },
		func(c *pieceHoldCommand) { c.Action = "AUTO_CREDIT" },
	} {
		c := base
		mutate(&c)
		if pieceHoldValid(c) {
			t.Errorf("unsafe hold %d accepted", i)
		}
	}
	c := base
	c.Note = "Observed one damaged sleeve panel"
	if pieceHoldChallenge(c) == x {
		t.Fatal("note not challenge-bound")
	}
	c = base
	c.Action = "REVIEW"
	c.Reason = ""
	c.HoldID = "0f629944-1fcf-49e4-b50d-b1fca541c241"
	c.ScanID = "0e629944-1fcf-49e4-b50d-b1fca541c242"
	c.ReviewAction = "SUPERVISOR_REVIEWED_FOR_WIP"
	if !pieceHoldValid(c) {
		t.Fatal("valid independent review rejected")
	}
	perm, role = pieceHoldPermission("REVIEW")
	if perm != "sewing.piece.review" || role != "sewing_supervisor" {
		t.Fatal("incorrect supervisor permission")
	}
	c.ScanID = ""
	if pieceHoldValid(c) {
		t.Fatal("review without new scan accepted")
	}
}
func TestF316ReadScopeAndFailClosed(t *testing.T) {
	q := url.Values{"tenant_id": {"tenant_01"}, "plant_id": {"plant_01"}, "lay_draft_id": {"c6e63b80-e150-4364-90be-759755e3ec46"}, "marker_position": {"1"}, "ply_group": {"25"}}
	v, err := parsePieceHoldQuery(q)
	if err != nil || v.Group != 25 || v.Position != 1 {
		t.Fatal("good query rejected", v, err)
	}
	for _, key := range []string{"tenant_id", "plant_id", "lay_draft_id", "marker_position", "ply_group"} {
		b := url.Values{}
		for k, vs := range q {
			b[k] = append([]string(nil), vs...)
		}
		b.Del(key)
		if _, e := parsePieceHoldQuery(b); e == nil {
			t.Errorf("missing %s accepted", key)
		}
	}
	for _, bad := range []url.Values{
		{"tenant_id": {"tenant_01"}, "plant_id": {"plant_01"}, "lay_draft_id": {"bad"}, "marker_position": {"1"}, "ply_group": {"2"}},
		{"tenant_id": {"tenant_01"}, "plant_id": {"plant_01"}, "lay_draft_id": {"c6e63b80-e150-4364-90be-759755e3ec46"}, "marker_position": {"81"}, "ply_group": {"2"}},
	} {
		if _, e := parsePieceHoldQuery(bad); e == nil {
			t.Fatal("invalid scope accepted")
		}
	}
	t.Setenv("ENV", "development")
	t.Setenv("AGIPAREL_PIECE_HOLDS_ENABLED", "true")
	p := newPieceHoldAPI(nil)
	w := httptest.NewRecorder()
	p.event(w, httptest.NewRequest(http.MethodPost, "/api/v1/wip/pieces/events", strings.NewReader(`{}`)))
	if w.Code != 503 || !strings.Contains(w.Body.String(), "ERR_F316_DISABLED") {
		t.Fatal("dev write gate not fail-closed")
	}
}
func TestF316IngressExplicitPaths(t *testing.T) {
	t.Setenv("ENV", "production")
	h := configuredOperatorGate(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(204) }))
	for _, path := range []string{"/api/v1/wip/pieces/challenge", "/api/v1/wip/pieces/events", "/api/v1/wip/pieces/groups"} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		if w.Code != 204 {
			t.Errorf("ingress missing %s: %d", path, w.Code)
		}
	}
	for _, path := range []string{"/api/v1/wip/pieces/delete", "/api/v1/wip/pieces/credit"} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		if w.Code == 204 {
			t.Error("overbroad ingress", path)
		}
	}
}
