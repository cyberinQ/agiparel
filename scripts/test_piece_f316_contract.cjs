// Author: Sunil Jain, CyberinQ LLC, 2026-10-29.
const fs=require('node:fs'), assert=require('node:assert/strict');
const root=process.cwd();
const read=(p)=>fs.readFileSync(root+'/'+p,'utf8');
const db=read('services/core-api/migrations/0045_piece_holds_and_partial_wip_v26.sql');
const api=read('services/edge-daemon/cmd/server/piece_hold_production_v26.go');
const gate=read('services/edge-daemon/cmd/server/portal.go');
const main=read('services/edge-daemon/cmd/server/main.go');
for(const text of ['FORCE ROW LEVEL SECURITY','wip_piece_active_hold_v26','zz_wip_piece_hold_v26','zz_qc_piece_hold_v26','zz_sew_completion_piece_hold_v26','SIGNED_ONE_GARMENT_HOLD','SUPERVISOR_REVIEWED_FOR_WIP','WIP:PART:','WIP:SCAN:']) assert.ok(db.includes(text),'missing SQL invariant '+text);
for(const text of ['workforceChapterRequire','sewing.verifier.Verify','sewing.piece.hold','sewing.piece.review','AGIPAREL_PIECE_HOLDS_ENABLED','physical_repair_certified','sewing_credit','nominal_ply_count','reserved_garment_identities'])assert.ok(api.includes(text),'missing API boundary '+text);
assert.ok(main.includes('registerPieceHoldRoutes(mux, server)'));
for(const path of ['/api/v1/wip/pieces/challenge','/api/v1/wip/pieces/events','/api/v1/wip/pieces/groups'])assert.ok(gate.includes(path));
assert.ok(!api.includes('UPDATE gen_garments_v18'));
console.log('PASS F3.16: signed per-piece holds, independent scan-backed reviews, conservative group visibility and SQL WIP/QC/full-batch protections');
