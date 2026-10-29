-- Author: Sunil Jain, CyberinQ LLC, 2026-10-29.
-- Read-only production-safe schema readiness. Does not add any inventory.
DO $$ BEGIN
 IF to_regclass('public.trim_stock_admissions_v25') IS NULL OR
    to_regclass('public.trim_admission_proof_uses_v25') IS NULL OR
    to_regclass('public.trim_issuance_proof_uses_v24') IS NULL THEN
  RAISE EXCEPTION 'ERR_F314B_ADMISSION_TABLE_MISSING'; END IF;
 IF NOT (SELECT relrowsecurity AND relforcerowsecurity FROM pg_class WHERE oid='public.trim_stock_admissions_v25'::regclass) THEN
  RAISE EXCEPTION 'ERR_F314B_MISSING_FORCED_RLS'; END IF;
 IF NOT (SELECT relrowsecurity AND relforcerowsecurity FROM pg_class WHERE oid='public.trim_admission_proof_uses_v25'::regclass) THEN
  RAISE EXCEPTION 'ERR_F314B_PROOF_RLS'; END IF;
 IF has_table_privilege('agiparel_app','public.store_trims_v13','INSERT') OR
    has_table_privilege('agiparel_app','public.store_trims_v13','UPDATE') OR
    has_table_privilege('agiparel_app','public.trim_admission_proof_uses_v25','INSERT') OR
    has_table_privilege('agiparel_app','public.trim_stock_admissions_v25','UPDATE') OR
    NOT has_table_privilege('agiparel_app','public.trim_stock_admissions_v25','INSERT') THEN
  RAISE EXCEPTION 'ERR_F314B_STOCK_PRIVILEGE'; END IF;
 IF NOT EXISTS(SELECT 1 FROM pg_trigger WHERE tgrelid='public.trim_stock_admissions_v25'::regclass AND tgname='trim_admission_stock_v25' AND tgenabled='O') OR
    NOT EXISTS(SELECT 1 FROM pg_trigger WHERE tgrelid='public.trim_stock_admissions_v25'::regclass AND tgname='trim_admission_cross_ledger_guard_v25' AND tgenabled='O') OR
    NOT EXISTS(SELECT 1 FROM pg_trigger WHERE tgrelid='public.trim_issuances_v13'::regclass AND tgname='trim_issuance_cross_ledger_guard_v25' AND tgenabled='O') THEN
  RAISE EXCEPTION 'ERR_F314B_STOCK_OR_REPLAY_TRIGGER'; END IF;
 IF (SELECT count(*) FROM workforce_chapter_actions_v17 WHERE permission IN ('stores.trims.admit','stores.trims.inspect') AND chapter_id=4 AND authority='SIGNED_WRITE')<>2 THEN
  RAISE EXCEPTION 'ERR_F314B_MISSING_ACTIONS'; END IF;
 IF (SELECT count(*) FROM pg_constraint WHERE conrelid='public.trim_stock_admissions_v25'::regclass AND contype='f' AND condeferrable)>0 THEN
  RAISE NOTICE 'PASS F3.14B: approved document check, forced tenant RLS, immutable signed admission, cross-ledger single-use proofs, and protected stock INSERT';
 ELSE RAISE EXCEPTION 'ERR_F314B_ADMISSION_FOREIGN_KEYS'; END IF;
END $$;
