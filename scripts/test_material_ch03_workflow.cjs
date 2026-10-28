// Author: Sunil Jain, 2026-10-28 — Exercise Chapter 03 PO isolation, preview fingerprints,
// uncertain stock economics and stale-response protection with a mock browser.
'use strict';
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');
const script = fs.readFileSync(path.join(__dirname, '../services/edge-daemon/internal/web/static/material_planning.js'), 'utf8');
const HASH = 'b'.repeat(64);
class Element {
  constructor(tag='div') {this.tagName=tag;this.children=[];this.events={};this._text='';this._value='';this.files=[];this.hidden=false;this.disabled=false;this.className='';}
  set value(v){this._value=String(v)} get value(){return this._value}
  set textContent(v){this.children=[];this._text=String(v)} get textContent(){return this._text+this.children.map(x=>x.textContent).join('')}
  append(...items){this.children.push(...items)} replaceChildren(...items){this.children=[];this._text='';this.append(...items)}
  addEventListener(event,fn){(this.events[event]??=[]).push(fn)}
  fire(event){for(const fn of this.events[event]??[])fn({preventDefault(){}})}
  click(){this.fire('click')}
}
const ids=['message','intake','order-summary','results','gate','scenario-subtitle','need-state','need-sub','have-state','have-sub','short-state','short-sub','material-lines','gross-cost','reuse-cost','net-cost','currency','hash','scenario-title','candidates','holds','history','scenario-form','stock','prices','stage','preview','template-stock','template-prices','refresh'];
const makeOrder = id=>({id,po_number:id,style_code:'STYLE-1',ordered_qty:200,status:'PROPOSED',materials:[{material_sku:'COTTON',category:'FABRIC',required:'400.000',uom:'M'}]});
const makeReport = id=>({intake_id:id,po_number:id,style_code:'STYLE-1',ordered_qty:200,scenario_sha256:HASH,materials:[{material_sku:'COTTON',category:'FABRIC',uom:'M',gross:'400.000',conditional_surplus:'50.000',net_indent:'350.000',buyer_supply_gap:'0.000',procurement_owner:'FACTORY',unit_price:'4.2500',price_source:'QUOTE-10',net_cost:'1487.5000'}],gross_material_cost:'1700.0000',conditional_reuse_value:'212.5000',net_purchase_exposure:'1487.5000',currency:'USD',candidates:[],holds:['Check ownership']});
async function tick(){for(let i=0;i<16;i++) await Promise.resolve()}
async function run(search=''){
  const nodes=Object.fromEntries(ids.map(id=>[id,new Element()]));
  let forms=[],requests=[],deferred=[],slowOrder=false,slowHistory=false,stagingConflict=false;
  const urlFor=id=>'/api/v1/order-intake?id='+id;
  const fetch=async(url,options={})=>{
    requests.push({url,options});
    if(url==='/api/v1/order-intake')return {ok:true,json:async()=>({items:[makeOrder('OI-A'),makeOrder('OI-B')]})};
    if(url.startsWith('/api/v1/order-intake?id=')){
      if(slowOrder)return new Promise(resolve=>deferred.push({url,resolve}));
      return {ok:true,json:async()=>makeOrder(url.split('=')[1])};
    }
    if(url.startsWith('/api/v1/material-scenario?intake=')){
      if(slowHistory)return new Promise(resolve=>deferred.push({url,resolve}));
      return {ok:true,json:async()=>({items:[{id:'MS-1',created_at:'2026-10-28T19:00:00Z'}]})};
    }
    if(url.startsWith('/api/v1/material-scenario?id='))return {ok:true,json:async()=>makeReport(nodes.intake.value)};
    if(url.includes('action=stage') && stagingConflict)return {ok:false,status:409,text:async()=> 'Preview no longer matches the uploaded sheets.'};
    if(url.includes('action=')){
      forms.push({url,fields:new Map(options.body.fields)});
      return {ok:true,json:async()=>makeReport(options.body.fields.find(([k])=>k==='intake_id')[1])};
    }
    throw Error('Unexpected request '+url);
  };
  class FormData {constructor(){this.fields=[]} append(k,v){this.fields.push([k,v])}}
  const document={getElementById:id=>{assert.ok(nodes[id],`unknown DOM id: ${id}`);return nodes[id]},createElement:tag=>new Element(tag)};
  vm.runInNewContext(script,{document,fetch,FormData,URLSearchParams,location:{search},encodeURIComponent,Date,Number,Array,String,Error,console}, {filename:'material_planning.js'});
  await tick();
  return {nodes,forms,requests,deferred,tick,setSlowOrder:x=>slowOrder=x,setSlowHistory:x=>slowHistory=x,setConflict:x=>stagingConflict=x};
}
(async()=>{
  // No arbitrary default PO selection: avoid issuing a plan for the wrong buyer.
  const blank=await run();
  assert.equal(blank.nodes.intake.value,'');
  assert.match(blank.nodes['order-summary'].textContent,/Choose the exact/);
  assert.equal(blank.nodes.stage.disabled,false,'static mock starts enabled; handlers must disable on first state change');
  const session=await run('?intake=OI-A');
  const {nodes,forms,requests}=session;
  assert.equal(nodes.intake.value,'OI-A');
  assert.match(nodes['order-summary'].textContent,/OI-A/);
  const stock={name:'stock.xlsx'},prices={name:'rates.xlsx'};
  nodes.stock.files=[stock];nodes.prices.files=[prices];
  nodes['scenario-form'].fire('submit');await tick();
  assert.equal(nodes.stage.disabled,false,'valid preview should enable saving');
  assert.match(nodes['have-state'].textContent,/Claims only/);
  assert.match(nodes['short-state'].textContent,/1 factory \/ 0 buyer/);
  assert.equal(nodes['material-lines'].children[0].children[1].children.at(-1).textContent,'Possible factory cost: 1487.5000');
  nodes.stage.click();await tick();
  assert.equal(forms.length,2);
  assert.equal(forms[0].fields.has('preview_sha256'),false,'preview does not need hash');
  assert.equal(forms[1].fields.get('preview_sha256'),HASH,'stage must bind exact preview');
  assert.equal(nodes.stage.disabled,true,'stage consumed fingerprint');
  // File change removes prior preview authority, and must not silently stage.
  nodes['scenario-form'].fire('submit');await tick();
  nodes.stock.files=[{name:'new-stock.xlsx'}];nodes.stock.fire('change');
  assert.equal(nodes.stage.disabled,true);
  nodes.stage.click();await tick();assert.equal(forms.length,3,'file edit cannot stage an old preview');
  // Server detects altered evidence even if browser File object remains the same.
  nodes['scenario-form'].fire('submit');await tick();
  session.setConflict(true);nodes.stage.click();await tick();
  assert.equal(nodes.stage.disabled,true);
  assert.match(nodes.message.textContent,/no longer matches/i);
  // History inspection also invalidates a previous save-capable preview.
  session.setConflict(false);nodes['scenario-form'].fire('submit');await tick();
  assert.equal(nodes.stage.disabled,false);
  assert.ok(nodes.history.children.length>0);
  nodes.history.children[0].click();await tick();assert.equal(nodes.stage.disabled,true);
  // PO switching must never render the previous PO's response.
  session.setSlowOrder(true);
  nodes.intake.value='OI-B';nodes.intake.fire('change');
  nodes.intake.value='OI-A';nodes.intake.fire('change');
  assert.equal(session.deferred.length,2);
  session.deferred[1].resolve({ok:true,json:async()=>makeOrder('OI-A')});await tick();
  session.deferred[0].resolve({ok:true,json:async()=>makeOrder('OI-B')});await tick();
  assert.match(nodes['order-summary'].textContent,/OI-A/);
  assert.equal(nodes.stage.disabled,true);
  console.log('PASS Chapter 03: explicit PO, evidence fingerprint, factory gap, invalidation, conflict, history and order race');
})().catch(error=>{console.error(error);process.exitCode=1});
