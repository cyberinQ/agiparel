// Author: Sunil Jain, 2026-10-28.
// Chapter 09: render trusted, read-only simulated facts; never infer missing factory KPIs.
(() => {
  'use strict';
  const $ = id => document.getElementById(id);
  const M = globalThis.AgiparelReportMetrics;
  const API = '/api/v1/intelligence/sim/';
  let last = null;
  let requestVersion = 0;
  const text = v => v == null ? '—' : String(v);
  function td(row, value) {
    const cell = document.createElement('td');
    cell.textContent = text(value);
    row.appendChild(cell);
  }
  function table(id, items, render) {
    const body = $(id);
    body.replaceChildren();
    for (const item of items || []) {
      const row = document.createElement('tr');
      render(row, item);
      body.appendChild(row);
    }
    if (!body.children.length) {
      const row = document.createElement('tr');
      const cell = document.createElement('td');
      cell.colSpan = body.closest('table')?.querySelectorAll('thead th').length || 1;
      cell.textContent = 'No usable records for this measure.';
      row.appendChild(cell);
      body.appendChild(row);
    }
  }
  function status(message) { $('status').textContent = message; }
  async function get(path) {
    const response = await fetch(API + path, {credentials:'same-origin', cache:'no-store'});
    const body = await response.json();
    if (!response.ok) throw Error(body.error || 'Unable to read factory records.');
    return body;
  }
  function reset() {
    requestVersion++;
    last = null;
    document.dispatchEvent?.(new CustomEvent('ag:report-cleared'));
    $('ask-aggie').disabled = true;
    $('capture').disabled = true;
    for (const id of ['metric-sewn','metric-accepted','metric-hold','metric-packed']) $(id).textContent = '—';
    for (const id of ['stages','residuals','intervals','operations']) $(id).replaceChildren();
    $('alerts').replaceChildren();
    const li = document.createElement('li'); li.textContent = 'Load a report to see what needs checking.';
    $('alerts').append(li);
    $('interval-exclusions').textContent = 'Load a report first.';
    $('snapshots').textContent = 'Choose a cutting plan first.';
    $('method').textContent = 'Load a report to read its methods.';
    $('evidence').textContent = 'No evidence loaded.';
  }
  async function catalog() {
    try {
      const result = await get('catalog');
      const select = $('lay');
      select.replaceChildren();
      const first = document.createElement('option');
      first.value = ''; first.textContent = 'Choose a cutting plan';
      select.append(first);
      for (const lay of result.lays || []) {
        const option = document.createElement('option');
        option.value = lay.lay_id;
        option.textContent = `${lay.intake_id} · cut ${lay.cut_no} · revision ${lay.revision}`;
        select.append(option);
      }
      status((result.lays || []).length ? 'Choose a cutting plan, then Show progress.' : 'No practice cutting plans found. Start with Chapter 05 — Cutting.');
    } catch (error) { status('Could not load cutting plans: ' + error.message); }
  }
  function showReport(report) {
    // Fail closed if the endpoint ever changes authority semantics.
    if (report.authority !== M.AUTHORITY) throw Error('This report has an unexpected evidence status. No metrics were shown.');
    const counts = M.summary(report);
    $('metric-sewn').textContent = M.showCount(counts.sewn);
    $('metric-accepted').textContent = M.showCount(counts.accepted);
    $('metric-hold').textContent = M.showCount(counts.hold);
    $('metric-packed').textContent = M.showCount(counts.packed);
    table('stages', report.stages, (row, item) => {
      td(row, item.name); td(row, item.count); td(row, item.unit); td(row, item.source);
    });
    table('residuals', report.residuals, (row, item) => {
      td(row, `${item.from} → ${item.to}`); td(row, item.unaccounted); td(row, item.status); td(row, item.explanation);
    });
    table('intervals', Object.entries(report.interval_stats || {}), (row, [label, stat]) => {
      td(row, label); td(row, stat.sample_count); td(row, stat.unknown_count);
      td(row, M.num(stat.p50_minutes)); td(row, M.num(stat.p90_minutes));
      td(row, M.num(stat.p95_minutes)); td(row, M.num(stat.cv));
      td(row, stat.rankable === true ? 'Comparable practice sample' : 'Too few samples for ranking');
    });
    table('operations', report.operation_stats, (row, item) => {
      td(row, item.operation); td(row, M.num(item.sam_seconds)); td(row, item.stats?.sample_count);
      td(row, M.num(item.stats?.p50_minutes)); td(row, M.num(item.stats?.p90_minutes));
      td(row, M.num(item.stats?.p95_minutes)); td(row, item.issue_events);
      const labels = {INSUFFICIENT_SAMPLE:'Too few samples', OBSERVED_HIGH_P90_VS_SAM:'Slow end over 1.5 × target', OBSERVED_WITHIN_1_5X_SAM:'Within practice comparison range'};
      td(row, labels[item.signal] || 'Unclassified');
    });
    const exclusions = [];
    for (const [label, reasons] of Object.entries(report.interval_exclusions || {})) {
      for (const [reason, count] of Object.entries(reasons || {})) exclusions.push(`${label}: ${count} · ${reason}`);
    }
    $('interval-exclusions').textContent = exclusions.join('\n') || 'No exclusions were recorded. This does not establish clock or shift-calendar quality.';
    $('alerts').replaceChildren();
    for (const item of M.findings(report)) {
      const li = document.createElement('li'); li.textContent = item; $('alerts').append(li);
    }
    $('method').textContent = (report.methodology || []).join(' ');
    $('evidence').textContent = `Authority: ${report.authority}. Source: ${report.evidence}. Generated: ${report.generated_at}. Cutting lay: ${report.lay_id}.`;
    $('ask-aggie').disabled = false;
    $('capture').disabled = false;
    status('Practice report loaded. Review the counts and checks below. This is not a production or payroll report.');
  }
  async function report() {
    const lay = $('lay').value;
    reset();
    if (!lay) return status('Choose a cutting plan first.');
    status('Checking saved practice records…');
    const version = requestVersion;
    const zone = $('timezone').value;
    try {
      const data = await get('report?lay_id=' + encodeURIComponent(lay) + '&timezone=' + encodeURIComponent(zone));
      // Never render a late reply from a previous lay/timezone selection.
      if (requestVersion !== version || $('lay').value !== lay || $('timezone').value !== zone) return;
      showReport(data);
      last = data;
      document.dispatchEvent?.(new CustomEvent('ag:report-loaded',{detail:data}));
      await snapshots();
    } catch (error) { if (requestVersion === version) status('Could not load report: ' + error.message); }
  }
  async function snapshots() {
    const lay = $('lay').value;
    if (!lay) return;
    try {
      const result = await get('snapshots?lay_id=' + encodeURIComponent(lay));
      if ($('lay').value !== lay) return;
      $('snapshots').textContent = (result.snapshots || [])
        .map(s => `${s.captured_at} · ${s.capture_id} · SHA-256 ${s.report_sha256}`).join('\n') || 'No saved reports for this plan yet.';
    } catch (error) { $('snapshots').textContent = 'Could not read earlier reports: ' + error.message; }
  }
  async function capture() {
    if (!last || last.lay_id !== $('lay').value) return status('Load the selected report first.');
    const lay = last.lay_id;
    const button = $('capture');
    button.disabled = true;
    const captureID = 'intel_' + crypto.randomUUID().replaceAll('-', '');
    try {
      const response = await fetch(API + 'capture', {
        method:'POST', credentials:'same-origin', headers:{'Content-Type':'application/json'},
        body:JSON.stringify({capture_id:captureID, lay_id:lay, timezone:$('timezone').value})
      });
      const data = await response.json();
      if (!response.ok) throw Error(data.error || 'Report could not be saved.');
      status(`Practice report saved · reference ${data.capture_id}. It is not an approved factory report.`);
      await snapshots();
    } catch (error) { status('Save failed: ' + error.message); }
    finally { button.disabled = !last; }
  }
  function askAggie() {
    if (!last) return;
    const launch = $('aggie-open');
    const panel = $('aggie-chat');
    const composer = $('aggie-input');
    if (!launch || !panel || !composer) return status('Aggie is not available on this page.');
    if (panel.hidden) launch.click();
    if (composer.value.trim()) return status('Aggie already has an unfinished message. Send or clear it before asking about this report.');
    composer.value = M.question(last);
    composer.dispatchEvent(new Event('input', {bubbles:true}));
    composer.focus();
    status('A source-labeled question is ready in Aggie. Check it and press Send when ready.');
  }
  $('refresh').addEventListener('click', report);
  $('capture').addEventListener('click', capture);
  $('ask-aggie').addEventListener('click', askAggie);
  $('lay').addEventListener('change', () => { reset(); status('Select Show progress to refresh this cutting plan.'); });
  $('timezone').addEventListener('change', () => { reset(); status('Time zone changed. Load the report again.'); });
  reset();
  catalog();
})();
