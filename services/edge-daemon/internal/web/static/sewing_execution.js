/* 09A-C — Emulated RFID button adapter. Never synthesizes actual RF/EPC/RSSI data. */
(() => {'use strict';
const $ = id => document.getElementById(id);
const api = '/api/v1/sewing/sim';
const queueKey = 'agiparel.sew09.pending.v1';
const state = {lays:[],bundles:[],ops:[],scanned:new Set(),cycle:null,cycleState:null,busy:false};
const esc = s => String(s??'').replace(/[&<>"']/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
const id = () => 'SEW-' + (crypto.randomUUID ? crypto.randomUUID().replaceAll('-','') : Array.from(crypto.getRandomValues(new Uint8Array(16))).map(x=>x.toString(16).padStart(2,'0')).join(''));
const selected = () => state.ops.find(x=>x.id===$('op').value);
const currentGroup = () => state.bundles.filter(b => `${b.marker_position}/${b.ply_group}`===$('group').value);
const selectedParts = () => {const op=selected();return op ? currentGroup().filter(b=>op.required_parts.includes(b.part_code)):[]};
const queue = () => {try{return JSON.parse(localStorage.getItem(queueKey)||'[]')}catch{return []}};
const saveQueue = q => {localStorage.setItem(queueKey,JSON.stringify(q));$('pending').textContent=q.length};
function message(s,error=false){$('feedback').className='status'+(error?' error':' ok');$('feedback').textContent=s;const li=document.createElement('li');li.textContent=new Date().toLocaleTimeString()+' — '+s;$('log').prepend(li)}
async function get(path){const r=await fetch(api+path,{cache:'no-store'});let body;try{body=await r.json()}catch{throw new Error('HTTP '+r.status)}if(!r.ok)throw new Error(body.error||'HTTP '+r.status);return body}
async function flush(){if(state.busy)return;state.busy=true;try{let q=queue();while(q.length){const next=q[0];let r;try{r=await fetch(api+next.path,{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(next.body)})}catch(e){$('netState').textContent='Offline: pending requests retained for replay';break}
let body;try{body=await r.json()}catch{break}
if(!r.ok){if(r.status>=500)break;message('Rejected pending '+next.body.action+': '+(body.error||r.status)+'. Resolve before replay.',true);break}
q.shift();saveQueue(q);$('netState').textContent='Connected — committed '+next.path;
if(next.path==='/command'&&next.body.action==='CLAIM'){state.cycle=body.cycle_id;state.cycleState='CLAIMED';state.scanned.clear()}
if(next.path==='/command'&&['PAUSE','RESUME','COMPLETE'].includes(next.body.action)){state.cycleState=next.body.action==='PAUSE'?'PAUSED':next.body.action==='RESUME'?'CLAIMED':'COMPLETED'}
message((body.replayed?'Idempotent replay: ':'Committed: ')+(next.body.action||'UI reader scan')+' — '+(body.evidence_state||body.provenance||'SIMULATED'));
}
}finally{state.busy=false;render();await refreshStatus().catch(()=>{})}}
async function submit(path,body){const q=queue();if(q.length>100){message('Offline queue full. Replay or resolve the oldest request first.',true);return}
q.push({path,body});saveQueue(q);await flush()}
function controls(){const active=state.cycleState==='CLAIMED',paused=state.cycleState==='PAUSED';$('pause').disabled=!active;$('resume').disabled=!paused;$('complete').disabled=!active;$('report').disabled=!state.cycle||state.cycleState==='COMPLETED';$('claim').disabled=!selected()||state.cycleState==='CLAIMED'||state.cycleState==='PAUSED'||selectedParts().length!==selected().required_parts.length||selectedParts().some(b=>!state.scanned.has(b.label_code))||queue().length>0}
function render(){const lay=state.lays.find(x=>x.lay_id===$('lay').value);
$('bundlePreview').textContent=lay?`Order ${lay.order} · Cut ${lay.cut} · Shade ${lay.shade} · ${lay.signed_cut_release?'Signed cutting release exists':'CUTTING DRAFT ONLY — NOT RELEASED'} · ${currentGroup().length} part bundles`:'Select a cutting lay.';
const op=selected();$('opInfo').textContent=op?`Unpublished simulation route | ${op.mode} | ${op.required_parts.join(' + ')} | prerequisites: ${op.predecessors.join(', ')||'none'} | ${op.sam_seconds}s SAM`:'Configure or select a simulated operation.';
const parts=selectedParts();$('scanButtons').innerHTML=parts.length?parts.map(b=>`<div class="scan-row"><div><strong>${esc(b.part_code)}</strong> <span class="chip">${state.scanned.has(b.label_code)?'UI SCAN ACK':'NOT SCANNED'}</span><div class="scan-meta mono">${esc(b.label_code)} · ${b.ply_end-b.ply_start+1} plies · ${esc(b.color)} / ${esc(b.size)}</div></div><button class="secondary" data-label="${esc(b.label_code)}">Emulate RFID scan</button></div>`).join(''):'<p class="muted">Select an operation and compatible group.</p>';
$('scanButtons').querySelectorAll('button[data-label]').forEach(btn=>btn.addEventListener('click',()=>scan(btn.dataset.label)));
$('scanStatus').textContent=`${state.scanned.size} UI-emulated scans acknowledged for the selected operation; EPC and RSSI are null (not physically observed).`;
$('currentCycle').textContent=state.cycle?`Cycle ${state.cycle}\n${state.cycleState} · Station ${$('station').value} · Worker ${$('worker').value}`:'No active cycle. Click each required simulated scan and then Claim.';
$('pending').textContent=queue().length;controls()}
async function loadLays(keep){try{const d=await get('/catalog');state.lays=d.lays;$('lay').innerHTML='<option value="">Choose cutting lay</option>'+d.lays.map(x=>`<option value="${esc(x.lay_id)}">${esc(x.order)} / cut ${x.cut} / ${esc(x.marker)} ${x.signed_cut_release?'[released]':'[draft]'}</option>`).join('');if(keep&&state.lays.some(x=>x.lay_id===keep))$('lay').value=keep;await loadLay()}catch(e){message('Cannot read cutting records: '+e.message,true)}}
async function loadLay(){state.scanned.clear();state.cycle=null;state.cycleState=null;const lay=$('lay').value;
if(!lay){state.bundles=[];state.ops=[];$('group').innerHTML='';$('op').innerHTML='';render();return}
try{const [a,b]=await Promise.all([get('/catalog?lay_id='+encodeURIComponent(lay)),get('/operations?lay_id='+encodeURIComponent(lay)+'&plant_id='+encodeURIComponent($('plant').value.trim()))]);state.bundles=a.bundles;state.ops=b.operations;const groups=[...new Set(state.bundles.map(x=>`${x.marker_position}/${x.ply_group}`))];$('group').innerHTML=groups.map(x=>`<option value="${esc(x)}">Position ${esc(x.split('/')[0])} · ply group ${esc(x.split('/')[1])}</option>`).join('');renderOps();render();await refreshStatus()}catch(e){message(e.message,true)}}
function renderOps(){const previous=$('op').value;$('op').innerHTML=state.ops.map(o=>`<option value="${esc(o.id)}">${esc(o.operation_code)} (${esc(o.mode)})</option>`).join('');if(state.ops.some(o=>o.id===previous))$('op').value=previous;const first=currentGroup();if(first.length)$('newParts').value=($('newMode').value==='BUNDLE_TO_SINGLE'?first.map(b=>b.part_code):[first[0].part_code]).join(',')}
async function createOperation(){const parts=$('newParts').value.split(',').map(x=>x.trim()).filter(Boolean),pred=$('newPred').value.split(',').map(x=>x.trim()).filter(Boolean);
try{const body={lay_id:$('lay').value,plant_id:$('plant').value.trim(),operation_code:$('newCode').value.trim(),mode:$('newMode').value,required_parts:parts,predecessors:pred,machine_class:$('machine').value.trim(),sam_seconds:Number($('newSam').value)};
const r=await fetch(api+'/operations',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(body)});const d=await r.json();if(!r.ok)throw new Error(d.error);const all=await get('/operations?lay_id='+encodeURIComponent(body.lay_id)+'&plant_id='+encodeURIComponent($('plant').value.trim()));state.ops=all.operations;renderOps();$('op').value=d.id;state.scanned.clear();render();message('Simulated operation created; not a published or approved bulletin')}catch(e){message('Operation rejected: '+e.message,true)}}
async function scan(label){if(!selected()||!$('lay').value)return;const body={event_id:id(),lay_id:$('lay').value,plant_id:$('plant').value.trim(),label_code:label,station_id:$('station').value.trim(),worker_id:$('worker').value.trim(),reader:{version:'SEW_READER_V1',source:'UI_SIMULATED',reader_id:'UI-READER-01',clicked_at:new Date().toISOString(),antenna_port:1,epc_hex:null,rssi_dbm:null}};
const before=queue().length;await submit('/scan',body);if(queue().length===before){state.scanned.add(label);render()}}
async function claim(){const op=selected();if(!op)return;const [p,g]=$('group').value.split('/').map(Number);
await submit('/command',{command_id:id(),action:'CLAIM',plant_id:$('plant').value.trim(),operation_id:op.id,lay_id:$('lay').value,marker_position:p,ply_group:g,station_id:$('station').value.trim(),worker_id:$('worker').value.trim(),machine_class:$('machine').value.trim()})}
async function progress(action){if(!state.cycle)return;const body={command_id:id(),action,plant_id:$('plant').value.trim(),cycle_id:state.cycle,station_id:$('station').value.trim(),worker_id:$('worker').value.trim()};if(action==='ISSUE'){body.detail=$('issue').value.trim();if(!body.detail){message('Describe the issue first.',true);return}}
await submit('/command',body)}
async function refreshStatus(){const lay=$('lay').value;if(!lay)return;try{const data=await get('/status?lay_id='+encodeURIComponent(lay)+'&plant_id='+encodeURIComponent($('plant').value.trim()));$('total').textContent=data.total_simulated_operation_units;
$('cycles').innerHTML=data.cycles.map(c=>`<tr><td>${esc(c.operation_code)}</td><td>${c.marker_position}/${c.ply_group}</td><td>${esc(c.state)}</td><td>${c.simulated_units}</td><td>${(c.active_ms/1000).toFixed(1)}s</td></tr>`).join('')||'<tr><td colspan="5">No cycles committed.</td></tr>';
if(!state.cycle){const c=data.cycles.find(c=>c.state==='CLAIMED'||c.state==='PAUSED');if(c&&c.station_id===$('station').value&&c.worker_id===$('worker').value){state.cycle=c.cycle_id;state.cycleState=c.state}}else{const c=data.cycles.find(c=>c.cycle_id===state.cycle);if(c)state.cycleState=c.state}render()}catch(e){message('Status unavailable: '+e.message,true)}}
$('lay').addEventListener('change',loadLay);$('group').addEventListener('change',()=>{state.scanned.clear();renderOps();render()});$('op').addEventListener('change',()=>{state.scanned.clear();render()});$('station').addEventListener('change',render);$('worker').addEventListener('change',render);$('machine').addEventListener('change',render);$('plant').addEventListener('change',()=>loadLay());$('newMode').addEventListener('change',()=>{const parts=currentGroup();$('newParts').value=($('newMode').value==='BUNDLE_TO_SINGLE'?parts:parts.slice(0,1)).map(b=>b.part_code).join(',')});
$('reload').addEventListener('click',()=>loadLays($('lay').value));$('create').addEventListener('click',createOperation);$('claim').addEventListener('click',claim);$('pause').addEventListener('click',()=>progress('PAUSE'));$('resume').addEventListener('click',()=>progress('RESUME'));$('complete').addEventListener('click',()=>progress('COMPLETE'));$('report').addEventListener('click',()=>progress('ISSUE'));$('sync').addEventListener('click',flush);$('refresh').addEventListener('click',refreshStatus);window.addEventListener('online',flush);
$('pending').textContent=queue().length;loadLays().then(flush);
})();
