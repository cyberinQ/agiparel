// Author: Sunil Jain, CyberinQ LLC, 2026-10-26.
package stores

import (
	"context"
	"database/sql"
	"errors"
	"strings"
)

// AuthorityHolder is a deploy-time service identity, not a request-controlled field.
// Lease activation is an offline/governed DB provisioning action; no public API here.
func (r MaterialRepository) requireAuthorizedScope(ctx context.Context, tx *sql.Tx, tenant, order, plant string) error {
	if strings.TrimSpace(r.AuthorityHolder) == "" {
		return errors.New("ERR_MATERIAL_AUTHORITY_UNCONFIGURED")
	}
	var mode string
	var fenced bool
	err := tx.QueryRowContext(ctx, `SELECT mode,legacy_writer_fenced FROM material_write_scopes_v5
   WHERE tenant_id=$1 AND order_id=$2 AND plant_id=$3 AND lease_holder=$4 AND lease_expires_at>now() FOR SHARE`,
		tenant, order, plant, r.AuthorityHolder).Scan(&mode, &fenced)
	if errors.Is(err, sql.ErrNoRows) {
		return errors.New("ERR_MATERIAL_SCOPE_NOT_DELEGATED")
	}
	if err != nil {
		return err
	}
	if !fenced || (mode != "M3" && mode != "M4") {
		return errors.New("ERR_MATERIAL_SCOPE_NOT_DELEGATED")
	}
	return nil
}
