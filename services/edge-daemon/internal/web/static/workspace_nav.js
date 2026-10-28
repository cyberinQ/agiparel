// Author: Sunil Jain, 2026-10-28
// One uncluttered chapter banner (brand / previous chapter, Home, next chapter / Aggie) and WIP restoration.
// Home alone owns the ten-chapter directory; this module never builds a chapter menu.
(() => {
  'use strict';
  // Canonical chapter order is the same as the ten Home tiles. One source of
  // truth for both navigation and chapter accents; aliases keep older routes working.
  const chapters = [
    {href:'/factory-setup', phase:'factory', name:'Factory setup'},
    {href:'/buyer-review', phase:'buyer', name:'Buyer order'},
    {href:'/material-planning', phase:'material', name:'Plan material'},
    {href:'/stores', phase:'stores', name:'Fabric store'},
    {href:'/cutting', phase:'cutting', name:'Cutting'},
    {href:'/sewing', phase:'sewing', name:'Sewing'},
    {href:'/quality', phase:'quality', name:'Quality'},
    {href:'/packing', phase:'packing', name:'Packing'},
    {href:'/intelligence', phase:'reports', name:'Factory report'},
    {href:'/documents', phase:'documents', name:'Upload files'}
  ];
  const aliases = {
    '/onboarding':'/factory-setup', '/order-intake':'/buyer-review',
    '/planning':'/material-planning', '/sew01':'/sewing',
    '/plant':'/intelligence'
  };
  const path = location.pathname.replace(/\/$/, '') || '/';
  const canonicalPath = aliases[path] || path;
  const currentChapterIndex = chapters.findIndex(chapter => chapter.href === canonicalPath);
  if (currentChapterIndex < 0) return;
  const mount = () => {
    if (document.getElementById('agiparel-workspace-navigation')) return;
    document.body.dataset.agPhase = chapters[currentChapterIndex].phase;
    const sheet = document.createElement('link');
    sheet.rel = 'stylesheet';
    sheet.href = '/static/workspace_nav.css';
    document.head.append(sheet);

    // Light DOM is intentional: Aggie's existing portal.js uses getElementById.
    const header = document.createElement('header');
    header.id = 'agiparel-workspace-navigation';
    header.className = 'ag-chapter-bar';
    const inner = document.createElement('div');
    inner.className = 'ag-chapter-bar-inner';
    const brand = document.createElement('a');
    brand.href = '/'; brand.className = 'ag-chapter-brand';
    brand.setAttribute('aria-label', 'Agiparel Home');
    const pipe = document.createElement('span'); pipe.className = 'ag-chapter-pipe';
    pipe.setAttribute('aria-hidden', 'true');
    brand.append(pipe, document.createTextNode('agiparel'));
    const actions = document.createElement('div'); actions.className = 'ag-chapter-actions';
    // Real chapter links, not browser history: both arrows work on direct entry,
    // refresh, and cross-chapter navigation. Endpoints omit unavailable arrows.
    const historyGroup = document.createElement('nav');
    historyGroup.className = 'ag-chapter-history-group';
    historyGroup.setAttribute('aria-label', 'Chapter navigation');
    if (currentChapterIndex > 0) {
      const previousChapter = chapters[currentChapterIndex - 1];
      const back = document.createElement('a');
      back.className = 'ag-chapter-history';
      back.href = previousChapter.href;
      back.setAttribute('aria-label', 'Previous chapter: ' + previousChapter.name);
      back.title = 'Previous chapter: ' + previousChapter.name;
      back.textContent = '←';
      historyGroup.append(back);
    }
    const home = document.createElement('a'); home.href = '/'; home.className = 'ag-chapter-home';
    home.setAttribute('aria-label', 'Home'); home.title = 'Home';
    // Outline house icon, identical on every chapter.
    home.innerHTML = '<svg viewBox="0 0 24 24" width="21" height="21" fill="none" stroke="currentColor" stroke-width="1.9" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="m3 10 9-7 9 7v10a1 1 0 0 1-1 1h-5v-7H9v7H4a1 1 0 0 1-1-1z"/></svg>';
    historyGroup.append(home);
    if (currentChapterIndex < chapters.length - 1) {
      const nextChapter = chapters[currentChapterIndex + 1];
      const forward = document.createElement('a');
      forward.className = 'ag-chapter-history';
      forward.href = nextChapter.href;
      forward.setAttribute('aria-label', 'Next chapter: ' + nextChapter.name);
      forward.title = 'Next chapter: ' + nextChapter.name;
      forward.textContent = '→';
      historyGroup.append(forward);
    }
    actions.append(historyGroup);
    let aggie = document.getElementById('aggie-open');
    const hasExistingAggie = Boolean(aggie && document.getElementById('aggie-chat'));
    if (!aggie) {
      aggie = document.createElement('button'); aggie.type = 'button';
      aggie.id = 'aggie-open'; aggie.className = 'aggie-invite';
      aggie.setAttribute('aria-controls', 'aggie-chat'); aggie.setAttribute('aria-expanded', 'false');
    }
    // Keep exactly the same label and icon on every chapter, even when a
    // legacy chapter supplied the button. Reparenting preserves its listeners.
    aggie.classList.add('aggie-invite');
    aggie.setAttribute('aria-controls', 'aggie-chat');
    const face = document.createElement('span'); face.className = 'aggie-face';
    face.setAttribute('aria-hidden', 'true'); face.textContent = '😄';
    const label = document.createElement('span'); label.textContent = 'Aggie';
    aggie.replaceChildren(face, label);
    actions.append(aggie);
    inner.append(brand, actions); header.append(inner);
    document.body.prepend(header);

    // The chapter footer is exactly the Home footer's three-part structure.
    // It is presentation only; it does not claim authorization or change records.
    const footer = document.createElement('footer');
    footer.id = 'agiparel-workspace-footer';
    footer.className = 'ag-chapter-footer';
    const footerBrand = document.createElement('strong');
    footerBrand.textContent = 'agiparel';
    const powered = document.createElement('span');
    powered.textContent = 'Empowered by CyberinQ.';
    const privacy = document.createElement('span');
    privacy.textContent = 'Customer Records are Private, Secured and Encrypted.';
    footer.append(footerBrand, powered, privacy);
    document.body.append(footer);
    if (!hasExistingAggie) {
      const panel = document.createElement('section');
      panel.id = 'aggie-chat'; panel.className = 'aggie-chat ag-shared-chat';
      panel.hidden = true; panel.setAttribute('role', 'dialog');
      panel.setAttribute('aria-label', 'Chat with Aggie'); panel.setAttribute('aria-modal', 'false');
      // Static, reviewed markup only; conversation responses always use textContent in portal.js.
      panel.innerHTML = '<div class="aggie-top"><div class="aggie-identity"><span class="aggie-chat-face" aria-hidden="true">😄</span><strong>Aggie</strong><span class="aggie-bot-note">Factory helper</span></div><button type="button" id="aggie-close" class="aggie-close" aria-label="Close Aggie chat">×</button></div><div id="aggie-messages" class="aggie-messages" role="log" aria-live="polite"><p class="aggie-msg aggie-msg-bot">Hi! What would you like to know?</p></div><div class="aggie-compose"><label for="aggie-input">Ask Aggie</label><div class="aggie-compose-row"><textarea id="aggie-input" rows="2" maxlength="1000" placeholder="Type your question…"></textarea><button id="aggie-send" type="button">Send</button></div><p id="aggie-status" role="status">Operational decisions still require approval.</p></div>';
      document.body.append(panel);
      // Pages 03 and 04 already own a working Aggie and load portal.js themselves.
      const script = document.createElement('script'); script.src = '/static/portal.js';
      document.head.append(script);
    }
    const draft = document.createElement('script'); draft.src = '/static/workspace_draft.js';
    document.head.append(draft);
  };
  if (document.readyState === 'loading') document.addEventListener('DOMContentLoaded', mount, {once: true});
  else mount();
})();
