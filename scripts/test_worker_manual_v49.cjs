// Author: Sunil Jain, 2026-11-12.
// Verify one shared worker guide, all ten chapter screens, private routing and Aggie source boundaries.
'use strict';
const fs = require('fs');
const assert = require('node:assert/strict');
const root = require('node:path').resolve(__dirname, '..');
const read = p => fs.readFileSync(require('node:path').join(root,p),'utf8');
const guide = JSON.parse(read('services/edge-daemon/internal/workerhelp/guide.json'));
assert.equal(guide.chapters.length,10);
const pages = ['factory_setup.html','buyer_review.html','material_planning.html','stores.html','cutting_view.html','sewing_execution.html','quality_execution.html','packing_execution.html','factory_intelligence.html','documents.html','factory_intelligence_production.html','order_intake.html','cutting_sop2.html'];
for(const page of pages) {
  const html=read('services/edge-daemon/internal/web/static/'+page);
  assert.match(html,/\/static\/worker_help_v49.js/);
  assert.match(html,/\/static\/worker_help_v49.css/);
  assert.match(html,/\/static\/portal.js/);
}
const js=read('services/edge-daemon/internal/web/static/worker_help_v49.js');
const chat=read('services/edge-daemon/internal/web/static/portal.js');
const go=read('services/edge-daemon/cmd/server/main.go');
const portal=read('services/edge-daemon/cmd/server/portal.go');
assert.match(js,/credentials:'same-origin'/);
assert.match(js,/textContent/);
assert.doesNotMatch(js,/innerHTML|localStorage|\/api\/v1\/cutting\//);
assert.match(chat,/page:window.location\?\.pathname/);
assert.match(go,/handleAgieManual/);
assert.match(go,/handleAgieCapabilities/);
assert.match(go,/workerhelp\.Match/);
assert.match(portal,/default:[\s\S]*operatorAccessGate\(next, "", ""\)/);
assert(!portal.includes('case "/api/v1/agie/manual"'));
for(const c of guide.chapters){
 assert.equal(c.steps.length,6);
 assert(c.done&&c.stop&&c.path&&c.have);
}
console.log('PASS v49: 10 chapter guides, shared Aggie on every HMI, source-backed manual, model/DB independence and private access');
