// Author: Sunil Jain, 2026-10-28.
// Exercise real DOM renderer contract, ratios/zeros/unknowns and fail-closed authority.
'use strict';
const assert=require('node:assert/strict'),fs=require('node:fs'),vm=require('node:vm');
const ids=['since','until','token','authmode','oidcscope','tenant','plant','refresh','clear','status','scope','measures','factorymeasures','factoryroutes','factorybasis','notes','sew','qc','held','stage','signin','signout','session-status'];
const nodes=Object.fromEntries(ids.map(id=>[id,{value:'',textContent:'',children:[],events:{},addEventListener(t,f){this.events[t]=f;},replaceChildren(){this.children=[];},append(n){this.children.push(n);}}]));
const document={getElementById:id=>nodes[id],createElement:tag=>({tag,textContent:'',children:[],append(node){this.children.push(node);}})};
let payload;
const ctx={document,Date,Number,Object,URLSearchParams,Error,console,window:{location:{assign:()=>{}}},fetch:async(url)=>{
 if(url==='/api/v1/workforce/session')return {ok:false,json:async()=>({error:'signed out'})};
 return {ok:true,json:async()=>payload};
}};
vm.runInNewContext(fs.readFileSync('services/edge-daemon/internal/web/static/factory_intelligence_production.js','utf8'),ctx);
const counts=[['cut_parts_accepted','parts',8],['cut_parts_rejected','parts',2],['garments_initially_inspected','garments',5],
 ['garments_quality_released','garments',3],['reported_cutting_consumption_mm','millimeters',12000],
 ['wip_completed_handoffs','handoffs',4],['garments_added_to_cartons','garments',3],['cartons_sealed','cartons',1],['manifests_staged','manifests',0]];
const mk=(id,unit,value,status='SIGNED_HUMAN_EVIDENCE')=>({id,unit,value,status,source:'signed_vXX',meaning:'Signed source evidence'});
const fMetrics=counts.map(([id,unit,n])=>mk(id,unit,n,id==='wip_completed_handoffs'?'SIGNED_GATEWAY_AND_HUMAN_EVIDENCE':id==='manifests_staged'?'STAGED_NOT_SHIPPED':'SIGNED_HUMAN_EVIDENCE'));
const rate=(id,n,d)=>({...mk(id,'basis_points',d===0?null:Math.round(10000*n/d),d===0?'NO_DENOMINATOR':'DERIVED_SIGNED_ATTESTATIONS'),numerator:n,denominator:d});
fMetrics.push(rate('cut_part_acceptance_pct',8,10),rate('garment_first_pass_yield_pct',4,5),rate('classified_fabric_use_pct',9,10));
fMetrics.push(mk('wip_handoff_p50_ms','milliseconds',12000,'DERIVED_RECORDED_HANDOFF'),mk('wip_handoff_p95_ms','milliseconds',50000,'DERIVED_RECORDED_HANDOFF'));
for(const id of ['actual_fabric_utilization_pct','oee_pct','labor_efficiency_pct','physically_dispatched_garments'])
 fMetrics.push(mk(id,id==='physically_dispatched_garments'?'garments':'basis_points',null,'NOT_MEASURED'));
const factory={authority:'SIGNED_UNIT_AND_CUSTODY_EVENTS_NOT_PHYSICAL_CERTIFICATION',production_release:false,
 basis:'Independent timestamp cohorts',metrics:fMetrics,limitations:['No calibrated scale verification'],slowest_routes:[
  {route_id:'route-id',from_station:'SEW-A',to_station:'FINISH',completed_handoffs:4,p50_milliseconds:12000,p95_milliseconds:50000,evidence:'SIGNED_CUSTODY_HANDOFF_INTERVAL_NOT_MACHINE_CYCLE_TIME'}]};
const groupMetric=(id,n)=>({id,value:n,unit:'groups',status:'OBSERVED_GROUP_EVENT',meaning:'Group cohort',source:'sew_prod_completions_v10'});
const payloadGroup=['sewing_completion_groups','qc_current_reviewed_groups','qc_current_hold_groups','qc_current_reinspection_groups','qc_unreviewed_groups','packing_current_staging_holds','packing_stale_staging_holds'].map((id,i)=>groupMetric(id,i));
for(const id of ['verified_garments','dispatched_garments','first_pass_yield','labor_efficiency','order_margin']) payloadGroup.push({id,value:null,unit:'groups',status:'NOT_MEASURED',meaning:'Legacy group scope only',source:'legacy'});
payload={authority:'AUTHENTICATED_GROUP_EVENTS_NOT_GARMENT_RELEASE',production_release:false,tenant_id:'tenant_A',plant_id:'plant_A',basis:'Completion groups',as_of:'2026-10-28T20:00:00Z',metrics:payloadGroup,limitations:[],factory_evidence:factory};
async function main(){
 nodes.authmode.value='pilot';nodes.since.value='2026-10-27T00:00';nodes.until.value='2026-10-28T00:00';nodes.token.value='ephemeral-token';
 await nodes.refresh.events.click();
 assert.equal(nodes.factorymeasures.children.length,18);
 assert.equal(nodes.factoryroutes.children.length,1);
 const find=id=>nodes.factorymeasures.children.find(r=>r.children[0].textContent===id);
 assert.equal(find('Garment first-pass yield').children[1].textContent,'80.00%');
 assert.equal(find('OEE').children[1].textContent,'Not measured');
 assert.equal(find('Staged manifests').children[1].textContent,'0');
 assert.equal(find('Handoff interval P95').children[1].textContent,'50.000 s');
 assert.match(nodes.factorybasis.textContent,/No calibrated scale verification/);
 const valid=fMetrics.find(m=>m.id==='oee_pct');
 payload={...payload,factory_evidence:{...factory,metrics:fMetrics.map(m=>m===valid?{...m,status:'DERIVED_SIGNED_ATTESTATIONS',value:9000}:m)}};
 await nodes.refresh.events.click();
 assert.equal(nodes.factorymeasures.children.length,0,'forged OEE must clear prior measured values');
 assert.equal(nodes.sew.textContent,'—','forged factory authority must clear legacy counts too');
 assert.match(nodes.status.textContent,/Unable to load/);
 payload={...payload,factory_evidence:{...factory,metrics:fMetrics.map(m=>m.id==='garment_first_pass_yield_pct'?{...m,status:'NO_DENOMINATOR',value:null,numerator:0,denominator:0}:m)}};
 await nodes.refresh.events.click();
 assert.equal(find('Garment first-pass yield').children[1].textContent,'Not measured');
 payload={...payload,factory_evidence:{...factory,metrics:fMetrics.slice(1)}};
 await nodes.refresh.events.click();
 assert.equal(nodes.factorymeasures.children.length,0,'incomplete data must fail closed');
 console.log('PASS F3.6 UI: rate basis, zero vs unknown, source labels, route timing, forged OEE and incomplete evidence rejected');
}
main().catch(e=>{console.error(e);process.exitCode=1});
