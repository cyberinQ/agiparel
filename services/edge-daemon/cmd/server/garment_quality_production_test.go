// Author: Sunil Jain, 2026-10-28.
// F3.4 fail-closed garment-level QC command and challenge tests.
package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func sampleGarmentQC() garmentQCCommand {
	return garmentQCCommand{CommandID: "qc-garment-00001", PlantID: "plant-01",
		GarmentID: "11111111-1111-4111-8111-111111111111", SpecID: "22222222-2222-4222-8222-222222222222",
		Action: "INSPECT_PASS", MeasurementsMM: map[string]int64{"chest": 1010, "sleeve": 610},
		Note: "Measured from approved garment revision"}
}
func TestF34ValidatedActionsAndRoles(t *testing.T) {
	cmd := sampleGarmentQC()
	if !garmentQCValid(cmd) {
		t.Fatal("valid pass rejected")
	}
	if permission, role, ok := garmentQCAction(cmd.Action); !ok || permission != "quality.inspect" || role != "quality_inspector" {
		t.Fatal("wrong inspect role")
	}
	for action, want := range map[string]string{"REPAIR_COMPLETE": "quality.repair", "FINISHING_COMPLETE": "quality.finish", "QUALITY_RELEASE": "quality.release"} {
		p, _, ok := garmentQCAction(action)
		if !ok || p != want {
			t.Fatalf("%s permission wrong", action)
		}
	}
	rejected := []func(*garmentQCCommand){
		func(c *garmentQCCommand) { c.PlantID = "bad plant" },
		func(c *garmentQCCommand) { c.CommandID = "short" },
		func(c *garmentQCCommand) { c.GarmentID = "not-uuid" },
		func(c *garmentQCCommand) { c.SpecID = "not-uuid" },
		func(c *garmentQCCommand) { c.Action = "SHIPPING_RELEASE" },
		func(c *garmentQCCommand) { c.Action = "INSPECT_HOLD"; c.DefectCode = ""; c.MeasurementsMM = nil },
		func(c *garmentQCCommand) { c.Action = "INSPECT_PASS"; c.DefectCode = "hole" },
		func(c *garmentQCCommand) { c.MeasurementsMM = map[string]int64{"chest": -1} },
		func(c *garmentQCCommand) { c.MeasurementsMM = map[string]int64{"chest": 1010, "bad measurement": 610} },
		func(c *garmentQCCommand) {
			c.Action = "QUALITY_RELEASE"
			c.MeasurementsMM = map[string]int64{"chest": 610}
		},
		func(c *garmentQCCommand) { c.Note = "short" },
	}
	for i, change := range rejected {
		c := sampleGarmentQC()
		change(&c)
		if garmentQCValid(c) {
			t.Fatalf("invalid variant %d accepted", i)
		}
	}
	held := sampleGarmentQC()
	held.Action = "INSPECT_HOLD"
	held.MeasurementsMM = nil
	held.DefectCode = "hole"
	if !garmentQCValid(held) {
		t.Fatal("valid hold rejected")
	}
}
func TestF34ChallengeCoversEverything(t *testing.T) {
	c := sampleGarmentQC()
	orig := garmentQCChallenge(c)
	mutations := []func(*garmentQCCommand){
		func(v *garmentQCCommand) { v.PlantID = "plant-02" },
		func(v *garmentQCCommand) { v.SpecID = "33333333-3333-4333-8333-333333333333" },
		func(v *garmentQCCommand) { v.Action = "REINSPECT_PASS" },
		func(v *garmentQCCommand) { v.MeasurementsMM = map[string]int64{"chest": 1011, "sleeve": 610} },
		func(v *garmentQCCommand) { v.Note = "Approved specification changed after issue" },
		func(v *garmentQCCommand) { v.CommandID = "qc-garment-00002" },
	}
	for i, fn := range mutations {
		v := sampleGarmentQC()
		fn(&v)
		if garmentQCChallenge(v) == orig {
			t.Fatalf("unbound field %d", i)
		}
	}
}
func TestF34DefaultsClosed(t *testing.T) {
	t.Setenv("AGIPAREL_GARMENT_QUALITY_ENABLED", "false")
	p := newGarmentQCAPI(&EdgeServer{})
	r := httptest.NewRequest(http.MethodPost, "/api/v1/quality/garments/challenge", strings.NewReader("{}"))
	w := httptest.NewRecorder()
	p.challenge(w, r)
	if w.Code != 503 {
		t.Fatalf("expected fail closed; got %d", w.Code)
	}
}
