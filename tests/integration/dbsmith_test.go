package integration

import (
	"github.com/cyberinq/agiparel/services/edge-daemon/pkg/dbsmith"
	"testing"
)

func TestDBSmithDynamicMigration(t *testing.T) {
	smith := dbsmith.NewDBSmithEngine("TENANT_NOBLE_OKHLA")

	req := dbsmith.CustomFieldRequest{
		EntityName: "store_rolls",
		FieldName:  "supplier_heat_number",
		FieldType:  "VARCHAR(64)",
		IsNullable: true,
	}

	query, err := smith.GenerateSafeMigration(req)
	if err != nil {
		t.Fatalf("Failed to generate migration: %v", err)
	}

	expected := "ALTER TABLE store_rolls ADD COLUMN IF NOT EXISTS supplier_heat_number VARCHAR(64) DEFAULT NULL;"
	if query != expected {
		t.Errorf("Expected query:\n%s\nGot:\n%s", expected, query)
	}
}
