// Author: Sunil Jain, 2026-11-11 — Read-only signed-evidence dashboard and explicit development target editor.
'use strict';
(() => {
  const $ = id => document.getElementById(id);
  let selected = '', sourceLines = [], requestKey = '';
  const numeric = n => Number(n || 0).toLocaleString('en-US');
  const format = n => Number(n || 0).toLocaleString('en-US');
  const percentBP = raw => Math.round(Number(raw) * 100);
  const status = (message, error = false) => { $('sop2-feedback').textContent = message; $('sop2-feedback').classList.toggle('error', error); };
  const td = value => { const c = document.createElement('td'); c.textContent = String(value); return c; };
  const api = async (path, body) => {
    const options = body === undefined ? {headers:{Accept:'application/json'}} : {method:'POST',headers:{'Content-Type':'application/json',Accept:'application/json'},body:JSON.stringify(body)};
    const response=await fetch(path, options), txt=await response.text();
    let data;try{data=JSON.parse(txt)}catch{throw new Error(`${response.status} ${txt.slice(0,180)}`)}
    if(!response.ok)throw new Error(data.error || `${response.status} request failed`);
    return data;
  };
  const newKey = () => {if(!requestKey)requestKey=crypto.randomUUID().replaceAll('-','');return requestKey;};
  const input = (type, cls, val, min, max) => {const e=document.createElement('input');e.type=type;e.className=cls;e.value=String(val);if(min!==undefined)e.min=String(min);if(max!==undefined)e.max=String(max);e.step='0.01';return e;};
  async function orders() {
    try {
      const result=await api('/api/v1/cutting/lay-drafts'); const options=$('sop2-order');
      for(const o of result.orders || []){const el=document.createElement('option');el.value=o.intake_id;el.textContent=`${o.po_number} · ${o.style_code} · ${numeric(o.ordered_qty)} units`;options.appendChild(el)}
      status(`Loaded ${(result.orders||[]).length} recent staged orders. Choose one for source-backed control.`);
    } catch(err){status(`Unable to load buyer order register: ${err.message}`,true)}
  }
  function displayBoard(d){
    $('m-version').textContent=d.target_version ? `v${d.target_version}` : 'Not frozen';
    const lines=d.lines || [];let planned=0,signed=0,balance=0;
    const table=$('sop2-ledger');table.replaceChildren();
    for(const line of lines){planned+=Number(line.draft_planned);signed+=Number(line.signed_released_equivalent);if(line.frozen_target !== undefined)balance+=Number(line.balance_after_signed_release);
      const row=document.createElement('tr');
      [line.color_code,line.size_code,format(line.buyer_ordered),line.frozen_target===undefined?'—':format(line.frozen_target),format(line.draft_planned),format(line.operator_reported),format(line.signed_released_equivalent),line.frozen_target===undefined?'—':format(line.balance_after_signed_release)].forEach(v=>row.appendChild(td(v)));
      table.appendChild(row);
    }
    if(!lines.length){const row=document.createElement('tr');const cell=td('No buyer SKU records');cell.colSpan=8;row.appendChild(cell);table.appendChild(row)}
    $('m-planned').textContent=format(planned);$('m-released').textContent=format(signed);$('m-balance').textContent=d.target_version?format(balance):'Not frozen';
    const alerts=$('sop2-alerts');alerts.replaceChildren();const notes=(d.alerts||[]);
    if(!d.target_version)notes.push('CUT TARGET NOT FROZEN: exact per-size production balance cannot be certified.');
    if(!notes.length)notes.push('No quantity exceptions in the current source-backed register. Other evidence gates remain pending.');
    notes.forEach(message=>{const li=document.createElement('li');li.textContent=message;alerts.appendChild(li)});
    $('sop2-freeze').disabled=!!d.target_version;
    status(`PO ${d.po_number} · ${d.style} · ${lines.length} size/colour combinations. Signed releases are not final QC acceptance.`);
  }
  function updateTargetsFromSource(){
    const root=$('sop2-target-input');root.replaceChildren();
    for(const s of sourceLines){const tr=document.createElement('tr');tr.dataset.color=s.color_code;tr.dataset.size=s.size_code;
      tr.appendChild(td(s.color_code));tr.appendChild(td(s.size_code));tr.appendChild(td(format(s.ordered_qty)));
      const min=td(''),max=td(''),target=td('');min.replaceChildren(input('number','extra-min',2,0,100));max.replaceChildren(input('number','extra-max',4,0,100));
      target.replaceChildren(input('number','exact-target',Number(s.ordered_qty)+Math.ceil(Number(s.ordered_qty)*.02),Number(s.ordered_qty),Number(s.ordered_qty)*2));
      tr.append(min,max,target);root.appendChild(tr)
    }
  }
  function prefillSolver(){
    const sample={skus:sourceLines.map(s=>({key:`${s.color_code}|${s.size_code}`,ordered:Number(s.ordered_qty),min_extra_bp:200,max_extra_bp:400})),po_min_extra_bp:200,po_max_extra_bp:400,max_cuts:24,max_states:40000,markers:[]};
    $('sop2-solver-json').value=JSON.stringify(sample,null,2);
    $('sop2-solver-result').textContent='Enter approved marker IDs, length, width, table length, max plies, expected minutes and color|size ratios. Unverified marker geometry must not be represented as approved.';
  }
  async function load(){
    const id=$('sop2-order').value;if(!id){status('Select a purchase order.',true);return}selected=id;requestKey='';
    try{const [board,source]=await Promise.all([api(`/api/v1/cutting/sop2/control-tower?intake_id=${encodeURIComponent(id)}`),api(`/api/v1/cutting/lay-drafts?intake_id=${encodeURIComponent(id)}`)]);
      if(selected!==id)return;const groups = new Map();
      for(const line of source.skus || []){const key=`${line.color_code}\u0000${line.size_code}`;const current=groups.get(key);if(current)current.ordered_qty=Number(current.ordered_qty)+Number(line.ordered_qty);else groups.set(key,{color_code:line.color_code,size_code:line.size_code,ordered_qty:Number(line.ordered_qty)})}
      sourceLines=[...groups.values()];
      displayBoard(board);updateTargetsFromSource();prefillSolver();
    }catch(err){status(`Could not load source-linked cutting control tower: ${err.message}`,true)}
  }
  async function freeze(){
    if(!selected){status('Select a buyer order.',true);return}
    const lines=[];
    for(const row of $('sop2-target-input').querySelectorAll('tr')){
      lines.push({color_code:row.dataset.color,size_code:row.dataset.size,min_extra_bp:percentBP(row.querySelector('.extra-min').value),max_extra_bp:percentBP(row.querySelector('.extra-max').value),target:Number(row.querySelector('.exact-target').value)})
    }
    const body={intake_id:selected,request_key:newKey(),po_min_extra_bp:percentBP($('sop2-pomin').value),po_max_extra_bp:percentBP($('sop2-pomax').value),lines,actor_note:$('sop2-actor').value.trim(),reason:$('sop2-reason').value.trim()};
    try{const d=await api('/api/v1/cutting/sop2/targets',body);status(`Recorded immutable development target v${d.target.version}. This does not authorize physical cutting.`);requestKey='';await load()}
    catch(err){status(`Cannot freeze target: ${err.message}`,true)}
  }
  async function solve(){
    try{const request=JSON.parse($('sop2-solver-json').value);const result=await api('/api/v1/cutting/sop2/solve',request);$('sop2-solver-result').textContent=JSON.stringify(result,null,2)}
    catch(err){$('sop2-solver-result').textContent=`Optimization blocked: ${err.message}`}
  }
  $('sop2-load').addEventListener('click',load);
  $('sop2-freeze').addEventListener('click',freeze);
  $('sop2-solve').addEventListener('click',solve);
  $('sop2-order').addEventListener('change',()=>{selected='';sourceLines=[];requestKey=''});
  $('sop2-target-input').addEventListener('input',()=>{requestKey=''});
  ['sop2-pomin','sop2-pomax','sop2-reason','sop2-actor'].forEach(id=>$(id).addEventListener('input',()=>{requestKey=''}));
  // Author: Sunil Jain, 2026-11-12 — Discoverable ONLY on MacBook localhost.
  // The route itself separately requires development + loopback binding.
  if ((location.hostname === '127.0.0.1' || location.hostname === 'localhost') &&
      document.querySelector('.navlinks')) {
    const training = document.createElement('a');
    training.href = '/cutting/demo';
    training.textContent = '▶ Cutter guided demo';
    document.querySelector('.navlinks').prepend(training);
  }
  orders();
})();
