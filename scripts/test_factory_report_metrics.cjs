// Author: Sunil Jain, 2026-10-28. Check Chapter 09 source-bound calculations and uncertainty labels.
'use strict';
const assert = require('node:assert/strict');
const fs = require('node:fs');
const vm = require('node:vm');
const path = require('node:path');
const target = path.join(__dirname, '../services/edge-daemon/internal/web/static/factory_report_metrics.js');
const context = {globalThis:{}};
vm.runInNewContext(fs.readFileSync(target,'utf8'), context);
const M = context.globalThis.AgiparelReportMetrics;
assert.equal(M.AUTHORITY, 'REHEARSAL_ONLY_NOT_PRODUCTION_CLOSURE');
const data = {
  lay_id:'52d160cd-980d-41d5-bc0e-909a546b9461',
  authority:M.AUTHORITY, evidence:'DERIVED_FROM_UI_SIMULATED_NO_RF_OBSERVATION',
  stages:[{name:'Sewing completed',count:41},{name:'QC accepted',count:38},{name:'QC held/reinspect',count:2},{name:'Currently packed',count:17}],
  residuals:[{from:'sewn',to:'qc_enrolled',unaccounted:1,status:'OPEN_WIP_OR_UNKNOWN_LOSS'}],
  alerts:['Missing cut verification']
};
const summary = M.summary(data);
assert.equal(summary.sewn,41);
assert.equal(summary.accepted,38);
assert.equal(summary.hold,2);
assert.equal(summary.packed,17);
assert.equal(M.summary({stages:[]}).accepted,null);
assert.equal(M.showCount(null),'Not measured');
assert.equal(M.showCount(0),'0');
assert.equal(M.num(null),'—');
assert.equal(M.num(3.459),'3.46');
assert.ok(M.findings(data).some(s=>s.includes('not matched')));
assert.ok(!M.findings(data).join(' ').includes('scrap'));
assert.ok(M.findings({residuals:[],alerts:[]})[0].includes('does not verify real production'));
assert.ok(M.question(data).includes('PRACTICE-ONLY'));
assert.ok(M.question(data).includes('Do not invent a cause'));
assert.ok(M.question(data).length<=1000);
assert.ok(!M.question(data).includes('real output=41'));
console.log('PASS Chapter 09: known/missing counts, non-causal WIP findings, practice-only Aggie question');
