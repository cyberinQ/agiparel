// Author: Sunil Jain, CyberinQ LLC, 2026-10-29.
// V41: production default-off safety and disposable-RLS load contract.
const assert = require('node:assert/strict');
const fs = require('./legacy_contract_source.cjs');
const read = p => fs.readFileSync(p, 'utf8');
const prod = read('compose/docker-compose.production.yml');
for (const flag of [
  'AGIPAREL_SEWING_PRODUCTION_ENABLED', 'AGIPAREL_DOCK_V34_ENABLED',
  'AGIPAREL_DOCK_EXTERNAL_V35_ENABLED', 'AGIPAREL_DOCK_INTEGRITY_V38_ENABLED',
]) {
  assert.ok(prod.includes(`${flag}: \${${flag}:-false}`), `Not default-off: ${flag}`);
}
const test = read('scripts/test_factory_v41_multitenant_load.sh');
for (const s of [
  'AGIPAREL_TEST_DB_DISPOSABLE', 'agiparel_v39_test_',
  'SET LOCAL ROLE agiparel_app', "set_config('app.current_tenant_id'",
  'dock_gateways_v34', 'dock_external_signers_v35',
  'UPDATE dock_gateways_v34 SET enabled=false', 'BEGIN READ ONLY',
]) assert.ok(test.includes(s), `Missing real tenant check: ${s}`);
assert.ok(!test.includes('session_replication_role'), 'RLS drill must not bypass triggers');
const sql = read('scripts/test_factory_v41_security_pg.sql');
for (const s of [
  'relforcerowsecurity', 'pg_policies', 'has_table_privilege',
  'has_function_privilege', 'dock_external_assertions_v35',
  'dock_integrity_report_v38', 'shipment_credit',
]) assert.ok(sql.includes(s), `Missing security posture assertion: ${s}`);
for (const workflow of ['.github/workflows/ci.yml', '.github/workflows/release.yml']) {
  const contents = read(workflow);
  for (const s of ['test_factory_v41_security_pg.sql', 'test_factory_v41_multitenant_load.sh']) {
    assert.ok(contents.includes(s), `${workflow} missing ${s}`);
  }
}
const installer = read('scripts/install_release_v41.sh');
for (const s of [
  "BASELINE='4831c15'", 'test_factory_v41_security_pg.sql',
  'test_factory_v41_multitenant_load.sh', 'pg_restore --exit-on-error',
  'go test -race -count=1 ./...', 'git push origin main',
]) assert.ok(installer.includes(s), `Release missing: ${s}`);
console.log('PASS v41: explicit production opt-in, forced RLS and real app-role parallel isolation drill');
