// Author: Sunil Jain, CyberinQ LLC — CEO Group Control Tower with Stage Rollups, Filters & Inline Shortfall Logging
(() => {
  'use strict';
  const $ = id => document.getElementById(id);
  let boardRows = [];
  let activeRow = null;

  function renderStageRollups(rows) {
    const stages = ['RAW_MATERIAL_ARRIVAL', 'CUTTING', 'SEWING', 'FINISHING', 'PACKING', 'SHIPMENT'];
    const labels = {
      RAW_MATERIAL_ARRIVAL: '1. Raw Material',
      CUTTING: '2. Cutting',
      SEWING: '3. Sewing',
      FINISHING: '4. Finishing',
      PACKING: '5. Packing',
      SHIPMENT: '6. Shipment'
    };
    const host = $('stage-rollup-host');
    host.replaceChildren();
    stages.forEach(st => {
      const subset = rows.filter(r => String(r.stage || '').toUpperCase().includes(st) || labels[st].toUpperCase().includes(String(r.stage || '').toUpperCase()));
      const planned = subset.reduce((s, r) => s + Number(r.planned_qty || 0), 0);
      const actual = subset.reduce((s, r) => s + Number(r.actual_qty || 0), 0);
      const variance = actual - planned;
      const div = document.createElement('div');
      div.className = 'stage-kpi';
      div.innerHTML = `<small>${labels[st]}</small><strong>${actual.toLocaleString()} / ${planned.toLocaleString()}</strong><span class="${variance < 0 ? 'neg' : 'pos'}">${variance >= 0 ? '+' : ''}${variance.toLocaleString()}</span>`;
      host.appendChild(div);
    });
  }

  function renderTable() {
    const fFilter = $('filter-factory').value;
    const sFilter = $('filter-stage').value;
    const onlyShort = $('filter-shortfalls').checked;
    const tbody = $('board-body');
    tbody.replaceChildren();

    const filtered = boardRows.filter(row => {
      if (fFilter && row.factory_name !== fFilter) return false;
      if (sFilter && row.stage !== sFilter) return false;
      if (onlyShort && Number(row.variance_qty) >= 0) return false;
      return true;
    });

    filtered.forEach(row => {
      const tr = document.createElement('tr');
      const cls = row.variance_qty < 0 ? 'neg' : 'pos';
      const reason = row.shortfall_reason_code
        ? row.shortfall_reason_code + (row.shortfall_note ? ' — ' + row.shortfall_note : '')
        : '—';
      [row.factory_name, row.po_id + ' · ' + row.style_id, row.plan_date, row.stage,
       row.planned_qty.toLocaleString(), row.actual_qty.toLocaleString(),
       (row.variance_qty >= 0 ? '+' : '') + row.variance_qty.toLocaleString(), reason
      ].forEach((val, idx) => {
        const td = document.createElement('td');
        if (idx === 6) td.className = cls;
        td.textContent = String(val);
        if (idx === 7 && row.variance_qty < 0) {
          const btn = document.createElement('button');
          btn.type = 'button';
          btn.className = 'mini';
          btn.style.marginLeft = '8px';
          btn.textContent = row.shortfall_reason_code ? 'Edit' : '+ Log Reason';
          btn.addEventListener('click', () => {
            activeRow = row;
            $('reason-target').textContent = `${row.factory_name} · PO ${row.po_id} · Stage ${row.stage} (Variance: ${row.variance_qty})`;
            $('reason-code').value = row.shortfall_reason_code || 'MATERIAL_DELAY';
            $('reason-note').value = row.shortfall_note || '';
            $('reason-editor').hidden = false;
          });
          td.appendChild(btn);
        }
        tr.appendChild(td);
      });
      tbody.appendChild(tr);
    });
  }

  async function refresh() {
    const res = await fetch('/api/v1/ceo/six-stage-board', { credentials: 'same-origin', cache: 'no-store' });
    if (!res.ok) return;
    const data = await res.json();

    const fHost = $('freshness-host');
    fHost.replaceChildren();
    (data.factory_freshness || []).forEach(f => {
      const div = document.createElement('div');
      div.className = 'fresh-pill' + (f.is_stale_over_15m ? ' fresh-stale' : '');
      const strong = document.createElement('strong');
      strong.textContent = f.factory_name;
      const small = document.createElement('small');
      small.style.display = 'block';
      small.textContent = 'Last sync: ' + new Date(f.last_sync_at).toLocaleString() + ' · ' + f.sync_status;
      div.append(strong, small);
      fHost.appendChild(div);
    });

    boardRows = data.six_stage_board || [];
    const factories = [...new Set(boardRows.map(r => r.factory_name).filter(Boolean))];
    const stages = [...new Set(boardRows.map(r => r.stage).filter(Boolean))];
    const prevF = $('filter-factory').value, prevS = $('filter-stage').value;
    $('filter-factory').innerHTML = '<option value="">All Factories</option>' + factories.map(f => `<option value="${f}">${f}</option>`).join('');
    $('filter-stage').innerHTML = '<option value="">All 6 Stages</option>' + stages.map(s => `<option value="${s}">${s}</option>`).join('');
    if (factories.includes(prevF)) $('filter-factory').value = prevF;
    if (stages.includes(prevS)) $('filter-stage').value = prevS;

    renderStageRollups(boardRows);
    renderTable();
  }

  $('reason-cancel').addEventListener('click', () => { $('reason-editor').hidden = true; activeRow = null; });
  $('reason-save').addEventListener('click', async () => {
    if (!activeRow) return;
    await fetch('/api/v1/ceo/shortfall-reason', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      credentials: 'same-origin',
      body: JSON.stringify({
        factory_id: activeRow.factory_id,
        po_id: activeRow.po_id,
        plan_date: activeRow.plan_date,
        stage: activeRow.stage,
        shortfall_reason_code: $('reason-code').value,
        shortfall_note: $('reason-note').value.trim()
      })
    }).catch(() => {});
    $('reason-editor').hidden = true;
    activeRow = null;
    await refresh();
  });

  ['filter-factory', 'filter-stage', 'filter-shortfalls'].forEach(id => $(id).addEventListener('change', renderTable));

  refresh();
  setInterval(refresh, 60000);
})();
