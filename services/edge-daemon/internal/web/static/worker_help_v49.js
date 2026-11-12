// Author: Sunil Jain, 2026-11-12.
// Add one accessible, collapsible worker guide to each protected AGIPAREL chapter.
(() => {
  'use strict';
  const chapterByPath = Object.freeze({
    '/factory-setup': 1, '/buyer-review': 2, '/material-planning': 3,
    '/stores': 4, '/cutting': 5, '/sewing': 6, '/quality': 7,
    '/packing': 8, '/intelligence': 9, '/documents': 10,
    '/order-intake': 2, '/cutting/sop2': 5
  });
  const number = chapterByPath[location.pathname];
  if (!number) return;
  // Some older chapters had no chat interface. Create it before portal.js loads.
  // The existing portal.js owns the conversation and uses same-origin requests.
  if (!document.getElementById('aggie-open')) {
    const launcher = document.createElement('button');
    launcher.id = 'aggie-open';
    launcher.className = 'ag-worker-aggie-open';
    launcher.type = 'button';
    launcher.textContent = 'Ask Aggie';
    launcher.setAttribute('aria-controls', 'aggie-chat');
    launcher.setAttribute('aria-expanded', 'false');
    document.body.append(launcher);
    const panel = document.createElement('section');
    panel.id = 'aggie-chat'; panel.className = 'aggie-chat ag-worker-aggie-chat';
    panel.hidden = true;
    panel.setAttribute('role', 'dialog');
    panel.setAttribute('aria-label', 'Ask Aggie for help');
    const top = document.createElement('div'); top.className = 'aggie-top';
    const title = document.createElement('strong'); title.textContent = 'Aggie · Factory helper';
    const close = document.createElement('button');
    close.id = 'aggie-close'; close.type = 'button'; close.textContent = 'Close';
    top.append(title, close);
    const messages = document.createElement('div');
    messages.id = 'aggie-messages'; messages.className = 'aggie-messages';
    messages.setAttribute('role','log'); messages.setAttribute('aria-live','polite');
    const initial = document.createElement('p'); initial.textContent = 'Ask me how to use this screen.';
    messages.append(initial);
    const label = document.createElement('label');
    label.htmlFor = 'aggie-input'; label.textContent = 'Your question';
    const input = document.createElement('textarea');
    input.id = 'aggie-input'; input.rows = 2; input.maxLength = 1000;
    const send = document.createElement('button');
    send.id = 'aggie-send'; send.type = 'button'; send.textContent = 'Send';
    const status = document.createElement('p'); status.id = 'aggie-status';
    status.setAttribute('role', 'status'); status.textContent = 'Factory facts require checked records.';
    panel.append(top, messages, label, input, send, status);
    document.body.append(panel);
  }
  const main = document.querySelector('main');
  if (!main || document.getElementById('ag-worker-help')) return;

  const guide = document.createElement('details');
  guide.id = 'ag-worker-help';
  guide.className = 'ag-worker-help';
  const heading = document.createElement('summary');
  heading.textContent = `Chapter ${String(number).padStart(2, '0')} · Show simple worker instructions`;
  const body = document.createElement('div');
  body.className = 'ag-worker-help-body';
  const intro = main.querySelector(':scope > .intro, :scope > .welcome, :scope > section:first-child');
  if (intro) intro.insertAdjacentElement('afterend', guide);
  else main.prepend(guide);
  guide.append(heading, body);

  function el(tag, text, className) {
    const item = document.createElement(tag);
    if (className) item.className = className;
    item.textContent = text;
    return item;
  }
  function putLine(label, value, className) {
    const line = el('p', '', className);
    line.append(el('strong', label + ' '), document.createTextNode(value));
    body.append(line);
  }
  function render(chapter) {
    body.replaceChildren();
    body.append(el('h2', chapter.title + ' — ' + chapter.subtitle));
    putLine('Your job:', chapter.goal);
    putLine('Have ready:', chapter.have);
    const steps = document.createElement('ol');
    for (const step of chapter.steps) steps.append(el('li', step));
    body.append(steps);
    putLine('DONE:', chapter.done, 'ag-worker-help-done');
    putLine('STOP IF:', chapter.stop, 'ag-worker-help-stop');
    if (chapter.mode === 'practice') {
      body.append(el('p', 'Practice only. This screen does not approve real goods or record confirmed factory output.', 'ag-worker-help-stop'));
    }
    const actions = el('div', '', 'ag-worker-help-actions');
    const ask = document.getElementById('aggie-open');
    if (ask) {
      const helpButton = el('button', 'Ask Aggie about this chapter');
      helpButton.type = 'button';
      helpButton.addEventListener('click', () => {
        const input = document.getElementById('aggie-input');
        ask.click();
        if (input && !input.value.trim()) {
          input.value = `Help me with Chapter ${String(number).padStart(2, '0')} — ${chapter.title}.`;
          input.dispatchEvent(new Event('input', {bubbles:true}));
          input.focus();
        }
      });
      actions.append(helpButton);
    }
    const all = el('a', 'See the 10 chapters');
    all.href = '/';
    actions.append(all);
    body.append(actions);
  }

  body.append(el('p', 'Opening the worker instructions…'));
  // Private endpoint: manual content is returned by AGIPAREL, not an AI guess.
  fetch(`/api/v1/agie/manual?chapter=${number}`, {credentials:'same-origin', cache:'no-store'})
    .then(r => { if (!r.ok) throw new Error('not available'); return r.json(); })
    .then(data => {
      if (data && data.chapter && data.chapter.number === number && Array.isArray(data.chapter.steps)) render(data.chapter);
      else throw new Error('incomplete chapter');
    })
    .catch(() => { body.replaceChildren(el('p', 'The guide is not available now. Ask your manager or try again later.')); });
})();
