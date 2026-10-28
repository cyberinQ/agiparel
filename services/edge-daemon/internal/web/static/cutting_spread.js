// Author: Sunil Jain, 2026-10-28
// Chapter 05: reconcile measured spread with signed issue balances before draft submission.
'use strict';
const spreadAPI = '/api/v1/cutting/spreads';
let spreadDraft=null;
let spreadIssues=[];
let spreadReports=[];
let spreadGeneration=0;
let spreadPendingKey='';
let spreadPendingSignature='';
const spreadEl=id=>document.getElementById(id);
const spreadMsg=value=>{spreadEl('spread-message').textContent=value;};
const spreadText=(tag,value)=>{const el=document.createElement(tag);el.textContent=String(value);return el;};
async function spreadFetch(url,opts={}) {
 const response=await fetch(url,{cache:'no-store',...opts});const raw=await response.text();let data;
 try{data=JSON.parse(raw);}catch{throw new Error(raw.slice(0,160)||`HTTP ${response.status}`);}
 if(!response.ok)throw new Error(data.error||`HTTP ${response.status}`);
 return data;
}
function spreadCheck(){
 const fields=spreadEl('spread-form').elements;
 const plies=Number(fields.namedItem('actual_plies').value);
 const marker=Number(fields.namedItem('measured_marker_length_mm').value);
 const used=Number(fields.namedItem('reported_consumption_mm').value);
 const output=spreadEl('spread-check');
 if(![plies,marker,used].every(Number.isSafeInteger) || plies<=0 || marker<=0 || used<=0) {
  output.textContent='Enter measured plies, marker and actual fabric used (mm); reconcile the whole amount to signed issue IDs.';
  return;
 }
 const minimum=marker*plies;
 const difference=used-minimum;
 const open=spreadIssues.reduce((sum,i)=>sum+i.available_for_attribution_mm,0);
 output.textContent=`Measured marker × plies: ${(minimum/1000).toFixed(3)} m minimum. Reported: ${(used/1000).toFixed(3)} m. Difference: ${(difference/1000).toFixed(3)} m. Open signed issues: ${(open/1000).toFixed(3)} m (not automatically consumed).${difference<0?' BLOCK: reported fabric is below the marker minimum.':''}`;
}
function spreadClear() {
 spreadGeneration++;
 spreadDraft=null;spreadIssues=[];spreadReports=[];
 spreadPendingKey='';spreadPendingSignature='';
 spreadEl('save-spread').disabled=true;
 spreadEl('spread-selection').textContent='Select a cutting draft before reporting measured fabric usage.';
 spreadEl('spread-issues').replaceChildren();
 spreadEl('spread-register').textContent='No selected lay.';
 spreadEl('spread-scans').textContent='No selected lay.';
 const form=spreadEl('spread-form').elements;
 for(const name of ['actual_plies','measured_marker_length_mm','reported_consumption_mm','issue_allocations','evidence_note']) form.namedItem(name).value='';
 spreadCheck();spreadMsg('No cutting draft selected.');
}
async function spreadRefresh() {
 if(!spreadDraft)return;
 const requestedDraftID=spreadDraft.draft_id;
 const generation=++spreadGeneration;
 spreadEl('save-spread').disabled=true;
 try{
  const data=await spreadFetch(`${spreadAPI}?draft_id=${encodeURIComponent(requestedDraftID)}`);
  if(generation!==spreadGeneration || !spreadDraft || spreadDraft.draft_id!==requestedDraftID)return;
  spreadIssues=data.matching_signed_issues||[];spreadReports=data.spreads||[];
  const issues=spreadEl('spread-issues');issues.replaceChildren();
  for(const i of spreadIssues){const tr=document.createElement('tr');for(const value of [i.issue_id,i.roll_id,i.issued_mm,i.already_attributed_mm,i.available_for_attribution_mm])tr.append(spreadText('td',value));issues.append(tr);}
  if(!issues.children.length){const tr=document.createElement('tr');const td=spreadText('td','No eligible signed issues. Independent stores issuance is required before reporting material consumption.');td.colSpan=5;tr.append(td);issues.append(tr);}
  const reg=spreadEl('spread-register');reg.replaceChildren();
  for(const s of spreadReports)reg.append(spreadText('p',`${s.report_id} · ${s.actual_plies} plies · ${s.reported_consumption_mm} mm · ${s.state} · signed label witnesses ${s.signed_bundle_witnesses}/${s.draft_bundle_count}`));
  if(!spreadReports.length)reg.append(spreadText('p','No spread evidence recorded.'));
  const witnesses=spreadEl('spread-scans');witnesses.replaceChildren();
  for(const w of data.bundle_witnesses||[])witnesses.append(spreadText('p',`${w.label_code} · EPC ${w.epc_hex} · ${w.witness_subject} · ${w.state}`));
  if(!(data.bundle_witnesses||[]).length)witnesses.append(spreadText('p','No signed EPC witnesses; printable draft labels are not RFID reads.'));
  spreadEl('save-spread').disabled=spreadReports.length>0 || !spreadIssues.some(i=>i.available_for_attribution_mm>0);
  spreadCheck();
  document.dispatchEvent(new CustomEvent('cut-spread-refreshed',{detail:{draft_id:requestedDraftID}}));
  spreadMsg(spreadReports.length ? 'Immutable spread report already exists. A new draft revision cannot overwrite physical evidence.' :
   'Signed issues loaded. Report exact measured mm; physical cut and RFID reader evidence remain unverified.');
 }catch(e){if(generation===spreadGeneration){spreadEl('save-spread').disabled=true;spreadMsg(`Cannot load issue evidence: ${e.message}`);}}
}
function spreadReadInput(form){
 if(!spreadDraft || spreadReports.length)throw new Error('Select a draft without an existing spread report');
 const data=new FormData(form);
 const entries=String(data.get('issue_allocations')||'').split(/\r?\n/).map(s=>s.trim()).filter(Boolean);
 if(entries.length<1 || entries.length>20)throw new Error('Specify 1–20 signed issue allocations');
 const allocations=entries.map((s,i)=>{
  const cols=s.split(',').map(x=>x.trim());
  if(cols.length!==2 || !/^[0-9a-fA-F-]{36}$/.test(cols[0]))throw new Error(`Allocation row ${i+1}: expected issue UUID,allocated_mm`);
  const amount=Number(cols[1]);
  if(!Number.isSafeInteger(amount) || amount<=0)throw new Error(`Allocation row ${i+1}: positive whole millimetres required`);
  return {issue_id:cols[0],allocated_mm:amount};
 });
 const input={draft_id:spreadDraft.draft_id,actual_plies:Number(data.get('actual_plies')),
  measured_marker_length_mm:Number(data.get('measured_marker_length_mm')),
  reported_consumption_mm:Number(data.get('reported_consumption_mm')),
  issue_allocations:allocations,evidence_note:String(data.get('evidence_note')||'').trim()};
 for(const name of ['actual_plies','measured_marker_length_mm','reported_consumption_mm'])
  if(!Number.isSafeInteger(input[name]) || input[name]<=0)throw new Error(`${name}: positive whole number required`);
 if(input.actual_plies!==spreadDraft.reported_plies)throw new Error('Actual plies differ from the lay; revise the lay before reporting');
 if(input.reported_consumption_mm<input.measured_marker_length_mm*input.actual_plies)throw new Error('Fabric consumption below measured marker × plies');
 if(new Set(allocations.map(a=>a.issue_id.toLowerCase())).size!==allocations.length)throw new Error('Each signed issue may appear only once');
 const total=allocations.reduce((sum,a)=>sum+a.allocated_mm,0);
 if(total!==input.reported_consumption_mm)throw new Error('Signed issue allocations must exactly equal reported consumption (mm)');
 const available=new Map(spreadIssues.map(i=>[i.issue_id.toLowerCase(),i.available_for_attribution_mm]));
 for(const a of allocations)if(available.has(a.issue_id.toLowerCase()) && a.allocated_mm>available.get(a.issue_id.toLowerCase()))
  throw new Error(`Signed issue ${a.issue_id} has insufficient unattributed mm`);
 const signature=JSON.stringify(input);
 if(signature!==spreadPendingSignature){
  const key=new Uint8Array(16);crypto.getRandomValues(key);
  spreadPendingKey='spread_'+Array.from(key,x=>x.toString(16).padStart(2,'0')).join('');
  spreadPendingSignature=signature;
 }
 return {...input,request_key:spreadPendingKey};
}
document.addEventListener('cut-draft-cleared',spreadClear);
document.addEventListener('cut-draft-selected',({detail})=>{
 spreadGeneration++;
 spreadDraft=detail.draft;
 spreadIssues=[];spreadReports=[];spreadPendingKey='';spreadPendingSignature='';
 spreadEl('spread-selection').textContent=`${spreadDraft.po_number} · Cut ${spreadDraft.cut_no} / revision ${spreadDraft.revision} · ${spreadDraft.fabric_sku} · shade ${spreadDraft.shade_band}`;
 const fields=spreadEl('spread-form').elements;
 fields.namedItem('actual_plies').value=spreadDraft.reported_plies;
 fields.namedItem('measured_marker_length_mm').value=spreadDraft.marker_length_mm;
 fields.namedItem('reported_consumption_mm').value='';
 fields.namedItem('issue_allocations').value='';
 fields.namedItem('evidence_note').value='';
 spreadEl('save-spread').disabled=true;
 spreadCheck();spreadRefresh();
});
spreadEl('spread-form').addEventListener('submit',async(event)=>{
 event.preventDefault();const target=spreadDraft;
 const btn=spreadEl('save-spread');
 try{
  if(btn.disabled || !target)throw new Error('Choose a draft and refresh signed issue evidence first');
  const input=spreadReadInput(event.currentTarget);
  btn.disabled=true;
  const saved=await spreadFetch(spreadAPI,{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(input)});
  spreadPendingKey='';spreadPendingSignature='';
  if(spreadDraft?.draft_id===target.draft_id){
   await spreadRefresh();
   spreadMsg(`Spread saved: ${saved.spread.report_id}. Cutting remains on hold until signed reader and approval evidence exists.`);
  }
 }catch(e){
  if(spreadDraft?.draft_id===target?.draft_id){
   const message=`Spread not saved: ${e.message}. Retrying unchanged inputs preserves the request key.`;
   await spreadRefresh();spreadMsg(message);
  }
 }
});
for(const name of ['actual_plies','measured_marker_length_mm','reported_consumption_mm'])
 spreadEl('spread-form').elements.namedItem(name).addEventListener('input',spreadCheck);
spreadCheck();
