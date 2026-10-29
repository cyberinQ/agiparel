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
  dispatched_garments:'Dispatched garments',first_pass_yield:'First-pass yield (group-only report)',
  labor_efficiency:'Labor efficiency',order_margin:'Order margin'
 };
 const FACTORY_AUTH='SIGNED_UNIT_AND_CUSTODY_EVENTS_NOT_PHYSICAL_CERTIFICATION';
 const factoryLabels={
  cut_parts_accepted:'Accepted cut parts',cut_parts_rejected:'Rejected cut parts',cut_part_acceptance_pct:'Cut-part acceptance rate',
  garments_initially_inspected:'Garments initially inspected',garment_first_pass_yield_pct:'Garment first-pass yield',
  garments_quality_released:'Garments quality released',reported_cutting_consumption_mm:'Reported cutting consumption',
  classified_fabric_use_pct:'Reported fabric-use length ratio',wip_completed_handoffs:'Completed custody handoffs',
  wip_handoff_p50_ms:'Handoff interval P50',wip_handoff_p95_ms:'Handoff interval P95',
  garments_added_to_cartons:'Garments added to cartons',cartons_sealed:'Sealed cartons',manifests_staged:'Staged manifests',
  actual_fabric_utilization_pct:'Actual fabric utilization',oee_pct:'OEE',labor_efficiency_pct:'Labor efficiency',
  physically_dispatched_garments:'Verified dispatched garments'
 };
 // Exact status/units per indicator: unsupported KPIs cannot be rendered as
 // "measured" even if an upstream response accidentally supplies a number.
 const factoryContract={
  cut_parts_accepted:['parts',['SIGNED_HUMAN_EVIDENCE']],
  cut_parts_rejected:['parts',['SIGNED_HUMAN_EVIDENCE']],
  cut_part_acceptance_pct:['basis_points',['DERIVED_SIGNED_ATTESTATIONS','NO_DENOMINATOR']],
  garments_initially_inspected:['garments',['SIGNED_HUMAN_EVIDENCE']],
  garment_first_pass_yield_pct:['basis_points',['DERIVED_SIGNED_ATTESTATIONS','NO_DENOMINATOR']],
  garments_quality_released:['garments',['SIGNED_HUMAN_EVIDENCE']],
  reported_cutting_consumption_mm:['millimeters',['SIGNED_HUMAN_EVIDENCE']],
  classified_fabric_use_pct:['basis_points',['DERIVED_SIGNED_ATTESTATIONS','NO_DENOMINATOR']],
  wip_completed_handoffs:['handoffs',['SIGNED_GATEWAY_AND_HUMAN_EVIDENCE']],
  wip_handoff_p50_ms:['milliseconds',['DERIVED_RECORDED_HANDOFF','NO_HANDOFF_PAIRS','INVALID_INTERVAL']],
  wip_handoff_p95_ms:['milliseconds',['DERIVED_RECORDED_HANDOFF','NO_HANDOFF_PAIRS','INVALID_INTERVAL']],
  garments_added_to_cartons:['garments',['SIGNED_HUMAN_EVIDENCE']],
  cartons_sealed:['cartons',['SIGNED_HUMAN_EVIDENCE']],
  manifests_staged:['manifests',['STAGED_NOT_SHIPPED']],
  actual_fabric_utilization_pct:['basis_points',['NOT_MEASURED']],
  oee_pct:['basis_points',['NOT_MEASURED']],
  labor_efficiency_pct:['basis_points',['NOT_MEASURED']],
  physically_dispatched_garments:['garments',['NOT_MEASURED']]
 };
 const factoryMissing=new Set(['NO_DENOMINATOR','NO_HANDOFF_PAIRS','NOT_MEASURED','INVALID_INTERVAL']);
 const cards = {sewing_completion_groups:'sew',qc_current_reviewed_groups:'qc',qc_current_hold_groups:'held',packing_current_staging_holds:'stage'};
 let version = 0;
 let csrf = null;
 let sessionActive = false;
 const status = message => { $('status').textContent=message; };
 const clearView = () => {
   version++;
   Object.values(cards).forEach(id => {$(id).textContent='—';});
   $('scope').textContent='Plant and authority will appear after authentication.';
   $('measures').replaceChildren();
   $('factorymeasures').replaceChildren();$('factoryroutes').replaceChildren();
   $('factorybasis').textContent='No F3.6 evidence loaded.';
   $('notes').replaceChildren();
   const item=document.createElement('li');item.textContent='Load evidence to see its limits.';$('notes').append(item);
 };
 function td(tr, text) { const c=document.createElement('td');c.textContent=String(text);tr.append(c); }
 function validateFactory(report) {
   if (!report) return null; // Compatible with an older, pre-F3.6 authenticated server.
   if (report.authority!==FACTORY_AUTH || report.production_release!==false ||
       !Array.isArray(report.metrics) || !Array.isArray(report.slowest_routes) ||
       report.slowest_routes.length>10 || typeof report.basis!=='string' ||
       !Array.isArray(report.limitations)) throw new Error('Invalid unit evidence authority.');
   const seen=new Set();
   for (const m of report.metrics) {
     const contract=factoryContract[m.id];
     if (!contract || seen.has(m.id) || m.unit!==contract[0] || !contract[1].includes(m.status) ||
         typeof m.source!=='string' || !m.source.trim() || typeof m.meaning!=='string' || !m.meaning.trim() ||
         (factoryMissing.has(m.status) ? m.value!==null : !Number.isSafeInteger(m.value) || m.value<0) ||
         (m.unit==='basis_points' && m.value!==null && m.value>10000))
       throw new Error('Invalid unit evidence metric.');
     if (m.unit==='basis_points' && m.status!=='NOT_MEASURED') {
       if (!Number.isSafeInteger(m.numerator) || !Number.isSafeInteger(m.denominator) ||
           m.numerator<0 || m.denominator<0 || m.numerator>m.denominator ||
           (m.status==='NO_DENOMINATOR' ? m.denominator!==0 : m.denominator===0))
         throw new Error('Invalid unit evidence denominator.');
     }
     seen.add(m.id);
   }
   if (seen.size!==Object.keys(factoryContract).length) throw new Error('Incomplete unit evidence metrics.');
   for (const rt of report.slowest_routes) {
     if (!Number.isSafeInteger(rt.completed_handoffs) || rt.completed_handoffs<=0 ||
         !Number.isSafeInteger(rt.p50_milliseconds) || rt.p50_milliseconds<0 ||
         !Number.isSafeInteger(rt.p95_milliseconds) || rt.p95_milliseconds<rt.p50_milliseconds ||
         rt.evidence!=='SIGNED_CUSTODY_HANDOFF_INTERVAL_NOT_MACHINE_CYCLE_TIME' ||
         typeof rt.from_station!=='string' || typeof rt.to_station!=='string')
       throw new Error('Invalid handoff evidence.');
   }
   return report;
 }
 function factoryDisplay(m) {
   if (factoryMissing.has(m.status)) return 'Not measured';
   if (m.unit==='basis_points') return (m.value/100).toFixed(2)+'%';
   if (m.unit==='milliseconds') return (m.value/1000).toFixed(3)+' s';
   if (m.unit==='millimeters') return (m.value/1000).toFixed(3)+' m';
   return m.value.toLocaleString();
 }
 function showFactory(report) {
   $('factorymeasures').replaceChildren();$('factoryroutes').replaceChildren();
   if (!report) {
     $('factorybasis').textContent='Unit-level evidence is not available from this server version.';
     return;
   }
   for (const m of report.metrics) {
     const row=document.createElement('tr');
     td(row,factoryLabels[m.id]);td(row,factoryDisplay(m));td(row,m.status);td(row,m.source);td(row,m.meaning);
     $('factorymeasures').append(row);
   }
   for (const rt of report.slowest_routes) {
     const row=document.createElement('tr');
     td(row,rt.from_station);td(row,rt.to_station);td(row,rt.completed_handoffs.toLocaleString());
     td(row,(rt.p50_milliseconds/1000).toFixed(3)+' s');td(row,(rt.p95_milliseconds/1000).toFixed(3)+' s');
     $('factoryroutes').append(row);
   }
   $('factorybasis').textContent=report.basis+' Limitations: '+report.limitations.join(' ');
 }
 function show(report) {
   if (report.authority!==AUTH || report.production_release!==false || !Array.isArray(report.metrics)) {
     throw new Error('Unrecognized evidence authority; no numbers were displayed.');
   }
   // Validate ALL evidence authority before writing any number to the DOM.
   const factory=validateFactory(report.factory_evidence);
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
   showFactory(factory);
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
   const mode=$('authmode').value;
   if (mode!=='session' && !token) return status('An identity-provider assertion is required.');
   if (mode==='session' && !sessionActive) return status('Sign in with SSO before loading evidence.');
   const tenant=$('tenant').value.trim(),plant=$('plant').value.trim();
   if ((mode==='oidc' || mode==='session') && (!/^[A-Za-z0-9_-]{2,64}$/.test(tenant) || !/^[A-Za-z0-9_-]{2,64}$/.test(plant))) {
     return status('Enter an authorized tenant and plant code.');
   }
   const query=new URLSearchParams({since:a.toISOString(),until:b.toISOString()});
   if (mode==='oidc' || mode==='session') {query.set('tenant_id',tenant);query.set('plant_id',plant);}
   status('Reading tenant- and plant-scoped evidence…');
   try {
     const response=await fetch('/api/v1/intelligence/production/report?'+query.toString(),{
       headers:mode==='session'?{}:{Authorization:'Bearer '+token},cache:'no-store',credentials:mode==='session'?'same-origin':'omit'
     });
     const result=await response.json();
     if (thisVersion!==version || $('since').value!==from || $('until').value!==to || $('token').value.trim()!==token || $('authmode').value!==mode || ((mode==='oidc' || mode==='session') && ($('tenant').value.trim()!==tenant || $('plant').value.trim()!==plant))) return;
     if (!response.ok) throw new Error(result.error||'Read failed');
     show(result);status('Authenticated group and unit-event evidence loaded. No physical shipping, OEE, labor or financial KPIs are inferred.');
   } catch (err) {if (thisVersion===version) {clearView();status('Unable to load: '+err.message);}}
 }
 $('refresh').addEventListener('click',load);
 $('clear').addEventListener('click',()=>{clearView();$('token').value='';status('View and identity assertion cleared.');});
 $('authmode').addEventListener('change',()=>{
   $('oidcscope').hidden=!['oidc','session'].includes($('authmode').value);
   clearView();status('Authentication mode changed; choose an authenticated identity.');
 });
 ['since','until','token','tenant','plant'].forEach(id=>$(id).addEventListener('change',()=>{clearView();status('Inputs changed; reload evidence.');}));
 async function checkSession() {
   csrf=null;sessionActive=false;
   try {
     const response=await fetch('/api/v1/workforce/session',{credentials:'same-origin',cache:'no-store'});
     if (!response.ok) throw new Error('Not signed in');
     const data=await response.json();
     if (data.authenticated!==true || typeof data.csrf!=='string') throw new Error('Session invalid');
     csrf=data.csrf;sessionActive=true;
     $('authmode').value='session';$('oidcscope').hidden=false;
     $('session-status').textContent='Signed in as '+data.subject+' (plant permissions checked when reporting).';
   } catch (_) { $('session-status').textContent='No active SSO session. Use Sign in with SSO.'; }
 }
 $('signin').addEventListener('click',()=>{window.location.assign('/auth/login');});
 $('signout').addEventListener('click',async()=>{
   if (!csrf) return $('session-status').textContent='No active SSO session.';
   try {
     const response=await fetch('/auth/logout',{method:'POST',credentials:'same-origin',headers:{'X-Agiparel-CSRF':csrf},cache:'no-store'});
     if (!response.ok) throw new Error('Sign-out failed');
     csrf=null;sessionActive=false;clearView();
     $('session-status').textContent='Signed out. Session revoked.';
   } catch (_) { $('session-status').textContent='Sign-out failed. Try again.'; }
 });
 clearView();
 checkSession();
})();
