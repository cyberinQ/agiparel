// Author: Sunil Jain, CyberinQ LLC, 2026-10-29.
// F3.12 test keys, events and mock TLS responses are never plant records.
package main

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"
)

func signingFixture(t *testing.T) ([]byte, string) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	ev := scanEvent{Version: "WIP_GATEWAY_SCAN_V1", TenantID: "tenant-one", PlantID: "plant-one", GatewayID: "gw-one", StationID: "station-one", EventID: "real-like-event-12345", PartID: partID, CarrierID: carrierID, EPCHex: "ABCDEF123456ABCDEF123456", ObservedAt: time.Now().UTC().Format(time.RFC3339Nano)}
	sig := ed25519.Sign(priv, gatewaySignedBytes(ev))
	raw, err := json.Marshal(envelope{Event: ev, SignatureHex: hex.EncodeToString(sig)})
	if err != nil {
		t.Fatal(err)
	}
	return raw, hex.EncodeToString(pub)
}
func verifyFixture(t *testing.T, raw []byte, pub string) commissionEvidence {
	t.Helper()
	e, err := commissioningVerify(raw, pub, "tenant-one", "plant-one", "gw-one", "station-one", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	return e
}
func TestF312AuthenticMessageAndDigests(t *testing.T) {
	raw, pub := signingFixture(t)
	e := verifyFixture(t, raw, pub)
	if !e.SignedEnvelopeValid || e.ReceiptVerified || e.DatabaseVerified || e.EnrolledKeyDatabaseChecked || e.PhysicalRFCertification || e.Authority != receiptAuthority {
		t.Fatal(e)
	}
	var signed envelope
	if err := json.Unmarshal(raw, &signed); err != nil {
		t.Fatal(err)
	}
	if e.EventSHA256 != sumHex(gatewaySignedBytes(signed.Event)) || e.EnvelopeSHA256 != contentHash(raw) {
		t.Fatal("hash mismatch")
	}
	if _, err := commissioningVerify(raw, pub, "tenant-two", "plant-one", "gw-one", "station-one", time.Now()); err == nil {
		t.Fatal("cross-tenant event accepted")
	}
	if _, err := commissioningVerify(raw, pub, "tenant-one", "plant-one", "gw-wrong", "station-one", time.Now()); err == nil {
		t.Fatal("cross-gateway event accepted")
	}
	signed.Event.PartID = carrierID
	changed, _ := json.Marshal(signed)
	if _, err := commissioningVerify(changed, pub, "tenant-one", "plant-one", "gw-one", "station-one", time.Now()); err == nil {
		t.Fatal("tampered event accepted")
	}
	_, wrong, _ := ed25519.GenerateKey(rand.Reader)
	if _, err := commissioningVerify(raw, hex.EncodeToString(wrong.Public().(ed25519.PublicKey)), "tenant-one", "plant-one", "gw-one", "station-one", time.Now()); err == nil {
		t.Fatal("wrong key accepted")
	}
	if _, err := commissioningVerify(raw, "deadbeef", "tenant-one", "plant-one", "gw-one", "station-one", time.Now()); err == nil {
		t.Fatal("short key accepted")
	}
}
func TestF312FutureClockFailClosed(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	ev := scanEvent{Version: "WIP_GATEWAY_SCAN_V1", TenantID: "tenant-one", PlantID: "plant-one", GatewayID: "gw-one", StationID: "station-one", EventID: "clock-event-012345", PartID: partID, CarrierID: carrierID, EPCHex: "ABCDEF123456ABCDEF123456", ObservedAt: time.Now().Add(5 * time.Minute).UTC().Format(time.RFC3339Nano)}
	raw, _ := json.Marshal(envelope{Event: ev, SignatureHex: hex.EncodeToString(ed25519.Sign(priv, gatewaySignedBytes(ev)))})
	if _, err := commissioningVerify(raw, hex.EncodeToString(pub), "tenant-one", "plant-one", "gw-one", "station-one", time.Now()); err == nil {
		t.Fatal("future scan accepted")
	}
}
func TestF312ReceiptRequiresExactAcceptedRecord(t *testing.T) {
	s := newState(t)
	raw, pub := signingFixture(t)
	e := verifyFixture(t, raw, pub)
	if _, err := commissioningReceipt(s, e); err == nil {
		t.Fatal("missing receipt accepted")
	}
	key, _, err := s.enqueue(raw)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := commissioningReceipt(s, e); err == nil {
		t.Fatal("pending event accepted")
	}
	if err := os.Remove(s.file("pending", key)); err != nil {
		t.Fatal(err)
	}
	ack := receipt{EnvelopeSHA256: e.EnvelopeSHA256, ServerScanID: serverScanID, AcknowledgedAt: time.Now()}
	buf, _ := json.Marshal(ack)
	if err := durableWrite(s.file("accepted", key), buf); err != nil {
		t.Fatal(err)
	}
	got, err := commissioningReceipt(s, e)
	if err != nil || !got.ReceiptVerified || got.ScanID != serverScanID || got.DatabaseVerified {
		t.Fatal(got, err)
	}
	ack.EnvelopeSHA256 = strings.Repeat("0", 64)
	buf, _ = json.Marshal(ack)
	if err := durableWrite(s.file("accepted", key), buf); err != nil {
		t.Fatal(err)
	}
	if _, err := commissioningReceipt(s, e); err == nil {
		t.Fatal("mismatched receipt accepted")
	}
}
func TestF312VerifyAndAuditCommandsRemainReadOnly(t *testing.T) {
	raw, pub := signingFixture(t)
	f := t.TempDir() + "/signed.json"
	if err := os.WriteFile(f, raw, 0600); err != nil {
		t.Fatal(err)
	}
	params := []string{"--file", f, "--public-key-hex", pub, "--tenant", "tenant-one", "--plant", "plant-one", "--gateway", "gw-one", "--station", "station-one"}
	var out bytes.Buffer
	if err := run(append([]string{"verify"}, params...), &out); err != nil {
		t.Fatal(err)
	}
	var verified commissionEvidence
	if err := json.Unmarshal(out.Bytes(), &verified); err != nil || !verified.SignedEnvelopeValid || verified.DatabaseVerified {
		t.Fatal(err, out.String())
	}
	if err := run(append([]string{"verify", "--dir", t.TempDir()}, params...), &out); err == nil {
		t.Fatal("verify touched state")
	}
	s := newState(t)
	if err := run(append([]string{"audit", "--dir", s.root}, params...), &out); err == nil {
		t.Fatal("audit passed without accepted receipt")
	}
	if err := run(append([]string{"audit", "--dir", s.root, "--endpoint", "https://factory.example" + scanPath}, params...), &out); err == nil {
		t.Fatal("audit allowed network endpoint")
	}
}
