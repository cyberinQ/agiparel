// Engineer Ops only. Read-only; all data rendered as text, not HTML.
(()=>{'use strict';const $=id=>document.getElementById(id);
function add(dl,label,value){const dt=document.createElement('dt'),dd=document.createElement('dd');dt.textContent=label;dd.textContent=String(value);dl.append(dt,dd)}
function fillDL(el,items){el.replaceChildren();for(const [k,v] of items)add(el,k,v)}
async function refresh(){const btn=$('refresh');btn.disabled=true;$('message').textContent='Checking live status…';try{
const response=await fetch('/api/v1/engineer-ops/overview',{credentials:'same-origin',cache:'no-store'});
if(!response.ok)throw new Error(response.status===401?'Administrator authentication required.':`Request failed (${response.status}).`);
const data=await response.json();fillDL($('service'),[['Status',data.service_status],['Release',data.release],['Environment',data.environment],['Checked',data.checked_at]]);
fillDL($('database'),[['Connection',data.database.status],['Name',data.database.name||'Unavailable'],['Role',data.database.role||'Unavailable'],['Tables visible',data.tables.length]]);
const modules=$('modules');modules.replaceChildren();for(const m of data.modules){const el=document.createElement('span');el.className='module';el.textContent=m;modules.appendChild(el)}
const catalog=$('catalog');catalog.replaceChildren();for(const t of data.tables){const tr=document.createElement('tr');for(const v of [t.schema,t.name,t.rls_enabled?'On':'Off',t.rls_forced?'Yes':'No']){const td=document.createElement('td');td.textContent=v;tr.appendChild(td)}catalog.appendChild(tr)}
$('message').textContent=data.tables_truncated?'Catalog limited to first 200 tables.':'Live status loaded.';
}catch(e){$('message').textContent=e.message}finally{btn.disabled=false}}
$('refresh').addEventListener('click',refresh);refresh()})();
