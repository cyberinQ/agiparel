-- Author: Sunil Jain, CyberinQ LLC, 2026-10-29.
-- Run ONLY on a disposable test database; no production fixture writes.
BEGIN;
INSERT INTO tenants(id,name) VALUES ('F36_ISOLATION_A','V36 A'),('F36_ISOLATION_B','V36 B');
SET LOCAL ROLE agiparel_app;
SELECT set_config('app.current_tenant_id','F36_ISOLATION_A',true);
DO $$ BEGIN
 IF EXISTS (SELECT 1 FROM dock_buyer_spec_bindings_v36 WHERE tenant_id='F36_ISOLATION_B')
 THEN RAISE EXCEPTION 'ERR_F36_CROSS_TENANT_BINDINGS_VISIBLE'; END IF;
 BEGIN
  INSERT INTO dock_buyer_spec_bindings_v36(tenant_id,plant_id,spec_id,buyer_code,contract_reference,approved_by)
  VALUES ('F36_ISOLATION_A','PLANT-01','11111111-1111-4111-8111-111111111111',
   'BUYER-TEST','CONTRACT-TEST-001','UNAUTHORIZED');
  RAISE EXCEPTION 'ERR_F36_UNPRIVILEGED_ENROLLMENT_SUCCEEDED';
 EXCEPTION WHEN insufficient_privilege THEN NULL; END;
END $$;
SELECT set_config('app.current_tenant_id','',true);
DO $$ BEGIN
 IF EXISTS(SELECT 1 FROM dock_buyer_spec_bindings_v36) THEN
  RAISE EXCEPTION 'ERR_F36_EMPTY_TENANT_LEAK'; END IF;
END $$;
ROLLBACK;
