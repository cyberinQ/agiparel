-- Author: Sunil Jain, 2026-10-29. Schema and incomplete-evidence negative tests.
DO $$
DECLARE report jsonb; material jsonb; fn text; function_name text;
BEGIN
 FOREACH function_name IN ARRAY ARRAY['dock_material_trace_v38(text,text,uuid)',
  'dock_integrity_report_v38(text,text,uuid)'] LOOP
  IF to_regprocedure('public.'||function_name) IS NULL THEN
   RAISE EXCEPTION 'ERR_F38_MISSING_FUNCTION %',function_name;
  END IF;
  IF NOT has_function_privilege('agiparel_app',function_name,'EXECUTE') THEN
   RAISE EXCEPTION 'ERR_F38_APP_EXECUTE_MISSING %',function_name;
  END IF;
  SELECT pg_get_functiondef(to_regprocedure('public.'||function_name)) INTO fn;
  IF position('SECURITY DEFINER' IN fn)>0 THEN RAISE EXCEPTION 'ERR_F38_PRIVILEGE_BYPASS'; END IF;
  IF EXISTS(SELECT 1 FROM pg_proc pr CROSS JOIN LATERAL
     aclexplode(coalesce(pr.proacl,acldefault('f',pr.proowner))) acl
   WHERE pr.oid=to_regprocedure('public.'||function_name)
    AND acl.grantee=0 AND acl.privilege_type='EXECUTE') THEN
   RAISE EXCEPTION 'ERR_F38_PUBLIC_EXECUTE';
  END IF;
 END LOOP;
 SELECT dock_material_trace_v38('missing-tenant','PLANT-01',
   '11111111-1111-4111-8111-111111111111'::uuid) INTO material;
 SELECT dock_integrity_report_v38('missing-tenant','PLANT-01',
   '11111111-1111-4111-8111-111111111111'::uuid) INTO report;
 IF coalesce((material->>'material_attribution_complete')::boolean,false)
 OR coalesce((report->>'evidence_review_complete')::boolean,false)
 OR (report->>'shipment_credit')::boolean
 OR (report->>'global_fabric_stock_conserved')::boolean
 OR (report->>'financial_ledger_conserved')::boolean
 OR (report->>'asn_network_delivery_proven')::boolean
 OR jsonb_array_length(material->'blockers')=0 THEN
  RAISE EXCEPTION 'ERR_F38_MINTED_AUTHORITY_FROM_ABSENT_EVIDENCE';
 END IF;
 -- All cross-ledger claim checks remain in current code, not a frozen approval.
 SELECT pg_get_functiondef('dock_material_trace_v38(text,text,uuid)'::regprocedure) INTO fn;
 IF position('cut_spread_issue_allocations_v8' IN fn)=0
 OR position('material_issues_v5' IN fn)=0
 OR position('cut_consumption_reconciliations_v19' IN fn)=0
 OR position('dock_lineage_audit_v37' IN fn)=0
 THEN RAISE EXCEPTION 'ERR_F38_REQUIRED_SOURCE_LEDGER_MISSING'; END IF;
 RAISE NOTICE 'PASS v38 bounded material attribution, operator integrity and no-credit invariants';
END $$;
