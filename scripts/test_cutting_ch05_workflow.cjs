// Author: Sunil Jain, 2026-10-28
// Chapter 05 browser regression: capped drafts, safe revisions, retry keys, evidence isolation and mm checks.
'use strict';
const assert=require('node:assert/strict');
const fs=require('node:fs');
const path=require('node:path');
const vm=require('node:vm');
const base=path.join(__dirname,'../services/edge-daemon/internal/web/static');
class El {
 constructor(tag='div'){this.tagName=tag;this.children=[];this.events={};this._text='';this.value='';this.disabled=false;this.className='';this.style={};this.classList={toggle(){}};this.options=this.children;}
 get textContent(){return this._text+this.children.map(c=>c.textContent||'').join('');}
 set textContent(v){this._text=String(v);this.children=[];this.options=this.children;}
 append(...v){this.children.push(...v);this.options=this.children;}
 replaceChildren(...v){this._text='';this.children=[];this.append(...v);}
 addEventListener(n,fn){(this.events[n]??=[]).push(fn);}
 fire(n){return Promise.all((this.events[n]||[]).map(fn=>fn({preventDefault(){},currentTarget:this})));}
}
const ids=['cut-message','cut-form','cut-order','cut-source-facts','metric-lays','metric-garments','metric-bundles','metric-cut-release','cut-register-scope','cut-register','cut-title','cut-summary','cut-bundles','revise-cut','print-cut','refresh-cut','save-cut','cut-planned-consumption',
 'spread-form','spread-selection','spread-issues','spread-register','spread-scans','save-spread','spread-message','spread-check','cut-release-evidence','release-refresh'];
