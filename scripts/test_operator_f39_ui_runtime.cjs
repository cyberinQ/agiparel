// Author: Sunil Jain, CyberinQ LLC, 2026-10-28.
// Exercise the actual operator UI in a DOM stub, no network or factory writes.
'use strict';
const fs=require('node:fs'),vm=require('node:vm'),assert=require('node:assert/strict');
const ids=['tenant','plant','station','refresh','status','inbox','detail-status','detail','blockers','route','prepare','challenge','signed-token','submit','cancel','result'];
function node(tag='div'){return {tag,type:'',textContent:'',value:'',disabled:false,children:[],events:{},
 addEventListener(t,f){this.events[t]=f},append(x){this.children.push(x)},appendChild(x){this.append(x)},replaceChildren(){this.children=[]}}}
const els=Object.fromEntries(ids.map(k=>[k,node()]));els.tenant.value='tenantA';els.plant.value='Plant_1';els.station.value='ST_1';
const idsU={scan:'c6e63b80-e150-4364-90be-759755e3ec46',part:'192e61d2-e8dc-48df-9ef6-8706a80a7004',garment:'09352679-cfe3-468a-9c13-4e5d9d53dc1f',bundle:'ca671005-a500-4f5e-839b-ff308e5c63ed',route:'234f0549-c9ee-4ff9-8895-a9cfe17b11e6',event:'fa914756-e929-40ed-ac94-f5f29c330a13'};
let detailStatus='READY_FOR_SIGNED_HANDOFF',writes=0,lastWrite=null,requests=[];
const authority='ENROLLED_GATEWAY_REPORT_AND_SIGNED_CUSTODY_NOT_PHYSICAL_CERTIFICATION';
const api=async(url,opts={})=>{requests.push([String(url),opts]);if(String(url).includes('/operator-station/inbox')){
 const selected=new URL('http://localhost'+url).searchParams.get('scan_id');const detail=selected?{scan_id:idsU.scan,part_id:idsU.part,garment_id:idsU.garment,bundle_id:idsU.bundle,part_code:'SEAM',received_at:'2026-10-28T22:00:00Z',last_action:'NONE',last_station:'',last_recorded_at:null,status:detailStatus,blockers:detailStatus==='BLOCKED'?['CARRIER_BINDING_RETIRED_OR_REASSIGNED']:[],next:detailStatus==='BLOCKED'?[]:[{route_id:idsU.route,action:'DEPART',station_id:'ST_1',other_station:'ST_2',revision:'REV1'}]}:null;
 return {ok:true,json:async()=>({tenant_id:'tenantA',plant_id:'Plant_1',station_id:'ST_1',authority,as_of:'2026-10-28T22:00:00Z',limitations:['not physically certified','not full inventory','not output credit'],scans:[{scan_id:idsU.scan,part_id:idsU.part,received_at:'2026-10-28T22:00:00Z'}],detail})};
 }if(String(url).endsWith('/movements/challenge'))return {ok:true,json:async()=>({challenge:'WIP_MOVE_V1:'+'a'.repeat(64),required_action:'sewing.transfer',required_auth:'webauthn_or_fido2',production_credit:false})};
 if(String(url).endsWith('/movements')){writes++;lastWrite=opts;return {ok:true,json:async()=>({id:idsU.event,action:'DEPART',authority:'SIGNED_SCAN_AND_WORKER_CUSTODY_ATTESTATION',production_credit:false,shipping_release:false})};}
 throw Error('unexpected URL: '+url)};
const ctx={document:{getElementById:id=>els[id],createElement:node},fetch:api,URLSearchParams,URL,crypto:{randomUUID:()=>idsU.event},Date,Number,Object,String,Error,console};
vm.runInNewContext(fs.readFileSync('services/edge-daemon/internal/web/static/factory_operator_station.js','utf8'),ctx);
const flush=()=>new Promise(resolve=>setImmediate(resolve));
(async()=>{
 els.refresh.events.click();await flush();await flush();
 assert.equal(els.inbox.children.length,1);assert.equal(writes,0);
 const select=els.inbox.children[0].children.at(-1).children[0];select.events.click();await flush();await flush();
 assert.equal(els.detail.children.length,9);assert.equal(els.prepare.disabled,false);assert.equal(els.route.children.length,1);els.route.value=idsU.route;
 await els.prepare.events.click();await flush();assert.match(els.challenge.textContent,/WIP_MOVE_V1:/);
 const token='eyJhbGciOiJFZERTQSJ9.eyJleHAiOjg4ODg4ODh9.'+'a'.repeat(86);
 els['signed-token'].value=token;await els.submit.events.click();await flush();await flush();
 assert.equal(writes,1);assert.equal(lastWrite.credentials,'omit');assert.equal(lastWrite.headers.Authorization,'Bearer '+token);
 assert.equal(els['signed-token'].value,'');assert.equal(els.submit.disabled,true);
 assert.equal(JSON.parse(lastWrite.body).command.scan_id,idsU.scan);
 // Valid signed response, not a fabricated completion or shipment.
 detailStatus='BLOCKED';els.refresh.events.click();await flush();await flush();
 els.inbox.children[0].children.at(-1).children[0].events.click();await flush();await flush();
 assert.equal(els.prepare.disabled,true);assert.match(els.blockers.textContent,/CARRIER_BINDING_RETIRED/);
 assert.equal(writes,1,'blocked scan must not create write');
 els.plant.value='Plant_2';els.plant.events.input();assert.equal(els.inbox.children.length,0,'plant switch must clear prior data');
 console.log('PASS F3.9 UI: signed scan, explicit challenge, ephemeral assertion, blocked stale binding, plant scope reset');
})().catch(e=>{console.error(e);process.exitCode=1});
