// Author: Sunil Jain, 2026-10-28. DOM-level click-through test for the indicator dashboard and full-screen detail view.
'use strict';
const assert=require('node:assert/strict');
const fs=require('node:fs');
const vm=require('node:vm');
const path=require('node:path');
const root=path.join(__dirname,'../services/edge-daemon/internal/web/static');
const ids=['indicator-grid','indicator-view','indicator-close','indicator-chapter','indicator-title','indicator-state',
  'indicator-value','indicator-unit','indicator-definition','indicator-caution','indicator-explanation',
  'indicator-evidence','indicator-source','indicator-source-link','indicator-report-link','indicator-checks','indicator-plot','indicator-graph-note'];
let focused=null;
const listeners={};
function node(tag='div'){
  return {tag,children:[],hidden:false,textContent:'',className:'',href:'',isConnected:true,attrs:{},handlers:{},
    style:{setProperty(k,v){this[k]=v}},classList:{add(){},remove(){}},
    append(...items){this.children.push(...items)},replaceChildren(...items){this.children=[...items]},
    addEventListener(k,fn){this.handlers[k]=fn},setAttribute(k,v){this.attrs[k]=v},
    focus(){focused=this},querySelectorAll(){return []},click(){this.handlers.click?.()}};
}
const els=new Map(ids.map(id=>[id,node()]));
els.get('indicator-view').hidden=true;
const doc={
  getElementById:id=>els.get(id),createElement:tag=>node(tag),
  activeElement:null,body:{classList:{add(){},remove(){}}},
  addEventListener:(kind,handler)=>{listeners[kind]=handler}
};
const scope={document:doc,globalThis:null,console};scope.globalThis=scope;
function run(file){vm.runInNewContext(fs.readFileSync(path.join(root,file),'utf8'),scope)}
run('factory_indicator_catalog.js');run('factory_indicator_panel.js');
const groups=els.get('indicator-grid').children;
assert.equal(groups.length,8);
assert.ok(groups.every(g=>g.children.length===4));
groups[0].children[1].click();
assert.equal(els.get('indicator-view').hidden,false);
assert.equal(els.get('indicator-state').textContent,'NOT MEASURED');
assert.equal(els.get('indicator-source-link').href,'/factory-setup');
assert.ok(els.get('indicator-plot').children.length>0);
els.get('indicator-close').click();
assert.equal(els.get('indicator-view').hidden,true);
const report={authority:'REHEARSAL_ONLY_NOT_PRODUCTION_CLOSURE',lay_id:'LAY-1',evidence:'SIMULATED',generated_at:'2026-10-28T00:00:00Z',
  stages:[{name:'Sewing completed',count:41},{name:'QC accepted',count:38},{name:'QC held/reinspect',count:2},{name:'Currently packed',count:17},{name:'In sealed carton',count:10}],
  operation_stats:[{operation:'hem',issue_events:2,stats:{sample_count:7,p50_minutes:.8,p90_minutes:1.3,p95_minutes:1.6}}]};
listeners['ag:report-loaded']({detail:report});
els.get('indicator-grid').children[5].children[2].click();
assert.equal(els.get('indicator-state').textContent,'SIMULATION / PRACTICE EVIDENCE');
assert.equal(els.get('indicator-value').textContent,'1');
assert.equal(els.get('indicator-source-link').href,'/sewing');
assert.ok(els.get('indicator-plot').children.length>0);
listeners.keydown({key:'Escape',preventDefault(){}});
assert.equal(els.get('indicator-view').hidden,true);
listeners['ag:report-cleared']();
els.get('indicator-grid').children[5].children[2].click();
assert.equal(els.get('indicator-state').textContent,'NOT MEASURED');
console.log('PASS: 8 chapters, 24 clickable indicators, full screen, valid practice plot, missing state, source links, close/reopen.');
