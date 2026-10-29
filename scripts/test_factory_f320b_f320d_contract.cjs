// Author: Sunil Jain, CyberinQ LLC, 2026-10-29.
// F3.20B-D fail-closed schema, route, proof and authority regression contract.
'use strict';
const assert=require('node:assert/strict');
const fs=require('node:fs');
const read=p=>fs.readFileSync(p,'utf8');
const sql=read('services/core-api/migrations/0053_dock_scans_custody_asn_drafts_v34.sql');
const api=read('services/edge-daemon/cmd/server/dock_handover_v34.go');
const portal=read('services/edge-daemon/cmd/server/portal.go');
const grants=read('services/edge-daemon/cmd/server/workforce_chapters.go');
const main=read('services/edge-daemon/cmd/server/main.go');
const compose=read('compose/docker-compose.production.yml');
for(const key of ['dock_gateways_v34','dock_carrier_authorizations_v34','dock_station_policies_v34',
'dock_carton_scans_v34','dock_container_seals_v34','dock_carrier_receipts_v34','dock_asn_drafts_v34',
'dock_manifest_safe_v34','guard_dock_scan_v34','guard_dock_seal_v34','guard_dock_carrier_v34',
'guard_dock_asn_v34','guard_dock_proof_v34','CREATE OR REPLACE FUNCTION guard_factory_proof_v32',
'CREATE OR REPLACE FUNCTION guard_factory_review_proof_v33','FORCE ROW LEVEL SECURITY',
'protect_cut_draft_v8','max_scan_age_minutes','garment_needle_safety_blocked_v32']) {
  assert(sql.includes(key),`Missing SQL boundary: ${key}`);
}
for(const key of ['ed25519.Verify','dockScanBytesV34','dockScanValidV34','workforceChapterRequire',
'packing.read','commissionTx','AGIPAREL_DOCK_V34_ENABLED','AGIPAREL_FACTORY_RELEASE_V33_ENABLED',
'dockCommandPolicyV34','dockPathValidV34','dockChallengeV34','SET LOCAL ROLE agiparel_app',
'"shipment_credit": false','"asn_transmitted": false','"external_carrier_api_verified": false']) {
  assert(api.includes(key),`Missing Go boundary: ${key}`);
}
for(const p of ['/api/v1/packing/dock/scans','/api/v1/packing/dock/verification',
'/api/v1/packing/dock/seals','/api/v1/packing/dock/seals/challenge',
'/api/v1/packing/dock/carrier-receipts','/api/v1/packing/dock/carrier-receipts/challenge',
'/api/v1/packing/dock/asn-drafts','/api/v1/packing/dock/asn-drafts/challenge']) {
 assert(portal.includes('"'+p+'"'),`Unsafe or missing ingress: ${p}`);
 assert(api.includes('"'+p+'"'),`Missing handler: ${p}`);
}
for(const permission of ['packing.dock.seal','packing.dock.carrier','packing.dock.asn.prepare']){
 assert(sql.includes("'"+permission+"'"));assert(grants.includes('"'+permission+'"'));
}
assert(main.includes('registerDockRoutesV34(mux, server)'));
for(const flag of ['AGIPAREL_FACTORY_EVIDENCE_V32_ENABLED','AGIPAREL_FACTORY_RELEASE_V33_ENABLED','AGIPAREL_DOCK_V34_ENABLED']) {
 assert(compose.includes(flag+': ${'+flag+':-false}'),`Unsafe production default: ${flag}`);
}
assert(!api.includes('"shipment_credit": true'));
assert(!api.includes('"asn_transmitted": true'));
console.log('PASS F3.20B-D contract: dock gateway, independent custody, ASN draft, proof lock, current QC/AQL and deny shipment authority');
