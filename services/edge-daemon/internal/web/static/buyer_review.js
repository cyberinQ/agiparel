// Author: Sunil Jain, 2026-10-27 — Guided buyer-file review and draft staging; no release authority.
(()=>{'use strict';
const $=id=>document.getElementById(id),elt=(tag,cls,txt,parent)=>{const node=document.createElement(tag);if(cls)node.className=cls;if(txt!==undefined)node.textContent=String(txt);if(parent)parent.append(node);return node};
const api=async(url,opts={})=>{const r=await fetch(url,{credentials:'same-origin',...opts});if(!r.ok)throw Error((await r.text()).trim().slice(0,350)||`HTTP ${r.status}`);return r.json()};
let sources=[],preview=null,poOptions=[],materials=[],techHints=[];
const report=(id,msg,fail=false)=>{const n=$(id);n.textContent=msg;n.style.color=fail?'#a22b31':'#647a94'};
function opt(select,value,label){const x=elt('option','',label,select);x.value=value}
async function loadSources(){try{const out=await api('/api/v1/documents');sources=(out.items||[]).filter(x=>['EXTRACTED','PARTIAL'].includes(x.status));for(const item of sources){const label=item.filename;opt($('po-source'),item.id,label);opt($('bom-source'),item.id,label)}if(sources.length===1){$('po-source').value=sources[0].id;$('bom-source').value=sources[0].id;}report('load-message',sources.length?`${sources.length} file(s) ready. Choose your order files.`:'No files ready yet. Upload a PO or tech pack, then return here.');}catch(err){report('load-message',err.message,true)}}
const input=(value,cls,placeholder='')=>{const n=elt('input',cls);n.value=value??'';n.placeholder=placeholder;return n};
function cell(row,node,label){const td=elt('td','',undefined,row);if(label)td.dataset.label=label;td.append(node);return td}
function makeRow(type,item={}){const target=$(type==='sku'?'sku-rows':'bom-rows'),r=elt('tr','',undefined,target);r.dataset.kind=type;
 function field(key,value,placeholder){const n=input(value,'',placeholder);n.dataset.field=key;cell(r,n,({color:'Color',size:'Size',quantity:'Pieces',upc:'UPC',material_sku:'Material code',per_garment:'Per garment',uom:'Unit'})[key]);return n}
 if(type==='sku'){field('color',item.color,'Color');field('size',item.size,'Size');field('quantity',item.quantity??'','Pieces');field('upc',item.upc??'','Optional UPC')}
 else{field('material_sku',item.material_sku,'Material code');const sel=elt('select');sel.dataset.field='category';for(const cat of ['FABRIC','TRIM'])opt(sel,cat,cat);sel.value=item.category||'FABRIC';cell(r,sel,'Type');field('per_garment',item.per_garment,'1.850');field('uom',item.uom,'m / pcs')}
 const provenance=elt('div');const ord=input(item.source_ordinal??0,'ord','0');ord.type='number';ord.min='0';ord.max='5000';ord.dataset.field='source_ordinal';provenance.append(ord);const src=elt('a','','Open evidence ↗',provenance);src.target='_blank';src.rel='noopener';src.href='#';ord.addEventListener('input',()=>setEvidenceLink());function setEvidenceLink(){const id=type==='sku'?$('po-source').value:$('bom-source').value;const n=Number(ord.value)||0;src.href=n?`/documents?document=${encodeURIComponent(id)}&segment=${n}`:`/documents?document=${encodeURIComponent(id)}`;src.textContent=n?`#${n} ↗`:'Manual';}setEvidenceLink();cell(r,provenance,'Source');
 const remove=elt('button','remove','×');remove.type='button';remove.title='Remove this line';remove.addEventListener('click',()=>r.remove());cell(r,remove,'Remove');if(type==='material'){const code=r.querySelector('[data-field="material_sku"]');if(code.value.trim()&&!validMaterialSKU(code.value.trim()))offerMaterialCode(r,code)}return r;
}
function setOptions(){const sel=$('po-choice');sel.replaceChildren();for(let i=0;i<poOptions.length;i++)opt(sel,String(i),`${poOptions[i].po_number} · ${poOptions[i].style_code} (${poOptions[i].skus.length} lines)`);$('multiple-po').hidden=poOptions.length<2;if(poOptions.length)applyOption(0);else { $('po-number').value='';$('style-code').value='';$('sku-rows').replaceChildren();makeRow('sku') } }
function applyOption(i){const item=poOptions[i];$('po-number').value=item.po_number;$('style-code').value=item.style_code;$('sku-rows').replaceChildren();for(const s of item.skus)makeRow('sku',s);renderMaterials();}
function renderMaterials(){$('bom-rows').replaceChildren();const current=$('style-code').value;const filtered=materials.filter(m=>!m.style_code||m.style_code===current);for(const m of filtered)makeRow('material',m);if(!filtered.length)makeRow('material')}
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
    use.disabled=true;use.textContent='Added for review';
    tr.querySelector('[data-field="per_garment"]').focus();
    tr.scrollIntoView({behavior:'smooth',block:'center'});
    report('stage-message','BOM suggestion added for REVIEW. Enter a verified per-garment usage rate and unit; remove inapplicable materials. Nothing is approved.');
  });
 }
}
function sourceExcerpts(src,target,title){$(target).replaceChildren();$(title).textContent=src.filename;const segments=src.segments||[];for(const s of segments.slice(0,35)){const row=elt('div','source-row',undefined,$(target)),lnk=elt('a','',`#${s.ordinal}`,row);lnk.href=`/documents?document=${encodeURIComponent(src.id)}&segment=${s.ordinal}`;lnk.target='_blank';lnk.rel='noopener';elt('span','',` · ${s.kind} · `,row);const pre=elt('pre','',s.content.slice(0,360)+(s.content.length>360?'…':''),row)}if(segments.length>35)elt('p','',`Showing 35 of ${segments.length} snippets. Open the document to search the rest.`,$(target))}
async function analyze(){const po=$('po-source').value,bom=$('bom-source').value;if(!po||!bom){report('load-message','Choose both files. If one file contains both, select it twice.',true);return}report('load-message','Reading source evidence…');$('analyze').disabled=true;try{const out=await api(`/api/v1/buyer-review?po_document_id=${encodeURIComponent(po)}&bom_document_id=${encodeURIComponent(bom)}`);preview=out;poOptions=out.po_options||[];materials=out.materials||[];techHints=out.techpack_material_hints||[];setOptions();renderTechPackHints();sourceExcerpts(out.po_source,'po-evidence','po-evidence-title');sourceExcerpts(out.bom_source,'bom-evidence','bom-evidence-title');$('review').hidden=false;$('result').hidden=true;
 const detected=poOptions.reduce((n,o)=>n+o.skus.length,0);$('coverage').textContent=`Found ${detected} size line(s) and ${materials.length} material line(s). Check every value before saving.${techHints.length ? ` ${techHints.length} extra material suggestion(s) need checking.` : ''}`;
 $('issues').hidden=!(out.issues||[]).length;if((out.issues||[]).length)$('issues').textContent=(out.issues||[]).join(' · ');
 report('load-message','Order details are ready. Check them below.');$('review').scrollIntoView({behavior:'smooth',block:'start'});
 }catch(err){report('load-message',err.message,true)}finally{$('analyze').disabled=false}}
