package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

const validPO = "po_number,style_code,color_code,size_code,ordered_qty,upc\nPO-501,ST-501,BLUE,M,15,012345\nPO-501,ST-501,BLUE,L,25,012346\n"
const validBOM = "style_code,material_sku,category,qty_per_garment,uom\nST-501,TWILL-501,FABRIC,1.850,M\nST-501,ZIP-501,TRIM,1.000,PCS\n"

func TestStructuredPOBOMIntakeValidation(t *testing.T) {
	s, e := parseIntake([]byte(validPO), []byte(validBOM), "po.csv", "bom.csv")
	if e != nil {
		t.Fatal(e)
	}
	if s.Report.Readiness != "HOLD" || s.Report.Status != "PROPOSED" || s.Report.OrderedQty != 40 {
		t.Fatalf("invalid readiness: %+v", s.Report)
	}
	if len(s.Report.SKUs) != 2 || len(s.Report.Materials) != 2 || s.Report.Materials[0].Required != "74.000" || s.Report.Materials[1].Required != "40.000" {
		t.Fatalf("invalid computed demand: %+v", s.Report)
	}
	if s.Report.SourceSHA256 == "" {
		t.Fatal("no content digest")
	}
	variants := []struct{ name, po, bom string }{
		{"mismatched style", validPO, strings.Replace(validBOM, "ST-501,TWILL", "ST-999,TWILL", 1)},
		{"duplicate SKU", validPO + "PO-501,ST-501,BLUE,M,3,012345\n", validBOM},
		{"PO mix", strings.Replace(validPO, "PO-501,ST-501,BLUE,L", "PO-502,ST-501,BLUE,L", 1), validBOM},
		{"bad quantity", strings.Replace(validPO, "BLUE,M,15", "BLUE,M,0", 1), validBOM},
		{"negative material", validPO, strings.Replace(validBOM, "1.850", "-1.850", 1)},
		{"fraction precision", validPO, strings.Replace(validBOM, "1.850", "1.8505", 1)},
		{"duplicate BOM", validPO, validBOM + "ST-501,ZIP-501,TRIM,1.000,PCS\n"},
		{"missing fabric", validPO, strings.Replace(validBOM, "FABRIC", "TRIM", 1)},
		{"empty BOM", validPO, "style_code,material_sku,category,qty_per_garment,uom\n"},
		{"formula safe text", strings.Replace(validPO, "BLUE,M", "=IMPORT,M", 1), validBOM},
		{"header tamper", strings.Replace(validPO, "ordered_qty", "fake_qty", 1), validBOM},
	}
	for _, v := range variants {
		t.Run(v.name, func(t *testing.T) {
			if _, e := parseIntake([]byte(v.po), []byte(v.bom), "po.csv", "bom.csv"); e == nil {
				t.Fatal("invalid input accepted")
			}
		})
	}
	// A reserialized source with changed data must not inherit the original evidence ID.
	p2 := strings.Replace(validPO, "BLUE,M,15", "BLUE,M,16", 1)
	next, e := parseIntake([]byte(p2), []byte(validBOM), "po.csv", "bom.csv")
	if e != nil {
		t.Fatal(e)
	}
	if next.Report.SourceSHA256 == s.Report.SourceSHA256 {
		t.Fatal("evidence hash did not change")
	}
}

func TestIntakeFailClosedWithoutDBOrTenant(t *testing.T) {
	t.Setenv("ENV", "development")
	t.Setenv("AGIPAREL_OPERATOR_TENANT", "TENANT-UNIT")
	server := &EdgeServer{}
	w := httptest.NewRecorder()
	server.handleOrderIntake(w, httptest.NewRequest("GET", "/api/v1/order-intake", nil))
	if w.Code != 503 {
		t.Fatalf("expected 503, got %d", w.Code)
	}
	t.Setenv("AGIPAREL_OPERATOR_TENANT", "")
	if _, e := intakeTenant(); e == nil {
		t.Fatal("expected missing tenant rejection")
	}
	t.Setenv("AGIPAREL_OPERATOR_TENANT", "TENANT-UNIT")
	t.Setenv("ENV", "production")
	if _, e := intakeTenant(); e == nil {
		t.Fatal("production identity bypass")
	}
}