const nodes=Object.fromEntries(ids.map(id=>[id,new El(id==='cut-order'?'select':'div')]));
const cutFields=['intake_id','cut_no','expected_revision','marker_code','fabric_sku','shade_band','reported_plies','bundle_ply_group_size','marker_length_mm','usable_width_mm','positions','parts','note'];
const spreadFields=['actual_plies','measured_marker_length_mm','reported_consumption_mm','issue_allocations','evidence_note'];
function form(fields){const byName=Object.fromEntries(fields.map(n=>[n,new El()]));const el=new El('form');el.elements={namedItem:n=>byName[n]};el.byName=byName;return el;}
const cf=nodes['cut-form']=form(cutFields),sf=nodes['spread-form']=form(spreadFields);
for(const [name,value] of Object.entries({intake_id:'OI-A',cut_no:'1',expected_revision:'0',marker_code:'MARK-1',fabric_sku:'FAB-1',shade_band:'A',reported_plies:'20',bundle_ply_group_size:'20',marker_length_mm:'5000',usable_width_mm:'1400',positions:'1,BLACK,M',parts:'FRONT,2',note:'Cut quantities reviewed against buyer order.'}))cf.byName[name].value=value;
for(const [name,value] of Object.entries({actual_plies:'20',measured_marker_length_mm:'5000',reported_consumption_mm:'101000',issue_allocations:'123e4567-e89b-12d3-a456-426614174001,101000',evidence_note:'Physical marker and spread measured by cutting operator.'}))sf.byName[name].value=value;
const handlers={};let selectedEvents=0,clearedEvents=0;
const document={getElementById:id=>{assert.ok(nodes[id],`missing mock id ${id}`);return nodes[id];},createElement:tag=>new El(tag),addEventListener:(n,fn)=>{(handlers[n]??=[]).push(fn);},dispatchEvent:event=>{
 if(event.type==='cut-draft-selected')selectedEvents++;
 if(event.type==='cut-draft-cleared')clearedEvents++;
 for(const fn of handlers[event.type]??[])fn(event);
}};
class FormData {
 constructor(el){this.el=el;}
 get(n){return this.el.byName[n]?.value??'';}
}
class Option extends El{constructor(text,value){super('option');this.textContent=text;this.value=value;}}
const uuid='123e4567-e89b-12d3-a456-426614174001';
const bundle={label_code:'CB08-AAAAAAAAAAAAAAAAAAAAAAAA',position:1,color_code:'BLACK',size_code:'M',part_code:'FRONT',pieces_per_garment:2,ply_group:1,ply_start:1,ply_end:20,piece_count:40,rfid_state:'UNBOUND_DRAFT_LABEL'};
const report={draft_id:'123e4567-e89b-12d3-a456-426614174000',intake_id:'OI-A',po_number:'PO-A',cut_no:1,revision:2,marker_code:'MARK-1',fabric_sku:'FAB-1',shade_band:'A',reported_plies:20,bundle_ply_group_size:20,marker_length_mm:5000,usable_width_mm:1400,planned_garments:20,bundle_count:1,planned_parts:40,position_count:1,verified_issue_count:1,verified_issued_length_mm:150000,state:'PROPOSED_HOLD',note:'Marker reviewed against buyer requirements.',bundles:[bundle]};
const order=(id)=>({intake_id:id,po_number:id,style_code:'S1',ordered_qty:100,status:'PROPOSED'});
let releaseRequests=[];
let cutRequests=[];
let randomCounter=0;
function response(data,ok=true){return {ok,status:ok?200:409,text:async()=>JSON.stringify(data),json:async()=>data};}
async function fetch(url,options={}){
 if(url.startsWith('/api/v1/cutting/release-status'))return new Promise(resolve=>releaseRequests.push({url,resolve}));
 if(url.startsWith('/api/v1/cutting/spreads'))return response({spreads:[],matching_signed_issues:[{issue_id:uuid,roll_id:'ROLL-1',issued_mm:150000,already_attributed_mm:0,available_for_attribution_mm:150000}],bundle_witnesses:[]});
 if(url.includes('?intake_id='))return response({skus:[{color_code:'BLACK',size_code:'M',ordered_qty:100,planned_qty:20,remaining_qty:80}],fabric_skus:['FAB-1']});
 if(url.includes('?draft_id='))return response({drafts:[report]});
 if(options.method==='POST'){cutRequests.push(JSON.parse(options.body));throw Error('simulated network timeout');}
 return response({orders:[order('OI-A'),order('OI-B')],drafts:[report],has_more:true});
}
const context=vm.createContext({document,fetch,Option,FormData,Uint8Array,crypto:{getRandomValues:a=>a.fill(++randomCounter)},CustomEvent:class{constructor(type,opts={}){this.type=type;this.detail=opts.detail;}},window:{print(){}},Number,String,Array,Map,Set,Promise,JSON,Error,Math,console,encodeURIComponent});
const load=name=>vm.runInContext(fs.readFileSync(path.join(base,name),'utf8'),context,{filename:name});
const wait=async()=>{for(let i=0;i<40;i++)await Promise.resolve();};
async function main(){
 load('cutting_lay.js');load('cutting_spread.js');load('cutting_release_status.js');await wait();
 assert.equal(nodes['metric-lays'].textContent,'1+','backend cap must be visible');
 assert.match(nodes['cut-register-scope'].textContent,/60 most recent/);
 assert.match(nodes['cut-planned-consumption'].textContent,/100 m/);
 await nodes['cut-register'].children[0].fire('click');await wait();
 assert.equal(selectedEvents,1);assert.equal(nodes['revise-cut'].disabled,false);
 assert.match(nodes['spread-selection'].textContent,/PO-A/);
 assert.equal(nodes['save-spread'].disabled,false);
 assert.ok(releaseRequests.length>=1);
 await nodes['revise-cut'].fire('click');await wait();
 assert.equal(cf.byName.expected_revision.value,'2','revision should use current saved revision');
 assert.equal(cf.byName.positions.value,'1,BLACK,M');
 assert.equal(cf.byName.parts.value,'FRONT,2');
 assert.equal(nodes['save-spread'].disabled,true,'switch to edit mode clears evidence attribution');
 const first=vm.runInContext("cutReadInput(cutEl('cut-form'))",context);
 const retry=vm.runInContext("cutReadInput(cutEl('cut-form'))",context);
 assert.equal(first.request_key,retry.request_key,'unchanged retry must reuse key');
 cf.byName.note.value+=' Revision required';
 const revised=vm.runInContext("cutReadInput(cutEl('cut-form'))",context);
 assert.notEqual(revised.request_key,first.request_key,'modified draft needs new key');
 cf.byName.note.value='Marker checked after change.';
 await nodes['cut-form'].fire('submit');await wait();
 assert.equal(cutRequests.length,1);
 await nodes['cut-form'].fire('submit');await wait();
 assert.equal(cutRequests[0].request_key,cutRequests[1].request_key,'failed save must retain idempotency key');
 // Resolve the stale release status after the revision/selection has been cleared.
 releaseRequests[0].resolve(response({released:true,expected_bundles:1}));await wait();
 assert.equal(nodes['metric-cut-release'].textContent,'NOT RELEASED','stale request must not claim release');
 assert.equal(clearedEvents,1);
 // Reopen the draft: test exact signed issue accounting before submission.
 await nodes['cut-register'].children[0].fire('click');await wait();
 sf.byName.actual_plies.value='20';sf.byName.measured_marker_length_mm.value='5000';
 sf.byName.reported_consumption_mm.value='100999';sf.byName.issue_allocations.value=`${uuid},100999`;
 const under=vm.runInContext('spreadReadInput(spreadEl("spread-form"))',context);
 assert.equal(under.reported_consumption_mm,100999);
 sf.byName.reported_consumption_mm.value='99999';sf.byName.issue_allocations.value=`${uuid},99999`;
 assert.throws(()=>vm.runInContext('spreadReadInput(spreadEl("spread-form"))',context),/below measured marker/);
 sf.byName.reported_consumption_mm.value='151000';sf.byName.issue_allocations.value=`${uuid},151000`;
 assert.throws(()=>vm.runInContext('spreadReadInput(spreadEl("spread-form"))',context),/insufficient unattributed/);
 sf.byName.reported_consumption_mm.value='101000';sf.byName.issue_allocations.value=`${uuid},101000`;
 const valid=vm.runInContext('spreadReadInput(spreadEl("spread-form"))',context);
 const same=vm.runInContext('spreadReadInput(spreadEl("spread-form"))',context);
 assert.equal(valid.request_key,same.request_key);
 nodes['cut-order'].value='OI-B';await nodes['cut-order'].fire('change');await wait();
 assert.equal(nodes['save-spread'].disabled,true);
 assert.match(nodes['spread-selection'].textContent,/Select a cutting draft/);
 assert.equal(nodes['cut-title'].textContent,'Choose or save a lay to inspect its labels.');
 const last=releaseRequests.at(-1);last.resolve(response({released:true,expected_bundles:1}));await wait();
 assert.equal(nodes['metric-cut-release'].textContent,'NOT RELEASED');
 console.log('PASS Chapter 05: capped plans, revision sourcing, draft key reuse, signed mm checks, stale release and order isolation');
}
main().catch(error=>{console.error(error);process.exitCode=1;});
