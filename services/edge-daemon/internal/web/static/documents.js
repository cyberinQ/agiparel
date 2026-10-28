// Author: Sunil Jain, 2026-10-27 — original-file intake and sortable evidence library.
(()=>{'use strict';
const $=id=>document.getElementById(id),text=(node,value)=>{node.textContent=value??'';return node};
const create=(tag,cls,contents,parent)=>{const n=document.createElement(tag);if(cls)n.className=cls;text(n,contents);if(parent)parent.append(n);return n};
let files=[],library=[],current=null,currentSegments=[];
const deepLink=new URLSearchParams(window.location.search);let deepLinkOpened=false;
const formatSize=n=>n>=1024*1024?`${(n/1024/1024).toFixed(1)} MiB`:`${(n/1024).toFixed(1)} KiB`;
function feedback(message,kind=''){$('feedback').className=`feedback ${kind}`;text($('feedback'),message)}
function selection(next){files=Array.from(next||[]);text($('selection'),files.length?files.map(f=>`${f.name} (${formatSize(f.size)})`).join('\n'):'No files selected');$('upload').disabled=!files.length}
$('files').addEventListener('change',e=>selection(e.target.files));
const drop=$('drop');drop.addEventListener('dragover',e=>{e.preventDefault();drop.classList.add('drag')});drop.addEventListener('dragleave',()=>drop.classList.remove('drag'));drop.addEventListener('drop',e=>{e.preventDefault();drop.classList.remove('drag');selection(e.dataTransfer.files)});
drop.addEventListener('keydown',e=>{if(e.key==='Enter'||e.key===' '){e.preventDefault();$('files').click()}});
const request=async(url,init)=>{const res=await fetch(url,{credentials:'same-origin',...init});if(!res.ok)throw Error((await res.text()).slice(0,250)||`HTTP ${res.status}`);return res.json()};
async function upload(){if(!files.length)return;$('upload').disabled=true;let saved=0,fail=[];feedback('Uploading original files…');
 try{let batch=[],total=0;const groups=[];for(const file of files){if(file.size>20*1024*1024){fail.push(`${file.name}: exceeds 20 MiB`);continue}if(file.size===0){fail.push(`${file.name}: empty`);continue}if(batch.length>=8||total+file.size>48*1024*1024){groups.push(batch);batch=[];total=0}batch.push(file);total+=file.size}if(batch.length)groups.push(batch);
 for(let i=0;i<groups.length;i++){feedback(`Uploading group ${i+1} of ${groups.length}… (${saved} saved)`);let form=new FormData();for(const file of groups[i])form.append('files',file,file.name);try{const out=await request('/api/v1/documents/uploads',{method:'POST',body:form});saved+=out.items.length;fail.push(...out.errors)}catch(e){fail.push(`Group ${i+1}: ${e.message}`)}}
 feedback(`${saved} file(s) stored. ${fail.length?`Issues: ${fail.join('; ')}`:'Ready for review.'}`,fail.length?'error':'success');if(saved&&!fail.length){selection([]);$('files').value='';}await refresh();
 }catch(e){feedback(e.message,'error')}finally{$('upload').disabled=false}}
$('upload').addEventListener('click',()=>void upload());
let sortKey='captured_at',sortAscending=false;
const fileName=(d)=>String(d.filename||'Unnamed file');
const sortValue=(d)=>{if(sortKey==='filename')return fileName(d).toLocaleLowerCase();if(sortKey==='uploaded_by')return String(d.uploaded_by||'').toLocaleLowerCase();if(sortKey==='format')return String(d.format||'').toLocaleLowerCase();return Date.parse(d.captured_at)||0};
function renderList(){
 const list=$('doc-list');list.replaceChildren();text($('total'),library.length);
 if(!library.length){const row=create('tr','',null,list);const cell=create('td','file-empty','No files yet. Upload one above.',row);cell.colSpan=5;return;}
 const sorted=[...library].sort((a,b)=>{const x=sortValue(a),y=sortValue(b);let cmp=typeof x==='number'&&typeof y==='number'?x-y:String(x).localeCompare(String(y));if(!cmp)cmp=String(a.id).localeCompare(String(b.id));return sortAscending?cmp:-cmp;});
 for(const d of sorted){
  const row=create('tr','',null,list);
  const nameCell=create('td','',null,row);const name=create('button','file-name',fileName(d),nameCell);name.type='button';name.title='View source details';name.addEventListener('click',()=>void inspect(d.id));
  create('td','',d.uploaded_by||'Not recorded',row);
  const moment=Date.parse(d.captured_at);create('td','',Number.isFinite(moment)?new Date(moment).toLocaleString():'Date unavailable',row);
  create('td','',d.format||'Unknown',row);
  const action=create('td','',null,row);const link=create('a','file-download','↓',action);link.href=`/api/v1/documents/${encodeURIComponent(d.id)}/source`;link.download=fileName(d);link.title=`Download ${fileName(d)}`;link.setAttribute('aria-label',`Download ${fileName(d)}`);
 }
}
$('sort-by').addEventListener('change',e=>{sortKey=e.target.value;sortAscending=sortKey!=='captured_at';$('sort-direction').textContent=sortAscending?'Ascending ↑':'Newest first ↓';renderList()});
$('sort-direction').addEventListener('click',()=>{sortAscending=!sortAscending;$('sort-direction').textContent=sortAscending?'Ascending ↑':'Descending ↓';renderList()});
async function refresh(){try{const result=await request('/api/v1/documents');library=result.items||[];renderList();if(!deepLinkOpened&&deepLink.has('document')){deepLinkOpened=true;const id=deepLink.get('document');if(library.some(d=>d.id===id))await inspect(id);}}catch(e){const list=$('doc-list');list.replaceChildren();const row=create('tr','',null,list);const cell=create('td','file-empty',`Cannot load files: ${e.message}`,row);cell.colSpan=5;}}
async function inspect(id){try{current=id;$('source-details').open=true;renderList();const payload=await request(`/api/v1/documents/${encodeURIComponent(id)}`),d=payload.document;text($('inspect-title'),d.filename);text($('inspect-desc'),`${d.format} · ${d.status} · ${d.segment_count} source segments. ${d.outcome||'Still being processed.'}`);text($('inspect-sha'),d.sha256);text($('inspect-readiness'),'UNREVIEWED · HOLD');$('download-source').hidden=false;$('download-source').href=`/api/v1/documents/${encodeURIComponent(id)}/source`;currentSegments=payload.segments||[];document.dispatchEvent(new CustomEvent('agiparel-document-selected',{detail:{id}}));$('search').disabled=false;$('search').value='';renderSegments();if(!deepLink.has('document'))$('source-details').scrollIntoView({behavior:'smooth',block:'start'});const ordinal=Number(deepLink.get('segment'));if(Number.isSafeInteger(ordinal)&&ordinal>0){const matched=document.getElementById(`source-segment-${ordinal}`);if(matched){matched.classList.add('source-highlight');matched.scrollIntoView({behavior:'smooth',block:'center'});}else{feedback(`The requested segment #${ordinal} is outside the first 200 shown here. Download the source to inspect it.`);}}}
 catch(e){text($('evidence-message'),`Cannot load source: ${e.message}`)}}
function renderSegments(){const q=$('search').value.trim().toLocaleLowerCase();const holder=$('segments');holder.replaceChildren();let found=0;for(const s of currentSegments){if(q&&!s.content.toLocaleLowerCase().includes(q)&&!JSON.stringify(s.locator).toLocaleLowerCase().includes(q))continue;found++;const card=create('article','segment','',holder);card.id=`source-segment-${s.ordinal}`;create('div','segment-label',`${s.kind} #${s.ordinal} · ${Object.entries(s.locator||{}).map(([k,v])=>`${k}: ${v}`).join(' · ')}`,card);create('pre','',s.content,card)}text($('evidence-message'),currentSegments.length?`${found} matching segment(s) of ${currentSegments.length} loaded. This view shows up to 200 source segments; the original is always available.`:'No extracted text for this document yet. Check the job status or download the original.')}
$('search').addEventListener('input',renderSegments);$('refresh').addEventListener('click',()=>void refresh());void refresh();
// Only poll when queued/processing; avoid unnecessary requests once settled.
setInterval(()=>{if(library.some(d=>d.status==='QUEUED'||d.status==='PROCESSING')){void refresh().then(()=>{if(current&&library.some(d=>d.id===current&&d.status!=='QUEUED'&&d.status!=='PROCESSING')&&currentSegments.length===0)void inspect(current)})}},4500);
})();
