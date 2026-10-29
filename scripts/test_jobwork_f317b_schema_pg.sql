-- Author: Sunil Jain, CyberinQ LLC, 2026-10-29.
-- Safe read-only PostgreSQL readiness check; no invented vendor or production evidence.
DO $$
DECLARE n integer;
BEGIN
 IF NOT EXISTS(SELECT 1 FROM pg_class c JOIN pg_namespace s ON s.oid=c.relnamespace
  WHERE s.nspname='public' AND c.relname='jobwork_discrepancies_v29'
   AND c.relrowsecurity AND c.relforcerowsecurity) THEN
  RAISE EXCEPTION 'ERR_F317B_RLS'; END IF;
 IF has_table_privilege('agiparel_app','public.jobwork_discrepancies_v29','UPDATE')
  OR has_table_privilege('agiparel_app','public.jobwork_discrepancies_v29','DELETE') THEN
  RAISE EXCEPTION 'ERR_F317B_APPEND_ONLY_PRIVILEGE'; END IF;
 IF NOT has_table_privilege('agiparel_app','public.jobwork_discrepancies_v29','INSERT')
  OR NOT has_table_privilege('agiparel_app','public.jobwork_discrepancies_v29','SELECT') THEN
  RAISE EXCEPTION 'ERR_F317B_SIGNED_ACCESS_MISSING'; END IF;
 SELECT count(*) INTO n FROM pg_trigger WHERE NOT tgisinternal
  AND tgname LIKE 'zzz_jobwork_disputed_%_v29';
 IF n<>6 THEN RAISE EXCEPTION 'ERR_F317B_PROGRESS_GUARDS_%',n; END IF;
 IF NOT EXISTS(SELECT 1 FROM pg_trigger WHERE NOT tgisinternal
  AND tgname='jobwork_discrepancy_guard_v29') THEN RAISE EXCEPTION 'ERR_F317B_REPORT_GUARD'; END IF;
 IF NOT EXISTS(SELECT 1 FROM workforce_chapter_actions_v17
  WHERE permission='jobwork.discrepancy.report' AND chapter_id=6 AND authority='SIGNED_WRITE') THEN
  RAISE EXCEPTION 'ERR_F317B_SIGNED_PERMISSION'; END IF;
 IF NOT EXISTS(SELECT 1 FROM pg_constraint WHERE conrelid='jobwork_discrepancies_v29'::regclass
  AND contype='u') THEN RAISE EXCEPTION 'ERR_F317B_DEDUPLICATION'; END IF;
 RAISE NOTICE 'PASS F3.17B: forced RLS, signed append-only discrepancy, six downstream holds and scoped reconciliation';
END $$;
