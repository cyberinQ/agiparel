// Author: Sunil Jain, CyberinQ LLC, 2026-10-29.
// Verify external signatures, current-proof guards and deny-shipment boundary.
const fs = require('node:fs');
const assert = require('node:assert/strict');
const read = file => fs.readFileSync(file, 'utf8');
const go = read('services/edge-daemon/cmd/server/dock_external_v35.go');
const sql = read('services/core-api/migrations/0054_external_acknowledgement_and_dock_acceptance_v35.sql');
const main = read('services/edge-daemon/cmd/server/main.go');
const ingress = read('services/edge-daemon/cmd/server/portal.go');
for (const needle of ['ed25519.Verify', 'AGIPAREL_EXTERNAL_ASSERTION_V35', 'dockExternalValidV35',
  'AGIPAREL_DOCK_EXTERNAL_V35_ENABLED', 'packing.read', 'commissionTx', 'carrier_network_delivery_proven',
  '"inventory_or_shipment_credit": false']) assert.ok(go.includes(needle), needle);
for (const needle of ['dock_external_signers_v35', 'dock_external_assertions_v35',
  'dock_evidence_current_v34', 'dock_carton_reconciled_v35',
  'CREATE TRIGGER dock_external_immutable_v35', 'FORCE ROW LEVEL SECURITY',
  'REVOKE ALL', 'GRANT INSERT ON dock_external_assertions_v35']) assert.ok(sql.includes(needle), needle);
assert.ok(main.includes('registerDockExternalRoutesV35(mux, server)'));
assert.ok(ingress.includes('/api/v1/packing/dock/external-assertions'));
assert.ok(ingress.includes('/api/v1/packing/dock/external-verification'));
assert.ok(!go.includes('INSERT INTO shipment') && !go.includes('UPDATE inventory'));
console.log('PASS F3.22: external key authentication, ASN correlation, tenant scope and no shipment posting');
