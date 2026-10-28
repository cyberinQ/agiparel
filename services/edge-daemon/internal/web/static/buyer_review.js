// Author: Sunil Jain, 2026-10-28 — Source-locked, multi-PO-safe buyer review; no release authority.
(()=>{'use strict';
const $=id=>document.getElementById(id),elt=(tag,cls,txt,parent)=>{const node=document.createElement(tag);if(cls)node.className=cls;if(txt!==undefined)node.textContent=String(txt);if(parent)parent.append(node);return node};
const api=async(url,opts={})=>{const r=await fetch(url,{credentials:'same-origin',...opts});if(!r.ok)throw Error((await r.text()).trim().slice(0,350)||`HTTP ${r.status}`);return r.json()};
let sources=[],preview=null,poOptions=[],materials=[],techHints=[];
let analyzedSources=null, analysisEpoch=0, selectedOption=-1;
const optionEdits=new Map();
const report=(id,msg,fail=false)=>{const n=$(id);n.textContent=msg;n.style.color=fail?'#a22b31':'#647a94'};
function opt(select,value,label){const x=elt('option','',label,select);x.value=value}
async function loadSources(){try{const out=await api('/api/v1/documents');sources=(out.items||[]).filter(x=>['EXTRACTED','PARTIAL'].includes(x.status));for(const item of sources){const label=item.filename;opt($('po-source'),item.id,label);opt($('bom-source'),item.id,label)}if(sources.length===1){$('po-source').value=sources[0].id;$('bom-source').value=sources[0].id;}report('load-message',sources.length?`${sources.length} file(s) ready. Choose your order files.`:'No files ready yet. Upload a PO or tech pack, then return here.');}catch(err){report('load-message',err.message,true)}}
const input=(value,cls,placeholder='')=>{const n=elt('input',cls);n.value=value??'';n.placeholder=placeholder;return n};
function cell(row,node,label){const td=elt('td','',undefined,row);if(label)td.dataset.label=label;td.append(node);return td}
function makeRow(type,item={}){const target=$(type==='sku'?'sku-rows':'bom-rows'),r=elt('tr','',undefined,target);r.dataset.kind=type;
 function field(key,value,placeholder){const n=input(value,'',placeholder);n.dataset.field=key;cell(r,n,({color:'Color',size:'Size',quantity:'Pieces',upc:'UPC',material_sku:'Material code',per_garment:'Per garment',uom:'Unit'})[key]);return n}
 if(type==='sku'){field('color',item.color,'Color');field('size',item.size,'Size');field('quantity',item.quantity??'','Pieces');field('upc',item.upc??'','Optional UPC')}
 else{r.dataset.styleCode=item.style_code||'';field('material_sku',item.material_sku,'Material code');const sel=elt('select');sel.dataset.field='category';for(const cat of ['FABRIC','TRIM'])opt(sel,cat,cat);sel.value=item.category||'FABRIC';cell(r,sel,'Type');field('per_garment',item.per_garment,'1.850');field('uom',item.uom,'m / pcs')}
 const provenance=elt('div');const ord=input(item.source_ordinal??0,'ord','0');ord.type='number';ord.min='0';ord.max='5000';ord.dataset.field='source_ordinal';provenance.append(ord);const src=elt('a','','Open evidence ↗',provenance);src.target='_blank';src.rel='noopener';src.href='#';ord.addEventListener('input',()=>setEvidenceLink());function setEvidenceLink(){const id=type==='sku'?$('po-source').value:$('bom-source').value;const n=Number(ord.value)||0;src.href=n?`/documents?document=${encodeURIComponent(id)}&segment=${n}`:`/documents?document=${encodeURIComponent(id)}`;src.textContent=n?`#${n} ↗`:'Manual';}setEvidenceLink();cell(r,provenance,'Source');
 const remove=elt('button','remove','×');remove.type='button';remove.title='Remove this line';remove.setAttribute('aria-label',`Remove ${type==='sku'?'size':'material'} row`);remove.addEventListener('click',()=>{r.remove();renderOrderSummary()});cell(r,remove,'Remove');if(type==='material'){const code=r.querySelector('[data-field="material_sku"]');if(code.value.trim()&&!validMaterialSKU(code.value.trim()))offerMaterialCode(r,code)}return r;
}
// Preserve independent review work for every detected PO, including corrected BOM rows.
// No browser persistence is used: unapproved buyer facts must not leak to local storage.
function captureOption(){
 if(selectedOption<0)return;
 optionEdits.set(selectedOption,{
  po_number:$('po-number').value,style_code:$('style-code').value,
  skus:readRows('sku-rows'),materials:readRows('bom-rows'),
  review_note:$('review-note').value,acknowledged:$('acknowledged').checked
 });
}
function setOptions(){
 const sel=$('po-choice');sel.replaceChildren();optionEdits.clear();selectedOption=-1;
 for(let i=0;i<poOptions.length;i++)opt(sel,String(i),`${poOptions[i].po_number} · ${poOptions[i].style_code} (${poOptions[i].skus.length} lines)`);
 $('multiple-po').hidden=poOptions.length<2;
 if(poOptions.length)applyOption(0);
 else{
  $('po-number').value='';$('style-code').value='';$('sku-rows').replaceChildren();makeRow('sku');
  $('bom-rows').replaceChildren();for(const m of materials.filter(m=>!m.style_code))makeRow('material',m);
  if(!$('bom-rows').children.length)makeRow('material');
  $('review-note').value='';$('acknowledged').checked=false;
 }
 renderOrderSummary();
}
function applyOption(i){
 if(!Number.isInteger(i)||i<0||i>=poOptions.length)return;
 const item=poOptions[i], saved=optionEdits.get(i);
 selectedOption=i;
 $('po-number').value=saved?.po_number??item.po_number;
 $('style-code').value=saved?.style_code??item.style_code;
 $('sku-rows').replaceChildren();for(const row of (saved?.skus??item.skus))makeRow('sku',row);
 $('bom-rows').replaceChildren();
 const rows=saved?.materials??materials.filter(m=>!m.style_code||m.style_code===item.style_code);
 for(const row of rows)makeRow('material',row);
 if(!rows.length)makeRow('material');
 $('review-note').value=saved?.review_note??'';
 $('acknowledged').checked=saved?.acknowledged??false;
 $('result').hidden=true;
 renderOrderSummary();
}
// A request can complete after the operator selects a different source. Never
// attach previously reviewed values to a newly selected document ID.
function invalidateReview(){
 analysisEpoch++;preview=null;analyzedSources=null;poOptions=[];materials=[];techHints=[];
 selectedOption=-1;optionEdits.clear();$('review').hidden=true;$('result').hidden=true;
 $('analyze').disabled=false;
 report('load-message','Sources changed. Read order details again before reviewing or saving.');
}
function reviewSourcesCurrent(){
 return Boolean(preview&&analyzedSources&&
  $('po-source').value===analyzedSources.po&&$('bom-source').value===analyzedSources.bom);
}
// Compact, non-authoritative quantity check in the existing panel. Monetary
// margins, wastage, purchasing and production are deliberately NOT inferred.
function renderOrderSummary(){
 const skus=readRows('sku-rows'),bom=readRows('bom-rows');
 let total=0n, valid=true;
 for(const row of skus){
  const value=String(row.quantity);
  if(!Number.isSafeInteger(row.quantity)||row.quantity<=0||row.quantity>100000000){valid=false;break}
  total+=BigInt(value);
 }
 if(total>100000000n)valid=false;
 const manual=skus.filter(r=>r.source_ordinal==null||r.source_ordinal===0).length+bom.filter(r=>r.source_ordinal==null||r.source_ordinal===0).length;
 const needsRate=bom.filter(r=>!/^(?:0|[1-9]\d*)(?:\.\d{1,3})?$/.test(String(r.per_garment))||Number(r.per_garment)<=0||Number(r.per_garment)>1000000||!r.uom).length;
 const styleMismatches=bom.filter(r=>r.style_code&&r.style_code!==$('style-code').value).length;
 const summary=$('order-summary');
 summary.textContent=`${skus.length} size line(s) · ${valid?total.toLocaleString('en-US'):'Check piece quantities'} pieces · ${bom.length} material line(s) · ${manual} manual-source line(s)`;
 const flags=[];
 if(needsRate)flags.push(`${needsRate} material usage/unit value(s) to verify`);
 if(styleMismatches)flags.push(`${styleMismatches} material line(s) belong to another style`);
 if(!bom.some(m=>m.category==='FABRIC'))flags.push('No fabric requirement entered');
 $('order-checks').textContent=flags.length?`Before saving: ${flags.join(' · ')}.`:'Draft quantities still require buyer confirmation, revision and delivery checks before any commitment.';
}
function renderTechPackHints(){
 const box=$('techpack-workbench'), list=$('techpack-candidates');list.replaceChildren();
 const coverage=preview?.techpack_bom_coverage||{};
 box.hidden=!techHints.length && !coverage.bom_pages;
 if(box.hidden)return;
 const total=Number(coverage.material_rows||techHints.length);
 $('techpack-summary').textContent=`Recognized ${total} candidate component(s) across ${coverage.bom_pages||0} BOM page(s). 0 verified per-garment rates were imported from these PDF suggestions. Every candidate requires human review.`;
 for(const hint of techHints){
  const item=elt('article','tech-candidate',undefined,list);
  const heading=elt('div','tech-candidate-heading',undefined,item);
  elt('strong','',hint.component,heading);
  elt('span','tech-state',hint.quantity_state==='ZERO_UNVERIFIED'?'Zero is not usage':'Rate unverified',heading);
  const parts=elt('div','tech-parts',undefined,item);
  elt('span','',`${hint.category} · ${hint.section} · page ${hint.page||'?'} · segment #${hint.source_ordinal}`,parts);
  elt('p','tech-material',hint.material_label,item);
  elt('p','tech-raw',`Document Quantity: ${hint.raw_quantity||'not shown'} · Suggested code: ${hint.suggested_code||'needs identification'}`,item);
  elt('p','tech-explain',hint.hint,item);
  const controls=elt('div','tech-actions',undefined,item);
  const link=elt('a','quiet','Inspect source ↗',controls);
  link.href=`/documents?document=${encodeURIComponent($('bom-source').value)}&segment=${hint.source_ordinal}`;
  link.target='_blank';link.rel='noopener';
  const use=elt('button','secondary','Review as BOM line +',controls);use.type='button';
  use.addEventListener('click',()=>{
    const tr=makeRow('material',{material_sku:hint.suggested_code,category:hint.category,per_garment:'',uom:'',source_ordinal:hint.source_ordinal});
    use.disabled=true;use.textContent='Added for review';renderOrderSummary();
    tr.querySelector('[data-field="per_garment"]').focus();
    tr.scrollIntoView({behavior:'smooth',block:'center'});
    report('stage-message','BOM suggestion added for REVIEW. Enter a verified per-garment usage rate and unit; remove inapplicable materials. Nothing is approved.');
  });
 }
}
function sourceExcerpts(src,target,title){$(target).replaceChildren();$(title).textContent=src.filename;const segments=src.segments||[];for(const s of segments.slice(0,35)){const row=elt('div','source-row',undefined,$(target)),lnk=elt('a','',`#${s.ordinal}`,row);lnk.href=`/documents?document=${encodeURIComponent(src.id)}&segment=${s.ordinal}`;lnk.target='_blank';lnk.rel='noopener';elt('span','',` · ${s.kind} · `,row);const pre=elt('pre','',s.content.slice(0,360)+(s.content.length>360?'…':''),row)}if(segments.length>35)elt('p','',`Showing 35 of ${segments.length} snippets. Open the document to search the rest.`,$(target))}
async function analyze(){
 const po=$('po-source').value,bom=$('bom-source').value;
 if(!po||!bom){report('load-message','Choose both files. If one file contains both, select it twice.',true);return}
 const epoch=++analysisEpoch;
 // Make the prior source review unavailable while a new analysis is requested.
 preview=null;analyzedSources=null;$('review').hidden=true;$('result').hidden=true;
 report('load-message','Reading source evidence…');$('analyze').disabled=true;
 try{
  const out=await api(`/api/v1/buyer-review?po_document_id=${encodeURIComponent(po)}&bom_document_id=${encodeURIComponent(bom)}`);
  if(epoch!==analysisEpoch||$('po-source').value!==po||$('bom-source').value!==bom)return;
  preview=out;analyzedSources={po,bom};poOptions=out.po_options||[];
  materials=out.materials||[];techHints=out.techpack_material_hints||[];
  setOptions();renderTechPackHints();
  sourceExcerpts(out.po_source,'po-evidence','po-evidence-title');
  sourceExcerpts(out.bom_source,'bom-evidence','bom-evidence-title');
  $('review').hidden=false;
  const detected=poOptions.reduce((n,o)=>n+o.skus.length,0);
  $('coverage').textContent=`Found ${detected} size line(s) and ${materials.length} material line(s). Check every value before saving.${techHints.length?` ${techHints.length} extra material suggestion(s) need checking.`:''}`;
  $('issues').hidden=!(out.issues||[]).length;
  if((out.issues||[]).length)$('issues').textContent=(out.issues||[]).join(' · ');
  report('load-message','Order details are ready. Check them below.');
  $('review').scrollIntoView({behavior:'smooth',block:'start'});
 }catch(err){if(epoch===analysisEpoch)report('load-message',err.message,true)}
 finally{if(epoch===analysisEpoch)$('analyze').disabled=false}
}
function readRows(id){return Array.from($(id).querySelectorAll('tr')).map(tr=>{const d={};for(const node of tr.querySelectorAll('[data-field]')){let value=node.value;if(typeof value==='string')value=value.trim();if(node.dataset.field==='quantity'||node.dataset.field==='source_ordinal')value=value===''?null:Number(value);d[node.dataset.field]=value}if(tr.dataset.kind==='material'&&tr.dataset.styleCode)d.style_code=tr.dataset.styleCode;return d})}
// A material name is not automatically a SKU. Suggest a safe code, then require a tap to confirm it.
// Server-side SKU validation remains authoritative; never silently rewrite source facts.
const validMaterialSKU=value=>typeof value==='string' && value.length>=1 && value.length<=128 && /^[A-Za-z0-9][A-Za-z0-9_./-]*$/.test(value);
function suggestMaterialCode(raw){
 const simplified=String(raw||'').normalize('NFKD').replace(/[\u0300-\u036f]/g,'').replace(/&/g,' AND ').replace(/\+/g,' PLUS ');
 const proposed=simplified.toUpperCase().replace(/[^A-Z0-9_./-]+/g,'-').replace(/^[^A-Z0-9]+/,'').replace(/[-./_]+$/,'').slice(0,128).replace(/[-./_]+$/,'');
 return validMaterialSKU(proposed)?proposed:'';
}
function clearMaterialHint(tr){tr.querySelector('.material-code-help')?.remove()}
function offerMaterialCode(tr,field){
 clearMaterialHint(tr);
 const raw=field.value.trim(),proposed=suggestMaterialCode(raw);
 if(!raw)return;
 const help=elt('div','material-code-help',undefined,field.parentElement);
 elt('span','','This is a material name. Confirm a short code for the draft.',help);
 if(proposed && proposed!==raw){
  const confirm=elt('button','secondary material-code-confirm',`Use ${proposed}`,help);
  confirm.type='button';
  confirm.addEventListener('click',()=>{field.value=proposed;field.removeAttribute('aria-invalid');clearMaterialHint(tr);report('stage-message','Material code confirmed. You can save the draft.');field.focus();renderOrderSummary()});
 }
}
function showMaterialError(index){
 const rows=$('bom-rows').querySelectorAll('tr');
 const tr=rows[index], field=tr?.querySelector('[data-field="material_sku"]');
 if(!field){report('stage-message',`Material ${index+1} needs a code.`,true);return false}
 const msg=`Material ${index+1}: ${field.value.trim()?'Confirm a short material code before saving.':'Enter the material name or code.'}`;
 report('stage-message',msg,true);
 offerMaterialCode(tr,field);
 field.setAttribute('aria-invalid','true');field.focus();field.scrollIntoView({block:'nearest'});
 return false;
}
function checkMaterialRows(){
 const rows=$('bom-rows').querySelectorAll('tr');
 for(let i=0;i<rows.length;i++){
  const field=rows[i].querySelector('[data-field="material_sku"]');
  if(field && !validMaterialSKU(field.value.trim()))return showMaterialError(i);
 }
 return true;
}
$('bom-rows').addEventListener('input',e=>{
 if(e.target?.dataset?.field==='material_sku'){e.target.removeAttribute('aria-invalid');clearMaterialHint(e.target.closest('tr'));}
});
$('bom-rows').addEventListener('change',e=>{
 if(e.target?.dataset?.field==='material_sku' && e.target.value.trim() && !validMaterialSKU(e.target.value.trim()))offerMaterialCode(e.target.closest('tr'),e.target);
});
function checkReviewedQuantities(req){
 const seenSizes=new Set(), seenMaterials=new Set();
 let total=0n;
 for(let i=0;i<req.skus.length;i++){
  const row=req.skus[i];
  if(!Number.isSafeInteger(row.quantity)||row.quantity<=0||row.quantity>100000000){
   report('stage-message',`Size line ${i+1}: pieces must be a whole number between 1 and 100,000,000.`,true);return false;
  }
  total+=BigInt(row.quantity);
  const key=`${row.color}\u0000${row.size}`;
  if(seenSizes.has(key)){report('stage-message',`Size line ${i+1}: duplicate color/size. Correct or remove the repeated line.`,true);return false}
  seenSizes.add(key);
 }
 if(total>100000000n){report('stage-message','Total order quantity exceeds 100,000,000 pieces.',true);return false}
 for(let i=0;i<req.materials.length;i++){
  const row=req.materials[i];
  if(row.style_code&&row.style_code!==req.style_code){
   report('stage-message',`Material ${i+1} comes from a different style. Recheck the source before saving.`,true);return false;
  }
  const rate=String(row.per_garment||'');
  if(!/^(?:0|[1-9]\d*)(?:\.\d{1,3})?$/.test(rate)||Number(rate)<0.001||Number(rate)>1000000||!row.uom){
   report('stage-message',`Material ${i+1}: verify positive per-garment usage (maximum 3 decimals) and unit.`,true);return false;
  }
  const key=`${row.material_sku}\u0000${row.category}\u0000${row.uom.toUpperCase()}`;
  if(seenMaterials.has(key)){report('stage-message',`Material ${i+1}: duplicate material/type/unit. Review the BOM.`,true);return false}
  seenMaterials.add(key);
 }
 return true;
}
async function stage(){if(!reviewSourcesCurrent()){report('stage-message','Source files changed. Read the order details again before saving.',true);return}const req={po_document_id:$('po-source').value,bom_document_id:$('bom-source').value,po_number:$('po-number').value.trim(),style_code:$('style-code').value.trim(),skus:readRows('sku-rows'),materials:readRows('bom-rows'),review_note:$('review-note').value.trim(),acknowledged:$('acknowledged').checked};
 if(!req.acknowledged){report('stage-message','Please check the confirmation box before saving.',true);return} if(!checkMaterialRows()||!checkReviewedQuantities(req))return; if(req.review_note.length<8){report('stage-message','Add a short note on what you checked (at least 8 characters).',true);$('review-note').focus();return} $('stage').disabled=true;report('stage-message','Checking and saving your draft…');try{const out=await api('/api/v1/buyer-review',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(req)});if(!reviewSourcesCurrent()||$('po-number').value.trim()!==req.po_number||$('style-code').value.trim()!==req.style_code){$('result').hidden=true;report('load-message',`Draft for ${out.intake.po_number} was saved, but the current review changed. Recheck the current order before saving.`,true);return}$('result').hidden=false;$('result-title').textContent=`${out.intake.po_number} · ${out.intake.ordered_qty.toLocaleString()} pieces`;$('result-message').textContent=`Draft ${out.reused?'already saved':'saved'} for ${out.intake.po_number}. Review material needs next. Buying, stock and production remain on hold.`;report('stage-message','Staged successfully. Review the result below.');$('result').scrollIntoView({behavior:'smooth',block:'start'});}catch(err){const match=/^BOM row (\d+): material SKU:/.exec(err.message);if(match)showMaterialError(Number(match[1])-2);else report('stage-message',err.message,true)}finally{$('stage').disabled=false}}
$('analyze').addEventListener('click',()=>void analyze());
for(const id of ['po-source','bom-source'])$(id).addEventListener('change',invalidateReview);
$('po-choice').addEventListener('change',e=>{captureOption();applyOption(Number(e.target.value));renderTechPackHints()});
// A style correction must never quietly discard corrected BOM rows.
$('style-code').addEventListener('change',()=>{renderOrderSummary();renderTechPackHints()});
$('add-sku').addEventListener('click',()=>{makeRow('sku');renderOrderSummary()});
$('add-material').addEventListener('click',()=>{makeRow('material');renderOrderSummary()});
$('review').addEventListener('input',e=>{if(e.target?.dataset?.field)renderOrderSummary()});
$('review').addEventListener('change',e=>{if(e.target?.dataset?.field)renderOrderSummary()});
$('stage').addEventListener('click',()=>void stage());void loadSources();
})();
