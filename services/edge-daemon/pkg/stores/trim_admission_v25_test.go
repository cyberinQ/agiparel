// Author: Sunil Jain, CyberinQ LLC, 2026-10-29.
package stores

import (
	"context"
	"errors"
	"testing"
)

type trimAdmissionTestVerifier struct{ calls int }

func (v *trimAdmissionTestVerifier) VerifyTrimAdmissionWitness(_ context.Context, p WitnessProof, ch string) error {
	v.calls++
	if p.Assertion != ch+":"+p.Subject {
		return errors.New("invalid signature")
	}
	return nil
}
func trimAdmissionFixture() TrimAdmissionCommand {
	c := TrimAdmissionCommand{TenantID: "TEN-A", PlantID: "PLANT-A", StockID: "TR-STOCK-100", TrimCode: "ZIP-05", Description: "zipper 5 inch", SupplierLot: "SLOT-20", ColorCode: "BLK", SizeCode: "", UOM: "PCS", QuantityMilli: 250000, BinLocation: "BIN-10", SourceDocumentID: "DOC-100", SourceSHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", CommandID: "trim_admission_000001"}
	c.Manager = WitnessProof{TenantID: c.TenantID, PlantID: c.PlantID, Role: "stores_manager", Subject: "manager-A", ProofID: "jti-manager", IDPIssuer: "https://idp.example"}
	c.Inspector = WitnessProof{TenantID: c.TenantID, PlantID: c.PlantID, Role: "stores_inspector", Subject: "inspector-B", ProofID: "jti-inspector", IDPIssuer: "https://idp.example"}
	ch := TrimAdmissionChallenge(c)
	c.Manager.Assertion = ch + ":" + c.Manager.Subject
	c.Inspector.Assertion = ch + ":" + c.Inspector.Subject
	return c
}
func TestTrimAdmissionSignedAndBound(t *testing.T) {
	c := trimAdmissionFixture()
	v := &trimAdmissionTestVerifier{}
	if err := VerifyTrimAdmissionWitnesses(context.Background(), v, c); err != nil || v.calls != 2 {
		t.Fatalf("verification failed: %v, %d", err, v.calls)
	}
	for name, alter := range map[string]func(*TrimAdmissionCommand){
		"changed quantity": func(c *TrimAdmissionCommand) { c.QuantityMilli++ },
		"changed document": func(c *TrimAdmissionCommand) { c.SourceDocumentID = "DOC-200" },
		"changed hash": func(c *TrimAdmissionCommand) {
			c.SourceSHA256 = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
		},
		"changed lot":    func(c *TrimAdmissionCommand) { c.SupplierLot = "OTHER" },
		"changed UOM":    func(c *TrimAdmissionCommand) { c.UOM = "M" },
		"wrong plant":    func(c *TrimAdmissionCommand) { c.Inspector.PlantID = "PLANT-B" },
		"same person":    func(c *TrimAdmissionCommand) { c.Inspector.Subject = c.Manager.Subject },
		"same proof":     func(c *TrimAdmissionCommand) { c.Inspector.ProofID = c.Manager.ProofID },
		"wrong role":     func(c *TrimAdmissionCommand) { c.Manager.Role = "stores_inspector" },
		"no issuer":      func(c *TrimAdmissionCommand) { c.Manager.IDPIssuer = "" },
		"tampered proof": func(c *TrimAdmissionCommand) { c.Inspector.Assertion = "tampered" },
	} {
		t.Run(name, func(t *testing.T) {
			d := trimAdmissionFixture()
			alter(&d)
			if err := VerifyTrimAdmissionWitnesses(context.Background(), &trimAdmissionTestVerifier{}, d); err == nil {
				t.Fatal("accepted invalid admission")
			}
		})
	}
}
func TestTrimAdmissionInputAndChallenge(t *testing.T) {
	c := trimAdmissionFixture()
	if !ValidTrimAdmission(c) || len(TrimAdmissionChallenge(c)) != 64 {
		t.Fatal("base invalid")
	}
	cases := []func(*TrimAdmissionCommand){
		func(c *TrimAdmissionCommand) { c.QuantityMilli = 0 },
		func(c *TrimAdmissionCommand) { c.QuantityMilli = 1000000000000 },
		func(c *TrimAdmissionCommand) { c.CommandID = "short" },
		func(c *TrimAdmissionCommand) { c.SourceSHA256 = "not hex" },
		func(c *TrimAdmissionCommand) { c.UOM = "some units" },
		func(c *TrimAdmissionCommand) { c.Description = "oops\nmore" },
		func(c *TrimAdmissionCommand) { c.BinLocation = "x" },
	}
	for _, f := range cases {
		d := trimAdmissionFixture()
		f(&d)
		if ValidTrimAdmission(d) {
			t.Fatalf("invalid admitted: %+v", d)
		}
	}
	for _, f := range []func(*TrimAdmissionCommand){
		func(c *TrimAdmissionCommand) { c.PlantID = "PLANT-B" },
		func(c *TrimAdmissionCommand) { c.StockID = "NEW-LOT" },
		func(c *TrimAdmissionCommand) { c.BinLocation = "BIN-20" },
		func(c *TrimAdmissionCommand) { c.CommandID = "trim_admission_000002" },
	} {
		d := trimAdmissionFixture()
		f(&d)
		if TrimAdmissionChallenge(d) == TrimAdmissionChallenge(c) {
			t.Fatal("unbound receipt detail")
		}
	}
}
