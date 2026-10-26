// Gate 06.5-B3: versioned onboarding drafts and human-led conflict resolution.
(() => {
 'use strict';
 const find = id => document.getElementById(id);
 const element = (tag, cls, contents) => {const e=document.createElement(tag); if(cls)e.className=cls; if(contents!==undefined)e.textContent=contents;return e;};
 const get = async (url,init={}) => {
  const response=await fetch(url,{credentials:'same-origin',headers:{Accept:'application/json',...(init.method?{'Content-Type':'application/json'}:{})},...init});
  if(!response.ok){let detail=`HTTP ${response.status}`;try{const j=await response.json();detail=j.error||detail;}catch{}throw Error(detail);}return response.json();
 };
 const root=find('draft-items');if(!root)return;
 const message=(text,bad=false)=>{find('draft-feedback').textContent=text;find('draft-feedback').classList.toggle('bad',bad);};
 const location = source => {const loc=source.source_locator||{};if(loc.sheet)return `${loc.sheet}, row ${loc.row||'?'}`;if(loc.page)return `page ${loc.page}`;if(loc.row)return `row ${loc.row}`;return `segment ${source.source_ordinal}`;};
 const link = source => {const a=element('a','draft-link',`${source.filename} · ${location(source)} ↗`);a.href=`/documents?document=${encodeURIComponent(source.document_id)}&segment=${encodeURIComponent(source.source_ordinal)}`;return a;};
 let truncated=false;
 const build = group => {
  const card=element('article',`draft-card ${group.evidence_state==='CONFLICT'?'draft-conflict':''}`);
  const top=element('div','draft-top');top.append(element('strong','',`${group.entity_type} · ${group.entity_key}`));top.append(element('span','draft-state',group.evidence_state.replaceAll('_',' ')));card.append(top);
  const description=group.evidence_state==='CONFLICT'?`${group.distinct_versions} different versions of this record were found. Please compare the originals and choose one to carry forward as a draft.`:group.evidence_state==='CORROBORATED'? 'Multiple sources agree on this provisional record. Confirm the source before using it further.': 'Only one source currently supports this record. You can keep it provisional until someone checks it.';
  card.append(element('p','draft-help',description));
  const choice=element('div','draft-choices');const options=[];
  for(const source of group.sources){
   const label=element('label','draft-choice');const input=document.createElement('input');input.type='radio';input.name=`review-${group.entity_type}-${group.entity_key}`;input.value=source.candidate_id;
   input.checked=source.candidate_id===(group.selected_candidate_id||group.sources[0].candidate_id);options.push(input);
   const body=element('span','draft-choice-body');const attrs=source.attributes||{};
   body.append(element('strong','draft-values',Object.entries(attrs).map(([k,v])=>`${k.replaceAll('_',' ')}: ${v}`).join(' · ')));body.append(link(source));label.append(input,body);choice.append(label);
  }
  card.append(choice);
  const state=element('p','draft-decision',`Current decision: ${group.decision.replaceAll('_',' ')} · revision ${group.revision}`);card.append(state);
  if(group.decision_note)card.append(element('p','draft-help',`Review note: ${group.decision_note}`));
  const note=document.createElement('textarea');note.className='draft-note';note.placeholder=group.evidence_state==='CONFLICT'?'Explain which evidence is correct and why (required, at least 12 characters)':'Optional review note or remaining question';note.maxLength=1000;note.rows=2;note.setAttribute('aria-label',`Review explanation for ${group.entity_type} ${group.entity_key}`);card.append(note);
  const actions=element('div','draft-actions');
  const submit=async(decision)=>{
   if(truncated){message('Too many records to display safely. Reconcile the document library before confirming.',true);return;}
   if(group.evidence_state==='CONFLICT' && note.value.trim().length<12){message(`Explain the discrepancy in ${group.entity_key} before recording a decision.`,true);note.focus();return;}
   const candidate=options.find(x=>x.checked)?.value;
   if(!candidate){message('Select the supporting source first.',true);return;}
   [...actions.querySelectorAll('button')].forEach(b=>b.disabled=true);
   try{
    const response=await get('/api/v1/factory-onboarding/draft-masters/review',{method:'POST',body:JSON.stringify({entity_type:group.entity_type,entity_key:group.entity_key,candidate_id:candidate,decision,note:note.value,expected_revision:group.revision})});
    message(`${group.entity_type} ${group.entity_key}: review revision ${response.revision} saved. This remains onboarding evidence, not operational master data.`);
    await load();
   }catch(e){message(`Review not saved: ${e.message}`,true);[...actions.querySelectorAll('button')].forEach(b=>b.disabled=false);}
  };
  const verify=element('button','btn primary','Mark reviewed draft');verify.type='button';verify.addEventListener('click',()=>submit('VERIFY_DRAFT'));
  const flag=element('button','btn secondary','Needs clarification');flag.type='button';flag.addEventListener('click',()=>submit('NEEDS_CLARIFICATION'));
  actions.append(verify,flag);card.append(actions);
  return card;
 };
 async function load(){
  try{
   const result=await get('/api/v1/factory-onboarding/draft-masters');truncated=!!result.truncated;const groups=result.items||[];
   const conflicts=groups.filter(g=>g.evidence_state==='CONFLICT').length;
   const verified=groups.filter(g=>g.decision==='VERIFY_DRAFT').length;
   find('draft-count').textContent=`${groups.length} entities · ${conflicts} disagreements · ${verified} reviewed drafts`;
   root.replaceChildren();if(!groups.length)root.append(element('p','empty','No accepted factory records to reconcile yet. Upload a factory document, find candidates and keep the relevant ones as provisional first.'));
   else for(const g of groups)root.append(build(g));
   if(truncated)message('More than 500 source rows exist. Display is incomplete; review actions are disabled until this is resolved.',true);
  }catch(e){message(`Cannot load draft review: ${e.message}. Apply migration 0012.`,true);}
 }
 find('refresh-factory-drafts').addEventListener('click',load);
 document.addEventListener('agiparel-factory-evidence-updated',load);
 void load();
})();
