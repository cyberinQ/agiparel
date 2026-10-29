// Author: Sunil Jain, CyberinQ LLC, 2026-10-29.
const fs=require('./legacy_contract_source.cjs'),assert=require('assert');
const read=p=>fs.readFileSync(p,'utf8');
const sql=read('services/core-api/migrations/0048_jobwork_discrepancy_reconciliation_v29.sql');
const api=read('services/edge-daemon/cmd/server/jobwork_reconciliation_v29.go');
const routes=read('services/edge-daemon/cmd/server/main.go');
const portal=read('services/edge-daemon/cmd/server/portal.go');
const permissions=read('services/edge-daemon/cmd/server/workforce_chapters.go');
for(const s of ['jobwork_discrepancies_v29','jobwork_part_disputed_v29','guard_jobwork_discrepancy_v29',
  'guard_jobwork_disputed_progress_v29','MISSING_AT_RECONCILIATION','DAMAGED_ON_RETURN',
  'FORCE ROW LEVEL SECURITY',"current_setting('app.current_tenant_id',true)",
  'CREATE TRIGGER jobwork_discrepancy_immutable_v29'])assert(sql.includes(s),`missing SQL ${s}`);
for(const table of ['jobwork_outwards_v28','wip_movements_v20','sew_prod_completions_v10',
  'sew_piece_completions_v27','qc_garment_events_v21','pack_carton_events_v22']){
  assert(sql.includes('BEFORE INSERT ON '+table),`missing hold gate on ${table}`);
}
assert(sql.includes('jobwork_inwards_v28') && sql.includes("o.sender_subject=NEW.reporter_subject"));
assert(sql.includes('ALTER FUNCTION jobwork_part_offsite_v28(text,uuid) VOLATILE'));
assert(sql.includes('ALTER FUNCTION jobwork_scan_spent_v28(text,uuid) VOLATILE'));
assert(sql.includes('LANGUAGE sql VOLATILE AS'));
assert(api.includes('COUNT(DISTINCT o.part_id)') && api.includes('repeated_part_outward_count'));
assert(sql.includes('jobwork_used_proofs_v28'));
for(const s of ['workforceChapterRequire','workforceChapterPermit','commissionTx(ctx,',
  'p.holds.wip.gen.sewing.verifier.Verify','SET LOCAL ROLE agiparel_app',
  'LIVE_LEDGER_RECONCILIATION_NOT_SIGNED_GATE_PASS_OR_VENDOR_ACK',
  'physical_loss_confirmed','replayed'])assert(api.includes(s),`missing API control ${s}`);
for(const s of ['/api/v1/jobwork/reconciliation','/api/v1/jobwork/discrepancies/challenge',
  '/api/v1/jobwork/discrepancies'])assert(portal.includes('"'+s+'"'));
assert(routes.includes('registerJobworkReconciliationRoutes(mux, server)'));
assert(permissions.includes('"jobwork.discrepancy.report"'));
console.log('PASS F3.17B: signed human-only discrepancy, immutable ledger, open vendor reconciliation, blocked disputed WIP/QC/packing');
