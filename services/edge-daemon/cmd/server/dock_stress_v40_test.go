// Author: Sunil Jain, 2026-10-29.
// V40 cryptographic mutation, freshness and strict admission-envelope tests.
package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestV40DockGatewayEverySignedFieldBound(t *testing.T) {
	now := time.Now().UTC()
	e := dockScanV34{Version: "V34", TenantID: "tenant-one", PlantID: "PLANT-01", ManifestID: "11111111-1111-4111-8111-111111111111",
		CartonID: "22222222-2222-4222-8222-222222222222", CartonCode: "CTN-001", GatewayID: "DOCK-GW-01",
		EventID: "dockscan-event-0001", ObservedAt: now.Format(time.RFC3339Nano)}
	if !dockScanValidV34(e, now) {
		t.Fatal("baseline gateway event rejected")
	}
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	original := dockScanBytesV34(e)
	sig := ed25519.Sign(priv, original)
	if !ed25519.Verify(pub, original, sig) {
		t.Fatal("original signature invalid")
	}
	changes := []func(*dockScanV34){
		func(x *dockScanV34) { x.Version = "V35" }, func(x *dockScanV34) { x.TenantID = "tenant-two" },
		func(x *dockScanV34) { x.PlantID = "PLANT-02" }, func(x *dockScanV34) { x.ManifestID = "33333333-3333-4333-8333-333333333333" },
		func(x *dockScanV34) { x.CartonID = "44444444-4444-4444-8444-444444444444" },
		func(x *dockScanV34) { x.CartonCode = "CTN-002" }, func(x *dockScanV34) { x.GatewayID = "DOCK-GW-02" },
		func(x *dockScanV34) { x.EventID = "dockscan-event-0002" },
		func(x *dockScanV34) { x.ObservedAt = now.Add(-time.Minute).Format(time.RFC3339Nano) },
	}
	for i, mutate := range changes {
		x := e
		mutate(&x)
		if ed25519.Verify(pub, dockScanBytesV34(x), sig) {
			t.Fatalf("gateway tamper %d accepted", i)
		}
	}
	if ed25519.Verify(pub, append([]byte("AGIPAREL_WRONG_DOMAIN\n"), original...), sig) {
		t.Fatal("cross-domain replay")
	}
	for _, delta := range []time.Duration{-16 * time.Minute, 3 * time.Minute} {
		x := e
		x.ObservedAt = now.Add(delta).Format(time.RFC3339Nano)
		if dockScanValidV34(x, now) {
			t.Fatal("stale/future scan admitted")
		}
	}
}

func TestV40ExternalEverySignedFieldBound(t *testing.T) {
	e := testDockExternalV35("CARRIER_ASN_ACK")
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	sig := ed25519.Sign(priv, dockExternalBytesV35(e))
	changes := []func(*dockExternalEventV35){
		func(x *dockExternalEventV35) { x.Version = "V34" }, func(x *dockExternalEventV35) { x.TenantID = "tenant-other" },
		func(x *dockExternalEventV35) { x.PlantID = "PLANT-OTHER" },
		func(x *dockExternalEventV35) { x.ManifestID = "33333333-3333-4333-8333-333333333333" },
		func(x *dockExternalEventV35) { x.ASNDraftID = "33333333-3333-4333-8333-333333333333" },
		func(x *dockExternalEventV35) { x.SignerID = "OTHER-GATEWAY" }, func(x *dockExternalEventV35) { x.PartyKind = "BUYER" },
		func(x *dockExternalEventV35) { x.PartyCode = "OTHER-PARTY" }, func(x *dockExternalEventV35) { x.AssertionKind = "BUYER_LOT_REVIEW" },
		func(x *dockExternalEventV35) { x.Decision = "REVIEWED_REJECT" }, func(x *dockExternalEventV35) { x.EventID = "external-event-20261029-0002" },
		func(x *dockExternalEventV35) { x.ExternalReference = "OTHER-REF-001" }, func(x *dockExternalEventV35) { x.ASNReference = "OTHER-ASN-001" },
		func(x *dockExternalEventV35) { x.PayloadSHA256 = strings.Repeat("b", 64) },
		func(x *dockExternalEventV35) {
			x.IssuedAt = time.Now().UTC().Add(-time.Minute).Format(time.RFC3339Nano)
		},
	}
	for i, mutate := range changes {
		x := e
		mutate(&x)
		if ed25519.Verify(pub, dockExternalBytesV35(x), sig) {
			t.Fatalf("external tamper %d accepted", i)
		}
	}
	if !dockExternalValidV35(e, time.Now().UTC()) {
		t.Fatal("valid external event rejected")
	}
}

func TestV40StrictJSONRejectsPayloadSmuggling(t *testing.T) {
	cases := []struct {
		payload, ct string
		want        bool
	}{
		{`{"event":{},"signature_hex":"abcd","unexpected":true}`, "application/json", false},
		{`{"event":{},"signature_hex":"abcd"} {}`, "application/json", false},
		{`{"event":{},"signature_hex":"abcd"}`, "text/plain", false},
		{`{"event":{},"signature_hex":"abcd"}`, "application/json", true},
		{`{"event":{},"signature_hex":"` + strings.Repeat("x", 1<<20) + `"}`, "application/json", false},
	}
	for i, tc := range cases {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPost, "/api/v1/packing/dock/external-assertions", strings.NewReader(tc.payload))
		r.Header.Set("Content-Type", tc.ct)
		var dst dockExternalEnvelopeV35
		got := sewProdStrictJSON(w, r, &dst)
		if got != tc.want {
			t.Fatalf("case %d got %v HTTP %d", i, got, w.Code)
		}
	}
}
