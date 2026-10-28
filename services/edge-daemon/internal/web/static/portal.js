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
