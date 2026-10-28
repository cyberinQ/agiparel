// Author: Sunil Jain, 2026-10-28 — Exercise Chapter 02 source locking,
// independent PO edits, row validation and protected staging without a browser.
'use strict';
const assert=require('node:assert/strict');
const fs=require('node:fs');
const vm=require('node:vm');
const path=require('node:path');
const src=fs.readFileSync(path.join(__dirname,'../services/edge-daemon/internal/web/static/buyer_review.js'),'utf8');

class Element {
 constructor(tag='div') {this.tagName=tag.toLowerCase();this.children=[];this.dataset={};this.listeners={};this.style={};this.attributes={};this.hidden=false;this.value='';this.checked=false;this.disabled=false;this._text='';}
 set value(value){this._value=String(value)}
 get value(){return this._value}
 set textContent(value){this.replaceChildren();this._text=String(value)}
 get textContent(){return this._text+this.children.map(x=>x.textContent).join('')}
 set className(v){this._className=v}
 get className(){return this._className||''}
 append(...items){for(const child of items){this.children.push(child);child.parentElement=this}}
 replaceChildren(...items){this.children=[];this._text='';this.append(...items)}
 remove(){if(this.parentElement)this.parentElement.children=this.parentElement.children.filter(x=>x!==this)}
 setAttribute(name,value){this.attributes[name]=String(value)}
 removeAttribute(name){delete this.attributes[name]}
 focus(){} scrollIntoView(){}
 addEventListener(name,fn){(this.listeners[name]??=[]).push(fn)}
 dispatch(name,target=this){for(const fn of this.listeners[name]||[])fn({target})}
 closest(selector){let e=this;while(e){if(e.matches(selector))return e;e=e.parentElement}return null}
 matches(selector){if(selector==='tr')return this.tagName==='tr';if(selector==='[data-field]')return Object.hasOwn(this.dataset,'field');const m=/^\[data-field="([^"]+)"\]$/.exec(selector);return !!m&&this.dataset.field===m[1]}
 querySelectorAll(selector){const result=[];const walk=x=>{for(const c of x.children){if(c.matches(selector))result.push(c);walk(c)}};walk(this);return result}
 querySelector(selector){return this.querySelectorAll(selector)[0]??null}
}

