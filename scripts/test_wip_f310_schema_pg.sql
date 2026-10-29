-- Author: Sunil Jain, CyberinQ LLC, 2026-10-29.
-- Read-only schema security verification; requires PostgreSQL 16 and 0042.
DO $$ DECLARE t text; BEGIN
 FOREACH t IN ARRAY ARRAY['wip_exception_cases_v23','wip_exception_closures_v23'] LOOP
  IF NOT EXISTS(SELECT 1 FROM pg_class WHERE oid=('public.'||t)::regclass AND relrowsecurity AND relforcerowsecurity) THEN
   RAISE EXCEPTION 'F3.10 mandatory RLS missing: %',t; END IF;
  IF NOT EXISTS(SELECT 1 FROM pg_policies WHERE schemaname='public' AND tablename=t AND policyname='wip_exception_tenant_v23') THEN
   RAISE EXCEPTION 'F3.10 tenant policy missing: %',t;END IF;
  IF NOT has_table_privilege('agiparel_app','public.'||t,'SELECT') OR NOT has_table_privilege('agiparel_app','public.'||t,'INSERT')
   OR has_table_privilege('agiparel_app','public.'||t,'UPDATE') OR has_table_privilege('agiparel_app','public.'||t,'DELETE') THEN
   RAISE EXCEPTION 'F3.10 unexpected table grants: %',t; END IF;
 END LOOP;
 IF NOT EXISTS(SELECT 1 FROM pg_trigger WHERE tgname='zz_wip_exception_hold_v23' AND tgenabled<>'D')
 OR NOT EXISTS(SELECT 1 FROM pg_trigger WHERE tgname='zz_qc_exception_hold_v23' AND tgenabled<>'D') THEN
  RAISE EXCEPTION 'F3.10 custody/QC hold missing'; END IF;
 IF (SELECT count(*) FROM workforce_chapter_actions_v17 WHERE permission IN('sewing.exception.report','sewing.exception.resolve') AND chapter_id=6 AND authority='SIGNED_WRITE')<>2 THEN
  RAISE EXCEPTION 'F3.10 permission catalog missing'; END IF;
 RAISE NOTICE 'PASS F3.10: forced RLS, immutable exceptions, WIP/QC hold triggers, signed role catalog';
END $$;
