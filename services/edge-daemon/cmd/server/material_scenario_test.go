package main

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

const scenarioPO = "po_number,style_code,color_code,size_code,ordered_qty,upc\nPO-DEMO-01,STYLE-100,NAVY,M,250,\nPO-DEMO-01,STYLE-100,NAVY,L,350,\n"
const scenarioBOM = "style_code,material_sku,category,qty_per_garment,uom\nSTYLE-100,COTTON-TWILL,FABRIC,1.850,M\nSTYLE-100,ZIP-18CM,TRIM,1.000,PCS\n"
const scenarioStockHeader = "lot_id,material_sku,uom,available_qty,fabric_spec,required_spec,gsm,required_gsm,gsm_tolerance_pct,shade_delta_e,shade_max_delta_e,usable_width_mm,marker_width_mm,inspection_status,age_days,max_age_days,buyer_escrow,held_for_po,source_reference\n"
const scenarioEligible = "LOT-101,COTTON-TWILL,M,500.000,TWILL220,TWILL220,220.000,220.000,3.000,0.600,1.000,1650.000,1600.000,ACCEPTED_PASS,60,365,ALLOWED,,GRN-101\n"
const scenarioWrongHold = "LOT-102,COTTON-TWILL,M,250.000,TWILL220,TWILL220,220.000,220.000,3.000,0.600,1.000,1650.000,1600.000,ACCEPTED_PASS,60,365,ALLOWED,OTHER-PO,GRN-102\n"
const scenarioPrices = "material_sku,category,uom,unit_price,currency,procurement_owner,source_reference\nCOTTON-TWILL,FABRIC,M,4.2500,USD,FACTORY,QUOTE-11\nZIP-18CM,TRIM,PCS,0.3300,USD,FACTORY,QUOTE-12\n"

