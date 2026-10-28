// Author: Sunil Jain, 2026-10-28
// Chapter 01: simulate the guided workflow, skip/resume, readiness and source audit.
'use strict';
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');
const js = fs.readFileSync(path.join(__dirname, '../services/edge-daemon/internal/web/static/factory_setup.js'), 'utf8');

class Element {
  constructor(tag = 'div') {
    this.tag = tag;
    this.children = [];
    this.handlers = {};
    this.style = {};
    this.value = '';
    this.disabled = false;
    this.hidden = false;
    this.textContent = '';
  }
  addEventListener(event, handler) { this.handlers[event] = handler; }
  append(...elements) { this.children.push(...elements); }
  replaceChildren(...elements) { this.children = [...elements]; }
  setAttribute(name, value) { this[name] = value; }
  showModal() { this.open = true; }
  close() { this.open = false; }
  focus() {}
  click() { assert.ok(this.handlers.click, `missing handler for ${this.tag}`); this.handlers.click(); }
}
const ids = new Map();
const getElementById = id => {
  if (!ids.has(id)) ids.set(id, new Element(id));
  return ids.get(id);
};
const document = {
  hidden: false,
  getElementById,
  createElement: tag => new Element(tag),
  addEventListener() {},
};
const questions = [
  {field_key:'factory.name', question:'What is your factory called?'},
  {field_key:'machines.sewing_count', question:'How many machines?'},
  {field_key:'workforce.skills', question:'What skills are available?'},
];
const model = {
  known_fields:1, confirmed_fields:0, total_fields:3,
  next_questions:questions,
  next_question:questions[0].question,
  fields:[
    {key:'factory.name',group:'Factory',label:'Factory name',kind:'TEXT',help:'What is it?'},
    {key:'machines.sewing_count',group:'Machines',label:'Machine count',kind:'INTEGER',help:'Count',unit:'machines'},
    {key:'workforce.skills',group:'People & skills',label:'Skills',kind:'TEXT',help:'Skills'},
  ],
  facts:{'factory.name':{value:'Unit A',state:'PROVISIONAL',revision:2,source_document_id:'DOC-OLD',note:'Unconfirmed'}},
  documents:[],
  readiness:[
    {key:'capacity_baseline',label:'Draft capacity baseline',status:'NEEDS_CONFIRMATION',missing:['Machine count'],explanation:'Needs confirmation'},
    {key:'costing_baseline',label:'Draft costing baseline',status:'NEEDS_CONFIRMATION',missing:['Labor cost'],explanation:'Estimate only'},
    {key:'material_baseline',label:'Opening material picture',status:'FOUNDATION_PRESENT',missing:[],explanation:'Not operational'},
    {key:'material_issue',label:'Material issue',status:'NOT_AUTHORIZED',missing:[],explanation:'Blocked'},
  ],
};
let posts = 0;
let lastSubmitted = null;
const fetch = async (url, options = {}) => {
  if (options.method === 'POST') { posts++; lastSubmitted = JSON.parse(options.body); }
  return {ok:true,json:async () => url.endsWith('/approvals/pending') ? {pending_records:1} : model};
};
vm.runInNewContext(js, {document,window:{location:{search:''}},URLSearchParams,fetch,console});
const tick = () => new Promise(resolve => setImmediate(resolve));
(async () => {
  await tick();
  assert.equal(getElementById('next-question').textContent, questions[0].question);
  assert.equal(getElementById('readiness').children.length, 3, 'only three initial assessment cards shown');
  assert.equal(getElementById('answer-next').disabled, false);
  const factoryTopic = getElementById('topics').children[0];
  const factRow = factoryTopic.children[1];
  const sourceText = factRow.children[0].children[3].textContent;
  assert.match(sourceText, /Source attached \(outside recent file list\)/, 'older source must not appear unlinked');
  for (const next of questions.slice(1)) {
    getElementById('skip-next').click();
    assert.equal(getElementById('next-question').textContent, next.question);
  }
  getElementById('skip-next').click();
  assert.equal(getElementById('answer-next').disabled, true);
  assert.match(getElementById('next-question').textContent,/skipped/);
  assert.equal(posts,0, 'skip must not modify observations');
  getElementById('skip-next').click();
  assert.equal(getElementById('next-question').textContent, questions[0].question);
  getElementById('answer-next').click();
  assert.equal(getElementById('edit-dialog').open, true);
  assert.equal(getElementById('dialog-title').textContent, 'Factory name');
  assert.equal(getElementById('fact-source').value,'DOC-OLD', 'editing must retain older source provenance');
  assert.ok(getElementById('fact-source').children.some(item=>item.value==='DOC-OLD'));
  getElementById('fact-value').value = 'Unit A verified';
  getElementById('fact-form').handlers.submit({preventDefault() {}});
  await tick();
  assert.equal(posts, 1, 'saving the edited fact must issue exactly one write');
  assert.equal(lastSubmitted.source_document_id, 'DOC-OLD', 're-saving must preserve older original source');
  assert.equal(lastSubmitted.expected_revision, 2, 'stale-write protection must remain');
  assert.equal(getElementById('edit-dialog').open, false);
  console.log('PASS: Chapter 01 visible readiness, guided skip/resume, unchanged audit source and revision');
})().catch(err => {console.error(err);process.exitCode=1;});
