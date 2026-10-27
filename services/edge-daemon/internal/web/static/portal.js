// No fabricated orders or events are displayed on the public home page.
// Aggie uses the existing protected factory-chat endpoint; no anonymous database access.
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

  const history = [];
  let busy = false;

  function openChat() {
    panel.hidden = false;
    trigger.setAttribute('aria-expanded', 'true');
    input.focus();
  }
  function closeChat() {
    panel.hidden = true;
    trigger.setAttribute('aria-expanded', 'false');
    trigger.focus();
  }
  function appendMessage(text, fromUser) {
    const p = document.createElement('p');
    p.className = 'aggie-msg ' + (fromUser ? 'aggie-msg-user' : 'aggie-msg-bot');
    p.textContent = text; // Never insert untrusted assistant output as HTML.
    messages.appendChild(p);
    messages.scrollTop = messages.scrollHeight;
    return p;
  }
  function updateStatus(text, isError = false) {
    status.textContent = text;
    status.classList.toggle('is-error', isError);
  }

  async function sendMessage() {
    const query = input.value.trim();
    if (!query || busy) return;
    busy = true;
    send.disabled = true;
    input.disabled = true;
    input.value = '';
    appendMessage(query, true);
    // Bound the context sent to the backend; never store a password in JavaScript.
    history.push({ role: 'user', content: query });
    const prior = history.slice(-12);
    const answerNode = appendMessage('Checking…', false);
    updateStatus('Checking available factory records…');
    try {
      const response = await fetch('/api/v1/agie/chat', {
        method: 'POST',
        credentials: 'same-origin',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ messages: prior })
      });
      if (!response.ok) {
        if (response.status === 401 || response.status === 403) {
          throw new Error('Sign in to use Aggie with factory records.');
        }
        if (response.status === 503) {
          throw new Error('Aggie cannot check factory records right now. Please try again later.');
        }
        throw new Error('Aggie is not available right now. Please try again.');
      }
      const data = await response.json();
      if (!data || typeof data.reply !== 'string' || !data.reply.trim()) {
        throw new Error('Aggie did not receive a verified answer. Please try again.');
      }
      answerNode.textContent = data.reply;
      history.push({ role: 'assistant', content: data.reply });
      updateStatus(data.source === 'live_postgresql' ? 'Answer checked against factory records.' : '');
    } catch (error) {
      answerNode.textContent = error.message || 'Aggie is unavailable right now.';
      updateStatus('No answer was saved.', true);
      // A failed request should not be replayed as an answered message in history.
      history.pop();
    } finally {
      busy = false;
      send.disabled = false;
      input.disabled = false;
      input.focus();
      messages.scrollTop = messages.scrollHeight;
    }
  }

  trigger.addEventListener('click', () => panel.hidden ? openChat() : closeChat());
  close.addEventListener('click', closeChat);
  send.addEventListener('click', sendMessage);
  input.addEventListener('keydown', event => {
    if (event.key === 'Enter' && !event.shiftKey && !event.isComposing) {
      event.preventDefault();
      sendMessage();
    }
    if (event.key === 'Escape') closeChat();
  });
  document.addEventListener('keydown', event => {
    if (event.key === 'Escape' && !panel.hidden) closeChat();
  });
})();
