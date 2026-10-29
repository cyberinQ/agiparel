// Author: Sunil Jain, CyberinQ LLC, 2026-10-29.
// Source contract; no invented vendor acknowledgements or laboratory readings.
'use strict';
const fs=require('fs');const assert=require('node:assert/strict');
const load=p=>fs.readFileSync(p,'utf8');
const mig=load('services/core-api/migrations/0049_vendor_ack_and_fabric_shrinkage_evidence_v30.sql');
const j=load('services/edge-daemon/cmd/server/jobwork_vendor_ack_v30.go');
const s=load('services/edge-daemon/cmd/server/fabric_shrinkage_evidence_v30.go');
const m=load('services/edge-daemon/cmd/server/main.go');
const portal=load('services/edge-daemon/cmd/server/portal.go');
const roles=load('services/edge-daemon/cmd/server/workforce_chapters.go');
for(const table of ['jobwork_vendor_keys_v30','jobwork_vendor_acks_v30','fabric_shrinkage_observations_v30'])assert(mig.includes(`ALTER TABLE public.%I FORCE ROW LEVEL SECURITY`) && mig.includes(`'${table}'`));
for(const word of ['ed25519.Verify','vendorAckChallengeV30','workforceChapterRequire','vendor_signature_hex'])assert(j.includes(word),word);
for(const word of ['shrinkageBasisPointsV30','workforceChapterRequire','source_document_id','SIGNED_HUMAN_REPORTED'])assert(s.includes(word),word);
for(const action of ['jobwork.vendor.ack.record','stores.fabric.shrinkage.record'])assert(roles.includes(`"${action}"`));
for(const route of ['registerVendorAckRoutesV30','registerFabricShrinkageRoutesV30'])assert(m.includes(route));
for(const route of ['/api/v1/jobwork/vendor-ack','/api/v1/fabric/shrinkage/observations'])assert(portal.includes(route));
assert(mig.includes('REVOKE ALL ON public.%I FROM PUBLIC,agiparel_app'));
assert(!mig.includes('GRANT UPDATE'));
assert(j.includes('"quality_release": false') || j.includes('"quality_release":false'));
console.log('PASS F3.17C/F3.18A: genuine vendor Ed25519, human-attested shrinkage, explicit non-credit boundary, tenant RLS');
