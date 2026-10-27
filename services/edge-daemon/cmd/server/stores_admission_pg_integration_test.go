package main

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/cyberinq/agiparel/services/edge-daemon/pkg/attestation"
	"github.com/cyberinq/agiparel/services/edge-daemon/pkg/stores"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func admissionSignedTestJWT(t *testing.T, priv ed25519.PrivateKey, sub, tenant, plant, role, action, challenge string) string {
	t.Helper()
	var seed [12]byte
	if _, e := rand.Read(seed[:]); e != nil {
		t.Fatal(e)
	}
	now := time.Now().UTC().Unix()
	body := attestation.Claims{Issuer: "agiparel-test-idp", Audience: "agiparel-test", Subject: sub, TenantID: tenant, PlantID: plant, Action: action, Challenge: challenge, JWTID: hex.EncodeToString(seed[:]), IssuedAt: now, NotBefore: now, ExpiresAt: now + 180, Roles: []string{role}, AuthMethods: []string{"webauthn"}}
	header, _ := json.Marshal(map[string]string{"alg": "EdDSA", "typ": "JWT", "kid": "test"})
	payload, _ := json.Marshal(body)
	msg := base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(payload)
	return msg + "." + base64.RawURLEncoding.EncodeToString(ed25519.Sign(priv, []byte(msg)))
}

