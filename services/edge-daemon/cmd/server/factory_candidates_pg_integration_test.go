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

// Integration test is explicit opt-in, uses a random scratch tenant, and never
// resets a schema. Admin fixture cleanup is tenant-scoped and in FK order.
func TestFactoryCandidatesPostgresReviewBoundary(t *testing.T) {
	adminURL, appURL := os.Getenv("AGIPAREL_TEST_ADMIN_DATABASE_URL"), os.Getenv("AGIPAREL_TEST_APP_DATABASE_URL")
	if adminURL == "" || appURL == "" {
		t.Skip("requires admin and restricted-app PostgreSQL fixture URLs")
	}
	if os.Getenv("AGIPAREL_DESTRUCTIVE_DB_TESTS") != "I_UNDERSTAND_TEST_FIXTURES" {
		t.Fatal("fixture cleanup opt-in required")
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
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()
	if e = admin.PingContext(ctx); e != nil {
		t.Fatal(e)
	}
	if e = app.PingContext(ctx); e != nil {
		t.Fatal(e)
	}
	var current string
	if e = app.QueryRowContext(ctx, `SELECT current_user`).Scan(&current); e != nil || current != "agiparel_app" {
		t.Fatalf("restricted role expected got %q (%v)", current, e)
	}
	var r [8]byte
	if _, e = rand.Read(r[:]); e != nil {
		t.Fatal(e)
	}
	tenant := "FB2-" + hex.EncodeToString(r[:])
	other := "FX2-" + hex.EncodeToString(r[:])
	doc := "DOC-" + hex.EncodeToString(append(r[:], r[:]...))
	cleanup := func() {
		// Do not touch any other tenant's files, models or other apps.
		for _, table := range []string{"factory_proposed_register_v65b2", "factory_candidate_reviews_v65b2", "factory_document_candidates_v65b2", "factory_fact_events_v65b", "document_segments_v65", "document_jobs_v65", "document_sources_v65"} {
			switch table {
			case "factory_proposed_register_v65b2", "factory_candidate_reviews_v65b2", "factory_document_candidates_v65b2", "factory_fact_events_v65b", "document_segments_v65", "document_jobs_v65", "document_sources_v65":
				_, _ = admin.ExecContext(context.Background(), `DELETE FROM `+table+` WHERE tenant_id=$1`, tenant)
			}
		}
	}
	t.Cleanup(cleanup)
	hash := hex.EncodeToString(append(r[:], r[:]...)) + hex.EncodeToString(append(r[:], r[:]...))
	_, e = admin.ExecContext(ctx, `INSERT INTO document_sources_v65(tenant_id,id,filename,media_type,format,byte_length,sha256,original)
 VALUES($1,$2,'machine-list.csv','text/csv','CSV',1,$3,$4)`, tenant, doc, hash, []byte("x"))
	if e != nil {
		t.Fatal(e)
	}
	_, e = admin.ExecContext(ctx, `INSERT INTO document_jobs_v65(tenant_id,document_id,status,segment_count)
 VALUES($1,$2,'EXTRACTED',2)`, tenant, doc)
	if e != nil {
		t.Fatal(e)
	}
	for i, content := range []string{`["machine_id","machine_type"]`, `["MC-12","Lockstitch"]`} {
		loc, _ := json.Marshal(map[string]int{"row": i + 1})
		_, e = admin.ExecContext(ctx, `INSERT INTO document_segments_v65(tenant_id,document_id,ordinal,kind,locator,content)
 VALUES($1,$2,$3,'ROW',$4::jsonb,$5)`, tenant, doc, i+1, string(loc), content)
		if e != nil {
			t.Fatal(e)
		}
	}
	t.Setenv("ENV", "development")
	t.Setenv("AGIPAREL_OPERATOR_TENANT", tenant)
	server := &EdgeServer{db: app}
	invoke := func(method, path string, payload any, call func(http.ResponseWriter, *http.Request)) (int, map[string]any) {
		t.Helper()
		raw, _ := json.Marshal(payload)
		req := httptest.NewRequest(method, path, bytes.NewReader(raw))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		call(w, req)
		body := map[string]any{}
		_ = json.Unmarshal(w.Body.Bytes(), &body)
		return w.Code, body
	}
	path := "/api/v1/factory-onboarding/candidates/scan"
	code, result := invoke("POST", path, map[string]string{"document_id": doc}, server.handleFactoryCandidateScan)
	if code != 200 || result["new_candidates"] != float64(1) {
		t.Fatalf("scan HTTP=%d result=%+v", code, result)
	}
	code, result = invoke("POST", path, map[string]string{"document_id": doc}, server.handleFactoryCandidateScan)
	if code != 200 || result["new_candidates"] != float64(0) {
		t.Fatalf("duplicate staging not idempotent: %d %+v", code, result)
	}
	code, result = invoke("GET", "/api/v1/factory-onboarding/candidates", nil, server.handleFactoryCandidates)
	if code != 200 {
		t.Fatalf("candidate read HTTP %d %+v", code, result)
	}
	rows := result["items"].([]any)
	if len(rows) != 1 {
		t.Fatalf("wanted one candidate %+v", result)
	}
	id := rows[0].(map[string]any)["id"].(string)
	code, result = invoke("POST", "/api/v1/factory-onboarding/candidates/review", map[string]string{"candidate_id": id, "decision": "ACCEPT_PROVISIONAL"}, server.handleFactoryCandidateReview)
	if code != 200 || result["operational_master_updated"] != false {
		t.Fatalf("review boundary %d %+v", code, result)
	}
	code, _ = invoke("POST", "/api/v1/factory-onboarding/candidates/review", map[string]string{"candidate_id": id, "decision": "ACCEPT_PROVISIONAL"}, server.handleFactoryCandidateReview)
	if code != 409 {
		t.Fatalf("repeated decision should conflict got %d", code)
	}
	code, result = invoke("GET", "/api/v1/factory-onboarding/proposed-register", nil, server.handleFactoryRegisterProposals)
	if code != 200 || len(result["items"].([]any)) != 1 {
		t.Fatalf("proposed register %d %+v", code, result)
	}
	// App role is forbidden to alter evidence or promote any master via UPDATE.
	if _, e = app.ExecContext(ctx, `UPDATE factory_proposed_register_v65b2 SET review_state='CONFIRMED' WHERE tenant_id=$1`, tenant); e == nil {
		t.Fatal("app could update proposed evidence")
	}
	t.Setenv("AGIPAREL_OPERATOR_TENANT", other)
	code, result = invoke("GET", "/api/v1/factory-onboarding/candidates", nil, server.handleFactoryCandidates)
	if code != 200 || len(result["items"].([]any)) != 0 {
		t.Fatalf("tenant isolation failed %d %+v", code, result)
	}
}
