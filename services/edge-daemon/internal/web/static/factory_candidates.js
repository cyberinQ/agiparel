// Factory evidence triage, Gate 06.5-B2. No direct writes to operational tables.
(() => {
 'use strict';
 const el = id => document.getElementById(id);
 const create = (tag, cls, value) => {const e=document.createElement(tag);if(cls)e.className=cls;if(value!==undefined)e.textContent=value;return e;};
 const http = async (url, options = {}) => {
  const res=await fetch(url,{credentials:'same-origin',headers:{Accept:'application/json',...(options.method?{'Content-Type':'application/json'}:{})},...options});
  if(!res.ok){let err=`HTTP ${res.status}`;try {const content=await res.json();err=content.error||err;}catch{}throw new Error(err);}
  return res.json();
 };
 const notice = (message, bad=false) => {el('candidate-feedback').textContent=message;el('candidate-feedback').classList.toggle('bad',bad);};
 const fmtLocator = c => {
  const x=c.source_locator||c.locator||{};
  if(x.sheet)return `Sheet ${x.sheet}, row ${x.row||'?'}`;
  if(x.row)return `Row ${x.row}`;
  if(x.page)return `Page ${x.page}`;
  if(x.paragraph)return `Paragraph ${x.paragraph}`;
  if(x.line)return `Line ${x.line}`;
  return `Segment ${c.source_ordinal||'?'}`;
 };
 const sourceLink = (c) => {
  const a=create('a','candidate-source-link',`${c.filename||'Original file'} · ${fmtLocator(c)} ↗`);
  a.href=`/documents?document=${encodeURIComponent(c.document_id)}&segment=${encodeURIComponent(c.source_ordinal||0)}`; a.title=`Review document ${c.document_id}, segment ${c.source_ordinal||'unknown'}`;
  return a;
 };
 const empty = (target,msg) => {target.replaceChildren(create('p','empty',msg));};
 const renderCandidate = c => {
  const article=create('article','candidate-card');
  const label=create('div','candidate-type',c.entity_label || c.entity_type);
  const status=create('span','candidate-state',c.status.replaceAll('_',' '));
  const top=create('div','candidate-card-top');top.append(label,status);article.append(top);
  article.append(create('strong','candidate-value',c.value),create('p','candidate-meaning',c.explanation),sourceLink(c));
  const props=create('dl','candidate-props');
  for(const [name,val] of Object.entries(c.attributes||{})){
   const dt=create('dt','',name.replaceAll('_',' ')),dd=create('dd','',String(val));
   props.append(dt,dd);
  }
  article.append(props);
  if(c.status==='PENDING'){
   const actions=create('div','candidate-actions');
   const accept=create('button','btn primary','Keep as provisional');accept.type='button';
   const reject=create('button','btn secondary','Not relevant');reject.type='button';
   accept.addEventListener('click',()=>review(c,'ACCEPT_PROVISIONAL',accept,reject));
   reject.addEventListener('click',()=>review(c,'REJECT',accept,reject));
   actions.append(accept,reject);article.append(actions);
  }
  return article;
 };
 async function loadDocuments(){
  const select=el('candidate-source');
  try{
   const data=await http('/api/v1/documents');select.replaceChildren();
   for(const d of data.items||[]){
    const o=create('option','',`${d.filename} · ${d.status}`);o.value=d.id;
    if(!['EXTRACTED','PARTIAL'].includes(d.status))o.disabled=true;
    select.append(o);
   }
   if(!select.options.length){const opt=create('option','', 'No uploaded documents yet');opt.value='';select.append(opt);}
  }catch(e){notice(`Could not load documents: ${e.message}`,true);}
 }
 async function refresh(){
  try {
   const [candidates,register]=await Promise.all([http('/api/v1/factory-onboarding/candidates'),http('/api/v1/factory-onboarding/proposed-register')]);
   const waiting=(candidates.items||[]).filter(c=>c.status==='PENDING');
   const reviewed=(candidates.items||[]).filter(c=>c.status!=='PENDING');
   el('candidate-count').textContent=`${waiting.length} to check · ${reviewed.length} reviewed`;
   const host=el('candidate-items');host.replaceChildren();
   if(!waiting.length)empty(host,'No pending suggestions. Choose a factory document above to scan, or upload a file in the Document Inbox.');
   else for(const c of waiting)host.append(renderCandidate(c));
   const rh=el('register-items');rh.replaceChildren();
   const items=register.items||[];el('register-count').textContent=`${items.length} proposed`;
   if(!items.length)empty(rh,'No proposed machine, skill, stock or rate rows yet.');
   else for(const p of items){
    const card=create('article','register-card');card.append(create('span','candidate-type',p.type),create('strong','candidate-value',p.entity_key));
    for(const [key,val] of Object.entries(p.attributes||{}))card.append(create('p','register-attribute',`${key.replaceAll('_',' ')}: ${val}`));
    card.append(sourceLink(p));rh.append(card);
   }
  }catch(e){notice(`Cannot read factory suggestions: ${e.message}. Check migration 0011.`,true);}
 }
 async function scan(){
  const id=el('candidate-source').value;
  if(!id){notice('Upload a factory document first, then return to scan it.',true);return;}
  const button=el('scan-factory-doc');button.disabled=true;notice('Finding explicitly labelled factory information in this document…');
  try{
   const result=await http('/api/v1/factory-onboarding/candidates/scan',{method:'POST',body:JSON.stringify({document_id:id})});
   notice(result.matched?`Found ${result.matched} possible records (${result.new_candidates} new). Review each suggestion before keeping it.`:'No clearly labelled machine, skill, inventory or rate rows found. Nothing was guessed. You can still enter information manually in Factory Setup.');
   await refresh();
  }catch(e){notice(`Could not examine this document: ${e.message}`,true);}
  finally{button.disabled=false;}
 }
 async function review(candidate, decision, accept, reject){
  accept.disabled=true;reject.disabled=true;
  try {
   const answer=await http('/api/v1/factory-onboarding/candidates/review',{method:'POST',body:JSON.stringify({candidate_id:candidate.id,decision,note:''})});
   notice(decision==='REJECT'?'Suggestion set aside.':'Saved as provisional evidence. Verify it before using it for business decisions.');
   await refresh();
   document.dispatchEvent(new Event('agiparel-factory-evidence-updated'));
   if(candidate.entity_type==='FIELD')document.dispatchEvent(new Event('agiparel-factory-evidence-updated'));
  }catch(e){notice(`Could not record your decision: ${e.message}`,true);accept.disabled=false;reject.disabled=false;}
 }
 el('scan-factory-doc').addEventListener('click',scan);
 el('refresh-factory-candidates').addEventListener('click',refresh);
 Promise.all([loadDocuments(),refresh()]);
})();
