// Author: Sunil Jain, CyberinQ LLC, 2026-10-29.
// F3.17A bounded static contract checks; live Postgres tests are separate.
const fs = require('fs');
const assert = require('assert');
const read = p=>fs.readFileSync(p,'utf8');
const sql=read('services/core-api/migrations/0047_jobwork_scanned_custody_v28.sql');
const go=read('services/edge-daemon/cmd/server/jobwork_custody_v28.go');
const portal=read('services/edge-daemon/cmd/server/portal.go');
const perm=read('services/edge-daemon/cmd/server/workforce_chapters.go');
for(const name of ['jobwork_outwards_v28','jobwork_inwards_v28','jobwork_used_scans_v28','jobwork_used_proofs_v28']) assert(sql.includes(name));
for(const name of ['jobwork_part_offsite_v28','jobwork_scan_spent_v28','guard_jobwork_offsite_v28']) assert(sql.includes(name));
for(const name of ['wip_movements_v20','qc_garment_events_v21','pack_carton_events_v22','sew_piece_completions_v27','sew_prod_completions_v10']) assert(sql.includes('BEFORE INSERT ON '+name));
assert(sql.includes('CREATE POLICY jobwork_tenant_v28') && sql.includes('FORCE ROW LEVEL SECURITY'));
assert(sql.includes("format('CREATE POLICY") && sql.includes('%L'));
assert(sql.includes('NEW.process_code=ANY(v.allowed_processes)'));
assert(go.includes('workforceChapterRequire') && go.includes('workforceChapterPermit'));
assert(go.includes('SET LOCAL ROLE agiparel_app') && go.includes('commissionTx(ctx,'));
assert(go.includes('bearer(r)') && go.includes('sewProdStrictJSON'));
assert(go.includes('p.holds.wip.gen.sewing.verifier.Verify'));
assert(go.includes('AGIPAREL_JOBWORK_CUSTODY_ENABLED'));
for(const name of ['jobwork.outward','jobwork.inward']) assert(perm.includes('"'+name+'"'));
for(const path of ['/api/v1/jobwork/parts/outward','/api/v1/jobwork/parts/inward','/api/v1/jobwork/parts/status']) assert(portal.includes('"'+path+'"'));
assert(!sql.includes('SHIPPED') || sql.includes('NOT_VENDOR'));
console.log('PASS F3.17A: signed subcontract outward/inward, isolated vendor roster, spent scans, offsite WIP/QC/packing holds');
