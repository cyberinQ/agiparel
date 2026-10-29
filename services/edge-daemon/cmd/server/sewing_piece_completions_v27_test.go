// Author: Sunil Jain, CyberinQ LLC, 2026-10-29.
package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func exampleSewPieceCommand() sewPieceCommand {
	return sewPieceCommand{CommandID: "PIECE_SEW_12345678", PlantID: "PLANT-01", StationID: "SEW-07",
		GarmentID: "c6e63b80-e150-4364-90be-759755e3ec46", RouteID: "0f629944-1fcf-49e4-b50d-b1fca541c241",
		ScanIDs: []string{"0e629944-1fcf-49e4-b50d-b1fca541c242", "10ff1234-1fcf-49e4-b50d-b1fca541c243"}}
}
func TestF316BValidationAndChallengeBinding(t *testing.T) {
	c := exampleSewPieceCommand()
	if !sewPieceValid(c) {
		t.Fatal("valid piece command rejected")
	}
	base := sewPieceChallenge(c)
	if !strings.HasPrefix(base, "SEW_PIECE_COMPLETE_V27:") || len(base) != len("SEW_PIECE_COMPLETE_V27:")+64 {
		t.Fatal("incorrect challenge")
	}
	for _, change := range []func(*sewPieceCommand){
		func(x *sewPieceCommand) { x.CommandID = "short" },
		func(x *sewPieceCommand) { x.PlantID = "other-plant" },
		func(x *sewPieceCommand) { x.StationID = "other-station" },
		func(x *sewPieceCommand) { x.GarmentID = "bad" },
		func(x *sewPieceCommand) { x.RouteID = "bad" },
		func(x *sewPieceCommand) { x.ScanIDs[0] = "bad" },
	} {
		other := c
		other.ScanIDs = append([]string(nil), c.ScanIDs...)
		change(&other)
		if sewPieceValid(other) && sewPieceChallenge(other) == base {
			t.Fatal("changed accepted command did not change challenge")
		}
	}
	x := c
	x.ScanIDs = []string{c.ScanIDs[0], strings.ToUpper(c.ScanIDs[0])}
	if sewPieceValid(x) {
		t.Fatal("case-insensitive scan replay must fail")
	}
	x = c
	x.ScanIDs = nil
	if sewPieceValid(x) {
		t.Fatal("no evidence should fail")
	}
	x = c
	x.ScanIDs = make([]string, 33)
	for i := range x.ScanIDs {
		x.ScanIDs[i] = c.ScanIDs[0]
	}
	if sewPieceValid(x) {
		t.Fatal("unbounded evidence should fail")
	}
	x = c
	x.ScanIDs = append([]string(nil), c.ScanIDs...)
	x.ScanIDs[0], x.ScanIDs[1] = x.ScanIDs[1], x.ScanIDs[0]
	if sewPieceChallenge(x) == base {
		t.Fatal("scan ordering should remain signed")
	}
}
func TestF316BFeatureFailsClosed(t *testing.T) {
	t.Setenv("AGIPAREL_SEW_PIECE_COMPLETIONS_ENABLED", "true")
	t.Setenv("ENV", "development")
	p := newSewPieceAPI(&EdgeServer{})
	if p.enabled {
		t.Fatal("development must not have production authority")
	}
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/sewing/pieces/completions/challenge", strings.NewReader("{}"))
	req.Header.Set("Content-Type", "application/json")
	p.challenge(w, req)
	if w.Code != 503 {
		t.Fatalf("expected disabled 503, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "ERR_F316B_DISABLED") {
		t.Fatal("incorrect fail-closed result")
	}
}
func TestF316BChapterPermissions(t *testing.T) {
	for _, permission := range []string{"sewing.piece.hold", "sewing.piece.review", "sewing.piece.complete"} {
		grant, ok := chapterPermissions[permission]
		if !ok || grant.Chapter != 6 || grant.Authority != "SIGNED_WRITE" {
			t.Fatalf("missing tenant-scoped permission %s", permission)
		}
	}
}
