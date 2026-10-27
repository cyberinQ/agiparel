package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"testing"
	"time"
)

func testCutReaderEvent(now time.Time) cutReaderEvent {
	return cutReaderEvent{Version: "CUT_READER_V1", TenantID: "dev", PlantID: "plant-1", GatewayID: "rfid-gw-01", StationID: "cutting-exit-1", EventID: "event_012345678901", ObservedAt: now.UTC().Format(time.RFC3339Nano), DraftID: "00000000-0000-4000-8000-000000000001", SpreadID: "00000000-0000-4000-8000-000000000002", LabelCode: "CB08-0123456789ABCDEF01234567", EPCHex: "3034257BF7194E4000001A85", AntennaPort: 1}
}
func TestCuttingReaderSignatureAndTamper(t *testing.T) {
	now := time.Now().UTC()
	e := testCutReaderEvent(now)
	if err := validReaderEvent(e, now); err != nil {
		t.Fatal(err)
	}
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := cutReaderPayload(e)
	sig := hex.EncodeToString(ed25519.Sign(private, b))
	if !readerSignatureOK(public, e, sig) {
		t.Fatal("valid enrolled key signature rejected")
	}
	bad := e
	bad.EPCHex = "3034257BF7194E4000001A86"
	if readerSignatureOK(public, bad, sig) {
		t.Fatal("EPC substitution accepted")
	}
	bad = e
	bad.TenantID = "other"
	if readerSignatureOK(public, bad, sig) {
		t.Fatal("tenant substitution accepted")
	}
	bad = e
	bad.StationID = "sewing-line-5"
	if readerSignatureOK(public, bad, sig) {
		t.Fatal("station substitution accepted")
	}
	bad = e
	bad.EventID = "event_012345678902"
	if readerSignatureOK(public, bad, sig) {
		t.Fatal("event replay ID substituted")
	}
	otherPub, _, _ := ed25519.GenerateKey(rand.Reader)
	if readerSignatureOK(otherPub, e, sig) {
		t.Fatal("unregistered key accepted")
	}
	if readerSignatureOK(public, e, sig[:len(sig)-2]) {
		t.Fatal("short signature accepted")
	}
}
func TestCuttingReaderValidations(t *testing.T) {
	now := time.Now().UTC()
	base := testCutReaderEvent(now)
	cases := []struct {
		name   string
		mutate func(*cutReaderEvent)
	}{
		{"old event", func(v *cutReaderEvent) { v.ObservedAt = now.Add(-4 * time.Minute).Format(time.RFC3339Nano) }},
		{"future", func(v *cutReaderEvent) { v.ObservedAt = now.Add(2 * time.Minute).Format(time.RFC3339Nano) }},
		{"unknown version", func(v *cutReaderEvent) { v.Version = "CUT_READER_V0" }},
		{"epc lower case", func(v *cutReaderEvent) { v.EPCHex = "3034257bf7194e4000001a85" }},
		{"invalid antenna", func(v *cutReaderEvent) { v.AntennaPort = 0 }},
		{"weak event id", func(v *cutReaderEvent) { v.EventID = "abc" }},
		{"label lower", func(v *cutReaderEvent) { v.LabelCode = "CB08-0123456789abcdef01234567" }},
		{"foreign draft", func(v *cutReaderEvent) { v.DraftID = "fake" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			v := base
			tc.mutate(&v)
			if validReaderEvent(v, now) == nil {
				t.Fatal("invalid event accepted")
			}
		})
	}
}
func TestCuttingReleaseChallengeBindsEvidence(t *testing.T) {
	base := cutReleaseEvidence{Version: "CUT_SEWING_RELEASE_V1", TenantID: "dev", PlantID: "plant-1", SpreadID: "spread-id", DraftID: "draft-id", SnapshotSHA: "sha", ConsumptionMM: 120000, BundleCount: 1, Entries: []cutReleaseEntry{{LabelCode: "CB08-ABC", EPCHex: "3034257BF7194E4000001A85", HumanProofID: "proof-1", ReaderGateway: "reader-1", ReaderEventID: "event-1", ReaderEventSHA: "sha-1"}}}
	h := cutEvidenceHash(base)
	variants := []cutReleaseEvidence{}
	v := base
	v.TenantID = "other"
	variants = append(variants, v)
	v = base
	v.PlantID = "other"
	variants = append(variants, v)
	v = base
	v.ConsumptionMM++
	variants = append(variants, v)
	v = base
	v.BundleCount++
	variants = append(variants, v)
	v = base
	v.Entries = append([]cutReleaseEntry(nil), base.Entries...)
	v.Entries[0].EPCHex = "different"
	variants = append(variants, v)
	v = base
	v.Entries = append([]cutReleaseEntry(nil), base.Entries...)
	v.Entries[0].ReaderEventSHA = "different"
	variants = append(variants, v)
	for i, other := range variants {
		if cutEvidenceHash(other) == h {
			t.Fatalf("mutation %d left approval challenge unchanged", i)
		}
	}
}
