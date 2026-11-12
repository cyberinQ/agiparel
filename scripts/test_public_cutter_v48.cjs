// Author: Sunil Jain, 2026-11-12.
// Source contracts for explicitly opt-in, API-free public Cutter Mode.
'use strict';
const assert = require('node:assert/strict');
const fs = require('node:fs');
const read = p => fs.readFileSync(p, 'utf8');
const base = 'services/edge-daemon/';
const gate = read(base + 'cmd/server/portal.go');
const view = read(base + 'cmd/server/public_cutter_demo_v48.go');
const js = read(base + 'internal/web/static/cutter_demo.js');
const compose = read('compose/docker-compose.dev.yml');
assert.match(gate, /"\/demo\/cutting"/);
assert.match(view, /AGIPAREL_PUBLIC_CUTTER_DEMO_ENABLED/);
assert.match(view, /data-demo-mode/);
assert.match(js, /publicDemo/);
assert.match(js, /if \(publicDemo\)/);
assert.match(compose, /AGIPAREL_PUBLIC_CUTTER_DEMO_ENABLED:-false/);
assert.doesNotMatch(view, /DATABASE_URL|INSERT INTO|UPDATE .* SET/);
assert.doesNotMatch(js, /localStorage|sessionStorage|indexedDB/);
console.log('PASS: v48 opt-in training, public route gating and production-data isolation source contracts');
