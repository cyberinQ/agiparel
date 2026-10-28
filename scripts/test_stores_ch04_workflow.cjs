// Author: Sunil Jain, 2026-10-28
// Chapter 04 browser regressions: source provenance, roll revisions, stock coverage and safe pick drafts.
'use strict';
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');
const folder = path.join(__dirname, '../services/edge-daemon/internal/web/static');
class Element {
  constructor(tag = 'div') {
    this.tagName = tag; this.children = []; this.handlers = {}; this._text = ''; this.value = '';
    this.disabled = false; this.readOnly = false; this.className = ''; this.href = '';
  }
  get childNodes() { return this.children; }
  get lastChild() { return this.children.at(-1); }
  set textContent(v) { this._text = String(v); this.children = []; }
  get textContent() { return this._text + this.children.map(c => typeof c === 'string' ? c : c.textContent).join(''); }
  append(...nodes) { this.children.push(...nodes); }
  replaceChildren(...nodes) { this._text = ''; this.children = []; this.append(...nodes); }
  addEventListener(name, handler) { (this.handlers[name] ||= []).push(handler); }
  fire(name) { return Promise.all((this.handlers[name] || []).map(fn => fn({preventDefault(){}}))); }
}
const wait = async () => { for (let i = 0; i < 25; i++) await Promise.resolve(); };
const roll = (id, source = 'DOC-12345678901234567890123456789012') => ({
  roll_id: id, revision: 3, event_kind: 'REINSPECT', epc_hex: '', material_sku: 'FAB-A', supplier_lot: 'LOT-1',
  shade_band: 'BLUE-A', bin_location: 'B-01', declared_length_m: '100.000', measured_length_m: '98.750',
  length_variance_m: '-1.250', usable_width_mm: 1480, inspection_result: 'PASS', defects: '',
  source_document_id: source, source_note: 'Measured again on inspection table',
  recorded_at: '2026-10-28T20:00:00Z', ledger_status: 'NOT_IN_STOCK_LEDGER'
});
async function checkRolls() {
  const fields = ['roll_id','epc_hex','material_sku','supplier_lot','shade_band','bin_location',
    'declared_length_m','measured_length_m','usable_width_mm','inspection_result','defects','source_note','expected_revision'];
  const ids = ['passport-form','source-document','roll-count-note','roll-list','passport-history','observed','passed',
    'held','message','verified','reserved','revision-label','save','new-roll','refresh'];
  const nodes = Object.fromEntries([...fields, ...ids].map(id => [id, new Element()]));
  const form = nodes['passport-form'];
  form.elements = {namedItem: name => nodes[name] || (name === 'source_document_id' ? nodes['source-document'] : undefined)};
  form.reset = () => { for (const name of fields) nodes[name].value = ''; nodes['source-document'].value = ''; };
  const requests = [];
  const fetch = async (url, init = {}) => {
    requests.push({url, init});
    let data;
    if (url === '/api/v1/stores/passports') data = {items: Array.from({length: 500}, (_, i) => roll(`ROLL-${i}`)), has_more: true};
    else if (url === '/api/v1/stores/authority') data = {inspected_pass_in_ledger: 12, open_reservations: 2};
    else if (url === '/api/v1/documents') data = {items: [{id:'DOC-NEW', filename:'latest.pdf', status:'READY'}]};
    else if (url.startsWith('/api/v1/stores/passports?')) data = {items: [roll('ROLL-0')]};
    else if (init.method === 'POST') data = {passport: roll('ROLL-0')};
    else throw Error(url);
    return {ok: true, json: async () => data};
  };
  class FormData {
    constructor() { return fields.map(name => [name, nodes[name].value])[Symbol.iterator](); }
  }
  const document = {getElementById: id => {assert.ok(nodes[id], `missing mock ${id}`);return nodes[id];}, createElement: tag => new Element(tag)};
  const code = fs.readFileSync(path.join(folder, 'stores.js'), 'utf8');
  vm.runInNewContext(code, {document, fetch, FormData, Number, String, Array, Date, Object, Promise,
    encodeURIComponent, window: {scrollTo(){}}, console}, {filename: 'stores.js'});
  await wait();
  assert.equal(nodes.observed.textContent, '500+', 'a capped list must not look like a factory-wide total');
  assert.match(nodes['roll-count-note'].textContent, /not factory-wide totals/);
  assert.match(nodes['roll-list'].children[0].textContent, /Short 1\.250 m \(1\.25%\)/);
  const first = nodes['roll-list'].children[0];
  await first.fire('click'); await wait();
  assert.equal(nodes.roll_id.readOnly, true, 'reinspection must lock physical roll identity');
  assert.equal(nodes['source-document'].value, roll('ROLL-0').source_document_id);
  assert.ok(nodes['source-document'].children.some(x => x.value === roll('ROLL-0').source_document_id), 'old evidence ID must remain an actual option');
  assert.equal(nodes.expected_revision.value, '3');
  assert.match(nodes['passport-history'].textContent, /Revision 3/);
  await nodes['new-roll'].fire('click');
  assert.equal(nodes.roll_id.readOnly, false);
  assert.equal(nodes.expected_revision.value, '0');
  assert.equal(nodes['source-document'].value, '');
  assert.match(nodes['passport-history'].textContent, /Select a roll/);
  assert.equal(requests.filter(x => x.init.method === 'POST').length, 0, 'new-roll reset must not write anything');
}
const order = (id, status = 'PROPOSED') => ({intake_id: id, po_number: id, style_code:'STYLE', ordered_qty: 50, status});
const report = (id, known = true) => ({
  order: order(id), materials: [{material_sku: 'FAB-A', uom: known ? 'M':'YARD', demand_known: known,
    demand_mm: known ? 250000:0, tentative_mm:0, uncovered_mm: known?250000:0,
    chosen_shade:'', tentative_picks: [], excluded_rolls: [], blockers: known ? []:['UNIT_CONVERSION_REQUIRED']}],
  warnings: ['Verify physical ownership']
});
async function checkPicks() {
  const ids = ['pick-results','pick-history','pick-feedback','pick-order','pick-note',
    'pick-form','pick-submit','pick-refresh'];
  const nodes = Object.fromEntries(ids.map(id => [id, new Element(id === 'pick-order' ? 'select':'div')]));
  const actions = [], held = [];
  let holdPost = false;
  const fetch = async (url, init = {}) => {
    actions.push({url, init});
    if (!init.method) return {ok:true,json:async()=>({orders:[order('PO-A'), order('PO-B'), order('PO-C','APPROVED')],
      proposals:[{state:'PROPOSED_HOLD', created_at:'2026-10-28T20:00:00Z', snapshot:report('PO-ARCHIVE', false)}]})};
    if (holdPost) return new Promise(resolve => held.push({init, resolve}));
    return {ok:true,json:async()=>({proposal:{snapshot:report(JSON.parse(init.body).intake_id), state:'PROPOSED_HOLD'}, replayed:false})};
  };
  const document = {getElementById: id=>{assert.ok(nodes[id], id);return nodes[id];},createElement:tag=>new Element(tag)};
  const code = fs.readFileSync(path.join(folder, 'stores_pick_proposals.js'), 'utf8');
  let next = 0;
  vm.runInNewContext(code, {document,fetch,crypto:{randomUUID:()=>`key-${++next}`},Number,String,Array,Date,JSON,console}, {filename: 'stores_pick_proposals.js'});
  await wait();
  assert.equal(nodes['pick-order'].children.length, 3, 'only 2 PROPOSED orders plus placeholder');
  await nodes['pick-history'].children[0].children.at(-1).fire('click');
  assert.match(nodes['pick-results'].textContent, /SAVED HISTORICAL PROPOSAL/);
  assert.match(nodes['pick-results'].textContent, /Unknown/,'unconvertible unit must not appear as zero purchase gap');
  nodes['pick-order'].value = 'PO-A'; await nodes['pick-order'].fire('change');
  assert.doesNotMatch(nodes['pick-results'].textContent, /PO-ARCHIVE/);
  nodes['pick-note'].value = 'Review shade and physical stock ownership';
  holdPost = true;
  const pending = nodes['pick-form'].fire('submit');
  await wait();
  assert.equal(held.length, 1);
  const firstKey = JSON.parse(held[0].init.body).request_key;
  nodes['pick-order'].value = 'PO-B'; await nodes['pick-order'].fire('change');
  held[0].resolve({ok:true,json:async()=>({proposal:{snapshot:report('PO-A'),state:'PROPOSED_HOLD'},replayed:false})});
  await pending; await wait();
  assert.doesNotMatch(nodes['pick-results'].textContent, /NEW DRAFT PROPOSAL.*PO-A/);
  nodes['pick-note'].value = 'Different quantity and shade review note';
  await nodes['pick-note'].fire('input');
  holdPost = false;
  await nodes['pick-form'].fire('submit'); await wait();
  const second = actions.filter(x => x.init.method === 'POST').at(-1);
  assert.notEqual(JSON.parse(second.init.body).request_key, firstKey, 'changed order/note needs new idempotency key');
  assert.match(nodes['pick-results'].textContent, /NEW DRAFT PROPOSAL.*PO-B/);
}
(async () => {
  await checkRolls();
  await checkPicks();
  console.log('PASS Chapter 04: capped roll totals, source preservation, locked revisions, reset, historical picks, unknown demand, and order races');
})().catch(err => {console.error(err);process.exitCode=1;});
