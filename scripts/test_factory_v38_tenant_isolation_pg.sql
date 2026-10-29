-- Author: Sunil Jain, 2026-10-29. Execute on disposable DB only.
BEGIN;
INSERT INTO tenants(id,name) VALUES('F38_ISOLATION_A','F38 A'),('F38_ISOLATION_B','F38 B');
SET LOCAL ROLE agiparel_app;
SELECT set_config('app.current_tenant_id','F38_ISOLATION_A',true);
DO $$ DECLARE report jsonb; BEGIN
 SELECT dock_integrity_report_v38('F38_ISOLATION_B','PLANT-01',
  '11111111-1111-4111-8111-111111111111'::uuid) INTO report;
 IF coalesce((report->>'evidence_review_complete')::boolean,false)
 OR coalesce((report->'material'->>'material_attribution_complete')::boolean,false)
 OR (report->'material'->>'garments')::bigint<>0
 THEN RAISE EXCEPTION 'ERR_F38_CROSS_TENANT_EVIDENCE'; END IF;
END $$;
SELECT set_config('app.current_tenant_id','',true);
DO $$ DECLARE report jsonb; BEGIN
 SELECT dock_integrity_report_v38('F38_ISOLATION_A','PLANT-01',
  '11111111-1111-4111-8111-111111111111'::uuid) INTO report;
 IF coalesce((report->>'evidence_review_complete')::boolean,false) THEN
  RAISE EXCEPTION 'ERR_F38_MISSING_RLS_CONTEXT_ALLOWED'; END IF;
END $$;
ROLLBACK;
