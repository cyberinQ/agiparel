/* Batch 07D: tenant-scoped order-to-roll pick proposals; no browser stock writes. */
(()=>{'use strict';
 const $=id=>document.getElementById(id);
 const make=(tag,text,css='')=>{const n=document.createElement(tag);n.textContent=text;if(css)n.className=css;return n};
 const fmt=mm=>(Number(mm||0)/1000).toLocaleString(undefined,{minimumFractionDigits:3,maximumFractionDigits:3})+' m';
 let orders=[],history=[],key='';
 const human=s=>String(s||'').replaceAll('_',' ').toLowerCase();
 function materialView(mat){const section=make('article','', 'pick-material');section.append(make('h3',mat.material_sku+' · '+mat.uom));
  const metrics=make('div','', 'pick-metrics');for(const [k,v] of [['Gross need',mat.demand_mm],['Tentative roll coverage',mat.tentative_mm],['Uncovered planning gap',mat.uncovered_mm]]){const d=make('div','');d.append(make('b',fmt(v)),make('span',k));metrics.append(d)}section.append(metrics);
  section.append(make('p',`Selected shade: ${mat.chosen_shade||'none'} · No cross-shade blending`, 'sub'));
  const ul=make('ul','', 'pick-rolls');for(const r of mat.tentative_picks||[]){const li=make('li','');li.append(make('b',r.roll_id+' · '),make('span',`${fmt(r.suggested_mm)} · width ${r.usable_width_mm} mm · ${r.shade} · tentative`));ul.append(li)}if(!ul.childNodes.length)ul.append(make('li','No eligible roll selection'));section.append(ul);
  if((mat.excluded_rolls||[]).length){const excluded=make('details','');excluded.append(make('summary',`Show ${mat.excluded_rolls.length} excluded ledger rolls`));const lst=make('ul','', 'pick-holds');for(const x of mat.excluded_rolls)lst.append(make('li',`${x.roll_id}: ${human(x.reason)}`));excluded.append(lst);section.append(excluded)}
  const holds=make('ul','', 'pick-holds');for(const h of mat.blockers||[])holds.append(make('li',human(h)));section.append(holds);return section;
 }
 function showReport(s){const dest=$('pick-results');dest.replaceChildren();if(!s||!s.materials){dest.append(make('p','No proposal selected.','empty'));return}
  dest.append(make('div',`PROPOSED / HOLD · ${s.order.po_number} · ${s.order.style_code} · ${s.order.ordered_qty.toLocaleString()} garments`, 'pick-notice'));
  for(const m of s.materials)dest.append(materialView(m));const notes=make('div','', 'pick-notice');notes.append(make('b','Still requires independent verification'));const list=make('ul','');for(const w of s.warnings||[])list.append(make('li',w));notes.append(list);dest.append(notes);
 }
 function showHistory(){const dest=$('pick-history');dest.replaceChildren();if(!history.length){dest.append(make('p','No saved pick proposals yet.','empty'));return}
  for(const p of history){const v=make('div','', 'pick-history-item');v.append(make('strong',`${p.snapshot.order.po_number} · ${p.state}`));v.append(make('small',`${new Date(p.created_at).toLocaleString()} · ${p.snapshot.materials.length} fabric materials`));const btn=make('button','Review saved proposal');btn.type='button';btn.addEventListener('click',()=>showReport(p.snapshot));v.append(btn);dest.append(v)}
 }
 async function refresh(){try{const r=await fetch('/api/v1/stores/pick-proposals',{credentials:'same-origin'});if(!r.ok)throw Error(`HTTP ${r.status}`);const d=await r.json();orders=d.orders||[];history=d.proposals||[];const sel=$('pick-order'),last=sel.value;sel.replaceChildren();const option=make('option','Select a staged buyer order');option.value='';sel.append(option);for(const o of orders){const x=make('option',`${o.po_number} · ${o.style_code} · ${o.ordered_qty.toLocaleString()} pcs`);x.value=o.intake_id;sel.append(x)}if(orders.some(x=>x.intake_id===last))sel.value=last;showHistory();$('pick-feedback').textContent=`${orders.length} staged orders available. Proposals are permanently marked HOLD.`;
  }catch(e){$('pick-feedback').textContent=`Unable to load buyer orders or pick history: ${e.message}. Check migration 0018.`}}
 $('pick-refresh').addEventListener('click',()=>void refresh());
 $('pick-order').addEventListener('change',()=>{key=''});
 $('pick-form').addEventListener('submit',async e=>{e.preventDefault();const intake_id=$('pick-order').value,note=$('pick-note').value.trim();if(!intake_id||note.length<12){$('pick-feedback').textContent='Choose an order and explain the purpose (at least 12 characters).';return}
  if(!key)key=crypto.randomUUID();const button=$('pick-submit');button.disabled=true;$('pick-feedback').textContent='Calculating a snapshot from the protected ledger…';
  try{const r=await fetch('/api/v1/stores/pick-proposals',{method:'POST',credentials:'same-origin',headers:{'Content-Type':'application/json'},body:JSON.stringify({intake_id,request_key:key,note})});const data=await r.json();if(!r.ok)throw Error(data.error||`HTTP ${r.status}`);showReport(data.proposal.snapshot);$('pick-feedback').textContent=`${data.replayed?'Existing':'New'} ${data.proposal.state} pick proposal saved. No stock was reserved or issued.`;key='';$('pick-note').value='';await refresh();
  }catch(e){$('pick-feedback').textContent=`Cannot save proposal: ${e.message}. A retry reuses the same key.`}finally{button.disabled=false}
 });
 void refresh();
})();
