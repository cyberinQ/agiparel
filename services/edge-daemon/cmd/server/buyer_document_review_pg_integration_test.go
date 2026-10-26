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

// Opt-in: only uses a random scratch tenant; never changes real factory records.
// Migration 0014 must be applied. Verifies atomic handoff, read-side visibility,
// idempotency, source checks, tenant isolation, and restricted DB privileges.
func TestBuyerReviewPostgresAtomicHandoff(t *testing.T) {
	adminURL, appURL := os.Getenv("AGIPAREL_TEST_ADMIN_DATABASE_URL"), os.Getenv("AGIPAREL_TEST_APP_DATABASE_URL")
	if adminURL == "" || appURL == "" {
		t.Skip("requires both PostgreSQL fixture connection URLs")
	}
	if os.Getenv("AGIPAREL_DESTRUCTIVE_DB_TESTS") != "I_UNDERSTAND_TEST_FIXTURES" {
		t.Fatal("scratch-tenant cleanup requires explicit fixture opt-in")
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
	ctx, cancel := context.WithTimeout(context.Background(), 35*time.Second)
	defer cancel()
	for _, db := range []*sql.DB{admin, app} {
		if err = db.PingContext(ctx); err != nil {
			t.Fatal(err)
		}
	}
	var role string
	if err = app.QueryRowContext(ctx, `SELECT current_user`).Scan(&role); err != nil || role != "agiparel_app" {
		t.Fatalf("restricted app role required, got %q %v", role, err)
	}
	var rnd [16]byte
	if _, err = rand.Read(rnd[:]); err != nil {
		t.Fatal(err)
	}
	uid := hex.EncodeToString(rnd[:])
	tenant := "BC1-" + uid[:14]
	other := "BX1-" + uid[:14]
	poID := "DOC-" + uid
	otherID := "DOC-" + strings.Repeat("f", 16) + uid[:16]
	t.Cleanup(func() {
		for _, q := range []string{
			`DELETE FROM buyer_order_handoffs_v65c1 WHERE tenant_id=$1`,
			`DELETE FROM order_intake_bom_v5 WHERE tenant_id=$1`,
			`DELETE FROM order_intake_skus_v5 WHERE tenant_id=$1`,
			`DELETE FROM order_intakes_v5 WHERE tenant_id=$1`,
			`DELETE FROM document_segments_v65 WHERE tenant_id=$1`,
			`DELETE FROM document_jobs_v65 WHERE tenant_id=$1`,
			`DELETE FROM document_sources_v65 WHERE tenant_id=$1`,
			`DELETE FROM tenants WHERE id=$1`,
		} {
			_, _ = admin.ExecContext(context.Background(), q, tenant)
		}
		_, _ = admin.ExecContext(context.Background(), `DELETE FROM tenants WHERE id=$1`, other)
	})
	if _, err = admin.ExecContext(ctx, `INSERT INTO tenants(id,name) VALUES ($1,'Buyer handoff test'),($2,'Buyer isolated tenant')`, tenant, other); err != nil {
		t.Fatal(err)
	}
	for i, d := range []struct {
		ID, Name, SHA string
		Rows          []string
	}{
		{poID, "buyer.csv", strings.Repeat("a", 64), []string{`["po_number","style_code","color_code","size_code","ordered_qty","upc"]`, `["REP3","GM09Y094","NAVY","M","40","0999"]`}},
		{otherID, "bom.csv", strings.Repeat("b", 64), []string{`["style_code","material_sku","category","qty_per_garment","uom"]`, `["GM09Y094","TWILL","FABRIC","1.850","M"]`}},
	} {
		if _, err = admin.ExecContext(ctx, `INSERT INTO document_sources_v65(tenant_id,id,filename,media_type,format,byte_length,sha256,original) VALUES($1,$2,$3,'text/csv','CSV',1,$4,$5)`, tenant, d.ID, d.Name, d.SHA, []byte("x")); err != nil {
			t.Fatal(err)
		}
		if _, err = admin.ExecContext(ctx, `INSERT INTO document_jobs_v65(tenant_id,document_id,status,segment_count) VALUES($1,$2,'EXTRACTED',2)`, tenant, d.ID); err != nil {
			t.Fatal(err)
		}
		for j, row := range d.Rows {
			if _, err = admin.ExecContext(ctx, `INSERT INTO document_segments_v65(tenant_id,document_id,ordinal,kind,locator,content) VALUES($1,$2,$3,'ROW',$4::jsonb,$5)`, tenant, d.ID, j+1, `{"row":2}`, row); err != nil {
				t.Fatal(err)
			}
		}
		_ = i
	}
	t.Setenv("ENV", "development")
	t.Setenv("AGIPAREL_OPERATOR_TENANT", tenant)
	server := &EdgeServer{db: app}
	invoke := func(method, path string, payload any) (int, map[string]any) {
		var body []byte
		if payload != nil {
			body, _ = json.Marshal(payload)
		}
		req := httptest.NewRequest(method, path, bytes.NewReader(body))
		if payload != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		w := httptest.NewRecorder()
		server.handleBuyerReview(w, req)
		res := map[string]any{}
		_ = json.Unmarshal(w.Body.Bytes(), &res)
		return w.Code, res
	}
	code, data := invoke("GET", "/api/v1/buyer-review?po_document_id="+poID+"&bom_document_id="+otherID, nil)
	if code != 200 || len(data["po_options"].([]any)) != 1 || len(data["materials"].([]any)) != 1 {
		t.Fatalf("preview status=%d data=%v", code, data)
	}
	req := buyerReviewRequest{PODocumentID: poID, BOMDocumentID: otherID, PONumber: "REP3", StyleCode: "GM09Y094", SKUs: []buyerSKU{{Color: "NAVY", Size: "M", UPC: "0999", Quantity: 40, SourceOrdinal: 2}}, Materials: []buyerMaterial{{StyleCode: "GM09Y094", SKU: "TWILL", Category: "FABRIC", PerGarment: "1.850", UOM: "M", SourceOrdinal: 2}}, ReviewNote: "Verified rows against original buyer files", Acknowledged: true}
	bad := req
	bad.SKUs = append([]buyerSKU{}, req.SKUs...)
	bad.SKUs[0].SourceOrdinal = 999
	code, _ = invoke("POST", "/api/v1/buyer-review", bad)
	if code != 400 {
		t.Fatalf("bad citation accepted %d", code)
	}
	var count int
	if err = admin.QueryRowContext(ctx, `SELECT count(*) FROM order_intakes_v5 WHERE tenant_id=$1`, tenant).Scan(&count); err != nil || count != 0 {
		t.Fatalf("bad review wrote an order %d %v", count, err)
	}
	code, data = invoke("POST", "/api/v1/buyer-review", req)
	if code != 201 {
		t.Fatalf("stage %d: %v", code, data)
	}
	result := data["intake"].(map[string]any)
	id := result["id"].(string)
	if result["readiness"] != "HOLD" || result["status"] != "PROPOSED" {
		t.Fatal(result)
	}
	code, replay := invoke("POST", "/api/v1/buyer-review", req)
	if code != 200 || replay["intake"].(map[string]any)["id"] != id {
		t.Fatalf("replay %d %v", code, replay)
	}
	var links int
	if err = admin.QueryRowContext(ctx, `SELECT count(*) FROM buyer_order_handoffs_v65c1 WHERE tenant_id=$1 AND intake_id=$2`, tenant, id).Scan(&links); err != nil || links != 1 {
		t.Fatalf("source links=%d %v", links, err)
	}
	w := httptest.NewRecorder()
	server.handleOrderIntake(w, httptest.NewRequest(http.MethodGet, "/api/v1/order-intake?id="+id, nil))
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"REVIEWED_DOCUMENT_MAPPING"`) {
		t.Fatalf("read-through=%d %s", w.Code, w.Body.String())
	}
	t.Setenv("AGIPAREL_OPERATOR_TENANT", other)
	code, _ = invoke("GET", "/api/v1/buyer-review?po_document_id="+poID+"&bom_document_id="+otherID, nil)
	if code != 404 {
		t.Fatalf("cross-tenant source leaked: %d", code)
	}
	if _, err = app.ExecContext(ctx, `UPDATE buyer_order_handoffs_v65c1 SET review_note='bad' WHERE tenant_id=$1`, tenant); err == nil {
		t.Fatal("app can mutate immutable source linkage")
	}
}
