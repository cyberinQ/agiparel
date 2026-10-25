package identity

import (
	"context"
	"database/sql"
	"fmt"
)

type TenantContextManager struct {
	db *sql.DB
}

func NewTenantContextManager(db *sql.DB) *TenantContextManager {
	return &TenantContextManager{db: db}
}

// WithTenantTx executes a database operation within a transaction strictly scoped to the tenant's RLS policy.
func (t *TenantContextManager) WithTenantTx(ctx context.Context, tenantID string, fn func(tx *sql.Tx) error) error {
	if t.db == nil {
		return fmt.Errorf("ERR_DB_NIL: Database connection not initialized")
	}
	
	tx, err := t.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("ERR_TX_BEGIN: %w", err)
	}
	defer tx.Rollback()

	// Enforce PostgreSQL session-level RLS context
	if _, err := tx.ExecContext(ctx, "SELECT set_config('app.current_tenant_id', $1, true)", tenantID); err != nil {
		return fmt.Errorf("ERR_RLS_SET: Failed to establish tenant scope %s: %w", tenantID, err)
	}

	if err := fn(tx); err != nil {
		return err
	}

	return tx.Commit()
}