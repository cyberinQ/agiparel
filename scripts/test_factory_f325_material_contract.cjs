// Author: Sunil Jain, 2026-10-29. Static fail-closed material/operator-contract regression.
const fs = require('node:fs');
const assert = require('node:assert/strict');
const sql = fs.readFileSync('services/core-api/migrations/0057_material_attribution_integrity_v38.sql','utf8');
const go = fs.readFileSync('services/edge-daemon/cmd/server/dock_external_v35.go','utf8');
const portal = fs.readFileSync('services/edge-daemon/cmd/server/portal.go','utf8');
for (const token of ['dock_material_trace_v38','dock_integrity_report_v38','cut_spread_issue_allocations_v8','material_issues_v5','store_rolls','cut_consumption_reconciliations_v19','dock_lineage_audit_v37','5000','material_attribution_complete',"'shipment_credit',false", "'global_fabric_stock_conserved',false", "'financial_ledger_conserved',false"]) assert.ok(sql.includes(token), token);
assert.doesNotMatch(sql,/SECURITY DEFINER/i);
assert.match(sql,/REVOKE ALL ON FUNCTION dock_material_trace_v38/);
assert.ok(go.includes('AGIPAREL_DOCK_INTEGRITY_V38_ENABLED'));
assert.ok(go.includes('SELECT dock_integrity_report_v38($1,$2,$3::uuid)'));
assert.ok(go.includes('workforceChapterPermit(ctx, tx, actor, "packing.read")'));
assert.ok(portal.includes('/api/v1/packing/dock/integrity-report'));
console.log('PASS F3.25 v38: bounded material attribution, tenant-scoped operator integrity report and no shipment credit');
