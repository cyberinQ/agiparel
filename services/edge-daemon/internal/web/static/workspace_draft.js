// Author: Sunil Jain, CyberinQ LLC
// Same-tab WIP recovery + Global Cross-Chapter PO & Style Context Selector across all 10 chapters.
// Never store files, secrets, approval checkboxes, hidden revision tokens, or API results.
(() => {
  'use strict';
  if (!document.body.dataset.agPhase || !window.sessionStorage) return;

  // 1. Cross-Chapter Active PO + Style Scope Bar (prevents mixing Tech Packs / Bulletins from another PO)
  const CTX_KEY = 'agiparel:active-po-context:v51';
  function getActiveContext() {
    try { return JSON.parse(sessionStorage.getItem(CTX_KEY) || 'null'); } catch { return null; }
  }
  function setActiveContext(ctx) {
    try {
      if (ctx && ctx.po_number) sessionStorage.setItem(CTX_KEY, JSON.stringify(ctx));
      else sessionStorage.removeItem(CTX_KEY);
    } catch {}
    document.dispatchEvent(new CustomEvent('agiparel:po-context-changed', { detail: ctx }));
  }
  async function mountPOContextRibbon() {
    if (typeof fetch !== 'function' || document.getElementById('agiparel-po-scope-bar')) return;
    let dossiers = [];
    try {
      const res = await fetch('/api/v1/document-dossiers', { credentials: 'same-origin' });
      if (res.ok) {
        const data = await res.json();
        dossiers = data.dossiers || [];
      }
    } catch { return; }
    if (!dossiers.length) return;
    const bar = document.createElement('section');
    bar.id = 'agiparel-po-scope-bar';
    bar.className = 'ag-wip-resume';
    bar.setAttribute('data-no-draft', 'true');
    bar.setAttribute('aria-label', 'Active Purchase Order and Style Scope');
    const label = document.createElement('span');
    label.textContent = 'Active PO & Style Scope (filters Tech Packs & Bulletins across chapters):';
    const sel = document.createElement('select');
    sel.style.cssText = 'min-height:36px;padding:6px 10px;border:1px solid #b9cad5;border-radius:8px;background:#fff;font-weight:700;color:#1d3b4f';
    const allOpt = document.createElement('option');
    allOpt.value = '';
    allOpt.textContent = 'All POs / Unfiltered';
    sel.append(allOpt);
    const saved = getActiveContext();
    for (const d of dossiers) {
      const opt = document.createElement('option');
      opt.value = d.id;
      const docCount = (d.documents || []).length;
      opt.textContent = `PO ${d.po_number} · Style ${d.style_code}${d.buyer_name ? ' (' + d.buyer_name + ')' : ''} — ${docCount} linked doc(s)`;
      if (saved && saved.po_number === d.po_number && saved.style_code === d.style_code) {
        opt.selected = true;
      }
      sel.append(opt);
    }
    const meta = document.createElement('small');
    function updateMeta(d) {
      if (!d) {
        meta.textContent = 'Select a PO + Style above so only its Tech Packs, Operation Bulletins, Machines, Workers, and RFID tags appear.';
        return;
      }
      const roles = (d.documents || []).map(x => `${x.doc_role.replaceAll('_', ' ')}: ${x.filename}`).join(' | ');
      const mach = (d.locked_machines || []).length ? ` · Locked Machines: ${d.locked_machines.join(', ')}` : '';
      const work = (d.locked_workers || []).length ? ` · Workers: ${d.locked_workers.join(', ')}` : '';
      meta.textContent = `Scoped to PO ${d.po_number} / Style ${d.style_code}${mach}${work}${roles ? ' · Docs: ' + roles : ''}`;
    }
    const activeDossier = dossiers.find(x => x.id === sel.value) || null;
    updateMeta(activeDossier);
    if (activeDossier) {
      setTimeout(() => document.dispatchEvent(new CustomEvent('agiparel:po-context-changed', { detail: activeDossier })), 60);
    }
    sel.addEventListener('change', () => {
      const picked = dossiers.find(x => x.id === sel.value) || null;
      updateMeta(picked);
      setActiveContext(picked);
    });
    bar.append(label, sel, meta);
    const main = document.querySelector('main') || document.querySelector('.app') || document.body;
    main.prepend(bar);
  }
  void mountPOContextRibbon();
  document.addEventListener('agiparel:dossiers-updated', () => {
    const old = document.getElementById('agiparel-po-scope-bar');
    if (old) old.remove();
    void mountPOContextRibbon();
  });

  // 2. Existing Same-Tab Unsaved Form Recovery
  const KEY = 'agiparel:wip:v1:' + location.pathname.replace(/\/$/, '');
  const MAX_AGE = 24 * 60 * 60 * 1000;
  const MAX_VALUE = 3000;
  let dirty = new Set();
  let cached = null;
  let saving = false;
  const sensitive = /(password|secret|token|credential|authorization|ssn|social.security|bank|payroll|birth|national.id|employee.id)/i;
  const allowedTypes = new Set(['text', 'number', 'search', 'date', 'datetime-local', 'time', 'tel', 'url']);
  function eligible(el) {
    if (!(el instanceof HTMLElement) || !el.matches('input, textarea, select')) return false;
    if (el.disabled || el.closest('[data-no-draft], #aggie-chat')) return false;
    if (sensitive.test([el.name, el.id, el.autocomplete].join(' '))) return false;
    if (el.matches('input')) {
      if (!allowedTypes.has((el.type || 'text').toLowerCase())) return false;
    }
    return true;
  }
  function keyFor(el) {
    if (el.id) return 'id:' + el.id;
    if (el.name) {
      const matches = Array.from(document.querySelectorAll('[name]')).filter(item => item.name === el.name);
      return 'name:' + el.name + ':' + matches.indexOf(el);
    }
    return null;
  }
  function findField(key) {
    if (key.startsWith('id:')) return document.getElementById(key.slice(3));
    if (key.startsWith('name:')) {
      const last = key.lastIndexOf(':');
      const name = key.slice(5, last);
      return Array.from(document.querySelectorAll('[name]')).filter(el => el.name === name)[Number(key.slice(last + 1))];
    }
    return null;
  }
  function read() {
    try {
      const saved = JSON.parse(sessionStorage.getItem(KEY) || 'null');
      if (!saved || !saved.at || Date.now() - saved.at > MAX_AGE || !saved.values) return null;
      return saved;
    } catch { return null; }
  }
  function persist() {
    if (saving || !dirty.size) return;
    const values = {...(cached?.values || {})};
    for (const key of dirty) {
      const el = findField(key);
      if (eligible(el) && el.value.length <= MAX_VALUE) values[key] = el.value;
    }
    try {
      sessionStorage.setItem(KEY, JSON.stringify({at: Date.now(), values, scroll: Math.round(scrollY)}));
      cached = read();
    } catch {}
  }
  for (const name of ['input', 'change']) {
    document.addEventListener(name, event => {
      if (!event.isTrusted || !eligible(event.target)) return;
      const key = keyFor(event.target);
      if (!key) return;
      dirty.add(key); persist();
    }, true);
  }
  window.addEventListener('pagehide', persist);
  document.addEventListener('agiparel:draft-committed', () => {
    sessionStorage.removeItem(KEY); cached = null; dirty.clear();
  });
  cached = read();
  if (!cached || !Object.keys(cached.values).length) return;
  const notice = document.createElement('aside');
  notice.className = 'ag-wip-resume'; notice.setAttribute('aria-label', 'Previous unfinished work');
  const explanation = document.createElement('span');
  explanation.textContent = 'You have unfinished entries from this tab. Restore them?';
  const resume = document.createElement('button'); resume.type = 'button'; resume.textContent = 'Resume work';
  const discard = document.createElement('button'); discard.type = 'button'; discard.textContent = 'Discard';
  const caveat = document.createElement('small'); caveat.textContent = 'Not saved to the factory database. File selections and approvals are never restored.';
  resume.addEventListener('click', () => {
    saving = true;
    let restored = 0;
    for (const [key, value] of Object.entries(cached.values)) {
      const el = findField(key);
      if (!eligible(el) || typeof value !== 'string') continue;
      if (el.tagName === 'SELECT' && !Array.from(el.options).some(opt => opt.value === value)) continue;
      el.value = value;
      dirty.add(key);
      el.dispatchEvent(new Event('change', {bubbles: true}));
      restored++;
    }
    saving = false;
    notice.remove();
    if (restored && Number.isFinite(cached.scroll)) scrollTo({top: cached.scroll, behavior: 'instant'});
  });
  discard.addEventListener('click', () => {sessionStorage.removeItem(KEY); cached = null; dirty.clear(); notice.remove();});
  notice.append(explanation, resume, discard, caveat);
  const main = document.querySelector('main') || document.querySelector('.app') || document.body;
  main.prepend(notice);
})();