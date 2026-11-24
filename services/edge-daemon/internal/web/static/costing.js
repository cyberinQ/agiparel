// Author: Sunil Jain, CyberinQ LLC — Full S1–S5 Costing & Planning Preview controller
(() => {
  'use strict';
  const $ = id => document.getElementById(id);
  const today = new Date();
  const iso = d => d.toISOString().slice(0, 10);
  const addDays = (d, n) => { const x = new Date(d); x.setDate(x.getDate() + n); return x; };

  $('order-date').value = iso(today);
  $('ship-date').value = iso(addDays(today, 65));
  $('tent-date').value = iso(addDays(today, 10));
  $('valid-date').value = iso(addDays(today, 14));

  let state = { sheetId: null, versionId: null, verdict: 'PENDING', stale: true, lastPreview: null, lastPrice: null, scenarios: [] };

  function setStage(stage) {
    document.querySelectorAll('#stage-pills .stage-pill').forEach(p => p.classList.toggle('active', p.dataset.stage === stage));
  }

  function markStale() {
    state.stale = true;
    $('preview-card').classList.add('preview-stale');
    $('stale-banner').textContent = 'Stale – recompute preview before Submit';
    $('btn-submit').disabled = true;
  }

  document.addEventListener('input', e => {
    if (e.target.closest('#tab-matrix, #tab-bom, #tab-ob, #tab-other') || ['unit-id','buyer-id','style-id','po-ref','order-date','ship-date','tent-date','valid-date','ob-id','margin-pct','quote-currency','fx-rate'].includes(e.target.id)) {
      markStale();
    }
  });

  document.querySelectorAll('.tab-btn').forEach(btn => {
    btn.addEventListener('click', () => {
      document.querySelectorAll('.tab-btn').forEach(b => b.classList.remove('active'));
      document.querySelectorAll('.tab-pane').forEach(p => p.hidden = true);
      btn.classList.add('active');
      $(btn.dataset.tab).hidden = false;
      if (btn.dataset.tab === 'tab-s3') setStage('S3');
    });
  });

  document.addEventListener('click', e => {
    if (e.target.classList.contains('rm-row')) {
      const tbody = e.target.closest('tbody');
      if (tbody && tbody.children.length > 1) { e.target.closest('tr').remove(); markStale(); }
    }
  });

  $('add-matrix-row').addEventListener('click', () => {
    const tr = document.createElement('tr');
    tr.innerHTML = '<td><input value="NAVY" /></td><td><input value="36/30" /></td><td><input placeholder="UPC" /></td><td><input type="number" value="1000" /></td><td><button type="button" class="danger rm-row">×</button></td>';
    $('matrix-body').appendChild(tr); markStale();
  });

  $('add-bom-row').addEventListener('click', () => {
    const tr = document.createElement('tr');
    tr.innerHTML = '<td><input placeholder="ITEM-SKU" /></td><td><select><option value="FABRIC">FABRIC</option><option value="TRIM" selected>TRIM</option></select></td><td><input value="SUP-03" /></td><td><input type="number" step="0.01" value="1.00" /></td><td><input value="PCS" /></td><td><input type="number" step="0.1" value="2" /></td><td><input type="number" step="0.01" value="0.15" /></td><td><input type="number" value="0" /></td><td><input type="number" value="14" /></td><td><button type="button" class="danger rm-row">×</button></td>';
    $('bom-body').appendChild(tr); markStale();
  });

  $('add-ob-row').addEventListener('click', () => {
    const tr = document.createElement('tr');
    const seq = ($('ob-body').children.length + 1) * 10;
    tr.innerHTML = `<td><input value="OP-${seq}" /></td><td><input type="number" value="${seq}" /></td><td><input value="SNLS_LOCKSTITCH" /></td><td><select><option value="A">A</option><option value="B" selected>B</option><option value="C">C</option></select></td><td><input type="number" step="0.01" value="0.50" /></td><td><input type="checkbox" /></td><td><button type="button" class="danger rm-row">×</button></td>`;
    $('ob-body').appendChild(tr); markStale();
  });

  $('add-cost-row').addEventListener('click', () => {
    const tr = document.createElement('tr');
    tr.innerHTML = '<td><input value="PACKING" /></td><td><select><option value="RATE">RATE</option><option value="MANUAL" selected>MANUAL</option></select></td><td><input value="" /></td><td><input type="number" step="0.01" value="0.20" /></td><td><input placeholder="Required note" /></td><td><button type="button" class="danger rm-row">×</button></td>';
    $('cost-body').appendChild(tr); markStale();
  });

  function readMatrix() {
    return Array.from($('matrix-body').querySelectorAll('tr')).map(tr => {
      const inputs = tr.querySelectorAll('input');
      return { colour: inputs[0].value.trim(), size: inputs[1].value.trim(), upc: inputs[2].value.trim(), qty: Number(inputs[3].value) || 0 };
    });
  }

  function readBOM() {
    const now = new Date().toISOString();
    return Array.from($('bom-body').querySelectorAll('tr')).map(tr => {
      const inputs = tr.querySelectorAll('input');
      const cat = tr.querySelector('select').value;
      return {
        item_id: inputs[0].value.trim(),
        category: cat,
        supplier_id: inputs[1].value.trim(),
        raw_consumption: inputs[2].value.trim(),
        uom: inputs[3].value.trim(),
        purchase_unit_multiple: 1,
        wastage_pct: Number(inputs[4].value) || 0,
        unit_price: Number(inputs[5].value) || 0,
        price_currency: $('quote-currency').value,
        avail_unreserved_stock: Number(inputs[6].value) || 0,
        stock_snapshot_at: now,
        lead_time_days: Number(inputs[7].value) || 0
      };
    });
  }

  function readOB() {
    return Array.from($('ob-body').querySelectorAll('tr')).map(tr => {
      const inputs = tr.querySelectorAll('input');
      const grade = tr.querySelector('select').value;
      return {
        operation_id: inputs[0].value.trim(),
        sequence: Number(inputs[1].value) || 10,
        machine_type_id: inputs[2].value.trim(),
        skill_grade: grade,
        sam: Number(inputs[3].value) || 0,
        is_critical: inputs[4].checked
      };
    });
  }

  function readCostLines() {
    return Array.from($('cost-body').querySelectorAll('tr')).map(tr => {
      const inputs = tr.querySelectorAll('input');
      const basis = tr.querySelector('select').value;
      return {
        cost_type: inputs[0].value.trim(),
        basis,
        rate_ref: inputs[1].value.trim(),
        amount_per_pc: Number(inputs[2].value) || 0,
        note: inputs[3].value.trim()
      };
    });
  }

  $('btn-save-draft').addEventListener('click', async () => {
    setStage('S2');
    $('status-msg').textContent = 'Validating V1–V6 rules & saving draft...';
    const payload = {
      unit_id: $('unit-id').value.trim(),
      buyer_id: $('buyer-id').value.trim(),
      style_id: $('style-id').value.trim(),
      po_ref: $('po-ref').value.trim(),
      currency: $('quote-currency').value,
      order_date: $('order-date').value,
      requested_ship_date: $('ship-date').value,
      tentative_confirmation_date: $('tent-date').value,
      quote_valid_until: $('valid-date').value,
      lines: readMatrix(),
      bom_lines: readBOM(),
      ob_id: $('ob-id').value.trim(),
      ob_version: 1,
      ob_state: 'RELEASED',
      ob_operations: readOB(),
      cost_lines: readCostLines(),
      margin_pct: Number($('margin-pct').value),
      fx_rate_ref: $('quote-currency').value + '-BASE',
      fx_rate: Number($('fx-rate').value) || 1.0
    };
    const res = await fetch('/api/v1/costing/sheets', {
      method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(payload)
    });
    if (!res.ok) { $('status-msg').textContent = 'Blocked: ' + await res.text(); return; }
    const data = await res.json();
    state.sheetId = data.sheet_id;
    state.versionId = data.version_id;
    state.lastPrice = data.price_summary;
    $('out-qty').textContent = data.matrix.total_qty.toLocaleString();
    $('out-cpm').textContent = '$' + Number(data.cpm_used).toFixed(4) + '/min (' + data.cpm_period + ')';
    $('out-mat').textContent = '$' + Number(data.price_summary.material_per_pc).toFixed(4);
    $('out-cm').textContent = '$' + Number(data.price_summary.cm_per_pc).toFixed(4);
    $('out-fob').textContent = '$' + Number(data.price_summary.fob_pc).toFixed(4);
    markStale();
    $('status-msg').textContent = 'Draft v' + data.version_no + ' saved. Run Planning Preview next.';
  });

  $('btn-run-preview').addEventListener('click', async () => {
    if (!state.versionId) { $('status-msg').textContent = 'Save draft first.'; return; }
    const res = await fetch('/api/v1/costing/preview', {
      method: 'POST', headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        version_id: state.versionId,
        peak_target_pcs_per_day: 2000,
        minutes_per_operator_per_day: 480,
        daily_ramp_pcs: [600, 1200, 2000],
        ob_operations: readOB(),
        owned_machines_by_type: { SNLS_LOCKSTITCH: 20, WAISTBAND_ATTACH: 6 },
        pool_operators_by_grade: { A: 30, B: 25, C: 20 },
        daily_available_minutes: 15000,
        daily_committed_minutes: 4000
      })
    });
    if (!res.ok) { $('status-msg').textContent = 'Preview error: ' + await res.text(); return; }
    const data = await res.json();
    const p = data.preview;
    state.stale = false;
    state.verdict = p.verdict;
    state.lastPreview = p;
    $('preview-card').classList.remove('preview-stale');
    $('stale-banner').textContent = 'Fresh snapshot #' + p.input_snapshot_hash.slice(0, 10);
    const badge = $('verdict-badge');
    badge.className = 'verdict-chip verdict-' + p.verdict;
    badge.textContent = p.verdict;
    $('out-cut').textContent = p.latest_plan_cut_date.slice(0, 10);
    $('out-exf').textContent = p.earliest_feasible_ex_factory.slice(0, 10);
    $('out-slack').textContent = p.slack_days + ' days';
    const ul = $('reasons-list');
    ul.innerHTML = '';
    (p.reasons || ['All material, machine, skill and capacity checks GREEN']).forEach(r => {
      const li = document.createElement('li'); li.textContent = r; ul.appendChild(li);
    });
    $('s3-detail-host').textContent = `Latest Plan Cut: ${p.latest_plan_cut_date.slice(0, 10)} · Earliest Feasible Ex-Factory: ${p.earliest_feasible_ex_factory.slice(0, 10)} · Slack: ${p.slack_days} days · Verdict: ${p.verdict}`;
    $('replan-panel').hidden = (p.verdict === 'GREEN');
    $('btn-submit').disabled = (p.verdict !== 'GREEN');
    $('status-msg').textContent = p.verdict === 'GREEN'
      ? 'Preview GREEN — ready to submit.'
      : 'Verdict ' + p.verdict + ': D5 prohibits override. Complete joint re-plan below before Submit.';
  });

  $('btn-save-scenario').addEventListener('click', () => {
    if (!state.lastPreview || !state.lastPrice) { $('status-msg').textContent = 'Save draft and run preview before snapshotting a scenario.'; return; }
    if (state.scenarios.length >= 3) state.scenarios.shift();
    state.scenarios.push({
      ship: $('ship-date').value,
      unit: $('unit-id').value,
      fob: $('out-fob').textContent,
      verdict: state.verdict,
      slack: state.lastPreview.slack_days
    });
    const grid = $('scenario-compare-grid');
    grid.replaceChildren();
    state.scenarios.forEach((sc, idx) => {
      const card = document.createElement('div');
      card.className = 'panel-card';
      card.style.marginBottom = '0';
      card.innerHTML = `<strong>Scenario ${idx + 1}</strong><p style="margin:4px 0;">Ship: ${sc.ship} · Unit: ${sc.unit}</p><p style="margin:4px 0;">FOB: ${sc.fob} · Slack: ${sc.slack}d</p><span class="verdict-chip verdict-${sc.verdict}">${sc.verdict}</span>`;
      grid.appendChild(card);
    });
  });

  $('btn-record-replan').addEventListener('click', async () => {
    const participants = $('replan-participants').value.split(',').map(s => s.trim()).filter(Boolean);
    const reason = $('replan-reason').value.trim();
    const res = await fetch('/api/v1/costing/replan', {
      method: 'POST', headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ sheet_id: state.sheetId, trigger_kind: state.verdict, participants, changes: { agreed_replan: true }, reason })
    });
    if (!res.ok) { $('status-msg').textContent = 'Re-plan blocked: ' + await res.text(); return; }
    $('btn-submit').disabled = false;
    $('status-msg').textContent = 'Joint re-plan recorded. Submit enabled.';
  });

  $('btn-submit').addEventListener('click', async () => {
    const res = await fetch('/api/v1/costing/submit', {
      method: 'POST', headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ version_id: state.versionId, actor_id: $('active-role').value.toLowerCase() })
    });
    if (!res.ok) { $('status-msg').textContent = 'Submit blocked: ' + await res.text(); return; }
    setStage('S4');
    $('btn-submit').disabled = true;
    $('btn-approve').disabled = false;
    $('s4-reject-panel').hidden = false;
    $('status-msg').textContent = 'Submitted & frozen (V9). Ready for S4 Approval.';
  });

  $('btn-approve').addEventListener('click', async () => {
    const note = $('s4-decision-note').value.trim() || 'Approved with capacity hold';
    const res = await fetch('/api/v1/costing/decide', {
      method: 'POST', headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ version_id: state.versionId, approver_id: 'factory_head_01', decision: 'APPROVED', reason: note, device: window.innerWidth < 768 ? 'PHONE' : 'DESKTOP' })
    });
    if (!res.ok) { $('status-msg').textContent = 'Approval failed: ' + await res.text(); return; }
    setStage('S5');
    $('btn-approve').disabled = true;
    $('s4-reject-panel').hidden = true;
    $('btn-confirm').disabled = false;
    $('status-msg').textContent = 'Quote Approved (QUOTED) & Capacity Held until tentative confirmation date (D2).';
  });

  $('btn-reject').addEventListener('click', async () => {
    const note = $('s4-decision-note').value.trim();
    if (note.length < 20) { $('status-msg').textContent = 'Rejection requires a written reason (minimum 20 characters).'; return; }
    const res = await fetch('/api/v1/costing/decide', {
      method: 'POST', headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ version_id: state.versionId, approver_id: 'factory_head_01', decision: 'REJECTED', reason: note, device: window.innerWidth < 768 ? 'PHONE' : 'DESKTOP' })
    });
    if (!res.ok) { $('status-msg').textContent = 'Rejection failed: ' + await res.text(); return; }
    setStage('S2');
    $('btn-approve').disabled = true;
    $('s4-reject-panel').hidden = true;
    $('status-msg').textContent = 'Quote Rejected and returned to Draft for revision.';
  });

  $('btn-confirm').addEventListener('click', async () => {
    const res = await fetch('/api/v1/costing/confirm', {
      method: 'POST', headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ version_id: state.versionId, actor_id: $('active-role').value.toLowerCase() })
    });
    if (!res.ok) { $('status-msg').textContent = 'Confirm blocked: ' + await res.text(); return; }
    const data = await res.json();
    $('btn-confirm').disabled = true;
    $('status-msg').textContent = 'Order CONFIRMED! Immutable PlanningBaseline ' + data.baseline_id + ' handed off to Planning & Sourcing (FR-13).';
  });
})();
