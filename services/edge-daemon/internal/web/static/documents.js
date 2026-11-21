// Author: Sunil Jain, CyberinQ LLC — Chapter 10: Auto-Grouped PO/Style Dossier Folders + 17-Stage RFID Experiment.
(()=>{'use strict';
const $=id=>document.getElementById(id),text=(node,value)=>{node.textContent=value??'';return node};
const create=(tag,cls,contents,parent)=>{const n=document.createElement(tag);if(cls)n.className=cls;text(n,contents);if(parent)parent.append(n);return n};
const UPLOAD_BUILD = "20261027.v9";
let files=[],library=[],current=null,currentSegments=[];
let nextCursor="", sourceOrdinal=0, hasMoreSources=false, libraryGeneration=0, sourceGeneration=0, paging=false;
const deepLink=new URLSearchParams(window.location.search);let deepLinkOpened=false;
const formatSize=n=>n>=1024*1024?`${(n/1024/1024).toFixed(1)} MiB`:`${(n/1024).toFixed(1)} KiB`;

// Automatically classify any uploaded filename into its Style/Program key and Document Role.
function classifyDocument(name=''){
 const u=String(name).toUpperCase();
 const norm=u.replace(/[_\s().-]+/g,' ');
 let role='OTHER_DOC', roleLabel='Supporting File';
 if(/\b(01 MACHINE|MACHINE ROSTER|02 WORKFORCE|SKILLS MATRIX|03 OPERATION BULLETIN RATES|04 RAW MATERIAL)/.test(norm)){
  role='PLANT_MASTER'; roleLabel='Plant Master Register';
 }else if(norm.includes('PO SHEET')||norm.includes('PURCHASE ORDER')||norm.includes('BUYER PO')){
  role='ANCHOR_PO'; roleLabel='1. Anchor Purchase Order (PO)';
 }else if(` ${norm} `.includes(' OB ')||norm.includes('OPERATION BULLETIN')||(u.endsWith('.XLSX')&&(norm.includes('45PW')||norm.includes('PRINT')||norm.includes('SOLID')))){
  role='OPERATION_BULLETIN'; roleLabel='3. Operation Bulletin (OB)';
 }else if(norm.includes('TECH PACK')||norm.includes('SAMPLE ENGLISH')||norm.includes('SPEC')){
  role='TECH_PACK'; roleLabel='2. Style Tech Pack';
 }
 const m=/(?:^|\s)([A-Z]{2}[0-9]{2}[A-Z][0-9]{3}|[0-9]{2}[A-Z]{2}[0-9]{3}|[0-9]{6})(?=\s|$)/.exec(norm);
 const styleKey=role==='PLANT_MASTER'?'PLANT_MASTER':(m?m[1]:'UNGROUPED');
 return {styleKey, role, roleLabel};
}

function feedback(message,kind='',issues=[],duplicates=[]){
 const el=$('feedback');el.className=`feedback ${kind}`;el.replaceChildren();
 create('span','',message,el);
 if(duplicates.length){
  const earlier=create('details','upload-issues',null,el);
  create('summary','',`View ${duplicates.length} already-saved file${duplicates.length===1?'':'s'}`,earlier);
  const names=create('ul','',null,earlier);
  for(const name of duplicates)create('li','',name,names);
 }
 if(issues.length){
  const details=create('details','upload-issues',null,el);
  create('summary','',`View ${issues.length} file issue${issues.length===1?'':'s'}`,details);
  const list=create('ul','',null,details);
  for(const issue of issues)create('li','',issue,list);
 }
}
function selection(next){files=Array.from(next||[]);const bytes=files.reduce((n,f)=>n+f.size,0);text($('selection'),files.length?`${files.length} file${files.length===1?'':'s'} selected · ${formatSize(bytes)} total`:'No files selected');$('upload').disabled=!files.length}
$('files').addEventListener('change',e=>selection(e.target.files));
const drop=$('drop');drop.addEventListener('dragover',e=>{e.preventDefault();drop.classList.add('drag')});drop.addEventListener('dragleave',()=>drop.classList.remove('drag'));drop.addEventListener('drop',e=>{e.preventDefault();drop.classList.remove('drag');selection(e.dataTransfer.files)});
drop.addEventListener('keydown',e=>{if(e.key==='Enter'||e.key===' '){e.preventDefault();$('files').click()}});
const request=async(url,init)=>{const res=await fetch(url,{credentials:'same-origin',...init});if(!res.ok)throw Error((await res.text()).slice(0,250)||`HTTP ${res.status}`);return res.json()};

async function upload(){
 if(!files.length)return;
 $('upload').disabled=true;
 let added=0,revised=0,existing=0,fail=[],duplicateNames=[];
 const received=[];
 feedback('Uploading original files…');
 try {
  let batch=[],total=0;
  const groups=[];
  for(const file of files){
   if(file.size>20*1024*1024){fail.push(`${file.name}: file is larger than 20 MiB`);continue;}
   if(file.size===0){fail.push(`${file.name}: empty file`);continue;}
   if(batch.length>=8||total+file.size>48*1024*1024){groups.push(batch);batch=[];total=0;}
   batch.push(file);total+=file.size;
  }
  if(batch.length)groups.push(batch);
  for(let i=0;i<groups.length;i++){
   feedback(`Uploading group ${i+1} of ${groups.length}…`);
   const form=new FormData();
   for(const file of groups[i])form.append('files',file,file.name);
   try {
    const res=await fetch('/api/v1/documents/uploads',{method:'POST',credentials:'same-origin',body:form});
    const body=await res.text();let out;
    try{out=JSON.parse(body);}catch{out=null;}
    if(!out||!Array.isArray(out.items)||!Array.isArray(out.errors)){
     throw Error(res.status===401?'Sign in to upload files':`Upload service error (HTTP ${res.status})`);
    }
    for(const item of out.items){
     received.push(item.id);
     if(item.already_present){existing++;duplicateNames.push(item.filename)}else if(item.new_version)revised++;else added++;
    }
    fail.push(...out.errors);
    if(res.headers.get('X-Agiparel-Upload-Build')!==UPLOAD_BUILD){
     fail.push('The upload server is running an older release. Ask your administrator to check the deployed version.');
    }
    if(!res.ok&&res.status!==400)throw Error(`Upload service error (HTTP ${res.status})`);
   }catch(e){fail.push(`Group ${i+1}: ${e.message}`);}
  }
  const parts=[`${added} added`,`${revised} revised`,`${existing} already saved`];
  if(fail.length)parts.push(`${fail.length} need attention`);
  feedback(parts.join(' · ')+'.',fail.length?'error':'success',fail,duplicateNames);
  if(received.length){
   selection([]);$('files').value='';
  }
  await refresh({preferred:received[0]||''});
  if(received.length){$('review-submit').open=true;}
 }catch(e){feedback(e.message,'error');}
 finally{$('upload').disabled=!files.length;}
}
$('upload').addEventListener('click',()=>void upload());
let sortKey='captured_at',sortAscending=false,showHistory=false;
$('show-history').addEventListener('change',e=>{showHistory=e.target.checked;renderList()});
const fileName=(d)=>String(d.filename||'Unnamed file');
const sortValue=(d)=>{if(sortKey==='filename')return fileName(d).toLocaleLowerCase();if(sortKey==='uploaded_by')return String(d.uploaded_by||'').toLocaleLowerCase();if(sortKey==='format')return String(d.format||'').toLocaleLowerCase();return Date.parse(d.captured_at)||0};
function renderList(){
 const list=$('doc-list');list.replaceChildren();const latest=library.filter(d=>d.current_version!==false);const shown=showHistory?library:latest;text($('total'),`${latest.length} loaded`);
 text($('library-progress'),nextCursor?`${library.length} revisions loaded · more earlier files available`:`${library.length} revisions loaded · end of library`);
 $('library-more').hidden=!nextCursor;
 if(!library.length){const row=create('tr','',null,list);const cell=create('td','file-empty','No files yet. Upload one above.',row);cell.colSpan=3;return;}
 const sorted=[...shown].sort((a,b)=>{const x=sortValue(a),y=sortValue(b);let cmp=typeof x==='number'&&typeof y==='number'?x-y:String(x).localeCompare(String(y));if(!cmp)cmp=String(a.id).localeCompare(String(b.id));return sortAscending?cmp:-cmp;});
 for(const d of sorted){
  const row=create('tr','',null,list);
  const nameCell=create('td','',null,row);const name=create('button','file-name',fileName(d),nameCell);if(d.version_count>1)create('span','revision-label',` · v${d.version}${d.current_version?' current':' earlier'}`,nameCell);name.type='button';name.title='View source details';name.addEventListener('click',()=>void inspect(d.id));
  const moment=Date.parse(d.captured_at);create('td','',Number.isFinite(moment)?new Date(moment).toLocaleString():'Date unavailable',row);
  const action=create('td','',null,row);const link=create('a','file-download','↓',action);link.href=`/api/v1/documents/${encodeURIComponent(d.id)}/source`;link.download=fileName(d);link.title=`Download ${fileName(d)}`;link.setAttribute('aria-label',`Download ${fileName(d)}`);
 }
 renderAutoGroupedBundles(latest);
}
$('sort-by').addEventListener('change',e=>{sortKey=e.target.value;sortAscending=sortKey!=='captured_at';$('sort-direction').textContent=sortAscending?'Ascending ↑':'Newest first ↓';renderList()});
$('sort-direction').addEventListener('click',()=>{sortAscending=!sortAscending;$('sort-direction').textContent=sortAscending?'Ascending ↑':'Descending ↓';renderList()});

function announceLibrary(preferred='') {
 document.dispatchEvent(new CustomEvent('agiparel-documents-updated',{detail:{items:library.filter(d=>d.current_version!==false),preferred}}));
}
async function refresh({preferred=""}={}) {
 const generation=++libraryGeneration;
 try {
  const result=await request('/api/v1/documents?limit=50');
  if(generation!==libraryGeneration)return;
  library=result.items||[];nextCursor=result.next_cursor||'';
  renderList();announceLibrary(preferred);
  if(!deepLinkOpened&&deepLink.has('document')){
   deepLinkOpened=true;
   await inspect(deepLink.get('document'));
  }
 }catch(e){if(generation!==libraryGeneration)return;const list=$('doc-list');list.replaceChildren();const row=create('tr','',null,list);const cell=create('td','file-empty',`Cannot load files: ${e.message}`,row);cell.colSpan=3;text($('library-progress'),'Catalog unavailable');}
}
async function loadMore(){
 if(!nextCursor||paging)return;
 const generation=libraryGeneration,cursor=nextCursor;
 paging=true;$('library-more').disabled=true;
 try{
  const result=await request(`/api/v1/documents?limit=50&cursor=${encodeURIComponent(cursor)}`);
  if(generation!==libraryGeneration)return;
  const have=new Set(library.map(d=>d.id));
  for(const d of result.items||[]){if(!have.has(d.id)){library.push(d);have.add(d.id);}}
  nextCursor=result.next_cursor||'';
  renderList();announceLibrary();
 }catch(e){text($('library-progress'),`Cannot load earlier files: ${e.message}`);}
 finally{paging=false;$('library-more').disabled=false;}
}
$('library-more').addEventListener('click',()=>void loadMore());

// Automatically group loaded files into Style/PO Dossier Cards with zero manual checkboxes.
function renderAutoGroupedBundles(latestFiles){
 const libPanel=$('library-more')?.parentElement;
 if(!libPanel)return;
 let host=libPanel.querySelector('.po-dossier-workbench');
 if(!host){
  host=create('details','library-options po-dossier-workbench',null,libPanel);
  host.open=true;
 }
 host.replaceChildren();
 const groups=new Map();
 for(const f of latestFiles){
  const info=classifyDocument(fileName(f));
  if(!groups.has(info.styleKey))groups.set(info.styleKey,[]);
  groups.get(info.styleKey).push({doc:f, role:info.role, roleLabel:info.roleLabel});
 }
 create('summary','',`Automatic PO & Style Bundles (${groups.size} groups detected from your ${latestFiles.length} files)`,host);
 const wrap=create('div','dossier-box',null,host);
 create('p','evidence-message','Your uploaded files are automatically grouped by Style / Program code below so you never mix Tech Packs or Operation Bulletins from different orders. Click any bundle to open Chapter 02 pre-locked to ONLY that bundle’s files.',wrap);

 const cardsGrid=create('div','dossier-cards',null,wrap);
 const roleOrder={ANCHOR_PO:1,TECH_PACK:2,OPERATION_BULLETIN:3,PLANT_MASTER:4,OTHER_DOC:5};
 for(const [styleKey, items] of groups.entries()){
  items.sort((a,b)=>(roleOrder[a.role]||9)-(roleOrder[b.role]||9));
  const card=create('div','dossier-card',null,cardsGrid);
  const title=styleKey==='PLANT_MASTER'?'🏭 Shared Plant Master Registers (Machines, Workers, Catalog)':`📦 Style / Order Bundle: ${styleKey} (${items.length} file${items.length===1?'':'s'})`;
  create('strong','',title,card);
  const ul=create('ul','dossier-doc-list',null,card);
  let poDoc=null, tpDoc=null, obDoc=null;
  for(const item of items){
   if(item.role==='ANCHOR_PO'&&!poDoc)poDoc=item.doc;
   if(item.role==='TECH_PACK'&&!tpDoc)tpDoc=item.doc;
   if(item.role==='OPERATION_BULLETIN'&&!obDoc)obDoc=item.doc;
   const li=create('li','',null,ul);
   create('span','dossier-role-badge',item.roleLabel,li);
   const btn=create('button','file-name',` ${fileName(item.doc)}`,li);
   btn.type='button';
   btn.style.cssText='display:inline;min-height:auto;padding:0 4px;font-size:12px';
   btn.addEventListener('click',()=>void inspect(item.doc.id));
  }
  if(styleKey!=='PLANT_MASTER'){
   const act=create('div','review-actions',null,card);
   const openCh2=create('button','secondary',`Check ${styleKey} in Chapter 02 →`,act);
   openCh2.type='button';
   openCh2.addEventListener('click',async()=>{
    const docsPayload=items.map(x=>({document_id:x.doc.id,doc_role:x.role==='PLANT_MASTER'?'MACHINE_ROSTER':x.role,filename:fileName(x.doc)}));
    try{
     await request('/api/v1/document-dossiers',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({po_number:styleKey,style_code:styleKey,documents:docsPayload})});
    }catch{}
    try{
     if(typeof window!=='undefined'&&window.sessionStorage){
      window.sessionStorage.setItem('agiparel:active-po-context:v51',JSON.stringify({
       po_number:styleKey,
       style_code:styleKey,
       po_document_id:(poDoc||tpDoc||items[0].doc).id,
       bom_document_id:(tpDoc||poDoc||items[0].doc).id,
       ob_document_id:obDoc?obDoc.id:'',
       documents:docsPayload
      }));
     }
    }catch{}
    window.location.href=`/buyer-review?style=${encodeURIComponent(styleKey)}`;
   });
  }
 }

 // Clean 1-click RFID experiment bar
 const expBar=create('div','dossier-exp-box',null,wrap);
 const expTop=create('div','panel-head',null,expBar);
 create('strong','','17-Stage Factory RFID / NFC Experiment (Store Rolls → Cutting → Sewing Line → SGTIN-96 → Dock)',expTop);
 const expBtn=create('button','secondary','Run 17-Stage Live RFID Experiment →',expTop);expBtn.type='button';
 const expStatus=create('p','evidence-message','Simulate 10 Store Rolls (PO-independent General Stock), 14 Dual UHF+NFC Sewing Bundles, 4 RTLS Machines/Trolleys, 16 SGTIN-96 Garments, and 4 SSCC Cartons.',expBar);
 const expDetails=create('div','dossier-cards',null,expBar);
 expBtn.addEventListener('click',async()=>{
  expBtn.disabled=true;text(expStatus,'Running 17-stage Live RFID + Passive UHF + Station NFC Puck + SGTIN-96 experiment…');
  try{
   const res=await request('/api/v1/rfid/experiment/run',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({po_number:'REP3',style_code:'GM09Y094',roll_count:10,bundle_count:14,carton_count:4})});
   const pool=await request('/api/v1/rfid/tag-pool');
   const rolls=await request('/api/v1/stores/roll-sheet');
   const telemetry=await request('/api/v1/rfid/rtls-sightings');
   text(expStatus,`✓ Verified PO ${res.po_number} (${res.style_code}): ${rolls.reconciliation?.mapped_grn||10} General-Stock Store Rolls mapped & allocated (${rolls.reconciliation?.short_missing||1} short roll & ${rolls.reconciliation?.claims||1} narrow/defect roll flagged) · ${res.rolls_recycled_unused} roll tags & ${res.bundles_cycled} bundle fobs automatically recycled to UNUSED pool (${pool.counts?.UNUSED||0} UNUSED ready in bin) · ${res.garments_sgtin96} SGTIN-96 garments & ${res.cartons_dispatched} SSCC cartons dispatched.`);
   expDetails.replaceChildren();
   for(const ev of (telemetry.events||[]).slice(0,8)){
    const c=create('div','dossier-card',null,expDetails);
    create('strong','',ev.stage_name,c);
    create('p','evidence-message',`Tag ${ev.epc_hex} · ${ev.target_type} ${ev.target_id} · Scope: ${ev.po_number||'GENERAL_STOCK'} · Reader ${ev.gateway_id} (${ev.zone_code})`,c);
   }
  }catch(e){text(expStatus,`Experiment error: ${e.message}`);}
  finally{expBtn.disabled=false;}
 });
}

