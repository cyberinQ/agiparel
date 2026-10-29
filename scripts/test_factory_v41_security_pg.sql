-- Author: Sunil Jain, CyberinQ LLC, 2026-10-29.
-- v41: privilege/RLS/immutability posture of dock trust and evidence tables.
-- Run on disposable PostgreSQL 16 after migration 0057. No mutation.
DO $$
DECLARE
  obj text;
  trust_roots text[] := ARRAY[
    'dock_gateways_v34','dock_carrier_authorizations_v34',
    'dock_station_policies_v34','dock_external_signers_v35',
    'dock_buyer_spec_bindings_v36'];
  evidence text[] := ARRAY[
    'dock_carton_scans_v34','dock_container_seals_v34',
    'dock_carrier_receipts_v34','dock_asn_drafts_v34',
    'dock_external_assertions_v35'];
  all_tables text[];
  r record;
  m uuid := '11111111-1111-4111-8111-111111111111'::uuid;
  status jsonb;
BEGIN
  all_tables := trust_roots || evidence;
  FOREACH obj IN ARRAY all_tables LOOP
    IF to_regclass('public.' || obj) IS NULL THEN
      RAISE EXCEPTION 'F41_MISSING_TABLE: %',obj;
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_class c
      WHERE c.oid=to_regclass('public.' || obj)
        AND c.relkind='r' AND c.relrowsecurity AND c.relforcerowsecurity) THEN
      RAISE EXCEPTION 'F41_RLS_NOT_FORCED: %',obj;
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_policies x WHERE x.schemaname='public'
      AND x.tablename=obj AND x.roles @> ARRAY['agiparel_app']::name[]
      AND x.qual LIKE '%current_tenant_id%' AND x.with_check LIKE '%current_tenant_id%') THEN
      RAISE EXCEPTION 'F41_MISSING_TENANT_POLICY: %',obj;
    END IF;
    IF NOT has_table_privilege('agiparel_app','public.' || obj,'SELECT')
       OR has_table_privilege('agiparel_app','public.' || obj,'UPDATE')
       OR has_table_privilege('agiparel_app','public.' || obj,'DELETE') THEN
      RAISE EXCEPTION 'F41_BAD_APP_PRIVILEGES: %',obj;
    END IF;
  END LOOP;
  FOREACH obj IN ARRAY trust_roots LOOP
    IF has_table_privilege('agiparel_app','public.' || obj,'INSERT') THEN
      RAISE EXCEPTION 'F41_APP_MAY_PROVISION_TRUST: %',obj;
    END IF;
  END LOOP;
  FOREACH obj IN ARRAY evidence LOOP
    IF NOT has_table_privilege('agiparel_app','public.' || obj,'INSERT') THEN
      RAISE EXCEPTION 'F41_APP_CANNOT_APPEND_EVIDENCE: %',obj;
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_trigger x
       WHERE x.tgrelid=to_regclass('public.' || obj)
       AND NOT x.tgisinternal AND x.tgenabled='O'
       AND x.tgname LIKE '%immutable%') THEN
      RAISE EXCEPTION 'F41_EVIDENCE_NOT_IMMUTABLE: %',obj;
    END IF;
  END LOOP;
  FOR r IN SELECT p.oid::regprocedure AS signature FROM pg_proc p
    WHERE p.oid IN (
      'public.dock_diagnostics_v36(text,text,uuid)'::regprocedure,
      'public.dock_lineage_audit_v37(text,text,uuid)'::regprocedure,
      'public.dock_integrity_report_v38(text,text,uuid)'::regprocedure)
      AND p.prosecdef
  LOOP
    RAISE EXCEPTION 'F41_REPORT_PRIVILEGE_ESCALATION: %',r.signature;
  END LOOP;
  IF has_function_privilege('agiparel_app','public.dock_integrity_report_v38(text,text,uuid)','EXECUTE') IS FALSE
    THEN RAISE EXCEPTION 'F41_MISSING_REPORT_GRANT';
  END IF;
  status:=dock_integrity_report_v38('F41_NONEXISTENT_TENANT','F41_PLANT',m);
  IF COALESCE((status->>'evidence_review_complete')::boolean,true)
    OR COALESCE((status->>'shipment_credit')::boolean,true)
    OR COALESCE((status->>'global_fabric_stock_conserved')::boolean,true)
    OR COALESCE((status->>'financial_ledger_conserved')::boolean,true) THEN
    RAISE EXCEPTION 'F41_EMPTY_MANIFEST_OR_CREDIT_FALSE_POSITIVE';
  END IF;
  RAISE NOTICE 'PASS v41: forced RLS, app least privilege, immutable evidence, read-only reports and no false credit';
END $$;
