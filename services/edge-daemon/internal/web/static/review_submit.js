// Author: Sunil Jain, 2026-10-27
// One-record-at-a-time field corrections, explicit review, and final audited submission.
(() => {
  'use strict';
  const $ = id => document.getElementById(id);
  if (!$('review-submit')) return;
  const node = (tag, cls, value, parent) => {
    const el = document.createElement(tag);
    if (cls) el.className = cls;
    if (value !== undefined) el.textContent = String(value);
    if (parent) parent.append(el);
    return el;
  };
  const api = async (url, init = {}) => {
    const response = await fetch(url, {
      credentials: 'same-origin',
      headers: { Accept: 'application/json', ...(init.body ? {'Content-Type':'application/json'} : {}) },
      ...init
    });
    if (!response.ok) {
      let error = `Request failed (${response.status})`;
      try {const data = await response.json(); error = data.error || error;} catch { /* Legacy API sends plaintext. */ }
      throw new Error(error);
    }
    return response.json();
  };
  const send = (path, value) => api(path, {method: 'POST', body: JSON.stringify(value)});
  const label = key => String(key).replaceAll('_', ' ').replaceAll('.', ' ');
  let files = [], items = [], submitted = false, busy = false, position = 0, editingSavedID = "";
  const message = (text, failure = false) => {
    $('review-feedback').textContent = text;
    $('review-feedback').classList.toggle('is-error', failure);
  };
  const docID = () => $('review-file').value;
  const pending = () => items.filter(x => !['KEEP','SKIP'].includes(x.action));
  const setCount = () => {
    const count = pending().length;
    $('review-pending').hidden = count === 0 || !docID() || submitted;
    $('review-pending').textContent = String(count);
    $('review-pending').setAttribute('aria-label', `${count} records need review`);
  };
  function renderFiles() {
    const select = $('review-file');
    const previous = select.value;
    select.replaceChildren();
    node('option', '', 'Choose a file', select).value = '';
    for (const d of files) {
      const opt = node('option', '', `${d.filename} · ${label(d.status).toLowerCase()}`, select);
      opt.value = d.id;
      opt.disabled = !['EXTRACTED','PARTIAL'].includes(d.status);
    }
    const fromURL = new URLSearchParams(window.location.search).get('document');
    const requested = [previous, fromURL].find(id => files.some(d => d.id === id && ['EXTRACTED','PARTIAL'].includes(d.status)));
    if (requested) select.value = requested;
    $('review-scan').disabled = !docID() || busy || submitted;
  }
  const source = record => {
    const loc = record.source_locator || {};
    if (loc.sheet) return `${loc.sheet}${loc.row ? ` · row ${loc.row}` : ''}`;
    if (loc.page) return `page ${loc.page}`;
    if (loc.row) return `row ${loc.row}`;
    return `item ${record.source_ordinal}`;
  };
  function render() {
    setCount();
    const card = $('review-card'); card.replaceChildren();
    const host = $('review-final'); host.replaceChildren();
    const empty = $('review-empty');
    $('review-steps').hidden = true;
    empty.hidden = false;
    if (!docID()) { empty.textContent = 'Choose a file to begin.'; return; }
    if (submitted) { empty.textContent = 'Submitted. These checked records are saved in the versioned factory reference list.'; return; }
    if (!items.length) {empty.textContent = 'No records found yet. Choose Find records.'; return;}
    const need = pending();
    if (!need.length && !editingSavedID) {
      empty.textContent = 'All records checked. Please review the final count below.';
      const kept = items.filter(x => x.action === 'KEEP').length;
      const skipped = items.filter(x => x.action === 'SKIP').length;
      const summary = node('div','review-final-card',undefined,host);
      node('strong','',`${kept} ready · ${skipped} skipped`,summary);
      const savedSelect = node('select', 'review-saved-select', undefined, summary);
      const blank = node('option', '', 'Change a reviewed record', savedSelect);blank.value='';
      for (const item of items) {const option=node('option','',`${label(item.entity_type)} · ${item.entity_key} (${label(item.action).toLowerCase()})`,savedSelect);option.value=item.id;}
      savedSelect.setAttribute('aria-label','Change a reviewed record');
      savedSelect.addEventListener('change',()=>{if(savedSelect.value){editingSavedID=savedSelect.value;render();}});
      node('p','review-note','Submit saves reviewed factory-reference records with source history. It does not change wages, stock, or production approvals.',summary);
      const confirmLabel = node('label','review-final-confirm',undefined,summary);
      const check = node('input','',undefined,confirmLabel);check.type='checkbox';
      node('span','',`I approve these ${kept} checked records for the factory reference list.`,confirmLabel);
      const button = node('button','primary','Submit checked records',summary);
      button.type='button';button.disabled = !kept || busy;
      check.addEventListener('change',()=>{button.disabled = busy || !check.checked || !kept;});
      button.addEventListener('click', async()=>{
        if (!check.checked || busy || !window.confirm(`Submit ${kept} reviewed record(s)? This will create an immutable approved reference snapshot.`)) return;
        busy = true;button.disabled = true;message('Submitting checked records…');
        try {
          const out = await send('/api/v1/factory-onboarding/approvals/submit', {document_id:docID(),expected_count:items.length,confirmation:'SUBMIT'});
          message(`${out.approved_records} records submitted. Submission: ${out.submission_id}. Operational records were not changed.`);
          await loadReview();
        } catch(e) {message(`Not submitted: ${e.message}`,true);}
        finally {busy = false;}
      });
      return;
    }
    if (position >= need.length) position = 0;
    const record = editingSavedID ? items.find(x=>x.id===editingSavedID) : need[position];
    if (!record) {editingSavedID='';render();return;}
    empty.hidden = true; $('review-steps').hidden = false;
    $('review-counter').textContent = editingSavedID ? 'Change saved record' : `Record ${position+1} of ${need.length} needing review`;
    node('span','review-entity',label(record.entity_type),card);
    node('h3','',record.entity_key,card);
    const link = node('a','review-source',`View original · ${record.filename} · ${source(record)} ↗`,card);
    link.href = `/documents?document=${encodeURIComponent(record.document_id)}&segment=${encodeURIComponent(record.source_ordinal)}`;
    const fields = {...record.attributes};
    if (record.entity_type === 'RATE' && !Object.hasOwn(fields,'rate_basis')) fields.rate_basis = '';
    const grid = node('div','review-attribute-list',undefined,card);
    const confirmed = new Set();
    const allFields = Object.keys(fields).sort();
    const buttons = node('div','review-actions',undefined,card);
    const keep = node('button','primary','Save checked record',buttons);keep.type='button';keep.disabled=true;
    const skip = node('button','secondary','Not needed',buttons);skip.type='button';
    const next = node('button','secondary','Next →',buttons);next.type='button';next.disabled = need.length < 2;
    next.addEventListener('click',()=>{position=(position+1)%need.length;render();});
    if(editingSavedID){const cancel=node('button','secondary','Back to summary',buttons);cancel.type='button';cancel.addEventListener('click',()=>{editingSavedID='';render();});}
    const sync = () => {keep.disabled = busy || allFields.some(k=>!confirmed.has(k) || !String(fields[k]).trim());};
    for (const key of allFields) {
      const row = node('div','review-attribute',undefined,grid);
      const group = node('label','review-edit',undefined,row);
      node('span','',label(key),group);
      let input;
      if (key === 'rate_basis') {
        input = node('select','',undefined,group);
        const blank=node('option','','Choose unit',input);blank.value='';
        for(const basis of ['HOUR','PIECE','METER','YARD','KG','DAY','UNIT']) {
          const option=node('option','',label(basis).toLowerCase(),input);option.value=basis;
        }
      } else {
        input = node('input','',undefined,group);
        input.type='text';input.maxLength=300;
      }
      input.value=fields[key]??'';
      const checkLabel=node('label','review-check',undefined,row);
      const checkbox=node('input','',undefined,checkLabel);checkbox.type='checkbox';
      checkbox.setAttribute('aria-label',`Approve ${label(key)}`);
      node('span','','Approve',checkLabel);
      input.addEventListener('input',()=>{fields[key]=input.value;confirmed.delete(key);checkbox.checked=false;sync();});
      input.addEventListener('change',()=>{fields[key]=input.value;confirmed.delete(key);checkbox.checked=false;sync();});
      checkbox.addEventListener('change',()=>{if(checkbox.checked)confirmed.add(key);else confirmed.delete(key);sync();});
    }
    const decide = async action => {
      if(busy)return;
      busy=true;keep.disabled=true;skip.disabled=true;next.disabled=true;
      message(action==='KEEP'?'Saving checked fields…':'Setting this record aside…');
      try {
        const res=await send('/api/v1/factory-onboarding/approvals/review', {
          candidate_id: record.id,action,attributes: action==='KEEP'? fields:{},
          checked_fields: action==='KEEP'? [...confirmed]:[],expected_revision:record.revision
        });
        message(action==='KEEP' ? `Checked record saved (revision ${res.revision}).` : 'Record set aside.');
        position=0;editingSavedID='';await loadReview();
      } catch(e) {
        message(`Could not save: ${e.message}`,true);
        busy=false;skip.disabled=false;next.disabled=!!editingSavedID||need.length<2;sync();
        return;
      }
      finally{busy=false;}
    };
    keep.addEventListener('click',()=>{if(!keep.disabled)void decide('KEEP');});
    skip.addEventListener('click',()=>void decide('SKIP'));
    node('p','review-note','Edit any value, then approve every field. Changes are not submitted until the final step.',card);
  }
  async function loadReview() {
    if(!docID()){items=[];submitted=false;render();return;}
    const out=await api(`/api/v1/factory-onboarding/approvals?document_id=${encodeURIComponent(docID())}`);
    items=out.items||[];submitted=out.submitted===true;render();
  }
  async function load() {
    try {
      const result=await api('/api/v1/documents');files=result.items||[];
      renderFiles();await loadReview();
      if(!submitted)message('Review one record at a time.');
    } catch(e) {message(`Review unavailable: ${e.message}`,true);}
  }
  async function scan() {
    if(!docID()||busy||submitted)return;
    busy=true;$('review-scan').disabled=true;message('Finding records in the selected file…');
    try {
      const result=await send('/api/v1/factory-onboarding/candidates/scan',{document_id:docID()});
      position=0;await loadReview();
      message(result.matched ? `Found ${result.matched} possible record(s). Check each one.` : 'No clearly labeled details found. Nothing was guessed.');
    } catch(e){message(`Cannot find records: ${e.message}`,true);}
    finally {busy=false;$('review-scan').disabled=!docID()||submitted;}
  }
  if (window.location.hash === '#review-submit') $('review-submit').open = true;
  $('review-file').addEventListener('change',()=>{position=0;editingSavedID='';void loadReview().catch(e=>message(e.message,true));});
  $('review-scan').addEventListener('click',()=>void scan());
  $('review-submit').addEventListener('toggle',()=>{if($('review-submit').open)void load();});
  document.addEventListener('agiparel-document-selected',event=>{
    const id=event.detail?.id;
    if(id && files.some(f=>f.id===id)){$('review-file').value=id;void loadReview().catch(e=>message(e.message,true));}
  });
  document.addEventListener('agiparel-documents-updated',event=>{
    const incoming=event.detail?.items;
    if(!Array.isArray(incoming))return;
    files=incoming;renderFiles();
  });
  void load();
})();
