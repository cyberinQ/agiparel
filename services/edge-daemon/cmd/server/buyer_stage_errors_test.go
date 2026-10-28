// Author: Sunil Jain, 2026-10-27 — Verify safe order-save errors and the reported buyer sample.
package main

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

func TestBuyerSaveErrorCodesAreActionableWithoutSQLLeaks(t *testing.T) {
	samples := []struct {
		code   string
		status int
		phrase string
	}{
		{"42P01", http.StatusServiceUnavailable, "migrations 0007 and 0014"},
		{"42501", http.StatusServiceUnavailable, "role permissions"},
		{"23505", http.StatusConflict, "conflicts"},
		{"23503", http.StatusServiceUnavailable, "required database link"},
		{"23514", http.StatusConflict, "stored-data rules"},
		{"57014", http.StatusServiceUnavailable, "timed out"},
		{"XX000", http.StatusServiceUnavailable, "server log"},
	}
	for _, sample := range samples {
		t.Run(sample.code, func(t *testing.T) {
			err := &pgconn.PgError{Code: sample.code, Message: "secret buyer data must not leak", Detail: "secret value"}
			code, message, cause := buyerSaveError(err)
			if code != sample.status || cause != sample.code || !strings.Contains(message, sample.phrase) {
				t.Fatalf("bad classification: code=%d message=%q cause=%q", code, message, cause)
			}
			if strings.Contains(message, "secret") || strings.Contains(message, "SQL") {
				t.Fatalf("exposed database details: %s", message)
			}
		})
	}
	code, message, _ := buyerSaveError(context.DeadlineExceeded)
	if code != http.StatusServiceUnavailable || !strings.Contains(message, "timed out") {
		t.Fatal(code, message)
	}
	code, message, _ = buyerSaveError(errors.New("password=never-log-this"))
	if code != http.StatusServiceUnavailable || strings.Contains(message, "never-log-this") {
		t.Fatal(code, message)
	}
}

func TestBuyerReportedREP3DraftIsValidBeforeDatabaseWrite(t *testing.T) {
	poDoc := testBuyerDoc("DOC-"+strings.Repeat("1", 32), strings.Repeat("a", 64), "Flat Front Pant PO.pdf", []buyerEvidence{{Ordinal: 1, Kind: "PAGE", Content: "buyer page"}})
	bomDoc := testBuyerDoc("DOC-"+strings.Repeat("2", 32), strings.Repeat("b", 64), "Flat Front Pant Tech Pack.pdf", []buyerEvidence{{Ordinal: 2, Kind: "PAGE", Content: "tech pack page"}})
	poDoc.Segments[0].Locator = json.RawMessage(`{"page":1}`)
	bomDoc.Segments[0].Locator = json.RawMessage(`{"page":2}`)
	sizes := []string{"30", "32", "34", "36", "38", "40", "42"}
	quantities := []int64{7200, 7200, 9600, 9600, 9600, 7200, 7200}
	req := buyerReviewRequest{PONumber: "REP3", StyleCode: "GM09Y094", ReviewNote: "trial, i have no idea", Acknowledged: true}
	for i, size := range sizes {
		req.SKUs = append(req.SKUs, buyerSKU{Color: "SOFT_SILVER", Size: size, Quantity: quantities[i], SourceOrdinal: 1})
	}
	req.Materials = []buyerMaterial{{StyleCode: "GM09Y094", SKU: "009", Category: "FABRIC", PerGarment: "1.09", UOM: "m/pc", SourceOrdinal: 2}}
	sub, err := prepareBuyerSubmission(req, poDoc, bomDoc)
	if err != nil {
		t.Fatalf("screen data should pass validation, not silently fail: %v", err)
	}
	if sub.Report.OrderedQty != 57600 || sub.Report.Materials[0].Required != "62784.000" || sub.Report.Status != "PROPOSED" || sub.Report.Readiness != "HOLD" {
		t.Fatalf("unexpected staging output: %+v", sub.Report)
	}
}

// Author: Sunil Jain, 2026-10-27 — Test tenant bootstrap and precise
// foreign-key classification without PostgreSQL or production credentials.
type tenantBootstrapRecorder struct {
	called int
	query  string
	args   []any
	err    error
}

func (r *tenantBootstrapRecorder) ExecContext(_ context.Context, query string, args ...any) (sql.Result, error) {
	r.called++
	r.query = query
	r.args = args
	return driver.RowsAffected(1), r.err
}

func TestEnsureDevelopmentOperatorTenantIsIdempotentAndIsolated(t *testing.T) {
	t.Setenv("ENV", "production")
	t.Setenv("AGIPAREL_OPERATOR_TENANT", "dev")
	fake := &tenantBootstrapRecorder{}
	if err := ensureDevelopmentOperatorTenant(context.Background(), fake); err != nil || fake.called != 0 {
		t.Fatalf("production must never auto-create tenants: calls=%d err=%v", fake.called, err)
	}
	t.Setenv("ENV", "development")
	if err := ensureDevelopmentOperatorTenant(context.Background(), fake); err != nil {
		t.Fatal(err)
	}
	if fake.called != 1 || !strings.Contains(fake.query, "ON CONFLICT (id) DO NOTHING") || len(fake.args) != 2 || fake.args[0] != "dev" {
		t.Fatalf("unsafe tenant provision: calls=%d args=%v query=%q", fake.called, fake.args, fake.query)
	}
	t.Setenv("AGIPAREL_OPERATOR_TENANT", "")
	if err := ensureDevelopmentOperatorTenant(context.Background(), fake); err == nil || fake.called != 1 {
		t.Fatalf("invalid tenant was not rejected: calls=%d err=%v", fake.called, err)
	}
}

func TestBuyerForeignKeyFailureIdentifiesMissingTenant(t *testing.T) {
	for _, test := range []struct {
		constraint, phrase string
		status             int
	}{
		{"order_intakes_v5_tenant_id_fkey", "Factory account", http.StatusServiceUnavailable},
		{"buyer_order_handoffs_v65c1_tenant_id_po_document_id_fkey", "source document", http.StatusConflict},
		{"other_fk", "required database link", http.StatusServiceUnavailable},
	} {
		status, message, _ := buyerSaveError(&pgconn.PgError{Code: "23503", ConstraintName: test.constraint})
		if status != test.status || !strings.Contains(message, test.phrase) {
			t.Errorf("constraint %s: %d %s", test.constraint, status, message)
		}
	}
}
