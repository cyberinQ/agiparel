// Author: Sunil Jain, CyberinQ LLC, 2026-10-29.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const root = path.resolve(__dirname, '..');
const read = file => fs.readFileSync(path.join(root,file),'utf8');
const migration=read('services/core-api/migrations/0051_calibration_aql_needle_evidence_v32.sql');
const server=read('services/edge-daemon/cmd/server/factory_evidence_v32.go');
const ingress=read('services/edge-daemon/cmd/server/portal.go');
const permissions=read('services/edge-daemon/cmd/server/workforce_chapters.go');
for(const t of ['factory_instrument_calibrations_v32','fabric_calibration_links_v32','buyer_aql_plans_v32','buyer_aql_sample_events_v32','needle_safety_events_v32']) {
 assert.match(migration,new RegExp(`CREATE TABLE ${t} \\(`));
 assert.match(migration,new RegExp(`'${t}'`));
}
for(const action of ['stores.fabric.calibration.link','quality.aql.inspect','quality.needle.report']) {
 assert.ok(migration.includes(`'${action}'`));
 assert.ok(permissions.includes(`"${action}"`));
}
for(const route of ['/api/v1/factory/evidence/challenge','/api/v1/factory/evidence/events','/api/v1/quality/aql/sampling','/api/v1/quality/needle/incidents']) {
 assert.ok(server.includes(`"${route}"`));assert.ok(ingress.includes(`"${route}"`));
}
assert.ok(migration.includes('FORCE ROW LEVEL SECURITY'));
assert.ok(migration.includes('GRANT INSERT ON fabric_calibration_links_v32,buyer_aql_sample_events_v32,needle_safety_events_v32'));
assert.ok(!migration.includes('GRANT INSERT ON factory_instrument_calibrations_v32'));
assert.ok(!migration.includes('GRANT INSERT ON buyer_aql_plans_v32'));
assert.match(server,/buyer_lot_acceptance[^\n]*false/);
assert.match(server,/"safety_clearance": false/);
console.log('PASS F3.18C/F3.19A/F3.19B: signed calibration links, buyer-selected AQL samples, human safety reports; no implicit approvals');
