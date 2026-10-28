// Author: Sunil Jain, 2026-10-27 — Idempotently provision the development
// operator's factory identity before uploading documents or staging orders.
// Never provision production tenants from the application's runtime role.
package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
)

type tenantBootstrapDB interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

func ensureDevelopmentOperatorTenant(ctx context.Context, db tenantBootstrapDB) error {
	if os.Getenv("ENV") != "development" {
		return nil // Production identity and provisioning are external.
	}
	if db == nil {
		return errors.New("development database is not configured")
	}
	tenant, err := intakeTenant() // Uses the server-controlled operator tenant.
	if err != nil {
		return fmt.Errorf("invalid development tenant identity: %w", err)
	}
	// Existing tenants and their names must never be rewritten. The tenant ID
	// comes from trusted server configuration, never a document or browser.
	_, err = db.ExecContext(ctx,
		`INSERT INTO tenants(id,name) VALUES($1,$2) ON CONFLICT (id) DO NOTHING`,
		tenant, "Development factory ("+tenant+")")
	if err != nil {
		return fmt.Errorf("database tenant provisioning failed: %w", err)
	}
	return nil
}
