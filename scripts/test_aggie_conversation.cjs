// Author: Sunil Jain, 2026-10-28
// Dependency-free test: Aggie session continuity, collapse/reopen, pending-reply retry.
'use strict';
const assert = require('node:assert/strict');
const fs = require('node:fs');
const vm = require('node:vm');
const path = require('node:path');
const script = fs.readFileSync(path.join(__dirname, '../services/edge-daemon/internal/web/static/portal.js'), 'utf8');

class Element {
  constructor(id = '') {
    this.id = id;
    this.children = [];
    this.listeners = {};
    this.value = '';
    this.textContent = '';
    this.hidden = id === 'aggie-chat';
    this.disabled = false;
    this.attributes = {};
    this.classes = new Set();
    this.scrollTop = 0;
    this.scrollHeight = 0;
    this.classList = {
      add: value => this.classes.add(value),
      contains: value => this.classes.has(value),
      toggle: (value, active) => active ? this.classes.add(value) : this.classes.delete(value)
    };
  }
  set className(value) { this.classes = new Set(value.split(/\s+/)); }
  get className() { return [...this.classes].join(' '); }
  addEventListener(name, callback) { (this.listeners[name] ||= []).push(callback); }
  async dispatch(name, event = {}) {
    for (const callback of this.listeners[name] || []) await callback(event);
  }
  setAttribute(name, value) { this.attributes[name] = value; }
  getAttribute(name) { return this.attributes[name]; }
  appendChild(child) { this.children.push(child); child.parent = this; }
  replaceChildren(...items) { this.children = []; for (const item of items) this.appendChild(item); }
  get lastElementChild() { return this.children[this.children.length - 1]; }
  remove() { this.parent.children = this.parent.children.filter(item => item !== this); }
  focus() {}
}

function newPage(storage, fetchImpl) {
  const ids = ['aggie-open', 'aggie-chat', 'aggie-close', 'aggie-input', 'aggie-send', 'aggie-messages', 'aggie-status'];
  const elements = Object.fromEntries(ids.map(id => [id, new Element(id)]));
  elements['aggie-messages'].appendChild(new Element()); // Initial greeting.
  const document = {
    getElementById: id => elements[id],
    createElement: () => new Element(),
    addEventListener() {}
  };
  const sessionStorage = {
    getItem: key => storage.get(key) || null,
    setItem: (key, value) => storage.set(key, value)
  };
  vm.runInNewContext(script, {document, sessionStorage, window: {addEventListener() {}}, fetch: fetchImpl});
  return elements;
}

(async () => {
  const storage = new Map();
  const fetchAnswer = async (_url, request) => {
    const query = JSON.parse(request.body).messages.at(-1).content;
    return {ok:true, json:async () => ({reply:'Confirmed: ' + query, source:'live_postgresql'})};
  };
  let page = newPage(storage, fetchAnswer);
  await page['aggie-open'].dispatch('click');
  assert.equal(page['aggie-chat'].hidden, false);
  assert.equal(page['aggie-close'].getAttribute('aria-label'), 'Collapse Aggie');
  page['aggie-input'].value = 'How much fabric?';
  await page['aggie-send'].dispatch('click');
  assert.equal(page['aggie-messages'].lastElementChild.textContent, 'Confirmed: How much fabric?');
  assert.equal(JSON.parse([...storage.values()][0]).messages.length, 2);

  // Simulate navigating to Chapter 04, preserving the open transcript.
  page = newPage(storage, fetchAnswer);
  assert.equal(page['aggie-chat'].hidden, false);
  assert.equal(page['aggie-messages'].children.length, 2);
  assert.equal(page['aggie-messages'].lastElementChild.textContent, 'Confirmed: How much fabric?');
  page['aggie-input'].value = 'How many rolls?';
  await page['aggie-input'].dispatch('input');
  await page['aggie-close'].dispatch('click');
  assert.equal(page['aggie-chat'].hidden, true);

  // Simulate Home. Collapse state and unsent question also persist.
  page = newPage(storage, fetchAnswer);
  assert.equal(page['aggie-chat'].hidden, true);
  assert.equal(page['aggie-input'].value, 'How many rolls?');
  await page['aggie-open'].dispatch('click');
  assert.equal(page['aggie-chat'].hidden, false);

  // An in-flight answer must not be fabricated after the operator changes chapters.
  await page['aggie-input'].dispatch('input');
  page['aggie-input'].value = 'Pending question';
  page['aggie-send'].dispatch('click'); // Deliberately do not await the pending fetch.
  assert.equal(JSON.parse([...storage.values()][0]).pending, true);
  page = newPage(storage, fetchAnswer);
  assert.equal(page['aggie-input'].value, 'Pending question');
  assert.equal(page['aggie-messages'].children.length, 2); // Earlier confirmed exchange only.
  assert.match(page['aggie-status'].textContent, /Press Send to try again/);
  assert.equal(JSON.parse([...storage.values()][0]).pending, false);
  await page['aggie-send'].dispatch('click');
  assert.equal(page['aggie-messages'].lastElementChild.textContent, 'Confirmed: Pending question');
  // A 503 from the AI service is not the same as missing factory records.
  // Keep the user's message ready for retry; do not add false answer bubbles.
  const modelDown = async () => ({ok:false,status:503,json:async () => ({code:'model_unavailable'})});
  const failed = newPage(new Map(), modelDown);
  failed['aggie-input'].value = "what's my factory name";
  await failed['aggie-send'].dispatch('click');
  assert.match(failed['aggie-status'].textContent, /start Ollama/);
  assert.equal(failed['aggie-input'].value, "what's my factory name");
  assert.equal(failed['aggie-messages'].children.length, 1); // Only the original greeting.
  assert.equal(JSON.parse([...storage.values()][0]).pending, false);
  const recordsDown = async () => ({ok:false,status:503,json:async () => ({code:'records_unavailable'})});
  const failedRecords = newPage(new Map(), recordsDown);
  failedRecords['aggie-input'].value = 'how many workers?';
  await failedRecords['aggie-send'].dispatch('click');
  assert.match(failedRecords['aggie-status'].textContent, /check the database/);
  assert.equal(failedRecords['aggie-input'].value, 'how many workers?');
  assert.equal(failedRecords['aggie-messages'].children.length, 1);
  console.log('PASS: Aggie answer history, same-tab navigation, collapse/reopen, unsent input, interrupted request retry');
})().catch(error => { console.error(error); process.exitCode = 1; });
