-- Author: Sunil Jain, CyberinQ LLC, 2026-10-29.
-- Read-only F3.14 rollout readiness; does not manufacture stock, grants or proofs.
DO $$
DECLARE tab text; t_count integer;
BEGIN
 FOREACH tab IN ARRAY ARRAY['store_trims_v13','trim_issuances_v13','trim_issuance_proof_uses_v24'] LOOP
  IF NOT EXISTS(SELECT 1 FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace
     WHERE n.nspname='public' AND c.relname=tab AND c.relkind='r' AND c.relrowsecurity AND c.relforcerowsecurity) THEN
   RAISE EXCEPTION 'ERR_F314_RLS_NOT_FORCED: %',tab;
  END IF;
 END LOOP;
 IF NOT EXISTS(SELECT 1 FROM pg_trigger WHERE tgrelid='public.trim_issuances_v13'::regclass
 AND tgname='trim_issuance_conservation_v24' AND tgenabled='O') THEN
   RAISE EXCEPTION 'ERR_F314_GUARD_MISSING';
 END IF;
 IF NOT EXISTS(SELECT 1 FROM pg_trigger WHERE tgrelid='public.trim_issuances_v13'::regclass
 AND tgname='trim_issuance_immutable_v24' AND tgenabled='O') THEN
   RAISE EXCEPTION 'ERR_F314_IMMUTABILITY_MISSING';
 END IF;
 IF has_table_privilege('agiparel_app','public.store_trims_v13','UPDATE') OR
    has_table_privilege('agiparel_app','public.store_trims_v13','INSERT') OR
    has_table_privilege('agiparel_app','public.trim_issuances_v13','UPDATE') OR
    has_table_privilege('agiparel_app','public.trim_issuance_proof_uses_v24','INSERT') THEN
    RAISE EXCEPTION 'ERR_F314_UNSAFE_APP_PRIVILEGES';
 END IF;
 IF NOT has_table_privilege('agiparel_app','public.trim_issuances_v13','INSERT') THEN
    RAISE EXCEPTION 'ERR_F314_MISSING_APP_INSERT';
 END IF;
 SELECT COUNT(*) INTO t_count FROM workforce_chapter_actions_v17
 WHERE (permission='stores.trims.issue' OR permission='stores.trims.receive')
 AND chapter_id=4 AND authority='SIGNED_WRITE';
 IF t_count<>2 THEN RAISE EXCEPTION 'ERR_F314_POLICY_CATALOG'; END IF;
 IF EXISTS(SELECT 1 FROM store_trims_v13 WHERE remaining_qty>received_qty OR remaining_qty<0) THEN
  RAISE EXCEPTION 'ERR_F314_PREEXISTING_STOCK_INCONSISTENCY';
 END IF;
 RAISE NOTICE 'PASS F3.14: atomic trigger, forced RLS, restricted stock privilege, two independently provisioned actions';
END $$;
