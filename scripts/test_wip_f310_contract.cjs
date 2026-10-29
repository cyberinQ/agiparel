// Author: Sunil Jain, CyberinQ LLC, 2026-10-29.
'use strict';
const fs=require('./legacy_contract_source.cjs'),assert=require('node:assert/strict');
const r=p=>fs.readFileSync(p,'utf8');
const sql=r('services/core-api/migrations/0042_wip_exceptions_and_recovery_v23.sql');
const go=r('services/edge-daemon/cmd/server/wip_exceptions_production.go');
const operator=r('services/edge-daemon/cmd/server/operator_station_production.go');
const ui=r('services/edge-daemon/internal/web/static/factory_wip_exceptions.js');
const html=r('services/edge-daemon/internal/web/static/factory_operator_station.html');
const main=r('services/edge-daemon/cmd/server/main.go');
const portal=r('services/edge-daemon/cmd/server/portal.go');
const workforce=r('services/edge-daemon/cmd/server/workforce_chapters.go');
const compose=r('compose/docker-compose.production.yml');
for(const x of ['wip_exception_cases_v23','wip_exception_closures_v23','ENABLE ROW LEVEL SECURITY','FORCE ROW LEVEL SECURITY','protect_cut_draft_v8','zz_wip_exception_hold_v23','zz_qc_exception_hold_v23','ERR_F310_UNRESOLVED_EXCEPTION_WIP_HOLD','ERR_F310_NEW_SIGNED_SCAN_REQUIRED','ERR_F310_INDEPENDENT_SUPERVISOR','ERR_F310_REPLACEMENT_BINDING_REQUIRED'])assert(sql.includes(x),x);
for(const x of ['s.observed_at<=c.opened_at','NOT EXISTS','pg_advisory_xact_lock','wip_gateway_scans_v20','gen_carrier_active_v18','sew_prod_worker_stations_v10'])assert(sql.includes(x),x);
assert(sql.includes('GRANT SELECT,INSERT')&&!sql.includes('GRANT UPDATE')&&!sql.includes('GRANT DELETE'));
for(const s of ['sewing.exception.report','sewing.exception.resolve']){assert(sql.includes(s));assert(go.includes(s));assert(workforce.includes(s));}
for(const s of ['exceptionChallenge','workforceChapterRequire','sewTxLock','sewing_supervisor','ERR_F310_CASE_OR_PROOF_CONFLICT'])assert(go.includes(s),s);
assert(!/UPDATE\s+wip_movements|DELETE\s+FROM\s+wip_movements|INSERT\s+INTO\s+wip_movements/i.test(go));
assert(operator.includes('UNRESOLVED_WIP_EXCEPTION_HOLD'));
assert(main.includes('registerWIPExceptionRoutes'));
assert(portal.includes('"/api/v1/wip/exceptions/events"')&&portal.includes('"/static/factory_wip_exceptions.js"'));
assert(compose.includes('AGIPAREL_WIP_EXCEPTIONS_ENABLED: ${AGIPAREL_WIP_EXCEPTIONS_ENABLED:-false}'));
assert(html.includes('factory_wip_exceptions.js')&&html.includes('Exception and supervised recovery'));
for(const term of ['innerHTML','localStorage','sessionStorage','document.cookie'])assert(!ui.includes(term),term);
assert(ui.includes("credentials:'same-origin'")&&ui.includes("credentials:'omit'"));
assert(ui.includes('data.custody_changed!==false')&&ui.includes('data.production_credit!==false'));
console.log('PASS F3.10: signed two-role exceptions, immutable ledger, SQL WIP/QC hold, scan-backed closure, zero custody credit');
