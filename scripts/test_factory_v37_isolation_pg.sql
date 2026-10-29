-- Author: Sunil Jain, CyberinQ LLC, 2026-10-29.
-- Disposable DB only. Verify tenant-scoped execution and no database mutation.
BEGIN;
INSERT INTO tenants(id,name) VALUES ('F37_ISOLATION_A','V37 A'),('F37_ISOLATION_B','V37 B');
SET LOCAL ROLE agiparel_app;
SELECT set_config('app.current_tenant_id','F37_ISOLATION_A',true);
DO $$ DECLARE result jsonb; BEGIN
 SELECT dock_lineage_audit_v37('F37_ISOLATION_B','PLANT-01',
   '11111111-1111-4111-8111-111111111111'::uuid) INTO result;
 IF (result->>'lineage_evidence_complete')::boolean
 THEN RAISE EXCEPTION 'ERR_F37_CROSS_TENANT_AUDIT'; END IF;
 IF (result->>'garments')::bigint<>0 THEN RAISE EXCEPTION 'ERR_F37_CROSS_TENANT_COUNTS'; END IF;
END $$;
SELECT set_config('app.current_tenant_id','',true);
DO $$ DECLARE result jsonb; BEGIN
 SELECT dock_lineage_audit_v37('F37_ISOLATION_A','PLANT-01',
  '11111111-1111-4111-8111-111111111111'::uuid) INTO result;
 IF (result->>'lineage_evidence_complete')::boolean
 THEN RAISE EXCEPTION 'ERR_F37_NO_TENANT_BYPASS'; END IF;
END $$;
ROLLBACK;
