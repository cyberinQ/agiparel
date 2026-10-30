// Author: Sunil Jain, CyberinQ LLC, 2026-10-29.
// Public navigation and real/about-page contract; existing factory copy stays unchanged.
'use strict';
const assert=require('node:assert/strict'),fs=require('node:fs'),path=require('node:path');
const base=path.resolve(__dirname,'..');
const read=p=>fs.readFileSync(path.join(base,p),'utf8');
const h=read('services/edge-daemon/internal/web/static/portal.html');
const about=read('services/edge-daemon/internal/web/static/about.html');
const css=read('services/edge-daemon/internal/web/static/portal.css');
const server=read('services/edge-daemon/cmd/server/public_account_v43_2_2.go');
const gate=read('services/edge-daemon/cmd/server/portal.go');
const main=read('services/edge-daemon/cmd/server/main.go');
const nav=h.match(/<header class="site-top">([\s\S]*?)<\/header>/);
assert(nav);
for (const [url,text] of [['/about','About'],['/login','Sign in'],['/register','Create account']]) {
  assert(nav[1].includes('href="'+url+'"'),url+' missing in home top nav');
  assert(about.includes('href="'+url+'"'),url+' missing in About nav');
}
assert(h.includes('class="account-symbol"'), 'person icon missing');
assert(h.includes('id="agiparel-wave-canvas"'));
assert(h.includes('Every garment<br><em>tells a story.</em>'));
assert(!h.includes('One connected factory.'));
assert(!h.includes('FROM FIRST THREAD TO FINAL CARTON'));
assert(about.includes('Where we are today.')&&about.includes('factory-specific validation'));
assert(css.includes('.site-links')&&css.includes('margin-left:auto')&&css.includes('.about-grid'));
assert(server.includes('registerPublicAccountRoutes'));
assert(server.includes('safeHostedRegistrationURL'));
assert(server.includes('b != nil && b.enabled'), 'registration must not be assumed ready');
assert(server.includes('Self-registration is being configured.'));
assert(server.includes('Your password is entered only at the configured identity provider'));
assert(!server.includes('1212'));
assert(main.includes('registerPublicAccountRoutes(mux, server.browser)'));
assert(gate.includes('mux.HandleFunc("/about", servePublicAbout)'));
assert(gate.includes('"/about", "/login", "/register"'));
console.log('PASS v43.2.2: home nav, truthful account paths and About, unchanged copy/waves, public read-only routes');
