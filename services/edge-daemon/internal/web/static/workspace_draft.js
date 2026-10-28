// Author: Sunil Jain, 2026-10-28
// Same-tab WIP recovery for chapter forms; NOT a server save or an authorization.
// Never store files, secrets, approval checkboxes, hidden revision tokens, or API results.
(() => {
  'use strict';
  if (!document.body.dataset.agPhase || !window.sessionStorage) return;
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
    // Never restore statements of consent, approval, or file selection.
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
    } catch { /* Storage can be disabled or full. No claim of saved draft then. */ }
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
  // Future chapter save handlers may dispatch this only AFTER an acknowledged server commit.
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
