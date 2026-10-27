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

// Explicit opt-in. Only creates/removes two random fixture tenants and their
// own passport / reconciliation rows. Never edits the authoritative ledger.
func TestStoresReconciliationPostgresIsolationAndIdempotency(t *testing.T) {
	adminURL, appURL := os.Getenv("AGIPAREL_TEST_ADMIN_DATABASE_URL"), os.Getenv("AGIPAREL_TEST_APP_DATABASE_URL")
	if adminURL == "" || appURL == "" {
		t.Skip("requires admin and restricted app database URLs")
	}
	if os.Getenv("AGIPAREL_DESTRUCTIVE_DB_TESTS") != "I_UNDERSTAND_TEST_FIXTURES" {
		t.Fatal("explicit isolated-fixture opt-in required")
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
	ctx, cancel := context.WithTimeout(context.Background(), 35*time.Second)
	defer cancel()
	for _, db := range []*sql.DB{admin, app} {
		if e = db.PingContext(ctx); e != nil {
			t.Fatal(e)
		}
	}
	var role string
	if e = app.QueryRowContext(ctx, "SELECT current_user").Scan(&role); e != nil || role != "agiparel_app" {
		t.Fatalf("only agiparel_app allowed got %q %v", role, e)
	}
	var seed [12]byte
	if _, e = rand.Read(seed[:]); e != nil {
		t.Fatal(e)
	}
	id := hex.EncodeToString(seed[:])
	tenantA := "S7B-" + id
	tenantB := "S7C-" + id
	for _, ten := range []string{tenantA, tenantB} {
		if _, e = admin.ExecContext(ctx, "INSERT INTO tenants(id,name) VALUES($1,$2)", ten, "07B isolated test fixture"); e != nil {
			t.Fatal(e)
		}
	}
	t.Cleanup(func() {
		for _, ten := range []string{tenantA, tenantB} {
			_, _ = admin.ExecContext(context.Background(), "DELETE FROM store_reconciliation_requests_v7 WHERE tenant_id=$1", ten)
			_, _ = admin.ExecContext(context.Background(), "DELETE FROM store_roll_passport_events_v7 WHERE tenant_id=$1", ten)
			_, _ = admin.ExecContext(context.Background(), "DELETE FROM store_roll_rfid_bindings_v7 WHERE tenant_id=$1", ten)
			_, _ = admin.ExecContext(context.Background(), "DELETE FROM tenants WHERE id=$1", ten)
		}
	})
	t.Setenv("ENV", "development")
	s := &EdgeServer{db: app}
	call := func(tenant, method, path string, body any) (int, map[string]any) {
		t.Helper()
		t.Setenv("AGIPAREL_OPERATOR_TENANT", tenant)
		raw, _ := json.Marshal(body)
		req := httptest.NewRequest(method, path, bytes.NewReader(raw))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		switch path {
		case "/api/v1/stores/passports":
			s.handleStoresPassports(w, req)
		case "/api/v1/stores/reconciliation":
			s.handleStoresReconciliation(w, req)
		case "/api/v1/stores/reconciliation-requests":
			s.handleStoresReconciliationRequests(w, req)
		default:
			t.Fatal("unexpected test endpoint")
		}
		obj := map[string]any{}
		_ = json.Unmarshal(w.Body.Bytes(), &obj)
		if w.Code >= 500 {
			t.Fatalf("endpoint %s %s failed with %d: %s", method, path, w.Code, w.Body.String())
		}
		return w.Code, obj
	}
	p := samplePassport()
	p.EPC = ""
	p.RollID = "ROLL-B7B"
	code, _ := call(tenantA, http.MethodPost, "/api/v1/stores/passports", p)
	if code != 201 {
		t.Fatalf("passport code %d", code)
	}
	req := storesReconciliationInput{RollID: p.RollID, ExpectedPassportRevision: 1, RequestKey: "review_" + id, Note: "Please reconcile the observed roll with warehouse ledger"}
	code, obj := call(tenantA, http.MethodPost, "/api/v1/stores/reconciliation", req)
	if code != 201 {
		t.Fatalf("review request code=%d body=%v", code, obj)
	}
	rec := obj["request"].(map[string]any)
	if rec["state"] != "PENDING_INDEPENDENT_RECONCILIATION" {
		t.Fatalf("unauthorized state: %v", rec)
	}
	assessment := rec["assessment"].(map[string]any)
	if assessment["state"] != "REQUIRES_REVIEW" {
		t.Fatalf("missing ledger not flagged: %v", assessment)
	}
	code, replay := call(tenantA, http.MethodPost, "/api/v1/stores/reconciliation", req)
	if code != 200 || replay["replayed"] != true {
		t.Fatalf("not idempotent: %d %v", code, replay)
	}
	altered := req
	altered.Note = "The meaning has changed; reject reused request key"
	code, _ = call(tenantA, http.MethodPost, "/api/v1/stores/reconciliation", altered)
	if code != 409 {
		t.Fatalf("request key reuse not rejected %d", code)
	}
	code, history := call(tenantB, http.MethodGet, "/api/v1/stores/reconciliation-requests", nil)
	if code != 200 || len(history["items"].([]any)) != 0 {
		t.Fatalf("tenant data leak %d %v", code, history)
	}
	code, records := call(tenantB, http.MethodGet, "/api/v1/stores/reconciliation", nil)
	if code != 200 || len(records["items"].([]any)) != 0 {
		t.Fatalf("tenant roll leak %d %v", code, records)
	}
	p.ExpectedRevision = 1
	p.MeasuredLengthM = "98.500"
	code, _ = call(tenantA, http.MethodPost, "/api/v1/stores/passports", p)
	if code != 201 {
		t.Fatalf("passport revision code %d", code)
	}
	late := req
	late.RequestKey = "late_" + id
	code, _ = call(tenantA, http.MethodPost, "/api/v1/stores/reconciliation", late)
	if code != 409 {
		t.Fatalf("stale revision not rejected: %d", code)
	}
	if _, e = app.ExecContext(ctx, "UPDATE store_reconciliation_requests_v7 SET note='changed' WHERE tenant_id=$1", tenantA); e == nil {
		t.Fatal("restricted role modified immutable reconciliation")
	}
	var stockRows int
	if e = admin.QueryRowContext(ctx, "SELECT count(*) FROM store_rolls WHERE tenant_id=$1", tenantA).Scan(&stockRows); e != nil || stockRows != 0 {
		t.Fatalf("review created authoritative stock rows: count=%d %v", stockRows, e)
	}
}
