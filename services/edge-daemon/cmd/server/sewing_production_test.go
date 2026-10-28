// Author: Sunil Jain, 2026-10-28.
// Production sewing contracts: enforce signed reader provenance, worker challenge and opt-in.
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

func sewProdExampleEvent() sewProdObservation {
	return sewProdObservation{Version: sewProdVersion, TenantID: "TENANT-A", PlantID: "PLANT-A", GatewayID: "READER-A", StationID: "SEW-07", EventID: "EVENT-123456789012", LayID: exampleLayID, LabelCode: "CB08-AAAAAAAAAAAAAAAAAAAAAAAA", EPCHex: "AABBCCDDEEFF001122334455", ObservedAt: time.Now().UTC().Format(time.RFC3339Nano)}
}
func TestSewProductionReaderSignatureAndTamper(t *testing.T) {
	pub, priv, e := ed25519.GenerateKey(rand.Reader)
	if e != nil {
		t.Fatal(e)
	}
	v := sewProdExampleEvent()
	if e = sewProdValidObservation(v, time.Now()); e != nil {
		t.Fatal(e)
	}
	signature := hex.EncodeToString(ed25519.Sign(priv, sewProdEventBytes(v)))
	if !sewProdSigned(v, signature, pub) {
		t.Fatal("enrolled gateway should verify")
	}
	v.StationID = "SEW-08"
	if sewProdSigned(v, signature, pub) {
		t.Fatal("station tampering must fail")
	}
	v = sewProdExampleEvent()
	v.EPCHex = "AABBCCDDEEFF001122334456" // same shape, different EPC
	if sewProdSigned(v, signature, pub) {
		t.Fatal("RF observation tampering must fail")
	}
}
func TestSewProductionCompletionChallengeBindsEveryField(t *testing.T) {
	c := sewProdCompleteCommand{CommandID: "COMPLETE-123456789", LayID: exampleLayID, PlantID: "PLANT-A", StationID: "SEW-07", RouteID: exampleLayID, MarkerPosition: 4, PlyGroup: 2, ObservationIDs: []string{exampleLayID}}
	if e := sewProdValidCommand(c); e != nil {
		t.Fatal(e)
	}
	before := sewProdChallenge(c)
	copy := c
	copy.StationID = "SEW-08"
	if before == sewProdChallenge(copy) {
		t.Fatal("station identity must be in attested challenge")
	}
	copy = c
	copy.ObservationIDs = []string{"87654321-4321-4321-4321-123456789abc"}
	if before == sewProdChallenge(copy) {
		t.Fatal("observation lineage must be in challenge")
	}
	copy = c
	copy.ObservationIDs = []string{exampleLayID, exampleLayID}
	if sewProdValidCommand(copy) == nil {
		t.Fatal("reusing one observation for multiple required parts rejected")
	}
}
func TestSewProductionRejectsInvalidObservationAndEnvironment(t *testing.T) {
	v := sewProdExampleEvent()
	v.ObservedAt = time.Now().Add(5 * time.Minute).UTC().Format(time.RFC3339Nano)
	if sewProdValidObservation(v, time.Now()) == nil {
		t.Fatal("future readings forbidden")
	}
	v = sewProdExampleEvent()
	v.EPCHex = "notanepc"
	if sewProdValidObservation(v, time.Now()) == nil {
		t.Fatal("untrustworthy EPC rejected")
	}
	t.Setenv("AGIPAREL_SEWING_PRODUCTION_ENABLED", "true")
	t.Setenv("ENV", "development")
	p := newSewProdAPI(&EdgeServer{})
	if p.enabled {
		t.Fatal("development cannot record production evidence")
	}
	r := httptest.NewRequest(http.MethodPost, "/api/v1/sewing/production/observations", strings.NewReader("{}"))
	w := httptest.NewRecorder()
	p.observation(w, r)
	if w.Code != 503 {
		t.Fatalf("fail-closed environment expected 503: %d", w.Code)
	}
}
func TestSewProductionMaxObservationAgeConfiguration(t *testing.T) {
	t.Setenv("AGIPAREL_SEWING_MAX_OBSERVATION_AGE", "12h")
	d, e := sewProdAgeLimit()
	if e != nil || d != 12*time.Hour {
		t.Fatal(d, e)
	}
	t.Setenv("AGIPAREL_SEWING_MAX_OBSERVATION_AGE", "0s")
	if _, e = sewProdAgeLimit(); e == nil {
		t.Fatal("must reject unsafe config")
	}
}

func TestSewProdRejectsUntrustedJSON(t *testing.T) {
	for _, tc := range []struct{ body, kind string }{
		{`{"event":{},"oops":true}`, "application/json"},
		{`{} {}`, "application/json"},
		{`{}`, "text/plain"},
	} {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPost, "/api/v1/sewing/production/observations", strings.NewReader(tc.body))
		r.Header.Set("Content-Type", tc.kind)
		var v sewProdObservationEnvelope
		if sewProdStrictJSON(w, r, &v) {
			t.Fatal("untrusted input accepted", tc)
		}
		if w.Code != 400 && w.Code != 415 {
			t.Fatal("wrong error status", w.Code)
		}
	}
}
