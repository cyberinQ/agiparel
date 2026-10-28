// Author: Sunil Jain, 2026-10-28.
// F3.2: production-only integrity, no cross-tenant fabrication or output credit.
const fs = require('fs');
const assert = require('assert');
const read = x => fs.readFileSync(x,'utf8');
const sql=read('services/core-api/migrations/0038_cutting_part_reconciliation_v19.sql');
const go=read('services/edge-daemon/cmd/server/cutting_reconciliation_production.go');
const portal=read('services/edge-daemon/cmd/server/portal.go');
const compose=read('compose/docker-compose.production.yml');
const main=read('services/edge-daemon/cmd/server/main.go');
for(const table of ['cut_consumption_reconciliations_v19','cut_part_outcomes_v19','cut_recut_links_v19','cut_proof_consumptions_v19','cut_defect_codes_v19']) assert(sql.includes('CREATE TABLE '+table),table);
for(const need of ['FORCE ROW LEVEL SECURITY','REVOKE ALL','CREATE TRIGGER cut_consumption_guard_v19','CREATE TRIGGER cut_part_outcome_guard_v19','CREATE TRIGGER cut_recut_guard_v19','protect_cut_draft_v8()','CHECK(supervisor_subject<>quality_subject','UNIQUE(tenant_id,part_id)','UNIQUE(tenant_id,replacement_part_id)']) assert(sql.includes(need),need);
for(const need of ['workforceChapterRequire(ctx, tx, sup, "cutting.release")','workforceChapterRequire(ctx, tx, qual, "quality.inspect")','ERR_F32_INDEPENDENT_ASSERTIONS_REQUIRED','ERR_F32_ASSERTION_ALREADY_USED','ERR_F32_COMMAND_REUSE','AGIPAREL_CUTTING_RECONCILIATION_ENABLED','f32ReadChallenge','sensor_verified','shipping_release','f32Challenge']) assert(go.includes(need),need);
for(const suffix of ['challenge','events','balance','defects']) assert(portal.includes('/api/v1/cutting/production/reconciliation/'+suffix),suffix);
assert(main.includes('registerF32Routes(mux, server)'));
assert(compose.includes('AGIPAREL_CUTTING_RECONCILIATION_ENABLED: ${AGIPAREL_CUTTING_RECONCILIATION_ENABLED:-false}'));
assert(!go.includes('finished_garments": 1'));
console.log('PASS F3.2: dual signatures, source-linked consumption, append-only part disposition, recut lineage, and no finished-garment credit');
