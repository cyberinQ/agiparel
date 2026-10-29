// Author: Sunil Jain, CyberinQ LLC, 2026-10-29.
'use strict';
const assert=require('node:assert/strict'),fs=require('node:fs'),vm=require('node:vm');
const ids=['tenant','plant','station','ex-part','ex-type','ex-note','ex-list','ex-case','ex-open','ex-close','ex-challenge','ex-token','ex-submit','ex-cancel','ex-status','ex-cases'];
class Node{constructor(){this.value='';this.textContent='';this.disabled=false;this.children=[];this.events={};this.length=0;}
 addEventListener(n,fn){this.events[n]=fn;}click(){return this.events.click?.();}
 replaceChildren(){this.children=[];this.textContent='';}
 append(n){this.children.push(n);}
}
const elements=Object.fromEntries(ids.map(id=>[id,new Node()]));
const calls=[];
const doc={getElementById(id){return elements[id]},createElement(){return new Node()},querySelector(){return null}};
const context={document:doc,URLSearchParams,console,crypto:{randomUUID(){return 'c6e63b80-e150-4364-90be-759755e3ec46'}},
 fetch:async (path,options)=>{calls.push({path:String(path),options});return {ok:true,json:async()=>({})}}};
vm.runInNewContext(fs.readFileSync('services/edge-daemon/internal/web/static/factory_wip_exceptions.js','utf8'),context);
(async()=>{
 await elements['ex-open'].click();
 assert.equal(calls.length,0,'bad scope cannot make a request');
 elements.tenant.value='tenant_01';elements.plant.value='plant_01';elements.station.value='station_01';
 elements['ex-part'].value='c6e63b80-e150-4364-90be-759755e3ec46';elements['ex-note'].value='Missing garment carrier at station';
 elements['ex-type'].value='LOST_CARRIER';
 await elements['ex-close'].click();assert.equal(calls.length,0,'closure without case/scan cannot be prepared');
 await elements['ex-open'].click();assert.equal(calls.length,1,'report should request only a challenge');
 assert(calls[0].path.endsWith('/api/v1/wip/exceptions/challenge'));
 const c=JSON.parse(calls[0].options.body).command;
 assert.equal(c.action,'OPEN');assert.equal(c.issue_type,'LOST_CARRIER');assert.equal(c.scan_id,'');
 assert.equal(c.part_id,elements['ex-part'].value);assert.equal(calls[0].options.credentials,'same-origin');
 assert.equal(elements['ex-submit'].disabled,true,'invalid response must not enable submission');
 await elements['ex-submit'].click();assert.equal(calls.length,1,'no write without bound challenge and signed JWT');
 console.log('PASS F3.10 UI runtime: no unauthenticated operation, no scanless closure, strict challenge and no false write');
})().catch(e=>{console.error(e);process.exitCode=1});