func makeScenarioIntake(t *testing.T) intakeReport {
	t.Helper()
	sub, e := parseIntake([]byte(scenarioPO), []byte(scenarioBOM), "buyer.csv", "bom.csv")
	if e != nil {
		t.Fatal(e)
	}
	sub.Report.ID = "OI-1234567890abcdef1234567890abcdef"
	return sub.Report
}
func TestMaterialScenarioSixChecksAndNetIndent(t *testing.T) {
	input := makeScenarioIntake(t)
	src, e := buildScenario(input, []byte(scenarioStockHeader+scenarioEligible+scenarioWrongHold), []byte(scenarioPrices), "surplus.csv", "prices.csv")
	if e != nil {
		t.Fatal(e)
	}
	r := src.Report
	if r.ReleaseGate != "HOLD" || r.Status != "PROPOSED" || r.EvidenceClass != "OPERATOR_SUBMITTED_CSV_UNVERIFIED" {
		t.Fatalf("unexpected authority: %+v", r)
	}
	if len(r.Materials) != 2 || r.Materials[0].Gross != "1110.000" || r.Materials[0].ConditionalSurplus != "500.000" || r.Materials[0].NetIndent != "610.000" || r.Materials[1].NetIndent != "600.000" {
		t.Fatalf("incorrect quantities: %+v", r.Materials)
	}
	if r.GrossMaterialCost != "4915.5000" || r.ConditionalReuseValue != "2125.0000" || r.NetPurchaseExposure != "2790.5000" {
		t.Fatalf("incorrect economic totals: %+v", r)
	}
	if len(r.Candidates) != 2 || r.Candidates[0].Verdict != "CONDITIONALLY_MATCHED" || r.Candidates[1].Checks[5].Pass || r.Candidates[1].SuggestedQty != "0.000" {
		t.Fatalf("other-PO borrow was accepted: %+v", r.Candidates)
	}
	if len(r.Candidates[0].Checks) != 6 {
		t.Fatal("missing six-dimensional reasoning")
	}
	if r.ScenarioSHA256 == "" {
		t.Fatal("missing scenario digest")
	}
}
func TestMaterialScenarioRejectsBadOrUncertainInputs(t *testing.T) {
	intake := makeScenarioIntake(t)
	failures := []struct{ name, stock, prices string }{
		{"wrong specification", scenarioStockHeader + strings.Replace(scenarioEligible, "TWILL220,TWILL220", "TWILL999,TWILL220", 1), scenarioPrices},
		{"shade outside limit", scenarioStockHeader + strings.Replace(scenarioEligible, ",0.600,1.000,", ",1.200,1.000,", 1), scenarioPrices},
		{"width too small", scenarioStockHeader + strings.Replace(scenarioEligible, ",1650.000,1600.000,", ",1500.000,1600.000,", 1), scenarioPrices},
		{"inspection not accepted", scenarioStockHeader + strings.Replace(scenarioEligible, "ACCEPTED_PASS", "PENDING", 1), scenarioPrices},
		{"aged beyond policy", scenarioStockHeader + strings.Replace(scenarioEligible, ",60,365,", ",390,365,", 1), scenarioPrices},
		{"escrow forbidden", scenarioStockHeader + strings.Replace(scenarioEligible, ",ALLOWED,,", ",DENIED,,", 1), scenarioPrices},
	}
	for _, v := range failures {
		t.Run(v.name, func(t *testing.T) {
			src, e := buildScenario(intake, []byte(v.stock), []byte(v.prices), "stock.csv", "prices.csv")
			if e != nil {
				t.Fatal(e)
			}
			if src.Report.Materials[0].ConditionalSurplus != "0.000" || src.Report.Materials[0].NetIndent != "1110.000" || src.Report.Candidates[0].Verdict != "INELIGIBLE" {
				t.Fatalf("unsafe surplus applied: %+v", src.Report)
			}
		})
	}
	bad := []struct{ name, stock, prices string }{
		{"duplicate lot", scenarioStockHeader + scenarioEligible + scenarioEligible, scenarioPrices},
		{"negative stock", scenarioStockHeader + strings.Replace(scenarioEligible, "500.000", "-1.000", 1), scenarioPrices},
		{"formula injection", scenarioStockHeader + strings.Replace(scenarioEligible, "LOT-101", "=EVAL", 1), scenarioPrices},
		{"wrong BOM fabric", scenarioStockHeader + strings.Replace(scenarioEligible, "COTTON-TWILL", "NOT-IN-BOM", 1), scenarioPrices},
		{"zero GSM", scenarioStockHeader + strings.Replace(scenarioEligible, ",220.000,220.000,", ",0.000,220.000,", 1), scenarioPrices},
		{"invalid price precision", scenarioStockHeader + scenarioEligible, strings.Replace(scenarioPrices, "4.2500", "4.25001", 1)},
		{"missing trim price", scenarioStockHeader + scenarioEligible, strings.Replace(scenarioPrices, "ZIP-18CM,TRIM,PCS,0.3300,USD,FACTORY,QUOTE-12\n", "", 1)},
		{"mixed currency", scenarioStockHeader + scenarioEligible, strings.Replace(scenarioPrices, "0.3300,USD", "0.3300,EUR", 1)},
		{"duplicate price", scenarioStockHeader + scenarioEligible, scenarioPrices + "COTTON-TWILL,FABRIC,M,4.2500,USD,FACTORY,QUOTE-11\n"},
	}
	for _, v := range bad {
		t.Run(v.name, func(t *testing.T) {
			if _, e := buildScenario(intake, []byte(v.stock), []byte(v.prices), "stock.csv", "prices.csv"); e == nil {
				t.Fatal("invalid scenario accepted")
			}
		})
	}
}
func TestScenarioWithoutSurplus(t *testing.T) {
	intake := makeScenarioIntake(t)
	src, err := buildScenario(intake, []byte(scenarioStockHeader), []byte(scenarioPrices), "empty.csv", "prices.csv")
	if err != nil {
		t.Fatal(err)
	}
	if len(src.Report.Candidates) != 0 || src.Report.Materials[0].NetIndent != "1110.000" || src.Report.NetPurchaseExposure != "4915.5000" {
		t.Fatalf("header-only zero-surplus scenario wrong: %+v", src.Report)
	}
}
func TestBuyerOwnedExcludesFactoryProcurement(t *testing.T) {
	intake := makeScenarioIntake(t)
	buyerPrices := strings.Replace(scenarioPrices, "4.2500,USD,FACTORY", "4.2500,USD,BUYER_OWNED", 1)
	src, err := buildScenario(intake, []byte(scenarioStockHeader), []byte(buyerPrices), "empty.csv", "prices.csv")
	if err != nil {
		t.Fatal(err)
	}
	m := src.Report.Materials[0]
	if m.ProcurementOwner != "BUYER_OWNED" || m.NetIndent != "0.000" || m.BuyerSupplyGap != "1110.000" || m.NetCost != "0.0000" || src.Report.NetPurchaseExposure != "198.0000" {
		t.Fatalf("buyer-owned fabric must never be factory procurement: %+v", src.Report)
	}
}
func TestScenarioLimitsAndRounding(t *testing.T) {
	for _, in := range []string{"-2", "+1", "1e6", "NaN", "10.1234", " 1.000", "0001.000", "1,000"} {
		if _, e := scenarioScaled(in, 3, 100000, false); e == nil {
			t.Fatalf("accepted %q", in)
		}
	}
	if scenarioMoney(1, 5000) != "0.0005" || scenarioMoney(1, 5500) != "0.0006" {
		t.Fatalf("currency half-up rounding broken")
	}
	intake := makeScenarioIntake(t)
	a, _ := buildScenario(intake, []byte(scenarioStockHeader+scenarioEligible), []byte(scenarioPrices), "s.csv", "p.csv")
	b, _ := buildScenario(intake, []byte(scenarioStockHeader+strings.Replace(scenarioEligible, "500.000", "501.000", 1)), []byte(scenarioPrices), "s.csv", "p.csv")
	if a.Report.ScenarioSHA256 == b.Report.ScenarioSHA256 {
		t.Fatal("source mutation reused identity")
	}
}