func makeIntakeHTTP(method, action, po, bom string) *http.Request {
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	p, _ := mw.CreateFormFile("po_file", "buyer.csv")
	_, _ = p.Write([]byte(po))
	b, _ := mw.CreateFormFile("bom_file", "style.csv")
	_, _ = b.Write([]byte(bom))
	_ = mw.Close()
	req := httptest.NewRequest(method, "/api/v1/order-intake?action="+action, &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	return req
}

func TestOrderIntakePreviewAndPostgres(t *testing.T) {
	adminURL, appURL := os.Getenv("AGIPAREL_TEST_ADMIN_DATABASE_URL"), os.Getenv("AGIPAREL_TEST_APP_DATABASE_URL")
	if adminURL == "" || appURL == "" {
		t.Skip("requires admin and app PostgreSQL test URLs")
	}
	if os.Getenv("AGIPAREL_DESTRUCTIVE_DB_TESTS") != "I_UNDERSTAND_TEST_FIXTURES" {
		t.Fatal("opt in to disposable PostgreSQL fixture tests")
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
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	for _, db := range []*sql.DB{admin, app} {
		if e = db.PingContext(ctx); e != nil {
			t.Fatal(e)
		}
	}
	var role string
	if e = app.QueryRowContext(ctx, "SELECT current_user").Scan(&role); e != nil || role != "agiparel_app" {
		t.Fatalf("restricted app DB role required: %s %v", role, e)
	}
	var seed [8]byte
	if _, e = rand.Read(seed[:]); e != nil {
		t.Fatal(e)
	}
	suffix := hex.EncodeToString(seed[:])
	tenant, other := "BI-"+suffix, "BX-"+suffix
	if _, e = admin.ExecContext(ctx, `INSERT INTO tenants(id,name) VALUES($1,'Batch05 test'),($2,'Batch05 isolated')`, tenant, other); e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() {
		for _, q := range []string{"DELETE FROM order_intake_bom_v5 WHERE tenant_id=$1", "DELETE FROM order_intake_skus_v5 WHERE tenant_id=$1", "DELETE FROM order_intakes_v5 WHERE tenant_id=$1", "DELETE FROM tenants WHERE id=$1"} {
			_, _ = admin.ExecContext(context.Background(), q, tenant)
		}
		_, _ = admin.ExecContext(context.Background(), "DELETE FROM tenants WHERE id=$1", other)
	})
	t.Setenv("ENV", "development")
	t.Setenv("AGIPAREL_OPERATOR_TENANT", tenant)
	server := &EdgeServer{db: app}
	invoke := func(req *http.Request) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		server.handleOrderIntake(w, req)
		return w
	}
	preview := invoke(makeIntakeHTTP("POST", "preview", validPO, validBOM))
	if preview.Code != 200 {
		t.Fatalf("preview %d: %s", preview.Code, preview.Body.String())
	}
	var c int
	if e = admin.QueryRowContext(ctx, `SELECT COUNT(*) FROM order_intakes_v5 WHERE tenant_id=$1`, tenant).Scan(&c); e != nil || c != 0 {
		t.Fatalf("preview wrote to DB: %d %v", c, e)
	}
	staged := invoke(makeIntakeHTTP("POST", "stage", validPO, validBOM))
	if staged.Code != 201 {
		t.Fatalf("stage %d: %s", staged.Code, staged.Body.String())
	}
	var got intakeReport
	if e = json.Unmarshal(staged.Body.Bytes(), &got); e != nil {
		t.Fatal(e)
	}
	if got.ID == "" || got.Readiness != "HOLD" || got.OrderedQty != 40 {
		t.Fatalf("bad stage report: %+v", got)
	}
	replay := invoke(makeIntakeHTTP("POST", "stage", validPO, validBOM))
	if replay.Code != 200 {
		t.Fatalf("replay %d: %s", replay.Code, replay.Body.String())
	}
	var duplicate intakeReport
	if e = json.Unmarshal(replay.Body.Bytes(), &duplicate); e != nil || duplicate.ID != got.ID {
		t.Fatalf("duplicate receipt %v %+v", e, duplicate)
	}
	if e = admin.QueryRowContext(ctx, `SELECT COUNT(*) FROM order_intakes_v5 WHERE tenant_id=$1`, tenant).Scan(&c); e != nil || c != 1 {
		t.Fatalf("duplicate inserted: %d %v", c, e)
	}
	var original string
	if e = admin.QueryRowContext(ctx, `SELECT convert_from(po_original,'UTF8') FROM order_intakes_v5 WHERE id=$1`, got.ID).Scan(&original); e != nil || original != validPO {
		t.Fatalf("original evidence mismatch %v", e)
	}
	details := invoke(httptest.NewRequest("GET", "/api/v1/order-intake?id="+got.ID, nil))
	if details.Code != 200 {
		t.Fatalf("detail: %d %s", details.Code, details.Body.String())
	}
	var obj intakeReport
	if e = json.Unmarshal(details.Body.Bytes(), &obj); e != nil || len(obj.SKUs) != 2 || obj.Materials[0].Required != "74.000" {
		t.Fatalf("DB material demand: %+v / %v", obj, e)
	}
	// Crucial: selecting a different tenant never returns the submitted PO.
	t.Setenv("AGIPAREL_OPERATOR_TENANT", other)
	hidden := invoke(httptest.NewRequest("GET", "/api/v1/order-intake?id="+got.ID, nil))
	if hidden.Code != 404 {
		t.Fatalf("cross-tenant leak: %d", hidden.Code)
	}
	hiddenList := invoke(httptest.NewRequest("GET", "/api/v1/order-intake", nil))
	if hiddenList.Code != 200 || strings.Contains(hiddenList.Body.String(), got.PONumber) {
		t.Fatalf("cross-tenant listing leak: %d %s", hiddenList.Code, hiddenList.Body.String())
	}
	// Runtime role must not edit proposed buyer evidence.
	tx, e := app.BeginTx(ctx, nil)
	if e != nil {
		t.Fatal(e)
	}
	_, e = tx.ExecContext(ctx, "SELECT set_config('app.current_tenant_id',$1,true)", tenant)
	if e != nil {
		t.Fatal(e)
	}
	_, e = tx.ExecContext(ctx, "UPDATE order_intakes_v5 SET po_number='TAMPER' WHERE id=$1", got.ID)
	_ = tx.Rollback()
	if e == nil {
		t.Fatal("runtime role can overwrite evidence")
	}
}
