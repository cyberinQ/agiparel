// Author: Sunil Jain, 2026-10-28. Signed carton and dispatch validation tests.
package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const testSpecID = "22222222-2222-4222-8222-222222222222"
const testCartonID = "11111111-1111-4111-8111-111111111111"

func sampleF35Carton() packUnitCommand {
	return packUnitCommand{CommandID: "pack-carton-000001", PlantID: "plant-01", CartonID: testCartonID,
		SpecID: testSpecID, Action: "OPEN", CartonCode: "BOX-001", Note: "Factory carton documented"}
}
func sampleF35Manifest() packManifestCommand {
	return packManifestCommand{CommandID: "pack-manifest-0001", PlantID: "plant-01",
		ManifestID: "33333333-3333-4333-8333-333333333333", SpecID: testSpecID, Action: "OPEN",
		ManifestRef: "SHIP-001", DestinationRef: "customer-dock", Note: "Prepared dispatch manifest"}
}
func TestF35CartonCommands(t *testing.T) {
	c := sampleF35Carton()
	if !packUnitValid(c) {
		t.Fatal("valid carton refused")
	}
	for _, f := range []func(*packUnitCommand){
		func(c *packUnitCommand) { c.CartonID = "bad" }, func(c *packUnitCommand) { c.Action = "SHIP" },
		func(c *packUnitCommand) { c.GarmentID = testCartonID }, func(c *packUnitCommand) { c.CommandID = "bad" },
		func(c *packUnitCommand) { c.ObservedG = 10 }, func(c *packUnitCommand) { c.Note = "short" },
	} {
		v := c
		f(&v)
		if packUnitValid(v) {
			t.Fatalf("accepted invalid command %+v", v)
		}
	}
	add := c
	add.Action = "ADD"
	add.CartonCode = ""
	add.GarmentID = "44444444-4444-4444-8444-444444444444"
	if !packUnitValid(add) {
		t.Fatal("valid add refused")
	}
	seal := c
	seal.Action = "SEAL"
	seal.CartonCode = ""
	seal.ObservedG = 1230
	seal.ExpectedG = 1200
	seal.ScaleReference = "SCALE-D7"
	if !packUnitValid(seal) {
		t.Fatal("valid seal refused")
	}
	if p, r, ok := packUnitPermission(seal.Action); !ok || p != "packing.seal" || r != "packing_supervisor" {
		t.Fatal("seal role wrong")
	}
	baseline := packUnitChallenge(seal)
	for _, f := range []func(*packUnitCommand){
		func(c *packUnitCommand) { c.ObservedG++ }, func(c *packUnitCommand) { c.ExpectedG++ },
		func(c *packUnitCommand) { c.ScaleReference = "SCALE-D8" }, func(c *packUnitCommand) { c.SpecID = testCartonID },
		func(c *packUnitCommand) { c.CommandID = "pack-carton-000002" },
	} {
		v := seal
		f(&v)
		if packUnitChallenge(v) == baseline {
			t.Fatal("unsigned carton mutation")
		}
	}
}
func TestF35ManifestCommands(t *testing.T) {
	c := sampleF35Manifest()
	if !packManifestValid(c) {
		t.Fatal("valid manifest refused")
	}
	add := c
	add.Action = "ADD_CARTON"
	add.ManifestRef = ""
	add.DestinationRef = ""
	add.CartonID = testCartonID
	if !packManifestValid(add) {
		t.Fatal("valid carton link refused")
	}
	stage := add
	stage.Action = "STAGE"
	stage.CartonID = ""
	if !packManifestValid(stage) {
		t.Fatal("valid staged manifest refused")
	}
	if p, r, ok := packManifestPermission(stage.Action); !ok || p != "packing.dispatch_stage" || r != "dispatch_supervisor" {
		t.Fatal("stage role wrong")
	}
	for _, f := range []func(*packManifestCommand){
		func(c *packManifestCommand) { c.Action = "SHIP" }, func(c *packManifestCommand) { c.DestinationRef = "" },
		func(c *packManifestCommand) { c.CartonID = testCartonID },
	} {
		v := c
		f(&v)
		if packManifestValid(v) {
			t.Fatal("invalid manifest accepted")
		}
	}
	if packManifestChallenge(c) == packManifestChallenge(add) {
		t.Fatal("manifest challenge not bound to carton")
	}
}
func TestF35DisabledByDefault(t *testing.T) {
	t.Setenv("AGIPAREL_CARTON_GENEALOGY_ENABLED", "false")
	p := newCartonGenealogyAPI(&EdgeServer{})
	for _, handler := range []func(http.ResponseWriter, *http.Request){p.unitChallenge, p.manifestChallenge} {
		w := httptest.NewRecorder()
		r := httptest.NewRequest("POST", "/", strings.NewReader("{}"))
		handler(w, r)
		if w.Code != 503 {
			t.Fatalf("expected disabled 503, got %d", w.Code)
		}
	}
}
