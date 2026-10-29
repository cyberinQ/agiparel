// Author: Sunil Jain, 2026-10-29. Check v37 lineage, authorization and no-credit boundary.
const fs = require('./legacy_contract_source.cjs');
const assert = require('node:assert/strict');
const sql = fs.readFileSync('services/core-api/migrations/0056_dock_cut_wip_lineage_audit_v37.sql', 'utf8');
const go = fs.readFileSync('services/edge-daemon/cmd/server/dock_external_v35.go', 'utf8');
const portal = fs.readFileSync('services/edge-daemon/cmd/server/portal.go', 'utf8');
for (const needle of ['cut_consumption_reconciliations_v19','qc_garment_parts_ready_v21','wip_piece_active_hold_v26','jobwork_part_offsite_v28','jobwork_part_disputed_v29','shipment_credit\',false','global_inventory_conserved\',false','cardinality(notes)=0']) {
  assert.ok(sql.includes(needle), `missing ${needle}`);
}
assert.match(sql,/REVOKE ALL ON FUNCTION dock_lineage_audit_v37\(text,text,uuid\) FROM PUBLIC/);
assert.doesNotMatch(sql,/SECURITY DEFINER/i);
assert.match(go,/workforceChapterPermit\(ctx, tx, actor, "packing.read"\)/);
assert.ok(go.includes('AGIPAREL_DOCK_LINEAGE_AUDIT_V37_ENABLED'));
assert.ok(go.includes('SELECT dock_lineage_audit_v37($1,$2,$3::uuid)'));
assert.ok(portal.includes('/api/v1/packing/dock/lineage-audit'));
console.log('PASS v37: cross-ledger read-only report, tenant authorization, bounded audit and no shipment credit');
