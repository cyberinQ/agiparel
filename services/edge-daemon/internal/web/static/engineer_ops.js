// Author: Sunil Jain, CyberinQ LLC, 2026-10-29.
// Restricted, read-only Engineer Ops. Render server data using textContent exclusively.
(() => {'use strict';
const $ = id => document.getElementById(id);
let knownAPIs=[];
function add(dl,label,value){const dt=document.createElement('dt'),dd=document.createElement('dd');dt.textContent=label;dd.textContent=String(value);dl.append(dt,dd);}
function fillDL(el,items){el.replaceChildren();for(const [k,v] of items)add(el,k,v);}
function showMetric(id,val){$(id).textContent=typeof val==='number'&&Number.isFinite(val)?val.toLocaleString('en-US'):'Unavailable';}
function apiArea(path){const p=path.split('/').filter(Boolean);return p[0]==='api'?(p[2]||'General'):p[0]||'General';}
function renderAPIs(){const q=$('api-filter').value.trim().toLowerCase();const filtered=knownAPIs.filter(x=>x.toLowerCase().includes(q)||apiArea(x).toLowerCase().includes(q));const rows=$('api-rows');rows.replaceChildren();for(const path of filtered){const tr=document.createElement('tr');for(const val of [apiArea(path),path]){const td=document.createElement('td');td.textContent=val;tr.appendChild(td);}rows.appendChild(tr);}$('api-result-count').textContent=filtered.length+' of '+knownAPIs.length+' registered paths displayed.';}
async function refresh(){const btn=$('refresh');btn.disabled=true;$('message').textContent='Checking live status…';try{
 const response=await fetch('/api/v1/engineer-ops/overview',{credentials:'same-origin',cache:'no-store'});
 if(!response.ok)throw new Error(response.status===401?'Administrator authentication required.':`Request failed (${response.status}).`);
 const data=await response.json();fillDL($('service'),[['Status',data.service_status],['Release',data.release],['Environment',data.environment],['Checked',data.checked_at]]);
 fillDL($('database'),[['Connection',data.database.status],['Name',data.database.name||'Unavailable'],['Role',data.database.role||'Unavailable'],['Tables visible',data.tables.length]]);
 const modules=$('modules');modules.replaceChildren();for(const m of data.modules){const el=document.createElement('span');el.className='module';el.textContent=m;modules.appendChild(el);}
 const catalog=$('catalog');catalog.replaceChildren();for(const t of data.tables){const tr=document.createElement('tr');for(const v of [t.schema,t.name,t.rls_enabled?'On':'Off',t.rls_forced?'Yes':'No']){const td=document.createElement('td');td.textContent=v;tr.appendChild(td);}catalog.appendChild(tr);}
 $('db-summary').textContent=data.tables.length+(data.tables_truncated?'+':'')+' tables';
 const analytics=data.analytics||{};showMetric('customer-count',analytics.customer_count);showMetric('user-count',analytics.user_count);showMetric('usage-count',analytics.requests_since_start);
 $('analytics-status').textContent=analytics.source_status||'Unavailable; count queries were not completed.';
 knownAPIs=Array.isArray(data.apis)?data.apis:[];renderAPIs();$('api-summary').textContent=knownAPIs.length+' registered API paths';
 $('message').textContent=data.tables_truncated?'Catalog limited to first 200 tables.':'Live status loaded.';
}catch(e){$('message').textContent=e.message;}finally{btn.disabled=false;}}
$('api-filter').addEventListener('input',renderAPIs);
$('refresh').addEventListener('click',refresh);refresh();
})();
