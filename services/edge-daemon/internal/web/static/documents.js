// Author: Sunil Jain, 2026-10-28 — original-file intake and compact, sortable evidence library.
(()=>{'use strict';
const $=id=>document.getElementById(id),text=(node,value)=>{node.textContent=value??'';return node};
const create=(tag,cls,contents,parent)=>{const n=document.createElement(tag);if(cls)n.className=cls;text(n,contents);if(parent)parent.append(n);return n};
const UPLOAD_BUILD = "20261027.v9";
let files=[],library=[],current=null,currentSegments=[];
let nextCursor="", sourceOrdinal=0, hasMoreSources=false, libraryGeneration=0, sourceGeneration=0, paging=false;
const deepLink=new URLSearchParams(window.location.search);let deepLinkOpened=false;
const formatSize=n=>n>=1024*1024?`${(n/1024/1024).toFixed(1)} MiB`:`${(n/1024).toFixed(1)} KiB`;
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
// Author: Sunil Jain, 2026-10-27 — Distinguish new, already-stored, and rejected originals.
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
}
$('sort-by').addEventListener('change',e=>{sortKey=e.target.value;sortAscending=sortKey!=='captured_at';$('sort-direction').textContent=sortAscending?'Ascending ↑':'Newest first ↓';renderList()});
$('sort-direction').addEventListener('click',()=>{sortAscending=!sortAscending;$('sort-direction').textContent=sortAscending?'Ascending ↑':'Descending ↓';renderList()});
// Every request is scoped to an operator-visible, bounded page. Never imply a partial
// catalog represents all files in the factory.
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
   // Direct evidence links are not restricted to the first catalog page.
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
   // An original's segment may be far beyond the first 200; do not pretend to display it.
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
// Only poll when queued/processing; avoid unnecessary requests once settled.
setInterval(()=>{if(library.some(d=>d.status==='QUEUED'||d.status==='PROCESSING')){void refresh().then(()=>{if(current&&library.some(d=>d.id===current&&d.status!=='QUEUED'&&d.status!=='PROCESSING')&&currentSegments.length===0)void inspect(current)})}},4500);
})();
