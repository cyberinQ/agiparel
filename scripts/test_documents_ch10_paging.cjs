// Author: Sunil Jain, 2026-10-28.
// Verify Chapter 10 bounded library/source pagination, authority labels, and context isolation.
'use strict';
const assert=require('node:assert/strict');
const vm=require('node:vm');
const fs=require('node:fs');
const path=require('node:path');
const html=fs.readFileSync(path.join(__dirname,'../services/edge-daemon/internal/web/static/documents.html'),'utf8');
const source=fs.readFileSync(path.join(__dirname,'../services/edge-daemon/internal/web/static/documents.js'),'utf8');
for(const key of ['library-more','source-more','library-progress','show-history'])assert.ok(html.includes(`id="${key}"`),`missing ${key}`);
class E {
 constructor(tag='div'){this.tagName=tag;this.children=[];this.listeners={};this._text='';this.value='';this.hidden=false;this.disabled=false;this.className='';this.classList={add(){},remove(){}};this.attributes={};this.style={};}
 set textContent(v){this._text=String(v??'');this.children=[];}
 get textContent(){return this._text+this.children.map(c=>c.textContent).join('');}
 append(...nodes){this.children.push(...nodes);for(const n of nodes)n.parentElement=this;}
 replaceChildren(...nodes){this._text='';this.children=[];this.append(...nodes);}
 addEventListener(name,cb){(this.listeners[name]??=[]).push(cb);}
 fire(name,extra={}){for(const cb of this.listeners[name]||[])cb({target:this,preventDefault(){},...extra});}
 setAttribute(k,v){this.attributes[k]=v;}
 scrollIntoView(){}
}
const ids=['feedback','files','selection','upload','drop','show-history','doc-list','total','library-progress','library-more','source-more','sort-by','sort-direction','refresh','source-details','inspect-title','inspect-desc','inspect-sha','inspect-readiness','download-source','search','evidence-message','segments','review-submit'];
const nodes=Object.fromEntries(ids.map(id=>[id,new E()]));
nodes['library-more'].hidden=true;nodes['source-more'].hidden=true;
const callbacks={};const viewed=[];
const document={getElementById:id=>{assert.ok(nodes[id],`unknown node ${id}`);return nodes[id]},createElement:tag=>new E(tag),dispatchEvent:e=>viewed.push(e.type)};
const d=(n,v=1)=>({id:`DOC-${String(n).padStart(32,'0')}`,filename:`src${n}.pdf`,captured_at:`2026-10-28T21:00:${String(60-n).padStart(2,'0')}Z`,version:v,version_count:v,current_version:true,format:'pdf',status:'EXTRACTED',segment_count:3,sha256:'a'.repeat(64)});
const fetchLog=[];
async function fetch(url){fetchLog.push(url);
 let body;
 if(url==='/api/v1/documents?limit=50')body={items:[d(1),d(2)],next_cursor:'PAGE_2',has_more:true};
 else if(url==='/api/v1/documents?limit=50&cursor=PAGE_2')body={items:[d(3),d(4)],next_cursor:'',has_more:false};
 else if(url===`/api/v1/documents/${d(1).id}`)body={document:d(1),segments:[{ordinal:1,kind:'text',content:'first',locator:{page:1}}],has_more_segments:true,next_ordinal:1};
 else if(url===`/api/v1/documents/${d(1).id}?after_ordinal=1`)body={document:d(1),segments:[{ordinal:2,kind:'text',content:'second',locator:{page:2}}],has_more_segments:false,next_ordinal:0};
 else throw Error(`unexpected URL ${url}`);
 return {ok:true,json:async()=>body};
}
const ctx=vm.createContext({document,fetch,URLSearchParams,window:{location:{search:'?document='+d(1).id,hash:''}},setInterval(){},CustomEvent:class{constructor(type,opts){this.type=type;this.detail=opts.detail;}},console,Array,String,Number,Map,Set,Date,JSON,Error,encodeURIComponent});
vm.runInContext(source,ctx,{filename:'documents.js'});
const settle=async()=>{for(let i=0;i<25;i++)await Promise.resolve();};
(async()=>{
 await settle();
 assert.equal(nodes['library-more'].hidden,false,'older documents must be accessible');
 assert.match(nodes.total.textContent,/2 loaded/,'must not present capped count as total');
 assert.equal(nodes['source-more'].hidden,false);
 assert.match(nodes['evidence-message'].textContent,/more evidence available/);
 nodes['source-more'].fire('click');await settle();
 assert.equal(nodes['source-more'].hidden,true);
 assert.equal(nodes.segments.children.length,2,'source continuation must append, not replace');
 assert.match(nodes['evidence-message'].textContent,/end of extracted evidence/);
 nodes['library-more'].fire('click');await settle();
 assert.equal(nodes['library-more'].hidden,true);
 assert.match(nodes['library-progress'].textContent,/4 revisions loaded/);
 assert.equal(nodes['doc-list'].children.length,4);
 assert.ok(fetchLog.includes('/api/v1/documents?limit=50&cursor=PAGE_2'));
 assert.ok(viewed.includes('agiparel-documents-updated'));
 console.log('PASS Chapter 10: multi-page catalog, source evidence, accurate loaded counts and review updates');
})().catch(e=>{console.error(e);process.exitCode=1;});
