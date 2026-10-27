// Batch 08A. DRAFT labels and plies are not physical cut or RFID evidence.
'use strict';
const cutAPI='/api/v1/cutting/lay-drafts';
let cutDrafts=[];
let cutSelected=null;
const cutEl=id=>document.getElementById(id);
const cutMsg=message=>{cutEl('cut-message').textContent=message;};
function cutNode(tag,text,cls){const node=document.createElement(tag); if(text!==undefined)node.textContent=String(text);if(cls)node.className=cls;return node;}
async function cutFetch(url,options){const response=await fetch(url,{cache:'no-store',...options});const body=await response.text();let data;try{data=JSON.parse(body);}catch{throw new Error(body.slice(0,240)||`HTTP ${response.status}`);}if(!response.ok)throw new Error(data.error||`HTTP ${response.status}`);return data;}
function cutCsv(value,columns,label){const lines=value.split(/\r?\n/).map(v=>v.trim()).filter(Boolean);if(!lines.length)throw new Error(`${label}: add at least one row`);return lines.map((line,i)=>{const cells=line.split(',').map(v=>v.trim());if(cells.length!==columns)throw new Error(`${label} row ${i+1}: expected ${columns} columns`);return cells;});}
function cutReadInput(form){
 const d=new FormData(form);
 const positions=cutCsv(String(d.get('positions')),3,'Positions').map(([position,color_code,size_code])=>({position:Number(position),color_code,size_code}));
 const parts=cutCsv(String(d.get('parts')),2,'Parts').map(([part_code,pieces_per_garment])=>({part_code,pieces_per_garment:Number(pieces_per_garment)}));
 const keyBytes=new Uint8Array(16);crypto.getRandomValues(keyBytes);
 const request_key='cut_'+Array.from(keyBytes,b=>b.toString(16).padStart(2,'0')).join('');
 return {intake_id:d.get('intake_id'),cut_no:Number(d.get('cut_no')),expected_revision:Number(d.get('expected_revision')),
 request_key,marker_code:d.get('marker_code').trim(),fabric_sku:d.get('fabric_sku').trim(),shade_band:d.get('shade_band').trim(),
 marker_length_mm:Number(d.get('marker_length_mm')),usable_width_mm:Number(d.get('usable_width_mm')),
 reported_plies:Number(d.get('reported_plies')),bundle_ply_group_size:Number(d.get('bundle_ply_group_size')),positions,parts,note:d.get('note').trim()};
}
function cutRenderDraft(report){
 cutSelected=report;cutEl('print-cut').disabled=false;
 cutEl('cut-title').textContent=`${report.po_number} · Cut ${report.cut_no} · Rev ${report.revision} · ${report.marker_code} · ${report.state}`;
 const summary=cutEl('cut-summary');summary.replaceChildren();
 for(const [label,value] of [['Planned garments',report.planned_garments],['Reported plies',report.reported_plies],['Max plies / sub-bundle',report.bundle_ply_group_size],['Draft bundles',report.bundle_count],['Part pieces',report.planned_parts],['Signed issue entries',report.verified_issue_count],['Issued mm',report.verified_issued_length_mm],['RFID','NOT BOUND']])summary.append(cutNode('span',`${label}: ${value}`));
 const body=cutEl('cut-bundles');body.replaceChildren();
 for(const b of report.bundles||[]){const tr=document.createElement('tr');for(const value of [b.label_code,b.position,`${b.color_code} / ${b.size_code}`,b.part_code,b.ply_group,`${b.ply_start}–${b.ply_end}`,b.piece_count,b.rfid_state])tr.append(cutNode('td',value));body.append(tr);}
 if(!(report.bundles||[]).length){const tr=document.createElement('tr');const td=cutNode('td','No bundles found.');td.colSpan=8;tr.append(td);body.append(tr);}
 cutMsg(report.warning||'Draft only. No physical cutting release.');
 document.dispatchEvent(new CustomEvent('cut-draft-selected',{detail:{draft:report}}));
}
async function cutOpen(id){try{const data=await cutFetch(`${cutAPI}?draft_id=${encodeURIComponent(id)}`);if(!data.drafts.length)throw new Error('Draft no longer found');cutRenderDraft(data.drafts[0]);}catch(e){cutMsg(`Could not load draft: ${e.message}`);}}
function cutRenderList(){const list=cutEl('cut-register');list.replaceChildren();if(!cutDrafts.length){list.append(cutNode('p','No cutting drafts yet. Create a lay from a buyer order.','hint'));return;}
 for(const d of cutDrafts){const btn=cutNode('button',undefined,'cut-entry');btn.type='button';btn.append(cutNode('strong',`${d.po_number} · Cut ${d.cut_no} / Rev ${d.revision}`));btn.append(cutNode('small',`${d.marker_code} · ${d.reported_plies} reported plies · ${d.planned_garments} planned units`));btn.append(cutNode('span',d.state,'pill'));btn.addEventListener('click',()=>cutOpen(d.draft_id));list.append(btn);}}
