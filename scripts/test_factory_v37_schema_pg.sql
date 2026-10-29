-- Author: Sunil Jain, CyberinQ LLC, 2026-10-29.
-- Read-only v37 schema security and fail-closed negative tests.
DO $$
DECLARE report jsonb; source text;
BEGIN
 IF to_regprocedure('public.dock_lineage_audit_v37(text,text,uuid)') IS NULL THEN
  RAISE EXCEPTION 'ERR_F37_AUDIT_FUNCTION_MISSING'; END IF;
 IF EXISTS (SELECT 1 FROM pg_proc pr CROSS JOIN LATERAL
  aclexplode(coalesce(pr.proacl,acldefault('f',pr.proowner))) acl
  WHERE pr.oid='dock_lineage_audit_v37(text,text,uuid)'::regprocedure
   AND acl.grantee=0 AND acl.privilege_type='EXECUTE') THEN
  RAISE EXCEPTION 'ERR_F37_PUBLIC_EXECUTE'; END IF;
 IF NOT has_function_privilege('agiparel_app','dock_lineage_audit_v37(text,text,uuid)','EXECUTE') THEN
  RAISE EXCEPTION 'ERR_F37_APP_CANNOT_AUDIT'; END IF;
 SELECT pg_get_functiondef('dock_lineage_audit_v37(text,text,uuid)'::regprocedure) INTO source;
 IF position('SECURITY DEFINER' IN source)>0
  OR position('jobwork_part_disputed_v29' IN source)=0
  OR position('cut_consumption_reconciliations_v19' IN source)=0
  OR position('qc_garment_parts_ready_v21' IN source)=0
  OR position('wip_piece_active_hold_v26' IN source)=0
 THEN RAISE EXCEPTION 'ERR_F37_AUDIT_BYPASS_OR_MISSING_LEDGERS'; END IF;
 SELECT dock_lineage_audit_v37('missing-tenant','PLANT-01',
  '11111111-1111-4111-8111-111111111111'::uuid) INTO report;
 IF (report->>'lineage_evidence_complete')::boolean
  OR (report->>'shipment_credit')::boolean
  OR (report->>'global_inventory_conserved')::boolean
  OR (report->>'financial_ledger_conserved')::boolean
  OR (report->>'asn_network_delivery_proven')::boolean
 THEN RAISE EXCEPTION 'ERR_F37_MINTED_UNVERIFIED_AUTHORITY'; END IF;
 IF jsonb_array_length(report->'blockers')=0 THEN RAISE EXCEPTION 'ERR_F37_MISSING_MANIFEST_UNBLOCKED'; END IF;
 RAISE NOTICE 'PASS v37 bounded cut/WIP audit, explicit incomplete scopes, no shipment authority';
END $$;
