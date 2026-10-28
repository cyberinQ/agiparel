// Author: Sunil Jain, 2026-10-28
// Dependency-free DOM test: every arrow targets the adjacent chapter, never browser history.
'use strict';
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');
const script = fs.readFileSync(path.join(__dirname, '../services/edge-daemon/internal/web/static/workspace_nav.js'), 'utf8');

class Element {
  constructor(tag = '') {
    this.tag = tag;
    this.children = [];
    this.attributes = {};
    this.classList = {add() {}};
  }
  setAttribute(k, v) { this.attributes[k] = v; }
  append(...children) { this.children.push(...children); }
  prepend(...children) { this.children.unshift(...children); }
  replaceChildren(...children) { this.children = [...children]; }
  addEventListener() {}
}

const allLinks = element => {
  const visited = [];
  const walk = n => {
    if (!n || typeof n !== 'object') return;
    if (n.tag === 'a' && n.href) visited.push(n);
    for (const child of n.children || []) walk(child);
  };
  walk(element);
  return visited;
};

function renderChapter(url) {
  const body = new Element('body'); body.dataset = {};
  const document = {
    readyState:'complete', body,
    head:new Element('head'),
    getElementById:id => id === 'aggie-open' || id === 'aggie-chat' ? new Element('button') : null,
    createElement:tag => new Element(tag),
    createTextNode:text => text
  };
  vm.runInNewContext(script, {document,location:{pathname:url},window:{}});
  return {links:allLinks(body), phase:body.dataset.agPhase};
}

const names = [
  '/factory-setup','/buyer-review','/material-planning','/stores','/cutting',
  '/sewing','/quality','/packing','/intelligence','/documents'
];
for (let i=0; i<names.length; i++) {
  const page = renderChapter(names[i]);
  const nav = page.links.filter(item => item.className === 'ag-chapter-history');
  assert.equal(nav.length, Number(i>0)+Number(i<9), `chapter ${i+1}: only live arrows`);
  if (i>0) assert.equal(nav[0].href, names[i-1], `chapter ${i+1}: previous`);
  if (i<9) assert.equal(nav[nav.length-1].href, names[i+1], `chapter ${i+1}: next`);
  assert.equal(page.links.filter(item=>item.className==='ag-chapter-home').length,1);
}
assert.equal(renderChapter('/order-intake').links.find(item=>item.attributes['aria-label'] === 'Next chapter: Plan material').href, '/material-planning');
assert.equal(renderChapter('/sew01').links.find(item=>item.attributes['aria-label'] === 'Next chapter: Quality').href, '/quality');
console.log('PASS: real chapter links for 01–10 and older aliases; unavailable endpoint arrows omitted');
