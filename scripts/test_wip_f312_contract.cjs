// Author: Sunil Jain, CyberinQ LLC, 2026-10-29.
// F3.12: source contract — authentic signature, real receipt, RLS DB audit; no manufactured scans.
'use strict';
const fs = require('node:fs');
const assert = require('node:assert/strict');
const read = p => fs.readFileSync(p,'utf8');
const server = read('services/edge-daemon/cmd/server/wip_production.go');
const relay = read('services/edge-daemon/cmd/scanrelay/main.go');
const check = read('services/edge-daemon/cmd/scanrelay/commission.go');
const pg = read('scripts/test_wip_f312_live_pg.sql');
const shell = read('scripts/run_factory_f312_acceptance.sh');
const test = read('services/edge-daemon/cmd/scanrelay/commission_test.go');
assert(server.includes('"AGIPAREL_WIP_GATEWAY_V1\\n"'));
assert(check.includes('"AGIPAREL_WIP_GATEWAY_V1\\n"'));
assert(server.includes('ed25519.Verify(') && check.includes('ed25519.Verify('));
for (const v of ['EventSHA256','SignatureSHA256','EnvelopeSHA256','SignedEnvelopeValid','ReceiptVerified','DatabaseVerified','PhysicalRFCertification','openExistingState','scope']) assert(check.includes(v),v);
for(const v of ['"verify"','"audit"','runCommission','--dir required']) assert(relay.includes(v),v);
for(const v of ['BEGIN READ ONLY','SET LOCAL ROLE agiparel_app','app.current_tenant_id','enrolled_key_match','immutable_scan_match','unique_replay_identity','false AS physically_certified','ROLLBACK']) assert(pg.includes(v),v);
for(const v of ['F312_FILE','F312_TENANT','F312_PLANT','F312_GATEWAY','F312_STATION','F312_PUBLIC_KEY_HEX','test_wip_f312_live_pg.sql']) assert(shell.includes(v),v);
for(const v of ['TestF312AuthenticMessageAndDigests','TestF312ReceiptRequiresExactAcceptedRecord','TestF312VerifyAndAuditCommandsRemainReadOnly','TestF312FutureClockFailClosed']) assert(test.includes(v),v);
for(const v of [relay,check,pg,shell]) {
 for(const forbidden of ['INSERT INTO wip_gateway_scans_v20','INSERT INTO wip_movements_v20','ed25519.Sign(', 'InsecureSkipVerify:true','DO $$ CREATE']) assert(!v.includes(forbidden),forbidden);
}
console.log('PASS F3.12: signature+scope verification, exact outbox receipt, tenant-RLS read-only PostgreSQL audit; RF proof not claimed');
