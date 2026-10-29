// Author: Sunil Jain, 2026-10-28.
// F1: fail-closed workforce permissions and backwards compatible read-only reporting.
'use strict';
const fs=require('./legacy_contract_source.cjs');
const assert=require('node:assert/strict');
const root='services/edge-daemon/';
const api=fs.readFileSync(root+'cmd/server/intelligence_production.go','utf8');
const verifier=fs.readFileSync(root+'pkg/workforceauth/verifier.go','utf8');
const migration=fs.readFileSync('services/core-api/migrations/0034_workforce_identity_grants_v15.sql','utf8');
const page=fs.readFileSync(root+'internal/web/static/factory_intelligence_production.html','utf8');
const js=fs.readFileSync(root+'internal/web/static/factory_intelligence_production.js','utf8');
for (const token of ['workforceReportGrant','workforce_identity_grants_v15','intelligence_prod_readers_v13','AGIPAREL_WORKFORCE_OIDC_ENABLED','p.sewing.verifier.Verify','"intelligence.read"']) assert(api.includes(token),token);
assert(api.includes('if enterprise {') && api.includes('if !granted {'));
assert(api.includes('p.reason = "ERR_INTEL_PROD_WORKFORCE_CONFIG"'));
for (const token of ['rsa.VerifyPKCS1v15','ed25519.Verify','c.Iss != v.Issuer','audienceIncludes','!strong','c.Exp <= t']) assert(verifier.includes(token),token);
assert(!verifier.includes('r.Header.Get(') && !verifier.includes('Mint('));
for (const token of ['FORCE ROW LEVEL SECURITY','GRANT SELECT','REVOKE ALL','enabled boolean NOT NULL DEFAULT false']) assert(migration.includes(token),token);
assert(!migration.includes('GRANT INSERT') && !migration.includes('GRANT UPDATE'));
for (const token of ['id="authmode"','id="tenant"','id="plant"']) assert(page.includes(token),token);
assert(js.includes("query.set('tenant_id',tenant)") && js.includes("query.set('plant_id',plant)"));
assert(!js.includes('localStorage') && !js.includes('innerHTML'));
console.log('PASS F1.1: signed identity, tenant grants, no pilot fallback, no mutation authority');
