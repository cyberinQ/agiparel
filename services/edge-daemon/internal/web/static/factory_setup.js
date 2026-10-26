// Factory-first onboarding. No extracted document becomes an approved business fact.
(() => {
 'use strict';
 const el = id => document.getElementById(id);
 let model = null, selected = null;
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
  el('next-question').textContent=model.next_question;
  renderReadiness();renderTopics();renderDocuments();
 }
 function renderReadiness() {
  const parent=el('readiness');clear(parent);
  for(const r of model.readiness) {
   const style = r.status==='AVAILABLE'||r.status==='FOUNDATION_PRESENT'?'ok':r.status==='NOT_AUTHORIZED'?'blocked':'review';
   const c=node('article',`readiness-card ${style}`);
   c.append(node('span','status',r.status.replaceAll('_',' ')),node('strong','',r.label),desc(r.explanation));
   if(r.missing.length){const list=node('ul');for(const missing of r.missing){list.append(node('li','',missing));}c.append(list);}
   parent.append(c);
  }
 }
 function renderTopics() {
  const host=el('topics');clear(host);
  for(const name of topics){
   const fields=model.fields.filter(f=>f.group===name);
   const done=fields.filter(f=>model.facts[f.key]?.state==='CONFIRMED').length;
   const panel=node('section','topic');const header=node('div','topic-header');header.append(node('h3','',name),node('span','',`${done}/${fields.length} confirmed`));panel.append(header);
   for(const spec of fields){
    const f=model.facts[spec.key];const row=node('div','fact-row');const left=node('div');
    left.append(node('div','label',spec.label),node('span','value',f ? f.value+(spec.unit?` ${spec.unit}`:'') : 'Not added yet'));
    if(f){left.append(node('span',`state ${f.state==='PROVISIONAL'?'provisional':''}`,stateText[f.state]||f.state));}
    else{left.append(node('span','hint','You can add this later.'));}
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
 el('answer-next').addEventListener('click',()=>{
  if(!model)return;
  const priorities=['factory.name','machines.sewing_count','workforce.headcount','machines.capabilities','workforce.skills','materials.categories','calendar.shift_hours','costs.currency','costs.labor_rate','costs.overhead_rate'];
  const key=priorities.find(k=>model.facts[k]?.state!=='CONFIRMED')||'factory.name';
  openEdit(model.fields.find(f=>f.key===key));
 });
 el('close-dialog').addEventListener('click',closeDialog);
 el('cancel-dialog').addEventListener('click',closeDialog);
 el('fact-form').addEventListener('submit',save);
 document.addEventListener('agiparel-factory-evidence-updated',refresh);
 refresh();
})();
