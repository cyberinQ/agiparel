// Author: Sunil Jain, 2026-10-26 — Verify independent witness authentication and replay challenge semantics.
package stores

import (
	"context"
	"errors"
	"testing"
)

type witnessTestVerifier struct{ denied string }

func (v witnessTestVerifier) VerifyMaterialWitness(_ context.Context, p WitnessProof, challenge string) error {
	if p.Subject == v.denied || p.Assertion != "signed:"+challenge {
		return errors.New("signature or audience invalid")
	}
	return nil
}

func TestIssueRequiresTwoIndependentlyVerifiedProofs(t *testing.T) {
	req := MaterialIssueCommand{TenantID: "TENANT-A", ReservationID: "RES-1", RollID: "ROLL-9", OrderID: "PO-1", CutNo: 1, LengthMM: 42000, IdempotencyKey: "COMMAND-33",
		Issuer:    WitnessProof{TenantID: "TENANT-A", Subject: "ISSUER-1", ProofID: "PROOF-A"},
		Recipient: WitnessProof{TenantID: "TENANT-A", Subject: "RECEIVER-1", ProofID: "PROOF-B"}}
	hash := MaterialIssueChallenge(req)
	req.Issuer.Assertion = "signed:" + hash
	req.Recipient.Assertion = "signed:" + hash
	if _, err := VerifyMaterialWitnesses(context.Background(), nil, req); err == nil {
		t.Fatal("no verifier must fail closed")
	}
	got, err := VerifyMaterialWitnesses(context.Background(), witnessTestVerifier{}, req)
	if err != nil || got != hash {
		t.Fatalf("expected two validated witnesses: %s %v", got, err)
	}
	req.Recipient.Subject = req.Issuer.Subject
	if _, err = VerifyMaterialWitnesses(context.Background(), witnessTestVerifier{}, req); err == nil {
		t.Fatal("same person cannot witness both sides")
	}
	req.Recipient.Subject = "RECEIVER-1"
	req.Recipient.Assertion = "tampered"
	if _, err = VerifyMaterialWitnesses(context.Background(), witnessTestVerifier{}, req); err == nil {
		t.Fatal("bad recipient signature must fail")
	}
}
