-- Author: Sunil Jain, CyberinQ LLC, 2026-10-29.
-- DANGEROUS TEST FIXTURES: run ONLY against a disposable PostgreSQL 16 clone.
-- The whole transaction is rolled back. Dummy proofs are DB-trigger fixtures ONLY;
-- NEVER substitute for the two real JWT signatures required by the HTTP API.
\set ON_ERROR_STOP on
BEGIN;
DO $$
DECLARE
 t text := 'F314-T-'||substr(md5(clock_timestamp()::text),1,12);
 p text := 'PLANT-F314'; wid text := 'WRK-F314'; sid text := 'STY-F314';
 oid text := 'ORD-F314'; lid text := 'LINE-F314'; tid text := 'TRIM-F314';
 remaining numeric; cnt integer;
BEGIN
 INSERT INTO tenants(id,name) VALUES(t,'F3.14 ROLLBACK FIXTURE');
 INSERT INTO workers(id,tenant_id,name,employee_code,primary_skill) VALUES(wid,t,'Temporary supervisor','F314','SEWING');
 INSERT INTO styles_master(id,tenant_id,style_code) VALUES(sid,t,'F314-TEST-STYLE');
 INSERT INTO dynamic_work_orders(id,tenant_id,plant_id,po_number,style_id,line_id,supervisor_id,ordered_qty,status)
 VALUES(oid,t,p,'F314-TEST-PO',sid,lid,wid,10,'LOCKED');
 INSERT INTO store_trims_v13(tenant_id,id,plant_id,trim_code,description,supplier_lot,uom,received_qty,remaining_qty)
 VALUES(t,tid,p,'BUTTON-F314','Temporary button lot','TEST-LOT','PCS',10,10);
 INSERT INTO trim_issuances_v13(tenant_id,plant_id,trim_stock_id,order_id,line_id,issued_qty,issuer_id,recipient_id,
 command_id,issuer_proof_id,recipient_proof_id,challenge_sha256,issuer_assertion_sha256,recipient_assertion_sha256)
 VALUES(t,p,tid,oid,lid,6.500,'ISSUER','RECIPIENT','trim_fixture_0001','jwt-fixture-A','jwt-fixture-B',repeat('a',64),repeat('b',64),repeat('c',64));
 SELECT remaining_qty INTO remaining FROM store_trims_v13 WHERE tenant_id=t AND id=tid;
 IF remaining<>3.500 THEN RAISE EXCEPTION 'ERR_F314_BALANCE_NOT_ATOMIC'; END IF;
 BEGIN
  INSERT INTO trim_issuances_v13(tenant_id,plant_id,trim_stock_id,order_id,line_id,issued_qty,issuer_id,recipient_id,
  command_id,issuer_proof_id,recipient_proof_id,challenge_sha256,issuer_assertion_sha256,recipient_assertion_sha256)
  VALUES(t,p,tid,oid,lid,4,'ISSUER','RECIPIENT','trim_fixture_0002','jwt-fixture-C','jwt-fixture-D',repeat('a',64),repeat('b',64),repeat('c',64));
  RAISE EXCEPTION 'ERR_F314_OVERISSUE_ACCEPTED';
 EXCEPTION WHEN OTHERS THEN
  IF SQLERRM NOT LIKE '%ERR_F314_INSUFFICIENT_STOCK%' THEN RAISE; END IF;
 END;
 BEGIN
  INSERT INTO trim_issuances_v13(tenant_id,plant_id,trim_stock_id,order_id,line_id,issued_qty,issuer_id,recipient_id,
  command_id,issuer_proof_id,recipient_proof_id,challenge_sha256,issuer_assertion_sha256,recipient_assertion_sha256)
  VALUES(t,p,tid,oid,lid,1,'ISSUER','RECIPIENT','trim_fixture_0003','jwt-fixture-A','jwt-fixture-X',repeat('a',64),repeat('b',64),repeat('c',64));
  RAISE EXCEPTION 'ERR_F314_PROOF_REUSE_ACCEPTED';
 EXCEPTION WHEN unique_violation THEN NULL;
 END;
 BEGIN
  INSERT INTO trim_issuances_v13(tenant_id,plant_id,trim_stock_id,order_id,line_id,issued_qty,issuer_id,recipient_id,
  command_id,issuer_proof_id,recipient_proof_id,challenge_sha256,issuer_assertion_sha256,recipient_assertion_sha256)
  VALUES(t,'WRONG-PLANT',tid,oid,lid,1,'ISSUER','RECIPIENT','trim_fixture_0004','jwt-fixture-Y','jwt-fixture-Z',repeat('a',64),repeat('b',64),repeat('c',64));
  RAISE EXCEPTION 'ERR_F314_WRONG_PLANT_ACCEPTED';
 EXCEPTION WHEN OTHERS THEN
  IF SQLERRM NOT LIKE '%ERR_F314_STOCK_PLANT_SCOPE%' THEN RAISE; END IF;
 END;
 SELECT remaining_qty INTO remaining FROM store_trims_v13 WHERE tenant_id=t AND id=tid;
 SELECT count(*) INTO cnt FROM trim_issuances_v13 WHERE tenant_id=t;
 IF remaining<>3.500 OR cnt<>1 THEN RAISE EXCEPTION 'ERR_F314_REJECTION_NOT_ROLLED_BACK'; END IF;
 RAISE NOTICE 'PASS F3.14 disposable: exact balance, overspend blocked, replay proof blocked, wrong plant blocked, no persisted fixtures';
END $$;
ROLLBACK;
