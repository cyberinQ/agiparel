/* Agiparel Batch 04: offline, read-only, entirely fictional demonstration.
   No network requests, credentials, database access, or write operations. */
(() => {
  'use strict';
  const stages = [
    {name:'Planning', short:'Buyer PO → release', carrier:'Buyer PO + approved BOM / marker references', reader:'Document intake / human approval', description:'Source documents become an executable, versioned plan. Unresolved BOM and production-feasibility questions must block authoritative release.'},
    {name:'Stores', short:'Roll reserved → issued', carrier:'Roll-core EPC with lot, shade and measured usable length', reader:'Roll inspection and issue/receipt RFID read points', description:'Allocate eligible fabric without cross-order borrowing. Two independently authenticated witnesses are required for physical material issuance.'},
    {name:'Cutting', short:'Ply → physical bundles', carrier:'Cut bundle EPC linked to marker, part, size, and actual consecutive ply range', reader:'Cutting table / bundling reader', description:'Conserve actual consumed roll length and create physical, genealogy-preserving sub-sub-bundles from the approved marker and lay.'},
    {name:'Sewing', short:'Bundles → assemblies', carrier:'Bundle identity transitioning to assembled garment identity', reader:'Operation-station RFID readers', description:'Credit only valid operations, with ordered assembly dependencies and exception evidence. Reader presence alone does not prove a finished piece.'},
    {name:'Quality', short:'Inspect → repair → pass', carrier:'Garment hangtag EPC bound to the assembled garment', reader:'Quality / finishing inspection station', description:'Record inspection, repair, re-inspection, and final garment identity with traceable disposition. Quality is not automatically passed.'},
    {name:'Dispatch', short:'Garments → cartons', carrier:'Garment EPC ↔ carton manifest identity', reader:'Packing and carton close-out read point', description:'Reconcile what was actually packed to garment identities, approved order quantities, and shipment manifest before dispatch.'}
  ];
  const orders = [
    {id:'MK6574J9', product:'Woven trousers · export sample workflow', quantity:600, flags:['Quality evidence not yet recorded','Packing scan not yet recorded'],
     timestamps:['2026-10-20T08:00:00Z','2026-10-20T13:00:00Z','2026-10-21T07:00:00Z','2026-10-22T12:00:00Z',null,null],
     refs:['PO-MK6574J9','ROLL-007 · 59 m inspected','CUT-MK-001 · Part 01 / ply 01–30','BND-MK-101 · Station 04',null,null],
     proofs:['Approved document snapshot (simulated)','Roll read + 2-person issue (simulated)','Bundle binding (simulated)','Operation completion event (simulated)',null,null]},
    {id:'GP240812', product:'Cotton shirts · completed demonstration journey', quantity:1200, flags:[],
     timestamps:['2026-10-14T07:00:00Z','2026-10-14T11:00:00Z','2026-10-15T06:00:00Z','2026-10-15T16:00:00Z','2026-10-16T09:00:00Z','2026-10-16T17:00:00Z'],
     refs:['PO-GP240812','ROLL-GP-055','CUT-GP-110 / ply 01–40','BND-GP-212','HT-GP-0001','CTN-GP-101'],
     proofs:['Plan release (simulated)','Roll issuance proof (simulated)','Cut genealogy (simulated)','Station evidence (simulated)','Quality pass (simulated)','Carton manifest (simulated)']},
    {id:'PO-4518', product:'Knit polo · material readiness demonstration', quantity:860, flags:['Marker awaiting release','Cutting not started'],
     timestamps:['2026-10-23T08:00:00Z','2026-10-23T15:00:00Z',null,null,null,null],
     refs:['PO-4518','ROLL-4518-002',null,null,null,null],
     proofs:['PO / BOM checks (simulated)','Inventory reservation (simulated)',null,null,null,null]}
  ];
  const byId = id => document.getElementById(id);
  const fmt = value => new Intl.NumberFormat('en-US').format(value);
  const element = (tag, className, text) => {
    const node = document.createElement(tag);
    if (className) node.className = className;
    if (text !== undefined) node.textContent = text;
    return node;
  };
  let activeOrder = orders[0];
  let activeStage = 0;
  const select = byId('order-select');
  orders.forEach(order => {
    const opt = document.createElement('option');
    opt.value = order.id;
    opt.textContent = `${order.id} · ${order.product.split(' · ')[0]}`;
    select.appendChild(opt);
  });
  select.addEventListener('change', () => {
    const match = orders.find(o => o.id === select.value);
    if (!match) return;
    activeOrder = match;
    activeStage = Math.min(activeStage, Math.max(0, match.timestamps.filter(Boolean).length - 1));
    render();
  });
  function duration(ms) {
    const hours = Math.round(ms / 3600000 * 10) / 10;
    if (hours >= 24) return `${(hours / 24).toFixed(1)} d`;
    return `${hours.toFixed(1)} h`;
  }
  function render() {
    const o = activeOrder;
    const completed = o.timestamps.filter(Boolean).length;
    const last = completed === stages.length;
    byId('order-title').textContent = o.id;
    byId('order-description').textContent = o.product;
    byId('order-badge').textContent = last ? 'Sample journey complete' : `Sample stage ${completed} of ${stages.length}`;
    byId('order-badge').classList.toggle('warning', !last);
    byId('metric-quantity').textContent = fmt(o.quantity);
    byId('metric-evidence').textContent = `${completed} / ${stages.length}`;
    byId('metric-alerts').textContent = String(o.flags.length);
    byId('metric-elapsed').textContent = completed >= 2 ? duration(Date.parse(o.timestamps[completed - 1]) - Date.parse(o.timestamps[0])) : '—';
    renderStages();
    renderDetail();
    renderDurations();
  }
  function renderStages() {
    const container = byId('journey-strip');
    container.replaceChildren();
    stages.forEach((stage, i) => {
      const finished = !!activeOrder.timestamps[i];
      const button = element('button', `journey-step${i === activeStage ? ' active' : ''}`);
      button.type = 'button';
      button.setAttribute('aria-pressed', String(i === activeStage));
      button.setAttribute('aria-label', `${stage.name}: ${finished ? 'simulated event recorded' : 'not recorded'}. Show stage details`);
      const top = element('span', 'step-top');
      top.append(element('span', '', `0${i + 1}`), element('span', `step-flag ${finished ? 'scanned' : 'pending'}`));
      button.append(top, element('strong', 'step-name', stage.name), element('span', 'step-sub', stage.short));
      button.addEventListener('click', () => { activeStage = i; renderStages(); renderDetail(); });
      container.appendChild(button);
    });
  }
  function renderDetail() {
    const i = activeStage, stage = stages[i], o = activeOrder, recorded = !!o.timestamps[i];
    byId('detail-kicker').textContent = `STAGE 0${i + 1} · ${recorded ? 'SIMULATED EVENT' : 'AWAITING EVIDENCE'}`;
    byId('detail-title').textContent = stage.name;
    byId('detail-status').textContent = recorded ? 'Sample evidence present' : 'No event in scenario';
    byId('detail-status').classList.toggle('pending', !recorded);
    byId('detail-description').textContent = stage.description;
    byId('detail-carrier').textContent = recorded && o.refs[i] ? o.refs[i] : stage.carrier;
    byId('detail-reader').textContent = stage.reader;
    byId('detail-proof').textContent = recorded ? o.proofs[i] : 'None in this sample — do not infer completion';
    byId('detail-foot').textContent = recorded ? `Illustrative event timestamp: ${new Date(o.timestamps[i]).toLocaleString('en-US', { timeZone:'UTC', dateStyle:'medium', timeStyle:'short' })} UTC · Not a live reader event.` : 'No read, signature, or completion event exists for this stage in the demonstration.';
  }
  function renderDurations() {
    const list = byId('duration-list');
    list.replaceChildren();
    const pairs = [];
    for (let i = 1; i < stages.length; i++) {
      if (activeOrder.timestamps[i] && activeOrder.timestamps[i - 1]) {
        pairs.push({ label:`${stages[i - 1].name} → ${stages[i].name}`, millis:Date.parse(activeOrder.timestamps[i]) - Date.parse(activeOrder.timestamps[i - 1]) });
      }
    }
    if (!pairs.length) {
      list.appendChild(element('p', '', 'The sample has fewer than two observed-stage timestamps. No elapsed interval can be calculated.'));
      return;
    }
    const maximum = Math.max(...pairs.map(p => p.millis), 1);
    pairs.forEach(p => {
      const row = element('div', `duration-row${p.millis === maximum && pairs.length > 1 ? ' long' : ''}`);
      const heading = element('div','duration-row-header');
      heading.append(element('strong','',p.label),element('span','',duration(p.millis)));
      const track = element('div','bar-track');
      const bar = element('div','bar-value');
      bar.style.width = `${Math.max(3,p.millis / maximum * 100)}%`;
      track.appendChild(bar);
      row.append(heading,track);
      list.appendChild(row);
    });
  }
  render();
})();
