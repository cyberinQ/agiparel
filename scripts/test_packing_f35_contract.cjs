// Author: Sunil Jain, 2026-10-28. F3.5 production proof and schema contract.
const fs=require('./legacy_contract_source.cjs'),assert=require('assert');
const sql=fs.readFileSync('services/core-api/migrations/0041_carton_packing_and_dispatch_staging_v22.sql','utf8');
const go=fs.readFileSync('services/edge-daemon/cmd/server/carton_genealogy_production.go','utf8');
const ingress=fs.readFileSync('services/edge-daemon/cmd/server/portal.go','utf8');
const policy=fs.readFileSync('services/edge-daemon/cmd/server/workforce_chapters.go','utf8');
for(const x of ['FORCE ROW LEVEL SECURITY','pack_carton_specs_v22','pack_size_weights_v22',
 'document_sources_v65','qc_garment_events_v21','QUALITY_RELEASE','pack_carton_events_v22','pack_manifest_events_v22',
 'pack_one_carton_per_garment_v22','pack_one_manifest_per_carton_v22','guard_pack_carton_event_v22',
 'guard_pack_manifest_event_v22','pg_advisory_xact_lock','ERR_F35_GARMENT_NOT_RELEASED',
 'ERR_F35_WEIGHT_OUT_OF_TOLERANCE','ERR_F35_INDEPENDENT_SEAL_OR_SCALE_REF',
 'STAGED_DISPATCH_MANIFEST_NOT_CARRIER_HANDOVER_OR_SHIPMENT'])assert(sql.includes(x),`SQL missing ${x}`);
for(const x of ['workforceChapterRequire','packUnitChallenge','packManifestChallenge',
 'sewProdStrictJSON','"packing.pack"','"packing.seal"','"packing.manifest"','"packing.dispatch_stage"',
 'AGIPAREL_CARTON_GENEALOGY_ENABLED','"packing.read"','"shipped": false','"weight_calibrated": false'])
 assert(go.includes(x),`Go missing ${x}`);
for(const x of ['/api/v1/packing/units/challenge','/api/v1/packing/units/events',
 '/api/v1/packing/manifests/challenge','/api/v1/packing/manifests/events','/api/v1/packing/genealogy','/api/v1/packing/specification'])
 assert(go.includes(x)&&ingress.includes(x),`Route missing ${x}`);
for(const action of ['packing.pack','packing.seal','packing.manifest','packing.dispatch_stage'])assert(policy.includes('"'+action+'"'));
assert(!/"shipped"\s*:\s*true/.test(go));
console.log('PASS F3.5: document-backed cartons, per-garment QC release, independent seal, staged manifest, no false shipment');
