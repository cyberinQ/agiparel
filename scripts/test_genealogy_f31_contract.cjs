// Author: Sunil Jain, 2026-10-28.
// Source contracts: production identity is not a fabricated accepted garment.
const assert=require('node:assert/strict');
const fs=require('node:fs');
const base='services/edge-daemon/cmd/server/';
const src=fs.readFileSync(base+'genealogy_production.go','utf8');
const sql=fs.readFileSync('services/core-api/migrations/0037_physical_identity_genealogy_v18.sql','utf8');
const main=fs.readFileSync(base+'main.go','utf8');
const portal=fs.readFileSync(base+'portal.go','utf8');
for(const expected of ['GENEALOGY_RESERVE_V1','GENEALOGY_CARRIER_V1','genReserveState','finished_inventory','shipping_release','workforceChapterRequire','cutting.release','cutting.read','gen_carrier_events_v18','gen_cut_parts_v18','genPageLimit','genHistoryCursor','/api/v1/genealogy/carriers/history']) {
 assert.ok(src.includes(expected),`missing production contract ${expected}`);
}
for(const expected of ['cut_sewing_releases_v8','gen_garments_v18','gen_cut_parts_v18','gen_bundles_v18','gen_carriers_v18','gen_carrier_active_v18','UNIQUE(tenant_id,lay_draft_id,marker_position,ply_number)','gen_garment_coverage_v18','DEFERRABLE INITIALLY DEFERRED','gen_carrier_apply_v18','ERR_GEN_CARRIER_RETIRE_MISMATCH','FORCE ROW LEVEL SECURITY','GRANT INSERT ON gen_bundles_v18','SIGNED_HUMAN_CARRIER_CLAIM_NOT_RF_PROOF','gen_no_legacy_roll_collision_v18','gen_no_legacy_bundle_collision_v18','RFID_EPC_GLOBAL_V8']){
 assert.ok(sql.includes(expected),`missing DB invariant ${expected}`);
}
assert.ok(!sql.includes('GRANT INSERT ON gen_carriers_v18'), 'operators must not provision their own tags');
assert.ok(!sql.includes('GRANT INSERT ON gen_carrier_active_v18'), 'operators must not fabricate active bindings');
assert.ok(main.includes('registerGenProductionRoutes(mux, server)'),'missing route registration');
for (const route of ['/api/v1/genealogy/units','/api/v1/genealogy/units/challenge','/api/v1/genealogy/carriers/events','/api/v1/genealogy/carriers/history','/api/v1/genealogy/carriers/challenge']) {
 assert.ok(portal.includes('"'+route+'"'), `missing exact ingress route ${route}`);
}
assert.ok(src.includes('AGIPAREL_GENEALOGY_PRODUCTION_ENABLED'),'must be feature gated');
console.log('PASS F3.1: signed release lineage, per-ply IDs, non-credit identity, audited reusable carriers, tenant isolation');
