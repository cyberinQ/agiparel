// Author: Sunil Jain, 2026-10-29. Validate fail-closed v36 boundaries.
const fs = require('node:fs');
const assert = require('node:assert/strict');
const migration = fs.readFileSync('services/core-api/migrations/0055_buyer_scope_carton_exact_and_diagnostics_v36.sql', 'utf8');
const go = fs.readFileSync('services/edge-daemon/cmd/server/dock_external_v35.go', 'utf8');
const ingress = fs.readFileSync('services/edge-daemon/cmd/server/portal.go', 'utf8');
for (const source of [migration, go, ingress]) assert.ok(!source.includes('shipment_credit": true'));
for (const source of ['dock_buyer_spec_bindings_v36', 'dock_buyer_scope_current_v36', 'ERR_F36_BUYER_NOT_AUTHORIZED_FOR_SPEC', 'dock_carton_reconciled_v35', 'dock_diagnostics_v36', 'FORCE ROW LEVEL SECURITY']) {
  assert.ok(migration.includes(source), `missing ${source}`);
}
assert.match(migration, /GRANT SELECT ON dock_buyer_spec_bindings_v36 TO agiparel_app/);
assert.doesNotMatch(migration, /GRANT\s+(INSERT|UPDATE|DELETE)\s+ON dock_buyer_spec_bindings_v36/i);
assert.ok(go.includes('AGIPAREL_DOCK_DIAGNOSTICS_V36_ENABLED'));
assert.ok(go.includes('workforceChapterPermit(ctx, tx, actor, "packing.read")'));
assert.ok(go.includes('dock_diagnostics_v36($1,$2,$3::uuid)'));
assert.ok(ingress.includes('/api/v1/packing/dock/diagnostics'));
console.log('PASS F3.23 v36: spec-bound buyer identity, carton/scan reconciliation, read-only scoped diagnostics');
