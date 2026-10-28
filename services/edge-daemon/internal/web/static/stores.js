// Author: Sunil Jain, 2026-10-28
// Chapter 04 roll observations: preserve audit evidence, identify shortages, and protect revisions.
(() => {
  'use strict';
  const $ = id => document.getElementById(id);
  const form = $('passport-form');
  const sourceSelect = $('source-document');
  const field = name => form.elements.namedItem(name);
  let records = [];
  let documents = [];
  let selectedRoll = '';
  let historyRequest = 0;
  let listRequest = 0;
  let hasMore = false;

  function status(message, kind = '') {
    $('message').textContent = message;
    $('message').className = 'message ' + kind;
  }
  async function api(url, init = {}) {
    const response = await fetch(url, {credentials: 'same-origin', ...init});
    let payload;
    try { payload = await response.json(); }
    catch { payload = {error: await response.text().catch(() => `HTTP ${response.status}`)}; }
    if (!response.ok) throw Error(payload.error || `HTTP ${response.status}`);
    return payload;
  }
  function line(label, value, parent) {
    const row = document.createElement('div');
    const title = document.createElement('span');
    const detail = document.createElement('b');
    title.textContent = label;
    detail.textContent = value ?? '—';
    row.append(title, detail);
    parent.append(row);
  }
  // The source inbox returns only recent documents. Keep an older linked source
  // selectable when revising a roll, rather than silently clearing provenance.
  function updateSourceOptions(requiredID = '') {
    const previous = requiredID || sourceSelect.value;
    sourceSelect.replaceChildren();
    const blank = document.createElement('option');
    blank.value = '';
    blank.textContent = 'No source attachment';
    sourceSelect.append(blank);
    for (const item of documents) {
      const opt = document.createElement('option');
      opt.value = item.id;
      opt.textContent = `${item.filename} (${item.status})`;
      sourceSelect.append(opt);
    }
    if (previous && !documents.some(x => x.id === previous)) {
      const older = document.createElement('option');
      older.value = previous;
      older.textContent = `Earlier linked document · ${previous}`;
      sourceSelect.append(older);
    }
    sourceSelect.value = previous;
  }
  function count() {
    const pass = records.filter(x => x.inspection_result === 'PASS').length;
    $('observed').textContent = records.length + (hasMore ? '+' : '');
    $('passed').textContent = pass + (hasMore ? '+' : '');
    $('held').textContent = (records.length - pass) + (hasMore ? '+' : '');
    const note = $('roll-count-note');
    if (note) note.textContent = hasMore
      ? 'First 500 recent rolls shown; counts are minimums, not factory-wide totals. Use the stock book for authoritative inventory.'
      : `${records.length} observed rolls displayed. These are not inventory balances.`;
  }
  function resetNewRoll() {
    historyRequest++;
    selectedRoll = '';
    form.reset();
    field('roll_id').readOnly = false;
    field('expected_revision').value = '0';
    updateSourceOptions('');
    $('revision-label').textContent = 'New receipt · revision 1';
    $('save').textContent = 'Save roll check';
    $('passport-history').textContent = 'Select a roll above to inspect revisions.';
    status('New roll check. Confirm the physical roll identity before saving.');
  }
  function render() {
    const host = $('roll-list');
    host.replaceChildren();
    count();
    if (!records.length) {
      const empty = document.createElement('div');
      empty.className = 'empty';
      empty.textContent = 'No roll receipts yet. Begin with one physical roll, or upload GRN documents first.';
      host.append(empty);
      return;
    }
    for (const p of records) {
      const button = document.createElement('button');
      button.type = 'button';
      button.className = 'roll';
      const top = document.createElement('div');
      top.className = 'roll-top';
      const title = document.createElement('strong');
      title.textContent = `${p.roll_id} · ${p.material_sku}`;
      const state = document.createElement('span');
      state.className = 'status ' + (p.inspection_result === 'PASS' ? 'pass' : p.inspection_result === 'REJECTED' ? 'rejected' : '');
      state.textContent = `REPORTED ${p.inspection_result}`;
      top.append(title, state);
      button.append(top);
      const grid = document.createElement('div');
      grid.className = 'roll-grid';
      line('Measured length', `${p.measured_length_m} m`, grid);
      const variance = Number(p.length_variance_m);
      const declared = Number(p.declared_length_m);
      const varianceLabel = Number.isFinite(variance) && Number.isFinite(declared) && declared > 0
        ? `${variance < 0 ? 'Short' : variance > 0 ? 'Over' : 'Exact'} ${Math.abs(variance).toFixed(3)} m (${(Math.abs(variance) / declared * 100).toFixed(2)}%)`
        : `${p.length_variance_m} m`;
      line('Measured vs declared', varianceLabel, grid);
      line('Usable width', `${p.usable_width_mm} mm`, grid);
      line('Location', p.bin_location, grid);
      line('Shade', p.shade_band, grid);
      line('Supplier lot', p.supplier_lot, grid);
      line('RFID EPC', p.epc_hex || 'Not assigned', grid);
      button.append(grid);
      if (variance !== 0 && Number.isFinite(variance)) {
        const caution = document.createElement('small');
        caution.className = 'roll-variance-note';
        caution.textContent = variance < 0
          ? 'Observed shortfall: verify the measurement and supplier claim; no automatic stock adjustment.'
          : 'Observed excess: verify against the supplier roll card; no automatic stock adjustment.';
        button.append(caution);
      }
      const foot = document.createElement('div');
      foot.className = 'roll-footer';
      const at = document.createElement('span');
      at.textContent = `Revision ${p.revision} · ${new Date(p.recorded_at).toLocaleString()}`;
      const ledger = document.createElement('b');
      ledger.textContent = p.ledger_status === 'NOT_IN_STOCK_LEDGER'
        ? 'Not in stock ledger'
        : `Ledger ${p.ledger_status} · ${p.ledger_remaining_m} m remaining · ${p.ledger_open_reservation_m} m open reservations`;
      foot.append(at, ledger);
      button.append(foot);
      button.addEventListener('click', () => choose(p));
      host.append(button);
    }
  }
  async function historyFor(id) {
    const request = ++historyRequest;
    const host = $('passport-history');
    host.textContent = 'Loading revision history…';
    try {
      const response = await api(`/api/v1/stores/passports?roll_id=${encodeURIComponent(id)}`);
      if (request !== historyRequest || selectedRoll !== id) return;
      host.replaceChildren();
      if (!response.items?.length) { host.textContent = 'No earlier observations'; return; }
      for (const p of response.items) {
        const row = document.createElement('div');
        row.className = 'revision';
        const heading = document.createElement('strong');
        heading.textContent = `Revision ${p.revision} · ${p.event_kind} · ${p.measured_length_m} m · ${p.inspection_result}`;
        row.append(heading);
        const note = document.createElement('small');
        note.textContent = `${new Date(p.recorded_at).toLocaleString()} · ${p.source_note}`;
        row.append(note);
        if (p.source_document_id) {
          const link = document.createElement('a');
          link.href = `/documents?document=${encodeURIComponent(p.source_document_id)}`;
          link.textContent = 'Source document ↗';
          row.append(link);
        }
        host.append(row);
      }
    } catch (err) {
      if (request === historyRequest) host.textContent = `Cannot load revisions: ${err.message}`;
    }
  }
  function choose(p) {
    if ($('save').disabled) return;
    selectedRoll = p.roll_id;
    for (const name of ['roll_id', 'epc_hex', 'material_sku', 'supplier_lot', 'shade_band', 'bin_location', 'declared_length_m', 'measured_length_m', 'usable_width_mm', 'inspection_result', 'defects', 'source_note']) {
      const el = field(name);
      if (el) el.value = p[name] ?? '';
    }
    // A correction to the identity requires a new receipt, not a revision that
    // silently changes which physical roll an old observation belongs to.
    field('roll_id').readOnly = true;
    updateSourceOptions(p.source_document_id || '');
    field('expected_revision').value = String(p.revision);
    $('revision-label').textContent = `Reinspection · revision ${p.revision + 1}`;
    $('save').textContent = 'Save next revision';
    status(`Editing ${p.roll_id}; earlier revisions remain preserved. RFID binding cannot be changed. PASS is operator-reported, not stock approval.`);
    void historyFor(p.roll_id);
    window.scrollTo({top: 0, behavior: 'smooth'});
  }
  async function refresh() {
    const request = ++listRequest;
    try {
      const [passports, authority] = await Promise.all([
        api('/api/v1/stores/passports'), api('/api/v1/stores/authority')
      ]);
      if (request !== listRequest) return;
      records = passports.items || [];
      hasMore = Boolean(passports.has_more);
      $('verified').textContent = authority.inspected_pass_in_ledger;
      $('reserved').textContent = authority.open_reservations;
      render();
    } catch (err) {
      if (request !== listRequest) return;
      status(`Cannot load stores: ${err.message}`, 'error');
      $('roll-list').textContent = 'Roll records could not be loaded. Refresh or ask your supervisor.';
    }
  }
  async function loadDocuments() {
    try {
      const data = await api('/api/v1/documents');
      documents = data.items || [];
      updateSourceOptions(sourceSelect.value);
    } catch { /* A source is optional for an observation, not for signed admission. */ }
  }
  form.addEventListener('submit', async event => {
    event.preventDefault();
    const payload = Object.fromEntries(new FormData(form));
    if (selectedRoll && payload.roll_id !== selectedRoll) {
      status('Roll identity changed. Start a new roll check instead of rewriting the selected roll.', 'error');
      return;
    }
    payload.usable_width_mm = Number(payload.usable_width_mm);
    payload.expected_revision = Number(payload.expected_revision);
    payload.epc_hex = (payload.epc_hex || '').trim().toUpperCase();
    const button = $('save');
    button.disabled = true;
    $('new-roll').disabled = true;
    status('Saving the roll check…');
    try {
      const saved = await api('/api/v1/stores/passports', {method: 'POST', headers: {'Content-Type': 'application/json'}, body: JSON.stringify(payload)});
      resetNewRoll();
      status(`Roll ${saved.passport.roll_id} saved as an observation. It is not approved stock.`, 'success');
      await refresh();
    } catch (err) {
      status(`Cannot save roll check: ${err.message}. Check for a concurrent reinspection and refresh the roll before retrying.`, 'error');
    } finally {
      button.disabled = false;
      $('new-roll').disabled = false;
    }
  });
  $('new-roll').addEventListener('click', resetNewRoll);
  $('refresh').addEventListener('click', () => void refresh());
  void Promise.all([refresh(), loadDocuments()]);
})();
