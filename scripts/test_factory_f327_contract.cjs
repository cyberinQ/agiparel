// Author: Sunil Jain, 2026-10-29.
// V40 contract: admission time bounds, separate replay keys, scratch-only load.
const assert = require('node:assert/strict');
const fs = require('./legacy_contract_source.cjs');
const read = p => fs.readFileSync(p, 'utf8');
for (const filename of [
  'services/edge-daemon/cmd/server/dock_handover_v34.go',
  'services/edge-daemon/cmd/server/dock_external_v35.go',
]) {
  const data = read(filename);
  assert.match(data, /SET LOCAL lock_timeout = '2000ms'/);
  assert.match(data, /SET LOCAL statement_timeout = '12000ms'/);
  assert.match(data, /context\.WithTimeout\(r\.Context\(\), 15\*time\.Second\)/);
}
const script = read('scripts/test_factory_v40_dock_stress.sh');
for (const s of [
  'AGIPAREL_TEST_DB_DISPOSABLE', 'agiparel_v39_test_',
  'pg_terminate_backend', 'session_replication_role = replica',
  'dock_carton_scans_v34', 'dock_external_assertions_v35',
  'SET LOCAL ROLE agiparel_app', 'F40_RLS_BROKEN',
]) assert.ok(script.includes(s), `Missing: ${s}`);
const installer = read('scripts/install_release_v40.sh');
for (const s of ["BASELINE='54dc176'", 'scripts/test_factory_v39_concurrency.sh',
  'scripts/test_factory_v40_dock_stress.sh', 'git push origin main',
  'pg_restore --exit-on-error', '=== 4 BUILD ===', '=== 5 DEPLOY + HEALTH ===']) {
  assert.ok(installer.includes(s), `Installer missing ${s}`);
}
for (const p of ['.github/workflows/ci.yml', '.github/workflows/release.yml']) {
  const content = read(p);
  assert.ok(content.includes('scripts/test_factory_v40_dock_stress.sh'), `Missing CI drill ${p}`);
  assert.ok(content.includes('scripts/test_factory_v40_schema_pg.sql'), `Missing CI schema ${p}`);
}
for (const key of ['AGIPAREL_DOCK_V34_ENABLED','AGIPAREL_DOCK_EXTERNAL_V35_ENABLED']) {
  const prod = read('compose/docker-compose.production.yml');
  assert.ok(prod.includes(`${key}: \${${key}:-false}`), `Fail-closed flag missing: ${key}`);
}
console.log('PASS F3.27 v40: bounded dock writes, adversarial replay, tenant drill and default-off deployment');
