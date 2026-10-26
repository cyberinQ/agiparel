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

// Opt-in integration test. Only deletes its own random scratch tenant.
// Requires 0009, 0011, 0012 and 0013. No factory operational tables are changed.
func TestFactoryReferencePostgresRevisionsAndTenantIsolation(t *testing.T) {
	adminURL, appURL := os.Getenv("AGIPAREL_TEST_ADMIN_DATABASE_URL"), os.Getenv("AGIPAREL_TEST_APP_DATABASE_URL")
	if adminURL == "" || appURL == "" {
		t.Skip("requires admin and restricted-app test connections")
	}
	if os.Getenv("AGIPAREL_DESTRUCTIVE_DB_TESTS") != "I_UNDERSTAND_TEST_FIXTURES" {
		t.Fatal("fixture cleanup must be explicitly allowed")
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
		t.Fatalf("restricted role required, got %q %v", role, err)
	}
	var random [16]byte
	if _, err = rand.Read(random[:]); err != nil {
		t.Fatal(err)
	}
	hexID := hex.EncodeToString(random[:])
	tenant := "BF4-" + hexID[:16]
	other := "BFX-" + hexID[:16]
	source := "DOC-" + hexID
	candidate := "FC-" + hexID
	t.Cleanup(func() {
		for _, table := range []string{"factory_reference_versions_v65b4", "factory_draft_decisions_v65b3", "factory_proposed_register_v65b2", "factory_candidate_reviews_v65b2", "factory_document_candidates_v65b2", "document_segments_v65", "document_jobs_v65", "document_sources_v65"} {
			_, _ = admin.ExecContext(context.Background(), `DELETE FROM `+table+` WHERE tenant_id=$1`, tenant)
		}
	})
	h := hexID + hexID
	fixture := []struct {
		q    string
		args []any
	}{
		{`INSERT INTO document_sources_v65(tenant_id,id,filename,media_type,format,byte_length,sha256,original) VALUES($1,$2,'machine_list.csv','text/csv','CSV',1,$3,$4)`, []any{tenant, source, h, []byte("x")}},
		{`INSERT INTO document_jobs_v65(tenant_id,document_id,status,segment_count) VALUES($1,$2,'EXTRACTED',1)`, []any{tenant, source}},
		{`INSERT INTO document_segments_v65(tenant_id,document_id,ordinal,kind,locator,content) VALUES($1,$2,1,'ROW','{"row":2}'::jsonb,'["MC-1","Lockstitch"]')`, []any{tenant, source}},
		{`INSERT INTO factory_document_candidates_v65b2(tenant_id,candidate_id,document_id,source_ordinal,source_locator,entity_type,entity_key,proposed_value,attributes,explanation,fingerprint) VALUES($1,$2,$3,1,'{"row":2}'::jsonb,'MACHINE','MC-1','MC-1 · Lockstitch','{"machine_id":"MC-1","machine_type":"Lockstitch"}'::jsonb,'Source identified',$4)`, []any{tenant, candidate, h}},
		{`INSERT INTO factory_candidate_reviews_v65b2(tenant_id,candidate_id,decision,reviewed_by) VALUES($1,$2,'ACCEPT_PROVISIONAL','dev')`, []any{tenant, candidate}},
		{`INSERT INTO factory_proposed_register_v65b2(tenant_id,candidate_id,entity_type,entity_key,attributes) VALUES($1,$2,'MACHINE','MC-1','{"machine_id":"MC-1","machine_type":"Lockstitch"}'::jsonb)`, []any{tenant, candidate}},
		{`INSERT INTO factory_draft_decisions_v65b3(tenant_id,entity_type,entity_key,revision,candidate_id,decision,review_note,reviewed_by) VALUES($1,'MACHINE','MC-1',1,$2,'VERIFY_DRAFT','Machine plate and type checked','dev')`, []any{tenant, candidate}},
	}
	for _, f := range fixture {
		if _, err = admin.ExecContext(ctx, f.q, f.args...); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("ENV", "development")
	t.Setenv("AGIPAREL_OPERATOR_TENANT", tenant)
	srv := &EdgeServer{db: app}
	do := func(method, path string, body any, handler func(http.ResponseWriter, *http.Request)) (int, map[string]any) {
		t.Helper()
		raw, _ := json.Marshal(body)
		r := httptest.NewRequest(method, path, bytes.NewReader(raw))
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		handler(w, r)
		out := map[string]any{}
		_ = json.Unmarshal(w.Body.Bytes(), &out)
		return w.Code, out
	}
	request := referenceMasterRequest{EntityType: "MACHINE", EntityKey: "MC-1", CandidateID: candidate, DraftRevision: 1, Action: "PUBLISH", Note: "Machine plate physically checked", Acknowledged: true}
	code, res := do("POST", "/api/v1/factory-onboarding/reference-masters/revise", request, srv.handleFactoryReferenceRevision)
	if code != 200 || res["authority"] != "FACTORY_REFERENCE_ONLY" || res["operational_master_updated"] != false {
		t.Fatalf("publish %d %+v", code, res)
	}
	code, res = do("GET", "/api/v1/factory-onboarding/reference-masters", nil, srv.handleFactoryReferenceMasters)
	if code != 200 || len(res["items"].([]any)) != 1 {
		t.Fatalf("catalog %d %+v", code, res)
	}
	code, _ = do("POST", "/api/v1/factory-onboarding/reference-masters/revise", request, srv.handleFactoryReferenceRevision)
	if code != 409 {
		t.Fatalf("stale revision must conflict; got %d", code)
	}
	if _, err = app.ExecContext(ctx, `UPDATE factory_reference_versions_v65b4 SET action='WITHDRAW' WHERE tenant_id=$1`, tenant); err == nil {
		t.Fatal("restricted app can rewrite reference history")
	}
	request.Action = "WITHDRAW"
	request.ExpectedRevision = 1
	request.Note = "Machine retired after service check"
	code, res = do("POST", "/api/v1/factory-onboarding/reference-masters/revise", request, srv.handleFactoryReferenceRevision)
	if code != 200 || res["revision"] != float64(2) {
		t.Fatalf("withdraw %d %+v", code, res)
	}
	code, res = do("GET", "/api/v1/factory-onboarding/reference-masters?history=1", nil, srv.handleFactoryReferenceMasters)
	if code != 200 || len(res["items"].([]any)) != 2 {
		t.Fatalf("immutable history %d %+v", code, res)
	}
	t.Setenv("AGIPAREL_OPERATOR_TENANT", other)
	code, res = do("GET", "/api/v1/factory-onboarding/reference-masters", nil, srv.handleFactoryReferenceMasters)
	if code != 200 || len(res["items"].([]any)) != 0 {
		t.Fatalf("cross-tenant reference leak %d %+v", code, res)
	}
}
