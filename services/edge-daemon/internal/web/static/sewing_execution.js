/* Author: Sunil Jain, 2026-10-28 — Chapter 06 scalable, context-safe sewing rehearsal.
 * UI taps are simulated evidence, never physical RFID or production credit.
 */
(() => {
'use strict';
const $ = id => document.getElementById(id);
const api = '/api/v1/sewing/sim';
const legacyQueueKey = 'agiparel.sew09.pending.v1';
const queuePrefix = 'agiparel.sew09.pending.v2.'; // Isolated by plant, station and worker.
const state = {lays:[],groups:[],bundles:[],ops:[],scanned:new Set(),cycle:null,cycleState:null,
  activeElsewhere:false,activeConflict:false,busy:false,layCursor:'',groupCursor:'',generation:0,statusGeneration:0};
const esc = s => String(s??'').replace(/[&<>"']/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
const id = () => 'SEW-' + (crypto.randomUUID ? crypto.randomUUID().replaceAll('-','') : Array.from(crypto.getRandomValues(new Uint8Array(16))).map(x=>x.toString(16).padStart(2,'0')).join(''));
const fields = () => ({plant_id:$('plant').value.trim(),station_id:$('station').value.trim(),worker_id:$('worker').value.trim()});
const validCode = s => /^[A-Za-z0-9_-]{2,64}$/.test(s);
const ready = () => Object.values(fields()).every(validCode) && validCode($('machine').value.trim());
const sameActor = body => Object.entries(fields()).every(([name,val])=>body[name]===val);
const selected = () => state.ops.find(x=>x.id===$('op').value);
const currentGroup = () => state.bundles.filter(b => `${b.marker_position}/${b.ply_group}`===$('group').value);
const selectedParts = () => {const op=selected();return op ? currentGroup().filter(b=>op.required_parts.includes(b.part_code)):[]};
function scopeKey(actor=fields()){
  if(!Object.values(actor).every(validCode))return '';
  return queuePrefix+[actor.plant_id,actor.station_id,actor.worker_id].join('.');
}
function queue(key=scopeKey()){
  if(!key)return [];
  try{const q=JSON.parse(localStorage.getItem(key)||'[]');return Array.isArray(q)?q:[]}catch{return []}
}
function saveQueue(q,key=scopeKey()){
  if(!key)throw new Error('Station identity is required for durable retry storage');
  localStorage.setItem(key,JSON.stringify(q));
  $('pending').textContent=queue().length;
}
// One-time partition of existing v1 requests; retry event IDs remain unchanged.
// If storage fails or an old event has invalid identity, do not discard v1.
function migrateLegacyQueue(){
  let old;
  try{old=JSON.parse(localStorage.getItem(legacyQueueKey)||'[]')}catch{return}
  if(!Array.isArray(old)||!old.length)return;
  const batches=new Map();
  for(const item of old){
    const key=scopeKey(item&&item.body||{});
    if(!key)return;
    if(!batches.has(key))batches.set(key,[]);
    batches.get(key).push(item);
  }
  try{
    for(const [key,items] of batches){
      const q=queue(key),seen=new Set(q.map(x=>x.body?.event_id||x.body?.command_id));
      for(const item of items){
        const event=item.body.event_id||item.body.command_id;
        if(!event||seen.has(event))continue;
        q.push(item);seen.add(event);
      }
      saveQueue(q,key);
    }
    localStorage.removeItem(legacyQueueKey);
  }catch{/* Keep the legacy queue; later attempts deduplicate by event ID. */}
}
function message(s,error=false){$('feedback').className='status'+(error?' error':' ok');$('feedback').textContent=s;const li=document.createElement('li');li.textContent=new Date().toLocaleTimeString()+' — '+s;$('log').prepend(li)}
async function get(path){const r=await fetch(api+path,{cache:'no-store'});let body;try{body=await r.json()}catch{throw new Error('HTTP '+r.status)}if(!r.ok)throw new Error(body.error||'HTTP '+r.status);return body}
function resetWork(){state.scanned.clear();state.cycle=null;state.cycleState=null;state.activeElsewhere=false;state.activeConflict=false;state.statusGeneration++;$('cycles').innerHTML='';$('total').textContent='—';}
function controls(){
  const op=selected(), parts=selectedParts(), q=queue();
  const active=state.cycleState==='CLAIMED', paused=state.cycleState==='PAUSED';
  $('pause').disabled=!active||!ready()||q.length>0;
  $('resume').disabled=!paused||!ready()||q.length>0;
  $('complete').disabled=!active||!ready()||q.length>0;
  $('report').disabled=!state.cycle||state.cycleState==='COMPLETED'||!ready()||q.length>0;
  $('claim').disabled=!ready()||!op||state.cycleState==='CLAIMED'||state.cycleState==='PAUSED'||state.activeElsewhere||state.activeConflict||
    parts.length!==op.required_parts.length||parts.some(b=>!state.scanned.has(b.label_code))||q.length>0;
}
function render(){
  const lay=state.lays.find(x=>x.lay_id===$('lay').value);
  $('bundlePreview').textContent=lay?`Order ${lay.order} · Cut ${lay.cut} · Shade ${lay.shade} · ${lay.signed_cut_release?'Signed cutting release exists':'CUTTING DRAFT ONLY — NOT RELEASED'} · ${currentGroup().length} part bundles`:'Select a cutting lay.';
  const op=selected();
  $('opInfo').textContent=op?`Unpublished simulation route | ${op.mode} | ${op.required_parts.join(' + ')} | prerequisites: ${op.predecessors.join(', ')||'none'} | ${op.sam_seconds}s SAM`:'Configure or select a simulated operation.';
  const parts=selectedParts();
  $('scanButtons').innerHTML=parts.length?parts.map(b=>`<div class="scan-row"><div><strong>${esc(b.part_code)}</strong> <span class="chip">${state.scanned.has(b.label_code)?'UI SCAN ACK':'NOT SCANNED'}</span><div class="scan-meta mono">${esc(b.label_code)} · ${b.ply_end-b.ply_start+1} plies · ${esc(b.color)} / ${esc(b.size)}</div></div><button class="secondary" data-label="${esc(b.label_code)}" ${queue().length>0||state.scanned.has(b.label_code)?'disabled':''}>Emulate RFID scan</button></div>`).join(''):'<p class="muted">Select a sewing job and compatible bundle group.</p>';
  $('scanButtons').querySelectorAll('button[data-label]').forEach(btn=>btn.addEventListener('click',()=>scan(btn.dataset.label)));
  $('scanStatus').textContent=`${state.scanned.size} server-acknowledged UI scans for this operation; EPC and RSSI are null (not physically observed).`;
  $('currentCycle').textContent=state.activeConflict?'Multiple open demo jobs for this station and worker. Resolve in the register before claiming another.':
    state.activeElsewhere?'An open demo job is assigned to another group. Select that group to resume.':
    state.cycle?`Cycle ${state.cycle}\n${state.cycleState} · Station ${$('station').value} · Worker ${$('worker').value}`:'No active cycle. Tap each required simulated scan, then start work.';
  $('moreLays').classList.toggle('hidden',!state.layCursor);
  $('moreGroups').classList.toggle('hidden',!state.groupCursor);
  $('pending').textContent=queue().length;
  $('dropPending').disabled=queue().length===0||state.busy;
  $('identityHelp').textContent=ready()?'Demo identities entered. No physical station authentication or payroll credit is implied.':'Enter valid plant, station, worker and machine codes (2–64 letters, digits, _ or -).';
  controls();
}
function layOptions(items){return items.map(x=>`<option value="${esc(x.lay_id)}">${esc(x.order)} / cut ${x.cut} / ${esc(x.marker)} ${x.signed_cut_release?'[released]':'[draft]'}</option>`).join('')}
async function loadLays(keep='',more=false){
  const token=++state.generation, cursor=more?state.layCursor:'';
  const previousLay=state.lays.find(x=>x.lay_id===keep);
  if(more&&!cursor)return;
  try{
    const d=await get('/catalog'+(cursor?'?lay_cursor='+encodeURIComponent(cursor):''));
    if(token!==state.generation)return;
    state.layCursor=d.lays_next_cursor||'';
    if(more){state.lays.push(...(d.lays||[]));$('lay').insertAdjacentHTML('beforeend',layOptions(d.lays||[]))}
    else{state.lays=d.lays||[];if(previousLay&&!state.lays.some(x=>x.lay_id===keep))state.lays.push(previousLay);$('lay').innerHTML='<option value="">Choose cutting lay</option>'+layOptions(state.lays);
      if(keep&&state.lays.some(x=>x.lay_id===keep))$('lay').value=keep;
      await loadLay();return;}
    render();
  }catch(e){if(token===state.generation)message('Cannot read cutting records: '+e.message,true)}
}
function groupOptions(groups){return groups.map(x=>`<option value="${x.marker_position}/${x.ply_group}">Position ${x.marker_position} · ply group ${x.ply_group} · ${x.part_count} parts</option>`).join('')}
async function loadGroups(more=false){
  const token=state.generation,lay=$('lay').value,cursor=more?state.groupCursor:'';
  if(!lay||(more&&!cursor))return;
  try{
    const d=await get('/catalog?groups_only=1&lay_id='+encodeURIComponent(lay)+(cursor?'&group_cursor='+encodeURIComponent(cursor):''));
    if(token!==state.generation||lay!==$('lay').value)return;
    state.groupCursor=d.groups_next_cursor||'';
    if(more){state.groups.push(...(d.groups||[]));$('group').insertAdjacentHTML('beforeend',groupOptions(d.groups||[]))}
    else{state.groups=d.groups||[];$('group').innerHTML=groupOptions(state.groups);await loadGroup();return}
    render();
  }catch(e){if(token===state.generation)message('Bundle groups unavailable: '+e.message,true)}
}
async function loadGroup(){
  const token=++state.generation,lay=$('lay').value,group=$('group').value;
  state.bundles=[];state.scanned.clear();state.cycle=null;state.cycleState=null;state.activeElsewhere=false;renderOps();render();
  if(!lay||!group)return;
  const [pos,ply]=group.split('/').map(Number);
  try{
    const d=await get('/catalog?lay_id='+encodeURIComponent(lay)+'&bundle_position='+pos+'&bundle_group='+ply);
    if(token!==state.generation||lay!==$('lay').value||group!==$('group').value)return;
    state.bundles=d.bundles||[];renderOps();render();await refreshStatus();
  }catch(e){if(token===state.generation)message('Selected bundle parts unavailable: '+e.message,true)}
}
async function loadLay(){
  const token=++state.generation,lay=$('lay').value,plant=$('plant').value.trim();
  resetWork();state.groups=[];state.bundles=[];state.ops=[];state.groupCursor='';
  $('group').innerHTML='';$('op').innerHTML='';render();
  if(!lay)return;
  // No custom station or plant defaults. Bundle groups can be browsed before identity entry.
  const opsPromise=validCode(plant)?get('/operations?lay_id='+encodeURIComponent(lay)+'&plant_id='+encodeURIComponent(plant)):Promise.resolve({operations:[]});
  try{
    const [ops,groups]=await Promise.all([opsPromise,get('/catalog?groups_only=1&lay_id='+encodeURIComponent(lay))]);
    if(token!==state.generation||lay!==$('lay').value||plant!==$('plant').value.trim())return;
    state.ops=ops.operations||[];state.groups=groups.groups||[];state.groupCursor=groups.groups_next_cursor||'';
    $('group').innerHTML=groupOptions(state.groups);renderOps();render();await loadGroup();
    if(!validCode(plant))message('Enter a plant code to load its simulation sewing operations.',true);
  }catch(e){if(token===state.generation)message('Cutting or sewing records unavailable: '+e.message,true)}
}
function renderOps(){
  const previous=$('op').value;
  $('op').innerHTML=state.ops.map(o=>`<option value="${esc(o.id)}">${esc(o.operation_code)} (${esc(o.mode)})</option>`).join('');
  if(state.ops.some(o=>o.id===previous))$('op').value=previous;
  const parts=currentGroup();
  if(parts.length)$('newParts').value=($('newMode').value==='BUNDLE_TO_SINGLE'?parts:parts.slice(0,1)).map(b=>b.part_code).join(',');
}
async function createOperation(){
  if(!ready()||!$('lay').value){message('Choose a lay and enter complete station details.',true);return}
  const parts=$('newParts').value.split(',').map(x=>x.trim()).filter(Boolean),pred=$('newPred').value.split(',').map(x=>x.trim()).filter(Boolean);
  const body={lay_id:$('lay').value,plant_id:$('plant').value.trim(),operation_code:$('newCode').value.trim(),mode:$('newMode').value,required_parts:parts,predecessors:pred,machine_class:$('machine').value.trim(),sam_seconds:Number($('newSam').value)};
  const token=state.generation;
  try{
    const r=await fetch(api+'/operations',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(body)});
    const d=await r.json();if(!r.ok)throw new Error(d.error||'HTTP '+r.status);
    if(token!==state.generation||body.lay_id!==$('lay').value||body.plant_id!==$('plant').value.trim())return;
    const all=await get('/operations?lay_id='+encodeURIComponent(body.lay_id)+'&plant_id='+encodeURIComponent(body.plant_id));
    if(token!==state.generation)return;
    state.ops=all.operations;renderOps();$('op').value=d.id;state.scanned.clear();render();message('Demo operation saved; not a published or approved operation bulletin.');
  }catch(e){if(token===state.generation)message('Operation rejected: '+e.message,true)}
}
async function flush(){
  if(state.busy)return new Set();state.busy=true;const committed=new Set();
  const key=scopeKey();
  try{
    let q=queue(key);
    while(q.length){
      const next=q[0];
      if(!ready()||scopeKey()!==key||!sameActor(next.body)){$('netState').textContent='Pending evidence belongs to a different or incomplete station identity. Return to that station to replay safely.';break}
      let r,body;
      try{r=await fetch(api+next.path,{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(next.body)});body=await r.json()}
      catch{$('netState').textContent='Offline: pending requests retained for replay';break}
      if(!r.ok){$('netState').textContent='Pending request blocked: '+(body.error||r.status)+'. No later event will be sent until resolved.';message('Rejected pending '+(next.body.action||'scan')+': '+(body.error||r.status)+'. Contact a supervisor before retrying.',true);break}
      q.shift();saveQueue(q,key);$('netState').textContent='Connected — committed '+next.path;
      committed.add(next.body.event_id||next.body.command_id);
      if(next.path==='/scan'&&sameActor(next.body)&&$('lay').value===next.body.lay_id&&currentGroup().some(b=>b.label_code===next.body.label_code))state.scanned.add(next.body.label_code);
      if(next.path==='/command'&&sameActor(next.body)&&next.body.action==='CLAIM'&&$('lay').value===next.body.lay_id){state.cycle=body.cycle_id;state.cycleState='CLAIMED';state.scanned.clear()}
      if(next.path==='/command'&&sameActor(next.body)&&state.cycle===body.cycle_id&&['PAUSE','RESUME','COMPLETE','ISSUE'].includes(next.body.action)){
        state.cycleState=next.body.action==='PAUSE'?'PAUSED':next.body.action==='RESUME'?'CLAIMED':next.body.action==='COMPLETE'?'COMPLETED':'REVIEW_REQUIRED';
      }
      message((body.replayed?'Idempotent replay: ':'Committed: ')+(next.body.action||'UI reader scan')+' — '+(body.evidence_state||body.provenance||'SIMULATED'));
    }
  }finally{state.busy=false;render();await refreshStatus().catch(()=>{})}
  return committed;
}
async function submit(path,body){
  if(!ready()){message('Enter valid plant, station, worker and machine identifiers first.',true);return false}
  if(queue().length>=100){message('Device retry queue is full. Replay or resolve the oldest request first.',true);return false}
  try{const q=queue();q.push({path,body});saveQueue(q)}
  catch{message('Cannot preserve this request locally. Nothing was sent.',true);return false}
  const committed=await flush();return committed.has(body.event_id||body.command_id);
}
async function scan(label){
  if(!ready()||queue().length>0||state.scanned.has(label)||!selected()||!$('lay').value||!selectedParts().some(b=>b.label_code===label)){message('Select a compatible group and enter station details first.',true);return}
  const body={event_id:id(),lay_id:$('lay').value,...fields(),label_code:label,reader:{version:'SEW_READER_V1',source:'UI_SIMULATED',reader_id:'UI-DEMO-READER',clicked_at:new Date().toISOString(),antenna_port:1,epc_hex:null,rssi_dbm:null}};
  await submit('/scan',body);render();
}
async function claim(){const op=selected();if(!op||$('claim').disabled)return;
  const [p,g]=$('group').value.split('/').map(Number);
  await submit('/command',{command_id:id(),action:'CLAIM',...fields(),operation_id:op.id,lay_id:$('lay').value,marker_position:p,ply_group:g,machine_class:$('machine').value.trim()});
}
async function progress(action){if(!state.cycle||!ready())return;
  const body={command_id:id(),action,...fields(),cycle_id:state.cycle};
  if(action==='ISSUE'){body.detail=$('issue').value.trim();if(!body.detail){message('Describe the issue first.',true);return}}
  await submit('/command',body);
}
async function refreshStatus(){
  const lay=$('lay').value,actor=fields(),group=$('group').value,token=++state.statusGeneration;
  if(!lay||!validCode(actor.plant_id)||!validCode(actor.station_id)||!validCode(actor.worker_id))return;
  try{
    const qs=new URLSearchParams({lay_id:lay,...actor});const data=await get('/status?'+qs.toString());
    if(token!==state.statusGeneration||lay!==$('lay').value||group!==$('group').value||!sameActor(actor))return;
    $('total').textContent=data.total_simulated_operation_units;
    $('historyScope').textContent=data.cycles_has_more?'Showing the latest '+data.cycles.length+' cycles (more history exists); total includes every committed cycle.':'Showing '+data.cycles.length+' recent cycles; total includes every committed cycle.';
    $('cycles').innerHTML=data.cycles.map(c=>`<tr><td>${esc(c.operation_code)}</td><td>${c.marker_position}/${c.ply_group}</td><td>${esc(c.state)}</td><td>${c.simulated_units}</td><td>${(c.active_ms/1000).toFixed(1)}s</td></tr>`).join('')||'<tr><td colspan="5">No cycles committed.</td></tr>';
    state.activeConflict=!!data.active_conflict;
    state.activeElsewhere=!!(data.active_cycle&&`${data.active_cycle.marker_position}/${data.active_cycle.ply_group}`!==group);
    if(data.active_cycle&&!state.activeElsewhere){state.cycle=data.active_cycle.cycle_id;state.cycleState=data.active_cycle.state}
    else{state.cycle=null;state.cycleState=null}
    render();
  }catch(e){if(token===state.statusGeneration)message('Status unavailable: '+e.message,true)}
}
$('lay').addEventListener('change',loadLay);
$('group').addEventListener('change',loadGroup);
$('op').addEventListener('change',()=>{state.scanned.clear();render()});
$('plant').addEventListener('change',loadLay);
for(const name of ['station','worker'])$(name).addEventListener('change',()=>{resetWork();render();refreshStatus();flush()});
$('machine').addEventListener('change',render);
$('newMode').addEventListener('change',()=>{const parts=currentGroup();$('newParts').value=($('newMode').value==='BUNDLE_TO_SINGLE'?parts:parts.slice(0,1)).map(b=>b.part_code).join(',')});
$('reload').addEventListener('click',()=>loadLays($('lay').value));
$('moreLays').addEventListener('click',()=>loadLays('',true));
$('moreGroups').addEventListener('click',()=>loadGroups(true));
$('create').addEventListener('click',createOperation);$('claim').addEventListener('click',claim);
$('pause').addEventListener('click',()=>progress('PAUSE'));
$('resume').addEventListener('click',()=>progress('RESUME'));
$('complete').addEventListener('click',()=>progress('COMPLETE'));
$('report').addEventListener('click',()=>progress('ISSUE'));
$('sync').addEventListener('click',flush);$('refresh').addEventListener('click',refreshStatus);
$('dropPending').addEventListener('click',()=>{
  if(state.busy)return;
  const pending=queue();if(!pending.length)return;
  if(!window.confirm('Discard the oldest LOCAL demo retry? This cannot undo a server-committed event. Check saved work before deleting.'))return;
  const removed=pending.shift();
  try{saveQueue(pending);message('Discarded one LOCAL retry ('+(removed.body.action||'simulated scan')+'). Server records, if any, remain immutable.');render()}
  catch{message('Could not update local retry queue. No item was removed.',true)}
});
window.addEventListener('online',flush);
migrateLegacyQueue();$('pending').textContent=queue().length;loadLays().then(()=>flush());
})();
