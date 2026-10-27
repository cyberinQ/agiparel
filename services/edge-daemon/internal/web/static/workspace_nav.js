// Agiparel customer navigation: one guided journey, advanced tools kept available.
// Shadow DOM isolates navigation styles from legacy operator screens.
(() => {
  'use strict';
  if (document.getElementById('agiparel-workspace-navigation')) return;

  const journey = [
    { path: '/', label: 'Home', phase: 'home' },
    { path: '/factory-setup', label: 'Factory setup', phase: 'factory' },
    { path: '/documents', label: 'Upload files', phase: 'documents' },
    { path: '/buyer-review', label: 'Buyer order', phase: 'buyer' },
    { path: '/material-planning', label: 'Plan material', phase: 'material' },
    { path: '/stores', label: 'Fabric store', phase: 'stores' },
    { path: '/cutting', label: 'Cutting', phase: 'cutting' },
    { path: '/sewing', label: 'Sewing', phase: 'sewing' },
    { path: '/quality', label: 'Quality', phase: 'quality' },
    { path: '/packing', label: 'Packing', phase: 'packing' },
    { path: '/intelligence', label: 'Factory report', phase: 'reports' }
  ];
  // Preserve specialist URLs and their existing implementations.
  const tools = [
    { path: '/order-intake', label: 'CSV order upload', phase: 'buyer' },
    { path: '/planning', label: 'Line planning', phase: 'material' },
    { path: '/plant', label: 'Floor board', phase: 'reports' },
    { path: '/onboarding', label: 'Setup help', phase: 'factory' },
    { path: '/sew01', label: 'Old station demo', phase: 'sewing' }
  ];
  const path = window.location.pathname.replace(/\/$/, '') || '/';
  const currentIndex = journey.findIndex(item => item.path === path);
  const current = journey[currentIndex] || tools.find(item => item.path === path) || journey[0];
  const palette = {
    home: '#167e88', factory: '#218aaa', documents: '#5966cf', buyer: '#306ccb',
    material: '#8557bd', stores: '#b47712', cutting: '#df6b23', sewing: '#087f79',
    quality: '#2b844b', packing: '#bb3975', reports: '#486484'
  };
  const accent = palette[current.phase] || palette.home;
  document.body.dataset.agPhase = current.phase;

  const host = document.createElement('div');
  host.id = 'agiparel-workspace-navigation';
  host.setAttribute('role', 'navigation');
  host.setAttribute('aria-label', 'Agiparel work areas');
  const root = host.attachShadow({ mode: 'open' });
  const css = document.createElement('style');
  css.textContent = `
    :host {display:block;position:sticky;top:0;z-index:1000;font-family:system-ui,-apple-system,'Segoe UI',sans-serif;color:#263747}
    *,*::before,*::after {box-sizing:border-box}
    a,button,summary {font:inherit}
    a {text-decoration:none;color:inherit}
    .shell {background:#fff;border-bottom:1px solid #dce3e9;box-shadow:0 2px 9px #132e4410}
    .top {max-width:1440px;margin:auto;min-height:60px;display:flex;align-items:center;gap:15px;padding:9px 22px}
    .brand {font-size:21px;font-weight:850;letter-spacing:-1.2px;color:#203744;white-space:nowrap}
    .brand i {display:inline-block;width:9px;height:22px;background:${accent};border-radius:7px;margin-right:8px;vertical-align:-4px}
    .phase {font-size:13px;font-weight:700;color:#506273;border-left:1px solid #dce3e9;padding-left:13px;min-width:0}
    .spacer {flex:1}
    .pager {display:flex;align-items:center;gap:8px}
    .pager a {border:1px solid #c7d1da;padding:9px 12px;border-radius:8px;font-size:13px;font-weight:750;white-space:nowrap;color:#2e4b5d;background:#fff}
    .pager a.next {background:#1f5262;border-color:#1f5262;color:#fff}
    .pager a:hover,.pager a:focus-visible {outline:2px solid ${accent};outline-offset:2px}
    .areas {max-width:1440px;margin:auto;display:flex;gap:3px;align-items:center;padding:0 18px 9px;overflow-x:auto;scrollbar-width:thin}
    .areas a {display:flex;gap:5px;align-items:center;white-space:nowrap;padding:8px 10px;border-radius:7px;font-size:12px;font-weight:700;color:#526476}
    .areas a .number {font-size:10px;font-weight:850;color:#7a8998}
    .areas a:hover,.areas a:focus-visible {background:#f1f5f7;color:#244d5d}
    .areas a[aria-current] {background:${accent};color:#fff}
    .areas a[aria-current] .number {color:#fff}
    .more {position:relative;margin-left:auto;flex-shrink:0}
    .more summary {cursor:pointer;list-style:none;border:1px solid #dbe3ea;border-radius:7px;padding:7px 11px;font-size:12px;font-weight:750;color:#3c5366;white-space:nowrap;background:#fff}
    .more summary::-webkit-details-marker {display:none}
    .more .menu {position:absolute;right:0;top:calc(100% + 8px);z-index:20;background:#fff;border:1px solid #d7e0e7;border-radius:10px;box-shadow:0 16px 35px #0e273526;min-width:210px;padding:7px}
    .more .menu a {display:block;font-size:13px;padding:11px 13px}
    .more[open] summary {border-color:${accent}}
    .mobile-menu {display:none}
    @media(max-width:900px){.top{padding:8px 13px;gap:9px}.brand{font-size:19px}.phase{font-size:12px}.pager a{padding:8px;font-size:12px}.areas{padding:0 9px 8px}.areas a{padding:8px}}
    @media(max-width:620px){.phase{display:none}.pager a{font-size:11px}.areas{display:none}.mobile-menu{display:block;padding:0 12px 10px}.mobile-menu summary{cursor:pointer;background:#eef4f6;border-radius:8px;padding:10px 14px;font-size:13px;font-weight:750}.mobile-links{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:4px;padding:8px 0}.mobile-links a{font-size:13px;padding:11px 8px;border-radius:7px}.mobile-links a[aria-current]{background:${accent};color:#fff}}
    @media(prefers-reduced-motion:reduce){*{scroll-behavior:auto!important}}
  `;
  root.append(css);
  const shell = document.createElement('div'); shell.className = 'shell';
  const top = document.createElement('div'); top.className = 'top';
  const brand = document.createElement('a'); brand.href = '/'; brand.className = 'brand';
  const bar = document.createElement('i'); bar.setAttribute('aria-hidden', 'true');
  brand.append(bar, document.createTextNode('agiparel')); top.append(brand);
  const phase = document.createElement('span'); phase.className = 'phase'; phase.textContent = current.label;
  top.append(phase);
  const spacer = document.createElement('span'); spacer.className = 'spacer'; top.append(spacer);
  const pager = document.createElement('div'); pager.className = 'pager';
  if (currentIndex > 0) addPager(pager, journey[currentIndex - 1], '← Back');
  if (currentIndex >= 0 && currentIndex < journey.length - 1) addPager(pager, journey[currentIndex + 1], 'Next →', 'next');
  if (currentIndex < 0) addPager(pager, journey[0], 'Home');
  top.append(pager); shell.append(top);
  const areas = document.createElement('nav'); areas.className = 'areas';
  areas.setAttribute('aria-label', 'Main factory journey');
  journey.forEach((item, i) => {
    const link = document.createElement('a'); link.href = item.path;
    if (item.path === path) link.setAttribute('aria-current', 'page');
    const n = document.createElement('span'); n.className = 'number';
    n.textContent = i === 0 ? '⌂' : String(i);
    link.append(n, document.createTextNode(item.label)); areas.append(link);
  });
  const more = document.createElement('details'); more.className = 'more';
  const summary = document.createElement('summary'); summary.textContent = 'More tools ▾'; more.append(summary);
  const menu = document.createElement('div'); menu.className = 'menu';
  tools.forEach(item => menu.append(linkTo(item)));
  more.append(menu); areas.append(more); shell.append(areas);
  const mobile = document.createElement('details'); mobile.className = 'mobile-menu';
  const mobileSummary = document.createElement('summary'); mobileSummary.textContent = 'Work areas ▾'; mobile.append(mobileSummary);
  const mobileLinks = document.createElement('div'); mobileLinks.className = 'mobile-links';
  [...journey, ...tools].forEach(item => mobileLinks.append(linkTo(item)));
  mobile.append(mobileLinks); shell.append(mobile);
  root.append(shell);
  function addPager(parent, item, text, extra = '') {
    const a = document.createElement('a'); a.href = item.path; a.className = extra;
    a.textContent = text; a.setAttribute('aria-label', `${text.replace(/[←→]/g, '').trim()}: ${item.label}`); parent.append(a);
  }
  function linkTo(item) {
    const a = document.createElement('a'); a.href = item.path; a.textContent = item.label;
    if (path === item.path) a.setAttribute('aria-current', 'page');
    return a;
  }
  const mount = () => { if (!host.isConnected) document.body.prepend(host); };
  if (document.readyState === 'loading') document.addEventListener('DOMContentLoaded', mount, {once:true});
  else mount();
})();
