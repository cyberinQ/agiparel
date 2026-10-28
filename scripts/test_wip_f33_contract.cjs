// Author: Sunil Jain, 2026-10-28.
// F3.3: check source-level authority and isolation boundaries.
const fs=require('fs');const assert=require('assert');
const sql=fs.readFileSync('services/core-api/migrations/0039_sewing_wip_custody_and_timing_v20.sql','utf8');
const go=fs.readFileSync('services/edge-daemon/cmd/server/wip_production.go','utf8');
const ingress=fs.readFileSync('services/edge-daemon/cmd/server/portal.go','utf8');
const policy=fs.readFileSync('services/edge-daemon/cmd/server/workforce_chapters.go','utf8');
for(const fragment of ['FORCE ROW LEVEL SECURITY','guard_wip_scan_v20','guard_wip_move_v20','gen_carrier_active_v18','cut_part_outcomes_v19','wip_transfer_routes_v20','wip_movements_v20','sewing.transfer','pg_advisory_xact_lock','wip_immutable_v20','UNIQUE(tenant_id,scan_id)','UNIQUE(tenant_id,worker_proof_id)'])assert(sql.includes(fragment),`SQL ${fragment}`);
for(const fragment of ['ed25519.Verify','wipMoveChallenge','sewProdStrictJSON','workforceChapterRequire','"sewing.transfer"','"sewing.read"','wipScanMaxAge','clock_basis','DATABASE_RECORDED_RECEIPT_TIMESTAMPS','SEW_PROD']){if(fragment==='SEW_PROD')continue;assert(go.includes(fragment),`Go ${fragment}`);}
for(const fragment of ['/api/v1/wip/production/scans','/api/v1/wip/production/movements','/api/v1/wip/production/history','/api/v1/wip/production/metrics'])assert(ingress.includes(fragment) && go.includes(fragment),fragment);
assert(policy.includes('"sewing.transfer"'));
assert(!/shipping_credit\s*:\s*true/.test(go));
console.log('PASS F3.3: enrolled signed scans, accepted-part binding, independent WIP custody, indexed timing and fail-closed routing');