async function suite(){
 const ids=['po-source','bom-source','load-message','analyze','review','result','po-choice','multiple-po','po-number','style-code','sku-rows','bom-rows','review-note','acknowledged','coverage','order-summary','order-checks','issues','techpack-workbench','techpack-candidates','techpack-summary','po-evidence','bom-evidence','po-evidence-title','bom-evidence-title','stage-message','stage','result-title','result-message','add-sku','add-material'];
 const nodes=Object.fromEntries(ids.map(id=>[id,new Element()]));
 nodes.review.hidden=true;nodes.result.hidden=true;
 const doc={getElementById:id=>{assert.ok(nodes[id],`unknown DOM ID ${id}`);return nodes[id]},createElement:tag=>new Element(tag)};
 let posts=0, resolves=[];
 const getData={
  po_source:{id:'DOC-A',filename:'order.pdf',segments:[]},
  bom_source:{id:'DOC-B',filename:'techpack.xlsx',segments:[]},
  po_options:[
   {po_number:'PO-01',style_code:'STYLE-A',skus:[{color:'BLUE',size:'M',quantity:10,source_ordinal:2}]},
   {po_number:'PO-02',style_code:'STYLE-B',skus:[{color:'RED',size:'L',quantity:20,source_ordinal:3}]}
  ],
  materials:[
   {style_code:'STYLE-A',material_sku:'CLOTH-A',category:'FABRIC',per_garment:'1.850',uom:'M',source_ordinal:4},
   {style_code:'STYLE-B',material_sku:'CLOTH-B',category:'FABRIC',per_garment:'2.250',uom:'M',source_ordinal:5}
  ],issues:[],techpack_material_hints:[]
 };
 let deferNext=false;
 const fetch=async(url,opts={})=>{
  if(url==='/api/v1/documents')return {ok:true,json:async()=>({items:[{id:'DOC-A',filename:'order.pdf',status:'EXTRACTED'},{id:'DOC-B',filename:'techpack.xlsx',status:'EXTRACTED'}]})};
  if(url.startsWith('/api/v1/buyer-review') && !opts.method){
   if(deferNext){deferNext=false;return new Promise(resolve=>resolves.push(()=>resolve({ok:true,json:async()=>getData})))}
   return {ok:true,json:async()=>getData};
  }
  if(opts.method==='POST'){
   posts++;
   return {ok:true,json:async()=>({intake:{po_number:'PO-01',ordered_qty:10},reused:false})};
  }
  throw Error(`unexpected fetch ${url}`);
 };
 vm.runInNewContext(src,{document:doc,fetch,encodeURIComponent,console,BigInt,Number,Array,String,Set,Map,Error}, {filename:'buyer_review.js'});
 const tick=async()=>{for(let i=0;i<8;i++)await Promise.resolve()};
 await tick();
 nodes['po-source'].value='DOC-A';nodes['bom-source'].value='DOC-B';
 nodes.analyze.dispatch('click');await tick();
 assert.equal(nodes.review.hidden,false);
 assert.match(nodes['order-summary'].textContent,/10 pieces/);
 assert.equal(nodes['bom-rows'].children[0].querySelector('[data-field="material_sku"]').value,'CLOTH-A');
 // Multi-PO editing cannot delete a supervisor's corrected values.
 nodes['sku-rows'].children[0].querySelector('[data-field="quantity"]').value='15';
 nodes['bom-rows'].children[0].querySelector('[data-field="per_garment"]').value='1.900';
 nodes['review-note'].value='Checked corrected blue PO';nodes.acknowledged.checked=true;
 nodes['po-choice'].value='1';nodes['po-choice'].dispatch('change');
 assert.equal(nodes['po-number'].value,'PO-02');
 assert.equal(nodes['bom-rows'].children[0].querySelector('[data-field="material_sku"]').value,'CLOTH-B');
 assert.equal(nodes.acknowledged.checked,false,'acknowledgement must be per PO');
 nodes['po-choice'].value='0';nodes['po-choice'].dispatch('change');
 assert.equal(nodes['sku-rows'].children[0].querySelector('[data-field="quantity"]').value,'15');
 assert.equal(nodes['bom-rows'].children[0].querySelector('[data-field="per_garment"]').value,'1.900');
 assert.equal(nodes['review-note'].value,'Checked corrected blue PO');
 assert.equal(nodes.acknowledged.checked,true);
 // Duplicates are rejected *before* a POST, with actionable error.
 nodes['add-sku'].dispatch('click');
 const duplicate=nodes['sku-rows'].children[1];
 for(const [field,value] of Object.entries({color:'BLUE',size:'M',quantity:'3'}))duplicate.querySelector(`[data-field="${field}"]`).value=value;
 nodes.stage.dispatch('click');await tick();
 assert.equal(posts,0);assert.match(nodes['stage-message'].textContent,/duplicate color\/size/);
 duplicate.remove();
 // A material retained from another style cannot be falsely attributed.
 nodes['style-code'].value='STYLE-OTHER';
 nodes.stage.dispatch('click');await tick();
 assert.equal(posts,0);assert.match(nodes['stage-message'].textContent,/different style/);
 nodes['style-code'].value='STYLE-A';
 // Changing a selected document closes stale review; no staged cross-source lineage.
 nodes['bom-source'].value='DOC-A';nodes['bom-source'].dispatch('change');
 assert.equal(nodes.review.hidden,true);
 nodes.stage.dispatch('click');await tick();assert.equal(posts,0);
 assert.match(nodes['stage-message'].textContent,/Source files changed/);
 // Late analysis must not re-open a review after files change in flight.
 deferNext=true;nodes.analyze.dispatch('click');await tick();
 nodes['po-source'].value='DOC-B';nodes['po-source'].dispatch('change');
 assert.equal(nodes.analyze.disabled,false,'source switch during request must unlock reanalysis');
 for(const resolve of resolves)resolve();await tick();
 assert.equal(nodes.review.hidden,true);
 // Fresh analysis and valid review POST can succeed (no commercial release implied).
 nodes['po-source'].value='DOC-A';nodes['po-source'].dispatch('change');
 nodes['bom-source'].value='DOC-B';nodes['bom-source'].dispatch('change');
 nodes.analyze.dispatch('click');await tick();
 nodes['review-note'].value='Checked source PO and the material schedule';nodes.acknowledged.checked=true;
 nodes.stage.dispatch('click');await tick();
 assert.equal(posts,1);assert.equal(nodes.result.hidden,false);
 console.log('Chapter 02 buyer review behavior: PASS (source locking, multi-PO drafts, duplicates, style scope, staging)');
}
suite().catch(error=>{console.error(error);process.exitCode=1});
