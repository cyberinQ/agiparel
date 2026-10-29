// Author: Sunil Jain, CyberinQ LLC, 2026-10-29.
const assert=require('node:assert/strict'); const fs=require('node:fs');
const read=p=>fs.readFileSync(p,'utf8');
const sql=read('services/core-api/migrations/0052_factory_safety_aql_and_dock_readiness_v33.sql');
const go=read('services/edge-daemon/cmd/server/factory_release_v33.go');
const portal=read('services/edge-daemon/cmd/server/portal.go');
const main=read('services/edge-daemon/cmd/server/main.go');
const grants=read('services/edge-daemon/cmd/server/workforce_chapters.go');
for(const fragment of ['needle_safety_clearances_v33','buyer_aql_review_rules_v33','buyer_aql_reviews_v33',
 'garment_needle_safety_blocked_v32','accounted_fragments<>incident.expected_fragments',
 "followup.event_kind<>'DETECTOR_CHALLENGE_PASS'",'CREATE POLICY factory_tenant_v33',
 'FORCE ROW LEVEL SECURITY','guard_factory_review_proof_v33','NOT EXISTS(SELECT 1 FROM needle_safety_clearances_v33']){
 assert(sql.includes(fragment),'Missing SQL guard: '+fragment);
}
for(const fragment of ['INTERNAL_RECOMMEND_ACCEPT','HOLD_FOR_BUYER_REVIEW',
  'factoryReleaseChallengeV33','workforceChapterRequire','SET LOCAL ROLE agiparel_app',
  'AGIPAREL_FACTORY_RELEASE_V33_ENABLED','newChapterAccessAPI','packing.read',
  'garment_needle_safety_blocked_v32','carrier_receipt_verified',
  'buyer_acceptance','shipment_credit','factoryReleasePathV33']){
 assert(go.includes(fragment),'Missing handler boundary: '+fragment);
}
for(const route of ['/api/v1/quality/safety/clearances/challenge','/api/v1/quality/safety/clearances',
 '/api/v1/quality/aql/reviews/challenge','/api/v1/quality/aql/reviews','/api/v1/packing/dock/readiness']){
 assert(portal.includes('"'+route+'"'),'Unauthorized ingress path '+route);
 assert(go.includes('"'+route+'"'),'Missing endpoint '+route);
}
assert(main.includes('registerFactoryReleaseRoutesV33(mux, server)'));
for(const permission of ['quality.needle.clear','quality.aql.review'])assert(grants.includes('"'+permission+'"'));
assert(!go.includes('shipment_credit": true')); assert(!go.includes('buyer_acceptance": true'));
console.log('PASS F3.19C/D + F3.20A: signed independent safety and AQL review, scoped read-only dock preflight, no shipment claim');
