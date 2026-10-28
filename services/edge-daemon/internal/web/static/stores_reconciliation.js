/* Author: Sunil Jain, 2026-10-28 — Reconciliation idempotency and stale-form protection. */
/* Batch 07B. Display-only ledger comparison, append-only review requests. */
(()=>{'use strict';
 const $=id=>document.getElementById(id);let assessments=[];let requestKey='';
 const name=s=>String(s||'').replace(/_/g,' ').toLowerCase().replace(/\b\w/g,x=>x.toUpperCase());
 function mm(n){return (Number(n||0)/1000).toLocaleString(undefined,{minimumFractionDigits:3,maximumFractionDigits:3})+' m'}
 function el(tag,cls,text){const v=document.createElement(tag);if(cls)v.className=cls;if(text!==undefined)v.textContent=text;return v}
 function clear(id){const node=$(id);node.replaceChildren();return node}
 async function get(path){const r=await fetch(path,{credentials:'same-origin'});if(!r.ok)throw Error(`${path}: HTTP ${r.status}`);return r.json()}
 function showComparisons(){const host=clear('reconcile-cards'),select=$('reconcile-roll'),old=select.value;select.replaceChildren();select.append(el('option','', 'Select a roll'));select.lastChild.value='';
  if(!assessments.length){host.append(el('p','empty','No observed rolls yet. Record a roll above to begin checking against stock ledger.'));return}
  for(const a of assessments){const card=el('article','compare-card');card.append(el('h3','',a.roll_id));card.append(el('p','',`Passport revision ${a.passport_revision} · Reported ${a.passport_inspection}`));
   card.append(el('p','',a.ledger.exists?`Ledger ${a.ledger.stock_state} / ${a.ledger.inspection_status}`:'No authoritative ledger roll found'));
   card.append(el('p','',a.ledger.exists?`Measured ${mm(a.measured_mm)} · Ledger original ${mm(a.ledger.original_mm)} · Ledger remaining ${mm(a.ledger.remaining_mm)}`:`Measured ${mm(a.measured_mm)} · Awaiting secure stock registration`));
   if(a.ledger.exists){card.append(el('p','',`Ledger open reservations ${mm(a.ledger.open_reservation_mm)} · Uncommitted ${mm(a.ledger_uncommitted_mm)}`));if(a.ledger.reserved_order_id)card.append(el('p','',`Legacy order ownership: ${a.ledger.reserved_order_id}`))}
   const flags=el('div','flag-list');for(const f of a.findings||[])flags.append(el('span',f==='AWAITING_INDEPENDENT_VERIFICATION'?'verified-pending':'',name(f)));card.append(flags);
   if(a.source_document_id){const aLink=el('a','', 'View original evidence ↗');aLink.href='/documents?document='+encodeURIComponent(a.source_document_id);card.append(aLink)}
   host.append(card);const option=el('option','',`${a.roll_id} · ${a.state}`);option.value=a.roll_id;select.append(option)}
  if(assessments.some(a=>a.roll_id===old))select.value=old;
 }
 function showRequests(rows){const host=clear('reconcile-history');if(!rows.length){host.append(el('p','empty','No review requests yet.'));return}
  for(const q of rows){const div=el('div','history-row');div.append(el('strong','',`${q.roll_id} · passport revision ${q.passport_revision}`));div.append(el('small','',`${new Date(q.created_at).toLocaleString()} · ${q.state}`));div.append(el('p','',q.note));div.append(el('small','',`Findings preserved: ${(q.assessment?.findings||[]).map(name).join(', ')}`));host.append(div)}
 }
 function showReservations(rows){const host=clear('secure-reservations');if(!rows.length){host.append(el('p','empty','No secured reservations recorded for this tenant yet. Observed rolls are not stock allocations.'));return}
  for(const r of rows){const line=el('div','reservation-line');const a=el('strong','',`${r.roll_id} · ${r.order_id} · Cut ${r.cut_no}`);const b=el('span','',`Reserved ${mm(r.reserved_mm)} · Issued ${mm(r.issued_mm)} · Released ${mm(r.released_mm)} · Open ${mm(r.open_mm)}`);line.append(a,b);host.append(line)}
 }
 async function refresh(){try{const [a,c,r]=await Promise.all([get('/api/v1/stores/reconciliation'),get('/api/v1/stores/reconciliation-requests'),get('/api/v1/stores/reservations')]);assessments=a.items||[];showComparisons();showRequests(c.items||[]);showReservations(r.items||[])}catch(e){$('reconcile-feedback').textContent=`Cannot load ledger review: ${e.message}. Check migration 0016.`}}
 $('reconcile-refresh').addEventListener('click',()=>void refresh());
 $('reconcile-roll').addEventListener('change',()=>{requestKey=''});
 $('reconcile-note').addEventListener('input',()=>{requestKey=''});
 $('reconciliation-form').addEventListener('submit',async event=>{event.preventDefault();const roll=$('reconcile-roll').value;const item=assessments.find(x=>x.roll_id===roll);const note=$('reconcile-note').value.trim();const btn=$('reconcile-submit');if(!item||note.length<12){$('reconcile-feedback').textContent='Choose a roll and enter at least 12 characters explaining what needs checking.';return}
  if(!requestKey)requestKey=crypto.randomUUID();const submittedKey=requestKey;btn.disabled=true;$('reconcile-feedback').textContent='Recording a source-linked review request…';
  try{const r=await fetch('/api/v1/stores/reconciliation',{method:'POST',credentials:'same-origin',headers:{'Content-Type':'application/json'},body:JSON.stringify({roll_id:roll,expected_passport_revision:item.passport_revision,request_key:submittedKey,note})});const data=await r.json();if(!r.ok)throw Error(data.error||`HTTP ${r.status}`);
   if(requestKey===submittedKey && $('reconcile-roll').value===roll){
     requestKey='';$('reconcile-note').value='';
     $('reconcile-feedback').textContent=`${data.replayed?'Existing':'New'} review request ${data.request.id} saved. This does not authorize stock or issuance.`;
   }
   await refresh();
  }catch(e){if(requestKey===submittedKey) $('reconcile-feedback').textContent=`Could not submit review: ${e.message}. Refresh if its revision changed; otherwise retry the unchanged request with the same key.`}finally{btn.disabled=false}
 });
 void refresh();
})();
