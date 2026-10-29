// Author: Sunil Jain, CyberinQ LLC, 2026-10-28.
'use strict';
const fs=require('node:fs');const assert=require('node:assert/strict');
const read=p=>fs.readFileSync(p,'utf8');const root='services/edge-daemon/';
const go=read(root+'cmd/server/commissioning_production.go');const main=read(root+'cmd/server/main.go');const portal=read(root+'cmd/server/portal.go');
const html=read(root+'internal/web/static/factory_commissioning.html');const js=read(root+'internal/web/static/factory_commissioning.js');
assert(go.includes('workforceChapterPermit')&&go.includes('"sewing.read"')&&go.includes('"sewing.transfer"'));
assert(go.includes('ReadOnly: true')&&go.includes('app.current_tenant_id')&&go.includes('LevelRepeatableRead'));
assert(go.includes('sew_prod_gateways_v10')&&go.includes('sew_prod_worker_stations_v10')&&go.includes('wip_transfer_routes_v20')&&go.includes('wip_gateway_scans_v20'));
assert(go.includes('AGIPAREL_COMMISSIONING_READ_ENABLED')&&go.includes('CONFIGURED_NOT_PHYSICALLY_CERTIFIED'));
assert(main.includes('registerCommissioningRoutes')&&portal.includes('"/api/v1/commissioning/stations"'));
assert(portal.includes('"/commissioning"')&&portal.includes('"/static/factory_commissioning.js"'));
assert(html.includes('factory_commissioning.js')&&js.includes("credentials:'same-origin'"));
for(const unsafe of ['innerHTML','localStorage','sessionStorage','method:\'POST\''])assert(!js.includes(unsafe),unsafe);
assert(!/INSERT\s+INTO|UPDATE\s+sew_prod|DELETE\s+FROM/i.test(go));
console.log('PASS F3.8: scoped read-only workstation commissioning, no claims of hardware certification');
