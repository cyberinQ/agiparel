// Author: Sunil Jain, CyberinQ LLC, 2026-10-29.
// Structural gate: no invented RF measurements or cloth-consumption constants.
const fs = require('node:fs');
const assert = require('node:assert/strict');
const rf = fs.readFileSync('services/edge-daemon/pkg/rfid/llrp_reader.go','utf8');
const pl = fs.readFileSync('services/edge-daemon/pkg/planning/analyzer.go','utf8');
const wb = fs.readFileSync('services/edge-daemon/pkg/onboarding/workbook.go','utf8');
for (const phrase of ['parseROAccessReport','llrpParameters','llrpEPCData','llrpEPC96','LastKeepaliveReceivedAt','ReaderUTCMeasured','AntennaMeasured','RSSIMeasured','llrpKeepaliveAck','DialContext','RetryMax','RejectedFrames']) assert(rf.includes(phrase),`missing LLRP safeguard ${phrase}`);
assert(!rf.includes('RSSI: -42.5'));
assert(!rf.includes('payload[:epcLength]'));
assert(!rf.includes('AntennaID: 1'));
assert(pl.includes('wb.BOMRows'));
assert(pl.includes('WORKBOOK-DECLARED ONLY'));
assert(!pl.includes('0.35'));
assert(!pl.includes('consumptionPerGarment = 1.25'));
assert(wb.includes('case "STYLE_BOM"'));
assert(wb.includes('QtyPerGarment'));
console.log('PASS F3.13/F3.15: genuine LLRP fields and provisional BOM-meter planning, no placeholder sensor or cloth values');
