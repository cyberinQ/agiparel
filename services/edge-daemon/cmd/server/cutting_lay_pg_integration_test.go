// Opt-in rollback-only integration test. Requires a superuser connection to
// the disposable test database; creates no durable factory facts.
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

func TestCuttingPostgresRLSAndAppendOnly(t *testing.T) {
	dsn := os.Getenv("AGIPAREL_TEST_ADMIN_DATABASE_URL")
	if dsn == "" || os.Getenv("AGIPAREL_CUT_SQL_TEST_OPT_IN") != "ROLLBACK_ONLY" {
		t.Skip("explicit rollback-only PostgreSQL test opt-in required")
	}
	db, e := sql.Open("pgx", dsn)
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
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
		t.Skip("requires DB superuser for role switching")
	}
	var suffixBytes [8]byte
	if _, e = rand.Read(suffixBytes[:]); e != nil {
		t.Fatal(e)
	}
	suffix := hex.EncodeToString(suffixBytes[:])
	tenant, other := "CUT8-"+suffix, "CUT8-other-"+suffix
	intake := "OI-" + suffix
	if _, e = tx.ExecContext(ctx, `INSERT INTO tenants(id,name) VALUES($1,'08A rollback fixture'),($2,'08A other tenant')`, tenant, other); e != nil {
		t.Fatal(e)
	}
	if _, e = tx.ExecContext(ctx, `INSERT INTO order_intakes_v5(id,tenant_id,po_number,style_code,ordered_qty,source_sha256,po_filename,bom_filename,po_original,bom_original)
 VALUES($1,$2,'PO-08A','STYLE-08A',100,repeat('a',64),'buyer.csv','bom.csv',$3,$4)`, intake, tenant, []byte("po"), []byte("bom")); e != nil {
		t.Fatal(e)
	}
	if _, e = tx.ExecContext(ctx, `SET LOCAL ROLE agiparel_app`); e != nil {
		t.Fatal(e)
	}
	if _, e = tx.ExecContext(ctx, `SELECT set_config('app.current_tenant_id',$1,true)`, tenant); e != nil {
		t.Fatal(e)
	}
	var id string
	e = tx.QueryRowContext(ctx, `INSERT INTO cut_lay_drafts_v8(tenant_id,intake_id,cut_no,revision,request_key,input_sha256,source_order_sha256,marker_code,fabric_sku,shade_band,marker_length_mm,usable_width_mm,reported_plies,bundle_ply_group_size,planned_garments,review_note)
 VALUES($1,$2,1,1,'cut_rollback_fixture',repeat('b',64),repeat('a',64),'MARK-08A','FAB-08A','A',6500,1450,30,20,30,'Operator draft only, no actual production') RETURNING id::text`, tenant, intake).Scan(&id)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = tx.ExecContext(ctx, `INSERT INTO cut_lay_snapshots_v8(tenant_id,lay_draft_id,snapshot,snapshot_sha256) VALUES($1,$2,'{}'::jsonb,repeat('c',64))`, tenant, id); e != nil {
		t.Fatal(e)
	}
	if _, e = tx.ExecContext(ctx, `INSERT INTO cut_lay_positions_v8(tenant_id,lay_draft_id,marker_position,color_code,size_code) VALUES($1,$2,1,'BLACK','32')`, tenant, id); e != nil {
		t.Fatal(e)
	}
	if _, e = tx.ExecContext(ctx, `INSERT INTO cut_bundle_drafts_v8(tenant_id,lay_draft_id,label_code,marker_position,color_code,size_code,part_code,pieces_per_garment,ply_group,ply_start,ply_end,piece_count)
 VALUES($1,$2,'CB08-AAAAAAAAAAAAAAAAAAAAAAAA',1,'BLACK','32','FRONT',2,1,1,20,40)`, tenant, id); e != nil {
		t.Fatal(e)
	}
	var own int
	if e = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM cut_bundle_drafts_v8 WHERE tenant_id=$1`, tenant).Scan(&own); e != nil || own != 1 {
		t.Fatalf("own bundles=%d err=%v", own, e)
	}
	if _, e = tx.ExecContext(ctx, `SELECT set_config('app.current_tenant_id',$1,true)`, other); e != nil {
		t.Fatal(e)
	}
	if e = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM cut_bundle_drafts_v8 WHERE tenant_id=$1`, tenant).Scan(&own); e != nil || own != 0 {
		t.Fatalf("cross tenant leaked bundles=%d err=%v", own, e)
	}
	if _, e = tx.ExecContext(ctx, `RESET ROLE`); e != nil {
		t.Fatal(e)
	}
	if _, e = tx.ExecContext(ctx, `SAVEPOINT immutable_test`); e != nil {
		t.Fatal(e)
	}
	_, e = tx.ExecContext(ctx, `UPDATE cut_lay_drafts_v8 SET reported_plies=40 WHERE tenant_id=$1`, tenant)
	if e == nil {
		t.Fatal("immutable lay draft was changed")
	}
	if _, e = tx.ExecContext(ctx, `ROLLBACK TO SAVEPOINT immutable_test`); e != nil {
		t.Fatal(e)
	}
	if _, e = tx.ExecContext(ctx, `SAVEPOINT immutable_child_test`); e != nil {
		t.Fatal(e)
	}
	_, e = tx.ExecContext(ctx, `DELETE FROM cut_bundle_drafts_v8 WHERE tenant_id=$1`, tenant)
	if e == nil {
		t.Fatal("immutable bundle draft was deleted")
	}
	if _, e = tx.ExecContext(ctx, `ROLLBACK TO SAVEPOINT immutable_child_test`); e != nil {
		t.Fatal(e)
	}
}
