// Author: Sunil Jain, CyberinQ LLC, 2026-10-29.
package stores

import (
	"context"
	"errors"
	"testing"
)

type trimTestVerifier struct{ calls int }

func (t *trimTestVerifier) VerifyMaterialWitness(_ context.Context, p WitnessProof, challenge string) error {
	t.calls++
	if p.Assertion != challenge+":"+p.Subject {
		return errors.New("wrong challenge")
	}
	return nil
}
func trimFixture() TrimIssueCommand {
	c := TrimIssueCommand{TenantID: "TEN-1", PlantID: "PLANT-1", TrimStockID: "TR-1", OrderID: "ORDER-1", LineID: "LINE-1", QuantityMilli: 2500, CommandID: "trim_command_0001"}
	c.Issuer = WitnessProof{IDPIssuer: "https://idp.example", TenantID: c.TenantID, PlantID: c.PlantID, Role: "stores_issuer", Subject: "worker-A", ProofID: "jwt-id-a"}
	c.Recipient = WitnessProof{IDPIssuer: "https://idp.example", TenantID: c.TenantID, PlantID: c.PlantID, Role: "stores_recipient", Subject: "worker-B", ProofID: "jwt-id-b"}
	ch := TrimIssueChallenge(c)
	c.Issuer.Assertion = ch + ":" + c.Issuer.Subject
	c.Recipient.Assertion = ch + ":" + c.Recipient.Subject
	return c
}
func TestTrimIssueAttestationBoundary(t *testing.T) {
	c := trimFixture()
	v := &trimTestVerifier{}
	if err := VerifyTrimWitnesses(context.Background(), v, c); err != nil || v.calls != 2 {
		t.Fatalf("witnesses failed: %v calls=%d", err, v.calls)
	}
	for name, alter := range map[string]func(*TrimIssueCommand){
		"same person":     func(c *TrimIssueCommand) { c.Recipient.Subject = c.Issuer.Subject },
		"same proof":      func(c *TrimIssueCommand) { c.Recipient.ProofID = c.Issuer.ProofID },
		"wrong plant":     func(c *TrimIssueCommand) { c.Recipient.PlantID = "other" },
		"wrong role":      func(c *TrimIssueCommand) { c.Issuer.Role = "stores_recipient" },
		"missing issuer":  func(c *TrimIssueCommand) { c.Issuer.IDPIssuer = "" },
		"qty changed":     func(c *TrimIssueCommand) { c.QuantityMilli++ },
		"wrong signature": func(c *TrimIssueCommand) { c.Recipient.Assertion = "tampered" },
	} {
		t.Run(name, func(t *testing.T) {
			bad := trimFixture()
			alter(&bad)
			if err := VerifyTrimWitnesses(context.Background(), &trimTestVerifier{}, bad); err == nil {
				t.Fatal("accepted invalid witness or altered quantity")
			}
		})
	}
}
func TestTrimChallengeCanonicalAndBounds(t *testing.T) {
	c := trimFixture()
	got := TrimIssueChallenge(c)
	if len(got) != 64 || !ValidTrimIssue(c) {
		t.Fatal("challenge invalid")
	}
	variants := []func(*TrimIssueCommand){func(c *TrimIssueCommand) { c.QuantityMilli++ }, func(c *TrimIssueCommand) { c.LineID = "LINE-2" }, func(c *TrimIssueCommand) { c.PlantID = "PLANT-2" }, func(c *TrimIssueCommand) { c.CommandID = "trim_command_0002" }}
	for _, f := range variants {
		d := trimFixture()
		f(&d)
		if TrimIssueChallenge(d) == got {
			t.Fatal("unbound input")
		}
	}
	for _, bad := range []int64{-1, 0, 1000000000000} {
		d := trimFixture()
		d.QuantityMilli = bad
		if ValidTrimIssue(d) {
			t.Fatal("accepted invalid quantity")
		}
	}
	d := trimFixture()
	d.CommandID = "short"
	if ValidTrimIssue(d) {
		t.Fatal("short key accepted")
	}
	d = trimFixture()
	d.CommandID = "trim command spaces"
	if ValidTrimIssue(d) {
		t.Fatal("unsafe key accepted")
	}
}
