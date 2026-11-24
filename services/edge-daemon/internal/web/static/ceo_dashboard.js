// Author: Sunil Jain, CyberinQ LLC (Generated 2026-11-24)
(() => {
  'use strict';
  const $ = id => document.getElementById(id);

  async function refresh() {
    const res = await fetch('/api/v1/ceo/six-stage-board', { credentials: 'same-origin', cache: 'no-store' });
    if (!res.ok) return;
    const data = await res.json();

    const fHost = $('freshness-host');
    fHost.replaceChildren();
    (data.factory_freshness || []).forEach(f => {
      const div = document.createElement('div');
      div.className = 'fresh-pill' + (f.is_stale_over_15m ? ' fresh-stale' : '');
      div.innerHTML = '<strong>' + f.factory_name + '</strong><br><small>Last sync: ' +
        new Date(f.last_sync_at).toLocaleString() + ' · ' + f.sync_status + '</small>';
      fHost.appendChild(div);
    });

    const tbody = $('board-body');
    tbody.replaceChildren();
    (data.six_stage_board || []).forEach(row => {
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
        tr.appendChild(td);
      });
      tbody.appendChild(tr);
    });
  }

  refresh();
  setInterval(refresh, 60000);
})();
