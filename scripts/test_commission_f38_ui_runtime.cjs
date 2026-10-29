// Author: Sunil Jain, CyberinQ LLC, 2026-10-28.
// Executes the real read-only HMI against simulated API replies; never writes factory records.
'use strict';
const fs=require('fs');const vm=require('vm');const assert=require('node:assert/strict');
const source=fs.readFileSync('services/edge-daemon/internal/web/static/factory_commissioning.js','utf8');
const elements=new Map();
const make=(tag='div')=>({tag,value:'',textContent:'',hidden:false,disabled:false,children:[],handlers:{},
 append(el){this.children.push(el)},appendChild(el){this.append(el)},replaceChildren(){this.children=[]},
 addEventListener(kind,fn){this.handlers[kind]=fn}});
for(const id of ['tenant','plant','load','more','status','summary','stations'])elements.set(id,make());
elements.get('tenant').value='Tenant_1';elements.get('plant').value='Plant_1';
let reply={tenant_id:'Tenant_1',plant_id:'Plant_1',authority:'ENROLLMENT_AND_SIGNED_REPORTS_NOT_PHYSICAL_CERTIFICATION',
 as_of:'2026-10-28T23:00:00Z',next_station:'',limitations:['not physically tested','not calibration','not production release'],
 stations:[{station_id:'ST_1',enabled_gateways:1,total_gateways:1,assigned_workers:2,
 enabled_inbound_routes:1,enabled_outbound_routes:0,last_signed_scan_received_at:null,
 status:'CONFIGURED_NOT_PHYSICALLY_CERTIFIED',blockers:[]}]};
let calls=0;
const fetch=async()=>{calls++;return {ok:true,json:async()=>reply}};
vm.runInNewContext(source,{document:{getElementById:id=>elements.get(id),createElement:make},fetch,URLSearchParams});
const flush=()=>new Promise(resolve=>setImmediate(resolve));
(async()=>{
 elements.get('load').handlers.click();await flush();await flush();
 assert.equal(calls,1);assert.equal(elements.get('stations').children.length,1);
 assert.match(elements.get('summary').textContent,/1 station configuration records/);
 assert.match(elements.get('stations').children[0].children.at(-1).textContent,/NOT_PHYSICALLY_CERTIFIED/);
 reply={...reply,stations:[{...reply.stations[0],enabled_gateways:0,status:'CONFIGURED_NOT_PHYSICALLY_CERTIFIED'}]};
 elements.get('load').handlers.click();await flush();await flush();
 assert.equal(elements.get('stations').children.length,0,'forged readiness must be rejected');
 assert.match(elements.get('status').textContent,/Invalid commissioning evidence/);
 elements.get('plant').value='Plant_2';elements.get('plant').handlers.input();
 assert.equal(elements.get('stations').children.length,0,'new plant must clear prior data');
 console.log('PASS F3.8 UI: authorized station data, spoofed readiness rejected, stale plant data cleared');
})().catch(err=>{console.error(err);process.exitCode=1});
