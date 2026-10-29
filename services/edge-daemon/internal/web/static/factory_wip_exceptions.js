// Author: Sunil Jain, CyberinQ LLC, 2026-10-29.
// F3.10: operator-reported WIP holds and supervisor scan-backed closure.
// No untrusted HTML, browser token storage, physical-state mutation or auto-clear.
(()=>{'use strict';
const $=id=>document.getElementById(id);
const code=/^[A-Za-z0-9_-]{2,64}$/;
const uuid=/^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$/;
const auth='SIGNED_EXCEPTION_REVIEW_NO_CUSTODY_CHANGE';
let pending=null,nonce=0;
function scope(){const tenant=$('tenant').value.trim(),plant=$('plant').value.trim(),station=$('station').value.trim(),part=$('ex-part').value.trim();
 if(!code.test(tenant)||!code.test(plant)||!code.test(station)||!uuid.test(part))throw Error('Enter an assigned tenant, plant, station and permanent cut-part UUID.');
 return {tenant,plant,station,part};}
function reset(){nonce++;pending=null;$('ex-submit').disabled=true;$('ex-token').value='';$('ex-challenge').textContent='No exception transaction prepared.';}
function note(){const n=$('ex-note').value.trim();if(n.length<12||n.length>500)throw Error('Explanation must be between 12 and 500 characters.');return n;}
function token(){const t=$('ex-token').value.trim();$('ex-token').value='';if(t.length<40||t.length>16384||!/^[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+$/.test(t))throw Error('Fresh transaction-bound JWT required.');return t;}
async function listCases(){reset();let s;try{s=scope();}catch(e){$('ex-status').textContent=e.message;return;}
 const gen=nonce;const q=new URLSearchParams({tenant_id:s.tenant,plant_id:s.plant,part_id:s.part,limit:'25'});
 $('ex-status').textContent='Reading reported exceptions…';
 try{const res=await fetch('/api/v1/wip/exceptions/cases?'+q,{credentials:'same-origin',cache:'no-store',headers:{Accept:'application/json'}});
 if(!res.ok)throw Error('Exception history unavailable (HTTP '+res.status+').');const data=await res.json();if(gen!==nonce)return;
 if(!data||data.authority!==auth||data.tenant_id!==s.tenant||data.plant_id!==s.plant||data.part_id!==s.part||data.custody_changed!==false||data.physical_recovery_certified!==false||!Array.isArray(data.cases)||data.cases.length>25)throw Error('Invalid exception evidence response.');
 $('ex-cases').replaceChildren();$('ex-case').replaceChildren();const empty=document.createElement('option');empty.value='';empty.textContent='Select an open case';$('ex-case').append(empty);
 for(const c of data.cases){if(!c||!uuid.test(c.id)||!code.test(c.station_id)||!Number.isFinite(Date.parse(c.opened_at))||!['LOST_CARRIER','UNREADABLE_CARRIER','MISSING_WIP','FAILED_SCAN','MISROUTED_PART','REJECTED_HANDOFF'].includes(c.issue_type)||typeof c.closed!=='boolean'||typeof c.note!=='string')throw Error('Invalid case entry.');
 const row=document.createElement('tr');for(const x of [c.opened_at,c.issue_type,c.station_id,c.closed?'Closed':'OPEN — custody held']){const col=document.createElement('td');col.textContent=x;row.append(col);} $('ex-cases').append(row);
 if(!c.closed&&c.station_id===s.station){const o=document.createElement('option');o.value=c.id;o.textContent=c.id+' · '+c.issue_type;$('ex-case').append(o);}}
 $('ex-status').textContent=data.cases.length+' documented case(s). This does not certify physical recovery.';
 }catch(e){if(gen===nonce){$('ex-cases').replaceChildren();$('ex-case').replaceChildren();$('ex-status').textContent=e.message;}}
}
async function prepare(action){reset();let s,n;try{s=scope();n=note();}catch(e){$('ex-status').textContent=e.message;return;}
 const c={command_id:crypto.randomUUID(),plant_id:s.plant,part_id:s.part,station_id:s.station,action,issue_type:'',case_id:'',scan_id:'',note:n};
 if(action==='OPEN')c.issue_type=$('ex-type').value;
 else{const selected=$('ex-case').value;if(!uuid.test(selected)){$('ex-status').textContent='Select an open case at your assigned station.';return;}
 c.case_id=selected;
 // Select a new, signed F3.9 gateway observation after the incident was opened.
 const selectedScan=(()=>{const p=document.querySelector('#detail tr');return p?document.querySelector('#detail tr:nth-child(5) td:nth-child(2)')?.textContent||'':'';})();
 if(!uuid.test(selectedScan)){$('ex-status').textContent='Select a newer signed scanner observation for the part before requesting closure.';return;}
 c.scan_id=selectedScan;}
 const gen=nonce,signature=[s.tenant,s.plant,s.station,s.part].join('|');
 try{const res=await fetch('/api/v1/wip/exceptions/challenge',{method:'POST',credentials:'same-origin',cache:'no-store',headers:{'Content-Type':'application/json',Accept:'application/json'},body:JSON.stringify({command:c})});
 if(!res.ok)throw Error('Exception challenge unavailable (HTTP '+res.status+').');const d=await res.json();if(gen!==nonce)return;
 if(!d||!/^WIP_EXCEPTION_V1:[0-9a-f]{64}$/.test(d.challenge)||d.authority!==auth||d.required_auth!=='webauthn_or_fido2'||d.custody_changed!==false||d.production_credit!==false||d.required_action!==(action==='OPEN'?'sewing.exception.report':'sewing.exception.resolve'))throw Error('Invalid challenge response.');
 pending={command:c,scope:signature,challenge:d.challenge};$('ex-challenge').textContent='Obtain a fresh '+d.required_role+' assertion for: '+d.challenge;$('ex-submit').disabled=false;
 $('ex-status').textContent=action==='OPEN'?'Report only; SQL holds further WIP moves until separately resolved.':'Supervised closure only; cannot create physical movement.';
 }catch(e){if(gen===nonce){reset();$('ex-status').textContent=e.message;}}
}
async function submit(){const p=pending;if(!p){$('ex-status').textContent='Prepare a fresh exception challenge.';return;}
 let s,t;try{s=scope();if(p.scope!==[s.tenant,s.plant,s.station,s.part].join('|'))throw Error('Scope changed; prepare again.');t=token();}catch(e){reset();$('ex-status').textContent=e.message;return;}
 reset();const gen=nonce;$('ex-status').textContent='Submitting signed exception event…';
 try{const res=await fetch('/api/v1/wip/exceptions/events',{method:'POST',credentials:'omit',cache:'no-store',headers:{'Content-Type':'application/json',Accept:'application/json',Authorization:'Bearer '+t},body:JSON.stringify({command:p.command})});t='';
 if(!res.ok)throw Error('Exception rejected (HTTP '+res.status+'). No custody transition occurred.');const data=await res.json();if(gen!==nonce)return;
 if(!data||!uuid.test(data.id)||data.action!==p.command.action||data.authority!==auth||data.custody_changed!==false||data.production_credit!==false||data.quality_release!==false||data.shipping_release!==false)throw Error('Unexpected response: verify server case history before retrying.');
 $('ex-status').textContent=(data.action==='OPEN'?'Case opened, WIP held: ':'Supervisor review closed case: ')+data.id+'. No custody change.';
 await listCases();
 }catch(e){if(gen===nonce)$('ex-status').textContent=e.message;}
}
$('ex-open').addEventListener('click',()=>prepare('OPEN'));
$('ex-close').addEventListener('click',()=>prepare('CLOSE'));
$('ex-list').addEventListener('click',listCases);
$('ex-submit').addEventListener('click',submit);
$('ex-cancel').addEventListener('click',()=>{reset();$('ex-status').textContent='Cleared.';});
for(const id of ['tenant','plant','station','ex-part','ex-type','ex-case','ex-note'])$(id).addEventListener('change',()=>reset());
})();
