// Author: Sunil Jain, 2026-11-12.
// Regression tests for the isolated, browser-only SOP-2 cutter demo and localhost integration.
'use strict';
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const M = require('../services/edge-daemon/internal/web/static/cutter_demo_model.js');
const root = path.resolve(__dirname, '..');
let passed = 0;
function test(name, fn) { try { fn(); passed++; console.log('PASS:', name); } catch(e) { console.error('FAIL:', name); throw e; } }
const launch = () => M.markOptimized(M.begin(M.initial()), null, 'training');
const lay = (s, plies, roll) => M.applyLay(s, plies, roll).ok;
function twoLays(a=51,b=51) {
  const s=launch(); assert(lay(s,a,'R-101')); assert(lay(s,b,'R-102')); return s;
}
test('buyer PO 400 + extras 8 = frozen teaching target 408', () => {
  assert.deepEqual(M.ORDER.buyer, {S:100,M:200,L:100});
  assert.deepEqual(M.ORDER.target, {S:102,M:204,L:102});
  assert.equal(M.snapshot(M.initial()).target,408);
});
test('loading training fixture does not claim an approved factory target',()=>{
 const s=M.begin(M.initial()); assert(s.started); assert(!s.optimized); assert.equal(M.snapshot(s).signedReleased,0);
});
test('cannot plan without training order',()=>{
 const s=M.initial(); M.markOptimized(s,null,'training'); assert(!s.optimized);
});
test('live optimizer result must exactly match the 408-piece two-cut demo',()=>{
 const good={feasible:true,cuts:2,steps:[{plies:51},{plies:51}],optimality_proven:true,target_by_sku:{'NAVY|S':102,'NAVY|M':204,'NAVY|L':102}};
 const s=M.begin(M.initial());M.markOptimized(s,{...good,cuts:3},'engine');assert(!s.optimized);
 M.markOptimized(s,good,'engine');assert(s.optimized);assert(s.optimizerEvidence.proven);
});
test('two 51-ply lays achieve exactly 102 / 204 / 102',()=>{
 const s=twoLays(); assert.deepEqual(M.produced(s),M.ORDER.target);assert.deepEqual(M.balance(s),{S:0,M:0,L:0});assert(M.doneCutting(s));
});
test('48-ply lay automatically leaves 54 and reaches zero',()=>{
 const s=launch();assert(lay(s,48,'R-101'));assert.equal(M.nextPlies(s),54);assert(lay(s,54,'R-102'));assert.equal(M.snapshot(s).remaining,0);
});
test('51/51 consumes 382.5 m per lay and conserves fabric length',()=>{
 const f=M.fabricBalance(twoLays());assert.equal(f[0].laidM,382.5);assert.equal(f[0].endM,16.5);assert.equal(f[1].endM,36.5);f.forEach(x=>assert.equal(x.deltaM,0));
});
test('48/54 conserves fabric with reallocated end bits',()=>{
 M.fabricBalance(twoLays(48,54)).forEach(x=>assert.equal(x.deltaM,0));
});
test('wrong-shade roll R103 blocked despite available length',()=>{
 const s=launch();const out=M.applyLay(s,51,'R-103');assert(!out.ok);assert.match(out.reason,/SHADE/);assert.equal(s.lays.length,0);
});
test('wrong allocated same-shade roll blocked',()=>{
 const s=launch();const out=M.applyLay(s,51,'R-102');assert(!out.ok);assert.match(out.reason,/ROLL ALLOCATION/);
});
test('overcut of second lay blocked after a 48-ply first',()=>{
 const s=launch();assert(lay(s,48,'R-101'));const r=M.applyLay(s,55,'R-102');assert(!r.ok);assert.match(r.reason,/OVER-CUT/);assert.equal(M.nextPlies(s),54);
});
test('first lay cannot leave an unfillable balance',()=>{
 const s=launch();const r=M.applyLay(s,44,'R-101');assert(!r.ok);assert.match(r.reason,/BALANCE/);assert.equal(s.lays.length,0);
});
test('final lay cannot leave undercut unaccounted',()=>{
 const s=launch();assert(lay(s,51,'R-101'));const r=M.applyLay(s,50,'R-102');assert(!r.ok);assert.match(r.reason,/UNDER-CUT/);
});
test('plies must be positive safe integers',()=>{
 const s=launch();for(const n of [0,-1,1.25,NaN,Number.MAX_SAFE_INTEGER+1])assert(!M.checkLay(s,n,'R-101').ok);
});
test('no scanning or release before both simulated cuts',()=>{
 const s=launch();assert(!M.scanPart(s,'FRONT').ok);assert(!M.issue(s).ok);assert.equal(s.scanned.length,0);
});
test('incomplete family cannot issue even with 3/4 scans',()=>{
 const s=twoLays();['FRONT','BACK','SR'].forEach(t=>assert(M.scanPart(s,t).ok));assert(!M.verifyBundle(s).ok);assert(!M.issue(s).ok);assert(!s.issued);
});
test('duplicate tag is rejected',()=>{
 const s=twoLays();assert(M.scanPart(s,'FRONT').ok);assert(!M.scanPart(s,'FRONT').ok);assert.equal(s.scanned.length,1);
});
test('unknown tag is rejected',()=>{
 const s=twoLays();assert(!M.scanPart(s,'UNRECOGNIZED').ok);
});
test('malformed duplicate marriage scan data is rejected',()=>{
 const s=twoLays();assert(!M.verifyBundle(s,['FRONT','BACK','SL','SL']).ok);
});
test('four valid simulated scans allow teaching handoff but never a signed release',()=>{
 const s=twoLays();M.PARTS.forEach(p=>assert(M.scanPart(s,p.id).ok));assert(M.verifyBundle(s).ok);assert(M.issue(s).ok);assert(M.snapshot(s).issued);assert.equal(M.snapshot(s).signedReleased,0);assert(!M.issue(s).ok);
});
test('three exception scenarios never alter production simulation counts',()=>{
 const s=twoLays();const before=M.snapshot(s);['shade','overcut','missing'].forEach(kind=>assert(!M.scenario(s,kind).ok));const after=M.snapshot(s);assert.equal(before.simulatedCut,after.simulatedCut);assert.equal(before.scans,after.scans);assert.equal(after.signedReleased,0);
});
test('reset creates an isolated fresh instance',()=>{
 const s=twoLays();const t=M.initial();assert.equal(M.snapshot(t).simulatedCut,0);assert.equal(M.snapshot(s).simulatedCut,408);
});
test('browser code is syntactically separate from the model and contains no write path',()=>{
 const js=fs.readFileSync(path.join(root,'services/edge-daemon/internal/web/static/cutter_demo.js'),'utf8');
 assert.match(js,/\/api\/v1\/cutting\/sop2\/solve/);
 assert(!/\/api\/v1\/cutting\/(?:sop2\/targets|lay-drafts|reader-events|spreads|release-status)/.test(js));
 assert(!/localStorage|sessionStorage|indexedDB|WebSocket/.test(js));
});
test('restricted localhost-only route is in existing SOP2 mux behind configured operator gate',()=>{
 const api=fs.readFileSync(path.join(root,'services/edge-daemon/cmd/server/cutting_sop2_api.go'),'utf8');
 const main=fs.readFileSync(path.join(root,'services/edge-daemon/cmd/server/main.go'),'utf8');
 assert.match(api,/mux.HandleFunc\("\/cutting\/demo"/);
 assert.match(api,/os.Getenv\("ENV"\) != "development"/);
 assert.match(api,/os.Getenv\("AGIPAREL_BIND_HOST"\) != "127\.0\.0\.1"/);
 assert.match(main,/Handler:\s+configuredOperatorGate\(mux\)/);
});
console.log(`PASS: ${passed} Cutter Mode acceptance checks`);
