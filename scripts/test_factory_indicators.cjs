// Author: Sunil Jain, 2026-10-28. Verify 8-chapter catalog, metric provenance, and missing-value safety.
'use strict';
const assert=require('node:assert/strict');
const fs=require('node:fs');
const path=require('node:path');
const vm=require('node:vm');
const root=path.join(__dirname,'../services/edge-daemon/internal/web/static');
const scope={globalThis:null};scope.globalThis=scope;
vm.runInNewContext(fs.readFileSync(path.join(root,'factory_indicator_catalog.js'),'utf8'),scope);
const catalog=scope.AgiparelIndicatorCatalog;
assert.equal(catalog.chapter.length,8);
assert.equal(catalog.indicators.length,24);
assert.equal(new Set(catalog.indicators.map(x=>x.id)).size,24);
for(const item of catalog.indicators){
  assert.ok(item.chapter.url.startsWith('/'));
  assert.ok(item.definition.length>20);
  assert.equal(catalog.resolve(item,null).status,'NOT_MEASURED');
}
const report={authority:'REHEARSAL_ONLY_NOT_PRODUCTION_CLOSURE',lay_id:'lay-1',evidence:'SIMULATED',
  cut_marker_positions:34,cut_bundle_candidate_units:8,
  stages:[{name:'Sewing completed',count:41},{name:'QC accepted',count:38},{name:'QC held/reinspect',count:2},
    {name:'Currently packed',count:17},{name:'In sealed carton',count:10}],
  residuals:[{from:'sewn',to:'qc_enrolled',unaccounted:2,status:'OPEN_WIP_OR_UNKNOWN_LOSS'}],
  operation_stats:[{operation:'hem',issue_events:2,stats:{sample_count:7,p50_minutes:.8,p90_minutes:1.3,p95_minutes:1.6}}]
};
const get=id=>catalog.resolve(catalog.indicators.find(x=>x.id===id),report);
assert.equal(get('cut_positions').value,34);
assert.equal(get('cut_candidates').value,8);
assert.equal(get('cut_handoff_gaps').value,1);
assert.equal(get('sew_complete').value,41);
assert.equal(get('sew_pace').value,1);
assert.equal(get('sew_issues').value,2);
assert.equal(get('qc_accepted').value,38);
assert.equal(get('qc_holds').value,2);
assert.equal(get('packing_current').value,17);
assert.equal(get('packing_sealed').value,10);
for(const id of ['material_coverage','bom_coverage','qc_first_pass','packing_delivery','stock_admitted','setup_coverage']) assert.equal(get(id).status,'NOT_MEASURED');
assert.equal(catalog.resolve(catalog.indicators.find(x=>x.id==='sew_complete'),{...report,authority:'OBSERVED_EVENT'}).status,'NOT_MEASURED');
const empty={...report,stages:[],operation_stats:[{operation:'hem',issue_events:0,stats:{sample_count:2,p50_minutes:.8,p90_minutes:1.3,p95_minutes:1.6}}]};
assert.equal(catalog.resolve(catalog.indicators.find(x=>x.id==='sew_pace'),empty).status,'NOT_MEASURED');
assert.equal(catalog.resolve(catalog.indicators.find(x=>x.id==='qc_accepted'),empty).status,'NOT_MEASURED');
console.log('PASS: 24 catalog metrics, exact backend mappings, source authority, sample limits, unknowns stay unknown.');
