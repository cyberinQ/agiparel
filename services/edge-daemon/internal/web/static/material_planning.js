// Author: Sunil Jain, 2026-10-27 — Show buyer-order demand before optional, evidence-backed sourcing calculations.
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
function download(name, content) {
  const url = URL.createObjectURL(new Blob([content], { type: 'text/csv;charset=utf-8' }));
  const link = document.createElement('a'); link.href = url; link.download = name; link.click();
  setTimeout(() => URL.revokeObjectURL(url), 1000);
}
function stockTemplate() {
  download('stock_check_blank.csv', 'lot_id,material_sku,uom,available_qty,fabric_spec,required_spec,gsm,required_gsm,gsm_tolerance_pct,shade_delta_e,shade_max_delta_e,usable_width_mm,marker_width_mm,inspection_status,age_days,max_age_days,buyer_escrow,held_for_po,source_reference\n');
}
function priceTemplate() {
  download('material_rates_blank.csv', 'material_sku,category,uom,unit_price,currency,procurement_owner,source_reference\n');
}
function line(parent, m, mode) {
  const item = add('div', '', parent, 'material-row');
  const name = add('div', '', item, 'material-name');
  add('strong', `${m.material_sku} · ${m.category}`, name);
  add('small', mode === 'need' ? 'Saved buyer-order requirement · unverified for purchasing' : `${m.procurement_owner} · proposed`, name);
  const amounts = add('div', '', item, 'material-amounts');
  if (mode === 'need') {
    add('span', `Need: ${m.required} ${m.uom}`, amounts);
    add('span', 'Available: not checked', amounts, 'muted');
    add('span', 'Shortage: unknown', amounts, 'muted');
  } else {
    add('span', `Need: ${m.gross} ${m.uom}`, amounts);
    add('span', `Potential stock: ${m.conditional_surplus} ${m.uom}`, amounts);
    add('span', `Estimated gap: ${m.net_indent} ${m.uom}`, amounts, 'accent-value');
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
  $('scenario-subtitle').textContent = 'Calculated from the two supplied CSV files. All material use remains on hold.';
  $('gate').textContent = 'DRAFT · HOLD';
  $('need-state').textContent = `${(report.materials || []).length} material${(report.materials || []).length === 1 ? '' : 's'}`;
  $('need-sub').textContent = `${Number(report.ordered_qty).toLocaleString()} ordered pieces`;
  $('have-state').textContent = 'Checked claims';
  $('have-sub').textContent = 'Conditional stock, not approved';
  $('short-state').textContent = 'See gaps below';
  $('short-sub').textContent = 'Not an approved purchase order';
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
  const requested = new URLSearchParams(location.search).get('intake');
  if (requested && items.some(item => item.id === requested)) options.value = requested;
  else if (items.length) options.value = items[0].id; // API returns latest staged intake first.
  if (options.value) await chooseOrder();
  else { $('order-summary').textContent = 'No saved buyer order yet. Start in Buyer Order.'; message('Save a buyer-order draft first.'); }
}
async function chooseOrder() {
  preview = null; selected = null; $('stage').disabled = true;
  $('results').hidden = true;
  const id = $('intake').value;
  if (!id) { $('order-summary').textContent = 'Choose a saved buyer order.'; $('material-lines').replaceChildren(); $('history').textContent = 'No order selected.'; return; }
  try {
    selected = await fetchData('/api/v1/order-intake?id=' + encodeURIComponent(id));
    $('order-summary').textContent = `${selected.po_number} · ${selected.style_code} · ${Number(selected.ordered_qty).toLocaleString()} pieces`;
    showNeed(selected);
    message('Order loaded. Material demand is a draft; stock and shortage have not been checked.');
    await refresh();
  } catch (error) { message(error.message, 'error'); }
}
async function evaluate(action) {
  const id = $('intake').value, stock = $('stock').files[0], prices = $('prices').files[0];
  if (!selected || !id || !stock || !prices) { message('Choose a buyer order and both checked CSV files.', 'error'); return; }
  const form = new FormData(); form.append('intake_id', id); form.append('stock_file', stock); form.append('prices_file', prices);
  $('preview').disabled = true; $('stage').disabled = true;
  try {
    message(action === 'preview' ? 'Checking stock claims and material costs…' : 'Saving the draft calculation…');
    const result = await json(await fetch('/api/v1/material-scenario?action=' + action, { method: 'POST', credentials: 'same-origin', body: form }));
    show(result);
    if (action === 'preview') {
      preview = { intake: id, stock, prices };
      $('stage').disabled = false;
      message('Calculation ready. Check the gaps below, then save if appropriate. Nothing was reserved.', 'success');
    } else {
      preview = null;
      message('Draft material plan saved. No buying or stock use has been approved.', 'success');
      await refresh();
    }
  } catch (error) { message(error.message, 'error'); } finally { $('preview').disabled = false; }
}
async function refresh() {
  if (!$('intake').value) { $('history').textContent = 'Choose a buyer order.'; return; }
  try {
    const list = await fetchData('/api/v1/material-scenario?intake=' + encodeURIComponent($('intake').value));
    const history = $('history'); history.replaceChildren();
    if (!(list.items || []).length) { history.textContent = 'No saved material plans yet.'; return; }
    for (const item of list.items) {
      const button = add('button', '', history, 'history-item'); button.type = 'button';
      add('strong', item.id, button);
      add('small', `${new Date(item.created_at).toLocaleString()} · Draft`, button);
      button.addEventListener('click', async () => {
        try { const report = await fetchData('/api/v1/material-scenario?id=' + encodeURIComponent(item.id)); show(report); message('Saved draft loaded. Stock and purchasing remain on hold.', 'success'); }
        catch (error) { message(error.message, 'error'); }
      });
    }
  } catch (error) { $('history').textContent = 'Could not load previous plans: ' + error.message; }
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
  preview = null; $('stage').disabled = true;
  if (selected) { showNeed(selected); message('Files changed. Check materials before saving.'); }
});
$('template-stock').addEventListener('click', stockTemplate);
$('template-prices').addEventListener('click', priceTemplate);
$('refresh').addEventListener('click', () => { void refresh(); });
void loadOrders().catch(error => message('Could not load saved buyer orders: ' + error.message, 'error'));
})();