function readRows(id){return Array.from($(id).querySelectorAll('tr')).map(tr=>{const d={};for(const node of tr.querySelectorAll('[data-field]')){let value=node.value;if(typeof value==='string')value=value.trim();if(node.dataset.field==='quantity'||node.dataset.field==='source_ordinal')value=Number(value);d[node.dataset.field]=value}return d})}
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
  confirm.addEventListener('click',()=>{field.value=proposed;field.removeAttribute('aria-invalid');clearMaterialHint(tr);report('stage-message','Material code confirmed. You can save the draft.');field.focus()});
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
async function stage(){if(!preview)return;const req={po_document_id:$('po-source').value,bom_document_id:$('bom-source').value,po_number:$('po-number').value.trim(),style_code:$('style-code').value.trim(),skus:readRows('sku-rows'),materials:readRows('bom-rows'),review_note:$('review-note').value.trim(),acknowledged:$('acknowledged').checked};
 if(!req.acknowledged){report('stage-message','Please check the confirmation box before saving.',true);return} if(!checkMaterialRows())return; if(req.review_note.length<8){report('stage-message','Add a short note on what you checked (at least 8 characters).',true);$('review-note').focus();return} $('stage').disabled=true;report('stage-message','Checking and saving your draft…');try{const out=await api('/api/v1/buyer-review',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(req)});$('result').hidden=false;$('result-title').textContent=`${out.intake.po_number} · ${out.intake.ordered_qty.toLocaleString()} pieces`;$('result-message').textContent=`Draft ${out.reused?'already saved':'saved'} for ${out.intake.po_number}. Review material needs next. Buying, stock and production remain on hold.`;report('stage-message','Staged successfully. Review the result below.');$('result').scrollIntoView({behavior:'smooth',block:'start'});}catch(err){const match=/^BOM row (\d+): material SKU:/.exec(err.message);if(match)showMaterialError(Number(match[1])-2);else report('stage-message',err.message,true)}finally{$('stage').disabled=false}}
$('analyze').addEventListener('click',()=>void analyze());$('po-choice').addEventListener('change',e=>{applyOption(Number(e.target.value));renderTechPackHints()});$('style-code').addEventListener('change',()=>{renderMaterials();renderTechPackHints()});$('add-sku').addEventListener('click',()=>makeRow('sku'));$('add-material').addEventListener('click',()=>makeRow('material'));$('stage').addEventListener('click',()=>void stage());void loadSources();
})();
