// Author: Sunil Jain, CyberinQ LLC (Generated 2026-11-24)
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

  let state = { sheetId: null, versionId: null, verdict: 'PENDING', stale: true };

  function markStale() {
    state.stale = true;
    $('preview-card').classList.add('preview-stale');
    $('stale-banner').textContent = 'Stale – recompute preview before Submit';
    $('btn-submit').disabled = true;
  }

  document.querySelectorAll('input').forEach(inp => inp.addEventListener('input', markStale));

  function readMatrix() {
    return Array.from($('matrix-body').querySelectorAll('tr')).map(tr => {
      const inputs = tr.querySelectorAll('input');
      return {
        colour: inputs[0].value.trim(),
        size: inputs[1].value.trim(),
        upc: inputs[2].value.trim(),
        qty: Number(inputs[3].value) || 0
      };
    });
  }

  $('btn-save-draft').addEventListener('click', async () => {
    $('status-msg').textContent = 'Validating V1–V6 rules & saving draft...';
    const payload = {
      unit_id: $('unit-id').value.trim(),
      buyer_id: $('buyer-id').value.trim(),
      style_id: $('style-id').value.trim(),
      po_ref: $('po-ref').value.trim(),
      currency: 'USD',
      order_date: $('order-date').value,
      requested_ship_date: $('ship-date').value,
      tentative_confirmation_date: $('tent-date').value,
      quote_valid_until: $('valid-date').value,
      lines: readMatrix(),
      bom_lines: [
        { item_id: 'DENIM-STRETCH-12OZ', category: 'FABRIC', supplier_id: 'SUP-01', raw_consumption: '1.50', uom: 'YD', purchase_unit_multiple: 1, wastage_pct: 2, unit_price: 3.10, price_currency: 'USD', avail_unreserved_stock: 0, stock_snapshot_at: new Date().toISOString(), lead_time_days: 28 },
        { item_id: 'INNER-WAISTBAND', category: 'TRIM', supplier_id: 'SUP-02', raw_consumption: '1.34', uom: 'YD', purchase_unit_multiple: 1, wastage_pct: 3, unit_price: 0.45, price_currency: 'USD', avail_unreserved_stock: 0, stock_snapshot_at: new Date().toISOString(), lead_time_days: 18 }
      ],
      ob_id: $('ob-id').value.trim(),
      ob_version: 1,
      ob_state: 'RELEASED',
      ob_operations: [
        { operation_id: 'OP-10', sequence: 10, machine_type_id: 'SNLS_LOCKSTITCH', skill_grade: 'A', sam: 0.60, is_critical: false },
        { operation_id: 'OP-20', sequence: 20, machine_type_id: 'WAISTBAND_ATTACH', skill_grade: 'A', sam: 0.85, is_critical: true }
      ],
      cost_lines: [
        { cost_type: 'OVERHEAD', basis: 'RATE', rate_ref: 'OH-STD', amount_per_pc: 0.35, note: 'Standard unit overhead' }
      ],
      margin_pct: Number($('margin-pct').value),
      fx_rate_ref: 'USD-BASE',
      fx_rate: 1.0
    };
    const res = await fetch('/api/v1/costing/sheets', {
      method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(payload)
    });
    if (!res.ok) {
      $('status-msg').textContent = 'Blocked: ' + await res.text();
      return;
    }
    const data = await res.json();
    state.sheetId = data.sheet_id;
    state.versionId = data.version_id;
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
        ob_operations: [
          { operation_id: 'OP-10', sequence: 10, machine_type_id: 'SNLS_LOCKSTITCH', skill_grade: 'A', sam: 0.60, is_critical: false },
          { operation_id: 'OP-20', sequence: 20, machine_type_id: 'WAISTBAND_ATTACH', skill_grade: 'A', sam: 0.85, is_critical: true }
        ],
        owned_machines_by_type: { SNLS_LOCKSTITCH: 20, WAISTBAND_ATTACH: 6 },
        pool_operators_by_grade: { A: 30 },
        daily_available_minutes: 15000,
        daily_committed_minutes: 4000
      })
    });
    if (!res.ok) { $('status-msg').textContent = 'Preview error: ' + await res.text(); return; }
    const data = await res.json();
    const p = data.preview;
    state.stale = false;
    state.verdict = p.verdict;
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
    $('replan-panel').hidden = (p.verdict === 'GREEN');
    $('btn-submit').disabled = (p.verdict !== 'GREEN');
    $('status-msg').textContent = p.verdict === 'GREEN'
      ? 'Preview GREEN — ready to submit.'
      : 'Verdict ' + p.verdict + ': D5 prohibits override. Complete joint re-plan below before Submit.';
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
      body: JSON.stringify({ version_id: state.versionId, actor_id: 'merchandiser_01' })
    });
    if (!res.ok) { $('status-msg').textContent = 'Submit blocked: ' + await res.text(); return; }
    $('btn-submit').disabled = true;
    $('btn-approve').disabled = false;
    $('status-msg').textContent = 'Submitted & frozen (V9). Ready for S4 Approval.';
  });

  $('btn-approve').addEventListener('click', async () => {
    const res = await fetch('/api/v1/costing/decide', {
      method: 'POST', headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ version_id: state.versionId, approver_id: 'factory_head_01', decision: 'APPROVED', reason: 'Approved with capacity hold', device: 'DESKTOP' })
    });
    if (!res.ok) { $('status-msg').textContent = 'Approval failed: ' + await res.text(); return; }
    $('btn-approve').disabled = true;
    $('btn-confirm').disabled = false;
    $('status-msg').textContent = 'Quote Approved (QUOTED) & CapacityHeld until tentative confirmation date (D2).';
  });

  $('btn-confirm').addEventListener('click', async () => {
    const res = await fetch('/api/v1/costing/confirm', {
      method: 'POST', headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ version_id: state.versionId, actor_id: 'merchandiser_01' })
    });
    if (!res.ok) { $('status-msg').textContent = 'Confirm blocked: ' + await res.text(); return; }
    const data = await res.json();
    $('btn-confirm').disabled = true;
    $('status-msg').textContent = 'Order CONFIRMED! Immutable PlanningBaseline ' + data.baseline_id + ' handed off to Planning & Sourcing (FR-13).';
  });
})();
