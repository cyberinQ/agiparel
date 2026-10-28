// Author: Sunil Jain, 2026-10-28. Simulated Chapter 09 DOM/API user journey without browser APIs or production data.
'use strict';
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');
const root = path.join(__dirname, '../services/edge-daemon/internal/web/static');
const elements = new Map();
function elt(id='') {
  return {id, value:'', textContent:'', disabled:false, hidden:true, children:[],listeners:{},
    appendChild(node){this.children.push(node);}, append(...nodes){this.children.push(...nodes);},
    replaceChildren(...nodes){this.children=[...nodes];},
    addEventListener(kind, handler){this.listeners[kind]=handler;},
    closest(){return {querySelectorAll(){return new Array(8).fill({})}};},
    click(){this.listeners.click?.()},focus(){this.focused=true;},
    dispatchEvent(ev){this.dispatched=ev.type;}
  };
}
const ids=['lay','timezone','status','refresh','capture','ask-aggie','stages','residuals','intervals','operations','alerts','snapshots','interval-exclusions','method','evidence','metric-sewn','metric-accepted','metric-hold','metric-packed','aggie-open','aggie-chat','aggie-input'];
for(const id of ids) elements.set(id,elt(id));
elements.get('timezone').value='Asia/Kolkata';
const layID='52d160cd-980d-41d5-bc0e-909a546b9461';
const result={lay_id:layID,authority:'REHEARSAL_ONLY_NOT_PRODUCTION_CLOSURE',evidence:'DERIVED_FROM_UI_SIMULATED_NO_RF_OBSERVATION',generated_at:'2026-10-28T01:00:00Z',
  stages:[{name:'Sewing completed',count:41,unit:'simulated garment serials',source:'sew_sim_credits_v9'},{name:'QC accepted',count:38},{name:'QC held/reinspect',count:2},{name:'Currently packed',count:17}],
  residuals:[{from:'sewn',to:'qc_enrolled',unaccounted:1,status:'OPEN_WIP_OR_UNKNOWN_LOSS',explanation:'May be WIP'}],
  alerts:['Missing cut verification'],interval_stats:{'sewn->qc':{sample_count:3,unknown_count:2,p50_minutes:5,p90_minutes:8,p95_minutes:9,cv:.25}},
  operation_stats:[],interval_exclusions:{'sewn->qc':{'CROSSES_FACTORY_DAY':2}},methodology:['No fabricated scans.']};
let saved=0;
const context={
  document:{getElementById:id=>elements.get(id),createElement:()=>elt()},
  fetch:async url=>{
    if(String(url).includes('catalog')) return {ok:true,json:async()=>({lays:[{lay_id:layID,intake_id:'REP3',cut_no:1,revision:1}]})};
    if(String(url).includes('report?')) return {ok:true,json:async()=>result};
    if(String(url).includes('snapshots?')) return {ok:true,json:async()=>({snapshots:[]})};
    if(String(url).includes('capture')) {saved++; return {ok:true,json:async()=>({capture_id:'intel_123'})};}
    throw Error('Unexpected endpoint '+url);
  },
  crypto:{randomUUID:()=> '00000000-0000-0000-0000-000000000000'},
  Event:class{constructor(type){this.type=type}},
  globalThis:null,
};
context.globalThis=context;
const js=name=>fs.readFileSync(path.join(root,name),'utf8');
vm.runInNewContext(js('factory_report_metrics.js'),context);
vm.runInNewContext(js('factory_intelligence.js'),context);
const flush=()=>new Promise(resolve=>setImmediate(resolve));
(async()=>{
  await flush();
  assert.equal(elements.get('lay').children.length,2);
  assert.equal(elements.get('capture').disabled,true);
  elements.get('lay').value=layID;
  elements.get('refresh').listeners.click();
  await flush();await flush();
  assert.equal(elements.get('metric-sewn').textContent,'41');
  assert.equal(elements.get('metric-accepted').textContent,'38');
  assert.equal(elements.get('capture').disabled,false);
  assert.equal(elements.get('alerts').children.length,2);
  assert.ok(elements.get('interval-exclusions').textContent.includes('CROSSES_FACTORY_DAY'));
  elements.get('aggie-chat').hidden=false;
  elements.get('aggie-input').value='Unfinished previous question';
  elements.get('ask-aggie').listeners.click();
  assert.equal(elements.get('aggie-input').value,'Unfinished previous question');
  assert.ok(elements.get('status').textContent.includes('unfinished message'));
  elements.get('aggie-input').value='';
  elements.get('ask-aggie').listeners.click();
  assert.ok(elements.get('aggie-input').value.includes('PRACTICE-ONLY'));
  assert.equal(elements.get('aggie-input').dispatched,'input');
  elements.get('capture').listeners.click();
  await flush();await flush();
  assert.equal(saved,1);
  assert.ok(elements.get('status').textContent.includes('Practice report saved'));
  elements.get('lay').value='';
  elements.get('lay').listeners.change();
  assert.equal(elements.get('metric-sewn').textContent,'—');
  assert.equal(elements.get('capture').disabled,true);
  console.log('PASS Chapter 09: catalog, real read-only report, uncertainty, Aggie question safety, immutable save and reset');
})().catch(error=>{console.error(error);process.exitCode=1});
