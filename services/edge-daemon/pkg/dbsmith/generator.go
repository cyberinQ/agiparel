package dbsmith

import (
	"encoding/json"
	"fmt"
	"strings"
)

type CustomFieldRequest struct {
	EntityName string `json:"entity_name"` // e.g., "store_rolls", "bundle_registry"
	FieldName  string `json:"field_name"`  // e.g., "dye_lot_batch"
	FieldType  string `json:"field_type"`  // e.g., "VARCHAR(64)", "NUMERIC(10,2)", "JSONB"
	IsNullable bool   `json:"is_nullable"`
}

type DBSmithEngine struct {
	tenantID string
}

func NewDBSmithEngine(tenantID string) *DBSmithEngine {
	return &DBSmithEngine{tenantID: tenantID}
}

// GenerateSafeMigration generates an ALTER TABLE script to safely extend schemas on the fly.
func (s *DBSmithEngine) GenerateSafeMigration(req CustomFieldRequest) (string, error) {
	entity := strings.ToLower(req.EntityName)
	if entity != "store_rolls" && entity != "bundle_registry" && entity != "floor_issues" {
		return "", fmt.Errorf("ERR_INVALID_ENTITY: Entity %s is protected or unsupported for dynamic extension", req.EntityName)
	}

	colName := strings.ToLower(req.FieldName)
	colType := strings.ToUpper(req.FieldType)

	nullClause := "NOT NULL DEFAULT ''"
	if req.IsNullable {
		nullClause = "DEFAULT NULL"
	}

	sqlQuery := fmt.Sprintf(
		"ALTER TABLE %s ADD COLUMN IF NOT EXISTS %s %s %s;",
		entity, colName, colType, nullClause,
	)

	return sqlQuery, nil
}

func (s *DBSmithEngine) GenerateDynamicJSONBPayload(data map[string]interface{}) (string, error) {
	bytes, err := json.Marshal(data)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}