// Author: Sunil Jain, CyberinQ LLC, 2026-10-28.
// F3.8 read-only session-scoped status; no browser storage, untrusted HTML or production writes.
(() => {
'use strict';
const $=id=>document.getElementById(id);
const code=/^[A-Za-z0-9_-]{2,64}$/;
const AUTH='ENROLLMENT_AND_SIGNED_REPORTS_NOT_PHYSICAL_CERTIFICATION';
let cursor='',pending=0,currentScope='',loaded=0;
function clear(){$('stations').replaceChildren();$('more').hidden=true;$('summary').textContent='No station records loaded.';loaded=0;cursor='';}
function cell(tr,value){const td=document.createElement('td');td.textContent=String(value);tr.appendChild(td);}
function validStation(s){
 if(!s||!code.test(s.station_id)||!Array.isArray(s.blockers)||
  !['CONFIGURATION_INCOMPLETE','CONFIGURED_NOT_PHYSICALLY_CERTIFIED'].includes(s.status))return false;
 for(const k of ['enabled_gateways','total_gateways','assigned_workers','enabled_inbound_routes','enabled_outbound_routes'])
  if(!Number.isSafeInteger(s[k])||s[k]<0)return false;
 if(s.enabled_gateways>s.total_gateways)return false;
 if(s.last_signed_scan_received_at!==null&&(typeof s.last_signed_scan_received_at!=='string'||
  !Number.isFinite(Date.parse(s.last_signed_scan_received_at))))return false;
 const expected=[];
 if(s.enabled_gateways===0)expected.push('NO_ENABLED_ENROLLED_GATEWAY');
 if(s.assigned_workers===0)expected.push('NO_ENABLED_WORKER_STATION_ASSIGNMENT');
 if(s.enabled_inbound_routes+s.enabled_outbound_routes===0)expected.push('NO_ENABLED_TRANSFER_ROUTE');
 return JSON.stringify(s.blockers)===JSON.stringify(expected) &&
  s.status===(expected.length?'CONFIGURATION_INCOMPLETE':'CONFIGURED_NOT_PHYSICALLY_CERTIFIED');
}
function validReport(r,tenant,plant,previous){
 if(!r||r.authority!==AUTH||r.tenant_id!==tenant||r.plant_id!==plant||
  !Array.isArray(r.stations)||r.stations.length>100||!Array.isArray(r.limitations)||
  r.limitations.length<3||typeof r.as_of!=='string'||!Number.isFinite(Date.parse(r.as_of))||
  typeof r.next_station!=='string')return false;
 let last=previous;
 for(const s of r.stations){if(!validStation(s)||s.station_id<=last)return false;last=s.station_id;}
 if(r.next_station && (r.stations.length===0||r.next_station!==last))return false;
 return true;
}
async function load(reset){
 const tenant=$('tenant').value.trim(),plant=$('plant').value.trim();
 if(!code.test(tenant)||!code.test(plant)){$('status').textContent='Enter valid tenant and plant IDs.';return;}
 const scope=tenant+'\n'+plant;
 if(reset||scope!==currentScope){clear();currentScope=scope;}
 const serial=++pending;const previous=cursor;
 $('load').disabled=true;$('more').disabled=true;
 $('status').textContent='Checking authenticated plant records…';
 try{
  const q=new URLSearchParams({tenant_id:tenant,plant_id:plant,limit:'50'});if(cursor)q.set('cursor',cursor);
  const res=await fetch('/api/v1/commissioning/stations?'+q.toString(),{method:'GET',credentials:'same-origin',headers:{'Accept':'application/json'},cache:'no-store'});
  if(!res.ok)throw new Error(res.status===401?'Sign in with SSO.':res.status===403?'Your identity is not provisioned for this plant.':'Configuration report unavailable (HTTP '+res.status+').');
  const data=await res.json();if(serial!==pending)return;
  if(!validReport(data,tenant,plant,previous))throw new Error('Invalid commissioning evidence response.');
  for(const s of data.stations){const tr=document.createElement('tr');cell(tr,s.station_id);cell(tr,s.enabled_gateways+' / '+s.total_gateways);
   cell(tr,s.assigned_workers);cell(tr,s.enabled_inbound_routes+' / '+s.enabled_outbound_routes);
   cell(tr,s.last_signed_scan_received_at||'No signed scan received');cell(tr,s.status+' — '+(s.blockers.length?s.blockers.join(', '):'No configuration blockers found'));$('stations').append(tr);}
  cursor=data.next_station;loaded+=data.stations.length;$('more').hidden=!cursor;
  $('summary').textContent=loaded+' station configuration records loaded · '+data.as_of+' · '+AUTH;
  $('status').textContent=loaded===0?'No station configuration records found for this authorized plant.':'Configuration records loaded. No physical certification implied.';
 }catch(err){if(serial===pending){clear();$('status').textContent=err.message;}}
 finally{if(serial===pending){$('load').disabled=false;$('more').disabled=false;}}
}
$('load').addEventListener('click',()=>load(true));$('more').addEventListener('click',()=>load(false));
for(const id of ['tenant','plant'])$(id).addEventListener('input',()=>{pending++;clear();currentScope='';$('load').disabled=false;$('status').textContent='Plant selection changed. Run a new check.';});
})();
