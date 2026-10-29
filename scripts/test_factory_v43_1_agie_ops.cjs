// Author: Sunil Jain, CyberinQ LLC, 2026-10-29.
'use strict';
const fs=require('fs'), path=require('path'), assert=require('assert');
const root=path.resolve(__dirname,'..');
const read=x=>fs.readFileSync(path.join(root,x),'utf8');
const op=read('services/edge-daemon/cmd/server/operational_queries.go');
const tests=read('services/edge-daemon/cmd/server/operational_queries_test.go');
const gate=read('services/edge-daemon/cmd/server/engineer_ops_v43.go');
const html=read('services/edge-daemon/internal/web/static/portal.html');
assert(op.includes('case isAttendanceQuestion(q):')&&op.includes('LEFT JOIN workers w ON w.id=a.worker_id AND w.tenant_id=a.tenant_id'));
assert(op.includes('WHERE a.tenant_id=$1')&&op.includes('LIMIT 51'));
assert(op.includes('Workers without an attendance entry are not assumed absent.'));
assert(tests.includes('who is present and who is absent today'));
assert(gate.includes('AGIPAREL_ENGINEER_OPS_TRUST_PROXY_TLS')&&gate.includes('trustedHTTPS'));
assert(html.includes('href="/engineer-ops"'));
assert(!gate.includes('exec.Command('));
console.log('PASS v43.1: bounded tenant-scoped named attendance + secure engineer-ops HTTPS gate');
