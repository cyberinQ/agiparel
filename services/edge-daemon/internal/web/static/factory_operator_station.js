// Author: Sunil Jain, CyberinQ LLC, 2026-10-28.
// F3.9: signed gateway inbox, fail-closed handoff previews, F3.3 signed writes.
// No browser storage, synthetic events, implicit credit or HTML injection.
(()=>{'use strict';
const $=id=>document.getElementById(id);
const code=/^[A-Za-z0-9_-]{2,64}$/;
const uuid=/^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$/;
const authority='ENROLLED_GATEWAY_REPORT_AND_SIGNED_CUSTODY_NOT_PHYSICAL_CERTIFICATION';
let generation=0,selected='',current=null,prepared=null,scope='';
function cell(tr,value){const td=document.createElement('td');td.textContent=String(value);tr.appendChild(td);return td;}
function clearPrepared(){prepared=null;$('challenge').textContent='No transaction prepared.';$('signed-token').value='';$('submit').disabled=true;}
function clearAll(){generation++;selected='';current=null;scope='';$('inbox').replaceChildren();$('detail').replaceChildren();$('detail-status').textContent='Select a signed scan.';$('blockers').textContent='No part selected.';
 $('route').replaceChildren();$('route').disabled=true;$('prepare').disabled=true;$('result').textContent='No handoff attempted.';clearPrepared();}
function parameters(){const tenant=$('tenant').value.trim(),plant=$('plant').value.trim(),station=$('station').value.trim();
 if(!code.test(tenant)||!code.test(plant)||!code.test(station))throw new Error('Enter valid tenant, plant and station IDs.');return {tenant,plant,station};}
function validDate(d){return typeof d==='string'&&Number.isFinite(Date.parse(d));}
function validResponse(v,s,scan){if(!v||v.authority!==authority||v.tenant_id!==s.tenant||v.plant_id!==s.plant||v.station_id!==s.station||!validDate(v.as_of)||
 !Array.isArray(v.scans)||v.scans.length>30||!Array.isArray(v.limitations)||v.limitations.length<3)return false;
 for(const x of v.scans)if(!x||!uuid.test(x.scan_id)||!uuid.test(x.part_id)||!validDate(x.received_at))return false;
 if(scan){const d=v.detail;if(!d||d.scan_id!==scan||!uuid.test(d.part_id)||!uuid.test(d.garment_id)||!uuid.test(d.bundle_id)||!(typeof d.part_code==='string'&&d.part_code.length>0&&d.part_code.length<=64)||
 !validDate(d.received_at)||!['NONE','DEPART','ARRIVE'].includes(d.last_action)||!['BLOCKED','READY_FOR_SIGNED_HANDOFF'].includes(d.status)||!Array.isArray(d.blockers)||!Array.isArray(d.next))return false;
 if(d.last_recorded_at!==null&&!validDate(d.last_recorded_at))return false;
 if(d.status==='READY_FOR_SIGNED_HANDOFF'&&(d.blockers.length!==0||d.next.length===0))return false;
 if(d.status==='BLOCKED'&&(d.blockers.length===0||d.next.length!==0))return false;
 for(const r of d.next)if(!uuid.test(r.route_id)||!['DEPART','ARRIVE'].includes(r.action)||r.station_id!==s.station||!code.test(r.other_station)||!(typeof r.revision==='string'&&r.revision.length>0&&r.revision.length<=64)||
 (d.last_action==='DEPART'&&r.action!=='ARRIVE')||(d.last_action!=='DEPART'&&r.action!=='DEPART'))return false;
 }else if(v.detail!==null)return false;
 return true;}
function showDetail(d){$('detail').replaceChildren();for(const [k,v] of [['Part',d.part_id],['Garment',d.garment_id],['Bundle',d.bundle_id],['Part type',d.part_code],['Signed scan',d.scan_id],['Scan received',d.received_at],['Last event',d.last_action],['Last station',d.last_station||'Unknown'],['Last event recorded',d.last_recorded_at||'No prior movement']]){
 const tr=document.createElement('tr');cell(tr,k);cell(tr,v);$('detail').append(tr);}
 $('detail-status').textContent=d.status;
 $('blockers').textContent=d.blockers.length?'Blocked: '+d.blockers.join(' · '):'No snapshot blockers. Signed write still required.';
 $('route').replaceChildren();for(const r of d.next){const option=document.createElement('option');option.value=r.route_id;
 option.textContent=r.action+' · '+r.station_id+' ↔ '+r.other_station+' · revision '+r.revision;$('route').append(option);}
 $('route').disabled=d.next.length===0;$('prepare').disabled=d.next.length===0;
}
async function read(scan=''){
 let s;try{s=parameters()}catch(e){clearAll();$('status').textContent=e.message;return;}
 const newScope=[s.tenant,s.plant,s.station].join('\n');if(newScope!==scope){clearAll();scope=newScope;}
 const ticket=++generation;selected=scan;current=null;clearPrepared();$('prepare').disabled=true;
 $('status').textContent='Reading scoped signed gateway reports…';
 try{const q=new URLSearchParams({tenant_id:s.tenant,plant_id:s.plant,station_id:s.station,limit:'30'});if(scan)q.set('scan_id',scan);
 const res=await fetch('/api/v1/operator-station/inbox?'+q,{method:'GET',credentials:'same-origin',cache:'no-store',headers:{Accept:'application/json'}});
 if(!res.ok)throw new Error(res.status===401?'Enterprise sign-in required.':res.status===403?'No assigned worker or plant grant.':res.status===404?'Signed scan unavailable at assigned station.':'Station inbox unavailable (HTTP '+res.status+').');
 const data=await res.json();if(ticket!==generation)return;
 if(!validResponse(data,s,scan))throw new Error('Invalid or inconsistent station evidence response.');
 $('inbox').replaceChildren();for(const v of data.scans){const tr=document.createElement('tr');cell(tr,v.received_at);cell(tr,v.part_id);cell(tr,v.scan_id);
 const td=document.createElement('td'),btn=document.createElement('button');btn.type='button';btn.textContent='Inspect';btn.addEventListener('click',()=>read(v.scan_id));td.append(btn);tr.append(td);$('inbox').append(tr);}
 $('status').textContent=data.scans.length+' recent signed reports loaded. Not an exhaustive WIP count.';
 if(scan){current=data.detail;showDetail(current);}else{$('detail').replaceChildren();$('detail-status').textContent='Select a signed scan.';$('blockers').textContent='No part selected.';$('route').replaceChildren();$('route').disabled=true;}
 }catch(e){if(ticket===generation){clearAll();$('status').textContent=e.message;}}
}
function chosenRoute(){if(!current||current.status!=='READY_FOR_SIGNED_HANDOFF')throw new Error('No eligible scan.');
 const route=current.next.find(r=>r.route_id===$('route').value);if(!route)throw new Error('Choose an approved next route.');return route;}
async function prepare(){clearPrepared();let route,s;try{route=chosenRoute();s=parameters();if(!uuid.test(selected)||current.scan_id!==selected)throw new Error('Reinspect the signed scan.');}catch(e){$('result').textContent=e.message;return;}
 const serial=++generation;const command={command_id:crypto.randomUUID(),plant_id:s.plant,part_id:current.part_id,route_id:route.route_id,scan_id:selected,action:route.action,station_id:s.station};
 try{const res=await fetch('/api/v1/wip/production/movements/challenge',{method:'POST',credentials:'same-origin',cache:'no-store',headers:{'Content-Type':'application/json',Accept:'application/json'},body:JSON.stringify({command})});
 if(!res.ok)throw new Error('Signed handoff challenge unavailable (HTTP '+res.status+').');const data=await res.json();if(serial!==generation)return;
 if(typeof data.challenge!=='string'||!/^WIP_MOVE_V1:[0-9a-f]{64}$/.test(data.challenge)||data.required_action!=='sewing.transfer'||data.required_auth!=='webauthn_or_fido2'||data.production_credit!==false)throw new Error('Invalid handoff challenge.');
 prepared={command,challenge:data.challenge,scope:[s.tenant,s.plant,s.station].join('\n'),scan:selected};$('challenge').textContent='FIDO2 transaction challenge: '+data.challenge+' · '+command.action+' at '+s.station+' · route '+command.route_id;
 $('submit').disabled=false;$('result').textContent='Obtain a fresh signed assertion from your provisioned worker IdP for this exact challenge.';
 }catch(e){if(serial===generation){clearPrepared();$('result').textContent=e.message;}}
}
async function submit(){if(!prepared){$('result').textContent='Prepare a new challenge.';return;}
 let s;try{s=parameters();if(prepared.scope!==[s.tenant,s.plant,s.station].join('\n')||prepared.scan!==selected)throw new Error('Scope or scan changed. Reprepare.');}catch(e){clearPrepared();$('result').textContent=e.message;return;}
 const token=$('signed-token').value.trim();$('signed-token').value='';if(token.length<40||token.length>16384||!/^[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+$/.test(token)){$('result').textContent='A valid short-lived signed worker assertion is required.';return;}
 const command=prepared.command;clearPrepared();const serial=++generation;$('result').textContent='Submitting signed transaction…';
 try{const res=await fetch('/api/v1/wip/production/movements',{method:'POST',credentials:'omit',cache:'no-store',headers:{Authorization:'Bearer '+token,'Content-Type':'application/json',Accept:'application/json'},body:JSON.stringify({command})});
 if(!res.ok)throw new Error('Handoff rejected by F3.3 (HTTP '+res.status+'). No custody change is claimed.');const data=await res.json();if(serial!==generation)return;
 if(!data||!uuid.test(data.id)||data.action!==command.action||data.production_credit!==false||data.shipping_release!==false||data.authority!=='SIGNED_SCAN_AND_WORKER_CUSTODY_ATTESTATION')throw new Error('Unexpected handoff response. Verify server records before retrying.');
 $('result').textContent='Signed custody event recorded: '+data.action+' · '+data.id+'. Production/QC/shipping credit: none.';
 await read(selected);
 }catch(e){if(serial===generation){$('result').textContent=e.message;}}
}
$('refresh').addEventListener('click',()=>read());$('prepare').addEventListener('click',prepare);$('submit').addEventListener('click',submit);
$('cancel').addEventListener('click',()=>{generation++;clearPrepared();$('result').textContent='Transaction cleared.';});
$('route').addEventListener('change',()=>{clearPrepared();$('result').textContent='Route changed. Prepare a fresh challenge.';});
for(const id of ['tenant','plant','station'])$(id).addEventListener('input',()=>{clearAll();$('status').textContent='Station scope changed. Reload signed scans.';});
})();