// Explicit opt-in, random scratch tenant, no production data. Exercises a
// signed admission and confirms stock balances and audit cannot be overwritten
// by replays, while another tenant cannot read its review queue.
func TestStoresSignedAdmissionPostgresEndToEnd(t *testing.T) {
	adminURL, appURL := os.Getenv("AGIPAREL_TEST_ADMIN_DATABASE_URL"), os.Getenv("AGIPAREL_TEST_APP_DATABASE_URL")
	if adminURL == "" || appURL == "" {
		t.Skip("requires both explicit PostgreSQL test URLs")
	}
	if os.Getenv("AGIPAREL_DESTRUCTIVE_DB_TESTS") != "I_UNDERSTAND_TEST_FIXTURES" {
		t.Fatal("explicit isolated fixture opt-in required")
	}
	admin, e := sql.Open("pgx", adminURL)
	if e != nil {
		t.Fatal(e)
	}
	defer admin.Close()
	app, e := sql.Open("pgx", appURL)
	if e != nil {
		t.Fatal(e)
	}
	defer app.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	if e = admin.PingContext(ctx); e != nil {
		t.Fatal(e)
	}
	if e = app.PingContext(ctx); e != nil {
		t.Fatal(e)
	}
	var role string
	if e = app.QueryRowContext(ctx, "SELECT current_user").Scan(&role); e != nil || role != "agiparel_app" {
		t.Fatalf("restricted role required: %s %v", role, e)
	}
	var random [10]byte
	if _, e = rand.Read(random[:]); e != nil {
		t.Fatal(e)
	}
	suffix := hex.EncodeToString(random[:])
	tenant := "S7C-" + suffix
	other := "S7C-other-" + suffix
	roll := "R7C-" + suffix
	doc := "DOC-" + suffix + "000000000000"
	// Fixture tenants only. Cleanup only these exact randomly generated IDs.
	for _, v := range []string{tenant, other} {
		if _, e = admin.ExecContext(ctx, `INSERT INTO tenants(id,name) VALUES($1,$2)`, v, "Signed admission fixture"); e != nil {
			t.Fatal(e)
		}
	}
	t.Cleanup(func() {
		c := context.Background()
		for _, ten := range []string{tenant, other} {
			for _, tbl := range []string{"material_outbox_v5", "store_roll_admission_proofs_v7", "store_roll_admissions_v7", "store_reconciliation_requests_v7", "store_roll_passport_events_v7", "store_roll_rfid_bindings_v7", "store_rolls", "document_sources_v65"} {
				_, _ = admin.ExecContext(c, fmt.Sprintf("DELETE FROM %s WHERE tenant_id=$1", tbl), ten)
			}
			_, _ = admin.ExecContext(c, "DELETE FROM tenants WHERE id=$1", ten)
		}
	})
	// An immutable original is required by the admission gate.
	raw := []byte("Original GRN for isolated 07C test fixture")
	hash := sha256.Sum256(raw)
	if _, e = admin.ExecContext(ctx, `INSERT INTO document_sources_v65(tenant_id,id,filename,media_type,format,byte_length,sha256,original) VALUES($1,$2,'fixture.txt','text/plain','TXT',$3,$4,$5)`, tenant, doc, len(raw), hex.EncodeToString(hash[:]), raw); e != nil {
		t.Fatal(e)
	}
	t.Setenv("ENV", "development")
	t.Setenv("AGIPAREL_OPERATOR_TENANT", tenant)
	s := &EdgeServer{db: app}
	post := func(endpoint string, payload any, fn http.HandlerFunc) (int, map[string]any) {
		t.Helper()
		b, _ := json.Marshal(payload)
		r := httptest.NewRequest("POST", endpoint, bytes.NewReader(b))
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		fn(w, r)
		var obj map[string]any
		_ = json.Unmarshal(w.Body.Bytes(), &obj)
		return w.Code, obj
	}
	p := samplePassport()
	p.RollID = roll
	p.EPC = "300833B2DDD9" + strings.ToUpper(suffix[:12])
	p.SourceDocumentID = doc
	p.InspectionResult = "PASS"
	status, obj := post("/api/v1/stores/passports", p, s.handleStoresPassports)
	if status != 201 {
		t.Fatalf("passport: %d %v", status, obj)
	}
	rid := "review_" + suffix
	status, obj = post("/api/v1/stores/reconciliation", storesReconciliationInput{RollID: roll, ExpectedPassportRevision: 1, RequestKey: rid, Note: "Original GRN inspected; independent validation requested"}, s.handleStoresReconciliation)
	if status != 201 {
		t.Fatalf("reconciliation: %d %v", status, obj)
	}
	requestID := obj["request"].(map[string]any)["id"].(string)
	pub, priv, e := ed25519.GenerateKey(rand.Reader)
	if e != nil {
		t.Fatal(e)
	}
	verifier := &attestation.Verifier{PublicKeys: map[string]ed25519.PublicKey{"test": pub}, Issuer: "agiparel-test-idp", Audience: "agiparel-test", MaxLifetime: 5 * time.Minute}
	a := &materialAPI{repository: stores.MaterialRepository{DB: app}, verifier: verifier}
	// Signed transaction is derived exclusively from the stored, immutable review.
	tx, e := s.docTransaction(ctx, tenant)
	if e != nil {
		t.Fatal(e)
	}
	c, e := loadAdmissionCandidate(ctx, tx, tenant, requestID)
	if e != nil {
		_ = tx.Rollback()
		t.Fatal(e)
	}
	if reasons := admissionReasons(c); len(reasons) != 0 {
		_ = tx.Rollback()
		t.Fatalf("not ready: %v", reasons)
	}
	_ = tx.Rollback()
	challenge := storesAdmissionChallenge(tenant, "PLANT_DEFAULT", c)
	actor := attestation.Claims{Subject: "manager-1", TenantID: tenant, PlantID: "PLANT_DEFAULT"}
	mgr := admissionSignedTestJWT(t, priv, "manager-1", tenant, "PLANT_DEFAULT", "stores_manager", "material.roll.admit.witness", challenge)
	inspector := admissionSignedTestJWT(t, priv, "inspector-2", tenant, "PLANT_DEFAULT", "stores_inspector", "material.roll.admit.witness", challenge)
	_, e = a.saveSignedRollAdmission(ctx, actor, admissionSubmitBody{RequestID: requestID, ManagerAttestation: mgr, InspectorAttestation: mgr})
	if e == nil {
		t.Fatal("same human/proof accepted for both roles")
	}
	receipt, e := a.saveSignedRollAdmission(ctx, actor, admissionSubmitBody{RequestID: requestID, ManagerAttestation: mgr, InspectorAttestation: inspector})
	if e != nil {
		t.Fatal(e)
	}
	if receipt.RollID != roll || receipt.AdmittedMM <= 0 {
		t.Fatalf("bad result: %+v", receipt)
	}
	_, e = a.saveSignedRollAdmission(ctx, actor, admissionSubmitBody{RequestID: requestID, ManagerAttestation: mgr, InspectorAttestation: inspector})
	if e == nil {
		t.Fatal("replay committed a second admission")
	}
	var n int
	var length int64
	if e = admin.QueryRowContext(ctx, `SELECT count(*),COALESCE(MAX(ROUND(remaining_length_m*1000)::bigint),0) FROM store_rolls WHERE tenant_id=$1 AND id=$2`, tenant, roll).Scan(&n, &length); e != nil || n != 1 || length != receipt.AdmittedMM {
		t.Fatalf("ledger mismatch rows=%d length=%d err=%v", n, length, e)
	}
	if _, e = app.ExecContext(ctx, `UPDATE store_roll_admissions_v7 SET roll_id='OTHER' WHERE tenant_id=$1`, tenant); e == nil {
		t.Fatal("runtime role modified admission history")
	}
	t.Setenv("AGIPAREL_OPERATOR_TENANT", other)
	w := httptest.NewRecorder()
	s.handleStoresAdmissions(w, httptest.NewRequest(http.MethodGet, "/api/v1/stores/admissions", nil))
	if w.Code != 200 {
		t.Fatalf("tenant read %d %s", w.Code, w.Body.String())
	}
	if strings.Contains(w.Body.String(), roll) {
		t.Fatal("roll leaked across tenants")
	}
}
