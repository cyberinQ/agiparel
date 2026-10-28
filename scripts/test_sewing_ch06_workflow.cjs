// Author: Sunil Jain, 2026-10-28
// Chapter 06 browser regression: paged catalogs, identity, stale data, and offline replay.
'use strict';
const assert=require('node:assert/strict');
const fs=require('node:fs');
const vm=require('node:vm');
const path=require('node:path');
class El {
 constructor(){this.value='';this.disabled=false;this._html='';this.textContent='';this.className='';this.events={};this.dataset={};this.buttons=[];this.classList={toggle:(name,on)=>{this.hidden=on;}};}
 set innerHTML(v){this._html=String(v);this.buttons=[];for(const match of this._html.matchAll(/data-label="([^"]+)"/g)){const b=new El();b.dataset.label=match[1];this.buttons.push(b)}}
 get innerHTML(){return this._html}
 querySelectorAll(selector){return selector==='button[data-label]'?this.buttons:[]}
 insertAdjacentHTML(_,html){this._html+=html}
 addEventListener(k,cb){(this.events[k]??=[]).push(cb)}
 async fire(k){for(const cb of this.events[k]||[])await cb()}
 prepend(){}
}
const names=['lay','group','bundlePreview','reload','moreLays','moreGroups','plant','station','worker','machine','identityHelp','op','newCode','newMode','newParts','newPred','newSam','create','opInfo','scanButtons','scanStatus','claim','currentCycle','pause','resume','complete','issue','report','total','pending','sync','refresh','netState','dropPending','historyScope','cycles','feedback','log'];
const nodes=Object.fromEntries(names.map(n=>[n,new El()]));
nodes.newMode.value='BUNDLE_TO_SINGLE';
const storage=new Map();
const localStorage={getItem:k=>storage.get(k)||null,setItem:(k,v)=>storage.set(k,v),removeItem:k=>storage.delete(k)};
let requests=[],postCount=0,statusHold=null;
const layId='12345678-1234-1234-1234-123456789abc';
const part={marker_position:1,ply_group:1,part_code:'FRONT',label_code:'CB08-AAAAAAAAAAAAAAAAAAAAAAAA',color:'NAVY',size:'M',ply_start:1,ply_end:20,pieces_per_garment:1};
const op={id:layId,operation_code:'SEAM',mode:'BUNDLE_TO_BUNDLE',required_parts:['FRONT'],predecessors:[],sam_seconds:38,machine_class:'GENERIC_CLASS'};
const response=(v,status=200)=>({ok:status<400,status,json:async()=>v});
async function fetch(url,options={}){
 requests.push({url,options});
 if(options.method==='POST'){
   postCount++;
   const b=JSON.parse(options.body);
   if(b.event_id&&postCount===1)throw Error('Offline');
   return response({cycle_id:layId,action:b.action,provenance:'UI_SIMULATED_NO_RF_OBSERVATION',evidence_state:'SIMULATED_NOT_PRODUCTION_CREDIT'});
 }
 if(url.includes('/operations?'))return response({operations:[op]});
 if(url.includes('/status?')){
   if(statusHold)return new Promise(r=>statusHold.push(r));
   return response({cycles:[],cycles_has_more:true,total_simulated_operation_units:444,active_cycle:null,active_conflict:false});
 }
 if(url.includes('bundle_position='))return response({bundles:[part],lays:[],groups:[]});
 if(url.includes('group_cursor='))return response({groups:[{marker_position:2,ply_group:1,part_count:1}],groups_next_cursor:'',lays:[],bundles:[]});
 if(url.includes('lay_id='))return response({groups:[{marker_position:1,ply_group:1,part_count:1}],groups_next_cursor:'1/1',lays:[],bundles:[]});
 if(url.includes('lay_cursor='))return response({lays:[{lay_id:'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa',order:'PO-B',cut:2,marker:'M2',shade:'B'}],lays_next_cursor:'',groups:[],bundles:[]});
 return response({lays:[{lay_id:layId,order:'PO-A',cut:1,marker:'M1',shade:'A'}],lays_next_cursor:'opaque-lay-cursor',groups:[],bundles:[]});
}
const document={getElementById:id=>{assert.ok(nodes[id],`DOM node ${id}`);return nodes[id]},createElement:()=>new El()};
const window={events:{},addEventListener(k,cb){this.events[k]=cb}};
let serial=0;
const crypto={randomUUID:()=>`12345678-1234-1234-1234-${String(++serial).padStart(12,'0')}`};
const context=vm.createContext({document,window,fetch,crypto,localStorage,URLSearchParams,Set,Map,Promise,JSON,Number,Array,Date,String,Math,console,encodeURIComponent});
vm.runInContext(fs.readFileSync(path.join(__dirname,'../services/edge-daemon/internal/web/static/sewing_execution.js'),'utf8'),context);
const wait=async()=>{for(let i=0;i<40;i++)await Promise.resolve()};
(async()=>{
 await wait();
 assert.equal(nodes.plant.value,'','no custom factory identity');
 assert.equal(nodes.machine.value,'','no predefined machine');
 assert.equal(nodes.moreLays.hidden,false,'lay cursor exposed');
 await nodes.moreLays.fire('click');await wait();
 assert.match(nodes.lay.innerHTML,/PO-B/,'paged lays appended');
 nodes.lay.value=layId;
 nodes.plant.value='PLANT_ALPHA';nodes.station.value='STATION_901';nodes.worker.value='DEMO_WORKER_3';nodes.machine.value='GENERIC_CLASS';
 await nodes.lay.fire('change');await wait();
 assert.equal(nodes.moreGroups.hidden,false,'group cursor exposed');
 await nodes.moreGroups.fire('click');await wait();
 assert.match(nodes.group.innerHTML,/Position 2/,'next bundle group page appended');
 nodes.group.value='1/1';nodes.op.value=layId;await nodes.group.fire('change');await wait();
 assert.match(nodes.scanButtons.innerHTML,/CB08-AAAA/);
 assert.match(nodes.historyScope.textContent,/more history exists/,'partial recent history labelled');
 assert.equal(nodes.total.textContent,444,'aggregate from server, not capped list');
 assert.equal(nodes.claim.disabled,true,'no confirmation before scan');
 assert.equal(nodes.scanButtons.buttons.length,1);
 await nodes.scanButtons.buttons[0].fire('click');await wait();
 assert.match(nodes.netState.textContent,/Offline/);
 assert.match(nodes.scanButtons.innerHTML,/NOT SCANNED/,'offline click is not acknowledged');
 assert.equal(Number(nodes.pending.textContent),1);
 nodes.station.value='OTHER_STATION';await nodes.station.fire('change');await wait();
 assert.equal(Number(nodes.pending.textContent),0,'other station has independent queue');
 assert.equal(postCount,1,'no unsafe replay');
 nodes.station.value='STATION_901';await nodes.station.fire('change');await wait();
 assert.equal(Number(nodes.pending.textContent),0,'exact event replayed at original station');
 assert.match(nodes.scanButtons.innerHTML,/UI SCAN ACK/);
 assert.equal(nodes.claim.disabled,false);
 // Status response for the old actor must not overwrite the new context.
 statusHold=[];
 const old=nodes.refresh.fire('click');await wait();
 const oldCallbacks=statusHold.splice(0);
 nodes.worker.value='DEMO_WORKER_4';await nodes.worker.fire('change');await wait();
 for(const reply of oldCallbacks)reply(response({cycles:[],total_simulated_operation_units:999999,cycles_has_more:false}));
 await old;await wait();
 assert.notEqual(nodes.total.textContent,999999,'stale status suppressed');
 for(const reply of statusHold)reply(response({cycles:[],total_simulated_operation_units:444,cycles_has_more:true}));
 console.log('PASS Chapter 06: dynamic identity, keyset pages, selected-group loading, truthful totals, station-scoped retry, stale request isolation');
})().catch(e=>{console.error(e);process.exitCode=1});
