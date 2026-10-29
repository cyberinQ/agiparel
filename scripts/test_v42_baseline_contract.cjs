// Author: Sunil Jain, 2026-10-29.
// Assert a single active canonical database baseline, safe role bootstrap and archived legacy fixtures.
'use strict';
const fs = require('node:fs');
const path = require('node:path');
const assert = require('node:assert/strict');
const root = path.resolve(__dirname, '..');
const migrationDir = path.join(root, 'services/core-api/migrations');
const migrationFiles = fs.readdirSync(migrationDir).filter(n => /^\d{4}_.*\.sql$/.test(n));
assert.deepEqual(migrationFiles, ['0001_v42_baseline.sql']);
const sql = fs.readFileSync(path.join(migrationDir, migrationFiles[0]), 'utf8');
for (const marker of ['CREATE TABLE', 'ROW LEVEL SECURITY', 'dock_integrity_report_v38', 'workforce_chapter_actions_v17']) {
  assert.ok(sql.includes(marker), `canonical schema missing ${marker}`);
}
assert.ok(!sql.includes('app_secure_runtime_pass') && !sql.includes('doc_worker_dev_only_change_in_production'));
const compose = fs.readFileSync(path.join(root, 'compose/docker-compose.dev.yml'), 'utf8');
assert.ok(compose.includes('0001_v42_baseline.sql'));
assert.ok(compose.includes('AGIPAREL_APP_DB_PASSWORD'));
assert.ok(compose.includes('edge_wal_v42'));
assert.ok(!compose.includes('app_secure_runtime_pass'));
const legacy = require('./legacy_contract_source.cjs');
assert.ok(legacy.readFileSync('services/core-api/migrations/0057_material_attribution_integrity_v38.sql', 'utf8').includes('dock_material_trace_v38'));
console.log('PASS v42: one active canonical schema, no embedded role passwords, archived historical contracts');
