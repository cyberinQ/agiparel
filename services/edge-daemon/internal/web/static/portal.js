// Author: Sunil Jain, 2026-10-28
// Shared Aggie conversation across Home and all ten chapters; clarify backend failures. Preserve completed turns,
// unsent text, and expanded/collapsed state within the current browser tab only.
// Factory responses are rendered as text and never copied into the DOM as HTML.
(() => {
  'use strict';
  const trigger = document.getElementById('aggie-open');
  const panel = document.getElementById('aggie-chat');
  const close = document.getElementById('aggie-close');
  const input = document.getElementById('aggie-input');
  const send = document.getElementById('aggie-send');
  const messages = document.getElementById('aggie-messages');
  const status = document.getElementById('aggie-status');
  if (!trigger || !panel || !close || !input || !send || !messages || !status) return;

  // Session storage is scoped to this tab. This is WIP recovery, not a server-side
  // audit or cross-device chat record. An authenticated server store is required
  // before a conversation can follow an operator between devices.
  const STORAGE_KEY = 'agiparel.aggie.conversation.v1';
  const MAX_TURNS = 50;
  const MAX_SAVED_REPLY = 16000;
  let history = [];
  let busy = false;
  let pending = false;
  let storageAvailable = true;

  function loadState() {
    try {
      const raw = sessionStorage.getItem(STORAGE_KEY);
      if (!raw) return null;
      const data = JSON.parse(raw);
      if (!data || data.version !== 1 || !Array.isArray(data.messages)) return null;
      return {
        messages: data.messages.filter(item => item &&
          (item.role === 'user' || item.role === 'assistant') &&
          typeof item.content === 'string' && item.content.length <= MAX_SAVED_REPLY
        ).slice(-MAX_TURNS),
        draft: typeof data.draft === 'string' ? data.draft.slice(0, 1000) : '',
        open: data.open === true,
        pending: data.pending === true
      };
    } catch (_) {
      storageAvailable = false;
      return null;
    }
  }

  function persist() {
    if (!storageAvailable) return;
    try {
      sessionStorage.setItem(STORAGE_KEY, JSON.stringify({
        version: 1, messages: history.slice(-MAX_TURNS),
        draft: input.value.slice(0, 1000), open: !panel.hidden, pending
      }));
    } catch (_) {
      storageAvailable = false;
      updateStatus('Conversation cannot be saved in this browser tab.', true);
    }
  }

  function appendMessage(text, fromUser) {
    const p = document.createElement('p');
    p.className = 'aggie-msg ' + (fromUser ? 'aggie-msg-user' : 'aggie-msg-bot');
    p.textContent = text; // Untrusted model output is never interpreted as HTML.
    messages.appendChild(p);
    messages.scrollTop = messages.scrollHeight;
    return p;
  }
  function updateStatus(text, isError = false) {
    status.textContent = text;
    status.classList.toggle('is-error', isError);
  }
  function openChat(focus = true) {
    panel.hidden = false;
    trigger.setAttribute('aria-expanded', 'true');
    persist();
    if (focus) input.focus();
  }
  function collapseChat() {
    panel.hidden = true;
    trigger.setAttribute('aria-expanded', 'false');
    persist();
    trigger.focus();
  }

  // Initialize on every page, including Home and pages with their own Aggie markup.
  close.textContent = '−';
  close.setAttribute('aria-label', 'Collapse Aggie');
  close.title = 'Collapse Aggie';
  const saved = loadState();
  if (saved) {
    history = saved.messages;
    input.value = saved.draft;
    if (history.length) {
      messages.replaceChildren();
      for (const item of history) appendMessage(item.content, item.role === 'user');
    }
    // Navigating while a reply is in flight cancels the fetch. Do not invent an
    // answer or silently replay the request: return that question to the composer.
    if (saved.pending && history.length && history[history.length - 1].role === 'user') {
      const interrupted = history.pop();
      if (!input.value.trim()) input.value = interrupted.content.slice(0, 1000);
      if (messages.lastElementChild && messages.lastElementChild.classList.contains('aggie-msg-user')) {
        messages.lastElementChild.remove();
      }
      updateStatus('Reply interrupted by navigation. Press Send to try again.');
      if (!history.length) appendMessage('Your question is ready to resend.', false);
    }
    panel.hidden = !saved.open;
    trigger.setAttribute('aria-expanded', String(saved.open));
    pending = false;
    persist();
  }

  async function sendMessage() {
    const query = input.value.trim();
    if (!query || busy) return;
    busy = true;
    pending = true;
    send.disabled = true;
    input.disabled = true;
    input.value = '';
    const questionNode = appendMessage(query, true);
    history.push({role:'user', content:query});
    history = history.slice(-MAX_TURNS);
    persist(); // Save the pending question BEFORE making a network request.
    const answerNode = appendMessage('Checking…', false);
    updateStatus('Checking available factory records…');
    try {
      const response = await fetch('/api/v1/agie/chat', {
        method: 'POST', credentials: 'same-origin',
        headers: {'Content-Type':'application/json'},
        body: JSON.stringify({messages:history.slice(-12)})
      });
      if (!response.ok) {
        if (response.status === 401 || response.status === 403) throw new Error('Sign in to use Aggie with factory records.');
        if (response.status === 503) {
          const detail = await response.json().catch(() => ({}));
          if (detail.code === 'model_unavailable') {
            throw new Error('Aggie needs its AI model running. Ask your administrator to start Ollama. Your question is kept below.');
          }
          if (detail.code === 'records_unavailable') {
            throw new Error('Factory records are temporarily unavailable. Please ask your administrator to check the database.');
          }
          throw new Error('Aggie is temporarily unavailable. Check the server log for the cause.');
        }
        throw new Error('Aggie is not available right now. Please try again.');
      }
      const data = await response.json();
      if (!data || typeof data.reply !== 'string' || !data.reply.trim()) {
        throw new Error('Aggie did not receive a verified answer. Please try again.');
      }
      answerNode.textContent = data.reply;
      history.push({role:'assistant', content:data.reply.slice(0, MAX_SAVED_REPLY)});
      history = history.slice(-MAX_TURNS);
      updateStatus(data.source === 'live_postgresql' ? 'Answer checked against factory records.' : '');
    } catch (error) {
      // Do not leave an unanswered question and an error-looking "answer" in
      // the transcript. The original question is returned to the composer.
      answerNode.remove();
      questionNode.remove();
      updateStatus(error.message || 'Aggie is unavailable. Your question is ready to retry.', true);
      // Keep the user's question in the input, not as an answered conversation turn.
      history.pop();
      input.value = query;
    } finally {
      pending = false;
      busy = false;
      send.disabled = false;
      input.disabled = false;
      persist();
      if (!panel.hidden) input.focus();
      messages.scrollTop = messages.scrollHeight;
    }
  }

  trigger.addEventListener('click', () => panel.hidden ? openChat() : collapseChat());
  close.addEventListener('click', collapseChat);
  send.addEventListener('click', sendMessage);
  input.addEventListener('input', persist);
  input.addEventListener('keydown', event => {
    if (event.key === 'Enter' && !event.shiftKey && !event.isComposing) {
      event.preventDefault();
      sendMessage();
    }
    if (event.key === 'Escape') collapseChat();
  });
  document.addEventListener('keydown', event => {
    if (event.key === 'Escape' && !panel.hidden) collapseChat();
  });
  window.addEventListener('pagehide', persist);
})();


