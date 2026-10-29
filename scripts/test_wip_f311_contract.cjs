// Author: Sunil Jain, CyberinQ LLC, 2026-10-29.
// F3.11 does not introduce a new WIP authority or relax server admission.
'use strict';
const fs = require('node:fs');
const assert = require('node:assert/strict');
const r = p => fs.readFileSync(p,'utf8');
const relay = r('services/edge-daemon/cmd/scanrelay/main.go');
const server = r('services/edge-daemon/cmd/server/wip_production.go');
const exceptionSQL = r('services/core-api/migrations/0042_wip_exceptions_and_recovery_v23.sql');
assert(server.includes('"/api/v1/wip/production/scans"'));
assert(server.includes('ed25519.Verify(') && server.includes('ERR_WIP_SCAN_EXPIRED'));
assert(server.includes('ERR_WIP_SCAN_REPLAY_MISMATCH'));
assert(relay.includes('/api/v1/wip/production/scans'));
for(const s of ['https','durableWrite','durableMove','LOCK_EX','maxPending','accepted','inflight','quarantine','SCAN_EXPIRED_REQUIRES_EXCEPTION_REVIEW','SIGNED_GATEWAY_REPORT_NOT_INDEPENDENT_RF_PROOF','production_credit']) assert(relay.includes(s),s);
for(const forbidden of ['INSERT INTO wip_movements','INSERT INTO wip_gateway_scans','UPDATE wip_movements','DELETE FROM wip_movements','ed25519.Sign(']) assert(!relay.includes(forbidden),forbidden);
assert(exceptionSQL.includes('ERR_F310_NEW_SIGNED_SCAN_REQUIRED'));
console.log('PASS F3.11: durable, bounded, HTTPS signed-envelope relay; server verification unchanged; quarantine not custody credit');
