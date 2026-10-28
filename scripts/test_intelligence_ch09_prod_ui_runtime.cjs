// Author: Sunil Jain, 2026-10-28.
// Runtime tests for Chapter 09: preserve unknowns, reject malformed authority, and scope signed reads.
'use strict';
const vm=require('node:vm');
const fs=require('node:fs');
const assert=require('node:assert/strict');
const ids=['since','until','token','authmode','oidcscope','tenant','plant','refresh','clear','status','scope','measures','notes','sew','qc','held','stage'];
const nodes=Object.fromEntries(ids.map(id=>[id,{
 value:'',textContent:'',children:[],events:{},
 addEventListener(type,fn){this.events[type]=fn;},
 replaceChildren(){this.children=[];},append(node){this.children.push(node);}
}]));
let payload=null, request=null;
const document={getElementById:id=>nodes[id],createElement:tag=>({tag,textContent:'',children:[],append(node){this.children.push(node);}})};
const ctx={document,Date,Number,Object,URLSearchParams,Error,console,
 fetch: async (url,options)=>{
  request={url,options};return {ok:true,json:async()=>payload};
 }};
vm.runInNewContext(fs.readFileSync('services/edge-daemon/internal/web/static/factory_intelligence_production.js','utf8'),ctx);
const value=(id,n)=>({id,value:n,status:'OBSERVED_GROUP_EVENT',unit:'groups',source:'sew_prod_completions_v10',meaning:'Group evidence'});
const missing=id=>({id,value:null,status:'NOT_MEASURED',unit:'groups',source:'not_available',meaning:'No accepted unit proof'});
const valid={authority:'AUTHENTICATED_GROUP_EVENTS_NOT_GARMENT_RELEASE',production_release:false,
 plant_id:'factory_A',tenant_id:'tenant_A',as_of:'2026-10-28T19:00:00Z',basis:'Current state',limitations:['Group counts do not represent garment output'],metrics:[
 value('sewing_completion_groups',4),value('qc_current_reviewed_groups',2),value('qc_current_hold_groups',1),value('packing_current_staging_holds',0),missing('verified_garments')
 ]};
async function main(){
 nodes.authmode.value='pilot';nodes.since.value='2026-10-27T00:00';nodes.until.value='2026-10-28T00:00';nodes.token.value='signed-short-lived-access';payload=valid;
 await nodes.refresh.events.click();
 assert.equal(nodes.sew.textContent,'4');assert.equal(nodes.stage.textContent,'0');
 assert(nodes.measures.children.some(row=>row.children[1].textContent==='Not measured'));
 assert.equal(request.options.credentials,'omit');
 assert.equal(request.options.headers.Authorization,'Bearer signed-short-lived-access');
 assert(request.url.includes('/api/v1/intelligence/production/report?'));
 payload={...valid,metrics:[...valid.metrics.slice(0,4),value('verified_garments',20)]};
 await nodes.refresh.events.click();
 assert.equal(nodes.sew.textContent,'—','unexpected garment authority must not leave earlier counts');
 assert.match(nodes.status.textContent,/Unable to load/);
 payload={...valid,production_release:true};
 await nodes.refresh.events.click();assert.equal(nodes.stage.textContent,'—');
 nodes.authmode.value='oidc';nodes.authmode.events.change();
 assert.equal(nodes.oidcscope.hidden,false);
 nodes.tenant.value='tenant_A';nodes.plant.value='factory_A';
 payload=valid;await nodes.refresh.events.click();
 assert(request.url.includes('tenant_id=tenant_A') && request.url.includes('plant_id=factory_A'));
 assert.equal(nodes.sew.textContent,'4');
 nodes.plant.value='';await nodes.refresh.events.click();assert.match(nodes.status.textContent,/authorized tenant and plant/);
 nodes.clear.events.click();assert.equal(nodes.token.value,'');
 console.log('PASS Chapter 09 UI: auth scope, zero vs unknown, fail-closed authority, ephemeral token');
}
main().catch(e=>{console.error(e);process.exitCode=1;});
