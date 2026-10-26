// Gate 06.5-B4: reference catalog UI, explicitly separated from operational authority.
(() => {
 'use strict';
 const root=document.getElementById('reference-ready');if(!root)return;
 const $=id=>document.getElementById(id);
 const el=(tag,cl,txt)=>{const x=document.createElement(tag);if(cl)x.className=cl;if(txt!==undefined)x.textContent=txt;return x;};
 let history=false; let busy=false;
 const url='/api/v1/factory-onboarding/reference-masters';
 const feedback=(text,bad=false)=>{const e=$('reference-feedback');e.textContent=text;e.classList.toggle('bad',bad);};
 const fetchJSON=async(path,opts={})=>{const r=await fetch(path,{credentials:'same-origin',headers:{Accept:'application/json',...(opts.method?{'Content-Type':'application/json'}:{})},...opts});let data;try{data=await r.json();}catch{data={error:`HTTP ${r.status}`};}if(!r.ok)throw Error(data.error||`HTTP ${r.status}`);return data;};
 const pretty=o=>Object.entries(o||{}).map(([k,v])=>`${k.replaceAll('_',' ')}: ${v}`).join(' · ');
 const sourceLink=doc=>{const link=el('a','',`${doc.filename||'Source document'} · segment ${doc.source_ordinal} ↗`);link.href=`/documents?document=${encodeURIComponent(doc.document_id)}&segment=${encodeURIComponent(doc.source_ordinal)}`;return link;};
 const acknowledge=()=>{const label=el('label','');const box=document.createElement('input');box.type='checkbox';label.append(box,document.createTextNode(' I checked the selected evidence. This is a factory reference, not authorization for production, inventory, payroll or approved costing.'));return {label,box};};
 const noteField=placeholder=>{const field=document.createElement('textarea');field.rows=2;field.maxLength=1000;field.placeholder=placeholder;field.setAttribute('aria-label','What was verified and by whom?');return field;};
 const save=async(payload)=>{if(busy)return;busy=true;try{const result=await fetchJSON(url+'/revise',{method:'POST',body:JSON.stringify(payload)});feedback(`${result.entity_type} ${result.entity_key}: catalog revision ${result.revision} ${result.action.toLowerCase()}. It remains a reference only.`);await load();}catch(e){feedback(`Not saved: ${e.message}`,true);}finally{busy=false;}};
 const refCard=(g,latest)=>{
  const card=el('article','reference-card');card.append(el('span','reference-badge',`${g.entity_type} · VERIFIED DRAFT #${g.revision}`),el('h4','',g.entity_key));
  const chosen=g.sources.find(x=>x.candidate_id===g.selected_candidate_id);if(!chosen)return card;
  card.append(el('p','reference-attrs',pretty(chosen.attributes)),sourceLink(chosen));
  if(g.evidence_state==='CONFLICT')card.append(el('p','reference-alert','The draft was selected from conflicting sources. Double-check why it was chosen.'));
  if(latest?.action==='PUBLISH' && latest.candidate_id===chosen.candidate_id && latest.draft_revision===g.revision){card.append(el('p','',`Already published · catalog revision ${latest.revision}`));return card;}
  const note=noteField('What physical record or responsible person verified this? (12+ characters)');card.append(note);
  let basis;
  if(g.entity_type==='RATE'){
   const label=el('label','','I confirmed the rate applies per:');basis=document.createElement('select');basis.setAttribute('aria-label','Confirmed rate basis');
   for(const [v,labelText] of [['','Choose rate basis'],['HOUR','Hour'],['PIECE','Piece'],['METER','Meter'],['YARD','Yard'],['KG','Kilogram'],['DAY','Day'],['UNIT','Unit']]){const o=el('option','',labelText);o.value=v;basis.append(o);}
   label.append(basis);card.append(label);
  }
  const ack=acknowledge();card.append(ack.label);
  const btn=el('button','btn primary','Publish factory reference');btn.type='button';btn.onclick=async()=>{
   if(note.value.trim().length<12 || !ack.box.checked || (basis && !basis.value)){feedback('Add a verification explanation, acknowledge the boundary, and confirm any rate unit.',true);return;}
   btn.disabled=true;await save({entity_type:g.entity_type,entity_key:g.entity_key,candidate_id:chosen.candidate_id,draft_revision:g.revision,expected_revision:latest?.revision||0,action:'PUBLISH',rate_basis:basis?.value||'',note:note.value,acknowledged:true});btn.disabled=false;
  };card.append(btn);return card;
 };
 const currentCard=m=>{
  const card=el('article',`reference-card ${m.action==='PUBLISH'?'reference-published':'reference-withdrawn'}`);
  card.append(el('span','reference-badge',`${m.entity_type} · ${m.action==='PUBLISH'?'PUBLISHED REFERENCE':'WITHDRAWN'} · revision ${m.revision}`),el('h4','',m.entity_key));
  card.append(el('p','reference-attrs',pretty(m.attributes)),sourceLink(m));
  if(m.rate_basis)card.append(el('p','',`Rate basis: ${m.rate_basis}`));
  card.append(el('p','',`Review note: ${m.reviewer_note}`));
  if(m.revalidation_required)card.append(el('p','reference-alert','New accepted evidence has arrived since the review. Reconcile sources before updating this reference.'));
  if(m.action==='PUBLISH' && !history){const note=noteField('Why should this reference be withdrawn? (12+ characters)');card.append(note);const ack=acknowledge();card.append(ack.label);const btn=el('button','btn secondary','Withdraw this reference');btn.onclick=async()=>{if(note.value.trim().length<12||!ack.box.checked){feedback('Explain the withdrawal and check the acknowledgement.',true);return;}btn.disabled=true;await save({entity_type:m.entity_type,entity_key:m.entity_key,candidate_id:m.candidate_id,draft_revision:m.draft_revision,expected_revision:m.revision,action:'WITHDRAW',rate_basis:m.rate_basis||'',note:note.value,acknowledged:true});btn.disabled=false;};card.append(btn);}
  return card;
 };
 async function load(){try{const [d,r]=await Promise.all([fetchJSON('/api/v1/factory-onboarding/draft-masters'),fetchJSON(url+(history?'?history=1':''))]);root.replaceChildren();$('reference-current').replaceChildren();
   const current=new Map();for(const m of r.items){const key=m.entity_type+'\0'+m.entity_key;if(!current.has(key))current.set(key,m);}
   let eligible=0,excluded=0;for(const group of d.items){if(group.entity_type==='MATERIAL'){excluded++;continue;}if(group.decision!=='VERIFY_DRAFT')continue;eligible++;root.append(refCard(group,current.get(group.entity_type+'\0'+group.entity_key)));}
   if(!eligible)root.append(el('p','register-note','Nothing ready to publish yet. Review suggested factory records and resolve disagreements above. Inventory observations remain in the stores-verification queue.'));
   for(const row of r.items)$('reference-current').append(currentCard(row));if(!r.items.length)$('reference-current').append(el('p','register-note','No published factory references. You can start with partial onboarding and add records gradually.'));
   $('reference-summary').textContent=`${eligible} reviewed drafts available · ${[...current.values()].filter(x=>x.action==='PUBLISH').length} current published references · ${excluded} inventory groups intentionally excluded`+(d.truncated||r.truncated?' · TRUNCATED: reconcile source library before continuing':'');
   if(d.truncated||r.truncated)feedback('The evidence list is truncated. Resolve the volume before updating references.',true);
  }catch(e){feedback(`Reference catalog unavailable: ${e.message}`,true);root.replaceChildren(el('p','register-note','Check database migration 0013 and operator access.'));}}
 $('reference-refresh').addEventListener('click',load);
 $('reference-history-toggle').addEventListener('click',()=>{history=!history;$('reference-history-toggle').textContent=history?'Show current references':'Show revision history';load();});
 window.addEventListener('agiparel:factory-drafts-updated',load);
 load();
})();