// v43.2: fabric-thread decorative waves only; no homepage copy changes. No third-party scripts or network calls.
// Honors reduced motion, tab visibility and off-screen pausing. Decorative canvas only.
(() => {
  'use strict';
  const canvas = document.getElementById('agiparel-wave-canvas');
  if (!canvas || typeof canvas.getContext !== 'function') return;
  const context = canvas.getContext('2d', {alpha: true});
  if (!context) return;
  const panel = canvas.closest('.fabric-wave-art');
  if (!panel) return;
  const reduceMotion = window.matchMedia && window.matchMedia('(prefers-reduced-motion: reduce)');
  let visible = false;
  let animation = 0;
  let lastFrame = -100;
  let width = 0;
  let height = 0;
  let disposed = false;
  const colors = ['22,126,136', '43,146,167', '55,116,164', '50,167,147'];

  function draw(time) {
    context.clearRect(0, 0, width, height);
    const strands = width < 650 ? 30 : 54;
    const step = Math.max(9, Math.min(20, width / 70));
    const t = time * .001;
    // Two gently interlaced groups of flowing parallel threads.
    for (let i = 0; i < strands; i++) {
      const p = i / Math.max(1, strands - 1);
      const base = height * (.10 + p * .83);
      const gradient = context.createLinearGradient(0, 0, width, 0);
      const color = colors[Math.floor(i / 14) % colors.length];
      gradient.addColorStop(0, 'rgba(' + color + ',0.02)');
      gradient.addColorStop(.31, 'rgba(' + color + ',' + (.16 + .12 * p) + ')');
      gradient.addColorStop(.78, 'rgba(' + color + ',' + (.28 + .19 * (1 - p)) + ')');
      gradient.addColorStop(1, 'rgba(' + color + ',0.06)');
      context.strokeStyle = gradient;
      context.lineWidth = (i % 9 === 0 ? 1.7 : 1.02);
      context.beginPath();
      for (let x = -step, first = true; x <= width + step; x += step) {
        const u = x / Math.max(width, 1);
        const rise = Math.sin(u * 5.2 - t * .32 + p * 1.8) * (28 + 13 * p);
        const weave = Math.sin(u * 12.1 + t * .24 - p * 3.8) * (8 + 8 * p);
        const arc = Math.exp(-Math.pow((u - .69) / .36, 2)) * (-27 + p * 18);
        const y = base + rise + weave + arc;
        if (first) {context.moveTo(x, y); first = false;}
        else context.lineTo(x, y);
      }
      context.stroke();
    }
  }

  function resize() {
    const rect = panel.getBoundingClientRect();
    width = Math.max(1, Math.round(rect.width));
    height = Math.max(1, Math.round(rect.height));
    // Limit device-pixel scaling to keep rendering modest on high-DPI phones.
    const dpr = Math.min(window.devicePixelRatio || 1, 2);
    canvas.width = Math.round(width * dpr);
    canvas.height = Math.round(height * dpr);
    context.setTransform(dpr, 0, 0, dpr, 0, 0);
    draw(0);
  }
  function shouldAnimate() {
    return !disposed && visible && !document.hidden && !(reduceMotion && reduceMotion.matches);
  }
  function stop() {
    if (animation) {cancelAnimationFrame(animation); animation = 0;}
  }
  function tick(timestamp) {
    animation = 0;
    if (!shouldAnimate()) return;
    // Approx. 30 fps; this is background art, not a game loop.
    if (timestamp - lastFrame >= 32) {draw(timestamp); lastFrame = timestamp;}
    animation = requestAnimationFrame(tick);
  }
  function update() {
    if (!shouldAnimate()) {stop(); draw(0); return;}
    if (!animation) animation = requestAnimationFrame(tick);
  }
  resize();
  let observer;
  if ('IntersectionObserver' in window) {
    observer = new IntersectionObserver(entries => {
      visible = entries.some(entry => entry.isIntersecting);
      update();
    }, {threshold: .03});
    observer.observe(panel);
  } else {
    visible = true;
    update();
  }
  let resizeObserver;
  if ('ResizeObserver' in window) {
    resizeObserver = new ResizeObserver(() => {resize(); update();});
    resizeObserver.observe(panel);
  } else window.addEventListener('resize', resize);
  document.addEventListener('visibilitychange', update);
  if (reduceMotion) {
    if (typeof reduceMotion.addEventListener === 'function') reduceMotion.addEventListener('change', update);
    else if (typeof reduceMotion.addListener === 'function') reduceMotion.addListener(update);
  }
  window.addEventListener('pagehide', () => {
    disposed = true;
    stop();
    if (observer) observer.disconnect();
    if (resizeObserver) resizeObserver.disconnect();
  }, {once:true});
})();
