package main

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

// Safe, opt-in ROLLBACK_ONLY policy check. Requires a disposable database on
// which migrations 0001..0021 were applied. No permanent test factory data.
func TestCuttingReaderSecurityPostgresRollbackOnly(t *testing.T) {
	if os.Getenv("AGIPAREL_CUT_SQL_TEST_OPT_IN") != "ROLLBACK_ONLY" || os.Getenv("AGIPAREL_TEST_ADMIN_DATABASE_URL") == "" {
		t.Skip("requires rollback-only opt-in + test database")
	}
	db, e := sql.Open("pgx", os.Getenv("AGIPAREL_TEST_ADMIN_DATABASE_URL"))
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	tx, e := db.BeginTx(ctx, nil)
	if e != nil {
		t.Fatal(e)
	}
	defer tx.Rollback()
	for _, name := range []string{"cut_reader_gateways_v8", "cut_reader_events_v8", "cut_sewing_releases_v8"} {
		var rls, forced bool
		e = tx.QueryRowContext(ctx, `SELECT relrowsecurity,relforcerowsecurity FROM pg_class WHERE oid=to_regclass($1)`, "public."+name).Scan(&rls, &forced)
		if e != nil || !rls || !forced {
			t.Fatalf("table %s RLS: enabled=%v forced=%v error=%v", name, rls, forced, e)
		}
	}
	var appCanInsertGateways, appCanInsertEvents bool
	e = tx.QueryRowContext(ctx, `SELECT has_table_privilege('agiparel_app','public.cut_reader_gateways_v8','INSERT'),has_table_privilege('agiparel_app','public.cut_reader_events_v8','INSERT')`).Scan(&appCanInsertGateways, &appCanInsertEvents)
	if e != nil || appCanInsertGateways || !appCanInsertEvents {
		t.Fatalf("app reader privileges: gateways insert=%v events insert=%v error=%v", appCanInsertGateways, appCanInsertEvents, e)
	}
	var super bool
	if e = tx.QueryRowContext(ctx, `SELECT rolsuper FROM pg_roles WHERE rolname=current_user`).Scan(&super); e != nil {
		t.Fatal(e)
	}
	if super {
		if _, e = tx.ExecContext(ctx, `SET LOCAL ROLE agiparel_app`); e != nil {
			t.Fatal(e)
		}
		var total int
		// No app.current_tenant_id is configured, so RLS must see no rows.
		e = tx.QueryRowContext(ctx, `SELECT count(*) FROM cut_reader_events_v8`).Scan(&total)
		if e != nil || total != 0 {
			t.Fatalf("tenant-unset reader events exposed: %d %v", total, e)
		}
	}
}
