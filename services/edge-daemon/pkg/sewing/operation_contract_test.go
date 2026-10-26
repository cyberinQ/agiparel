// Author: Sunil Jain, 2026-10-26 — Prove typed operation validation rejects mismatched components and serials.
package sewing

import "testing"

func TestOperationClaimRequiresExactMatchingBundles(t *testing.T) {
	c := OperationContract{ID: "OP-21", StyleRevision: "STYLE-3", RouteRevision: "ROUTE-4", MachineClass: "LOCKSTITCH", Mode: BundleToSingle,
		RequiredParts: []string{"POCKET", "FRONT"}, RequiredPredecessors: []string{"CUT-ACCEPTED"}}
	a := SourcePart{CarrierID: "TAG-A", PartCode: "POCKET", Kind: "BUNDLE", StyleRevision: "STYLE-3", Shade: "S1", Lot: "L1", GarmentSerials: []string{"G1", "G2"}}
	b := SourcePart{CarrierID: "TAG-B", PartCode: "FRONT", Kind: "BUNDLE", StyleRevision: "STYLE-3", Shade: "S1", Lot: "L1", GarmentSerials: []string{"G2", "G1"}}
	claim := OperationClaim{CommandID: "CMD-1", RouteRevision: "ROUTE-4", MachineClass: "LOCKSTITCH", Inputs: []SourcePart{a, b}, OutputGarmentSerials: []string{"G1", "G2"}, CompletedPredecessors: []string{"CUT-ACCEPTED"}}
	valid, err := ValidateOperationClaim(c, claim)
	if err != nil || !valid.Eligible || valid.CreditableUnits != 2 {
		t.Fatalf("expected eligible %+v err %v", valid, err)
	}
	claim.Inputs[1].GarmentSerials = []string{"G1", "G3"}
	if _, err = ValidateOperationClaim(c, claim); err == nil {
		t.Fatal("cross-garment input must reject")
	}
	claim.Inputs[1] = b
	claim.Inputs[1].Shade = "S2"
	if _, err = ValidateOperationClaim(c, claim); err == nil {
		t.Fatal("cross-shade assembly must reject")
	}
	claim.Inputs[1] = b
	claim.CompletedPredecessors = nil
	if _, err = ValidateOperationClaim(c, claim); err == nil {
		t.Fatal("incomplete predecessor must reject")
	}
	claim.CompletedPredecessors = []string{"CUT-ACCEPTED"}
	claim.Inputs[1].CarrierID = "TAG-A"
	if _, err = ValidateOperationClaim(c, claim); err == nil {
		t.Fatal("reused carrier must reject")
	}
}

func TestSingleModeCreditCannotExpandOneIntoTwo(t *testing.T) {
	c := OperationContract{ID: "OP-1", StyleRevision: "S", RouteRevision: "R", MachineClass: "M", Mode: SingleToSingle, RequiredParts: []string{"COLLAR"}}
	claim := OperationClaim{CommandID: "X", RouteRevision: "R", MachineClass: "M", Inputs: []SourcePart{{CarrierID: "P1", PartCode: "COLLAR", Kind: "PIECE", StyleRevision: "S", Lot: "L", Shade: "A", GarmentSerials: []string{"G1"}}}, OutputGarmentSerials: []string{"G1", "G2"}}
	if _, err := ValidateOperationClaim(c, claim); err == nil {
		t.Fatal("S2S may not credit 2 pieces from one")
	}
}
