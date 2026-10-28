// Author: Sunil Jain, 2026-10-28
// Chapter 04 non-authoritative roll matching: prevent stale-order views and false zero demand.
(() => {
  'use strict';
  const $ = id => document.getElementById(id);
  const make = (tag, text, css = '') => {
    const node = document.createElement(tag);
    node.textContent = text;
    if (css) node.className = css;
    return node;
  };
  const fmt = mm => (Number(mm) / 1000).toLocaleString(undefined, {minimumFractionDigits: 3, maximumFractionDigits: 3}) + ' m';
  const human = value => String(value || '').replaceAll('_', ' ').toLowerCase();
  let orders = [], history = [], key = '', keyBasis = '', generation = 0, refreshNumber = 0;

  function clearReport(message = 'Choose a staged buyer order to see possible fabric rolls.') {
    const host = $('pick-results');
    host.replaceChildren(make('p', message, 'empty'));
  }
  function materialView(mat) {
    const section = make('article', '', 'pick-material');
    section.append(make('h3', `${mat.material_sku} · ${mat.uom}`));
    const metrics = make('div', '', 'pick-metrics');
    const known = mat.demand_known === true || (mat.demand_known === undefined && !(mat.blockers || []).includes('UNIT_CONVERSION_REQUIRED'));
    const values = known ? [
      ['Gross need', fmt(mat.demand_mm)],
      ['Tentative roll coverage', fmt(mat.tentative_mm)],
      ['Uncovered planning gap', fmt(mat.uncovered_mm)]
    ] : [
      ['Gross need', 'Unknown'],
      ['Tentative roll coverage', 'Not calculated'],
      ['Uncovered planning gap', 'Unknown']
    ];
    for (const [label, value] of values) {
      const item = make('div', '');
      item.append(make('b', value), make('span', label));
      metrics.append(item);
    }
    section.append(metrics);
    if (!known) section.append(make('p', `The BOM uses ${mat.uom}. Convert and verify the consumption unit before calculating metres or purchasing.`, 'reservation-warning'));
    section.append(make('p', `Selected shade: ${mat.chosen_shade || 'none'} · No cross-shade blending`, 'sub'));
    const rolls = make('ul', '', 'pick-rolls');
    for (const roll of mat.tentative_picks || []) {
      const item = make('li', '');
      item.append(make('b', `${roll.roll_id} · `), make('span', `${fmt(roll.suggested_mm)} · width ${roll.usable_width_mm} mm · ${roll.shade} · tentative`));
      rolls.append(item);
    }
    if (!rolls.childNodes.length) rolls.append(make('li', 'No eligible roll selection'));
    section.append(rolls);
    if ((mat.excluded_rolls || []).length) {
      const excluded = make('details', '');
      excluded.append(make('summary', `Show ${mat.excluded_rolls.length} excluded ledger rolls`));
      const list = make('ul', '', 'pick-holds');
      for (const roll of mat.excluded_rolls) list.append(make('li', `${roll.roll_id}: ${human(roll.reason)}`));
      excluded.append(list);
      section.append(excluded);
    }
    const blockers = make('ul', '', 'pick-holds');
    for (const warning of mat.blockers || []) blockers.append(make('li', human(warning)));
    section.append(blockers);
    return section;
  }
  function showReport(snapshot, historical = false, savedAt = '') {
    if (!snapshot?.materials || !snapshot.order) { clearReport('No proposal selected.'); return; }
    const host = $('pick-results');
    host.replaceChildren();
    host.append(make('div', `${historical ? 'SAVED HISTORICAL PROPOSAL' : 'NEW DRAFT PROPOSAL'} / HOLD · ${snapshot.order.po_number} · ${snapshot.order.style_code} · ${snapshot.order.ordered_qty.toLocaleString()} garments`, 'pick-notice'));
    if (historical) host.append(make('p', `Captured ${new Date(savedAt).toLocaleString()} · Historical snapshot only, NOT current stock availability.`, 'reservation-warning'));
    for (const material of snapshot.materials) host.append(materialView(material));
    const warning = make('div', '', 'pick-notice');
    warning.append(make('b', 'Still requires independent verification'));
    const list = make('ul', '');
    for (const text of snapshot.warnings || []) list.append(make('li', text));
    warning.append(list);
    host.append(warning);
  }
  function showHistory() {
    const host = $('pick-history');
    host.replaceChildren();
    if (!history.length) { host.append(make('p', 'No saved pick proposals yet.', 'empty')); return; }
    for (const entry of history) {
      const item = make('div', '', 'pick-history-item');
      item.append(make('strong', `${entry.snapshot.order.po_number} · ${entry.state}`));
      item.append(make('small', `${new Date(entry.created_at).toLocaleString()} · ${entry.snapshot.materials.length} fabric materials`));
      const button = make('button', 'Review saved proposal');
      button.type = 'button';
      button.addEventListener('click', () => showReport(entry.snapshot, true, entry.created_at));
      item.append(button);
      host.append(item);
    }
  }
  function invalidateOrderView() {
    generation++;
    key = '';
    keyBasis = '';
    clearReport();
  }
  async function refresh() {
    const seq = ++refreshNumber;
    try {
      const response = await fetch('/api/v1/stores/pick-proposals', {credentials: 'same-origin'});
      if (!response.ok) throw Error(`HTTP ${response.status}`);
      const data = await response.json();
      if (seq !== refreshNumber) return;
      orders = (data.orders || []).filter(order => order.status === 'PROPOSED');
      history = data.proposals || [];
      const select = $('pick-order');
      const previous = select.value;
      select.replaceChildren();
      const empty = make('option', 'Select a staged buyer order');
      empty.value = '';
      select.append(empty);
      for (const order of orders) {
        const item = make('option', `${order.po_number} · ${order.style_code} · ${order.ordered_qty.toLocaleString()} pcs`);
        item.value = order.intake_id;
        select.append(item);
      }
      if (orders.some(order => order.intake_id === previous)) select.value = previous;
      else if (previous) invalidateOrderView();
      showHistory();
      $('pick-feedback').textContent = `${orders.length} staged orders available. Saved proposals remain HOLD and never reserve stock.`;
    } catch (err) {
      if (seq === refreshNumber) $('pick-feedback').textContent = `Unable to load staged orders or proposals: ${err.message}. Confirm store migration 0018.`;
    }
  }
  $('pick-refresh').addEventListener('click', () => void refresh());
  $('pick-order').addEventListener('change', invalidateOrderView);
  $('pick-note').addEventListener('input', () => { key = ''; keyBasis = ''; });
  $('pick-form').addEventListener('submit', async event => {
    event.preventDefault();
    const orderID = $('pick-order').value;
    const note = $('pick-note').value.trim();
    if (!orders.some(order => order.intake_id === orderID) || note.length < 12) {
      $('pick-feedback').textContent = 'Choose a staged order and enter a note (at least 12 characters).';
      return;
    }
    const basis = JSON.stringify([orderID, note]);
    if (!key || basis !== keyBasis) { key = crypto.randomUUID(); keyBasis = basis; }
    const requestKey = key;
    const requestGeneration = generation;
    const button = $('pick-submit');
    button.disabled = true;
    $('pick-feedback').textContent = 'Calculating a non-authoritative snapshot from the protected ledger…';
    try {
      const response = await fetch('/api/v1/stores/pick-proposals', {
        method: 'POST', credentials: 'same-origin', headers: {'Content-Type': 'application/json'},
        body: JSON.stringify({intake_id: orderID, request_key: requestKey, note})
      });
      const data = await response.json();
      if (!response.ok) throw Error(data.error || `HTTP ${response.status}`);
      if (requestGeneration === generation && $('pick-order').value === orderID) {
        showReport(data.proposal.snapshot);
        if (key === requestKey) { key = ''; keyBasis = ''; $('pick-note').value = ''; }
      }
      await refresh();
      if (requestGeneration === generation && $('pick-order').value === orderID)
        $('pick-feedback').textContent = `${data.replayed ? 'Existing' : 'New'} ${data.proposal.state} draft saved. No stock was reserved or issued.`;
    } catch (err) {
      if (requestGeneration === generation) $('pick-feedback').textContent = `Cannot save proposal: ${err.message}. Retry unchanged input with the same request key.`;
    } finally { button.disabled = false; }
  });
  void refresh();
})();
