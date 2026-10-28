// Author: Sunil Jain, 2026-10-28.
// F1.3: enforce a central, unprivileged policy catalog without opening legacy APIs.
'use strict';
const fs=require('node:fs');const assert=require('node:assert/strict');
const base='services/edge-daemon/cmd/server/';
const acl=fs.readFileSync(base+'workforce_chapters.go','utf8');
const migration=fs.readFileSync('services/core-api/migrations/0036_workforce_chapter_policy_v17.sql','utf8');
const portal=fs.readFileSync(base+'portal.go','utf8');
const main=fs.readFileSync(base+'main.go','utf8');
for(const p of ['sewing.complete','quality.inspect','quality.repair','quality.read','packing.stage','packing.read','intelligence.read']) {
 assert(acl.includes('"'+p+'"'),p);
 assert(migration.includes("'"+p+"'"),p);
}
for(let ch=1;ch<=10;ch++) assert(migration.includes(' ('+ch+','),'missing chapter '+ch);
assert(acl.includes('JOIN workforce_chapter_actions_v17')&&acl.includes('workforce_identity_grants_v15'));
assert(acl.includes('g.tenant_id=$1 AND g.plant_id=$2 AND g.idp_issuer=$3 AND g.idp_subject=$4'));
assert(acl.includes('AND g.enabled'));
assert(migration.includes('GRANT SELECT')&&!migration.includes('GRANT INSERT')&&!migration.includes('GRANT UPDATE'));
assert(acl.includes('NO_PRODUCTION_ROUTE_AVAILABLE')&&acl.includes('SIGNED_TRANSACTION_PROOF_REQUIRED'));
assert(portal.includes('"/api/v1/workforce/chapter-access"')&&main.includes('registerWorkforceChapterRoutes'));
const cases=[['sewing_production.go','sewing.complete'],['quality_production.go','quality.read'],['quality_production.go','qcProdRole(c.Action)'],['packing_production.go','packing.stage'],['packing_production.go','packing.read'],['intelligence_production.go','workforceReportGrant']];
for(const [name,p] of cases)assert(fs.readFileSync(base+name,'utf8').includes(p),`${name}:${p}`);
assert(!portal.includes('case "/api/v1/":'));
console.log('PASS F1.3: shared chapter policy, provisioned grants, no implicit production access');
