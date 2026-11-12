// Author: Sunil Jain, 2026-11-11 — Frozen SOP-2 target is a single authority for
// cutting-draft quantity caps; fall back to the older buyer-order limit until
// a target exists. Physical release stays governed by the signed v8 workflow.
package main

import (
	"context"
	"database/sql"
	"errors"
)

func resolveSOP2CutLimits(ctx context.Context, tx *sql.Tx, tenant, intake string, buyer map[string]int64, buyerTotal int64) (map[string]int64, int64, bool, error) {
	var schemaAvailable bool
	if err := tx.QueryRowContext(ctx, `SELECT to_regclass('public.cut_sop2_target_versions_v46') IS NOT NULL`).Scan(&schemaAvailable); err != nil {
		return nil, 0, false, err
	}
	if !schemaAvailable {
		return buyer, buyerTotal, false, nil
	}
	var frozenID, sourceHash string
	err := tx.QueryRowContext(ctx, `SELECT id::text,source_order_sha256 FROM cut_sop2_target_versions_v46 WHERE tenant_id=$1 AND intake_id=$2 ORDER BY version DESC LIMIT 1`, tenant, intake).Scan(&frozenID, &sourceHash)
	if errors.Is(err, sql.ErrNoRows) {
		return buyer, buyerTotal, false, nil
	}
	if err != nil {
		return nil, 0, false, err
	}
	var currentHash string
	if err = tx.QueryRowContext(ctx, `SELECT source_sha256 FROM order_intakes_v5 WHERE tenant_id=$1 AND id=$2`, tenant, intake).Scan(&currentHash); err != nil {
		return nil, 0, false, err
	}
	if currentHash != sourceHash {
		return nil, 0, false, errors.New("FROZEN_TARGET_STALE_SOURCE_ORDER_CHANGE_CONTROL_REQUIRED")
	}
	rows, err := tx.QueryContext(ctx, `SELECT color_code,size_code,ordered_qty,frozen_target FROM cut_sop2_target_skus_v46 WHERE tenant_id=$1 AND target_id=$2::uuid`, tenant, frozenID)
	if err != nil {
		return nil, 0, false, err
	}
	allowed := map[string]int64{}
	var total int64
	for rows.Next() {
		var c, z string
		var ordered, target int64
		if err = rows.Scan(&c, &z, &ordered, &target); err != nil {
			break
		}
		k := c + "\x00" + z
		if buyer[k] != ordered || target < ordered || target < 1 {
			err = errors.New("FROZEN_TARGET_SKU_SOURCE_DIVERGENCE")
			break
		}
		allowed[k] = target
		total += target
	}
	if x := rows.Err(); err == nil {
		err = x
	}
	rows.Close()
	if err != nil {
		return nil, 0, false, err
	}
	if len(allowed) != len(buyer) || total < buyerTotal {
		return nil, 0, false, errors.New("FROZEN_TARGET_INCOMPLETE")
	}
	return allowed, total, true, nil
}
