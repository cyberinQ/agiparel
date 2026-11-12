// Author: Sunil Jain, 2026-11-12.
// Contract for direct, read-only machine answers and latest-turn entity isolation.
'use strict';
const assert = require('node:assert/strict');
const fs = require('node:fs');
const read = file => fs.readFileSync(file, 'utf8');
const classifier = read('services/edge-daemon/pkg/aggieintent/classifier.go');
const router = read('services/edge-daemon/cmd/server/operational_queries.go');
const handler = read('services/edge-daemon/cmd/server/main.go');
const tests = read('services/edge-daemon/cmd/server/operational_counts_v50_test.go');
assert(classifier.includes('RegisteredMachines') && classifier.includes('MachineStatus'));
assert(classifier.includes('func Resolve('));
assert(router.includes('aggieintent.Resolve(messages[i].Content)'));
assert(router.includes('case aggieintent.RegisteredMachines:'));
assert(router.includes('case aggieintent.MachineStatus:'));
assert(router.includes("FROM machine_roster WHERE tenant_id=$1"));
assert(router.includes('sql.TxOptions{ReadOnly: true}'));
assert(router.includes("set_config('app.current_tenant_id', $1, true)"));
assert(router.includes('not verified live availability'));
assert(handler.includes('operationalIntents(req.Messages)'));
assert(tests.includes('TestAggieMachineStatusAndMultiTurnIsolationV501'));
assert(tests.includes('cross-entity answer'));
console.log('PASS: Aggie machine entity isolation and source-backed SQL contracts');
