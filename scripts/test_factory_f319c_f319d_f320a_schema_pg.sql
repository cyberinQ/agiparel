-- Author: Sunil Jain, CyberinQ LLC, 2026-10-29.
-- Read-only schema/security acceptance, executable against non-disposable database.
DO $$
DECLARE n integer;
BEGIN
 SELECT count(*) INTO n FROM pg_class c JOIN pg_namespace ns ON ns.oid=c.relnamespace
  WHERE ns.nspname='public' AND c.relname IN
 ('needle_safety_clearances_v33','buyer_aql_review_rules_v33','buyer_aql_reviews_v33')
 AND c.relkind='r' AND c.relrowsecurity AND c.relforcerowsecurity;
 IF n<>3 THEN RAISE EXCEPTION 'ERR_F33_RLS_TABLES'; END IF;
 IF has_table_privilege('agiparel_app','buyer_aql_review_rules_v33','INSERT')
 OR has_table_privilege('agiparel_app','buyer_aql_review_rules_v33','UPDATE')
 OR has_table_privilege('agiparel_app','buyer_aql_review_rules_v33','DELETE')
 OR has_table_privilege('agiparel_app','needle_safety_clearances_v33','UPDATE')
 OR has_table_privilege('agiparel_app','buyer_aql_reviews_v33','DELETE')
 OR NOT has_table_privilege('agiparel_app','needle_safety_clearances_v33','INSERT')
 OR NOT has_table_privilege('agiparel_app','buyer_aql_reviews_v33','INSERT')
 THEN RAISE EXCEPTION 'ERR_F33_PROVISIONER_OR_IMMUTABILITY_PRIVILEGE'; END IF;
 IF NOT EXISTS(SELECT 1 FROM pg_trigger WHERE tgrelid='needle_safety_clearances_v33'::regclass AND tgname='needle_safety_clearance_guard_v33' AND NOT tgisinternal)
 OR NOT EXISTS(SELECT 1 FROM pg_trigger WHERE tgrelid='buyer_aql_reviews_v33'::regclass AND tgname='buyer_aql_review_guard_v33' AND NOT tgisinternal)
 OR NOT EXISTS(SELECT 1 FROM pg_trigger WHERE tgrelid='needle_safety_clearances_v33'::regclass AND tgname='safety_review_proof_v33' AND NOT tgisinternal)
 OR NOT EXISTS(SELECT 1 FROM pg_trigger WHERE tgrelid='buyer_aql_reviews_v33'::regclass AND tgname='aql_review_proof_v33' AND NOT tgisinternal)
 THEN RAISE EXCEPTION 'ERR_F33_GUARDS_MISSING'; END IF;
 IF NOT EXISTS(SELECT 1 FROM workforce_chapter_actions_v17 WHERE permission='quality.needle.clear' AND authority='SIGNED_WRITE')
 OR NOT EXISTS(SELECT 1 FROM workforce_chapter_actions_v17 WHERE permission='quality.aql.review' AND authority='SIGNED_WRITE')
 THEN RAISE EXCEPTION 'ERR_F33_ACTIONS_MISSING'; END IF;
 IF position('needle_safety_clearances_v33' in pg_get_functiondef('garment_needle_safety_blocked_v32(text,uuid)'::regprocedure))=0
 THEN RAISE EXCEPTION 'ERR_F33_UNRESOLVED_SAFETY_GATE_NOT_UPDATED'; END IF;
 RAISE NOTICE 'PASS F3.19C/D+F3.20A: forced RLS, immutable signed reviews, independent followups, provisioner-only buyer thresholds and safety gate';
END $$;
