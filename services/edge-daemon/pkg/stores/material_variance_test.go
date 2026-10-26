package stores

import (
	"context"
	"testing"
)

func TestVarianceChallengeBindsImmutableStoryAndPlant(t *testing.T) {
	c := VarianceRequestCommand{TenantID: "TENANT-1", PlantID: "P1", ReservationID: "r1", RollID: "roll1", OrderID: "o1", CutNo: 1, AdditionalMM: 42000, ActualStory: "Water damage from roll end required recuts", ReasonCode: "DEFECT_RECUT", RequestedBy: "manager", IdempotencyKey: "k1"}
	first := varianceChallenge(c, "id1")
	if len(first) != 64 {
		t.Fatalf("expected digest, got %q", first)
	}
	c.ActualStory = "Different variance narrative changed"
	if varianceChallenge(c, "id1") == first {
		t.Fatal("changed story retained prior approval challenge")
	}
	c.ActualStory = "Water damage from roll end required recuts"
	c.PlantID = "P2"
	if varianceChallenge(c, "id1") == first {
		t.Fatal("cross-plant replay retained challenge")
	}
}
func TestNewMaterialRepoCannotApproveWithoutDatabase(t *testing.T) {
	_, err := (MaterialRepository{}).ApproveVariance(context.Background(), VarianceApprovalCommand{TenantID: "T", PlantID: "P", RequestID: "r", CEOSubject: "ceo", CEOProofID: "p", CEOAssertion: "a"})
	if err == nil {
		t.Fatal("missing DB must fail closed")
	}
}
