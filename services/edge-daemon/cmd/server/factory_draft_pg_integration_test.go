package main

import (
	"bytes"
	"context"
	"crypto/rand"
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

// Explicit opt-in, append-only B3 PostgreSQL integration test. Isolates its own
// random scratch tenant and cleans *only* its own fixture rows, in FK order.
func TestFactoryDraftReviewPostgresIsolationAndRevisions(t *testing.T) {
	adminURL, appURL := os.Getenv("AGIPAREL_TEST_ADMIN_DATABASE_URL"), os.Getenv("AGIPAREL_TEST_APP_DATABASE_URL")
	if adminURL == "" || appURL == "" {
		t.Skip("requires restricted-app and admin fixture URLs")
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
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()
	if err = admin.PingContext(ctx); err != nil {
		t.Fatal(err)
	}
	if err = app.PingContext(ctx); err != nil {
		t.Fatal(err)
	}
	var role string
	if err = app.QueryRowContext(ctx, `SELECT current_user`).Scan(&role); err != nil || role != "agiparel_app" {
		t.Fatalf("need restricted app role, got %q (%v)", role, err)
	}
	var r [8]byte
	if _, err = rand.Read(r[:]); err != nil {
		t.Fatal(err)
	}
	tenant := "FB3-" + hex.EncodeToString(r[:])
	other := "FC3-" + hex.EncodeToString(r[:])
	t.Cleanup(func() {
		for _, table := range []string{"factory_draft_decisions_v65b3", "factory_proposed_register_v65b2", "factory_candidate_reviews_v65b2", "factory_document_candidates_v65b2", "factory_fact_events_v65b", "document_segments_v65", "document_jobs_v65", "document_sources_v65"} {
			_, _ = admin.ExecContext(context.Background(), `DELETE FROM `+table+` WHERE tenant_id=$1`, tenant)
		}
	})
	idBytes := append(append([]byte{}, r[:]...), r[:]...)
	hash := hex.EncodeToString(idBytes) + hex.EncodeToString(idBytes)
	ids := []string{"FC-" + hex.EncodeToString(idBytes), "FC-" + hex.EncodeToString(append(append([]byte{}, r[:]...), []byte{1, 2, 3, 4, 5, 6, 7, 8}...))}
	docs := []string{"DOC-" + hex.EncodeToString(idBytes), "DOC-" + hex.EncodeToString(append(append([]byte{}, r[:]...), []byte{2, 3, 4, 5, 6, 7, 8, 9}...))}
	for i, doc := range docs {
		fingerprint := hash
		if i == 1 {
			fingerprint = hex.EncodeToString(append(append([]byte{}, r[:]...), r[:]...)) + hex.EncodeToString(append(append([]byte{}, r[:]...), []byte{9, 9, 9, 9, 9, 9, 9, 9}...))
		}
		_, err = admin.ExecContext(ctx, `INSERT INTO document_sources_v65(tenant_id,id,filename,media_type,format,byte_length,sha256,original) VALUES($1,$2,$3,'text/csv','CSV',1,$4,$5)`, tenant, doc, []string{"machines-A.csv", "machines-B.csv"}[i], fingerprint, []byte("x"))
		if err != nil {
			t.Fatal(err)
		}
		_, err = admin.ExecContext(ctx, `INSERT INTO document_jobs_v65(tenant_id,document_id,status,segment_count) VALUES($1,$2,'EXTRACTED',1)`, tenant, doc)
		if err != nil {
			t.Fatal(err)
		}
		_, err = admin.ExecContext(ctx, `INSERT INTO document_segments_v65(tenant_id,document_id,ordinal,kind,locator,content) VALUES($1,$2,1,'ROW','{"row":1}'::jsonb,'["machine_id","machine_type"]')`, tenant, doc)
		if err != nil {
			t.Fatal(err)
		}
		typ := []string{"Lockstitch", "Overlock"}[i]
		attrs, _ := json.Marshal(map[string]string{"machine_id": "MC-100", "machine_type": typ})
		_, err = admin.ExecContext(ctx, `INSERT INTO factory_document_candidates_v65b2
  (tenant_id,candidate_id,document_id,source_ordinal,source_locator,entity_type,entity_key,proposed_value,attributes,explanation,fingerprint)
  VALUES($1,$2,$3,1,'{"row":1}'::jsonb,'MACHINE','MC-100',$4,$5::jsonb,'Source evidence only',$6)`, tenant, ids[i], doc, "MC-100 · "+typ, string(attrs), fingerprint)
		if err != nil {
			t.Fatal(err)
		}
		_, err = admin.ExecContext(ctx, `INSERT INTO factory_candidate_reviews_v65b2(tenant_id,candidate_id,decision,reviewed_by) VALUES($1,$2,'ACCEPT_PROVISIONAL','dev')`, tenant, ids[i])
		if err != nil {
			t.Fatal(err)
		}
		_, err = admin.ExecContext(ctx, `INSERT INTO factory_proposed_register_v65b2(tenant_id,candidate_id,entity_type,entity_key,attributes) VALUES($1,$2,'MACHINE','MC-100',$3::jsonb)`, tenant, ids[i], string(attrs))
		if err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("ENV", "development")
	t.Setenv("AGIPAREL_OPERATOR_TENANT", tenant)
	srv := &EdgeServer{db: app}
	call := func(method, path string, input any, handler func(http.ResponseWriter, *http.Request)) (int, map[string]any) {
		t.Helper()
		raw, _ := json.Marshal(input)
		req := httptest.NewRequest(method, path, bytes.NewReader(raw))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		handler(w, req)
		result := map[string]any{}
		_ = json.Unmarshal(w.Body.Bytes(), &result)
		return w.Code, result
	}
	code, result := call("GET", "/api/v1/factory-onboarding/draft-masters", nil, srv.handleFactoryDraftMasters)
	if code != 200 {
		t.Fatalf("read %d %+v", code, result)
	}
	groups, ok := result["items"].([]any)
	if !ok || len(groups) != 1 {
		t.Fatalf("expected one group %+v", result)
	}
	group := groups[0].(map[string]any)
	if group["evidence_state"] != "CONFLICT" || group["distinct_versions"] != float64(2) {
		t.Fatalf("missed disagreement %+v", group)
	}
	review := map[string]any{"entity_type": "MACHINE", "entity_key": "MC-100", "candidate_id": ids[0], "decision": "VERIFY_DRAFT", "expected_revision": 0, "note": "Machine plate inspected on site"}
	code, result = call("POST", "/api/v1/factory-onboarding/draft-masters/review", review, srv.handleFactoryDraftReview)
	if code != 200 || result["revision"] != float64(1) || result["operational_master_updated"] != false {
		t.Fatalf("review %d %+v", code, result)
	}
	code, result = call("POST", "/api/v1/factory-onboarding/draft-masters/review", review, srv.handleFactoryDraftReview)
	if code != 409 {
		t.Fatalf("optimistic lock bypassed %d %+v", code, result)
	}
	review["expected_revision"] = 1
	review["note"] = "Not enough evidence"
	review["decision"] = "NEEDS_CLARIFICATION"
	code, result = call("POST", "/api/v1/factory-onboarding/draft-masters/review", review, srv.handleFactoryDraftReview)
	if code != 200 || result["revision"] != float64(2) {
		t.Fatalf("second review failed %d %+v", code, result)
	}
	if _, err = app.ExecContext(ctx, `UPDATE factory_draft_decisions_v65b3 SET decision='VERIFY_DRAFT' WHERE tenant_id=$1`, tenant); err == nil {
		t.Fatal("app role unexpectedly can rewrite review history")
	}
	t.Setenv("AGIPAREL_OPERATOR_TENANT", other)
	code, result = call("GET", "/api/v1/factory-onboarding/draft-masters", nil, srv.handleFactoryDraftMasters)
	if code != 200 || len(result["items"].([]any)) != 0 {
		t.Fatalf("cross-tenant exposure %d %+v", code, result)
	}
}
