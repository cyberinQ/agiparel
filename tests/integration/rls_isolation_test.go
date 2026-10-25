// Author: Sunil Jain, 2026-10-25 — Keep PostgreSQL integration tests opt-in.
package integration

import (
	"context"
	"database/sql"
	"os"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func TestRLSTenantIsolation(t *testing.T) {
	if os.Getenv("RUN_DB_TESTS") != "1" {
		t.Skip("Set RUN_DB_TESTS=1 for PostgreSQL integration tests")
	}
	ctx := context.Background()
	connStr := "postgres://agiparel_app:app_secure_runtime_pass@localhost:5432/agiparel_dev?sslmode=disable"
	db, err := sql.Open("pgx", connStr)
	if err != nil {
		t.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// 0. Ensure foreign key parent tenants exist
	tenantSetupQuery := `
		INSERT INTO tenants (id, name) VALUES 
		('TENANT_NOBLE_OKHLA', 'Noble Garments Okhla Unit'),
		('TENANT_TEST_99', 'Test Tenant 99')
		ON CONFLICT (id) DO NOTHING;
	`
	if _, err := db.ExecContext(ctx, tenantSetupQuery); err != nil {
		t.Fatalf("Failed to seed prerequisite test tenants: %v", err)
	}

	rollA_ID := "ROLL-NOBLE-001"

	// 1. Tenant A writes a record inside a tenant-scoped transaction
	txA, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("Failed to begin transaction A: %v", err)
	}
	defer txA.Rollback()

	if _, err := txA.ExecContext(ctx, "SET LOCAL app.current_tenant_id = 'TENANT_NOBLE_OKHLA'"); err != nil {
		t.Fatalf("Failed to set tenant context for A: %v", err)
	}

	_, err = txA.ExecContext(ctx, `
		INSERT INTO store_rolls (id, tenant_id, factory_sku, original_length_m, remaining_length_m, usable_width_cm, shade_band, bin_location, status)
		VALUES ($1, 'TENANT_NOBLE_OKHLA', 'DENIM-STRETCH-12OZ', 100.0, 100.0, 148.0, 'A', 'BIN-A1', 'AVAILABLE')
		ON CONFLICT (id) DO UPDATE SET remaining_length_m = EXCLUDED.remaining_length_m;
	`, rollA_ID)
	if err != nil {
		t.Fatalf("Failed to insert roll for tenant A: %v", err)
	}

	if err := txA.Commit(); err != nil {
		t.Fatalf("Failed to commit transaction A: %v", err)
	}

	// 2. Tenant B attempts to read Tenant A's record inside its own tenant context
	txB, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("Failed to begin transaction B: %v", err)
	}
	defer txB.Rollback()

	if _, err := txB.ExecContext(ctx, "SET LOCAL app.current_tenant_id = 'TENANT_TEST_99'"); err != nil {
		t.Fatalf("Failed to set tenant context for B: %v", err)
	}

	var count int
	err = txB.QueryRowContext(ctx, "SELECT count(*) FROM store_rolls WHERE id = $1", rollA_ID).Scan(&count)
	if err != nil {
		t.Fatalf("Failed to scan rolls under Tenant B: %v", err)
	}

	if count != 0 {
		t.Fatalf("RLS VIOLATION: Tenant B detected %d records belonging to Tenant A", count)
	}
}
