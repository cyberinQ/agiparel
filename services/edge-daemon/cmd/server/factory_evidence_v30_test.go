// Author: Sunil Jain, CyberinQ LLC, 2026-10-29.
package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestF317CVendorAckDetachedSignature(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	c := jobworkVendorAckCommandV30{TenantID: "TENANT01", CommandID: "ACK_EVENT_00001", PlantID: "PLANT01", OutwardID: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", PartID: "cccccccc-cccc-4ccc-8ccc-cccccccccccc", JobReference: "VENDOR_JOB001", Process: "EMBROIDERY", VendorID: "VENDOR01", KeyID: "KEY01", AckKind: "OUTWARD_REPORTED_RECEIVED"}
	if !validVendorAckV30(c) {
		t.Fatal("valid ack rejected")
	}
	challenge := vendorAckChallengeV30(c)
	sig := hex.EncodeToString(ed25519.Sign(priv, []byte(challenge)))
	if !verifyVendorDetachedV30(hex.EncodeToString(pub), sig, challenge) {
		t.Fatal("vendor signature failed")
	}
	c.VendorID = "VENDOR02"
	if verifyVendorDetachedV30(hex.EncodeToString(pub), sig, vendorAckChallengeV30(c)) {
		t.Fatal("cross-vendor reuse accepted")
	}
	c.VendorID = "VENDOR01"
	c.TenantID = "TENANT02"
	if verifyVendorDetachedV30(hex.EncodeToString(pub), sig, vendorAckChallengeV30(c)) {
		t.Fatal("cross-tenant replay accepted")
	}
	c.TenantID = "TENANT01"
	c.OutwardID = "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"
	if verifyVendorDetachedV30(hex.EncodeToString(pub), sig, vendorAckChallengeV30(c)) {
		t.Fatal("other outward accepted")
	}
	if verifyVendorDetachedV30(hex.EncodeToString(pub), strings.Repeat("0", 128), challenge) {
		t.Fatal("bad signature accepted")
	}
	if verifyVendorDetachedV30(strings.Repeat("0", 64), sig, challenge) {
		t.Fatal("wrong key accepted")
	}
	c.Process = "PRODUCTION_RELEASE"
	if validVendorAckV30(c) {
		t.Fatal("invalid process accepted")
	}
	c.Process = "EMBROIDERY"
	c.AckKind = "RETURN_CERTIFIED"
	if validVendorAckV30(c) {
		t.Fatal("unimplemented vendor certification accepted")
	}
}
func TestF318AReportedDimensions(t *testing.T) {
	c := fabricShrinkageCommandV30{CommandID: "SHRINKAGE_TEST_001", PlantID: "PLANT01", RollID: "ROLL01", MaterialSKU: "COTTON01", SampleID: "LOT_SAMPLE_01", TreatmentCode: "WASH", WarpBeforeMM: 500, WarpAfterMM: 475, WeftBeforeMM: 400, WeftAfterMM: 408, SourceDocumentID: "document-001", SourceSHA256: strings.Repeat("a", 64), MethodNote: "Measured a marked sample after the stated wash process."}
	if !shrinkageValidV30(c) {
		t.Fatal("legitimate observation rejected")
	}
	if shrinkageBasisPointsV30(500, 475) != 500 || shrinkageBasisPointsV30(400, 408) != -200 {
		t.Fatal("basis point computation incorrect")
	}
	h := shrinkageChallengeV30(c)
	c.WarpAfterMM = 470
	if h == shrinkageChallengeV30(c) {
		t.Fatal("dimensions not bound to assertion")
	}
	c.WarpAfterMM = 475
	c.SourceSHA256 = strings.Repeat("b", 64)
	if h == shrinkageChallengeV30(c) {
		t.Fatal("source digest not bound")
	}
	c.SourceSHA256 = "bad"
	if shrinkageValidV30(c) {
		t.Fatal("source digest accepted")
	}
	c.SourceSHA256 = strings.Repeat("a", 64)
	c.WarpAfterMM = 310
	if shrinkageValidV30(c) {
		t.Fatal("unbounded apparent shrinkage accepted")
	}
	c.WarpAfterMM = 475
	c.TreatmentCode = "BUYER_APPROVED"
	if shrinkageValidV30(c) {
		t.Fatal("fabric source claim accepted as approval")
	}
}
func TestF317CF318AFailClosedNoAPIs(t *testing.T) {
	p := newJobworkAPI(nil)
	w := httptest.NewRecorder()
	p.vendorAckEvent(w, httptest.NewRequest(http.MethodPost, "/api/v1/jobwork/vendor-ack", strings.NewReader("{}")))
	if w.Code != 503 {
		t.Fatalf("disabled jobwork responded %d", w.Code)
	}
	s := newFabricShrinkageAPIv30(nil)
	w = httptest.NewRecorder()
	s.event(w, httptest.NewRequest(http.MethodPost, "/api/v1/fabric/shrinkage/observations", strings.NewReader("{}")))
	if w.Code != 503 {
		t.Fatalf("disabled shrinkage responded %d", w.Code)
	}
}
