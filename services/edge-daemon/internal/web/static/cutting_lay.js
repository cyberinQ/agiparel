// Author: Sunil Jain, 2026-10-28
// Chapter 05: source-bound draft revisions, capped-register transparency, and safe operator retries.
'use strict';
const cutAPI = '/api/v1/cutting/lay-drafts';
let cutDrafts = [];
let cutSelected = null;
let cutViewGeneration = 0;
let cutRefreshGeneration = 0;
let cutPendingKey = '';
let cutPendingSignature = '';
const cutEl = id => document.getElementById(id);
const cutMsg = message => { cutEl('cut-message').textContent = message; };
function cutNode(tag, text, cls) {
 const node = document.createElement(tag);
 if (text !== undefined) node.textContent = String(text);
 if (cls) node.className = cls;
 return node;
}
async function cutFetch(url, options) {
 const response = await fetch(url, {cache:'no-store', ...options});
 const body = await response.text();
 let data;
 try { data = JSON.parse(body); } catch { throw new Error(body.slice(0,240) || `HTTP ${response.status}`); }
 if (!response.ok) throw new Error(data.error || `HTTP ${response.status}`);
 return data;
}
function cutCsv(value, columns, label) {
 const lines = value.split(/\r?\n/).map(v => v.trim()).filter(Boolean);
 if (!lines.length) throw new Error(`${label}: add at least one row`);
 return lines.map((line,i) => {
  const cells = line.split(',').map(v => v.trim());
  if (cells.length !== columns || cells.some(c => !c)) throw new Error(`${label} row ${i+1}: expected ${columns} nonempty cells`);
  return cells;
 });
}
function cutRandomKey() {
 const bytes = new Uint8Array(16);
 crypto.getRandomValues(bytes);
 return 'cut_' + Array.from(bytes,b => b.toString(16).padStart(2,'0')).join('');
}
function cutReadInput(form) {
 const d = new FormData(form);
 const positions = cutCsv(String(d.get('positions') || ''),3,'Positions')
  .map(([position,color_code,size_code]) => ({position:Number(position),color_code,size_code}));
 const parts = cutCsv(String(d.get('parts') || ''),2,'Parts')
  .map(([part_code,pieces_per_garment]) => ({part_code,pieces_per_garment:Number(pieces_per_garment)}));
 for (const p of positions) if (!Number.isInteger(p.position)) throw new Error('Positions must use whole-number indices');
 for (const p of parts) if (!Number.isInteger(p.pieces_per_garment)) throw new Error('Each part needs a whole-number piece count');
 const input = {intake_id:d.get('intake_id'),cut_no:Number(d.get('cut_no')),expected_revision:Number(d.get('expected_revision')),
  marker_code:String(d.get('marker_code') || '').trim(),fabric_sku:String(d.get('fabric_sku') || '').trim(),
  shade_band:String(d.get('shade_band') || '').trim(),marker_length_mm:Number(d.get('marker_length_mm')),
  usable_width_mm:Number(d.get('usable_width_mm')),reported_plies:Number(d.get('reported_plies')),
  bundle_ply_group_size:Number(d.get('bundle_ply_group_size')),positions,parts,note:String(d.get('note') || '').trim()};
 if (!input.intake_id) throw new Error('Select a buyer order before saving');
 for (const name of ['cut_no','expected_revision','marker_length_mm','usable_width_mm','reported_plies','bundle_ply_group_size']) {
  if (!Number.isSafeInteger(input[name])) throw new Error(`${name}: a whole number is required`);
 }
 const signature = JSON.stringify(input);
 if (signature !== cutPendingSignature) {
  cutPendingSignature = signature;
  cutPendingKey = cutRandomKey();
 }
 return {...input, request_key:cutPendingKey};
}
function cutUpdatePlannedConsumption() {
 const f = cutEl('cut-form').elements;
 const length = Number(f.namedItem('marker_length_mm').value);
 const plies = Number(f.namedItem('reported_plies').value);
 const output = cutEl('cut-planned-consumption');
 if (!Number.isSafeInteger(length) || !Number.isSafeInteger(plies) || length <= 0 || plies <= 0) {
  output.textContent = 'Planned minimum fabric: enter marker length and plies. Not measured consumption or available stock.';
  return;
 }
 const mm = length * plies;
 output.textContent = `Theoretical marker length × plies: ${(mm/1000).toLocaleString(undefined,{maximumFractionDigits:3})} m (${mm.toLocaleString()} mm), excluding spread ends, defects and waste. NOT measured or issued fabric.`;
}
function cutClearSelected(message) {
 cutViewGeneration++;
 cutSelected = null;
 cutEl('revise-cut').disabled = true;
 cutEl('print-cut').disabled = true;
 cutEl('cut-title').textContent = 'Choose or save a lay to inspect its labels.';
 cutEl('cut-summary').replaceChildren();
 const row = cutNode('tr'); const td = cutNode('td','No lay selected.');td.colSpan = 8;row.append(td);
 cutEl('cut-bundles').replaceChildren(row);
 document.dispatchEvent(new CustomEvent('cut-draft-cleared'));
 if (message) cutMsg(message);
}
function cutRenderDraft(report) {
 cutSelected = report;
 cutEl('print-cut').disabled = false;
 // Preparing a revision is only safe from a complete, currently latest snapshot.
 const latest = cutDrafts.find(d => d.intake_id === report.intake_id && d.cut_no === report.cut_no);
 cutEl('revise-cut').disabled = !latest || latest.revision !== report.revision || !(report.bundles||[]).length;
 cutEl('cut-title').textContent = `${report.po_number} · Cut ${report.cut_no} · Rev ${report.revision} · ${report.marker_code} · ${report.state}`;
 const summary = cutEl('cut-summary');summary.replaceChildren();
 const theoreticalMM = report.marker_length_mm * report.reported_plies;
 for (const [label,value] of [
  ['Planned garments',report.planned_garments],['Reported plies',report.reported_plies],
  ['Max plies / sub-bundle',report.bundle_ply_group_size],['Draft bundles',report.bundle_count],
  ['Part pieces',report.planned_parts],['Theoretical minimum mm',theoreticalMM],
  ['Signed issue entries',report.verified_issue_count],['Signed issued mm (not available balance)',report.verified_issued_length_mm],
  ['RFID','NOT BOUND']]) summary.append(cutNode('span',`${label}: ${value}`));
 const body = cutEl('cut-bundles');body.replaceChildren();
 for (const b of report.bundles||[]) {
  const tr = document.createElement('tr');
  for (const value of [b.label_code,b.position,`${b.color_code} / ${b.size_code}`,b.part_code,b.ply_group,`${b.ply_start}–${b.ply_end}`,b.piece_count,b.rfid_state]) tr.append(cutNode('td',value));
  body.append(tr);
 }
 if (!(report.bundles||[]).length) {const tr=cutNode('tr');const td=cutNode('td','No bundles found.');td.colSpan=8;tr.append(td);body.append(tr);}
 cutMsg(report.warning || 'Draft only. No physical cutting release.');
 document.dispatchEvent(new CustomEvent('cut-draft-selected',{detail:{draft:report}}));
}
async function cutOpen(id) {
 const generation = ++cutViewGeneration;
 try {
  const data = await cutFetch(`${cutAPI}?draft_id=${encodeURIComponent(id)}`);
  if (generation !== cutViewGeneration) return;
  if (!data.drafts.length) throw new Error('Draft no longer found');
  cutRenderDraft(data.drafts[0]);
 } catch (e) {if (generation === cutViewGeneration) cutMsg(`Could not load draft: ${e.message}`);}
}
function cutRenderList() {
 const list = cutEl('cut-register');list.replaceChildren();
 if (!cutDrafts.length) {list.append(cutNode('p','No cutting drafts yet. Create a lay from a buyer order.','hint'));return;}
 for (const d of cutDrafts) {
  const btn = cutNode('button',undefined,'cut-entry');btn.type='button';
  btn.append(cutNode('strong',`${d.po_number} · Cut ${d.cut_no} / Rev ${d.revision}`));
  btn.append(cutNode('small',`${d.marker_code} · ${d.reported_plies} reported plies · ${d.planned_garments} planned units`));
  btn.append(cutNode('span',d.state,'pill'));
  btn.addEventListener('click',()=>cutOpen(d.draft_id));list.append(btn);
 }
}
async function cutRefresh() {
 const generation = ++cutRefreshGeneration;
 try {
  const data = await cutFetch(cutAPI);
  if (generation !== cutRefreshGeneration) return;
  cutDrafts = data.drafts || [];
  const selector = cutEl('cut-order');const previous = selector.value;
  selector.replaceChildren();selector.append(new Option('Select buyer PO',''));
  for (const o of data.orders||[]) if (o.status === 'PROPOSED') selector.append(new Option(`${o.po_number} · ${o.style_code} · ${o.ordered_qty} pcs`,o.intake_id));
  selector.value = Array.from(selector.options).some(o=>o.value===previous) ? previous : '';
  cutEl('metric-lays').textContent = `${cutDrafts.length}${data.has_more ? '+' : ''}`;
  cutEl('metric-garments').textContent = cutDrafts.reduce((n,d)=>n+d.planned_garments,0).toLocaleString();
  cutEl('metric-bundles').textContent = cutDrafts.reduce((n,d)=>n+d.bundle_count,0).toLocaleString();
  cutEl('cut-register-scope').textContent = data.has_more
   ? `Showing the 60 most recent latest revisions. Older plans exist. Totals include only shown plans, NOT verified output.`
   : `Showing ${cutDrafts.length} latest-revision drafts. Totals are plans, not actual cuts or verified output.`;
  cutRenderList();
  if (!(data.orders||[]).length) cutMsg('No staged buyer orders yet. Open Order Intake / Buyer Review before planning a cut.');
  else cutMsg('Cutting register loaded. Physical release remains blocked.');
 } catch (e) {if (generation === cutRefreshGeneration) cutMsg(`Cutting database unavailable: ${e.message}`);}
}
function cutUpdateExpectedRevision() {
 const order = cutEl('cut-order').value;
 const cutNo = Number(cutEl('cut-form').elements.namedItem('cut_no').value);
 const existing = cutDrafts.find(d=>d.intake_id===order && d.cut_no===cutNo);
 cutEl('cut-form').elements.namedItem('expected_revision').value = existing ? existing.revision : 0;
}
async function cutSourceFacts(id) {
 const facts = cutEl('cut-source-facts');
 if (!id) {facts.textContent='Select a buyer order to inspect its SKU matrix.';return;}
 try {
  facts.textContent = 'Loading buyer order matrix…';
  const data = await cutFetch(`${cutAPI}?intake_id=${encodeURIComponent(id)}`);
  if (cutEl('cut-order').value !== id) return;
  const skus = (data.skus||[]).map(s=>`${s.color_code}/${s.size_code}: ${s.ordered_qty} ordered / ${s.planned_qty ?? "?"} draft-planned / ${s.remaining_qty ?? "?"} unplanned`).join(' · ');
  facts.textContent = `FABRIC BOM: ${(data.fabric_skus||[]).join(', ') || '(none)'}\nORDER SKU MATRIX: ${skus || '(none)'}`;
 } catch(e) {if (cutEl('cut-order').value===id) facts.textContent = `Source lookup failed: ${e.message}`;}
}
function cutPrepareRevision() {
 const report=cutSelected;
 if (!report || !report.bundles?.length) return;
 const latest=cutDrafts.find(d=>d.intake_id===report.intake_id && d.cut_no===report.cut_no);
 if (!latest || latest.revision!==report.revision) {cutMsg('This is an older revision. Refresh the register and open the latest revision.');return;}
 const order = cutEl('cut-order');
 if (!Array.from(order.options).some(o=>o.value===report.intake_id)) {
  cutMsg('Buyer order is outside the available order list. Refresh before revising.');return;
 }
 const form=cutEl('cut-form');
 const positionMap=new Map(),partMap=new Map();
 for (const b of report.bundles) {
  if (!positionMap.has(b.position)) positionMap.set(b.position,`${b.position},${b.color_code},${b.size_code}`);
  if (!partMap.has(b.part_code)) partMap.set(b.part_code,`${b.part_code},${b.pieces_per_garment}`);
 }
 if (positionMap.size!==report.position_count) {cutMsg('Bundle snapshot is incomplete; cannot prepare a revision.');return;}
 order.value=report.intake_id;
 cutClearSelected();
 cutSourceFacts(report.intake_id);
 const set=(name,val)=>{form.elements.namedItem(name).value=String(val);};
 set('cut_no',report.cut_no);set('expected_revision',report.revision);
 for (const name of ['marker_code','fabric_sku','shade_band','marker_length_mm','usable_width_mm','reported_plies','bundle_ply_group_size','note']) set(name,report[name]);
 set('positions',[...positionMap.entries()].sort((a,b)=>a[0]-b[0]).map(x=>x[1]).join('\n'));
 set('parts',[...partMap.entries()].sort((a,b)=>a[0].localeCompare(b[0])).map(x=>x[1]).join('\n'));
 cutPendingSignature='';cutPendingKey='';cutUpdatePlannedConsumption();
 cutMsg(`Prepared cut ${report.cut_no}, revision ${report.revision+1} from its latest saved snapshot. Review and change inputs before saving. Any spread evidence will freeze revisions on the server.`);
}
cutEl('cut-form').addEventListener('submit',async event=>{
 event.preventDefault();const button=cutEl('save-cut');
 try {
  button.disabled=true;
  const input=cutReadInput(event.currentTarget);
  const selectedOrderAtSubmit=cutEl('cut-order').value;
  const data=await cutFetch(cutAPI,{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(input)});
  cutPendingSignature='';cutPendingKey='';
  await cutRefresh();
  if (cutEl('cut-order').value === selectedOrderAtSubmit) {
   cutRenderDraft(data.draft);
   cutEl('cut-form').elements.namedItem('expected_revision').value = data.draft.revision;
  } else cutMsg('Draft saved for the previously selected order. Refresh its register before continuing.');
 } catch(e) {cutMsg(`Draft not saved: ${e.message}. Retrying unchanged inputs preserves the request key.`);}
 finally {button.disabled=false;}
});
cutEl('refresh-cut').addEventListener('click',cutRefresh);
cutEl('revise-cut').addEventListener('click',cutPrepareRevision);
cutEl('print-cut').addEventListener('click',()=>{if(cutSelected)window.print();});
cutEl('cut-order').addEventListener('change',()=>{
 cutClearSelected('Buyer order changed. Select a saved lay again before using cutting evidence.');
 cutUpdateExpectedRevision();cutSourceFacts(cutEl('cut-order').value);
});
cutEl('cut-form').elements.namedItem('cut_no').addEventListener('change',()=>{
 cutClearSelected('Cut number changed. Select a saved lay to inspect evidence.');cutUpdateExpectedRevision();
});
for (const name of ['marker_length_mm','reported_plies']) cutEl('cut-form').elements.namedItem(name).addEventListener('input',cutUpdatePlannedConsumption);
cutUpdatePlannedConsumption();
cutRefresh();
