// Batch 07C: read-only preparation. Never accept approvals from Basic login.
(()=>{'use strict';
 const $=id=>document.getElementById(id), host=$('admission-queue'), feedback=$('admission-identity');
 const el=(name,text,cls='')=>{const n=document.createElement(name);n.textContent=text;n.className=cls;return n};
 const names={ALREADY_ADMITTED:'Already registered to the stock ledger',ROLL_ALREADY_IN_LEDGER:'A ledger roll already exists; secure receipt cannot overwrite it',PASSPORT_REVISION_STALE:'New observation since review; request reconciliation again',INSPECTION_NOT_PASSED:'Physical inspection reported HOLD or REJECTED',RFID_EPC_NOT_BOUND:'Attach an unambiguous roll RFID EPC first',SOURCE_DOCUMENT_REQUIRED:'Link original GRN or inspection source document',REVIEW_SNAPSHOT_CONFLICT:'Reviewed evidence does not match the roll record',INVALID_MEASUREMENTS:'Measured length or width invalid',INVALID_ROLL_IDENTITY:'Roll identity requires correction',INVALID_REVIEW_REQUEST:'Review request identifier invalid',INVALID_SNAPSHOT_HASH:'Stored assessment digest invalid'};
 async function refresh(){host.replaceChildren(el('p','Loading…'));try{
  const r=await fetch('/api/v1/stores/admissions',{credentials:'same-origin'});if(!r.ok)throw Error(`HTTP ${r.status} — migration 0017 may be missing`);const data=await r.json();
  feedback.textContent=data.identity_enabled?'External signed-material identity verifier is configured. Admission still requires two fresh, distinct, challenge-bound WebAuthn/FIDO2 assertions.':'Signed roll admission is disabled until an external identity provider and the material authority configuration are provisioned. Readiness is informational only.';
  host.replaceChildren();const items=data.items||[];if(!items.length){host.append(el('p','No reconciliation requests yet. Record a passport and submit an independent-review request above.','empty'));return}
  for(const q of items){const card=el('article','','admission-card');card.append(el('h3',`${q.roll_id} · revision ${q.passport_revision}`));const badge=el('span',q.state.replaceAll('_',' '),'pill '+(q.state==='ADMITTED_TO_LEDGER'?'approved':q.state==='NOT_READY'?'disabled':''));card.append(badge);
   card.append(el('p',`Measured ${(q.measured_mm/1000).toFixed(3)} m · ${q.epc_hex?'EPC bound':'EPC missing'} · ${q.source_document_id?'source linked':'source missing'}`));
   card.append(el('p',`Immutable review: ${q.request_id}`));
   if(q.admitted_at)card.append(el('p',`Admitted ${new Date(q.admitted_at).toLocaleString()}`));
   if(q.reasons?.length){const ul=document.createElement('ul');for(const reason of q.reasons){ul.append(el('li',names[reason]||reason))}card.append(ul)}
   if(q.state==='READY_FOR_SIGNED_REVIEW')card.append(el('p','Eligible for external dual-signature challenge. This does not mean inspection is independently authenticated.'));
   host.append(card)}
 }catch(e){feedback.textContent=`Cannot load admission readiness: ${e.message}`;host.replaceChildren(el('p','Roll admission records unavailable.','empty'))}}
 $('admission-refresh').addEventListener('click',()=>void refresh());void refresh();
})();
