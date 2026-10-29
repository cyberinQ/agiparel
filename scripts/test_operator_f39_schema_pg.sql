-- Author: Sunil Jain, CyberinQ LLC, 2026-10-28.
-- F3.9: read-only production source readiness; no simulated production rows.
DO $$ DECLARE tbl text; BEGIN
 FOREACH tbl IN ARRAY ARRAY[
  'workforce_identity_grants_v15','sew_prod_worker_stations_v10',
  'wip_gateway_scans_v20','wip_movements_v20','wip_transfer_routes_v20',
  'gen_cut_parts_v18','gen_garments_v18','gen_carrier_active_v18',
  'cut_part_outcomes_v19','cut_recut_links_v19'
 ] LOOP
  IF to_regclass('public.'||tbl) IS NULL THEN RAISE EXCEPTION 'F3.9 missing %',tbl; END IF;
  IF NOT (SELECT relrowsecurity AND relforcerowsecurity FROM pg_class WHERE oid=to_regclass('public.'||tbl)) THEN
   RAISE EXCEPTION 'F3.9 missing forced tenant RLS: %',tbl;
  END IF;
  IF NOT has_table_privilege('agiparel_app','public.'||tbl,'SELECT') THEN
   RAISE EXCEPTION 'F3.9 missing app SELECT: %',tbl;
  END IF;
 END LOOP;
 IF to_regclass('public.workforce_chapter_actions_v17') IS NULL OR
  NOT EXISTS(SELECT 1 FROM workforce_chapter_actions_v17 WHERE permission='sewing.read') OR
  NOT EXISTS(SELECT 1 FROM workforce_chapter_actions_v17 WHERE permission='sewing.transfer') THEN
   RAISE EXCEPTION 'F3.9 missing grant catalog';
 END IF;
 RAISE NOTICE 'PASS F3.9: signed scan, movement, carrier, part and grant sources; forced tenant RLS';
END $$;
