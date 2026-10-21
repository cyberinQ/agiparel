package sqlguard

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

		tables := make([]string, 0)
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

	cols := make([]ColumnInfo, 0)
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

func (c *CatalogInspector) ExecuteSafeSQL(ctx context.Context, query string) (string, error) {
	if c.db == nil {
		return "", errors.New("database connection is nil")
	}

	// Clean trailing semicolons and whitespace
	cleanQuery := strings.TrimSpace(query)
	cleanQuery = strings.TrimRight(cleanQuery, "; \t\n\r")
	upper := strings.ToUpper(cleanQuery)

	if !strings.HasPrefix(upper, "SELECT") {
		return "", fmt.Errorf("SECURITY_ERROR: Only SELECT queries are permitted (received: %s)", cleanQuery)
	}

	// Strictly block destructive keywords and dangerous system functions
	destructiveTokens := []string{
		"INSERT", "UPDATE", "DELETE", "DROP", "ALTER", "TRUNCATE", "GRANT", "REVOKE",
		"PG_SLEEP", "PG_READ_FILE", "PG_STAT_FILE", "LO_IMPORT", "LO_EXPORT", "DBLINK", "SET_CONFIG",
	}
	for _, tok := range destructiveTokens {
		if hasWholeWord(upper, tok) {
			return "", fmt.Errorf("SECURITY_ERROR: Query contains forbidden token: '%s'", tok)
		}
	}

	// Disallow stacked queries (semicolons inside the statement)
	if strings.Contains(cleanQuery, ";") {
		return "", errors.New("SECURITY_ERROR: Stacked queries with ';' are forbidden")
	}

	queryCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	tx, err := c.db.BeginTx(queryCtx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return "", fmt.Errorf("SQL_TX_ERROR: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(queryCtx, "SET LOCAL app.current_tenant_id = 'TENANT_DEFAULT'"); err != nil {
		return "", fmt.Errorf("RLS_CONTEXT_ERROR: %w", err)
	}

	// Hard limit of 100 rows for edge safety
	boundedQuery := fmt.Sprintf("SELECT * FROM (%s) AS q LIMIT 100", cleanQuery)

	rows, err := tx.QueryContext(queryCtx, boundedQuery)
	if err != nil {
		return "", fmt.Errorf("SQL_ERROR: %w", err)
	}
	defer rows.Close()

	colNames, err := rows.Columns()
	if err != nil {
		return "", err
	}

	results := make([]map[string]interface{}, 0)
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

func hasWholeWord(s, word string) bool {
	fields := strings.FieldsFunc(s, func(r rune) bool {
		return r == ' ' || r == '\t' || r == '\n' || r == '(' || r == ')' || r == ','
	})
	for _, f := range fields {
		if f == word {
			return true
		}
	}
	return false
}