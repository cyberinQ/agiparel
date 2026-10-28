// Author: Sunil Jain, 2026-10-28.
// Verify Chapter 09 doesn't confuse rehearsal evidence, production groups and finished garments.
'use strict';
const fs=require('node:fs');
const assert=require('node:assert/strict');
const base='services/edge-daemon/';
const api=fs.readFileSync(base+'cmd/server/intelligence_production.go','utf8');
const page=fs.readFileSync(base+'internal/web/static/factory_intelligence_production.html','utf8');
const js=fs.readFileSync(base+'internal/web/static/factory_intelligence_production.js','utf8');
const migration=fs.readFileSync('services/core-api/migrations/0032_intelligence_production_readers_v13.sql','utf8');
for(const required of ['intelligence.read','factory_manager','AGIPAREL_INTELLIGENCE_PRODUCTION_ENABLED','transaction_timestamp()','sql.LevelRepeatableRead','qc_prod_group_events_v11','pack_prod_holds_v12','NOT_MEASURED','"production_release"','intelligence_prod_readers_v13']) {
 // the transaction isolation is provided by the existing intelTx helper.
 const source=required==='sql.LevelRepeatableRead'?fs.readFileSync(base+'cmd/server/factory_intelligence.go','utf8'):api;
 assert(source.includes(required),required);
}
assert(api.includes('WHERE c.tenant_id=$1 AND c.plant_id=$2 AND c.created_at >= $3 AND c.created_at < $4'));
assert(!api.includes('sew_sim_credits_v9'));
assert(migration.includes('FORCE ROW LEVEL SECURITY'));
assert(migration.includes('GRANT SELECT') && !migration.includes('GRANT INSERT'));
assert(page.includes('factory_intelligence.css'));
assert(js.includes('NOT_MEASURED') && js.includes("credentials:'omit'"));
assert(!js.includes('localStorage') && !js.includes('innerHTML'));
console.log('PASS Chapter 09: scoped authenticated group evidence; unreleased garments and finance unmeasured');
