// Author: Sunil Jain, 2026-10-28.
// Static, fast contract checks for real production sewing boundary and migration.
'use strict';
const fs = require('./legacy_contract_source.cjs');
const assert = require('assert');
const src = file => fs.readFileSync(file,'utf8');
const base = 'services/edge-daemon/cmd/server/';
const prod = src(base+'sewing_production.go');
const gate = src(base+'portal.go');
const main = src(base+'main.go');
const sql = src('services/core-api/migrations/0029_sewing_production_evidence_v10.sql');
const sim = src(base+'sewing_simulation.go');
assert(prod.includes('AGIPAREL_SEWING_PRODUCTION_ENABLED'));
assert(prod.includes('AGIPAREL_IDP_ED25519_PUBLIC_KEY_B64'));
assert(prod.includes('sewProdSigned(v, req.SignatureHex, key)'));
assert(prod.includes('p.verifier.Verify(token, "sewing.complete", sewProdChallenge(c), "sewing_operator", true)'));
assert(prod.includes('ERR_SEW_PROD_SIGNED_CUT_EVIDENCE_REQUIRED'));
assert(prod.includes('ERR_SEW_PROD_APPROVED_ROUTE_AND_RELEASE_REQUIRED'));
assert(main.includes('registerSewProdRoutes(mux, server)'));
for (const path of ['/api/v1/sewing/production/observations','/api/v1/sewing/production/completions']) {
  assert(gate.includes(path), 'independent identity ingress missing');
}
for(const table of ['sew_prod_gateways_v10','sew_prod_routes_v10','sew_prod_worker_stations_v10','sew_prod_observations_v10','sew_prod_completions_v10','sew_prod_used_observations_v10']) {
  assert(sql.includes('CREATE TABLE '+table));
}
assert(sql.includes('FORCE ROW LEVEL SECURITY'));
assert(sql.includes('ERR_SEW_PROD_OBSERVATION_ALREADY_CONSUMED'));
assert(sql.includes('PRIMARY KEY(tenant_id,observation_id)'));
assert(sql.includes('ERR_SEW_PROD_WORKER_NOT_ASSIGNED'));
assert(sql.includes('GRANT INSERT ON sew_prod_observations_v10,sew_prod_completions_v10,sew_prod_used_observations_v10 TO agiparel_app'));
assert(!/GRANT INSERT ON sew_prod_gateways_v10|GRANT INSERT ON sew_prod_routes_v10/.test(sql));
assert(sim.includes('UI_SIMULATED_NO_RF_OBSERVATION'));
assert(sim.includes('ERR_SIM_ONLY_DEVELOPMENT'));
console.log('PASS: production-only sewing signed reader, scoped IdP, published route, database lineage and replay boundaries');
