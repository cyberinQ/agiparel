// Author: Sunil Jain, CyberinQ LLC — Source-locked, auto-downselecting PO->TechPack/OB buyer review with colleague conflict reconciliation.
(()=>{'use strict';
const $=id=>document.getElementById(id),elt=(tag,cls,txt,parent)=>{const node=document.createElement(tag);if(cls)node.className=cls;if(txt!==undefined)node.textContent=String(txt);if(parent)parent.append(node);return node};
const api=async(url,opts={})=>{
 const r=await fetch(url,{credentials:'same-origin',...opts});
 if(!r.ok){
  const text=await r.text();
  let parsed=null;
  try{parsed=JSON.parse(text)}catch{}
  if(parsed&&parsed.code==='COLLEAGUE_DRAFT_CONFLICT'){
   const err=Error(parsed.message||'Colleague draft conflict');
   err.conflict=parsed;
   throw err;
  }
  throw Error(text.trim().slice(0,350)||`HTTP ${r.status}`);
 }
 return r.json();
};
let sources=[],preview=null,poOptions=[],materials=[],techHints=[],drawings=[],existingDrafts=[];
let analyzedSources=null, analysisEpoch=0, selectedOption=-1, confirmedSupersedeDigest='';
const optionEdits=new Map();
const report=(id,msg,fail=false)=>{const n=$(id);n.textContent=msg;n.style.color=fail?'#a22b31':'#647a94'};
function opt(select,value,label){const x=elt('option','',label,select);x.value=value}

function extractFileStyleAndRole(name=''){
 const u=String(name).toUpperCase();
 const norm=u.replace(/[_\s().-]+/g,' ');
 let role='OTHER';
 if(/\b(01 MACHINE|MACHINE ROSTER|02 WORKFORCE|SKILLS MATRIX|03 OPERATION BULLETIN RATES|04 RAW MATERIAL)/.test(norm))role='PLANT_MASTER';
 else if(norm.includes('PO SHEET')||norm.includes('PURCHASE ORDER')||norm.includes('BUYER PO'))role='ANCHOR_PO';
 else if(` ${norm} `.includes(' OB ')||norm.includes('OPERATION BULLETIN')||(u.endsWith('.XLSX')&&(norm.includes('45PW')||norm.includes('PRINT')||norm.includes('SOLID'))))role='OPERATION_BULLETIN';
 else if(norm.includes('TECH PACK')||norm.includes('SAMPLE ENGLISH')||norm.includes('SPEC'))role='TECH_PACK';
 const m=/(?:^|\s)([A-Z]{2}[0-9]{2}[A-Z][0-9]{3}|[0-9]{2}[A-Z]{2}[0-9]{3}|[0-9]{6})(?=\s|$)/.exec(norm);
 return {styleKey:m?m[1]:'', role};
}

// Down-select #bom-source automatically whenever a PO file is chosen in #po-source
// so the user only sees Tech Packs and Bulletins belonging to that PO's style.
function syncScopedBOMOptions(selectedPOId, showAll=false){
 const bomSel=$('bom-source');
 const prevBOM=bomSel.value;
 bomSel.replaceChildren();
 opt(bomSel,'','Choose file…');
 const poItem=sources.find(x=>x.id===selectedPOId);
 const poInfo=poItem?extractFileStyleAndRole(poItem.filename):{styleKey:''};
 if(!poInfo.styleKey || showAll){
  for(const item of sources)opt(bomSel,item.id,item.filename);
  if(sources.some(x=>x.id===prevBOM))bomSel.value=prevBOM;
  return;
 }
 const matching=sources.filter(x=>{
  const info=extractFileStyleAndRole(x.filename);
  return info.styleKey===poInfo.styleKey;
 });
 const candidates=matching.length?matching:sources;
 for(const item of candidates){
  const info=extractFileStyleAndRole(item.filename);
  const tag=info.role==='TECH_PACK'?' [Tech Pack]':info.role==='OPERATION_BULLETIN'?' [Operation Bulletin]':info.role==='ANCHOR_PO'?' [PO Sheet]':'';
  opt(bomSel,item.id,item.filename+tag);
 }
 // Auto-select the matching Tech Pack (or OB/PO if no separate Tech Pack)
 const preferredTP=matching.find(x=>extractFileStyleAndRole(x.filename).role==='TECH_PACK')||matching.find(x=>x.id!==selectedPOId)||matching[0];
 if(matching.some(x=>x.id===prevBOM))bomSel.value=prevBOM;
 else if(preferredTP)bomSel.value=preferredTP.id;
 report('load-message',`Scoped to Style ${poInfo.styleKey}: filtered Tech Pack / Bulletin list to ${matching.length} matching file(s) (${matching.map(x=>x.filename).join(' · ')}).`);
}

async function loadSources(){
 try{
  const out=await api('/api/v1/documents');
  sources=(out.items||[]).filter(x=>['EXTRACTED','PARTIAL'].includes(x.status));
  // Sort PO / Order files to the top of #po-source, and hide Plant Master CSVs unless no other files exist.
  const orderFiles=sources.filter(x=>extractFileStyleAndRole(x.filename).role!=='PLANT_MASTER');
  const poList=orderFiles.length?orderFiles:sources;
  poList.sort((a,b)=>{
   const ra=extractFileStyleAndRole(a.filename).role==='ANCHOR_PO'?0:1;
   const rb=extractFileStyleAndRole(b.filename).role==='ANCHOR_PO'?0:1;
   return ra-rb;
  });
  for(const item of poList){
   const info=extractFileStyleAndRole(item.filename);
   const prefix=info.styleKey?`[${info.styleKey}] `:'';
   opt($('po-source'),item.id,prefix+item.filename);
  }
  for(const item of sources)opt($('bom-source'),item.id,item.filename);
  if(sources.length===1){$('po-source').value=sources[0].id;$('bom-source').value=sources[0].id;}
  // Check if user clicked "Check <Style> in Chapter 02" from Chapter 10
  if(typeof window!=='undefined'&&window.sessionStorage){
   try{
    const ctx=JSON.parse(window.sessionStorage.getItem('agiparel:active-po-context:v51')||'null');
    if(ctx&&ctx.po_document_id&&sources.some(x=>x.id===ctx.po_document_id)){
     $('po-source').value=ctx.po_document_id;
     syncScopedBOMOptions(ctx.po_document_id,false);
     if(ctx.bom_document_id&&sources.some(x=>x.id===ctx.bom_document_id))$('bom-source').value=ctx.bom_document_id;
     return;
    }
   }catch{}
  }
  report('load-message',sources.length?`${sources.length} file(s) ready. Pick a Purchase Order above to auto-filter its matching Tech Pack & Operation Bulletin.`:'No files ready yet. Upload a PO or tech pack, then return here.');
 }catch(err){report('load-message',err.message,true)}
}

const input=(value,cls,placeholder='')=>{const n=elt('input',cls);n.value=value??'';n.placeholder=placeholder;return n};
function cell(row,node,label){const td=elt('td','',undefined,row);if(label)td.dataset.label=label;td.append(node);return td}

function describeOrigin(type, ordinal){
 const n=Number(ordinal)||0;
 const role=type==='sku'?'PO File':'Tech Pack';
 const srcObj=type==='sku'?preview?.po_source:preview?.bom_source;
 const fname=srcObj?.filename||role;
 if(!n)return {label:'Manual Entry',title:'Entered or modified manually by reviewer',href:`/documents?document=${encodeURIComponent(type==='sku'?$('po-source').value:$('bom-source').value)}`};
 const seg=(srcObj?.segments||[]).find(s=>Number(s.ordinal)===n);
 let locText=`Section #${n}`;
 if(seg?.locator){
  const loc=typeof seg.locator==='string'?(()=>{try{return JSON.parse(seg.locator)}catch{return {}}})():seg.locator;
  if(loc.page)locText=`Page ${loc.page}`;
  else if(loc.sheet&&loc.row)locText=`${loc.sheet} · Row ${loc.row}`;
  else if(loc.row)locText=`Row ${loc.row}`;
  else if(loc.line)locText=`Line ${loc.line}`;
 }
 const docId=type==='sku'?$('po-source').value:$('bom-source').value;
 return {
  label:`${role} · ${locText} ↗`,
  title:`Extracted from ${fname} (${locText}, segment #${n})`,
  href:`/documents?document=${encodeURIComponent(docId)}&segment=${n}`
 };
}

function buildAutoUPC(r){
 const style=(r.dataset.buyerStyleNo||$('style-code').value||'STYLE').trim().replace(/[^A-Za-z0-9_-]/g,'');
 const color=(r.querySelector('[data-field="color"]')?.value||'CLR').trim().replace(/[^A-Za-z0-9_-]/g,'_');
 const size=(r.querySelector('[data-field="size"]')?.value||'SZ').trim().replace(/[^A-Za-z0-9_-]/g,'');
 return `${style}-${color}-${size}`.slice(0,64);
}

function makeRow(type,item={}){const target=$(type==='sku'?'sku-rows':'bom-rows'),r=elt('tr','',undefined,target);r.dataset.kind=type;
 function field(key,value,placeholder){const n=input(value,'',placeholder);n.dataset.field=key;cell(r,n,({color:'Color',size:'Size',quantity:'Pieces',upc:'UPC / Item Barcode',material_sku:'Material code',per_garment:'Per garment',uom:'Unit'})[key]);return n}
 if(type==='sku'){
  r.dataset.materialNumber=item.material_number||'';
  r.dataset.buyerStyleNo=item.buyer_style_no||'';
  r.dataset.packRatio=item.pack_ratio||'';
  r.dataset.unitCost=item.unit_cost||'';
  field('color',item.color,'Color');
  field('size',item.size,'Size');
  field('quantity',item.quantity??'','Pieces');
  const upcWrap=elt('div','upc-cell-wrap');
  const upcInput=input(item.upc??'','','Not in PO — enter or auto-assign');
  upcInput.dataset.field='upc';
  upcWrap.append(upcInput);
  const assignBtn=elt('button','secondary upc-assign-btn','Auto-assign',upcWrap);
  assignBtn.type='button';
  assignBtn.title='Assign deterministic Style-Color-Size barcode identifier when UPC is not printed in PO';
  const syncUpcBtn=()=>{assignBtn.hidden=Boolean(upcInput.value.trim())};
  upcInput.addEventListener('input',syncUpcBtn);
  assignBtn.addEventListener('click',()=>{upcInput.value=buildAutoUPC(r);syncUpcBtn();renderOrderSummary()});
  syncUpcBtn();
  cell(r,upcWrap,'UPC / Item Barcode');
 }
 else{r.dataset.styleCode=item.style_code||'';field('material_sku',item.material_sku,'Material code');const sel=elt('select');sel.dataset.field='category';for(const cat of ['FABRIC','TRIM'])opt(sel,cat,cat);sel.value=item.category||'FABRIC';cell(r,sel,'Type');field('per_garment',item.per_garment,'1.850');field('uom',item.uom,'m / pcs')}
 const provenance=elt('div','origin-cell');
 const ord=input(item.source_ordinal??0,'ord','0');ord.type='number';ord.min='0';ord.max='5000';ord.dataset.field='source_ordinal';ord.hidden=true;
 provenance.append(ord);
 const src=elt('a','origin-badge','Open evidence ↗',provenance);src.target='_blank';src.rel='noopener';src.href='#';
 const toggleManual=elt('button','origin-toggle','',provenance);toggleManual.type='button';
 const initialOrd=Number(item.source_ordinal)||0;
 function setEvidenceLink(){
  const n=Number(ord.value)||0;
  const info=describeOrigin(type,n);
  src.href=info.href;
  src.textContent=info.label;
  src.title=info.title;
  src.className=n?'origin-badge is-doc':'origin-badge is-manual';
  if(initialOrd>0){
   toggleManual.hidden=false;
   toggleManual.textContent=n?'Mark manual':'Restore doc link';
   toggleManual.title=n?'Mark this row as manually overridden':'Restore link to original document page';
  }else{
   toggleManual.hidden=true;
  }
 }
 ord.addEventListener('input',()=>setEvidenceLink());
 toggleManual.addEventListener('click',()=>{ord.value=Number(ord.value)>0?'0':String(initialOrd);setEvidenceLink();renderOrderSummary()});
 setEvidenceLink();
 cell(r,provenance,'Document Origin');
 const remove=elt('button','remove','×');remove.type='button';remove.title='Remove this line';remove.setAttribute('aria-label',`Remove ${type==='sku'?'size':'material'} row`);remove.addEventListener('click',()=>{r.remove();renderOrderSummary()});cell(r,remove,'Remove');if(type==='material'){const code=r.querySelector('[data-field="material_sku"]');if(code.value.trim()&&!validMaterialSKU(code.value.trim()))offerMaterialCode(r,code)}return r;
}
function assignMissingUPCs(){
 let assigned=0;
 for(const tr of $('sku-rows').querySelectorAll('tr')){
  const upcField=tr.querySelector('[data-field="upc"]');
  if(upcField && !upcField.value.trim()){
   upcField.value=buildAutoUPC(tr);
   const btn=tr.querySelector('.upc-assign-btn');
   if(btn)btn.hidden=true;
   assigned++;
  }
 }
 renderOrderSummary();
 report('stage-message',assigned?`Assigned ${assigned} SKU barcode identifier(s) from Style-Color-Size. Replace with GS1 12-digit UPCs if provided on buyer ticket sheets.`:'All size rows already have a UPC / barcode identifier.');
}
function captureOption(){
 if(selectedOption<0)return;
 optionEdits.set(selectedOption,{
  po_number:$('po-number').value,style_code:$('style-code').value,
  skus:readRows('sku-rows'),materials:readRows('bom-rows'),
  review_note:$('review-note').value,acknowledged:$('acknowledged').checked,
  supersede_digest:confirmedSupersedeDigest
 });
}
function setOptions(){
 const sel=$('po-choice');sel.replaceChildren();optionEdits.clear();selectedOption=-1;confirmedSupersedeDigest='';
 for(let i=0;i<poOptions.length;i++){
  const so=poOptions[i].details?.sales_order_no?` (SO# ${poOptions[i].details.sales_order_no})`:'';
  opt(sel,String(i),`${poOptions[i].po_number}${so} · ${poOptions[i].style_code} (${poOptions[i].skus.length} lines)`);
 }
 $('multiple-po').hidden=poOptions.length<2 && !poOptions[0]?.details && !drawings.length && !existingDrafts.length;
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
 confirmedSupersedeDigest=saved?.supersede_digest??'';
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
 clearConflictBanner();
 renderOrderSummary();
}
function invalidateReview(){
 analysisEpoch++;preview=null;analyzedSources=null;poOptions=[];materials=[];techHints=[];drawings=[];existingDrafts=[];
 selectedOption=-1;confirmedSupersedeDigest='';optionEdits.clear();$('review').hidden=true;$('result').hidden=true;
 $('analyze').disabled=false;
 clearConflictBanner();
 report('load-message','Sources changed. Read order details again before reviewing or saving.');
}
function reviewSourcesCurrent(){
 return Boolean(preview&&analyzedSources&&
  $('po-source').value===analyzedSources.po&&$('bom-source').value===analyzedSources.bom);
}

function diffAgainstColleagueDraft(colleague, currentReq){
 const diffs=[];
 if(colleague.style_code && currentReq.style_code && colleague.style_code!==currentReq.style_code){
  diffs.push({scope:'Order Header',key:'Style Code',colleague_value:colleague.style_code,your_value:currentReq.style_code});
 }
 const savedSKUs=new Map(), currSKUs=new Map(), skuKeys=[];
 for(const s of (colleague.skus||[])){const k=`${s.color} · Size ${s.size}`;savedSKUs.set(k,s);if(!skuKeys.includes(k))skuKeys.push(k);}
 for(const s of (currentReq.skus||[])){const k=`${s.color} · Size ${s.size}`;currSKUs.set(k,s);if(!skuKeys.includes(k))skuKeys.push(k);}
 for(const k of skuKeys){
  const a=savedSKUs.get(k), b=currSKUs.get(k);
  if(a&&!b)diffs.push({scope:'SKU Line',key:k,colleague_value:`${a.quantity} pcs (UPC: ${a.upc||'none'})`,your_value:'Removed / Missing'});
  else if(!a&&b)diffs.push({scope:'SKU Line',key:k,colleague_value:'Not present',your_value:`${b.quantity} pcs (UPC: ${b.upc||'none'})`});
  else if(a&&b&&(Number(a.quantity)!==Number(b.quantity)||(a.upc||'')!==(b.upc||''))){
   diffs.push({scope:'SKU Line',key:k,colleague_value:`${a.quantity} pcs · UPC ${a.upc||'none'}`,your_value:`${b.quantity} pcs · UPC ${b.upc||'none'}`});
  }
 }
 const savedBOM=new Map(), currBOM=new Map(), bomKeys=[];
 for(const m of (colleague.materials||[])){const k=`${m.category} · ${m.material_sku}`;savedBOM.set(k,m);if(!bomKeys.includes(k))bomKeys.push(k);}
 for(const m of (currentReq.materials||[])){const k=`${m.category} · ${m.material_sku}`;currBOM.set(k,m);if(!bomKeys.includes(k))bomKeys.push(k);}
 for(const k of bomKeys){
  const a=savedBOM.get(k), b=currBOM.get(k);
  if(a&&!b)diffs.push({scope:'BOM Material',key:k,colleague_value:`${a.per_garment} ${a.uom}`,your_value:'Removed / Missing'});
  else if(!a&&b)diffs.push({scope:'BOM Material',key:k,colleague_value:'Not present',your_value:`${b.per_garment} ${b.uom}`});
  else if(a&&b&&(String(a.per_garment)!==String(b.per_garment)||String(a.uom||'').toUpperCase()!==String(b.uom||'').toUpperCase())){
   diffs.push({scope:'BOM Material',key:k,colleague_value:`${a.per_garment} ${a.uom}`,your_value:`${b.per_garment} ${b.uom}`});
  }
 }
 return diffs;
}

function clearConflictBanner(){
 $('review')?.querySelector('.colleague-conflict-card')?.remove();
}

function renderConflictBanner(colleague, diffs, fromServer=false){
 const host=$('review');
 if(!host||!colleague)return;
 let card=host.querySelector('.colleague-conflict-card');
 if(!diffs.length && !fromServer){if(card)card.remove();return;}
 if(!card){
  card=elt('section','colleague-conflict-card',undefined);
  const stageBtn=$('stage')?.parentElement;
  if(stageBtn && stageBtn.parentElement===host)host.insertBefore(card,stageBtn);
  else host.append(card);
 }
 card.replaceChildren();
 const timeStr=colleague.captured_at?new Date(colleague.captured_at).toLocaleString():'recently';
 const head=elt('div','conflict-head',undefined,card);
 elt('strong','',`⚠️ Colleague Draft Conflict on PO ${colleague.po_number}`,head);
 elt('span','conflict-badge',`Saved by ${colleague.reviewed_by||'colleague'} · ${timeStr}`,head);
 elt('p','conflict-desc',`Colleague "${colleague.reviewed_by||'dev'}" already saved a draft for PO ${colleague.po_number} (Intake ${colleague.intake_id}) with note: "${colleague.review_note||'—'}". Coordinate by phone/in-person to reconcile the ${diffs.length} difference(s) below so Material Planning and Cutting do not receive conflicting orders.`,card);
 if(diffs.length){
  const wrap=elt('div','scroll',undefined,card);
  const tbl=elt('table','conflict-diff-table',undefined,wrap);
  const thead=elt('thead','',undefined,tbl), hr=elt('tr','',undefined,thead);
  elt('th','','Section',hr);elt('th','','Item / Size / Material',hr);
  elt('th','',`${colleague.reviewed_by||'Colleague'}'s Saved Value`,hr);
  elt('th','','Your Current Screen Value',hr);
  const tbody=elt('tbody','',undefined,tbl);
  for(const d of diffs){
   const tr=elt('tr','',undefined,tbody);
   elt('td','',d.scope,tr);elt('td','',d.key,tr);
   elt('td','diff-colleague',d.colleague_value,tr);
   elt('td','diff-yours',d.your_value,tr);
  }
 }
 const actions=elt('div','actions conflict-actions',undefined,card);
 const adoptBtn=elt('button','secondary',`Load ${colleague.reviewed_by||'colleague'}'s saved values into my screen`,actions);
 adoptBtn.type='button';
 adoptBtn.addEventListener('click',()=>{
  $('po-number').value=colleague.po_number||$('po-number').value;
  $('style-code').value=colleague.style_code||$('style-code').value;
  if((colleague.skus||[]).length){
   $('sku-rows').replaceChildren();
   for(const s of colleague.skus)makeRow('sku',s);
  }
  if((colleague.materials||[]).length){
   $('bom-rows').replaceChildren();
   for(const m of colleague.materials)makeRow('material',m);
  }
  confirmedSupersedeDigest='';
  clearConflictBanner();
  renderOrderSummary();
  report('stage-message',`Loaded ${colleague.reviewed_by}'s saved draft for PO ${colleague.po_number}. Your screen now matches their saved record.`);
 });
 const overrideBtn=elt('button','primary',`I spoke with ${colleague.reviewed_by||'colleague'} — authorize override`,actions);
 overrideBtn.type='button';
 overrideBtn.addEventListener('click',()=>{
  confirmedSupersedeDigest=colleague.source_digest;
  const note=$('review-note');
  const prefix=`[Reconciled with ${colleague.reviewed_by}: `;
  if(!note.value.includes(prefix)){
   note.value=`${prefix}agreed to supersede ${colleague.intake_id}] ${note.value}`.trim();
  }
  clearConflictBanner();
  report('stage-message',`Override authorized for ${colleague.reviewed_by}'s draft (${colleague.intake_id}). Click "Save draft order →" to commit your reconciled revision.`);
  note.focus();
 });
}

function checkLiveColleagueConflict(){
 const poNum=$('po-number').value.trim();
 if(!poNum||!existingDrafts||!existingDrafts.length)return;
 const match=existingDrafts.find(d=>d.po_number===poNum);
 if(!match){clearConflictBanner();return;}
 if(confirmedSupersedeDigest===match.source_digest)return;
 const currentReq={
  po_number:poNum,
  style_code:$('style-code').value.trim(),
  skus:readRows('sku-rows'),
  materials:readRows('bom-rows')
 };
 const diffs=diffAgainstColleagueDraft(match,currentReq);
 if(diffs.length>0){
  renderConflictBanner(match,diffs,false);
 }else{
  clearConflictBanner();
 }
}

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
 const missingUpc=skus.filter(r=>!r.upc).length;
 const needsRate=bom.filter(r=>!/^(?:0|[1-9]\d*)(?:\.\d{1,3})?$/.test(String(r.per_garment))||Number(r.per_garment)<=0||Number(r.per_garment)>1000000||!r.uom).length;
 const styleMismatches=bom.filter(r=>r.style_code&&r.style_code!==$('style-code').value).length;
 const summary=$('order-summary');
 const details=selectedOption>=0?poOptions[selectedOption]?.details:null;
 const metaParts=[];
 if(details){
  if(details.sales_order_no)metaParts.push(`SO# ${details.sales_order_no}`);
  if(details.buyer_style_no)metaParts.push(`Style ${details.buyer_style_no}`);
  if(details.material_no)metaParts.push(`Mat# ${details.material_no}`);
  if(details.total_cases)metaParts.push(`${details.total_cases} cases`);
  if(details.unit_cost)metaParts.push(`${details.unit_cost}/unit`);
  if(details.total_order_value)metaParts.push(`Total ${details.total_order_value}`);
 }
 summary.textContent=`${skus.length} size line(s) · ${valid?total.toLocaleString('en-US'):'Check piece quantities'} pieces · ${bom.length} material line(s) · ${manual} manual-source line(s)`+(metaParts.length?` · ${metaParts.join(' · ')}`:'');
 renderDrawingsCard();
 renderOrderDetailsCard(details);
 renderSizePivotMatrix(skus, details);
 checkLiveColleagueConflict();
 const flags=[];
 if(missingUpc)flags.push(`${missingUpc} size line(s) have no UPC in PO (use "Assign missing UPCs / SKU IDs" or enter buyer GTIN)`);
 if(needsRate)flags.push(`${needsRate} material usage/unit value(s) to verify`);
 if(styleMismatches)flags.push(`${styleMismatches} material line(s) belong to another style`);
 if(!bom.some(m=>m.category==='FABRIC'))flags.push('No fabric requirement entered');
 $('order-checks').textContent=flags.length?`Before saving: ${flags.join(' · ')}.`:'Draft quantities still require buyer confirmation, revision and delivery checks before any commitment.';
}
function renderDrawingsCard(){
 const host=$('multiple-po');
 if(!host)return;
 let box=host.querySelector('.po-drawings-card');
 if(!drawings||!drawings.length){if(box)box.remove();return;}
 host.hidden=false;
 if(!box){box=elt('details','techpack-panel po-drawings-card',undefined,host);box.open=true;}
 box.replaceChildren();
 elt('summary','',`Product drawings & construction sketches (${drawings.length})`,box);
 const grid=elt('div','po-drawings-grid',undefined,box);
 for(const d of drawings){
  const card=elt('figure','po-drawing-item',undefined,grid);
  const img=elt('img','po-drawing-img',undefined,card);
  img.src=d.data_url;img.alt=d.title;img.loading='lazy';
  const cap=elt('figcaption','po-drawing-cap',undefined,card);
  elt('strong','',d.title,cap);
  elt('span','',`${d.source} · ${d.width}×${d.height}px · Click to enlarge`,cap);
  card.addEventListener('click',()=>openDrawingLightbox(d));
 }
}
function openDrawingLightbox(d){
 let modal=document.querySelector?.('.po-drawing-modal');
 if(modal)modal.remove();
 modal=elt('div','po-drawing-modal',undefined,document.body);
 const inner=elt('div','po-drawing-modal-inner',undefined,modal);
 const head=elt('div','po-drawing-modal-head',undefined,inner);
 elt('strong','',`${d.title} (${d.source})`,head);
 const close=elt('button','secondary','Close ×',head);close.type='button';
 close.addEventListener('click',()=>modal.remove());
 const img=elt('img','po-drawing-modal-img',undefined,inner);
 img.src=d.data_url;img.alt=d.title;
 modal.addEventListener('click',e=>{if(e.target===modal)modal.remove()});
}
function renderOrderDetailsCard(details){
 const host=$('multiple-po');
 if(!host)return;
 let box=host.querySelector('.po-details-card');
 if(!details){if(box)box.remove();return;}
 host.hidden=false;
 if(!box){box=elt('details','techpack-panel po-details-card',undefined,host);box.open=true;}
 box.replaceChildren();
 elt('summary','','Commercial, logistics, fabric & packing details from PO',box);
 const grid=elt('div','select-grid',undefined,box);
 const pairs=[
  ['SO / PO #',details.sales_order_no],['Customer PO #',details.customer_po_no],
  ['Project / Style',details.project_no],['Buyer Style #',details.buyer_style_no],
  ['Material #',details.material_no],['PLM #',details.plm_no],
  ['Program #',details.program_no],['Supplier #',details.supplier_no],
  ['Customer / Buyer',[details.customer,details.buyer].filter(Boolean).join(' · ')],
  ['Vendor / Goods Supplier',[details.vendor_code,details.goods_supplier].filter(Boolean).join(' · ')],
  ['Country of Origin',details.country_of_origin],['Factory ID',details.factory_id],
  ['Dept / Fineline / Season',[details.dept,details.fineline,details.season,details.season_code].filter(Boolean).join(' · ')],
  ['Unit Cost / Retail / Total',[details.unit_cost,details.retail_price,details.total_order_value].filter(Boolean).join(' · ')],
  ['Total Cases / Whse',[details.total_cases,details.warehouse,details.ship_to].filter(Boolean).join(' · ')],
  ['In-Whse / MABD / Cancel',[details.in_whse_date,details.mabd,details.cancel_date].filter(Boolean).join(' · ')],
  ['Ex-Factory / Ex-Port / ETA',[details.ex_factory_date,details.ex_port_date,details.eta_to_port].filter(Boolean).join(' · ')],
  ['Incoterms / Ship Mode / Ports',[details.incoterms,details.ship_mode,details.port_of_exit,details.port_of_entry].filter(Boolean).join(' · ')],
  ['Fabric Spec & Content',[details.fabric_spec,details.fabric_content,details.fabric_weight_gsm,details.hts_code].filter(Boolean).join(' · ')]
 ];
 for(const [label,val] of pairs){
  if(!val)continue;
  const wrap=elt('div','',undefined,grid);
  elt('strong','',`${label}: `,wrap);
  elt('span','',val,wrap);
 }
 const acc=details.accessories||{};
 const accList=Object.entries(acc).map(([k,v])=>`${k}: ${v}`);
 const pack=details.packing_rules||[];
 const rulesText=[accList.length?`Accessories: ${accList.join(' | ')}`:'',pack.length?`Packing: ${pack.join(' · ')}`:''].filter(Boolean).join(' — ');
 if(rulesText)elt('p','message',rulesText,box);
}
function renderSizePivotMatrix(skus, details){
 const host=$('multiple-po');
 if(!host)return;
 let box=host.querySelector('.po-pivot-card');
 if(!skus||!skus.length||!skus.some(s=>s.color&&s.size)){if(box)box.remove();return;}
 host.hidden=false;
 if(!box){box=elt('details','techpack-panel po-pivot-card',undefined,host);box.open=true;}
 box.replaceChildren();
 elt('summary','','Order size matrix by colorway (matches PO layout — edit quantities directly)',box);
 const sizes=[];
 const byColor=new Map();
 skus.forEach((row,idx)=>{
  if(!row.color||!row.size)return;
  if(!sizes.includes(row.size))sizes.push(row.size);
  const key=row.color;
  if(!byColor.has(key)){
   byColor.set(key,{
    color:row.color,
    material_number:row.material_number||details?.material_no||'',
    buyer_style_no:row.buyer_style_no||details?.buyer_style_no||'',
    unit_cost:row.unit_cost||details?.unit_cost||'',
    cells:new Map()
   });
  }
  byColor.get(key).cells.set(row.size,{row,idx});
 });
 const wrap=elt('div','scroll',undefined,box);
 const tbl=elt('table','po-pivot-table',undefined,wrap);
 const thead=elt('thead','',undefined,tbl), hr=elt('tr','',undefined,thead);
 elt('th','','Material #',hr);
 elt('th','','Colorway',hr);
 elt('th','','Buyer Style #',hr);
 for(const sz of sizes){
  let ratio='';
  for(const g of byColor.values()){const c=g.cells.get(sz);if(c?.row?.pack_ratio){ratio=c.row.pack_ratio;break;}}
  elt('th','',ratio?`Size ${sz} (${ratio})`:`Size ${sz}`,hr);
 }
 elt('th','','Total Units',hr);
 elt('th','','Cartons',hr);
 elt('th','','Unit Cost',hr);
 elt('th','','Line Total',hr);
 const tbody=elt('tbody','',undefined,tbl);
 for(const g of byColor.values()){
  const tr=elt('tr','',undefined,tbody);
  elt('td','',g.material_number||'—',tr);
  elt('td','',g.color.replaceAll('_',' '),tr);
  elt('td','',g.buyer_style_no||$('style-code').value||'—',tr);
  let rowUnits=0, rowCartons=0, hasCartonRule=false;
  for(const sz of sizes){
   const td=elt('td','',undefined,tr);
   const entry=g.cells.get(sz);
   if(!entry){td.textContent='—';continue;}
   const q=Number(entry.row.quantity)||0;
   rowUnits+=q;
   const ratio=entry.row.pack_ratio||'';
   let ctnCount=0;
   if(ratio.includes('/')){
    const ctnPcs=Number(ratio.split('/')[0]);
    if(ctnPcs>0&&q>0){ctnCount=Math.ceil(q/ctnPcs);rowCartons+=ctnCount;hasCartonRule=true;}
   }
   const inp=elt('input','pivot-qty-input',undefined,td);
   inp.type='number';inp.min='1';inp.max='100000000';inp.value=String(entry.row.quantity??'');
   inp.addEventListener('change',()=>{
    const targetRow=$('sku-rows').querySelectorAll('tr')[entry.idx];
    const qField=targetRow?.querySelector('[data-field="quantity"]');
    if(qField){qField.value=inp.value.trim();renderOrderSummary();}
   });
   if(ctnCount>0)elt('small','pivot-sub',`${ctnCount.toLocaleString()} ctns (${ratio})`,td);
  }
  elt('td','pivot-total',rowUnits.toLocaleString('en-US'),tr);
  elt('td','',hasCartonRule?rowCartons.toLocaleString('en-US'):(details?.total_cases||'—'),tr);
  elt('td','',g.unit_cost||'—',tr);
  const numericCost=Number(String(g.unit_cost||'').replace(/[^0-9.]/g,''));
  elt('td','pivot-total',numericCost>0?`$${(rowUnits*numericCost).toLocaleString('en-US',{minimumFractionDigits:2,maximumFractionDigits:2})}`:(details?.total_order_value||'—'),tr);
 }
}
function renderTechPackHints(){
 const box=$('techpack-workbench'), list=$('techpack-candidates');list.replaceChildren();
 const coverage=preview?.techpack_bom_coverage||{};
 const poHints=(selectedOption>=0&&poOptions[selectedOption]?.po_materials)||[];
 const allHints=[...poHints,...techHints];
 box.hidden=!allHints.length && !coverage.bom_pages;
 if(box.hidden)return;
 const total=Number(coverage.material_rows||techHints.length)+poHints.length;
 $('techpack-summary').textContent=`Recognized ${total} candidate component(s) (${poHints.length} from PO, ${techHints.length} from Tech Pack across ${coverage.bom_pages||0} BOM page(s)). 0 verified per-garment rates were imported from these PDF suggestions. Every candidate requires human review.`;
 for(const hint of allHints){
  const item=elt('article','tech-candidate',undefined,list);
  const heading=elt('div','tech-candidate-heading',undefined,item);
  elt('strong','',hint.component,heading);
  elt('span','tech-state',hint.quantity_state==='ZERO_UNVERIFIED'?'Zero is not usage':'Rate unverified',heading);
  const parts=elt('div','tech-parts',undefined,item);
  const roleLabel=hint.section?.startsWith('PO ')?'PO File':'Tech Pack';
  elt('span','',`${hint.category} · ${hint.section} · ${roleLabel} Page ${hint.page||'?'}`,parts);
  elt('p','tech-material',hint.material_label,item);
  const refMeta=[hint.reference_no?`Ref: ${hint.reference_no}`:'',hint.colorways?`Colorways: ${hint.colorways}`:''].filter(Boolean).join(' · ');
  if(refMeta)elt('p','tech-raw',refMeta,item);
  elt('p','tech-raw',`Document Quantity: ${hint.raw_quantity||'not shown'} · Suggested code: ${hint.suggested_code||'needs identification'}`,item);
  elt('p','tech-explain',hint.hint,item);
  const controls=elt('div','tech-actions',undefined,item);
  const link=elt('a','quiet',`Open ${roleLabel} Page ${hint.page||hint.source_ordinal} ↗`,controls);
  const docId=hint.section?.startsWith('PO ')?$('po-source').value:$('bom-source').value;
  link.href=`/documents?document=${encodeURIComponent(docId)}&segment=${hint.source_ordinal}`;
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
function sourceExcerpts(src,target,title){$(target).replaceChildren();$(title).textContent=src.filename;const segments=src.segments||[];for(const s of segments.slice(0,35)){const row=elt('div','source-row',undefined,$(target)),lnk=elt('a','',`Page/Row #${s.ordinal}`,row);lnk.href=`/documents?document=${encodeURIComponent(src.id)}&segment=${s.ordinal}`;lnk.target='_blank';lnk.rel='noopener';elt('span','',` · ${s.kind} · `,row);const pre=elt('pre','',s.content.slice(0,360)+(s.content.length>360?'…':''),row)}if(segments.length>35)elt('p','',`Showing 35 of ${segments.length} snippets. Open the document to search the rest.`,$(target))}
async function analyze(){
 const po=$('po-source').value,bom=$('bom-source').value;
 if(!po||!bom){report('load-message','Choose both files. If one file contains both, select it twice.',true);return}
 const epoch=++analysisEpoch;
 preview=null;analyzedSources=null;$('review').hidden=true;$('result').hidden=true;
 report('load-message','Reading source evidence…');$('analyze').disabled=true;
 try{
  const out=await api(`/api/v1/buyer-review?po_document_id=${encodeURIComponent(po)}&bom_document_id=${encodeURIComponent(bom)}`);
  if(epoch!==analysisEpoch||$('po-source').value!==po||$('bom-source').value!==bom)return;
  preview=out;analyzedSources={po,bom};poOptions=out.po_options||[];
  materials=out.materials||[];techHints=out.techpack_material_hints||[];drawings=out.drawings||[];existingDrafts=out.existing_drafts||[];
  setOptions();renderTechPackHints();
  sourceExcerpts(out.po_source,'po-evidence','po-evidence-title');
  sourceExcerpts(out.bom_source,'bom-evidence','bom-evidence-title');
  $('review').hidden=false;
  const detected=poOptions.reduce((n,o)=>n+o.skus.length,0);
  const savedNotice=existingDrafts.length?` ${existingDrafts.length} saved colleague draft(s) found for these files.`:'';
  $('coverage').textContent=`Found ${detected} size line(s) and ${materials.length} material line(s). Check every value before saving.${techHints.length?` ${techHints.length} extra material suggestion(s) need checking.`:''}${savedNotice}`;
  $('issues').hidden=!(out.issues||[]).length;
  if((out.issues||[]).length)$('issues').textContent=(out.issues||[]).join(' · ');
  report('load-message','Order details are ready. Check them below.');
  $('review').scrollIntoView({behavior:'smooth',block:'start'});
 }catch(err){if(epoch===analysisEpoch)report('load-message',err.message,true)}
 finally{if(epoch===analysisEpoch)$('analyze').disabled=false}
}
function readRows(id){return Array.from($(id).querySelectorAll('tr')).map(tr=>{const d={};for(const node of tr.querySelectorAll('[data-field]')){let value=node.value;if(typeof value==='string')value=value.trim();if(node.dataset.field==='quantity'||node.dataset.field==='source_ordinal')value=value===''?null:Number(value);d[node.dataset.field]=value}if(tr.dataset.kind==='sku'){if(tr.dataset.materialNumber)d.material_number=tr.dataset.materialNumber;if(tr.dataset.buyerStyleNo)d.buyer_style_no=tr.dataset.buyerStyleNo;if(tr.dataset.packRatio)d.pack_ratio=tr.dataset.packRatio;if(tr.dataset.unitCost)d.unit_cost=tr.dataset.unitCost;}if(tr.dataset.kind==='material'&&tr.dataset.styleCode)d.style_code=tr.dataset.styleCode;return d})}
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
async function stage(){
 if(!reviewSourcesCurrent()){report('stage-message','Source files changed. Read the order details again before saving.',true);return}
 const req={
  po_document_id:$('po-source').value,bom_document_id:$('bom-source').value,
  po_number:$('po-number').value.trim(),style_code:$('style-code').value.trim(),
  skus:readRows('sku-rows'),materials:readRows('bom-rows'),
  order_details:selectedOption>=0?poOptions[selectedOption]?.details:undefined,
  review_note:$('review-note').value.trim(),acknowledged:$('acknowledged').checked,
  supersede_digest:confirmedSupersedeDigest||undefined
 };
 if(!req.acknowledged){report('stage-message','Please check the confirmation box before saving.',true);return}
 if(!checkMaterialRows()||!checkReviewedQuantities(req))return;
 if(req.review_note.length<8){report('stage-message','Add a short note on what you checked (at least 8 characters).',true);$('review-note').focus();return}
 $('stage').disabled=true;report('stage-message','Checking for colleague conflicts and saving your draft…');
 try{
  const out=await api('/api/v1/buyer-review',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(req)});
  if(!reviewSourcesCurrent()||$('po-number').value.trim()!==req.po_number||$('style-code').value.trim()!==req.style_code){
   $('result').hidden=true;
   report('load-message',`Draft for ${out.intake.po_number} was saved, but the current review changed. Recheck the current order before saving.`,true);
   return;
  }
  clearConflictBanner();
  $('result').hidden=false;
  $('result-title').textContent=`${out.intake.po_number} · ${out.intake.ordered_qty.toLocaleString()} pieces`;
  $('result-message').textContent=`Draft ${out.reused?'already saved':'saved'} for ${out.intake.po_number}. Review material needs next. Buying, stock and production remain on hold.`;
  report('stage-message','Staged successfully. Review the result below.');
  $('result').scrollIntoView({behavior:'smooth',block:'start'});
 }catch(err){
  if(err.conflict && err.conflict.colleague){
   const c=err.conflict.colleague;
   const idx=existingDrafts.findIndex(d=>d.po_number===c.po_number);
   if(idx>=0)existingDrafts[idx]=c;else existingDrafts.unshift(c);
   renderConflictBanner(c,err.conflict.discrepancies||[],true);
   report('stage-message',err.conflict.message||'Colleague conflict detected. Resolve discrepancies above before saving.',true);
   return;
  }
  const match=/^BOM row (\d+): material SKU:/.exec(err.message);
  if(match)showMaterialError(Number(match[1])-2);
  else report('stage-message',err.message,true);
 }finally{$('stage').disabled=false}
}
$('analyze').addEventListener('click',()=>void analyze());
$('po-source').addEventListener('change',()=>{
 invalidateReview();
 syncScopedBOMOptions($('po-source').value,false);
});
$('bom-source').addEventListener('change',invalidateReview);
$('po-choice').addEventListener('change',e=>{captureOption();applyOption(Number(e.target.value));renderTechPackHints()});
$('style-code').addEventListener('change',()=>{renderOrderSummary();renderTechPackHints()});
$('acknowledged').addEventListener('change',()=>{if($('acknowledged').checked)checkLiveColleagueConflict()});
$('add-sku').addEventListener('click',()=>{makeRow('sku');renderOrderSummary()});
$('review').addEventListener('click',e=>{if(e.target?.id==='assign-upcs')assignMissingUPCs()});
$('add-material').addEventListener('click',()=>{makeRow('material');renderOrderSummary()});
$('review').addEventListener('input',e=>{if(e.target?.dataset?.field)renderOrderSummary()});
$('review').addEventListener('change',e=>{if(e.target?.dataset?.field)renderOrderSummary()});
if(typeof document.addEventListener==='function'){
 document.addEventListener('agiparel:po-context-changed',e=>{
  const d=e.detail;
  if(d&&d.po_document_id&&sources.some(x=>x.id===d.po_document_id)){
   $('po-source').value=d.po_document_id;
   syncScopedBOMOptions(d.po_document_id,false);
  }
 });
}
$('stage').addEventListener('click',()=>void stage());void loadSources();
})();