// Author: Sunil Jain, CyberinQ LLC, 2026-10-29.
// Tests use local signed envelopes and mock TLS, never the production database.
package main

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

const (
	partID       = "11111111-1111-4111-8111-111111111111"
	carrierID    = "22222222-2222-4222-8222-222222222222"
	serverScanID = "33333333-3333-4333-8333-333333333333"
)

func newSignedEnvelope(t *testing.T, id string, observed time.Time) []byte {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	e := scanEvent{Version: "WIP_GATEWAY_SCAN_V1", TenantID: "tenant-A", PlantID: "plant-A", GatewayID: "gate-A", StationID: "cutting-out", EventID: id, PartID: partID, CarrierID: carrierID, EPCHex: "ABCDEF0123456789ABCDEF01", ObservedAt: observed.UTC().Format(time.RFC3339Nano)}
	signed := []byte("AGIPAREL_WIP_GATEWAY_V1\n" + strings.Join([]string{e.Version, e.TenantID, e.PlantID, e.GatewayID, e.StationID, e.EventID, e.PartID, e.CarrierID, e.EPCHex, e.ObservedAt}, "\n") + "\n")
	b, err := json.Marshal(envelope{Event: e, SignatureHex: hex.EncodeToString(ed25519.Sign(priv, signed))})
	if err != nil {
		t.Fatal(err)
	}
	return b
}
func newState(t *testing.T) state {
	t.Helper()
	s, err := initState(filepath.Join(t.TempDir(), "secure"))
	if err != nil {
		t.Fatal(err)
	}
	return s
}
func acceptedHandler(status int, counts *int) (*httptest.Server, *http.Client, string) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*counts++
		if r.Method != "POST" || r.URL.Path != scanPath || r.Header.Get("Content-Type") != "application/json" {
			w.WriteHeader(405)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		if status == 200 || status == 201 {
			fmt.Fprintf(w, `{"id":%q,"replayed":false,"authority":%q,"production_credit":false}`, serverScanID, receiptAuthority)
		}
	}))
	return srv, srv.Client(), srv.URL + scanPath
}
func TestReplayConflictAndStrictInput(t *testing.T) {
	s := newState(t)
	now := time.Now()
	raw := newSignedEnvelope(t, "scan-event-00000001", now)
	id, repeat, err := s.enqueue(raw)
	if err != nil || repeat || len(id) != 64 {
		t.Fatal(id, repeat, err)
	}
	spaced := append([]byte("  \n"), raw...)
	_, repeat, err = s.enqueue(spaced)
	if err != nil || !repeat {
		t.Fatal("semantically identical replay must be idempotent", err)
	}
	var v envelope
	if err = json.Unmarshal(raw, &v); err != nil {
		t.Fatal(err)
	}
	v.Event.PartID = carrierID
	changed, _ := json.Marshal(v)
	if _, _, err = s.enqueue(changed); err == nil {
		t.Fatal("event-ID reuse with changed content must fail")
	}
	cases := [][]byte{
		[]byte("{}"), []byte("{invalid"), append(raw, []byte("{}")...),
		[]byte(strings.Replace(string(raw), `"version":"WIP_GATEWAY_SCAN_V1"`, `"version":"BAD"`, 1)),
		[]byte(strings.Replace(string(raw), `"event":{`, `"event":{"unexpected":true,`, 1)),
		[]byte(strings.Replace(string(raw), `"signature_hex":"`, `"signature_hex":"x`, 1)),
	}
	for _, c := range cases {
		if _, _, e := s.enqueue(c); e == nil {
			t.Fatalf("accepted invalid envelope %q", c)
		}
	}
	counts, err := s.status()
	if err != nil || counts["pending"] != 1 {
		t.Fatal(counts, err)
	}
}
func TestRetryThenAcceptedAndCrashReplay(t *testing.T) {
	s := newState(t)
	raw := newSignedEnvelope(t, "scan-event-00000002", time.Now())
	key, _, err := s.enqueue(raw)
	if err != nil {
		t.Fatal(err)
	}
	attempts := 0
	srv, client, target := acceptedHandler(503, &attempts)
	defer srv.Close()
	got, err := s.flush(context.Background(), client, target, 12*time.Minute, 50, time.Now)
	if err != nil || got.retried != 1 || got.delivered != 0 || attempts != 1 {
		t.Fatal(got, err)
	}
	counts, _ := s.status()
	if counts["pending"] != 1 || counts["inflight"] != 0 {
		t.Fatal(counts)
	}
	// A process crash after pending->inflight must be recovered idempotently.
	if err = durableMove(s.file("pending", key), s.file("inflight", key)); err != nil {
		t.Fatal(err)
	}
	srv2, client2, target2 := acceptedHandler(201, &attempts)
	defer srv2.Close()
	got, err = s.flush(context.Background(), client2, target2, 12*time.Minute, 50, time.Now)
	if err != nil || got.delivered != 1 || attempts != 2 {
		t.Fatal(got, err)
	}
	counts, _ = s.status()
	if counts["accepted"] != 1 || counts["inflight"] != 0 || counts["pending"] != 0 {
		t.Fatal(counts)
	}
	if _, repeat, e := s.enqueue(raw); e != nil || !repeat {
		t.Fatal("accepted receipt does not deduplicate", e)
	}
	var ack receipt
	b, e := os.ReadFile(s.file("accepted", key))
	if e != nil {
		t.Fatal(e)
	}
	if json.Unmarshal(b, &ack) != nil || ack.ServerScanID != serverScanID || ack.EnvelopeSHA256 != contentHash(raw) {
		t.Fatal(ack)
	}
}
func TestExpiryAndPermanentRejectQuarantine(t *testing.T) {
	s := newState(t)
	now := time.Now()
	expiredKey, _, e := s.enqueue(newSignedEnvelope(t, "scan-event-00000003", now.Add(-16*time.Minute)))
	if e != nil {
		t.Fatal(e)
	}
	freshKey, _, e := s.enqueue(newSignedEnvelope(t, "scan-event-00000004", now))
	if e != nil {
		t.Fatal(e)
	}
	attempts := 0
	srv, client, target := acceptedHandler(403, &attempts)
	defer srv.Close()
	got, err := s.flush(context.Background(), client, target, 12*time.Minute, 50, time.Now)
	if err != nil || got.quarantined != 2 || attempts != 1 {
		t.Fatal(got, err)
	}
	counts, _ := s.status()
	if counts["pending"] != 0 || counts["quarantine"] != 2 {
		t.Fatal(counts)
	}
	for key, want := range map[string]string{expiredKey: "SCAN_EXPIRED_REQUIRES_EXCEPTION_REVIEW", freshKey: "SERVER_HTTP_403"} {
		if _, e = os.Stat(s.file("quarantine", key)); e != nil {
			t.Fatal("signed envelope missing", e)
		}
		b, e := os.ReadFile(filepath.Join(s.sub("quarantine"), key+".reason"))
		if e != nil || strings.TrimSpace(string(b)) != want {
			t.Fatal(string(b), e)
		}
	}
}
func TestNoForgedAckAndNoRedirect(t *testing.T) {
	s := newState(t)
	_, _, e := s.enqueue(newSignedEnvelope(t, "scan-event-00000005", time.Now()))
	if e != nil {
		t.Fatal(e)
	}
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(201)
		fmt.Fprint(w, `{"id":"irrelevant","replayed":false,"authority":"FABRICATED","production_credit":true}`)
	}))
	defer srv.Close()
	got, err := s.flush(context.Background(), srv.Client(), srv.URL+scanPath, 12*time.Minute, 50, time.Now)
	if err != nil || got.retried != 1 || got.delivered != 0 {
		t.Fatal(got, err)
	}
	if e = checkEndpoint("http://factory.example" + scanPath); e == nil {
		t.Fatal("insecure HTTP allowed")
	}
	if e = checkEndpoint(srv.URL + scanPath + "?token=secret"); e == nil {
		t.Fatal("query URL allowed")
	}
}
func TestConcurrentEnqueueSameKey(t *testing.T) {
	s := newState(t)
	raw := newSignedEnvelope(t, "scan-event-00000006", time.Now())
	var wg sync.WaitGroup
	errs := make(chan error, 20)
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); _, _, e := s.enqueue(raw); errs <- e }()
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		if e != nil {
			t.Fatal(e)
		}
	}
	counts, e := s.status()
	if e != nil || counts["pending"] != 1 {
		t.Fatal(counts, e)
	}
}
func TestSecurePathsAndCommand(t *testing.T) {
	root := t.TempDir()
	if _, err := initState("relative"); err == nil {
		t.Fatal("relative path accepted")
	}
	if err := os.Chmod(root, 0755); err != nil {
		t.Fatal(err)
	}
	if _, err := initState(root); err == nil {
		t.Fatal("world-readable state allowed")
	}
	if err := os.Symlink(t.TempDir(), filepath.Join(t.TempDir(), "s")); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	s := newState(t)
	if err := run([]string{"status", "--dir", s.root}, &buf); err != nil || !strings.Contains(buf.String(), "pending=0") {
		t.Fatal(buf.String(), err)
	}
	if err := run([]string{"flush", "--dir", s.root, "--endpoint", "http://bad"}, &buf); err == nil {
		t.Fatal("HTTP should fail")
	}
	if !errors.Is(context.Canceled, context.Canceled) {
		t.Fatal("unreachable")
	}
}
