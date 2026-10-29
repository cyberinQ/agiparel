// Author: Sunil Jain, CyberinQ LLC, 2026-10-29.
// F3.20 dock boundary: exact actions, immutable challenges and Ed25519 scan framing.
package main

import (
	"crypto/ed25519"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func testDockCommandV34(kind string) dockCommandV34 {
	c := dockCommandV34{Kind: kind, CommandID: "dock-command-v34-0001", PlantID: "PLANT-01",
		ManifestID: "11111111-1111-4111-8111-111111111111", Note: "Independent handover proof recorded on the factory floor"}
	switch kind {
	case "CONTAINER_SEAL":
		c.ContainerRef = "CONTAINER-112"
		c.SealCode = "SEAL-112"
		c.ScannedCartons = 3
	case "CARRIER_RECEIPT":
		c.SealID = "22222222-2222-4222-8222-222222222222"
		c.CarrierCode = "CARRIER-ABC"
		c.ReceiptReference = "RECEIPT-112"
	case "ASN_DRAFT":
		c.CarrierReceiptID = "33333333-3333-4333-8333-333333333333"
		c.ASNReference = "ASN-112"
		c.PayloadSHA256 = strings.Repeat("a", 64)
	}
	return c
}
func TestDockV34ActionsAndTamperBinding(t *testing.T) {
	for _, tc := range []struct{ kind, action, role, path string }{
		{"CONTAINER_SEAL", "packing.dock.seal", "dispatch_supervisor", "/api/v1/packing/dock/seals"},
		{"CARRIER_RECEIPT", "packing.dock.carrier", "carrier_agent", "/api/v1/packing/dock/carrier-receipts"},
		{"ASN_DRAFT", "packing.dock.asn.prepare", "dispatch_coordinator", "/api/v1/packing/dock/asn-drafts"},
	} {
		c := testDockCommandV34(tc.kind)
		action, role, ok := dockCommandPolicyV34(c)
		if !ok || action != tc.action || role != tc.role || !dockPathValidV34(tc.path, tc.kind) || !dockPathValidV34(tc.path+"/challenge", tc.kind) {
			t.Fatalf("invalid dock policy: %+v", tc)
		}
		for _, wrong := range []string{"/api/v1/packing/dock/seals", "/api/v1/packing/dock/carrier-receipts", "/api/v1/packing/dock/asn-drafts"} {
			if wrong != tc.path && dockPathValidV34(wrong, tc.kind) {
				t.Fatalf("cross-workflow %s", wrong)
			}
		}
		before := dockChallengeV34(c)
		c.Note += " changed after signing"
		if before == dockChallengeV34(c) {
			t.Fatal("challenge did not bind note")
		}
		c = testDockCommandV34(tc.kind)
		c.ManifestID = "44444444-4444-4444-8444-444444444444"
		if before == dockChallengeV34(c) {
			t.Fatal("challenge did not bind manifest")
		}
		c = testDockCommandV34(tc.kind)
		c.SealCode = "different"
		if tc.kind == "CONTAINER_SEAL" && before == dockChallengeV34(c) {
			t.Fatal("challenge did not bind seal")
		}
	}
}
func TestDockV34RejectCrossWorkflowAndCredit(t *testing.T) {
	c := testDockCommandV34("CARRIER_RECEIPT")
	c.ScannedCartons = 1
	if _, _, ok := dockCommandPolicyV34(c); ok {
		t.Fatal("carrier received forbidden seal field")
	}
	c = testDockCommandV34("CONTAINER_SEAL")
	c.CarrierCode = "CARRIER-X"
	if _, _, ok := dockCommandPolicyV34(c); ok {
		t.Fatal("seal received carrier field")
	}
	c = testDockCommandV34("ASN_DRAFT")
	c.PayloadSHA256 = "not-a-hash"
	if _, _, ok := dockCommandPolicyV34(c); ok {
		t.Fatal("ASN accepted unsafe hash")
	}
	c = testDockCommandV34("ASN_DRAFT")
	c.Kind = "SHIPMENT_CREDIT"
	if _, _, ok := dockCommandPolicyV34(c); ok {
		t.Fatal("unsupported shipment authority")
	}
}
func TestDockV34GatewaySignatureAndFreshness(t *testing.T) {
	now := time.Now().UTC()
	scan := dockScanV34{Version: "V34", TenantID: "tenant-one", PlantID: "PLANT-01", GatewayID: "DOCK-GATEWAY-1",
		ManifestID: "11111111-1111-4111-8111-111111111111", CartonID: "22222222-2222-4222-8222-222222222222",
		CartonCode: "CTN-101", EventID: "dock-scan-20261029-001", ObservedAt: now.Add(-time.Minute).Format(time.RFC3339Nano)}
	if !dockScanValidV34(scan, now) {
		t.Fatal("valid scan rejected")
	}
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	sig := ed25519.Sign(priv, dockScanBytesV34(scan))
	if !ed25519.Verify(pub, dockScanBytesV34(scan), sig) {
		t.Fatal("valid signature rejected")
	}
	scan.CartonCode = "CTN-102"
	if ed25519.Verify(pub, dockScanBytesV34(scan), sig) {
		t.Fatal("carton swap survived signature")
	}
	scan.CartonCode = "CTN-101"
	scan.TenantID = "tenant-two"
	if ed25519.Verify(pub, dockScanBytesV34(scan), sig) {
		t.Fatal("tenant swap survived signature")
	}
	scan.TenantID = "tenant-one"
	scan.ObservedAt = now.Add(-16 * time.Minute).Format(time.RFC3339Nano)
	if dockScanValidV34(scan, now) {
		t.Fatal("stale dock scan accepted")
	}
	scan.ObservedAt = now.Add(3 * time.Minute).Format(time.RFC3339Nano)
	if dockScanValidV34(scan, now) {
		t.Fatal("future dock scan accepted")
	}
	scan.ObservedAt = now.Format(time.RFC3339Nano)
	scan.CartonCode = "CTN-101"
	scan.GatewayID = "gateway\nforgery"
	if dockScanValidV34(scan, now) {
		t.Fatal("delimiter injection accepted")
	}
}
func TestDockV34FailsClosed(t *testing.T) {
	t.Setenv("AGIPAREL_DOCK_V34_ENABLED", "false")
	p := newDockAPIv34(&EdgeServer{})
	if p.enabled {
		t.Fatal("unconfigured dock enabled")
	}
	for i, endpoint := range []func(http.ResponseWriter, *http.Request){p.scan, p.challenge, p.event, p.status} {
		method := http.MethodPost
		if i == 3 {
			method = http.MethodGet
		}
		req := httptest.NewRequest(method, "/api/v1/packing/dock/seals", nil)
		resp := httptest.NewRecorder()
		endpoint(resp, req)
		if resp.Code != http.StatusServiceUnavailable {
			t.Fatalf("disabled operation not rejected with 503: %d", resp.Code)
		}
	}
}
