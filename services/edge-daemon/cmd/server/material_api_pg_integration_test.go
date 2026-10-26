// Author: CyberinQ LLC, 2026-10-26.
// HTTP -> pinned Ed25519 signed identity -> real restricted PG16 -> committed receipts.
// Test keys are ephemeral; NO private key is installed in the production service.
package main

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
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
	_ "github.com/jackc/pgx/v5/stdlib"
)

// run only against a disposable DB already initialized with 0001..0006.
func TestMaterialHTTPWithSignedIdentityAndPostgres(t *testing.T) {
	adminURL, appURL := os.Getenv("AGIPAREL_TEST_ADMIN_DATABASE_URL"), os.Getenv("AGIPAREL_TEST_APP_DATABASE_URL")
	if adminURL == "" || appURL == "" {
		t.Skip("set AGIPAREL_TEST_ADMIN_DATABASE_URL and AGIPAREL_TEST_APP_DATABASE_URL")
	}
	if os.Getenv("AGIPAREL_DESTRUCTIVE_DB_TESTS") != "I_UNDERSTAND_TEST_FIXTURES" {
		t.Fatal("disposable database fixture opt-in required")
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
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	for _, db := range []*sql.DB{admin, app} {
		if err = db.PingContext(ctx); err != nil {
			t.Fatal(err)
		}
	}
	var dbRole string
	if err = app.QueryRowContext(ctx, "SELECT current_user").Scan(&dbRole); err != nil {
		t.Fatal(err)
	}
	if dbRole != "agiparel_app" {
		t.Fatalf("restricted DB role required, got %s", dbRole)
	}
	seed := make([]byte, 8)
	if _, err = rand.Read(seed); err != nil {
		t.Fatal(err)
	}
	suffix := strings.ToUpper(hex.EncodeToString(seed))
	tenant, otherTenant, plant, order, roll := "HT-"+suffix, "HX-"+suffix, "HP-"+suffix, "HO-"+suffix, "HR-"+suffix
	for _, q := range []struct {
		sql  string
		args []any
	}{
		{`INSERT INTO tenants(id,name) VALUES ($1,'Batch03 HTTP fixture')`, []any{tenant}},
		{`INSERT INTO tenants(id,name) VALUES ($1,'Batch03 other tenant')`, []any{otherTenant}},
		{`INSERT INTO store_rolls(id,tenant_id,plant_id,factory_sku,original_length_m,remaining_length_m,usable_width_cm,shade_band,bin_location,status,inspection_status,reserved_order_id) VALUES($1,$2,$3,'COTTON',100,100,150,'A','BIN-3','AVAILABLE','PASS',$4)`, []any{roll, tenant, plant, order}},
		{`INSERT INTO material_write_scopes_v5(tenant_id,order_id,plant_id,mode,lease_holder,lease_revision,legacy_writer_fenced,approved_by,lease_expires_at) VALUES($1,$2,$3,'M3','batch03-http',1,true,'test-admin',now()+interval '1 hour')`, []any{tenant, order, plant}},
		{`INSERT INTO material_ceo_principals_v5(tenant_id,subject,enrolled_by,active) VALUES($1,'ceo-human','test-admin',true)`, []any{tenant}},
	} {
		if _, err = admin.ExecContext(ctx, q.sql, q.args...); err != nil {
			t.Fatalf("fixture: %v", err)
		}
	}
	t.Cleanup(func() {
		for _, q := range []string{
			"DELETE FROM material_used_proofs_v5 WHERE tenant_id=$1",
			"DELETE FROM material_outbox_v5 WHERE tenant_id=$1",
			"DELETE FROM material_variances_v5 WHERE tenant_id=$1",
			"DELETE FROM material_issues_v5 WHERE tenant_id=$1",
			"DELETE FROM material_reservations_v5 WHERE tenant_id=$1",
			"DELETE FROM material_write_scopes_v5 WHERE tenant_id=$1",
			"DELETE FROM material_ceo_principals_v5 WHERE tenant_id=$1",
			"DELETE FROM store_rolls WHERE tenant_id=$1",
			"DELETE FROM tenants WHERE id=$1",
		} {
			_, _ = admin.ExecContext(context.Background(), q, tenant)
		}
		_, _ = admin.ExecContext(context.Background(), "DELETE FROM tenants WHERE id=$1", otherTenant)
	})
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("AGIPAREL_MATERIAL_API_ENABLED", "true")
	t.Setenv("AGIPAREL_IDP_ED25519_PUBLIC_KEY_B64", base64.RawURLEncoding.EncodeToString(pub))
	t.Setenv("AGIPAREL_IDP_KID", "batch03")
	t.Setenv("AGIPAREL_IDP_ISSUER", "batch03-test-idp")
	t.Setenv("AGIPAREL_IDP_AUDIENCE", "agiparel-material-http-test")
	t.Setenv("AGIPAREL_MATERIAL_AUTHORITY_HOLDER", "batch03-http")
	mux := http.NewServeMux()
	registerMaterialRoutes(mux, app)
	now := time.Now().UTC()
	counter := 0
	sign := func(tenantID, subject, action, challenge string, roles []string, methods []string, jti string) string {
		if jti == "" {
			counter++
			jti = fmt.Sprintf("proof-%d", counter)
		}
		c := attestation.Claims{Issuer: "batch03-test-idp", Audience: "agiparel-material-http-test", Subject: subject, TenantID: tenantID, PlantID: plant, Action: action, Challenge: challenge, JWTID: jti, IssuedAt: now.Unix(), NotBefore: now.Unix(), ExpiresAt: now.Add(2 * time.Minute).Unix(), Roles: roles, AuthMethods: methods}
		hdr, _ := json.Marshal(map[string]string{"alg": "EdDSA", "typ": "JWT", "kid": "batch03"})
		payload, _ := json.Marshal(c)
		msg := base64.RawURLEncoding.EncodeToString(hdr) + "." + base64.RawURLEncoding.EncodeToString(payload)
		return msg + "." + base64.RawURLEncoding.EncodeToString(ed25519.Sign(priv, []byte(msg)))
	}
	call := func(method, path, token string, body any) (int, map[string]any) {
		t.Helper()
		var b []byte
		if body != nil {
			var err error
			b, err = json.Marshal(body)
			if err != nil {
				t.Fatal(err)
			}
		}
		req := httptest.NewRequest(method, path, bytes.NewReader(b))
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)
		var v map[string]any
		_ = json.Unmarshal(w.Body.Bytes(), &v)
		return w.Code, v
	}
	assertStatus := func(want, got int, detail map[string]any) {
		t.Helper()
		if want != got {
			t.Fatalf("wanted HTTP %d, got %d, body=%v", want, got, detail)
		}
	}
	id := func(v map[string]any) string {
		t.Helper()
		s, _ := v["id"].(string)
		if s == "" {
			t.Fatalf("receipt has no id: %v", v)
		}
		return s
	}
	reserveBody := map[string]any{"roll_id": roll, "order_id": order, "cut_no": 1, "length_mm": int64(60000), "idempotency_key": "http-reserve"}
	code, _ := call("POST", "/api/v1/material/reservations", "", reserveBody)
	assertStatus(401, code, nil)
	code, _ = call("POST", "/api/v1/material/reservations", sign(tenant, "intruder", "material.reserve", "", []string{"stores_recipient"}, []string{"fido2"}, ""), reserveBody)
	if code == 200 {
		t.Fatal("wrong role accepted")
	}
	manager := sign(tenant, "manager", "material.reserve", "", []string{"stores_manager"}, []string{"fido2"}, "")
	code, res := call("POST", "/api/v1/material/reservations", manager, reserveBody)
	assertStatus(200, code, res)
	reservationID := id(res)
	code, replay := call("POST", "/api/v1/material/reservations", manager, reserveBody)
	assertStatus(200, code, replay)
	if id(replay) != reservationID {
		t.Fatal("reservation idempotency not durable")
	}
	code, _ = call("POST", "/api/v1/material/reservations", sign(otherTenant, "manager", "material.reserve", "", []string{"stores_manager"}, []string{"fido2"}, ""), reserveBody)
	if code == 200 {
		t.Fatal("cross-tenant roll access granted")
	}
	varianceBody := map[string]any{"reservation_id": reservationID, "roll_id": roll, "order_id": order, "cut_no": 1, "additional_mm": int64(10000), "actual_story": "Measured stains near end of roll; recut required", "reason_code": "DEFECT_RECUT", "idempotency_key": "http-variance"}
	code, vreq := call("POST", "/api/v1/material/variances", sign(tenant, "manager", "material.variance.request", "", []string{"stores_manager"}, []string{"webauthn"}, ""), varianceBody)
	assertStatus(200, code, vreq)
	varianceID := id(vreq)
	reviewURL := "/api/v1/material/variances/" + varianceID + "/challenge"
	code, _ = call("GET", reviewURL, sign(tenant, "deputy", "material.variance.review", "", []string{"ceo"}, []string{"fido2"}, ""), nil)
	if code == 200 {
		t.Fatal("unregistered deputy passed CEO enrollment")
	}
	code, review := call("GET", reviewURL, sign(tenant, "ceo-human", "material.variance.review", "", []string{"ceo"}, []string{"fido2"}, ""), nil)
	assertStatus(200, code, review)
	challenge, _ := review["challenge"].(string)
	if len(challenge) != 64 {
		t.Fatal("server variance challenge missing")
	}
	approveURL := "/api/v1/material/variances/" + varianceID + "/approve"
	code, _ = call("POST", approveURL, sign(tenant, "ceo-human", "material.variance.approve", "WRONG", []string{"ceo"}, []string{"fido2"}, ""), map[string]any{})
	if code == 200 {
		t.Fatal("CEO approval accepted wrong challenge")
	}
	approve := sign(tenant, "ceo-human", "material.variance.approve", challenge, []string{"ceo"}, []string{"webauthn"}, "unique-ceo-proof")
	code, approved := call("POST", approveURL, approve, nil)
	assertStatus(200, code, approved)
	if approved["status"] != "APPROVED" {
		t.Fatal("CEO approval not durable")
	}
	// Challenge preparation uses the same canonical fields as the later issue.
	issueBody := map[string]any{"reservation_id": reservationID, "roll_id": roll, "order_id": order, "cut_no": 1, "length_mm": int64(65000), "idempotency_key": "http-issue"}
	issueActor := sign(tenant, "stores-issuer", "material.issue.prepare", "", []string{"stores_issuer"}, []string{"fido2"}, "")
	code, challengeResp := call("POST", "/api/v1/material/issues/challenge", issueActor, issueBody)
	assertStatus(200, code, challengeResp)
	issueChallenge, _ := challengeResp["challenge"].(string)
	if len(issueChallenge) != 64 {
		t.Fatalf("missing issue challenge: %v", challengeResp)
	}
	issuerProof := sign(tenant, "stores-issuer", "material.issue.witness", issueChallenge, []string{"stores_issuer"}, []string{"fido2"}, "unique-issuer-proof")
	wrongRecipient := sign(tenant, "stores-issuer", "material.issue.witness", issueChallenge, []string{"stores_recipient"}, []string{"fido2"}, "wrong-recipient-proof")
	submit := sign(tenant, "stores-issuer", "material.issue.submit", "", []string{"stores_issuer"}, []string{"fido2"}, "")
	badBody := copyMap(issueBody)
	badBody["issuer_attestation"] = issuerProof
	badBody["recipient_attestation"] = wrongRecipient
	code, _ = call("POST", "/api/v1/material/issues", submit, badBody)
	if code == 200 {
		t.Fatal("same-human dual witness accepted")
	}
	recipientProof := sign(tenant, "stores-recipient", "material.issue.witness", issueChallenge, []string{"stores_recipient"}, []string{"webauthn"}, "unique-recipient-proof")
	badBody["recipient_attestation"] = recipientProof
	code, issued := call("POST", "/api/v1/material/issues", submit, badBody)
	assertStatus(200, code, issued)
	issueID := id(issued)
	code, issuedAgain := call("POST", "/api/v1/material/issues", submit, badBody)
	assertStatus(200, code, issuedAgain)
	if id(issuedAgain) != issueID {
		t.Fatal("issue replay was not idempotent")
	}
	// Reuse the same nonce IDs on another signed transaction; DB must roll back.
	different := copyMap(issueBody)
	different["idempotency_key"] = "http-issue-2"
	different["length_mm"] = int64(1000)
	code, c2 := call("POST", "/api/v1/material/issues/challenge", issueActor, different)
	assertStatus(200, code, c2)
	ch2, _ := c2["challenge"].(string)
	different["issuer_attestation"] = sign(tenant, "stores-issuer", "material.issue.witness", ch2, []string{"stores_issuer"}, []string{"fido2"}, "unique-issuer-proof")
	different["recipient_attestation"] = sign(tenant, "stores-recipient", "material.issue.witness", ch2, []string{"stores_recipient"}, []string{"fido2"}, "unique-recipient-proof")
	code, _ = call("POST", "/api/v1/material/issues", submit, different)
	if code == 200 {
		t.Fatal("two witness proof IDs reused across separate issuance")
	}
	var remain float64
	if err = admin.QueryRowContext(ctx, `SELECT remaining_length_m::float8 FROM store_rolls WHERE tenant_id=$1 AND id=$2`, tenant, roll).Scan(&remain); err != nil {
		t.Fatal(err)
	}
	if remain != 35 {
		t.Fatalf("expected 35m physically remaining after one issue, got %.3fm", remain)
	}
	var issueCount int
	if err = admin.QueryRowContext(ctx, `SELECT COUNT(*) FROM material_issues_v5 WHERE tenant_id=$1`, tenant).Scan(&issueCount); err != nil {
		t.Fatal(err)
	}
	if issueCount != 1 {
		t.Fatalf("expected 1 committed issuance, got %d", issueCount)
	}
}

func copyMap(src map[string]any) map[string]any {
	dst := make(map[string]any, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}
