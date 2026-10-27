// Batch 08B: operator reported cutting evidence. Never claims production release.
'use strict';
const spreadAPI='/api/v1/cutting/spreads';
let spreadDraft=null;
const spreadEl=id=>document.getElementById(id);
const spreadMsg=value=>{spreadEl('spread-message').textContent=value;};
const spreadText=(tag,value)=>{const el=document.createElement(tag);el.textContent=String(value);return el;};
async function spreadFetch(url,opts={}){
 const response=await fetch(url,{cache:'no-store',...opts});const raw=await response.text();let data;
 try{data=JSON.parse(raw);}catch{throw new Error(raw.slice(0,160)||`HTTP ${response.status}`);}
 if(!response.ok)throw new Error(data.error||`HTTP ${response.status}`);
 return data;
}
async function spreadRefresh(){
 if(!spreadDraft)return;
 const requestedDraftID=spreadDraft.draft_id;
 try{
  const data=await spreadFetch(`${spreadAPI}?draft_id=${encodeURIComponent(requestedDraftID)}`);
  if(!spreadDraft||spreadDraft.draft_id!==requestedDraftID)return;
  const issues=spreadEl('spread-issues');issues.replaceChildren();
  for(const i of data.matching_signed_issues||[]){const tr=document.createElement('tr');for(const value of [i.issue_id,i.roll_id,i.issued_mm,i.already_attributed_mm,i.available_for_attribution_mm])tr.append(spreadText('td',value));issues.append(tr);}
  if(!issues.children.length){const tr=document.createElement('tr');const td=spreadText('td','No eligible signed issues. An independently witnessed stores issuance is required before reporting material consumption.');td.colSpan=5;tr.append(td);issues.append(tr);}
  const reg=spreadEl('spread-register');reg.replaceChildren();
  for(const s of data.spreads||[]){reg.append(spreadText('p',`${s.report_id} · ${s.actual_plies} plies · ${s.reported_consumption_mm} mm · ${s.state} · signed label witnesses ${s.signed_bundle_witnesses}/${s.draft_bundle_count}`));}
  if(!(data.spreads||[]).length)reg.append(spreadText('p','No spread evidence recorded.'));
  const witnesses=spreadEl('spread-scans');witnesses.replaceChildren();
  for(const w of data.bundle_witnesses||[])witnesses.append(spreadText('p',`${w.label_code} · EPC ${w.epc_hex} · ${w.witness_subject} · ${w.state}`));
  if(!(data.bundle_witnesses||[]).length)witnesses.append(spreadText('p','No signed EPC witnesses; printable 08A labels are not RFID reads.'));
  const hasReport=(data.spreads||[]).length>0;
  spreadEl('save-spread').disabled=hasReport||!(data.matching_signed_issues||[]).some(i=>i.available_for_attribution_mm>0);
  spreadMsg(hasReport?'Immutable report already exists for this draft. A new revision cannot overwrite physical evidence.':'Report requires matching signed issued mm; physical cut and RFID reader evidence remain unverified.');
 }catch(e){spreadEl('save-spread').disabled=true;spreadMsg(`Cannot load issue evidence: ${e.message}`);}
}
document.addEventListener('cut-draft-selected',({detail})=>{
 spreadDraft=detail.draft;
 spreadEl('spread-selection').textContent=`${spreadDraft.po_number} · Cut ${spreadDraft.cut_no} / revision ${spreadDraft.revision} · ${spreadDraft.fabric_sku} · shade ${spreadDraft.shade_band}`;
 const fields=spreadEl('spread-form').elements;
 fields.namedItem('actual_plies').value=spreadDraft.reported_plies;
 fields.namedItem('measured_marker_length_mm').value=spreadDraft.marker_length_mm;
 spreadEl('save-spread').disabled=true;
 spreadRefresh();
});
spreadEl('spread-form').addEventListener('submit',async(event)=>{
 event.preventDefault();if(!spreadDraft)return;
 try{
 const form=event.currentTarget;const data=new FormData(form);const entries=String(data.get('issue_allocations')).split(/\r?\n/).map(s=>s.trim()).filter(Boolean);
 const allocations=entries.map(s=>{const cols=s.split(',').map(x=>x.trim());if(cols.length!==2)throw new Error('Each allocation must contain issue UUID,allocated_mm');return {issue_id:cols[0],allocated_mm:Number(cols[1])};});
 const key=new Uint8Array(16);crypto.getRandomValues(key);
 const input={draft_id:spreadDraft.draft_id,request_key:'spread_'+Array.from(key,x=>x.toString(16).padStart(2,'0')).join(''),
  actual_plies:Number(data.get('actual_plies')),measured_marker_length_mm:Number(data.get('measured_marker_length_mm')),
  reported_consumption_mm:Number(data.get('reported_consumption_mm')),issue_allocations:allocations,evidence_note:String(data.get('evidence_note')).trim()};
  spreadEl('save-spread').disabled=true;
  const saved=await spreadFetch(spreadAPI,{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(input)});
  spreadMsg(`Saved immutable ${saved.spread.report_id}; remains NOT RELEASED.`);
  await spreadRefresh();
 }catch(e){spreadMsg(`Spread evidence rejected: ${e.message}`);spreadEl('save-spread').disabled=false;}
});
