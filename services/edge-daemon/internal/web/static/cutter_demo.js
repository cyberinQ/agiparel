/* Author: Sunil Jain, 2026-11-12.
 * AGIPAREL MacBook guided operator demo. Browser-only simulated state; optional read-only advisory solver POST.
 */
(() => {
  'use strict';
  const publicDemo = document.body.dataset.demoMode === 'public';
  const M = window.CutterDemoModel;
  if (!M) throw new Error('Cutter training model did not load');
  const $ = id => document.getElementById(id);
  const names = ['Customer order', 'Choose cut plan', 'Record lays', 'Scan bundles', 'Ready for sewing'];
  const notes = ['Who needs what?', 'Save time & fabric', 'No over-cut', 'Complete parts only', 'Verified handoff'];
  let state = M.initial();
  let step = 0;
  let busy = false;
  const nums = n => Number(n).toLocaleString('en-US');
  function completed(i) {
    return i === 0 ? state.started : i === 1 ? state.optimized : i === 2 ? M.doneCutting(state) : i === 3 ? M.verifyBundle(state).ok : state.issued;
  }
  function escapeHTML(v) { return String(v).replace(/[&<>"']/g, c => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' })[c]); }
  const key = (x, cls = '') => `<span class="counter ${cls}">${x}</span>`;
  function stat(label, value, sub) { return `<div class="metric"><span>${label}</span><strong>${nums(value)}</strong><small>${sub}</small></div>`; }
  function pageHeader(kicker, title, subtitle, stage) {
    return `<div class="section-top"><div><span class="eyebrow">${kicker}</span><h2>${title}</h2><p>${subtitle}</p></div>${key('STEP ' + stage + ' / 5')}</div>`;
  }
  function presenter(text) { return `<div class="quote"><strong>Say to the cutter:</strong> “${text}”</div>`; }
  function actions(buttons) { return `<div class="actions">${buttons.join('')}</div>`; }
  function button(label, action, variant = 'primary', disabled = false, attr = '') {
    return `<button type="button" data-action="${action}" class="${variant}" ${disabled || busy ? 'disabled' : ''} ${attr}>${label}</button>`;
  }
  function view0() {
    return pageHeader('01 / UNDERSTAND THE JOB', 'A customer orders 400 navy T-shirts', 'Buyer sizes are fixed; the factory chooses additional cutting protection separately.', 1) +
      `<div class="cols"><div class="panel"><h3>Customer purchase order</h3><div class="qtygrid">${Object.entries(M.ORDER.buyer).map(([s,v])=>`<div class="qtybox"><small>SIZE ${s}</small><strong>${v}</strong></div>`).join('')}</div><p class="cardnote">Order: ${M.ORDER.id} · Style: ${M.ORDER.style} · Colour: Navy</p>${presenter('The customer wants 100 small, 200 medium and 100 large shirts. These are the numbers we must deliver.')}</div>
      <div class="panel muted"><h3>Teaching target: 2% extra</h3><table class="targettable"><thead><tr><th>Size</th><th>Buyer</th><th>Extra</th><th>Target</th></tr></thead><tbody>${['S','M','L'].map(s=>`<tr><td>${s}</td><td>${M.ORDER.buyer[s]}</td><td>+${M.ORDER.target[s]-M.ORDER.buyer[s]}</td><td>${M.ORDER.target[s]}</td></tr>`).join('')}<tr><td>Total</td><td>400</td><td>+8</td><td>408</td></tr></tbody></table><p class="cardnote">A hypothetical factory policy, not an approved or persisted frozen target.</p></div></div>` +
      actions([button(state.started ? 'Demo order loaded ✓' : 'Start with this sample order', 'begin', 'primary', state.started), button('Next: find a plan →', 'next', 'secondary', !state.started)]);
  }
  function view1() {
    const solver = state.optimizerSource === 'engine' ? `SOP-2 solver: ${state.optimizerEvidence?.proven ? '2-cut objective proven for supplied marker search' : 'feasible 2-cut candidate; optimality unproven'}. Roll-fit plan checked separately.` : state.optimizerSource === 'training' ? 'Preloaded teaching scenario; live optimizer was not used.' : 'Not evaluated. Training marker only; no CAD nesting or roll inspection has been verified.';
    return pageHeader('02 / PLAN BEFORE CUTTING', 'Two cuts, exactly 408 pieces', 'The marker places one small, two medium and one large shirt on each ply.', 2) +
      `<div class="cols"><div class="panel"><h3>Illustrative marker layout • 7.5 m</h3><div class="marker"><div class="piece">S<small>1 shirt</small></div><div class="piece">M<small>1 shirt</small></div><div class="piece">M<small>1 shirt</small></div><div class="piece">L<small>1 shirt</small></div></div><p class="cardnote">Conceptual ratio only, NOT a verified CAD pattern or production cutting instruction.</p><div class="laycards"><div class="laycard"><strong>Lay 1 · 51 plies</strong><small>51 S + 102 M + 51 L<br>Roll R-101 · Navy-A</small></div><div class="laycard"><strong>Lay 2 · 51 plies</strong><small>51 S + 102 M + 51 L<br>Roll R-102 · Navy-A</small></div></div>${presenter('Four garments fit on each ply. Two 51-ply cuts give us exactly 408 garments—no leftovers to cut.')}</div>
      <div class="panel muted"><h3>Choose the right fabric rolls</h3><div class="rollrow"><div class="roll"><strong>R-101</strong>400 m · Navy-A<small>For lay 1</small></div><div class="roll roll-b"><strong>R-102</strong>420 m · Navy-A<small>For lay 2 / extra flexibility</small></div></div><p style="margin-top:14px">Each lay consumes 382.5 m in the default 51-ply plan. We retain 1 m of same-roll reserve for recuts. Usable width 1.55 m; marker requires 1.50 m.</p><div class="proof ${state.optimized ? state.optimizerSource === 'engine' ? '' : 'warning' : 'neutral'}">${escapeHTML(solver)}</div></div></div>` +
      actions(publicDemo ? [button('Use preloaded training plan', 'training', 'primary', state.optimized), button('Next: record the lay →', 'next', 'secondary', !state.optimized)] : [button(busy ? 'Evaluating...' : 'Check with live SOP-2 optimizer', 'optimize', 'primary', state.optimized), button('Use preloaded training plan', 'training', 'secondary', state.optimized), button('Next: record the lay →', 'next', 'secondary', !state.optimized)]);
  }
  function view2() {
    const info = M.snapshot(state);
    const next = M.nextPlies(state);
    const targetRoll = M.ROLLS[Math.min(state.lays.length, 1)];
    const canLay = state.optimized && !info.doneCutting;
    const bal = M.balance(state);
    const rows = state.lays.map(l=>`<div class="laycard"><strong>SIMULATED LAY ${l.number}: ${l.plies} plies</strong><small>Roll ${l.rollID} · ${l.garmentCount} garments<br>Fabric ${l.usedM} m + reserve ${l.reserveM} m + end ${l.endM} m<br>Parts stay with one roll/ply genealogy</small></div>`).join('');
    return pageHeader('03 / CONTROL THE PHYSICAL PLAN', 'Count every ply. Never over-cut.', 'Report the number actually laid in the rehearsal; the balance recalculates automatically.', 3) +
      `<div class="cols"><div class="panel"><h3>Remaining garment balance • teaching only</h3><div class="qtygrid">${['S','M','L'].map(k=>`<div class="qtybox"><small>SIZE ${k}</small><strong>${bal[k]}</strong></div>`).join('')}</div><div class="progress"><span style="width:${Math.round(100 * info.simulatedCut / info.target)}%"></span></div><p class="laysummary">${info.simulatedCut} / 408 simulated pieces · ${state.lays.length}/2 simulated cuts · ${info.signedReleased} actual signed releases</p>
      ${info.doneCutting ? `<div class="proof">✓ Balance exactly zero. Fabric conservation also balances for both demo rolls. This is NOT signed cutting evidence.</div>` : `<div class="formfields"><label>Plies actually laid<input type="number" id="plies" inputmode="numeric" min="1" max="55" step="1" value="${canLay ? (state.lays.length === 0 ? 51 : next) : 51}" ${canLay ? '' : 'disabled'}></label><label>Fabric roll<select id="roll" ${canLay ? '' : 'disabled'}><option value="${targetRoll.id}">${targetRoll.id} · NAVY-A</option><option value="R-103">R-103 · NAVY-B (wrong shade)</option></select></label></div>${key(`Suggested next: ${next} plies`)}<p class="cardnote">Demo roll R-101 = 400 m; R-102 = 420 m; 1 m reserved per used roll.</p>`}
      ${presenter('If I spread fewer plies than planned, the next cut changes. But the final balance still must be exactly zero.')}</div><div class="panel muted"><h3>Simulated cutting record</h3><div class="laycards">${rows || `<div class="laycard"><strong>No lays reported yet</strong><small>Only an operator-reported simulation may appear here.</small></div>`}</div><p class="cardnote">Fabric conservation: roll length = marker consumption + same-roll reserve + end bit.</p><div class="proof neutral">${info.doneCutting ? info.fabric.map(f=>`${f.rollID}: ${f.issuedM} = ${f.laidM} + ${f.reserveM} + ${f.endM} m (Δ ${f.deltaM})`).join('<br>') : 'No physical issue or inventory consumption is claimed.'}</div></div></div>` +
      actions([button('Report simulated lay', 'lay', 'primary', !canLay), button('Try 48 plies on first lay', 'fortyeight', 'secondary', !canLay || state.lays.length !== 0), button('Next: scan garment parts →', 'next', 'secondary', !info.doneCutting)]);
  }
  function view3() {
    const info = M.snapshot(state);
    const verdict = M.verifyBundle(state);
    return pageHeader('04 / NO MIXED OR MISSING PARTS', 'Four parts make one complete shirt', 'Example: a bundle of 20 M-size shirts with front, back, left sleeve and right sleeve sub-bundles.', 4) +
      `<div class="cols"><div class="panel"><h3>Tap each part to simulate scanning</h3><div class="scanner">${M.PARTS.map(part=>`<button type="button" data-action="scan" data-id="${part.id}" class="scanpart ${state.scanned.includes(part.id) ? 'scanned' : ''}" ${!info.doneCutting || state.scanned.includes(part.id) || info.issued ? 'disabled' : ''}><span class="scanmark">${state.scanned.includes(part.id) ? '✓' : '▦'}</span><span><b>${part.label}</b><small>${part.tag} · 20 pcs · ${state.scanned.includes(part.id) ? 'SIMULATED SCAN OK' : 'Not yet scanned'}</small></span></button>`).join('')}</div><p class="cardnote">Training garment family: M001–M020, first 10 plies of lay 1, roll R-101. All part families carry matching garment numbers and shade.</p>${presenter('A shirt needs all four parts. We cannot send a half-complete bundle to sewing.')}</div>
      <div class="panel muted"><h3>Marriage check</h3><div class="bigstat">${info.scans}<small> / 4 parts scanned</small></div><div class="progress"><span style="width:${25*info.scans}%"></span></div><div class="proof ${verdict.ok ? '' : 'warning'}">${escapeHTML(verdict.reason)}</div><p class="cardnote">These are on-screen clicks, not NFC hardware events, signed identifiers or actual warehouse handoffs.</p></div></div>` +
      actions([button('Try issuing an incomplete kit', 'testissue', 'danger', !info.doneCutting || verdict.ok), button('Next: sewing handoff →', 'next', 'secondary', !verdict.ok)]);
  }
  function view4() {
    const info = M.snapshot(state);
    return pageHeader('05 / FINISH THE HANDOFF', 'Only complete bundles go to sewing', 'The cutter sees what is ready; the manager can independently verify real release evidence.', 5) +
      `<div class="cols"><div class="panel"><h3>20 M-size kits • sample bundle B-001</h3><div class="bigstat">${info.kitReady ? 20 : 0}<small> demo kits eligible</small></div><div class="proof ${info.kitReady ? '' : 'warning'}">${info.kitReady ? '✓ All four simulated part families match. Teaching kit is ready for a DEMO handoff.' : 'Not ready: balance or marriage check incomplete.'}</div><p class="cardnote">${info.issued ? 'The simulated handoff was recorded ONLY in browser memory.' : 'One last teaching-only click demonstrates a handoff, never a real sewing release.'}</p>${presenter('If one sleeve is missing, nothing moves. When everything matches, the bundle can proceed through the real signed handoff process.')}</div>
      <div class="panel muted"><h3>Do not confuse these numbers</h3><table class="targettable"><tbody><tr><td>Buyer ordered</td><td>400</td></tr><tr><td>Teaching cut target</td><td>408</td></tr><tr><td>Simulated cut / reported</td><td>${info.simulatedCut}</td></tr><tr><td>Demo kits handed off</td><td>${info.issued ? 20 : 0}</td></tr><tr><td><strong>REAL signed released</strong></td><td><strong>0</strong></td></tr></tbody></table><p class="cardnote">No operator authentication, NFC cryptography, signature, warehouse posting, or sewing receipt occurs in Cutter Mode.</p></div></div>` +
      actions([button(info.issued ? 'Training handoff complete ✓' : 'Simulate handoff of 20 kits', 'issue', 'primary', !info.kitReady || info.issued), button('Restart for another walkthrough', 'reset', 'secondary')]);
  }
  const pages = [view0, view1, view2, view3, view4];
  function render() {
    const k = M.snapshot(state);
    $('demo-metrics').innerHTML = stat('Customer ordered', k.ordered, 'buyer PO · sample') + stat('Demo cutting target', k.target, '+8 extra shirts') + stat('Simulated cut', k.simulatedCut, 'on-screen reports only') + stat('REAL signed released', 0, 'no factory evidence');
    $('demo-nav').innerHTML = names.map((name,i) => `<button type="button" class="stepnav ${step === i ? 'active' : ''} ${completed(i) ? 'complete' : ''}" data-action="go" data-step="${i}" aria-current="${step===i ? 'step' : 'false'}"><span class="navnum">${completed(i) ? '✓' : i+1}</span><span class="steptxt">${name}<small>${notes[i]}</small></span></button>`).join('');
    $('demo-content').innerHTML = pages[step]();
    const events = state.activity;
    $('demo-events').innerHTML = events.length ? events.map(e=>`<li><span class="event-number">#${e.sequence}</span><span class="event-kind ${e.kind}">${e.kind.toUpperCase()}</span>${escapeHTML(e.message)}</li>`).join('') : `<li>Nothing yet. Start the sample order in Step 1.</li>`;
    const alert = $('demo-alert');
    if (events.length) {
      alert.textContent = events[0].message;
      alert.className = 'alert ' + (events[0].kind === 'blocked' ? 'blocked' : events[0].kind === 'ok' ? 'ok' : '');
    } else {
      alert.textContent = 'Tap an exception to test a safety check. No factory transaction is created.';
      alert.className = 'alert';
    }
  }
  async function optimize() {
    if (publicDemo) { state.activity.unshift({ sequence:state.nextEvent++, kind:'blocked', message:'Public training never calls production APIs. Use the preloaded teaching plan.' }); render(); return; }
    if (busy) return;
    busy = true; render();
    try {
      const request = {
        skus: Object.keys(M.ORDER.buyer).map(s=>({ key:`NAVY|${s}`, ordered:M.ORDER.buyer[s], min_extra_bp:200, max_extra_bp:200 })),
        po_min_extra_bp:200, po_max_extra_bp:200, max_cuts:3, max_states:40000,
        markers: [{ id:M.MARKER.id, fabric_sku:'TRAINING-FAB-NAVY', shade:'NAVY-A', length_mm:7500,
          width_mm:1500, table_length_mm:9000, max_plies:55, minutes_per_cut:45,
          ratios:{'NAVY|S':1,'NAVY|M':2,'NAVY|L':1}, approved:true }]
      };
      const resp = await fetch('/api/v1/cutting/sop2/solve', { method:'POST', credentials:'same-origin', headers:{'Content-Type':'application/json'}, body:JSON.stringify(request), signal:AbortSignal.timeout(15000) });
      if (!resp.ok) throw Error(`SOP-2 optimizer returned HTTP ${resp.status}`);
      const data = await resp.json();
      M.markOptimized(state, data.result, 'engine');
      if (state.optimized) step = 2;
    } catch (error) {
      // A disconnected optimizer NEVER becomes a claimed solver success.
      state.activity.unshift({ sequence:state.nextEvent++, kind:'blocked', message:`Live optimizer unavailable (${String(error.message).slice(0,150)}). Choose the explicitly labeled preloaded teaching plan instead.` });
    } finally { busy = false; render(); }
  }
  function handle(action, target) {
    if (busy) return;
    switch (action) {
      case 'begin': M.begin(state); step=1; break;
      case 'training': M.markOptimized(state, null, 'training'); if(state.optimized)step=2; break;
      case 'next': if (completed(step) && step < 4) step++; break;
      case 'go': step = Math.max(0, Math.min(4, Number(target.dataset.step) || 0)); break;
      case 'fortyeight': $('plies').value = '48'; return;
      case 'lay': {
        const plies = Number($('plies').value);
        const rollID = $('roll').value;
        const out = M.applyLay(state, plies, rollID);
        if (out.ok && M.doneCutting(state)) step = 3;
        break;
      }
      case 'scan': {
        M.scanPart(state, target.dataset.id);
        if (M.verifyBundle(state).ok) step = 4;
        break;
      }
      case 'testissue': M.issue(state); break;
      case 'issue': M.issue(state); break;
      case 'exception': M.scenario(state, target.dataset.kind); break;
      case 'reset': state = M.initial(); step = 0; break;
      case 'print': window.print(); return;
      default: return;
    }
    render();
  }
  document.addEventListener('click', event => {
    const target = event.target.closest('button[data-action]');
    if (!target || target.disabled) return;
    if (target.dataset.action === 'optimize') { void optimize(); return; }
    handle(target.dataset.action, target);
  });
  render();
})();
