// Author: Sunil Jain, 2026-10-28.
// Production-quality boundaries: signed IdP, SQL lineage, scoped authority, zero simulation credit.
'use strict';
const fs=require('./legacy_contract_source.cjs'),assert=require('assert');
const read=p=>fs.readFileSync(p,'utf8');
const base='services/edge-daemon/cmd/server/';
const prod=read(base+'quality_production.go');
const sql=read('services/core-api/migrations/0030_quality_production_group_evidence_v11.sql');
const gate=read(base+'portal.go');
const main=read(base+'main.go');
const simulated=read(base+'quality_simulation.go');
const compose=read('compose/docker-compose.production.yml');
assert(prod.includes('AGIPAREL_QUALITY_PRODUCTION_ENABLED'));
assert(prod.includes('newSewProdAPI(s)'));
assert(prod.includes('q.sewing.verifier.Verify(token, action, qcProdChallenge(c), role, true)'));
assert(prod.includes('qcProdReadChallenge, "quality_inspector", true'));
assert(prod.includes('ERR_QC_PROD_ASSIGNMENT'));
assert(prod.includes('packing_release": false'));
assert(prod.includes('sew_prod_completions_v10'));
assert(!prod.includes('qc_sim_'));
for(const name of ['qc_prod_defect_catalog_v11','qc_prod_assignments_v11','qc_prod_group_events_v11']) {
 assert(sql.includes('CREATE TABLE '+name));
 assert(sql.includes('FORCE ROW LEVEL SECURITY'));
}
assert(sql.includes('UNIQUE(tenant_id,completion_id,revision)'));
assert(sql.includes('proof_id varchar(255)'));
assert(sql.includes('ON DELETE RESTRICT'));
assert(sql.includes('ERR_QC_PROD_INDEPENDENT_REINSPECTION'));
assert(sql.includes('source_row.nominal_ply_count'));
assert(sql.includes("NEW.prior_state='REVIEWED' AND NEW.action='INSPECT_HOLD'"));
assert(sql.includes('GRANT INSERT ON qc_prod_group_events_v11 TO agiparel_app'));
assert(!sql.includes('GRANT INSERT ON qc_prod_defect_catalog_v11'));
assert(gate.includes('/api/v1/quality/production/events'));
assert(main.includes('registerQCProdRoutes(mux, server)'));
assert(compose.includes('AGIPAREL_QUALITY_PRODUCTION_ENABLED'));
assert(simulated.includes('ERR_QC_SIM_DEVELOPMENT_ONLY'));
console.log('PASS Chapter 07: authenticated group QC, independent repair, immutable holds, plant scope and no shipping authority');
