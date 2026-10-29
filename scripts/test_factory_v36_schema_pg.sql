-- Author: Sunil Jain, CyberinQ LLC, 2026-10-29.
-- V36 structural and live fail-closed scope checks (read-only).
DO $$
DECLARE n integer;
BEGIN
 SELECT count(*) INTO n FROM pg_class c JOIN pg_namespace s ON s.oid=c.relnamespace
 WHERE s.nspname='public' AND c.relname='dock_buyer_spec_bindings_v36'
 AND c.relkind='r' AND c.relrowsecurity AND c.relforcerowsecurity;
 IF n<>1 THEN RAISE EXCEPTION 'ERR_F36_BUYER_BINDING_RLS'; END IF;
 IF NOT has_table_privilege('agiparel_app','dock_buyer_spec_bindings_v36','SELECT')
 OR has_table_privilege('agiparel_app','dock_buyer_spec_bindings_v36','INSERT')
 OR has_table_privilege('agiparel_app','dock_buyer_spec_bindings_v36','UPDATE')
 OR has_table_privilege('agiparel_app','dock_buyer_spec_bindings_v36','DELETE')
 THEN RAISE EXCEPTION 'ERR_F36_BUYER_ENROLLMENT_PRIVILEGES'; END IF;
 IF NOT EXISTS (SELECT 1 FROM pg_trigger WHERE tgname='dock_buyer_binding_insert_guard_v36'
 AND tgrelid='dock_buyer_spec_bindings_v36'::regclass AND NOT tgisinternal)
 THEN RAISE EXCEPTION 'ERR_F36_BUYER_PLANT_GUARD'; END IF;
 IF position('dock_buyer_spec_bindings_v36' IN pg_get_functiondef('guard_dock_external_v35()'::regprocedure))=0
 OR position('dock_buyer_scope_current_v36' IN pg_get_functiondef('dock_external_current_v35(text,text,uuid)'::regprocedure))=0
 OR position('dock_carton_reconciled_v35' IN pg_get_functiondef('dock_external_current_v35(text,text,uuid)'::regprocedure))=0
 THEN RAISE EXCEPTION 'ERR_F36_EXTERNAL_SCOPE_MISSING'; END IF;
 IF position('dock_gateways_v34' IN pg_get_functiondef('dock_carton_reconciled_v35(text,text,uuid)'::regprocedure))=0
 OR position('garment_needle_safety_blocked_v32' IN pg_get_functiondef('dock_carton_reconciled_v35(text,text,uuid)'::regprocedure))=0
 THEN RAISE EXCEPTION 'ERR_F36_RECONCILIATION_MISSING'; END IF;
 IF dock_buyer_scope_current_v36('missing-tenant','PLANT-01',
 '11111111-1111-4111-8111-111111111111'::uuid)
 OR dock_carton_reconciled_v35('missing-tenant','PLANT-01',
 '11111111-1111-4111-8111-111111111111'::uuid)
 OR dock_external_current_v35('missing-tenant','PLANT-01',
 '11111111-1111-4111-8111-111111111111'::uuid)
 THEN RAISE EXCEPTION 'ERR_F36_PHANTOM_ACCEPTANCE'; END IF;
 IF (dock_diagnostics_v36('missing-tenant','PLANT-01',
 '11111111-1111-4111-8111-111111111111'::uuid)->>'shipment_credit')::boolean
 THEN RAISE EXCEPTION 'ERR_F36_DIAGNOSTICS_MINTED_CREDIT'; END IF;
 RAISE NOTICE 'PASS v36 contract-scoped buyer authorization, exact carton reconciliation, no shipment credit';
END $$;
