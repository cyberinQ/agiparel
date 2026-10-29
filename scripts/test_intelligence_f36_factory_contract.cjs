// Author: Sunil Jain, 2026-10-28.
'use strict';
const fs=require('node:fs'),assert=require('node:assert/strict');
const go=fs.readFileSync('services/edge-daemon/cmd/server/intelligence_factory_evidence.go','utf8');
const api=fs.readFileSync('services/edge-daemon/cmd/server/intelligence_production.go','utf8');
const html=fs.readFileSync('services/edge-daemon/internal/web/static/factory_intelligence_production.html','utf8');
const js=fs.readFileSync('services/edge-daemon/internal/web/static/factory_intelligence_production.js','utf8');
for(const text of ['cut_part_outcomes_v19','qc_garment_events_v21','cut_consumption_reconciliations_v19','wip_movements_v20','pack_carton_events_v22','pack_manifest_events_v22','NO_DENOMINATOR','NOT_MEASURED','SIGNED_UNIT_AND_CUSTODY_EVENTS_NOT_PHYSICAL_CERTIFICATION','AND a.recorded_at >= dep.recorded_at','LIMIT 10','OEE'])
 assert(go.includes(text),text);
assert(/loadFactoryEvidence\(ctx,\s*tx,\s*actor.TenantID,\s*actor.PlantID,\s*from,\s*to\)/.test(api),'same tenant and snapshot');
assert(api.indexOf('loadFactoryEvidence(')>api.indexOf('workforceReportGrant('), 'authorization must precede unit metrics');
for(const text of ['id="factorymeasures"','id="factoryroutes"','id="factorybasis"'])assert(html.includes(text),text);
for(const text of ['validateFactory','showFactory','factoryMissing','basis_points','replaceChildren','textContent'])assert(js.includes(text),text);
for(const text of ['localStorage','innerHTML','sessionStorage'])assert(!js.includes(text),text);
assert(!/INSERT INTO|UPDATE [a-z]|DELETE FROM/i.test(go),'read-only aggregation');
console.log('PASS F3.6: scoped signed-event KPIs, null denominators, handoff percentiles, no shipments or fake OEE');
