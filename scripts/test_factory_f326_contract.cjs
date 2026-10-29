// Author: Sunil Jain, CyberinQ LLC, 2026-10-29.
// Regression contract: v39 security test isolation and production gates.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const prod = fs.readFileSync('compose/docker-compose.production.yml', 'utf8');
const script = fs.readFileSync('scripts/test_factory_v39_concurrency.sh', 'utf8');
const ci = fs.readFileSync('.github/workflows/ci.yml', 'utf8');
const release = fs.readFileSync('.github/workflows/release.yml', 'utf8');
const installer = fs.readFileSync('scripts/install_release_v39.sh', 'utf8');
const api = fs.readFileSync('services/edge-daemon/cmd/server/dock_external_v35.go', 'utf8');
for (const key of [
  'AGIPAREL_DOCK_EXTERNAL_V35_ENABLED',
  'AGIPAREL_DOCK_DIAGNOSTICS_V36_ENABLED',
  'AGIPAREL_DOCK_LINEAGE_AUDIT_V37_ENABLED',
  'AGIPAREL_DOCK_INTEGRITY_V38_ENABLED',
]) assert.ok(prod.includes(`${key}: \${${key}:-false}`), `production gate ${key} not disabled by default`);
for (const needle of ['AGIPAREL_TEST_DB_DISPOSABLE', 'session_replication_role = replica',
  'pack_one_carton_per_garment_v22', 'pg_terminate_backend', 'Cross-tenant']) {
  assert.ok(script.includes(needle), `concurrency guard missing: ${needle}`);
}
for (const flow of [ci, release]) {
  assert.ok(flow.includes('scripts/test_factory_v39_concurrency.sh'));
  assert.ok(flow.includes('scripts/test_factory_v39_schema_pg.sql'));
}
assert.ok(api.includes("SET LOCAL statement_timeout = '8000ms'"));
assert.ok(api.includes("SET LOCAL lock_timeout = '1500ms'"));
assert.ok(installer.includes('git rev-parse --short HEAD'));
assert.ok(installer.includes('git push origin main'));
assert.ok(installer.includes('scripts/test_factory_v39_concurrency.sh'));
assert.ok(installer.includes('=== 2 MIGRATION: NONE'));
console.log('PASS v39: locked production flags, scratch-only race/crash testing, CI and installer');
