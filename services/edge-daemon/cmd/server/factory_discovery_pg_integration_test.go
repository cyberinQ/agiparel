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
	"strings"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

// Explicit opt-in: the real database test requires migrations 0009 and 0010.
// Fixtures live in isolated random tenant IDs, and cleanup touches only the
// factory_fact_events_v65b rows created by this test.
func TestFactoryDiscoveryPostgresTenantAndRevisions(t *testing.T) {
	adminURL, appURL := os.Getenv("AGIPAREL_TEST_ADMIN_DATABASE_URL"), os.Getenv("AGIPAREL_TEST_APP_DATABASE_URL")
	if adminURL == "" || appURL == "" {
		t.Skip("requires isolated PostgreSQL fixture URLs")
	}
	if os.Getenv("AGIPAREL_DESTRUCTIVE_DB_TESTS") != "I_UNDERSTAND_TEST_FIXTURES" {
		t.Fatal("fixture opt-in required")
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
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	for _, db := range []*sql.DB{admin, app} {
		if e = db.PingContext(ctx); e != nil {
			t.Fatal(e)
		}
	}
	var role string
	if e = app.QueryRowContext(ctx, `SELECT current_user`).Scan(&role); e != nil || role != "agiparel_app" {
		t.Fatalf("expected restricted role, got %q err=%v", role, e)
	}
	var rnd [8]byte
	if _, e = rand.Read(rnd[:]); e != nil {
		t.Fatal(e)
	}
	tenant := "FD-" + hex.EncodeToString(rnd[:])
	other := "FO-" + hex.EncodeToString(rnd[:])
	t.Cleanup(func() {
		_, _ = admin.ExecContext(context.Background(), `DELETE FROM factory_fact_events_v65b WHERE tenant_id IN ($1,$2)`, tenant, other)
	})
	t.Setenv("ENV", "development")
	t.Setenv("AGIPAREL_OPERATOR_TENANT", tenant)
	server := &EdgeServer{db: app}
	call := func(method, path string, body any) (int, map[string]any) {
		t.Helper()
		var raw []byte
		if body != nil {
			raw, _ = json.Marshal(body)
		}
		req := httptest.NewRequest(method, path, bytes.NewReader(raw))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		if method == http.MethodPost {
			server.handleFactoryFact(w, req)
		} else {
			server.handleFactoryDiscovery(w, req)
		}
		var res map[string]any
		_ = json.Unmarshal(w.Body.Bytes(), &res)
		return w.Code, res
	}
	code, _ := call(http.MethodGet, "/api/v1/factory-onboarding", nil)
	if code != 200 {
		t.Fatalf("GET initial %d", code)
	}
	payload := factorySubmit{FieldKey: "machines.sewing_count", Value: "42", State: "PROVISIONAL", ExpectedRevision: 0}
	code, res := call(http.MethodPost, "/api/v1/factory-onboarding/facts", payload)
	if code != 201 {
		t.Fatalf("POST evidence %d %v", code, res)
	}
	code, _ = call(http.MethodPost, "/api/v1/factory-onboarding/facts", payload)
	if code != 409 {
		t.Fatalf("expected optimistic revision conflict, got %d", code)
	}
	payload.ExpectedRevision = 1
	payload.State = "CONFIRMED"
	code, _ = call(http.MethodPost, "/api/v1/factory-onboarding/facts", payload)
	if code != 201 {
		t.Fatalf("update %d", code)
	}
	code, res = call(http.MethodGet, "/api/v1/factory-onboarding", nil)
	if code != 200 {
		t.Fatalf("GET tenant %d", code)
	}
	facts, ok := res["facts"].(map[string]any)
	if !ok {
		t.Fatal("no facts")
	}
	machine := facts["machines.sewing_count"].(map[string]any)
	if machine["state"] != "CONFIRMED" || machine["revision"] != float64(2) {
		t.Fatalf("latest revision %+v", machine)
	}
	// No SELECT/UPDATE grants to this role should allow direct overwrites.
	if _, e = app.ExecContext(ctx, `UPDATE factory_fact_events_v65b SET value_text='999' WHERE tenant_id=$1`, tenant); e == nil {
		t.Fatal("append-only fact table permitted UPDATE")
	}
	// Tenant pin governs read access, not the payload.
	t.Setenv("AGIPAREL_OPERATOR_TENANT", other)
	code, res = call(http.MethodGet, "/api/v1/factory-onboarding", nil)
	if code != 200 {
		t.Fatalf("other tenant HTTP %d", code)
	}
	if mf, ok := res["facts"].(map[string]any); !ok || len(mf) != 0 {
		t.Fatalf("cross-tenant facts leaked: %+v", mf)
	}
	if strings.Contains(strings.ToLower(res["next_question"].(string)), "password") {
		t.Fatal("irrelevant question")
	}
}
