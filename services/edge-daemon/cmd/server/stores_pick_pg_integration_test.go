// Batch 07D. Opt-in, ROLLBACK-ONLY PostgreSQL tests. Never truncate live data.
package main

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"os"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func TestStoresPickPostgresTenantAndImmutability(t *testing.T) {
	dsn := os.Getenv("AGIPAREL_TEST_ADMIN_DATABASE_URL")
	if dsn == "" {
		t.Skip("requires explicit PostgreSQL admin test connection")
	}
	if os.Getenv("AGIPAREL_PICK_SQL_TEST_OPT_IN") != "ROLLBACK_ONLY" {
		t.Skip("requires AGIPAREL_PICK_SQL_TEST_OPT_IN=ROLLBACK_ONLY")
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback() // Absolute isolation: no fixture or schema changes persist.
	var isSuper bool
	if err = tx.QueryRowContext(ctx, `SELECT rolsuper FROM pg_roles WHERE rolname=current_user`).Scan(&isSuper); err != nil {
		t.Fatal(err)
	}
	if !isSuper {
		t.Skip("rollback-only role-switch test requires admin/superuser")
	}
	var bytes [8]byte
	if _, err = rand.Read(bytes[:]); err != nil {
		t.Fatal(err)
	}
	suffix := hex.EncodeToString(bytes[:])
	tenant, other := "P7D-"+suffix, "P7D-other-"+suffix
	orderID := "O7D-" + suffix
	key := "pick_" + suffix
	if _, err = tx.ExecContext(ctx, `INSERT INTO tenants(id,name) VALUES($1,'07D rollback fixture'),($2,'07D rollback other')`, tenant, other); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO order_intakes_v5 (id,tenant_id,po_number,style_code,ordered_qty,source_sha256,po_filename,bom_filename,po_original,bom_original)
 VALUES($1,$2,'PO-07D','STYLE-07D',100,repeat('a',64),'test.csv','bom.csv',$3,$4)`, orderID, tenant, []byte("test"), []byte("bom")); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.ExecContext(ctx, `SET LOCAL ROLE agiparel_app`); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.ExecContext(ctx, `SELECT set_config('app.current_tenant_id',$1,true)`, tenant); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO store_pick_proposals_v7(tenant_id,intake_id,request_key,review_note,source_order_sha256,snapshot,snapshot_sha256)
 VALUES($1,$2,$3,'Rollback-only planned roll selection',repeat('a',64),'{}'::jsonb,repeat('b',64))`, tenant, orderID, key); err != nil {
		t.Fatal(err)
	}
	var count int
	if err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM store_pick_proposals_v7 WHERE tenant_id=$1`, tenant).Scan(&count); err != nil || count != 1 {
		t.Fatalf("own tenant pick count=%d err=%v", count, err)
	}
	if _, err = tx.ExecContext(ctx, `SELECT set_config('app.current_tenant_id',$1,true)`, other); err != nil {
		t.Fatal(err)
	}
	if err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM store_pick_proposals_v7 WHERE tenant_id=$1`, tenant).Scan(&count); err != nil || count != 0 {
		t.Fatalf("cross tenant leaked %d %v", count, err)
	}
	// Return to admin role and attempt an UPDATE with full DB privileges.
	if _, err = tx.ExecContext(ctx, `RESET ROLE`); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.ExecContext(ctx, `SAVEPOINT immutable_attempt`); err != nil {
		t.Fatal(err)
	}
	_, err = tx.ExecContext(ctx, `UPDATE store_pick_proposals_v7 SET review_note='changed' WHERE tenant_id=$1`, tenant)
	if err == nil {
		t.Fatal("immutable planning proposal unexpectedly updated")
	}
	if _, e := tx.ExecContext(ctx, `ROLLBACK TO SAVEPOINT immutable_attempt`); e != nil {
		t.Fatal(e)
	}
}