// Run only against disposable Agiparel PostgreSQL with explicit opt-in.
func TestMaterialScenarioPostgresPersistenceAndTenantRLS(t *testing.T) {
	adminURL, appURL := os.Getenv("AGIPAREL_TEST_ADMIN_DATABASE_URL"), os.Getenv("AGIPAREL_TEST_APP_DATABASE_URL")
	if adminURL == "" || appURL == "" {
		t.Skip("requires disposable PostgreSQL admin/app URLs")
	}
	if os.Getenv("AGIPAREL_DESTRUCTIVE_DB_TESTS") != "I_UNDERSTAND_TEST_FIXTURES" {
		t.Fatal("explicit fixture opt-in required")
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
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	for _, db := range []*sql.DB{admin, app} {
		if e = db.PingContext(ctx); e != nil {
			t.Fatal(e)
		}
	}
	var role string
	if e = app.QueryRowContext(ctx, `SELECT current_user`).Scan(&role); e != nil || role != "agiparel_app" {
		t.Fatalf("restricted database role required: %s %v", role, e)
	}
	seed := make([]byte, 8)
	if _, e = rand.Read(seed); e != nil {
		t.Fatal(e)
	}
	suffix := hex.EncodeToString(seed)
	tenant, other := "S6-"+suffix, "X6-"+suffix
	_, e = admin.ExecContext(ctx, `INSERT INTO tenants(id,name) VALUES($1,'Batch06 owner'),($2,'Batch06 other')`, tenant, other)
	if e != nil {
		t.Fatal(e)
	}
	defer func() {
		for _, q := range []string{`DELETE FROM material_scenarios_v6 WHERE tenant_id=$1`, `DELETE FROM order_intake_bom_v5 WHERE tenant_id=$1`, `DELETE FROM order_intake_skus_v5 WHERE tenant_id=$1`, `DELETE FROM order_intakes_v5 WHERE tenant_id=$1`, `DELETE FROM tenants WHERE id=$1`} {
			admin.ExecContext(context.Background(), q, tenant)
		}
		admin.ExecContext(context.Background(), `DELETE FROM tenants WHERE id=$1`, other)
	}()
	intake := makeScenarioIntake(t)
	intake.ID = "OI-" + suffix + "1234567890abcdef"
	// Admin inserts one fixture intake and its BOM; app can only write material scenarios.
	_, e = admin.ExecContext(ctx, `INSERT INTO order_intakes_v5(id,tenant_id,po_number,style_code,ordered_qty,source_sha256,po_filename,bom_filename,po_original,bom_original) VALUES($1,$2,$3,$4,$5,$6,'buyer.csv','bom.csv',$7,$8)`, intake.ID, tenant, intake.PONumber, intake.StyleCode, intake.OrderedQty, intake.SourceSHA256, []byte(scenarioPO), []byte(scenarioBOM))
	if e != nil {
		t.Fatal(e)
	}
	for i, m := range intake.Materials {
		_, e = admin.ExecContext(ctx, `INSERT INTO order_intake_bom_v5(tenant_id,intake_id,line_no,material_sku,category,qty_per_garment,uom) VALUES($1,$2,$3,$4,$5,$6,$7)`, tenant, intake.ID, i+1, m.SKU, m.Category, m.PerGarment, m.UOM)
		if e != nil {
			t.Fatal(e)
		}
	}
	server := &EdgeServer{db: app}
	sourced, e := server.scenarioIntake(ctx, tenant, intake.ID)
	if e != nil {
		t.Fatal(e)
	}
	sources, e := buildScenario(sourced, []byte(scenarioStockHeader+scenarioEligible), []byte(scenarioPrices), "stock.csv", "price.csv")
	if e != nil {
		t.Fatal(e)
	}
	first, reused, e := server.persistMaterialScenario(ctx, tenant, sources)
	if e != nil || reused || first.ID == "" {
		t.Fatalf("persist failure: %+v %t %v", first, reused, e)
	}
	again, reused, e := server.persistMaterialScenario(ctx, tenant, sources)
	if e != nil || !reused || again.ID != first.ID {
		t.Fatalf("non-idempotent: %+v %t %v", again, reused, e)
	}
	var count int
	if e = admin.QueryRowContext(ctx, `SELECT count(*) FROM material_scenarios_v6 WHERE tenant_id=$1`, tenant).Scan(&count); e != nil || count != 1 {
		t.Fatalf("replay created duplicate: %d %v", count, e)
	}
	// Without scoped RLS, runtime role sees nothing; other tenant is denied.
	for _, identity := range []string{"", other} {
		tx, err := app.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
		if err != nil {
			t.Fatal(err)
		}
		tx.ExecContext(ctx, `SELECT set_config('app.current_tenant_id',$1,true)`, identity)
		var n int
		err = tx.QueryRowContext(ctx, `SELECT count(*) FROM material_scenarios_v6 WHERE id=$1`, first.ID).Scan(&n)
		tx.Rollback()
		if err != nil || n != 0 {
			t.Fatalf("tenant leakage to %q: %d %v", identity, n, err)
		}
	}
	var v json.RawMessage
	if e = admin.QueryRowContext(ctx, `SELECT report_json FROM material_scenarios_v6 WHERE id=$1`, first.ID).Scan(&v); e != nil || !json.Valid(v) {
		t.Fatalf("invalid saved audit: %v", e)
	}
	// An active app session cannot rewrite or delete historical scenario evidence.
	tx, e := app.BeginTx(ctx, nil)
	if e != nil {
		t.Fatal(e)
	}
	tx.ExecContext(ctx, `SELECT set_config('app.current_tenant_id',$1,true)`, tenant)
	if _, e = tx.ExecContext(ctx, `UPDATE material_scenarios_v6 SET release_gate='HOLD' WHERE id=$1`, first.ID); e == nil {
		t.Fatal("runtime role unexpectedly has UPDATE capability")
	}
	tx.Rollback()
}
