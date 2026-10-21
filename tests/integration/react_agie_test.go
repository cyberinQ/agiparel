package integration

import (
	"context"
	"database/sql"
	"strings"
	"testing"

	"github.com/cyberinq/agiparel/services/edge-daemon/pkg/sqlguard"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func TestCatalogInspectorSafeSQL(t *testing.T) {
	ctx := context.Background()
	connStr := "postgres://agiparel_app:app_secure_runtime_pass@localhost:5432/agiparel_dev?sslmode=disable"
	db, err := sql.Open("pgx", connStr)
	if err != nil {
		t.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// 0. Ipasdek ti session-level tenant context tapno maikkat ti RLS isolation filter
	_, err = db.ExecContext(ctx, "SET app.current_tenant_id = 'TENANT_DEFAULT'")
	if err != nil {
		t.Fatalf("Failed to set tenant context: %v", err)
	}

	// Siguraduen nga adda test worker ken attendance record para ken W-03
	_, err = db.ExecContext(ctx, `
		INSERT INTO tenants (id, name) VALUES ('TENANT_DEFAULT', 'Noble Garments') ON CONFLICT DO NOTHING;
		INSERT INTO workers (id, tenant_id, name, employee_code, primary_skill, skill_grade)
		VALUES ('W-03', 'TENANT_DEFAULT', 'Sunita Sharma', 'EMP_A813', 'KANSAI_HEMMING', 'B')
		ON CONFLICT (id) DO NOTHING;
		INSERT INTO attendance_register (tenant_id, worker_id, shift_date, status, assigned_line_id, assigned_station_id)
		VALUES ('TENANT_DEFAULT', 'W-03', CURRENT_DATE, 'ABSENT', 'Line 14', 'ST-03')
		ON CONFLICT DO NOTHING;
	`)
	if err != nil {
		t.Fatalf("Failed to seed attendance record: %v", err)
	}

	inspector := sqlguard.NewCatalogInspector(db)

	// 1. Test Schema Inspection
	tablesJSON, err := inspector.InspectCatalog(ctx, "")
	if err != nil {
		t.Fatalf("InspectCatalog failed: %v", err)
	}
	if !strings.Contains(tablesJSON, "attendance_register") {
		t.Errorf("Expected attendance_register in catalog, got: %s", tablesJSON)
	}

	// 2. Test Safe Read Query on Seeded Data
	query := "SELECT worker_id, status FROM attendance_register WHERE status = 'ABSENT'"
	resultsJSON, err := inspector.ExecuteSafeSQL(ctx, query)
	if err != nil {
		t.Fatalf("ExecuteSafeSQL valid query failed: %v", err)
	}
	if !strings.Contains(resultsJSON, "W-03") {
		t.Errorf("Expected absent worker W-03 in results, got: %s", resultsJSON)
	}

	// 3. Test Security Guardrail: Reject Mutation
	badQuery := "DELETE FROM attendance_register WHERE worker_id = 'W-03'"
	_, err = inspector.ExecuteSafeSQL(ctx, badQuery)
	if err == nil {
		t.Fatalf("Expected security error on DELETE query, got nil")
	}

	// 4. Test Security Guardrail: Reject Semicolon Multi-Statement
	stackedQuery := "SELECT 1; DROP TABLE workers"
	_, err = inspector.ExecuteSafeSQL(ctx, stackedQuery)
	if err == nil {
		t.Fatalf("Expected security error on stacked query, got nil")
	}
}