// Author: Sunil Jain, 2026-10-28.
// F3.3: typed signed scans, immutable command challenges, fail-closed admission.
package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

const wipTestUUID1 = "11111111-1111-4111-8111-111111111111"
const wipTestUUID2 = "22222222-2222-4222-8222-222222222222"
const wipTestUUID3 = "33333333-3333-4333-8333-333333333333"

func sampleWIPMove() wipMoveCommand {
	return wipMoveCommand{CommandID: "wip-movement-00001", PlantID: "plant-A", PartID: wipTestUUID1, RouteID: wipTestUUID2, ScanID: wipTestUUID3, Action: "DEPART", StationID: "cutting-out"}
}
func TestWIPMoveSchemaAndChallenge(t *testing.T) {
	base := sampleWIPMove()
	if !wipValidMove(base) {
		t.Fatal("valid transfer rejected")
	}
	orig := wipMoveChallenge(base)
	for _, change := range []func(*wipMoveCommand){
		func(c *wipMoveCommand) { c.PlantID = "plant-B" },
		func(c *wipMoveCommand) { c.StationID = "sewing-in" },
		func(c *wipMoveCommand) { c.RouteID = wipTestUUID3 },
		func(c *wipMoveCommand) { c.ScanID = wipTestUUID1 },
		func(c *wipMoveCommand) { c.Action = "ARRIVE" },
		func(c *wipMoveCommand) { c.PartID = wipTestUUID2 },
	} {
		c := base
		change(&c)
		if wipMoveChallenge(c) == orig {
			t.Fatal("challenge not bound to move fields")
		}
	}
	for _, change := range []func(*wipMoveCommand){
		func(c *wipMoveCommand) { c.CommandID = "bad" },
		func(c *wipMoveCommand) { c.PlantID = "bad plant" },
		func(c *wipMoveCommand) { c.StationID = "bad\nstation" },
		func(c *wipMoveCommand) { c.Action = "CREDIT" },
		func(c *wipMoveCommand) { c.ScanID = "not-a-uuid" },
	} {
		c := base
		change(&c)
		if wipValidMove(c) {
			t.Fatal("invalid move allowed", c)
		}
	}
}
func TestWIPSignedScanBytesAndMutation(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	base := wipScan{Version: wipScanVersion, TenantID: "tenant-A", PlantID: "plant-A", GatewayID: "gateway-A", StationID: "cutting-out", EventID: "observed-event-00001", PartID: wipTestUUID1, CarrierID: wipTestUUID2, EPCHex: "ABCDEF0123456789ABCDEF01", ObservedAt: time.Now().UTC().Format(time.RFC3339Nano)}
	if !wipValidScan(base, time.Now().UTC()) {
		t.Fatal("valid signed scan payload rejected")
	}
	sig := ed25519.Sign(priv, wipScanBytes(base))
	if !ed25519.Verify(pub, wipScanBytes(base), sig) {
		t.Fatal("signature mismatch")
	}
	for _, f := range []func(*wipScan){
		func(c *wipScan) { c.PlantID = "plant-B" },
		func(c *wipScan) { c.CarrierID = wipTestUUID3 },
		func(c *wipScan) { c.PartID = wipTestUUID3 },
		func(c *wipScan) { c.StationID = "sewing-in" },
		func(c *wipScan) { c.EPCHex = "ABCDEF0123456789ABCDEF02" },
		func(c *wipScan) { c.ObservedAt = time.Now().UTC().Add(time.Minute).Format(time.RFC3339Nano) },
	} {
		v := base
		f(&v)
		if ed25519.Verify(pub, wipScanBytes(v), sig) {
			t.Fatal("tampered scan verified")
		}
	}
	if hex.EncodeToString(sig) == "" {
		t.Fatal("missing Ed25519 signature")
	}
	for _, f := range []func(*wipScan){
		func(c *wipScan) { c.Version = "WIP_GATEWAY_SCAN_V0" },
		func(c *wipScan) { c.EPCHex = "not hex" },
		func(c *wipScan) { c.ObservedAt = "yesterday" },
		func(c *wipScan) { c.PartID = "" },
		func(c *wipScan) { c.EventID = "evt\n" },
	} {
		v := base
		f(&v)
		if wipValidScan(v, time.Now().UTC()) {
			t.Fatal("invalid scan accepted", v)
		}
	}
}
func TestWIPFailClosedInDevelopment(t *testing.T) {
	t.Setenv("ENV", "development")
	t.Setenv("AGIPAREL_WIP_PRODUCTION_ENABLED", "true")
	t.Setenv("AGIPAREL_GENEALOGY_PRODUCTION_ENABLED", "true")
	t.Setenv("AGIPAREL_SEWING_PRODUCTION_ENABLED", "true")
	t.Setenv("AGIPAREL_CUTTING_RECONCILIATION_ENABLED", "true")
	p := newWIPAPI(&EdgeServer{})
	if p.enabled {
		t.Fatal("WIP production must be disabled in development")
	}
	for _, h := range []func(http.ResponseWriter, *http.Request){p.scan, p.challenge, p.move, p.history, p.metrics} {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/wip/production/scans", strings.NewReader("{}"))
		h(w, req)
		if w.Code != 503 && w.Code != 405 {
			t.Fatalf("expected fail-closed, got %d", w.Code)
		}
	}
}
func TestWIPMaxAgeConfig(t *testing.T) {
	t.Setenv("AGIPAREL_WIP_MAX_SCAN_AGE", "17m")
	d, err := wipScanMaxAge()
	if err != nil || d != 17*time.Minute {
		t.Fatal("configured scan age not respected")
	}
	for _, v := range []string{"0s", "99h", "invalid"} {
		t.Setenv("AGIPAREL_WIP_MAX_SCAN_AGE", v)
		if _, e := wipScanMaxAge(); e == nil {
			t.Fatalf("unsafe age %s accepted", v)
		}
	}
}
