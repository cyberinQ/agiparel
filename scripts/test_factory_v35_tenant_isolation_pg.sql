-- Author: Sunil Jain, CyberinQ LLC, 2026-10-29.
-- Must run against a disposable restored database. Inserts rollback on success.
BEGIN;
INSERT INTO tenants(id,name) VALUES ('F35_ISOLATION_A','V35 isolation A'),('F35_ISOLATION_B','V35 isolation B');
INSERT INTO dock_external_signers_v35(tenant_id,plant_id,signer_id,party_kind,party_code,public_key,enabled,approved_by)
VALUES ('F35_ISOLATION_A','PLANT-01','BUYER-PROOF-1','BUYER','BUYER-A',decode(repeat('ab',32),'hex'),true,'CI_PROVISIONER'),
       ('F35_ISOLATION_B','PLANT-01','BUYER-PROOF-1','BUYER','BUYER-B',decode(repeat('cd',32),'hex'),true,'CI_PROVISIONER');
SET LOCAL ROLE agiparel_app;
SELECT set_config('app.current_tenant_id','F35_ISOLATION_A',true);
DO $$ BEGIN
 IF (SELECT count(*) FROM dock_external_signers_v35)<>1
 OR EXISTS(SELECT 1 FROM dock_external_signers_v35 WHERE party_code='BUYER-B')
 THEN RAISE EXCEPTION 'ERR_F35_CROSS_TENANT_A'; END IF;
END $$;
-- A provisioned signing key is necessary but insufficient: no ASN draft or
-- staged manifest exists, so inserting apparent ACK evidence must fail closed.
DO $$ BEGIN
 BEGIN
  INSERT INTO dock_external_assertions_v35
   (tenant_id,plant_id,manifest_id,asn_draft_id,signer_id,party_kind,party_code,assertion_kind,
    decision,event_id,external_reference,asn_reference,payload_sha256,issued_at,
    event_sha256,signer_key_sha256,signature_sha256)
  VALUES ('F35_ISOLATION_A','PLANT-01','11111111-1111-4111-8111-111111111111',
    '22222222-2222-4222-8222-222222222222','BUYER-PROOF-1','BUYER','BUYER-A',
    'BUYER_LOT_REVIEW','REVIEWED_ACCEPT','f35-negative-proof-000001','REVIEW-A-001','ASN-001',
    repeat('a',64),clock_timestamp(),repeat('b',64),repeat('c',64),repeat('d',64));
  RAISE EXCEPTION 'ERR_F35_FAKE_ACK_ACCEPTED';
 EXCEPTION WHEN OTHERS THEN
  IF SQLERRM NOT LIKE '%ERR_F35_EXTERNAL_ASSERTION_NOT_CURRENT%'
  THEN RAISE EXCEPTION 'ERR_F35_WRONG_FAIL_CLOSED: %',SQLERRM; END IF;
 END;
END $$;
SELECT set_config('app.current_tenant_id','F35_ISOLATION_B',true);
DO $$ BEGIN
 IF (SELECT count(*) FROM dock_external_signers_v35)<>1
 OR EXISTS(SELECT 1 FROM dock_external_signers_v35 WHERE party_code='BUYER-A')
 THEN RAISE EXCEPTION 'ERR_F35_CROSS_TENANT_B'; END IF;
END $$;
SELECT set_config('app.current_tenant_id','',true);
DO $$ BEGIN
 IF EXISTS(SELECT 1 FROM dock_external_signers_v35) THEN RAISE EXCEPTION 'ERR_F35_EMPTY_TENANT_LEAK'; END IF;
END $$;
ROLLBACK;
