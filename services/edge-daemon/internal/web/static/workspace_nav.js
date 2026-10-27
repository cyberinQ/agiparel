// Shared restricted-workspace navigation. Does not read or modify factory records.
// Encapsulated styles avoid changing legacy HMI layouts and their event handlers.
(() => {
  'use strict';
  if (document.getElementById('agiparel-workspace-navigation')) return;
  const pages = [
    ['/', 'Home'],
    ['/factory-setup', 'Set up factory'],
    ['/documents', 'Upload documents'],
    ['/buyer-review', 'Buyer review'],
    ['/order-intake', 'Order readiness'],
    ['/material-planning', 'Material planning'],
    ['/stores', 'Fabric stores'],
    ['/planning', 'Factory planning'],
    ['/plant', 'Plant control'],
    ['/cutting', 'Cutting'],
    ['/sewing', 'Sewing execution'],
    ['/quality', 'Quality & finishing'],
    ['/packing', 'Packing & dispatch'],
    ['/onboarding', 'Onboarding']
  ];
  const pathname = window.location.pathname;
  const known = new Set(pages.map(([path]) => path));
  const host = document.createElement('div');
  host.id = 'agiparel-workspace-navigation';
  host.setAttribute('role', 'navigation');
  host.setAttribute('aria-label', 'Agiparel global navigation');
  const root = host.attachShadow({ mode: 'open' });
  const css = document.createElement('style');
  css.textContent = `
    :host{display:block;position:sticky;top:0;z-index:2147480000;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Arial,sans-serif;color:#1c3151}
    *,*::before,*::after{box-sizing:border-box}
    .bar{display:flex;align-items:center;gap:12px;padding:9px 18px;background:#fff;border-bottom:1px solid #dce4ef;box-shadow:0 1px 3px #16325a0a;min-height:54px}
    a,button{font-family:inherit;font-weight:600;font-size:12px;line-height:1.3;color:#304968;text-decoration:none;white-space:nowrap;border-radius:7px}
    a{padding:8px 10px}
    a:hover,a:focus-visible,button:hover,button:focus-visible{background:#e8f0ff;color:#2059cd;outline-offset:2px}
    a[aria-current]{background:#eaf1ff;color:#1d56c9;font-weight:800}
    .brand{font-weight:900;color:#17365e;font-size:16px;letter-spacing:-.9px}
    .brand small{font-size:8px;letter-spacing:1.1px;font-weight:800;margin-left:6px;color:#8393ae}
    .back{border:1px solid #d6e0ee;background:#f8fafe;padding:8px 11px;cursor:pointer;font-weight:750}
    .links{display:flex;align-items:center;gap:3px;flex:1;overflow-x:auto;scrollbar-width:thin}
    .upload{background:#245ee6!important;color:#fff!important;padding:10px 14px!important;font-weight:800!important}
    .location{font-size:11px;color:#788ba8;white-space:nowrap}
    @media(max-width:850px){.bar{padding:8px}.location,.brand small{display:none}.links{gap:0}.brand{font-size:14px}.back{padding:8px}}
  `;
  root.append(css);
  const bar = document.createElement('div');
  bar.className = 'bar';
  const brand = document.createElement('a');
  brand.className = 'brand';
  brand.href = '/';
  brand.textContent = 'agiparel';
  const small = document.createElement('small');
  small.textContent = 'FACTORY OS';
  brand.append(small);
  bar.append(brand);
  const back = document.createElement('button');
  back.className = 'back';
  back.type = 'button';
  back.textContent = '← Back';
  back.setAttribute('aria-label', 'Return to previous Agiparel page or home');
  back.addEventListener('click', () => {
    try {
      const previous = new URL(document.referrer);
      if (previous.origin === window.location.origin && known.has(previous.pathname) && previous.pathname !== pathname) {
        window.location.assign(previous.pathname + previous.search + previous.hash);
        return;
      }
    } catch (_) { /* Direct visit or untrusted referrer: go to home. */ }
    window.location.assign('/');
  });
  bar.append(back);
  const links = document.createElement('nav');
  links.className = 'links';
  links.setAttribute('aria-label', 'Factory OS pages');
  for (const [url, label] of pages) {
    const a = document.createElement('a');
    a.href = url;
    a.textContent = label;
    if (url === pathname) a.setAttribute('aria-current', 'page');
    if (url === '/documents') a.className = 'upload';
    links.append(a);
  }
  bar.append(links);
  const location = document.createElement('span');
  location.className = 'location';
  location.textContent = 'DEV · EVIDENCE FIRST';
  bar.append(location);
  root.append(bar);
  document.body.prepend(host);
})();