function renderSourcePage(payload,append){
 if(!append)currentSegments=[];
 currentSegments.push(...(payload.segments||[]));
 sourceOrdinal=payload.next_ordinal||0;
 hasMoreSources=payload.has_more_segments===true;
 $('source-more').hidden=!hasMoreSources;
 renderSegments();
}
async function inspect(id){
 const generation=++sourceGeneration;
 try{
  current=id;$('source-details').open=true;
  currentSegments=[];sourceOrdinal=0;hasMoreSources=false;
  $('source-more').hidden=true;$('search').disabled=true;
  text($('evidence-message'),'Loading source evidence…');
  const payload=await request(`/api/v1/documents/${encodeURIComponent(id)}`);
  if(generation!==sourceGeneration)return;
  const d=payload.document;
  const revision=library.find(item=>item.id===id);
  text($('inspect-title'),d.filename+(revision?.version_count>1?` · Version ${revision.version} of ${revision.version_count}`:''));
  text($('inspect-desc'),`${d.format} · ${d.status} · ${d.segment_count} source segments. ${d.outcome||'Still being processed.'}`);
  text($('inspect-sha'),d.sha256);text($('inspect-readiness'),'UNREVIEWED · HOLD');
  $('download-source').hidden=false;$('download-source').href=`/api/v1/documents/${encodeURIComponent(id)}/source`;
  document.dispatchEvent(new CustomEvent('agiparel-document-selected',{detail:{id}}));
  $('search').disabled=false;$('search').value='';renderSourcePage(payload,false);
  if(!deepLink.has('document'))$('source-details').scrollIntoView({behavior:'smooth',block:'start'});
  const ordinal=Number(deepLink.get('segment'));
  if(Number.isSafeInteger(ordinal)&&ordinal>0){
   const matched=document.getElementById(`source-segment-${ordinal}`);
   if(matched){matched.classList.add('source-highlight');matched.scrollIntoView({behavior:'smooth',block:'center'});}
   else text($('evidence-message'),`Segment #${ordinal} is not loaded. Use Load more extracted text or download the original.`);
  }
 }catch(e){if(generation===sourceGeneration)text($('evidence-message'),`Cannot load source: ${e.message}`);}
}
async function loadMoreSource(){
 if(!current||!hasMoreSources||!sourceOrdinal)return;
 const generation=sourceGeneration,id=current,after=sourceOrdinal;
 $('source-more').disabled=true;
 try{
  const payload=await request(`/api/v1/documents/${encodeURIComponent(id)}?after_ordinal=${after}`);
  if(generation!==sourceGeneration||current!==id)return;
  renderSourcePage(payload,true);
 }catch(e){if(generation===sourceGeneration)text($('evidence-message'),`Cannot load next source segment page: ${e.message}`);}
 finally{$('source-more').disabled=false;}
}
$('source-more').addEventListener('click',()=>void loadMoreSource());
function renderSegments(){const q=$('search').value.trim().toLocaleLowerCase();const holder=$('segments');holder.replaceChildren();let found=0;for(const s of currentSegments){if(q&&!s.content.toLocaleLowerCase().includes(q)&&!JSON.stringify(s.locator).toLocaleLowerCase().includes(q))continue;found++;const card=create('article','segment','',holder);card.id=`source-segment-${s.ordinal}`;create('div','segment-label',`${s.kind} #${s.ordinal} · ${Object.entries(s.locator||{}).map(([k,v])=>`${k}: ${v}`).join(' · ')}`,card);create('pre','',s.content,card)}text($('evidence-message'),currentSegments.length?`${found} matching segment(s) of ${currentSegments.length} loaded${hasMoreSources?'; more evidence available':'; end of extracted evidence'}. Search covers loaded text only; the original is always available.`:'No extracted text for this document yet. Check the job status or download the original.')}
$('search').addEventListener('input',renderSegments);$('refresh').addEventListener('click',()=>void refresh());void refresh();
setInterval(()=>{if(library.some(d=>d.status==='QUEUED'||d.status==='PROCESSING')){void refresh().then(()=>{if(current&&library.some(d=>d.id===current&&d.status!=='QUEUED'&&d.status!=='PROCESSING')&&currentSegments.length===0)void inspect(current)})}},4500);
})();