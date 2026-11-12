// Author: Sunil Jain, 2026-11-12.
// Check local-only deployment, explicit image pins, and no unsafe AeroLake pruning.
'use strict';
const fs = require('fs');
const assert = require('assert');
const read = p => fs.readFileSync(p, 'utf8');
const compose = read('compose/docker-compose.factory-v50.yml');
const bridge = read('services/aerolake-bridge/main.go');
const deploy = read('scripts/factory_pilot_v50.sh');
const server = read('services/edge-daemon/cmd/server/operational_queries.go');
const handler = read('services/edge-daemon/cmd/server/main.go');
for (const service of ['postgres:', 'redis:', 'ollama:', 'aerolake-agentd:', 'aerolake-bridge:', 'document-intake:', 'plant-edge-os:']) {
  assert(compose.includes(service), `missing ${service}`);
}
assert(compose.includes('127.0.0.1:${AGIPAREL_PILOT_PORT:-18539}:18530'));
assert(compose.includes('AEROLAKE_PRUNING_KILL_SWITCH: "true"'));
assert(compose.includes('AGIPAREL_CUT_SOP2_WRITES_ENABLED: "false"'));
assert(compose.includes('AGIPAREL_PUBLIC_CUTTER_DEMO_ENABLED: "false"'));
assert(compose.includes('AGIPAREL_OLLAMA_IMAGE:?') && compose.includes('AGIPAREL_AEROLAKE_IMAGE:?'));
assert(!/prune_v45_telemetry_batch\s*\(/.test(bridge), 'unsafe retention call reintroduced');
assert(bridge.includes('RETENTION BLOCKED'));
assert(deploy.includes('check_pins') && deploy.includes('AGIPAREL_CONFIRM_V50_PILOT'));
assert(!/^\s*compose down\s+(--volumes|-v)/m.test(deploy));
assert(server.includes('case aggieintent.InventoryRolls:') && server.includes('case aggieintent.AttendanceToday:'));
assert(handler.includes('operationalIntents(req.Messages)'));
assert(read('.gitignore').includes('.env.factory-v50'));
console.log('PASS: v50 multi-count, isolated factory pilot, no destructive telemetry pruning, secret exclusion');