async function cutRefresh(){try{const data=await cutFetch(cutAPI);cutDrafts=data.drafts||[];const selector=cutEl('cut-order');const previous=selector.value;selector.replaceChildren();selector.append(new Option('Select buyer PO',''));
 for(const o of data.orders||[])selector.append(new Option(`${o.po_number} · ${o.style_code} · ${o.ordered_qty} pcs`,o.intake_id));
 selector.value=previous;
 cutEl('metric-lays').textContent=cutDrafts.length;cutEl('metric-garments').textContent=cutDrafts.reduce((s,d)=>s+d.planned_garments,0).toLocaleString();cutEl('metric-bundles').textContent=cutDrafts.reduce((s,d)=>s+d.bundle_count,0).toLocaleString();cutRenderList();
 if(!(data.orders||[]).length)cutMsg('No staged buyer orders yet. Open Order Intake / Buyer Review before planning a cut.');else cutMsg('Cutting register loaded. Physical release remains blocked.');
 }catch(e){cutMsg(`Cutting database unavailable: ${e.message}`);}}
cutEl('cut-form').addEventListener('submit',async event=>{event.preventDefault();const button=cutEl('save-cut');try{button.disabled=true;const input=cutReadInput(event.currentTarget);const data=await cutFetch(cutAPI,{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(input)});await cutRefresh();cutRenderDraft(data.draft);cutEl('cut-form').elements.namedItem('expected_revision').value=data.draft.revision;}catch(e){cutMsg(`Draft not saved: ${e.message}`);}finally{button.disabled=false;}});
cutEl('refresh-cut').addEventListener('click',cutRefresh);
cutEl('print-cut').addEventListener('click',()=>{if(cutSelected)window.print();});
cutRefresh();

function cutUpdateExpectedRevision(){
 const order=cutEl('cut-order').value;
 const cutNo=Number(cutEl('cut-form').elements.namedItem('cut_no').value);
 const existing=cutDrafts.find(d=>d.intake_id===order && d.cut_no===cutNo);
 cutEl('cut-form').elements.namedItem('expected_revision').value=existing?existing.revision:0;
}
cutEl('cut-order').addEventListener('change',async()=>{
 const id=cutEl('cut-order').value;
 cutUpdateExpectedRevision();
 const facts=cutEl('cut-source-facts');
 if(!id){facts.textContent='Select a buyer order to inspect its SKU matrix.';return;}
 try{
  facts.textContent='Loading original order matrix…';
  const data=await cutFetch(`${cutAPI}?intake_id=${encodeURIComponent(id)}`);
  if(cutEl('cut-order').value!==id)return;
  const skus=(data.skus||[]).map(s=>`${s.color_code}/${s.size_code}: ${s.ordered_qty}`).join(' · ');
  facts.textContent=`FABRIC BOM: ${(data.fabric_skus||[]).join(', ')||'(none)'}\nORDER SKU MATRIX: ${skus||'(none)'}`;
 }catch(e){facts.textContent=`Source lookup failed: ${e.message}`;}
});
cutEl('cut-form').elements.namedItem('cut_no').addEventListener('change',cutUpdateExpectedRevision);
