-- Author: Sunil Jain, CyberinQ LLC, 2026-10-28.
-- Read-only commissioning prerequisite check. No sample data or modifications.
DO $$ DECLARE n text; BEGIN
 FOREACH n IN ARRAY ARRAY['sew_prod_gateways_v10','sew_prod_worker_stations_v10','wip_transfer_routes_v20','wip_gateway_scans_v20','workforce_identity_grants_v15'] LOOP
  IF to_regclass('public.'||n) IS NULL THEN RAISE EXCEPTION 'Missing commissioning source %',n; END IF;
  IF NOT (SELECT relrowsecurity AND relforcerowsecurity FROM pg_class WHERE oid=to_regclass('public.'||n)) THEN
    RAISE EXCEPTION 'RLS missing for %',n;
  END IF;
  IF NOT has_table_privilege('agiparel_app','public.'||n,'SELECT') THEN RAISE EXCEPTION 'Missing app SELECT on %',n; END IF;
 END LOOP;
 IF to_regclass('public.workforce_chapter_actions_v17') IS NULL OR NOT has_table_privilege('agiparel_app','public.workforce_chapter_actions_v17','SELECT') THEN
  RAISE EXCEPTION 'Missing permission catalog or app SELECT';
 END IF;
 IF NOT EXISTS (SELECT 1 FROM workforce_chapter_actions_v17 WHERE permission='sewing.read') OR
    NOT EXISTS (SELECT 1 FROM workforce_chapter_actions_v17 WHERE permission='sewing.transfer') THEN
  RAISE EXCEPTION 'Missing sewing permission catalog';
 END IF;
 RAISE NOTICE 'PASS F3.8: tenant RLS, reader/route/worker tables and permission catalog';
END $$;
