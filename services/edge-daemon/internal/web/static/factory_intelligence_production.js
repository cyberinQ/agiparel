// Author: Sunil Jain, 2026-10-28.
// Read-only production evidence viewer; never persists identity assertions or invents KPIs.
(() => {
 'use strict';
 const $ = id => document.getElementById(id);
 const AUTH = 'AUTHENTICATED_GROUP_EVENTS_NOT_GARMENT_RELEASE';
 const labels = {
  sewing_completion_groups:'Sewing operation groups',qc_current_reviewed_groups:'QC reviewed groups',
  qc_current_hold_groups:'QC held groups',qc_current_reinspection_groups:'Awaiting reinspection',
  qc_unreviewed_groups:'Unreviewed groups',packing_current_staging_holds:'Current staging holds',
  packing_stale_staging_holds:'Stale staging holds',verified_garments:'Verified garments',
  dispatched_garments:'Dispatched garments',first_pass_yield:'First-pass yield',
  labor_efficiency:'Labor efficiency',order_margin:'Order margin'
 };
 const cards = {sewing_completion_groups:'sew',qc_current_reviewed_groups:'qc',qc_current_hold_groups:'held',packing_current_staging_holds:'stage'};
 let version = 0;
 const status = message => { $('status').textContent=message; };
 const clearView = () => {
   version++;
   Object.values(cards).forEach(id => {$(id).textContent='—';});
   $('scope').textContent='Plant and authority will appear after authentication.';
   $('measures').replaceChildren();
   $('notes').replaceChildren();
   const item=document.createElement('li');item.textContent='Load evidence to see its limits.';$('notes').append(item);
 };
 function td(tr, text) { const c=document.createElement('td');c.textContent=String(text);tr.append(c); }
 function show(report) {
   if (report.authority!==AUTH || report.production_release!==false || !Array.isArray(report.metrics)) {
     throw new Error('Unrecognized evidence authority; no numbers were displayed.');
   }
   // Validate all fields BEFORE writing any number to the DOM.
   const ids=new Set();
   const measuredGroups=new Set(['sewing_completion_groups','qc_current_reviewed_groups','qc_current_hold_groups',
      'qc_current_reinspection_groups','qc_unreviewed_groups','packing_current_staging_holds','packing_stale_staging_holds']);
   for (const m of report.metrics) {
     if (!Object.hasOwn(labels,m.id) || ids.has(m.id) || (measuredGroups.has(m.id) ? m.status!=='OBSERVED_GROUP_EVENT' || m.unit!=='groups' : m.status!=='NOT_MEASURED') ||
         (m.status==='OBSERVED_GROUP_EVENT' && (!Number.isSafeInteger(m.value) || m.value<0)) ||
         (m.status==='NOT_MEASURED' && m.value!==null)) throw new Error('Metric contract invalid; no numbers were displayed.');
     ids.add(m.id);
   }
   for (const id of Object.keys(cards)) if (!ids.has(id)) throw new Error('Incomplete metric contract.');
   $('measures').replaceChildren();
   for (const m of report.metrics) {
     const row=document.createElement('tr');
     td(row,labels[m.id]);td(row,m.status==='NOT_MEASURED'?'Not measured':m.value.toLocaleString());
     td(row,m.status);td(row,m.source);td(row,m.meaning);
     $('measures').append(row);
     if (cards[m.id]) $(cards[m.id]).textContent=m.status==='NOT_MEASURED'?'—':m.value.toLocaleString();
   }
   $('scope').textContent=`Plant: ${report.plant_id} · Tenant: ${report.tenant_id} · Snapshot: ${report.as_of} · Authority: ${report.authority} · ${report.basis}`;
   $('notes').replaceChildren();
   const source = Object.fromEntries(report.metrics.map(m => [m.id,m.value]));
   const checks = [
      ['qc_current_hold_groups','groups have a current QC HOLD; inspect the quality evidence before planning further work'],
      ['qc_current_reinspection_groups','groups await independent QC reinspection'],
      ['qc_unreviewed_groups','groups have no QC decision recorded yet'],
      ['packing_stale_staging_holds','groups have a staging HOLD that no longer matches the latest QC revision']
   ];
   for (const [id,meaning] of checks) if (source[id]>0) {
     const item=document.createElement('li');
     item.textContent=`${source[id].toLocaleString()} ${meaning} (selected cohort, not a factory-wide total).`;
     $('notes').append(item);
   }
   if (!$('notes').children.length) {
     const item=document.createElement('li');
     item.textContent='No exception group count is recorded in this selected completion cohort; this does not prove the factory has no other exceptions.';
     $('notes').append(item);
   }
   for (const note of report.limitations || []) {
     const item=document.createElement('li');item.textContent=note;$('notes').append(item);
   }
 }
 async function load() {
   clearView();
   const thisVersion=version;
   const from=$('since').value,to=$('until').value,token=$('token').value.trim();
   const a=from && new Date(from),b=to && new Date(to);
   if (!a || !b || !Number.isFinite(a.getTime()) || !Number.isFinite(b.getTime()) || a>=b) {
     return status('Provide valid ordered start and end times.');
   }
   if (!token) return status('An identity-provider assertion is required.');
   const query=new URLSearchParams({since:a.toISOString(),until:b.toISOString()});
   status('Reading tenant- and plant-scoped evidence…');
   try {
     const response=await fetch('/api/v1/intelligence/production/report?'+query.toString(),{
       headers:{Authorization:'Bearer '+token},cache:'no-store',credentials:'omit'
     });
     const result=await response.json();
     if (thisVersion!==version || $('since').value!==from || $('until').value!==to || $('token').value.trim()!==token) return;
     if (!response.ok) throw new Error(result.error||'Read failed');
     show(result);status('Source-backed group report loaded. Individual garment and financial KPIs remain unmeasured.');
   } catch (err) {if (thisVersion===version) {clearView();status('Unable to load: '+err.message);}}
 }
 $('refresh').addEventListener('click',load);
 $('clear').addEventListener('click',()=>{clearView();$('token').value='';status('View and identity assertion cleared.');});
 ['since','until','token'].forEach(id=>$(id).addEventListener('change',()=>{clearView();status('Inputs changed; reload evidence.');}));
 clearView();
})();
