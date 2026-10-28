// Author: Sunil Jain, 2026-10-27
// Opt-in PG16 integration: tenant isolation, corrected field reviews, audited atomic commit,
// idempotence and fail-closed post-submission scanning.
package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func TestFactoryApprovalPostgresAtomicCommit(t *testing.T) {
	adminURL, appURL := os.Getenv("AGIPAREL_TEST_ADMIN_DATABASE_URL"), os.Getenv("AGIPAREL_TEST_APP_DATABASE_URL")
	if adminURL == "" || appURL == "" {
		t.Skip("requires explicit PostgreSQL integration URLs")
	}
	if os.Getenv("AGIPAREL_DESTRUCTIVE_DB_TESTS") != "I_UNDERSTAND_TEST_FIXTURES" {
		t.Fatal("fixture cleanup opt-in required")
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
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	if err = admin.PingContext(ctx); err != nil {
		t.Fatal(err)
	}
	if err = app.PingContext(ctx); err != nil {
		t.Fatal(err)
	}
	var role string
	if err = app.QueryRowContext(ctx, `SELECT current_user`).Scan(&role); err != nil || role != "agiparel_app" {
		t.Fatalf("restricted app role required: %s %v", role, err)
	}
	var nonce [16]byte
	if _, err = rand.Read(nonce[:]); err != nil {
		t.Fatal(err)
	}
	tenant := "FAS-" + hex.EncodeToString(nonce[:8])
	other := "FOX-" + hex.EncodeToString(nonce[:8])
	doc := "DOC-" + hex.EncodeToString(nonce[:])
	hash := sha256.Sum256(nonce[:])
	sha := hex.EncodeToString(hash[:])
	t.Cleanup(func() {
		for _, table := range []string{
			"factory_approved_reference_versions_v65b5", "factory_document_submissions_v65b5", "factory_candidate_review_versions_v65b5",
			"factory_proposed_register_v65b2", "factory_candidate_reviews_v65b2", "factory_document_candidates_v65b2",
			"factory_fact_events_v65b", "document_segments_v65", "document_jobs_v65", "document_sources_v65",
		} {
			_, _ = admin.ExecContext(context.Background(), `DELETE FROM `+table+` WHERE tenant_id=$1`, tenant)
		}
	})
	if _, err = admin.ExecContext(ctx, `INSERT INTO document_sources_v65(tenant_id,id,filename,media_type,format,byte_length,sha256,original)
 VALUES($1,$2,'machine-test.csv','text/csv','CSV',1,$3,$4)`, tenant, doc, sha, []byte("x")); err != nil {
		t.Fatal(err)
	}
	if _, err = admin.ExecContext(ctx, `INSERT INTO document_jobs_v65(tenant_id,document_id,status,segment_count) VALUES($1,$2,'EXTRACTED',2)`, tenant, doc); err != nil {
		t.Fatal(err)
	}
	for i, content := range []string{`["machine_id","machine_type"]`, `["MC-12","Lockstitch"]`} {
		loc, _ := json.Marshal(map[string]int{"row": i + 1})
		if _, err = admin.ExecContext(ctx, `INSERT INTO document_segments_v65(tenant_id,document_id,ordinal,kind,locator,content)
 VALUES($1,$2,$3,'ROW',$4::jsonb,$5)`, tenant, doc, i+1, string(loc), content); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("ENV", "development")
	t.Setenv("AGIPAREL_OPERATOR_TENANT", tenant)
	t.Setenv("AGIPAREL_OPERATOR_USER", "dev")
	t.Setenv("AGIPAREL_OPERATOR_PASSWORD", "safe-dev-pass")
	t.Setenv("AGIPAREL_REVIEW_MANAGER_USER", "dev")
	server := &EdgeServer{db: app}
	invoke := func(method, path string, body any, handler func(http.ResponseWriter, *http.Request)) (int, map[string]any) {
		t.Helper()
		raw, _ := json.Marshal(body)
		req := httptest.NewRequest(method, path, bytes.NewReader(raw))
		req.Header.Set("Content-Type", "application/json")
		req.SetBasicAuth("dev", "safe-dev-pass")
		recorder := httptest.NewRecorder()
		handler(recorder, req)
		out := map[string]any{}
		_ = json.Unmarshal(recorder.Body.Bytes(), &out)
		return recorder.Code, out
	}
	code, result := invoke("POST", "/api/v1/factory-onboarding/candidates/scan", map[string]string{"document_id": doc}, server.handleFactoryCandidateScan)
	if code != 200 || result["new_candidates"] != float64(1) {
		t.Fatalf("scan: %d %+v", code, result)
	}
	code, result = invoke("GET", "/api/v1/factory-onboarding/approvals?document_id="+doc, nil, server.handleFactoryApprovalList)
	if code != 200 {
		t.Fatalf("list: %d %+v", code, result)
	}
	entries := result["items"].([]any)
	if len(entries) != 1 {
		t.Fatalf("expected 1 candidate got %d", len(entries))
	}
	id := entries[0].(map[string]any)["id"].(string)
	edited := map[string]string{"machine_id": "MC-12", "machine_type": "Overlock", "location_note": "Line 3"}
	review := approvalReviewInput{CandidateID: id, Action: "KEEP", Attributes: edited, CheckedFields: []string{"machine_id", "machine_type", "location_note"}, ExpectedRevision: 0}
	code, result = invoke("POST", "/api/v1/factory-onboarding/approvals/review", review, server.handleFactoryApprovalReview)
	if code != 200 || result["revision"] != float64(1) {
		t.Fatalf("save: %d %+v", code, result)
	}
	code, _ = invoke("POST", "/api/v1/factory-onboarding/approvals/review", review, server.handleFactoryApprovalReview)
	if code != 409 {
		t.Fatalf("stale edit not rejected: %d", code)
	}
	submit := approvalCommitInput{DocumentID: doc, ExpectedCount: 1, Confirmation: "SUBMIT"}
	code, result = invoke("POST", "/api/v1/factory-onboarding/approvals/submit", submit, server.handleFactoryApprovalSubmit)
	if code != 200 || result["operational_master_updated"] != false {
		t.Fatalf("final commit: %d %+v", code, result)
	}
	var rows int
	if err = admin.QueryRowContext(ctx, `SELECT COUNT(*) FROM factory_approved_reference_versions_v65b5 WHERE tenant_id=$1 AND document_id=$2 AND attributes->>'machine_type'='Overlock'`, tenant, doc).Scan(&rows); err != nil || rows != 1 {
		t.Fatalf("audited snapshot missing: %d %v", rows, err)
	}
	code, _ = invoke("POST", "/api/v1/factory-onboarding/approvals/submit", submit, server.handleFactoryApprovalSubmit)
	if code != 409 {
		t.Fatalf("repeated commit not blocked: %d", code)
	}
	code, _ = invoke("POST", "/api/v1/factory-onboarding/candidates/scan", map[string]string{"document_id": doc}, server.handleFactoryCandidateScan)
	if code != 409 {
		t.Fatalf("post approval scan not blocked: %d", code)
	}
	t.Setenv("AGIPAREL_OPERATOR_TENANT", other)
	code, result = invoke("GET", "/api/v1/factory-onboarding/approvals?document_id="+doc, nil, server.handleFactoryApprovalList)
	if code != 200 || len(result["items"].([]any)) != 0 {
		t.Fatalf("tenant read isolation: %d %+v", code, result)
	}
}
