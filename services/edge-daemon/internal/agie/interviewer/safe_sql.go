package interviewer

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

type CatalogInspector struct {
	db *sql.DB
}

func NewCatalogInspector(db *sql.DB) *CatalogInspector {
	return &CatalogInspector{db: db}
}

// InspectCatalog returns table names or detailed column information.
func (c *CatalogInspector) InspectCatalog(ctx context.Context, tableName string) (string, error) {
	if c.db == nil {
		return "", errors.New("database connection is nil")
	}

	queryCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if strings.TrimSpace(tableName) == "" {
		rows, err := c.db.QueryContext(queryCtx, `
			SELECT table_name 
			FROM information_schema.tables 
			WHERE table_schema = 'public' AND table_type = 'BASE TABLE'
			ORDER BY table_name;
		`)
		if err != nil {
			return "", err
		}
		defer rows.Close()

		var tables []string
		for rows.Next() {
			var tbl string
			if err := rows.Scan(&tbl); err == nil {
				tables = append(tables, tbl)
			}
		}
		out, _ := json.Marshal(map[string]interface{}{"public_tables": tables})
		return string(out), nil
	}

	rows, err := c.db.QueryContext(queryCtx, `
		SELECT column_name, data_type, is_nullable
		FROM information_schema.columns
		WHERE table_schema = 'public' AND table_name = $1
		ORDER BY ordinal_position;
	`, tableName)
	if err != nil {
		return "", err
	}
	defer rows.Close()

	type ColumnInfo struct {
		Column   string `json:"column"`
		Type     string `json:"type"`
		Nullable string `json:"nullable"`
	}

	var cols []ColumnInfo
	for rows.Next() {
		var col, dtype, nullable string
		if err := rows.Scan(&col, &dtype, &nullable); err == nil {
			cols = append(cols, ColumnInfo{Column: col, Type: dtype, Nullable: nullable})
		}
	}

	out, _ := json.Marshal(map[string]interface{}{
		"table":   tableName,
		"columns": cols,
	})
	return string(out), nil
}

// ExecuteSafeSQL validates that the query is strictly a read-only SELECT and bounds the output.
func (c *CatalogInspector) ExecuteSafeSQL(ctx context.Context, query string) (string, error) {
	if c.db == nil {
		return "", errors.New("database connection is nil")
	}

	cleanQuery := strings.TrimSpace(query)
	upper := strings.ToUpper(cleanQuery)

	if !strings.HasPrefix(upper, "SELECT") {
		return "", fmt.Errorf("SECURITY_ERROR: Only SELECT queries are permitted (received: %s)", cleanQuery)
	}

	// Strictly block destructive keywords and multiple statements
	forbiddenTokens := []string{"INSERT", "UPDATE", "DELETE", "DROP", "ALTER", "TRUNCATE", "GRANT", "REVOKE", ";"}
	for _, tok := range forbiddenTokens {
		// Enforce token separation to prevent catching substrings inside column names
		if containsToken(upper, tok) {
			return "", fmt.Errorf("SECURITY_ERROR: Query contains forbidden token: '%s'", tok)
		}
	}

	// Wrap in subquery with hard ceiling of 100 rows
	boundedQuery := fmt.Sprintf("SELECT * FROM (%s) AS q LIMIT 100", cleanQuery)

	queryCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	rows, err := c.db.QueryContext(queryCtx, boundedQuery)
	if err != nil {
		return "", fmt.Errorf("SQL_ERROR: %w", err)
	}
	defer rows.Close()

	colNames, err := rows.Columns()
	if err != nil {
		return "", err
	}

	var results []map[string]interface{}
	for rows.Next() {
		colVals := make([]interface{}, len(colNames))
		colPointers := make([]interface{}, len(colNames))
		for i := range colVals {
			colPointers[i] = &colVals[i]
		}

		if err := rows.Scan(colPointers...); err != nil {
			return "", err
		}

		rowMap := make(map[string]interface{})
		for i, col := range colNames {
			val := colVals[i]
			if b, ok := val.([]byte); ok {
				rowMap[col] = string(b)
			} else {
				rowMap[col] = val
			}
		}
		results = append(results, rowMap)
	}

	data, err := json.Marshal(results)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func containsToken(query, token string) bool {
	if token == ";" {
		return strings.Contains(query, ";")
	}
	for _, word := range strings.Fields(query) {
		if strings.Trim(word, "(),") == token {
			return true
		}
	}
	return false
}