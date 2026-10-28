// Factory-first onboarding. No extracted document becomes an approved business fact.
(() => {
 'use strict';
 const el = id => document.getElementById(id);
 let model = null, selected = null;
 const skippedQuestions = new Set();
 let activeQuestion = null;
 const topics = ['Factory','Machines','People & skills','Materials & stores','Costs & rates','Calendar','Quality'];
 const stateText = {CONFIRMED:'You confirmed',PROVISIONAL:'Estimate · check later'};
 function node(tag, className, text) {
  const x = document.createElement(tag);
  if (className) x.className = className;
  if (text !== undefined) x.textContent = text;
  return x;
 }
 function clear(parent) { parent.replaceChildren(); }
 function button(text,handler) {
  const b=node('button','',text);b.type='button';b.addEventListener('click',handler);return b;
 }
 function desc(text) {return node('p','',text);}
 async function apiJSON(path, options) {
  const res=await fetch(path,Object.assign({credentials:'same-origin',headers:{Accept:'application/json'}},options || {}));
  if(!res.ok) { const error = await res.text(); throw new Error(error.slice(0,200) || `HTTP ${res.status}`); }
  return res.json();
 }
 async function refresh() {
  try {
   model=await apiJSON('/api/v1/factory-onboarding');
   render();
  } catch(err) {
   el('next-question').textContent = 'We could not open your factory records yet. Verify migration 0010 and your development tenant.';
   el('readiness').textContent = `Cannot load factory readiness: ${err.message}`;
   el('documents').textContent = 'Document suggestions are temporarily unavailable.';
  }
 }
 function render() {
  const known = model.known_fields || 0, total = model.total_fields || 1;
  el('progress-value').textContent = `${known} / ${total}`;
  el('progress-bar').style.width = `${100*known/total}%`;
  el('progress-description').textContent = `${model.confirmed_fields} checked · ${known-model.confirmed_fields} provisional. You can keep adding information anytime.`;
  el('fact-count').textContent = `${model.confirmed_fields} confirmed · ${known-model.confirmed_fields} provisional`;
  renderQuestion();renderReadiness();renderTopics();renderDocuments();
 }
 function renderQuestion() {
  const questions = model.next_questions || [];
  activeQuestion = questions.find(q => !skippedQuestions.has(q.field_key)) || null;
  const answer = el('answer-next'), skip = el('skip-next');
  if (activeQuestion) {
   el('next-question').textContent = activeQuestion.question;
   answer.textContent = model.facts[activeQuestion.field_key] ? 'Check answer' : 'Add answer';
   answer.disabled = false;
   skip.textContent = 'Skip for now';
   skip.disabled = false;
  } else if (questions.length) {
   el('next-question').textContent = 'You skipped the outstanding questions for this visit. Your answers were not changed.';
   answer.disabled = true;
   skip.textContent = 'Review skipped questions';
   skip.disabled = false;
  } else {
   el('next-question').textContent = model.next_question;
   answer.disabled = true;
   skip.textContent = 'All basics checked';
   skip.disabled = true;
  }
 }
 function renderReadiness() {
  const parent=el('readiness');clear(parent);
  const areas = new Set(['capacity_baseline','costing_baseline','material_baseline']);
  for(const r of model.readiness.filter(item=>areas.has(item.key))) {
   const style = r.status==='FOUNDATION_PRESENT'?'ok':'review';
   const c=node('article',`readiness-card ${style}`);
   c.append(node('span','status',r.status==='FOUNDATION_PRESENT'?'BASICS CHECKED':'NEEDS EVIDENCE'),node('strong','',r.label));
   if(r.missing.length){
    c.append(desc('Check next:'));
    const list=node('ul');for(const missing of r.missing){list.append(node('li','',missing));}c.append(list);
   } else {c.append(desc('Inputs checked for an initial estimate. Not an operational approval.'));}
   c.append(node('p','readiness-explanation',r.explanation));
   parent.append(c);
  }
 }
 function renderTopics() {
  const host=el('topics');clear(host);
  for(const name of topics){
   const fields=model.fields.filter(f=>f.group===name);
   const done=fields.filter(f=>model.facts[f.key]?.state==='CONFIRMED').length;
   const panel=node('details','topic');const header=node('summary','topic-header');header.append(node('strong','',name),node('span','',`${done}/${fields.length} checked`));panel.append(header);
   for(const spec of fields){
    const f=model.facts[spec.key];const row=node('div','fact-row');const left=node('div');
    left.append(node('div','label',spec.label),node('span','value',f ? f.value+(spec.unit?` ${spec.unit}`:'') : 'Not added yet'));
    if(f){
     left.append(node('span',`state ${f.state==='PROVISIONAL'?'provisional':''}`,stateText[f.state]||f.state));
     const source = model.documents.find(d=>d.id===f.source_document_id);
     left.append(node('small','fact-source',`Revision ${f.revision} · ${source?`Source: ${source.filename}`:(f.source_document_id?'Source attached (outside recent file list)':'No document attached')}`));
    } else {left.append(node('span','hint','You can add this later.'));}
    row.append(left,button(f?'Update':'Add',()=>openEdit(spec)));panel.append(row);
   }
   host.append(panel);
  }
 }
 function renderDocuments() {
  const host=el('documents');clear(host);
  if(!model.documents.length){host.append(node('p','empty','No documents uploaded yet. Use the single Document Intelligence inbox when you are ready.'));return;}
  for(const d of model.documents){
   const item=node('article','doc-row');const body=node('div','doc-body');
   body.append(node('strong','',d.filename),node('small','',`${d.status} · ${d.reason}`));
   const link=node('a','', 'Review →');link.href=`/documents`;
   item.append(body,node('span','doc-pill',d.suggested_area),link);host.append(item);
  }
 }
 function openEdit(spec){
  selected=spec;const old=model.facts[spec.key]||{};
  el('dialog-title').textContent=spec.label;
  el('dialog-help').textContent=spec.help+(spec.unit?` Unit: ${spec.unit}.`:'');
  const host=el('value-host');clear(host);
  let input;
  if(spec.kind==='CHOICE'){
   input=node('select');for(const [value,label] of [['UNKNOWN','I do not know'],['ESTIMATED','Estimated quantity'],['COUNTED','Physically counted'],['INSPECTED','Counted and inspected']]){const opt=node('option','',label);opt.value=value;input.append(opt)}input.value=old.value||'UNKNOWN';
  } else if(spec.kind==='TEXT' && (spec.key.endsWith('skills')||spec.key.endsWith('capabilities')||spec.key.endsWith('categories')||spec.key.endsWith('maintenance')||spec.key.endsWith('product_types')||spec.key.endsWith('inspection_policy'))){
   input=node('textarea');input.rows=3;input.value=old.value||'';
  } else {
   input=node('input');input.type=spec.kind==='INTEGER'?'number':spec.kind==='DECIMAL'?'text':'text';
   input.value=old.value||'';input.autocomplete='off';
   if(spec.kind==='INTEGER'){input.min='0';input.step='1';}if(spec.kind==='DECIMAL'){input.inputMode='decimal';}
  }
  input.id='fact-value';input.name='fact-value';input.required=true;input.maxLength=2000;host.append(input);
  el('fact-state').value=old.state||'PROVISIONAL';
  const sources=el('fact-source');sources.replaceChildren();const none=node('option','', 'No source yet / personal observation');none.value='';sources.append(none);
  for(const d of model.documents){const opt=node('option','',d.filename);opt.value=d.id;sources.append(opt);}
  // The discovery feed shows only the newest 40 files; never erase a fact's
  // older source link just because it is not in that recent-file dropdown.
  if (old.source_document_id && !model.documents.some(d=>d.id===old.source_document_id)) {
   const retained=node('option','','Previously attached document (outside recent list)');
   retained.value=old.source_document_id;sources.append(retained);
  }
  sources.value=old.source_document_id||'';
  el('fact-note').value=old.note||'';
  el('dialog-error').hidden=true;
  el('edit-dialog').showModal();
  input.focus();
 }
 function closeDialog(){el('edit-dialog').close();}
 async function save(event){
  event.preventDefault();if(!selected)return;
  const old=model.facts[selected.key]||{};
  const body={field_key:selected.key,value:el('fact-value').value,
   state:el('fact-state').value,source_document_id:el('fact-source').value,
   note:el('fact-note').value,expected_revision:old.revision||0};
  const submit=el('save-fact');submit.disabled=true;
  try{
   await apiJSON('/api/v1/factory-onboarding/facts',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(body)});
   closeDialog();await refresh();
  }catch(err){el('dialog-error').textContent=`We couldn't save that yet: ${err.message}. Check the value or refresh if someone else updated it.`;el('dialog-error').hidden=false;}
  finally{submit.disabled=false;}
 }
 const approvals=el('approval-dialog');
 // Review & Submit links to /documents#review-submit; not a dialog button.
 // The earlier evidence, conflict and reference tools remain available on demand.
 if(new URLSearchParams(window.location.search).get('advanced-review')==='1'){
  document.addEventListener('DOMContentLoaded',()=>{if(!approvals.open)approvals.showModal();},{once:true});
 }
 el('approval-close').addEventListener('click',()=>approvals.close());
 // Show the real outstanding count, including saved reviews and submitted files.
 const badge=el('approval-count');
 async function updateBadge(){
  try {
   const response=await apiJSON('/api/v1/factory-onboarding/approvals/pending');
   const count=response.pending_records;
   badge.textContent=String(count);
   badge.hidden=count===0;
   badge.setAttribute('aria-label',`${count} records awaiting review`);
  } catch {badge.hidden=true;}
 }
 document.addEventListener('visibilitychange',()=>{if(!document.hidden)void updateBadge();});
 void updateBadge();
 el('answer-next').addEventListener('click',()=>{
  if (!activeQuestion || !model) return;
  const spec = model.fields.find(f=>f.key===activeQuestion.field_key);
  if (spec) openEdit(spec);
 });
 el('skip-next').addEventListener('click',()=>{
  if (activeQuestion) skippedQuestions.add(activeQuestion.field_key);
  else skippedQuestions.clear();
  if (model) renderQuestion();
 });
 el('close-dialog').addEventListener('click',closeDialog);
 el('cancel-dialog').addEventListener('click',closeDialog);
 el('fact-form').addEventListener('submit',save);
 document.addEventListener('agiparel-factory-evidence-updated',refresh);
 refresh();
})();
