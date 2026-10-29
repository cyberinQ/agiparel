// Author: Sunil Jain, 2026-10-28. F3.4 structural release gates.
const fs=require('./legacy_contract_source.cjs'),assert=require('assert');
const sql=fs.readFileSync('services/core-api/migrations/0040_garment_quality_specs_and_release_v21.sql','utf8');
const go=fs.readFileSync('services/edge-daemon/cmd/server/garment_quality_production.go','utf8');
const ingress=fs.readFileSync('services/edge-daemon/cmd/server/portal.go','utf8');
const policy=fs.readFileSync('services/edge-daemon/cmd/server/workforce_chapters.go','utf8');
for(const x of ['FORCE ROW LEVEL SECURITY','qc_garment_specs_v21','qc_garment_measure_rules_v21','qc_garment_defects_v21','qc_garment_events_v21','guard_qc_garment_event_v21','qc_garment_parts_ready_v21','cut_recut_links_v19','cut_part_outcomes_v19','wip_movements_v20','document_sources_v65','quality.finish','quality.release','pg_advisory_xact_lock','ERR_F34_MEASUREMENT_OUT_OF_TOLERANCE','ERR_F34_REPAIR_SEQUENCE','QUALITY_RELEASE'])assert(sql.includes(x),`SQL missing ${x}`);
for(const x of ['workforceChapterRequire','garmentQCChallenge','sewProdStrictJSON','"quality.inspect"','"quality.repair"','"quality.finish"','"quality.release"','quality_released','shipping_release','false','AGIPAREL_GARMENT_QUALITY_ENABLED','quality.read'])assert(go.includes(x),`Go missing ${x}`);
for(const x of ['/api/v1/quality/garments/challenge','/api/v1/quality/garments/events','/api/v1/quality/garments/history','/api/v1/quality/garments/specification'])assert(go.includes(x)&&ingress.includes(x),`route missing ${x}`);
assert(policy.includes('"quality.finish"')&&policy.includes('"quality.release"'));
assert(!/"shipping_release":\s*true/.test(go));
console.log('PASS F3.4: governed source-backed QC, per-garment state, independent signers, release not shipment');
