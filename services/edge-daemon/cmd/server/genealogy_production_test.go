// Author: Sunil Jain, 2026-10-28.
package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestF31PermanentIdentityCommandsAreStrict(t *testing.T) {
	valid := genReserveCommand{CommandID: "PlySerialClaim0001", PlantID: "plant-A", ReleaseID: "11111111-1111-4111-8111-111111111111", LayID: "22222222-2222-4222-8222-222222222222", MarkerPosition: 1, PlyNumber: 1}
	if !genValidReserve(valid) {
		t.Fatal("valid serial reservation rejected")
	}
	cases := []genReserveCommand{
		{CommandID: "bad", PlantID: valid.PlantID, ReleaseID: valid.ReleaseID, LayID: valid.LayID, MarkerPosition: 1, PlyNumber: 1},
		{CommandID: valid.CommandID, PlantID: "../../X", ReleaseID: valid.ReleaseID, LayID: valid.LayID, MarkerPosition: 1, PlyNumber: 1},
		{CommandID: valid.CommandID, PlantID: valid.PlantID, ReleaseID: "invalid", LayID: valid.LayID, MarkerPosition: 1, PlyNumber: 1},
		{CommandID: valid.CommandID, PlantID: valid.PlantID, ReleaseID: valid.ReleaseID, LayID: valid.LayID, MarkerPosition: 0, PlyNumber: 1},
		{CommandID: valid.CommandID, PlantID: valid.PlantID, ReleaseID: valid.ReleaseID, LayID: valid.LayID, MarkerPosition: 1, PlyNumber: -1},
	}
	for i, c := range cases {
		if genValidReserve(c) {
			t.Fatalf("invalid reserve %d accepted", i)
		}
	}
	changed := valid
	changed.PlyNumber++
	if genReserveChallenge(valid) == genReserveChallenge(changed) {
		t.Fatal("ply changes must change challenge")
	}
	if strings.Contains(genReserveChallenge(valid), "garment_id") {
		t.Fatal("challenge must not manufacture serial")
	}
}
func TestF31TagLifecycleIsIndependentOfIdentity(t *testing.T) {
	c := genCarrierCommand{CommandID: "CarrierBinding0001", PlantID: "plant-A", CarrierID: "11111111-1111-4111-8111-111111111111", Action: "BIND", TargetType: "CUT_PART", TargetID: "22222222-2222-4222-8222-222222222222"}
	if !genValidCarrier(c) {
		t.Fatal("valid tag binding rejected")
	}
	retired := c
	retired.Action = "RETIRE"
	if !genValidCarrier(retired) || genCarrierChallenge(c) == genCarrierChallenge(retired) {
		t.Fatal("BIND and RETIRE must be distinct signed commands")
	}
	wrong := c
	wrong.TargetType = "ROLL"
	if genValidCarrier(wrong) {
		t.Fatal("unimplemented cross-chapter tag adoption allowed")
	}
	wrong = c
	wrong.Action = "UPDATE"
	if genValidCarrier(wrong) {
		t.Fatal("mutable tag binding action allowed")
	}
	wrong = c
	wrong.TargetID = "xyz"
	if genValidCarrier(wrong) {
		t.Fatal("invalid target allowed")
	}
}
func TestF31EndpointFailsClosed(t *testing.T) {
	p := &genProductionAPI{reason: "ERR_GEN_DISABLED"}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/genealogy/units?id=bad", nil)
	w := httptest.NewRecorder()
	p.trace(w, req)
	if w.Code != 503 || !strings.Contains(w.Body.String(), "ERR_GEN_DISABLED") {
		t.Fatalf("fail closed result: %d %s", w.Code, w.Body.String())
	}
	req = httptest.NewRequest(http.MethodPost, "/api/v1/genealogy/units", strings.NewReader(`{"command":{}}`))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	p.reserve(w, req)
	if w.Code != 503 {
		t.Fatalf("reservation reachable without config: %d", w.Code)
	}
}
func TestF31PaginationBounds(t *testing.T) {
	for _, raw := range []string{"-1", "0", "101", "nope"} {
		if _, e := genPageLimit(raw); e == nil {
			t.Fatalf("accepted limit %q", raw)
		}
	}
	v, e := genPageLimit("")
	if e != nil || v != 50 {
		t.Fatalf("bad default %d %v", v, e)
	}
	v, e = genPageLimit("100")
	if e != nil || v != 100 {
		t.Fatalf("bad maximum %d %v", v, e)
	}
}
