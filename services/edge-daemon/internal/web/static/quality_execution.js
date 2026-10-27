// Author: Sunil Jain, CyberInQ. Batch 10A/B - strict simulation, no RF device claims.
(()=>{'use strict';
const $=id=>document.getElementById(id), api='/api/v1/quality/sim';
const st={lays:[],candidates:[],units:[],route:null,busy:false};
const pendingKey='agiparel.qc.v10.pending';
function pending(){try{return JSON.parse(localStorage.getItem(pendingKey)||'null')}catch{return null}}
function storePending(event){localStorage.setItem(pendingKey,JSON.stringify(event))}

function esc(s){return String(s??'').replace(/[&<>"']/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]))}
function eventID(){return 'QC-'+crypto.randomUUID().replace(/-/g,'')}
function info(text,bad=false){$('feedback').textContent=text;$('feedback').className='info'+(bad?' error':' ok')}
async function request(path,body){const r=await fetch(api+path,{method:body?'POST':'GET',headers:body?{'Content-Type':'application/json'}:{},body:body?JSON.stringify(body):undefined,cache:'no-store'});let d;try{d=await r.json()}catch{throw Error('Unexpected HTTP '+r.status)}if(!r.ok)throw Error(d.error||'HTTP '+r.status);return d}
function selected(){return st.units.find(x=>x.serial===$('serial').value)}
function source(){return st.candidates.find(x=>x.serial===$('serial').value)}
function steps(){return st.route?.steps||[]}
function render(){const u=selected(),s=source(),route=steps();$('sourceStatus').textContent=`${st.candidates.length} source-derived simulated garment identities · ${st.units.length} enrolled; no authoritative finished garments.`;
$('garmentInfo').textContent=s?`${s.serial} | ${s.color} / ${s.size} | marker ${s.position} | ${u?u.qc_state:'NOT ENROLLED'}`:'';
const hasPending=!!pending();$('pendingInfo').textContent=hasPending?'One unacknowledged simulation event is retained for exact-payload replay. Retry before new operations.':'No pending simulation events.';
$('retry').disabled=!hasPending||st.busy;$('discard').disabled=!hasPending||st.busy;
$('enroll').disabled=!s||!!u||st.busy||hasPending;$('profile').disabled=!!st.route||st.busy;$('setRoute').disabled=!!st.route||st.busy||!$('lay').value;
for(const [id,enabled] of Object.entries({pass:u?.qc_state==='PENDING',fail:u?.qc_state==='PENDING',repair:u?.qc_state==='HOLD',repass:u?.qc_state==='REINSPECT',refail:u?.qc_state==='REINSPECT',finish:u?.qc_state==='ACCEPTED'&&route.length>0&&u.finished_steps<route.length,bind:u?.qc_state==='ACCEPTED'&&route.length>0&&u.finished_steps===route.length&&!u.buyer_code}))$(id).disabled=!enabled||st.busy||hasPending;
$('routeSteps').innerHTML=route.map((x,i)=>`<span class="step ${u&&i<u.finished_steps?'done':''}">${i+1}. ${esc(x)}</span>`).join('')||'<span class="muted">No rehearsal route selected</span>';
$('routeInfo').textContent=st.route?st.route.profile+' · UNAPPROVED SIMULATION ROUTE':'Choose one route for this lay and plant.';
$('finishInfo').textContent=u?`${u.finished_steps}/${route.length} simulated finishing steps${route[u.finished_steps]?' · next '+route[u.finished_steps]:''}`:'Select enrolled garment';
if(s){$('size').value=s.size}
const totals={PENDING:0,HOLD:0,REINSPECT:0,ACCEPTED:0};st.units.forEach(x=>{totals[x.qc_state]=(totals[x.qc_state]||0)+1});
$('metrics').innerHTML=[['Sewing source',st.candidates.length],['Enrolled',st.units.length],['Held',totals.HOLD],['Awaiting reinspection',totals.REINSPECT],['QA passes (sim)',totals.ACCEPTED],['Buyer bindings',st.units.filter(x=>x.buyer_code).length]].map(([k,v])=>`<div><div class="stat">${v}</div><div class="muted">${esc(k)}</div></div>`).join('');
$('units').innerHTML=st.units.map(x=>`<tr data-serial="${esc(x.serial)}" class="${x.serial===$('serial').value?'selected':''}"><td class="mono">${esc(x.serial)}</td><td>${esc(x.color)} / ${esc(x.size)}</td><td><span class="badge">${esc(x.qc_state)}</span></td><td>${esc(x.defect)}</td><td>${x.finished_steps}/${route.length}</td><td>${esc(x.buyer_code||'—')}</td></tr>`).join('')||'<tr><td colspan="6">No enrolled simulated garments yet.</td></tr>';
$('units').querySelectorAll('[data-serial]').forEach(tr=>tr.addEventListener('click',()=>{$('serial').value=tr.dataset.serial;render()}));}
async function refresh(keep=true){try{const lay=$('lay').value,plant=$('plant').value.trim(),current=$('serial').value;const d=await request('/catalog'+(lay?'?lay_id='+encodeURIComponent(lay)+'&plant_id='+encodeURIComponent(plant):''));st.lays=d.lays;
$('lay').innerHTML='<option value="">Choose sewing source lay</option>'+d.lays.map(x=>`<option value="${esc(x.lay_id)}">${esc(x.intake_id)} · Cut ${x.cut} (${x.simulated_sewing_serials} simulated serials)</option>`).join('');if(lay&&st.lays.some(x=>x.lay_id===lay))$('lay').value=lay;
st.candidates=d.candidates||[];st.units=d.units||[];st.route=d.route;
$('serial').innerHTML='<option value="">Choose source-derived garment</option>'+st.candidates.map(x=>`<option value="${esc(x.serial)}">${esc(x.serial)} · ${esc(x.color)} / ${esc(x.size)}</option>`).join('');if(keep&&st.candidates.some(x=>x.serial===current))$('serial').value=current;
if(st.route)$('profile').value=st.route.profile;render();}catch(e){info('Source refresh failed: '+e.message,true)}}
async function flushPending(){const event=pending();if(!event||st.busy)return;
st.busy=true;render();try{const result=await request('/event',event);localStorage.removeItem(pendingKey);
info(`${event.action} ${result.replayed?'replayed':'committed'} · ${event.serial} · SIMULATED ONLY`);await refresh();
}catch(e){info('Pending event retained for safe exact-payload retry: '+e.message,true)}finally{st.busy=false;render()}}
async function action(a){if(st.busy)return;if(pending()){info('Retry or explicitly discard the pending request first.',true);return}const src=source();if(!src){info('Select a source-derived garment',true);return}const v={event_id:eventID(),lay_id:$('lay').value,plant_id:$('plant').value.trim(),serial:src.serial,action:a,actor:$('actor').value.trim(),station_id:$('station').value.trim(),defect_code:['INSPECT_FAIL','REINSPECT_FAIL'].includes(a)?$('defect').value.trim():'',step:a==='FINISH_STEP'?steps()[selected()?.finished_steps]:'',buyer_code:a==='BIND_BUYER'?$('buyer').value.trim():'',style_code:a==='BIND_BUYER'?$('style').value.trim():'',size_code:a==='BIND_BUYER'?$('size').value.trim():'',reader:{version:'QC_READER_V1',source:'UI_SIMULATED',reader_id:'UI-QC-READER-01',antenna_port:1,clicked_at:new Date().toISOString(),epc_hex:null,rssi_dbm:null}};
storePending(v);await flushPending()}
$('lay').addEventListener('change',()=>refresh(false));$('plant').addEventListener('change',()=>refresh(false));$('serial').addEventListener('change',render);$('reload').addEventListener('click',()=>refresh());
$('retry').addEventListener('click',flushPending);
$('discard').addEventListener('click',()=>{if(!pending())return;const answer=confirm('Discard only the saved UI request? If it reached PostgreSQL, its evidence remains recorded. Refresh and verify the garment state before a new action.');if(answer){localStorage.removeItem(pendingKey);info('Pending UI request discarded; database not changed. Verify current state.');refresh()}});
$('setRoute').addEventListener('click',async()=>{try{const r=await request('/route',{lay_id:$('lay').value,plant_id:$('plant').value.trim(),profile:$('profile').value});info('Simulated route '+r.profile+' created; independent approval still required');await refresh()}catch(e){info('Route rejected: '+e.message,true)}});
for(const [b,a] of Object.entries({enroll:'ENROLL',pass:'INSPECT_PASS',fail:'INSPECT_FAIL',repair:'REPAIR_COMPLETE',repass:'REINSPECT_PASS',refail:'REINSPECT_FAIL',finish:'FINISH_STEP',bind:'BIND_BUYER'}))$(b).addEventListener('click',()=>action(a));
refresh(false).then(()=>{if(pending())flushPending()});
})();
