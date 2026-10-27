// Batch 08B. Explicit opt-in rollback-only test on a fully migrated disposable
// PostgreSQL database. It does not create any lasting factory or RFID facts.
package main

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	_ "github.com/jackc/pgx/v5/stdlib"
	"os"
	"testing"
	"time"
)

func TestCuttingSpreadPGConstraintsRollbackOnly(t *testing.T) {
	dsn := os.Getenv("AGIPAREL_TEST_ADMIN_DATABASE_URL")
	if dsn == "" || os.Getenv("AGIPAREL_CUT_SQL_TEST_OPT_IN") != "ROLLBACK_ONLY" {
		t.Skip("requires admin test DSN + explicit rollback-only opt-in")
	}
	db, e := sql.Open("pgx", dsn)
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 35*time.Second)
	defer cancel()
	tx, e := db.BeginTx(ctx, nil)
	if e != nil {
		t.Fatal(e)
	}
	defer tx.Rollback()
	var super bool
	if e = tx.QueryRowContext(ctx, `SELECT rolsuper FROM pg_roles WHERE rolname=current_user`).Scan(&super); e != nil {
		t.Fatal(e)
	}
	if !super {
		t.Skip("superuser needed for rollback-only SET ROLE")
	}
	var randBytes [6]byte
	if _, e = rand.Read(randBytes[:]); e != nil {
		t.Fatal(e)
	}
	tag := hex.EncodeToString(randBytes[:])
	tenant, other, order, roll := "CUTB-"+tag, "CUTX-"+tag, "OI-"+tag, "RL-"+tag
	exec := func(q string, args ...any) {
		t.Helper()
		if _, x := tx.ExecContext(ctx, q, args...); x != nil {
			t.Fatal(x)
		}
	}
	exec(`INSERT INTO tenants(id,name) VALUES($1,'08B rollback fixture'),($2,'08B other tenant')`, tenant, other)
	exec(`INSERT INTO order_intakes_v5(id,tenant_id,po_number,style_code,ordered_qty,source_sha256,po_filename,bom_filename,po_original,bom_original)
 VALUES($1,$2,$3,'STYLE-08B',100,repeat('a',64),'po.csv','bom.csv',$4,$5)`, order, tenant, "PO-"+tag, []byte("po"), []byte("bom"))
	exec(`INSERT INTO store_rolls(id,tenant_id,plant_id,factory_sku,original_length_m,remaining_length_m,usable_width_cm,shade_band,bin_location,status,inspection_status,reserved_order_id)
 VALUES($1,$2,'PLANT_DEFAULT','FAB-08B',100,100,150,'A','BIN-08B','AVAILABLE','PASS',$3)`, roll, tenant, order)
	// Fixtures are syntactically valid but entirely synthetic and rolled back;
	// production material issue transactions still demand real signatures.
	var reserveID, issueID, draftID, reportID string
	e = tx.QueryRowContext(ctx, `INSERT INTO material_reservations_v5(tenant_id,roll_id,order_id,cut_no,reserved_mm,request_key)
 VALUES($1,$2,$3,1,50000,$4) RETURNING id::text`, tenant, roll, order, "fixture_reserve_"+tag).Scan(&reserveID)
	if e != nil {
		t.Fatal(e)
	}
	e = tx.QueryRowContext(ctx, `INSERT INTO material_issues_v5(tenant_id,reservation_id,roll_id,order_id,cut_no,length_mm,command_key,issuer_subject,recipient_subject,issuer_proof_id,recipient_proof_id,issuer_assertion_hash,recipient_assertion_hash,signed_challenge_hash)
 VALUES($1,$2::uuid,$3,$4,1,50000,$5,'issuer-fixture','recipient-fixture',$6,$7,repeat('c',64),repeat('d',64),repeat('e',64)) RETURNING id::text`, tenant, reserveID, roll, order, "fixture_issue_"+tag, "fi-"+tag, "fr-"+tag).Scan(&issueID)
	if e != nil {
		t.Fatal(e)
	}
	e = tx.QueryRowContext(ctx, `INSERT INTO cut_lay_drafts_v8(tenant_id,intake_id,cut_no,revision,request_key,input_sha256,source_order_sha256,marker_code,fabric_sku,shade_band,marker_length_mm,usable_width_mm,reported_plies,bundle_ply_group_size,planned_garments,review_note)
 VALUES($1,$2,1,1,$3,repeat('b',64),repeat('a',64),'MK-08B','FAB-08B','A',1000,1500,20,20,20,'Rollback-only cutting draft fixture') RETURNING id::text`, tenant, order, "fixture_cut_"+tag).Scan(&draftID)
	if e != nil {
		t.Fatal(e)
	}
	exec(`INSERT INTO cut_lay_positions_v8(tenant_id,lay_draft_id,marker_position,color_code,size_code) VALUES($1,$2::uuid,1,'BLACK','L')`, tenant, draftID)
	label := "CB08-FFFFFFFFFFFFFFFFFFFFFFFF"
	exec(`INSERT INTO cut_bundle_drafts_v8(tenant_id,lay_draft_id,label_code,marker_position,color_code,size_code,part_code,pieces_per_garment,ply_group,ply_start,ply_end,piece_count)
 VALUES($1,$2::uuid,$3,1,'BLACK','L','FRONT',2,1,1,20,40)`, tenant, draftID, label)
	// The app role is allowed to append evidence, but always tenant-restricted.
	exec(`SET LOCAL ROLE agiparel_app`)
	exec(`SELECT set_config('app.current_tenant_id',$1,true)`, tenant)
	e = tx.QueryRowContext(ctx, `INSERT INTO cut_spread_reports_v8(tenant_id,lay_draft_id,request_key,input_sha256,draft_snapshot_sha256,reported_actual_plies,measured_marker_length_mm,reported_consumption_mm,evidence_note)
 VALUES($1,$2::uuid,$3,repeat('a',64),repeat('b',64),20,1000,40000,'Rollback-only reported physical spread') RETURNING id::text`, tenant, draftID, "fixture_spread_"+tag).Scan(&reportID)
	if e != nil {
		t.Fatal(e)
	}
	exec(`INSERT INTO cut_spread_issue_allocations_v8(tenant_id,spread_report_id,issue_id,allocated_mm) VALUES($1,$2::uuid,$3::uuid,40000)`, tenant, reportID, issueID)
	exec(`SET CONSTRAINTS cut_spread_full_coverage_v8 IMMEDIATE`)
	exec(`INSERT INTO cut_bundle_scan_witnesses_v8(tenant_id,plant_id,spread_report_id,lay_draft_id,label_code,epc_hex,scanner_subject,scanner_proof_id,scanner_assertion_hash,signed_challenge_hash)
 VALUES($1,'PLANT_DEFAULT',$2::uuid,$3::uuid,$4,'300833B2DDD9014000000001','operator-fixture',$5,repeat('f',64),repeat('e',64))`, tenant, reportID, draftID, label, "proof_fixture_"+tag)
	var count int
	if e = tx.QueryRowContext(ctx, `SELECT count(*) FROM cut_bundle_scan_witnesses_v8 WHERE tenant_id=$1`, tenant).Scan(&count); e != nil || count != 1 {
		t.Fatalf("own witnesses=%d: %v", count, e)
	}
	exec(`SELECT set_config('app.current_tenant_id',$1,true)`, other)
	if e = tx.QueryRowContext(ctx, `SELECT count(*) FROM cut_bundle_scan_witnesses_v8 WHERE tenant_id=$1`, tenant).Scan(&count); e != nil || count != 0 {
		t.Fatalf("cross tenant visible: %d %v", count, e)
	}
	exec(`RESET ROLE`)
	exec(`SAVEPOINT immutable_check`)
	if _, e = tx.ExecContext(ctx, `DELETE FROM cut_spread_reports_v8 WHERE tenant_id=$1`, tenant); e == nil {
		t.Fatal("immutable spread was deleted")
	}
	exec(`ROLLBACK TO SAVEPOINT immutable_check`)
	exec(`SAVEPOINT epc_check`)
	if _, e = tx.ExecContext(ctx, `INSERT INTO store_roll_rfid_bindings_v7(tenant_id,epc_hex,roll_id) VALUES($1,'300833B2DDD9014000000001',$2)`, tenant, roll); e == nil {
		t.Fatal("roll-bound EPC was allowed to collide with a cut bundle")
	}
	exec(`ROLLBACK TO SAVEPOINT epc_check`)
	exec(`SAVEPOINT overissue_check`)
	// A second allocation for the same issue cannot exceed its signed amount.
	if _, e = tx.ExecContext(ctx, `INSERT INTO cut_spread_issue_allocations_v8(tenant_id,spread_report_id,issue_id,allocated_mm) VALUES($1,$2::uuid,$3::uuid,11000)`, tenant, reportID, issueID); e == nil {
		t.Fatal("double attribution accepted")
	}
	exec(`ROLLBACK TO SAVEPOINT overissue_check`)
}
