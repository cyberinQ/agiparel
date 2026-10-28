// Author: Sunil Jain, 2026-10-28.
// Verify SSO session reads never transmit pasted identity assertions or save tokens.
'use strict';
const assert=require('node:assert/strict');const fs=require('node:fs');const vm=require('node:vm');
const ids=['since','until','token','authmode','oidcscope','tenant','plant','refresh','clear','status','scope','measures','notes','sew','qc','held','stage','signin','signout','session-status'];
const nodes=Object.fromEntries(ids.map(id=>[id,{value:'',textContent:'',hidden:false,children:[],events:{},addEventListener(t,f){this.events[t]=f;},replaceChildren(){this.children=[];},append(x){this.children.push(x);}}]));
const calls=[];
const ctx={document:{getElementById:id=>nodes[id],createElement:t=>({tag:t,children:[],append(x){this.children.push(x)}})},Number,Object,Date,URLSearchParams,Error,console,window:{location:{assign:()=>{}}},fetch:async(u,opts)=>{
 calls.push({u,opts});
 if(u==='/api/v1/workforce/session')return {ok:true,json:async()=>({authenticated:true,issuer:'https://idp.example.test',subject:'worker-7',csrf:'csrf-secret'})};
 if(u==='/auth/logout')return {ok:true,json:async()=>({logged_out:true})};
 return {ok:false,json:async()=>({error:'No records in test'})};
}};
vm.runInNewContext(fs.readFileSync('services/edge-daemon/internal/web/static/factory_intelligence_production.js','utf8'),ctx);
(async()=>{
 await new Promise(resolve=>setImmediate(resolve));
 assert.match(nodes['session-status'].textContent,/Signed in as/);
 nodes.authmode.value='session';nodes.authmode.events.change();
 nodes.tenant.value='tenant_A';nodes.plant.value='plant_B';nodes.token.value='do-not-send-this';
 nodes.since.value='2026-10-20T10:00';nodes.until.value='2026-10-21T10:00';
 await nodes.refresh.events.click();
 const read=calls.find(x=>x.u.startsWith('/api/v1/intelligence/production/report?'));
 assert(read);assert.equal(read.opts.credentials,'same-origin');assert.equal(Object.keys(read.opts.headers).length,0);
 assert(read.u.includes('tenant_id=tenant_A') && read.u.includes('plant_id=plant_B'));
 await nodes.signout.events.click();
 const logout=calls.find(x=>x.u==='/auth/logout');
 assert.equal(logout.opts.method,'POST');assert.equal(logout.opts.headers['X-Agiparel-CSRF'],'csrf-secret');
 assert.equal(logout.opts.credentials,'same-origin');
 assert.match(nodes['session-status'].textContent,/Signed out/);
 assert(!JSON.stringify(calls).includes('do-not-send-this'));
 console.log('PASS F1.2: authenticated browser session, read-only scope, CSRF sign-out, no leaked bearer');
})().catch(e=>{console.error(e);process.exitCode=1;});
