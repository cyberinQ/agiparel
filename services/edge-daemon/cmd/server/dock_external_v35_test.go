// Author: Sunil Jain, CyberinQ LLC, 2026-10-29.
// F3.22 external signing, replay binding, policy and deny-by-default tests.
package main

import (
	"crypto/ed25519"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func testDockExternalV35(kind string) dockExternalEventV35 {
	e := dockExternalEventV35{Version: "V35", TenantID: "tenant-A", PlantID: "PLANT-01", ManifestID: "11111111-1111-4111-8111-111111111111",
		ASNDraftID: "22222222-2222-4222-8222-222222222222", SignerID: "CARRIER-GW-01", PartyKind: "CARRIER", PartyCode: "CARRIER-A",
		AssertionKind: "CARRIER_ASN_ACK", Decision: "ACKNOWLEDGED", EventID: "external-event-20261029-0001",
		ExternalReference: "ACK-EXTERNAL-01", ASNReference: "ASN-2026-001", PayloadSHA256: strings.Repeat("a", 64),
		IssuedAt: time.Now().UTC().Format(time.RFC3339Nano)}
	if kind == "BUYER_LOT_REVIEW" {
		e.SignerID = "BUYER-GW-01"
		e.PartyKind = "BUYER"
		e.PartyCode = "BUYER-A"
		e.AssertionKind = kind
		e.Decision = "REVIEWED_ACCEPT"
	}
	return e
}
func TestExternalV35ValidAndTamper(t *testing.T) {
	for _, kind := range []string{"CARRIER_ASN_ACK", "BUYER_LOT_REVIEW"} {
		e := testDockExternalV35(kind)
		if !dockExternalValidV35(e, time.Now().UTC()) {
			t.Fatalf("valid %s rejected", kind)
		}
		pub, priv, err := ed25519.GenerateKey(nil)
		if err != nil {
			t.Fatal(err)
		}
		sig := ed25519.Sign(priv, dockExternalBytesV35(e))
		if !ed25519.Verify(pub, dockExternalBytesV35(e), sig) {
			t.Fatal("valid signature rejected")
		}
		changes := []func(*dockExternalEventV35){
			func(e *dockExternalEventV35) { e.TenantID = "tenant-B" },
			func(e *dockExternalEventV35) { e.ManifestID = "33333333-3333-4333-8333-333333333333" },
			func(e *dockExternalEventV35) { e.ASNDraftID = "33333333-3333-4333-8333-333333333333" },
			func(e *dockExternalEventV35) { e.PayloadSHA256 = strings.Repeat("b", 64) },
			func(e *dockExternalEventV35) { e.Decision = "REVIEWED_REJECT" },
			func(e *dockExternalEventV35) { e.ExternalReference = "different-reference" },
			func(e *dockExternalEventV35) { e.PartyCode = "PARTY-B" },
		}
		for i, change := range changes {
			mutated := e
			change(&mutated)
			if ed25519.Verify(pub, dockExternalBytesV35(mutated), sig) {
				t.Fatalf("tamper %d survived", i)
			}
		}
	}
}
func TestExternalV35FailClosedPolicy(t *testing.T) {
	e := testDockExternalV35("CARRIER_ASN_ACK")
	e.Decision = "REVIEWED_ACCEPT"
	if dockExternalValidV35(e, time.Now()) {
		t.Fatal("carrier forged buyer decision")
	}
	e = testDockExternalV35("BUYER_LOT_REVIEW")
	e.Decision = "ACKNOWLEDGED"
	if dockExternalValidV35(e, time.Now()) {
		t.Fatal("buyer forged carrier ack")
	}
	e = testDockExternalV35("BUYER_LOT_REVIEW")
	e.AssertionKind = "SHIPMENT_CREDIT"
	if dockExternalValidV35(e, time.Now()) {
		t.Fatal("shipment posting admitted")
	}
	e = testDockExternalV35("BUYER_LOT_REVIEW")
	e.IssuedAt = time.Now().Add(-25 * time.Hour).Format(time.RFC3339Nano)
	if dockExternalValidV35(e, time.Now()) {
		t.Fatal("expired proof accepted")
	}
	e = testDockExternalV35("BUYER_LOT_REVIEW")
	e.IssuedAt = time.Now().Add(3 * time.Minute).Format(time.RFC3339Nano)
	if dockExternalValidV35(e, time.Now()) {
		t.Fatal("future proof accepted")
	}
	e = testDockExternalV35("CARRIER_ASN_ACK")
	e.ExternalReference = "ACK\nINJECTION"
	if dockExternalValidV35(e, time.Now()) {
		t.Fatal("unexpected control character accepted")
	}
}
func TestExternalV35DisabledByDefault(t *testing.T) {
	t.Setenv("AGIPAREL_DOCK_EXTERNAL_V35_ENABLED", "false")
	p := newDockExternalAPIv35(&EdgeServer{})
	if p.enabled {
		t.Fatal("external workflow silently enabled")
	}
	for _, method := range []string{http.MethodPost, http.MethodGet} {
		handler := p.receive
		if method == http.MethodGet {
			handler = p.status
		}
		req := httptest.NewRequest(method, "/api/v1/packing/dock/external-assertions", nil)
		w := httptest.NewRecorder()
		handler(w, req)
		if w.Code != http.StatusServiceUnavailable {
			t.Fatalf("disabled %s status %d", method, w.Code)
		}
	}
}
