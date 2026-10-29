-- Author: Sunil Jain, CyberinQ LLC, 2026-10-29.
-- V35 schema authorization, safety invariants, signer custody, immutable records.
DO $$
DECLARE n integer;
BEGIN
 SELECT count(*) INTO n FROM pg_class c JOIN pg_namespace ns ON ns.oid=c.relnamespace
 WHERE ns.nspname='public' AND c.relname IN ('dock_external_signers_v35','dock_external_assertions_v35')
 AND c.relkind='r' AND c.relrowsecurity AND c.relforcerowsecurity;
 IF n<>2 THEN RAISE EXCEPTION 'ERR_F35_RLS_REQUIRED'; END IF;
 IF NOT has_table_privilege('agiparel_app','dock_external_signers_v35','SELECT')
 OR has_table_privilege('agiparel_app','dock_external_signers_v35','INSERT')
 OR has_table_privilege('agiparel_app','dock_external_signers_v35','UPDATE')
 OR has_table_privilege('agiparel_app','dock_external_signers_v35','DELETE')
 THEN RAISE EXCEPTION 'ERR_F35_EXTERNAL_TRUST_MUST_BE_PROVISIONER_ONLY'; END IF;
 IF NOT has_table_privilege('agiparel_app','dock_external_assertions_v35','SELECT')
 OR NOT has_table_privilege('agiparel_app','dock_external_assertions_v35','INSERT')
 OR has_table_privilege('agiparel_app','dock_external_assertions_v35','UPDATE')
 OR has_table_privilege('agiparel_app','dock_external_assertions_v35','DELETE')
 THEN RAISE EXCEPTION 'ERR_F35_ASSERTIONS_MUST_BE_APPEND_ONLY'; END IF;
 IF NOT EXISTS(SELECT 1 FROM pg_trigger WHERE tgrelid='dock_external_assertions_v35'::regclass
 AND tgname='dock_external_guard_v35' AND NOT tgisinternal)
 OR NOT EXISTS(SELECT 1 FROM pg_trigger WHERE tgrelid='dock_external_assertions_v35'::regclass
 AND tgname='dock_external_immutable_v35' AND NOT tgisinternal)
 THEN RAISE EXCEPTION 'ERR_F35_MISSING_VALIDATION_OR_IMMUTABILITY'; END IF;
 IF position('dock_evidence_current_v34' IN pg_get_functiondef('dock_external_current_v35(text,text,uuid)'::regprocedure))=0
 OR position('dock_carton_reconciled_v35' IN pg_get_functiondef('dock_external_current_v35(text,text,uuid)'::regprocedure))=0
 THEN RAISE EXCEPTION 'ERR_F35_CURRENT_CHAIN_RECHECK'; END IF;
 IF position('dock_evidence_current_v34' IN pg_get_functiondef('guard_dock_external_v35()'::regprocedure))=0
 OR position('asn_reference' IN pg_get_functiondef('guard_dock_external_v35()'::regprocedure))=0
 THEN RAISE EXCEPTION 'ERR_F35_EXTERNAL_SQL_ADMISSION'; END IF;
 RAISE NOTICE 'PASS v35 external trust, RLS, immutable acknowledgements, ASN correlation, live reconciliation';
END $$;
