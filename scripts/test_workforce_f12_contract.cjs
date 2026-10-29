// Author: Sunil Jain, 2026-10-28.
// F1.2 invariant regression: secure sessions, no implicit authority, existing styles.
'use strict';
const fs=require('./legacy_contract_source.cjs');const assert=require('node:assert/strict');
const api=fs.readFileSync('services/edge-daemon/cmd/server/workforce_browser.go','utf8');
const report=fs.readFileSync('services/edge-daemon/cmd/server/intelligence_production.go','utf8');
const page=fs.readFileSync('services/edge-daemon/internal/web/static/factory_intelligence_production.html','utf8');
const ui=fs.readFileSync('services/edge-daemon/internal/web/static/factory_intelligence_production.js','utf8');
const migration=fs.readFileSync('services/core-api/migrations/0035_workforce_oidc_sessions_v16.sql','utf8');
for(const key of ['code_challenge_method','S256','code_verifier','nonce','VerifyIDToken','__Host-agiparel-session','HttpOnly: true','Secure: true','SameSite: http.SameSiteLaxMode','workforce_session_revoke_v16','X-Agiparel-CSRF']) assert(api.includes(key),key);
for(const key of ['FORCE ROW LEVEL SECURITY','SECURITY DEFINER','workforce_oidc_consume_v16','workforce_session_get_v16','workforce_session_csrf_rotate_v16','REVOKE ALL'])assert(migration.includes(key),key);
assert(!migration.includes('GRANT SELECT ON workforce_browser_sessions_v16'));
assert(report.includes('workforceReportGrant') && report.includes('intelligence_prod_readers_v13'));
assert(page.includes('id="signin"') && page.includes('id="signout"'));
assert(ui.includes("credentials:mode==='session'?'same-origin':'omit'"));
assert(!ui.includes('localStorage')&&!ui.includes('innerHTML'));
console.log('PASS F1.2: OIDC PKCE, nonce, opaque sessions, CSRF and scoped read-only authorization');
