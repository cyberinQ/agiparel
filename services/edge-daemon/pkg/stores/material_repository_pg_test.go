package stores

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

// Run only against a disposable PG16+ test instance with 0001..0006 applied.
// TEST_ADMIN is a migration/fixture owner; TEST_APP MUST be non-BYPASSRLS agiparel_app.
func TestMaterialDurabilityWithPostgres(t *testing.T) {
	adminURL, appURL := os.Getenv("AGIPAREL_TEST_ADMIN_DATABASE_URL"), os.Getenv("AGIPAREL_TEST_APP_DATABASE_URL")
	if adminURL == "" || appURL == "" {
		t.Skip("set both AGIPAREL_TEST_*_DATABASE_URL to run actual PostgreSQL tests")
	}
	if os.Getenv("AGIPAREL_DESTRUCTIVE_DB_TESTS") != "I_UNDERSTAND_TEST_FIXTURES" {
		t.Fatal("explicit staging-fixture test flag required")
	}
	admin, err := sql.Open("pgx", adminURL)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	app, err := sql.Open("pgx", appURL)
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	if err = admin.PingContext(ctx); err != nil {
		t.Fatal(err)
	}
	if err = app.PingContext(ctx); err != nil {
		t.Fatal(err)
	}
	var dbUser string
	if err = app.QueryRowContext(ctx, "SELECT current_user").Scan(&dbUser); err != nil {
		t.Fatal(err)
	}
	if dbUser != "agiparel_app" {
		t.Fatalf("test must use restricted agiparel_app, got %s", dbUser)
	}
	id := make([]byte, 8)
	if _, err = rand.Read(id); err != nil {
		t.Fatal(err)
	}
	suffix := strings.ToUpper(hex.EncodeToString(id))
	tenant, plant, order, otherOrder, roll := "T-"+suffix, "P-"+suffix, "PO-"+suffix, "PO2-"+suffix, "R-"+suffix
	cleanup := []string{
		"DELETE FROM material_used_proofs_v5 WHERE tenant_id=$1",
		"DELETE FROM material_outbox_v5 WHERE tenant_id=$1",
		"DELETE FROM material_variances_v5 WHERE tenant_id=$1",
		"DELETE FROM material_issues_v5 WHERE tenant_id=$1",
		"DELETE FROM material_reservations_v5 WHERE tenant_id=$1",
		"DELETE FROM material_write_scopes_v5 WHERE tenant_id=$1",
		"DELETE FROM material_ceo_principals_v5 WHERE tenant_id=$1",
		"DELETE FROM store_rolls WHERE tenant_id=$1",
		"DELETE FROM tenants WHERE id=$1",
	}
	defer func() {
		for _, q := range cleanup {
			_, _ = admin.ExecContext(context.Background(), q, tenant)
		}
	}()
	for _, stmt := range []struct {
		q    string
		args []any
	}{
		{"INSERT INTO tenants(id,name) VALUES($1,'Batch02 test')", []any{tenant}},
		{"INSERT INTO store_rolls(id,tenant_id,plant_id,factory_sku,original_length_m,remaining_length_m,usable_width_cm,shade_band,bin_location,status,inspection_status,reserved_order_id) VALUES($1,$2,$3,'COTTON',100,100,150,'A','BIN-A','AVAILABLE','PASS',$4)", []any{roll, tenant, plant, order}},
		{"INSERT INTO material_write_scopes_v5(tenant_id,order_id,plant_id,mode,lease_holder,lease_revision,legacy_writer_fenced,approved_by,lease_expires_at) VALUES($1,$2,$3,'M3','edge-test',1,true,'test-admin',now()+interval '1 hour')", []any{tenant, order, plant}},
		{"INSERT INTO material_write_scopes_v5(tenant_id,order_id,plant_id,mode,lease_holder,lease_revision,legacy_writer_fenced,approved_by,lease_expires_at) VALUES($1,$2,$3,'M3','edge-test',1,true,'test-admin',now()+interval '1 hour')", []any{tenant, otherOrder, plant}},
		{"INSERT INTO material_ceo_principals_v5(tenant_id,subject,enrolled_by,active) VALUES($1,'ceo-human','test-admin',true)", []any{tenant}},
	} {
		if _, err = admin.ExecContext(ctx, stmt.q, stmt.args...); err != nil {
			t.Fatalf("staging fixture failed: %v", err)
		}
	}
	repo := MaterialRepository{DB: app, AuthorityHolder: "edge-test", WitnessVerifier: testPGWitness{}, CEOApprovalVerifier: testPGCEO{}}
	reserve := MaterialReserveCommand{TenantID: tenant, PlantID: plant, RollID: roll, OrderID: order, CutNo: 1, LengthMM: 60000, IdempotencyKey: "reserve-1"}
	receipt, err := repo.Reserve(ctx, reserve)
	if err != nil {
		t.Fatal(err)
	}
	replay, err := (MaterialRepository{DB: app, AuthorityHolder: "edge-test"}).Reserve(ctx, reserve)
	if err != nil || replay.ID != receipt.ID {
		t.Fatalf("non-durable or non-idempotent reservation: %v", err)
	}
	second := reserve
	second.IdempotencyKey = "too-much"
	second.LengthMM = 50000
	if _, err = repo.Reserve(ctx, second); err == nil {
		t.Fatal("overreservation accepted")
	}
	wrongOrder := reserve
	wrongOrder.OrderID = otherOrder
	wrongOrder.IdempotencyKey = "cross-po"
	wrongOrder.LengthMM = 1000
	if _, err = repo.Reserve(ctx, wrongOrder); err == nil {
		t.Fatal("cross-PO borrowing accepted")
	}
	req := VarianceRequestCommand{TenantID: tenant, PlantID: plant, ReservationID: receipt.ID, RollID: roll, OrderID: order, CutNo: 1, AdditionalMM: 10000, ActualStory: "Measured end-of-roll stains required extra fabric", ReasonCode: "DEFECT_RECUT", RequestedBy: "manager", IdempotencyKey: "variance-1"}
	v, err := repo.RequestVariance(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	challenge, err := repo.LookupVarianceChallenge(ctx, tenant, plant, v.ID, "ceo-human")
	if err != nil || challenge != v.Challenge {
		t.Fatalf("CEO challenge mismatch: %v", err)
	}
	if _, err = repo.LookupVarianceChallenge(ctx, tenant, plant, v.ID, "deputy"); err == nil {
		t.Fatal("deputy accessed CEO request")
	}
	if _, err = repo.ApproveVariance(ctx, VarianceApprovalCommand{TenantID: tenant, PlantID: plant, RequestID: v.ID, CEOSubject: "manager", CEOProofID: "p-bad", CEOAssertion: "signed:" + v.Challenge}); err == nil {
		t.Fatal("non-CEO approval accepted")
	}
	approved, err := repo.ApproveVariance(ctx, VarianceApprovalCommand{TenantID: tenant, PlantID: plant, RequestID: v.ID, CEOSubject: "ceo-human", CEOProofID: "p-good", CEOAssertion: "signed:" + v.Challenge})
	if err != nil || approved.Status != "APPROVED" {
		t.Fatalf("CEO approval failed: %v", err)
	}
	issue := MaterialIssueCommand{TenantID: tenant, PlantID: plant, ReservationID: receipt.ID, RollID: roll, OrderID: order, CutNo: 1, LengthMM: 65000, IdempotencyKey: "issue-1"}
	hash := MaterialIssueChallenge(issue)
	issue.Issuer = WitnessProof{TenantID: tenant, PlantID: plant, Subject: "issuer", Role: "stores_issuer", ProofID: "witness1", Assertion: "signed:" + hash}
	issue.Recipient = WitnessProof{TenantID: tenant, PlantID: plant, Subject: "recipient", Role: "stores_recipient", ProofID: "witness2", Assertion: "signed:" + hash}
	issued, err := repo.Issue(ctx, issue)
	if err != nil {
		t.Fatal(err)
	}
	if issued.IssuedMM != 65000 {
		t.Fatal("issued quantity mismatch")
	}
	reissue, err := repo.Issue(ctx, issue)
	if err != nil || reissue.ID != issued.ID {
		t.Fatalf("non-idempotent replay: %v", err)
	}
	issue.IdempotencyKey = "issue-2"
	issue.LengthMM = 10000
	hash = MaterialIssueChallenge(issue)
	issue.Issuer.ProofID = "witness3"
	issue.Issuer.Assertion = "signed:" + hash
	issue.Recipient.ProofID = "witness4"
	issue.Recipient.Assertion = "signed:" + hash
	if _, err = repo.Issue(ctx, issue); err == nil {
		t.Fatal("overspent approved reservation")
	}
}

type testPGWitness struct{}

func (testPGWitness) VerifyMaterialWitness(_ context.Context, p WitnessProof, challenge string) error {
	if p.Assertion != "signed:"+challenge {
		return errors.New("wrong challenge")
	}
	if p.Role != "stores_issuer" && p.Role != "stores_recipient" {
		return errors.New("wrong role")
	}
	return nil
}

type testPGCEO struct{}

func (testPGCEO) VerifyCEOApproval(_ context.Context, _, _, _, assertion, challenge string) error {
	if assertion != "signed:"+challenge {
		return errors.New("wrong CEO signature")
	}
	return nil
}
