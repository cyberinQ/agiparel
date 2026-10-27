// CyberInQ E11: reader-shaped UI simulation; no EPC/RSSI claims.
(()=>{'use strict';
const $=id=>document.getElementById(id),api='/api/v1/packing/sim',key='agiparel.pack.v11.pending';
let data={candidates:[],cartons:[],manifests:[]},busy=false;
const esc=s=>String(s??'').replace(/[&<>"']/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
const pending=()=>{try{return JSON.parse(localStorage.getItem(key)||'null')}catch{return null}};
const status=(s)=>{$('feedback').textContent=s};
async function request(path,body){const r=await fetch(api+path,{method:body?'POST':'GET',headers:body?{'Content-Type':'application/json'}:{},body:body?JSON.stringify(body):undefined,cache:'no-store'});const d=await r.json();if(!r.ok)throw Error(d.error||'HTTP '+r.status);return d}
function selected(){return data.candidates.find(x=>x.serial===$('serial').value)}
function render(){const s=selected();$('garmentInfo').textContent=s?`${s.buyer_code} · ${s.color}/${s.size} · ${s.plant_id} · ${s.carton?'Already packed: '+s.carton:'Available'}`:'No ready piece selected';$('pending').textContent=pending()?'Tap waiting to send. Try again before you continue.':'Nothing waiting to send';$('retry').disabled=!pending()||busy;$('discard').disabled=!pending()||busy;
$('cartons').innerHTML=data.cartons.map(x=>`<tr><td>${esc(x.carton)}</td><td>${esc(x.plant_id)}</td><td>${esc(x.state)}</td><td>${x.items}</td><td>${esc(x.manifest)}</td></tr>`).join('')||'<tr><td colspan="5">No cartons yet</td></tr>';
$('manifests').innerHTML=data.manifests.map(x=>`<tr><td>${esc(x.manifest)}</td><td>${esc(x.plant_id)}</td><td>${esc(x.state)}</td><td>${x.cartons}</td><td>${x.garments}</td></tr>`).join('')||'<tr><td colspan="5">No manifests yet</td></tr>';
document.querySelectorAll('[data-action]').forEach(b=>b.disabled=busy||!!pending());}
async function refresh(){const previous=$('serial').value;try{data=await request('/catalog');$('serial').innerHTML='<option value="">Select eligible garment</option>'+data.candidates.map(x=>`<option value="${esc(x.serial)}">${esc(x.serial)} · ${esc(x.buyer_code)}${x.carton?' · PACKED':''}</option>`).join('');if(data.candidates.some(x=>x.serial===previous))$('serial').value=previous;render()}catch(e){status('Could not load pieces: '+e.message)}}
async function flush(){const v=pending();if(!v||busy)return;busy=true;render();try{const r=await request('/event',v);localStorage.removeItem(key);status(`${v.action} ${r.replayed?'replayed':'committed'} · DEMO ONLY`);await refresh()}catch(e){status('Event retained for exact replay: '+e.message)}finally{busy=false;render()}}
async function act(action){if(busy||pending()){status('Try sending the last tap again, or clear it before continuing.');return}const s=selected(),v={event_id:'PK-'+crypto.randomUUID().replace(/-/g,''),action,carton_code:$('carton').value.trim(),manifest_code:$('manifest').value.trim(),lay_id:'',plant_id:'',serial:'',actor:$('actor').value.trim(),station_id:$('station').value.trim(),reader:{version:'PACK_READER_V1',source:'UI_SIMULATED',reader_id:$('reader').value.trim(),antenna_port:1,clicked_at:new Date().toISOString(),epc_hex:null,rssi_dbm:null}};
if(action==='OPEN_CARTON'){if(!s){status('Choose a garment to derive the lay and plant');return}v.lay_id=s.lay_id;v.plant_id=s.plant_id;v.manifest_code=''}
if(['ADD_GARMENT','REMOVE_GARMENT'].includes(action)){if(!s){status('Choose a garment');return}v.serial=s.serial;v.manifest_code=''}
if(action==='SEAL_CARTON')v.manifest_code='';
if(action==='LOAD_CARTON'){v.serial='';v.lay_id='';v.plant_id=''}
if(action==='OPEN_MANIFEST'){v.plant_id=$('plant').value.trim();v.carton_code=''}
if(action==='DISPATCH_REHEARSAL')v.carton_code='';
localStorage.setItem(key,JSON.stringify(v));await flush()}
$('refresh').onclick=refresh;$('retry').onclick=flush;$('discard').onclick=()=>{if(confirm('Discard the saved UI request only? The server may have committed it. Refresh before retrying.')){localStorage.removeItem(key);refresh()}};$('serial').onchange=render;document.querySelectorAll('[data-action]').forEach(b=>b.onclick=()=>act(b.dataset.action));refresh().then(()=>{if(pending())flush()});
})();
