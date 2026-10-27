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

// Opt-in only: isolated random scratch tenants; deletes only its own passport
// and binding rows. Requires migration 0015. No authoritative stock is edited.
func TestStoresPassportPostgresIsolationAndRFIDCollision(t *testing.T) {
	adminURL, appURL := os.Getenv("AGIPAREL_TEST_ADMIN_DATABASE_URL"), os.Getenv("AGIPAREL_TEST_APP_DATABASE_URL")
	if adminURL == "" || appURL == "" {
		t.Skip("requires admin and restricted-app fixture connections")
	}
	if os.Getenv("AGIPAREL_DESTRUCTIVE_DB_TESTS") != "I_UNDERSTAND_TEST_FIXTURES" {
		t.Fatal("explicit test-fixture opt-in required")
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
	var user string
	if e = app.QueryRowContext(ctx, `SELECT current_user`).Scan(&user); e != nil || user != "agiparel_app" {
		t.Fatalf("restricted role required got %q, %v", user, e)
	}
	var r [12]byte
	if _, e = rand.Read(r[:]); e != nil {
		t.Fatal(e)
	}
	id := hex.EncodeToString(r[:])
	tenant := "S7-" + id
	other := "S7X-" + id
	epc := "300833B2DDD9" + id[:12] // 24 hex
	for _, ten := range []string{tenant, other} {
		_, e = admin.ExecContext(ctx, `INSERT INTO tenants(id,name) VALUES($1,$2)`, ten, "Stores passport fixture")
		if e != nil {
			t.Fatal(e)
		}
	}
	t.Cleanup(func() {
		for _, ten := range []string{tenant, other} {
			_, _ = admin.ExecContext(context.Background(), `DELETE FROM store_roll_passport_events_v7 WHERE tenant_id=$1`, ten)
			_, _ = admin.ExecContext(context.Background(), `DELETE FROM store_roll_rfid_bindings_v7 WHERE tenant_id=$1`, ten)
			_, _ = admin.ExecContext(context.Background(), `DELETE FROM tenants WHERE id=$1`, ten)
		}
	})
	t.Setenv("ENV", "development")
	t.Setenv("AGIPAREL_OPERATOR_TENANT", tenant)
	s := &EdgeServer{db: app}
	call := func(method string, v any) (int, map[string]any) {
		t.Helper()
		raw, _ := json.Marshal(v)
		req := httptest.NewRequest(method, "/api/v1/stores/passports", bytes.NewReader(raw))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		s.handleStoresPassports(w, req)
		got := map[string]any{}
		_ = json.Unmarshal(w.Body.Bytes(), &got)
		if w.Code >= 500 {
			t.Fatalf("unexpected server failure %d: %s", w.Code, w.Body.String())
		}
		return w.Code, got
	}
	v := samplePassport()
	v.EPC = epc
	code, res := call(http.MethodPost, v)
	if code != 201 {
		t.Fatalf("create %d %+v", code, res)
	}
	code, _ = call(http.MethodPost, v)
	if code != 409 {
		t.Fatalf("stale revision not rejected: %d", code)
	}
	code, res = call(http.MethodGet, nil)
	if code != 200 || len(res["items"].([]any)) != 1 {
		t.Fatalf("tenant list %d %+v", code, res)
	}
	v.ExpectedRevision = 1
	v.MeasuredLengthM = "97.125"
	code, res = call(http.MethodPost, v)
	if code != 201 {
		t.Fatalf("revision %d %+v", code, res)
	}
	v2 := v
	v2.RollID = "ROLL-OTHER"
	v2.ExpectedRevision = 0
	code, _ = call(http.MethodPost, v2)
	if code != 409 {
		t.Fatalf("EPC reused for second roll: %d", code)
	}
	if _, e = app.ExecContext(ctx, `UPDATE store_roll_passport_events_v7 SET measured_length_mm=1 WHERE tenant_id=$1`, tenant); e == nil {
		t.Fatal("restricted writer modified immutable roll events")
	}
	t.Setenv("AGIPAREL_OPERATOR_TENANT", other)
	code, res = call(http.MethodGet, nil)
	if code != 200 || len(res["items"].([]any)) != 0 {
		t.Fatalf("cross-tenant data leaked: %d %+v", code, res)
	}
}
