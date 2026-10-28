// Author: Sunil Jain, 2026-10-28 — Bind sourcing drafts to reviewed evidence; prevent wrong-PO/race saves.
(() => {
'use strict';
const $ = id => document.getElementById(id);
const add = (tag, value, parent, className) => {
  const element = document.createElement(tag);
  element.textContent = value ?? '';
  if (className) element.className = className;
  parent.append(element);
  return element;
};
const message = (value, type = '') => { $('message').textContent = value; $('message').className = 'message ' + type; };
async function json(response) {
  if (response.ok) return response.json();
  if (response.status === 401 || response.status === 403) throw new Error('Sign in to check material records.');
  throw new Error((await response.text()).trim().slice(0, 280) || `Request failed (${response.status})`);
}
const fetchData = url => fetch(url, { credentials: 'same-origin' }).then(json);
let selected = null;
let preview = null;
let generation = 0; // Invalidate outdated order, evidence, and history requests.
let pending = false;
function invalidatePreview() {
  generation += 1;
  preview = null;
  $('stage').disabled = true;
  return generation;
}
function downloadExcel(name) {
  const a = document.createElement('a');
  a.href = '/static/' + name;
  a.download = name;
  a.click();
}
function stockTemplate() { downloadExcel('agiparel_stock_check_template.xlsx'); }
function priceTemplate() { downloadExcel('agiparel_material_rates_template.xlsx'); }
function line(parent, m, mode) {
  const item = add('div', '', parent, 'material-row');
  const name = add('div', '', item, 'material-name');
  add('strong', `${m.material_sku} · ${m.category}`, name);
  add('small', mode === 'need' ? 'Saved buyer-order requirement · unverified for purchasing' : (m.procurement_owner === 'BUYER_OWNED' ? 'Buyer must supply · verify delivery' : 'Factory to purchase · unapproved'), name);
  if (mode !== 'need') add('small', `Submitted rate ${m.unit_price}/${m.uom} · ${m.price_source}`, name);
  const amounts = add('div', '', item, 'material-amounts');
  if (mode === 'need') {
    add('span', `Need: ${m.required} ${m.uom}`, amounts);
    add('span', 'Available: not checked', amounts, 'muted');
    add('span', 'Shortage: unknown', amounts, 'muted');
  } else {
    add('span', `Need: ${m.gross} ${m.uom}`, amounts);
    add('span', `Potential stock: ${m.conditional_surplus} ${m.uom}`, amounts);
    add('span', `Factory purchase gap: ${m.net_indent} ${m.uom}`, amounts, 'accent-value');
    if (m.procurement_owner !== 'BUYER_OWNED') add('small', `Possible factory cost: ${m.net_cost}`, amounts, 'muted');
    if (m.buyer_supply_gap && Number(m.buyer_supply_gap) > 0) add('small', `Buyer-supplied gap: ${m.buyer_supply_gap} ${m.uom}`, amounts, 'muted');
  }
}
function showNeed(order) {
  $('results').hidden = true;
  $('gate').textContent = 'DRAFT · HOLD';
  $('scenario-subtitle').textContent = 'Amounts come from the saved order. Confirm the material use before purchasing.';
  const materials = Array.isArray(order.materials) ? order.materials : [];
  $('need-state').textContent = `${materials.length} material${materials.length === 1 ? '' : 's'}`;
  $('need-sub').textContent = `${Number(order.ordered_qty).toLocaleString()} ordered pieces`;
  $('have-state').textContent = 'Not checked';
  $('have-sub').textContent = 'No eligible stock verified';
  $('short-state').textContent = 'Unknown';
  $('short-sub').textContent = 'Cannot assume zero inventory';
  const list = $('material-lines'); list.replaceChildren();
  if (!materials.length) add('p', 'This order has no reviewed material amounts yet. Add them in Buyer Order before calculating.', list, 'empty');
  else for (const material of materials) line(list, material, 'need');
}
function show(report) {
  $('results').hidden = false;
  $('scenario-title').textContent = `${report.po_number} · ${report.style_code}`;
  $('scenario-subtitle').textContent = 'Calculated from your checked Excel or CSV sheets. All material use remains on hold.';
  $('gate').textContent = 'DRAFT · HOLD';
  $('need-state').textContent = `${(report.materials || []).length} material${(report.materials || []).length === 1 ? '' : 's'}`;
  $('need-sub').textContent = `${Number(report.ordered_qty).toLocaleString()} ordered pieces`;
  $('have-state').textContent = 'Claims only';
  $('have-sub').textContent = 'Submitted sheets · not verified stores stock';
  const factoryGaps = (report.materials || []).filter(m => m.procurement_owner !== 'BUYER_OWNED' && Number(m.net_indent) > 0).length;
  const buyerGaps = (report.materials || []).filter(m => Number(m.buyer_supply_gap) > 0).length;
  $('short-state').textContent = `${factoryGaps} factory / ${buyerGaps} buyer`;
  $('short-sub').textContent = 'Lines with unmet quantities · not an approved purchase order';
  $('gross-cost').textContent = report.gross_material_cost;
  $('reuse-cost').textContent = report.conditional_reuse_value;
  $('net-cost').textContent = report.net_purchase_exposure;
  $('currency').textContent = report.currency;
  $('hash').textContent = report.scenario_sha256;
  const list = $('material-lines'); list.replaceChildren();
  for (const material of report.materials || []) line(list, material, 'calculated');
  const body = $('candidates'); body.replaceChildren();
  for (const candidate of report.candidates || []) {
    const tr = add('tr', '', body);
    const lot = add('td', candidate.lot_id, tr); add('small', candidate.source_reference, lot, 'source');
    add('td', candidate.material_sku, tr); add('td', candidate.submitted_qty, tr);
    const used = add('td', candidate.suggested_qty, tr); add('small', candidate.verdict.replaceAll('_', ' ').toLowerCase(), used, 'source');
    const checks = add('td', '', tr);
    for (const check of candidate.checks || []) {
      const badge = add('span', `${check.pass ? '✓' : '×'} ${check.criterion.replaceAll('_', ' ')}`, checks, check.pass ? 'check' : 'check fail');
      badge.title = check.detail;
    }
  }
  if (!body.children.length) { const row = add('tr', '', body); const td = add('td', 'No matching stock claims.', row); td.colSpan = 5; }
  const holds = $('holds'); holds.replaceChildren();
  for (const hold of report.holds || []) add('li', hold, holds);
  if (!holds.children.length) add('li', 'Even matching stock still needs operational authorization.', holds);
}
async function loadOrders() {
  const list = await fetchData('/api/v1/order-intake');
  const options = $('intake'); options.replaceChildren();
  const blank = add('option', 'Choose a saved buyer order', options); blank.value = '';
  const items = Array.isArray(list.items) ? list.items : [];
  for (const order of items) {
    const option = add('option', `${order.po_number} · ${order.style_code} · ${Number(order.ordered_qty).toLocaleString()} pieces`, options);
    option.value = order.id;
  }
  // Never silently select the latest buyer order: material purchasing is PO-specific.
  const requested = new URLSearchParams(location.search).get('intake');
  if (requested && items.some(item => item.id === requested)) {
    options.value = requested;
    await chooseOrder();
  } else {
    options.value = '';
    $('order-summary').textContent = items.length ? 'Choose the exact buyer PO before checking material.' : 'No saved buyer order yet. Start in Buyer Order.';
    if (requested) message('The linked buyer order was not found. Select a saved PO explicitly.', 'error');
    else message(items.length ? 'Choose a buyer order to begin.' : 'Save a buyer-order draft first.');
  }
}
async function chooseOrder() {
  const token = invalidatePreview();
  selected = null;
  $('results').hidden = true;
  $('material-lines').replaceChildren();
  $('history').textContent = 'Choose an order to see saved plans.';
  const id = $('intake').value;
  if (!id) {
    $('order-summary').textContent = 'Choose a saved buyer order.';
    $('need-state').textContent = '—';
    $('need-sub').textContent = 'From saved buyer order';
    $('have-state').textContent = 'Not checked';
    $('short-state').textContent = 'Unknown';
    message('Choose the exact buyer order to begin.');
    return;
  }
  $('order-summary').textContent = 'Loading the chosen buyer order…';
  try {
    const order = await fetchData('/api/v1/order-intake?id=' + encodeURIComponent(id));
    if (token !== generation || $('intake').value !== id) return;
    if (order.id !== id) throw new Error('Buyer order identity mismatch. Refresh and choose again.');
    selected = order;
    $('order-summary').textContent = `${order.po_number} · ${order.style_code} · ${Number(order.ordered_qty).toLocaleString()} pieces`;
    showNeed(order);
    message('Order loaded. Confirm the uploaded stock and rate sheets belong to this PO; availability is not verified.');
    await refresh(id, token);
  } catch (error) {
    if (token === generation) message(error.message, 'error');
  }
}
async function evaluate(action) {
  if (pending) return;
  const id = $('intake').value, stock = $('stock').files[0], prices = $('prices').files[0];
  if (!selected || selected.id !== id || !stock || !prices) {
    message('Choose the exact buyer order, a stock sheet, and a material-rate sheet.', 'error'); return;
  }
  if (!Array.isArray(selected.materials) || !selected.materials.length) {
    message('No material requirement exists for this PO. Complete the BOM in Buyer Order first.', 'error'); return;
  }
  if (action === 'stage' && (!preview || preview.intake !== id || preview.stock !== stock || preview.prices !== prices || !preview.sha)) {
    message('The order or evidence changed. Check materials again before saving.', 'error'); return;
  }
  const token = generation;
  const form = new FormData();
  form.append('intake_id', id); form.append('stock_file', stock); form.append('prices_file', prices);
  if (action === 'stage') form.append('preview_sha256', preview.sha);
  pending = true;
  $('preview').disabled = true; $('stage').disabled = true;
  try {
    message(action === 'preview' ? 'Checking stock claims and material costs…' : 'Saving the reviewed draft calculation…');
    const result = await json(await fetch('/api/v1/material-scenario?action=' + action, { method: 'POST', credentials: 'same-origin', body: form }));
    if (token !== generation || $('intake').value !== id || $('stock').files[0] !== stock || $('prices').files[0] !== prices) return;
    if (result.intake_id !== id) throw new Error('The calculation belongs to a different order. Review again.');
    show(result);
    if (action === 'preview') {
      if (!/^[0-9a-f]{64}$/.test(result.scenario_sha256 || '')) throw new Error('Missing evidence fingerprint; cannot save this draft.');
      preview = { intake: id, stock, prices, sha: result.scenario_sha256 };
      $('stage').disabled = false;
      message('Calculation ready. Review proposed gaps, sources and holds before saving. No stock reserved.', 'success');
    } else {
      preview = null;
      message('Draft plan saved. No buying or stock use has been approved.', 'success');
      await refresh(id, token);
    }
  } catch (error) {
    if (token === generation) {
      preview = null;
      $('stage').disabled = true;
      message(error.message, 'error');
    }
  } finally {
    pending = false;
    $('preview').disabled = false;
  }
}
async function refresh(id = $('intake').value, token = generation) {
  if (!id) { $('history').textContent = 'Choose a buyer order.'; return; }
  try {
    const list = await fetchData('/api/v1/material-scenario?intake=' + encodeURIComponent(id));
    if (token !== generation || $('intake').value !== id) return;
    const history = $('history'); history.replaceChildren();
    if (!(list.items || []).length) { history.textContent = 'No saved material plans yet.'; return; }
    for (const item of list.items) {
      const button = add('button', '', history, 'history-item'); button.type = 'button';
      add('strong', item.id, button);
      add('small', `${new Date(item.created_at).toLocaleString()} · Draft`, button);
      button.addEventListener('click', async () => {
        const historyToken = invalidatePreview();
        try {
          const report = await fetchData('/api/v1/material-scenario?id=' + encodeURIComponent(item.id));
          if (historyToken !== generation || $('intake').value !== id) return;
          if (report.intake_id !== id) throw new Error('Saved plan belongs to a different order.');
          show(report);
          message('Historical draft loaded for review. Recheck the uploaded sheets before saving a new plan.', 'success');
        } catch (error) { if (historyToken === generation) message(error.message, 'error'); }
      });
    }
  } catch (error) {
    if (token === generation) $('history').textContent = 'Could not load previous plans: ' + error.message;
  }
}
$('scenario-form').addEventListener('submit', event => { event.preventDefault(); void evaluate('preview'); });
$('stage').addEventListener('click', () => {
  if (!preview || preview.intake !== $('intake').value || preview.stock !== $('stock').files[0] || preview.prices !== $('prices').files[0]) {
    message('The files changed. Check materials again before saving.', 'error'); return;
  }
  void evaluate('stage');
});
$('intake').addEventListener('change', () => { void chooseOrder(); });
for (const id of ['stock', 'prices']) $(id).addEventListener('change', () => {
  invalidatePreview();
  if (selected) { showNeed(selected); message('Files changed. Check materials before saving.'); }
});
$('template-stock').addEventListener('click', stockTemplate);
$('template-prices').addEventListener('click', priceTemplate);
$('refresh').addEventListener('click', () => { void refresh(); });
void loadOrders().catch(error => message('Could not load saved buyer orders: ' + error.message, 'error'));
})();
