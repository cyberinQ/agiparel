// Author: Sunil Jain, 2026-11-11 — Verify native SOP-2 attachment without weakening existing release authority.
'use strict';
const fs=require('fs'),assert=require('assert');
const file=p=>fs.readFileSync(p,'utf8');
const main=file('services/edge-daemon/cmd/server/main.go');
const api=file('services/edge-daemon/cmd/server/cutting_sop2_api.go');
const bridge=file('services/edge-daemon/cmd/server/cutting_sop2_bridge.go');
const draft=file('services/edge-daemon/cmd/server/cutting_lay_drafts.go');
const release=file('services/edge-daemon/cmd/server/cutting_release.go');
const spread=file('services/edge-daemon/cmd/server/cutting_spread_api.go');
const sql=file('services/core-api/migrations/0005_cut_sop2_control_plane_v46.sql');
const html=file('services/edge-daemon/internal/web/static/cutting_sop2.html');
const js=file('services/edge-daemon/internal/web/static/cutting_sop2.js');
assert(main.includes('registerCuttingSOP2Routes(mux, server)'));
for (const path of ['/targets','/control-tower','/solve','/allocate-rolls','/schedule','/family-audit','/reconcile','/inspections'])
  assert(api.includes(`/api/v1/cutting/sop2${path}`),`missing API ${path}`);
for (const required of ['sop2WriteEnabled','intakeTenant()','s.docTransaction(ctx, tenant)','pg_advisory_xact_lock','ValidateFrozenTargets','SIGNED_RELEASE_DISTINGUISHED_FROM_DRAFT_AND_REPORTED'])
  assert(api.includes(required),`missing safe authority ${required}`);
assert(bridge.includes('resolveSOP2CutLimits')&&bridge.includes('FROZEN_TARGET_STALE_SOURCE_ORDER_CHANGE_CONTROL_REQUIRED'));
assert(draft.includes('resolveSOP2CutLimits(ctx, tx, tenant, v.IntakeID, skuAllowed, totalQty)'));
assert(draft.includes('checkCutQuantities(skuAllowed, previous, proposed)'));
assert(draft.includes('checkCutTotal(totalQty, previous, proposed)'));
assert(spread.includes('ERR_CUT_SPREAD_PLY_MISMATCH_REVISE_DRAFT'));
for (const signedControl of ['cut_reader_events_v8','cut_bundle_scan_witnesses_v8','cut_sewing_releases_v8','st.Ready = len(st.Reasons) == 0'])
  assert(release.includes(signedControl),`release authority was weakened: ${signedControl}`);
for (const table of ['cut_sop2_target_versions_v46','cut_sop2_target_skus_v46','cut_sop2_roll_inspections_v46','cut_sop2_inspection_policies_v46','cut_sop2_change_requests_v46','cut_sop2_change_approvals_v46','cut_sop2_schedule_versions_v46','cut_sop2_nfc_bindings_v46','cut_sop2_marriage_edges_v46','cut_sop2_material_reconciliations_v46'])
  assert(sql.includes(`CREATE TABLE IF NOT EXISTS public.${table}`),`table missing: ${table}`);
assert(sql.includes('FORCE ROW LEVEL SECURITY')&&sql.includes("current_setting(''app.current_tenant_id''"));
assert(html.includes('/static/cutting_sop2.js')&&html.includes('Signed released'));
assert(js.includes('textContent')&&!js.includes('innerHTML'));
assert(!api.includes('INSERT INTO cut_sewing_releases_v8')&&!api.includes('INSERT INTO cut_reader_events_v8'));
console.log('PASS: SOP-2 routes, tenant isolation, quantity bridge, immutable schema, evidence controls, safe UI');
