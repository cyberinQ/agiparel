-- Author: Sunil Jain, 2026-10-28. Read-only F3.3 migration checks.
DO $$
DECLARE name text;
BEGIN
 FOREACH name IN ARRAY ARRAY['wip_transfer_routes_v20','wip_gateway_scans_v20','wip_movements_v20'] LOOP
  IF NOT EXISTS(SELECT 1 FROM pg_class WHERE oid=('public.'||name)::regclass AND relrowsecurity AND relforcerowsecurity) THEN
   RAISE EXCEPTION 'F3.3 requires FORCE RLS on %',name;
  END IF;
  IF has_table_privilege('agiparel_app','public.'||name,'UPDATE') OR has_table_privilege('agiparel_app','public.'||name,'DELETE') THEN
   RAISE EXCEPTION 'F3.3 evidence must be append-only for %',name;
  END IF;
 END LOOP;
 IF has_table_privilege('agiparel_app','public.wip_transfer_routes_v20','INSERT') THEN
  RAISE EXCEPTION 'F3.3 app must not self-approve transfer routes';
 END IF;
 IF NOT EXISTS(SELECT 1 FROM pg_trigger WHERE tgrelid='public.wip_gateway_scans_v20'::regclass AND tgname='wip_scan_guard_v20') OR
  NOT EXISTS(SELECT 1 FROM pg_trigger WHERE tgrelid='public.wip_movements_v20'::regclass AND tgname='wip_move_guard_v20') THEN
  RAISE EXCEPTION 'F3.3 missing database custody guards'; END IF;
 IF NOT EXISTS(SELECT 1 FROM workforce_chapter_actions_v17 WHERE permission='sewing.transfer' AND chapter_id=6 AND authority='SIGNED_WRITE') THEN
  RAISE EXCEPTION 'F3.3 registered transfer permission missing'; END IF;
 RAISE NOTICE 'PASS F3.3: forced tenant RLS, approved routes, immutable scans/transfers and permission catalog';
END $$;
