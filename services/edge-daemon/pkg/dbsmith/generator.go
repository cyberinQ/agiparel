package dbsmith

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

var validIdentifierRe = regexp.MustCompile(`^[a-z][a-z0-9_]{1,62}$`)

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
	entity := strings.ToLower(strings.TrimSpace(req.EntityName))
	if entity != "store_rolls" && entity != "bundle_registry" && entity != "floor_issues" {
		return "", fmt.Errorf("ERR_INVALID_ENTITY: Entity %s is protected or unsupported for dynamic extension", req.EntityName)
	}

	colName := strings.ToLower(strings.TrimSpace(req.FieldName))
	if !validIdentifierRe.MatchString(colName) {
		return "", fmt.Errorf("ERR_INVALID_FIELD_NAME: Field name '%s' violates SQL identifier safety rules", req.FieldName)
	}

	colType := strings.ToUpper(strings.TrimSpace(req.FieldType))
	var defaultNonNull string
	switch colType {
	case "VARCHAR(64)", "VARCHAR(128)", "VARCHAR(255)", "TEXT":
		defaultNonNull = "NOT NULL DEFAULT ''"
	case "NUMERIC(10,2)", "NUMERIC(6,2)", "INT", "BIGINT":
		defaultNonNull = "NOT NULL DEFAULT 0"
	case "JSONB":
		defaultNonNull = "NOT NULL DEFAULT '{}'::jsonb"
	case "BOOLEAN":
		defaultNonNull = "NOT NULL DEFAULT false"
	case "TIMESTAMPTZ":
		defaultNonNull = "NOT NULL DEFAULT CURRENT_TIMESTAMP"
	default:
		return "", fmt.Errorf("ERR_UNSUPPORTED_TYPE: Column type '%s' is not whitelisted", req.FieldType)
	}

	nullClause := defaultNonNull
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
