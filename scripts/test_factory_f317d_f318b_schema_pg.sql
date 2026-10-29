-- Author: Sunil Jain, CyberinQ LLC, 2026-10-29.
-- Read-only structural checks, safe for existing staging data. No synthetic records.
DO $$ DECLARE t text; BEGIN
 FOREACH t IN ARRAY ARRAY['jobwork_discrepancy_resolutions_v31','fabric_shrinkage_specs_v31'] LOOP
  IF NOT EXISTS(SELECT 1 FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace
    WHERE n.nspname='public' AND c.relname=t AND c.relrowsecurity AND c.relforcerowsecurity)
  THEN RAISE EXCEPTION 'ERR_F317D_F318B_RLS_%',t; END IF;
  IF has_table_privilege('agiparel_app',format('public.%I',t),'UPDATE') OR
     has_table_privilege('agiparel_app',format('public.%I',t),'DELETE')
  THEN RAISE EXCEPTION 'ERR_F317D_F318B_IMMUTABILITY_%',t; END IF;
 END LOOP;
 IF NOT has_table_privilege('agiparel_app','jobwork_discrepancy_resolutions_v31','INSERT')
 OR has_table_privilege('agiparel_app','fabric_shrinkage_specs_v31','INSERT')
 THEN RAISE EXCEPTION 'ERR_F317D_F318B_PROVISIONER_BOUNDARY'; END IF;
 IF NOT EXISTS(SELECT 1 FROM workforce_chapter_actions_v17 WHERE permission='jobwork.discrepancy.resolve'
 AND chapter_id=6 AND authority='SIGNED_WRITE')
 THEN RAISE EXCEPTION 'ERR_F317D_SIGNED_PERMISSION'; END IF;
 IF NOT EXISTS(SELECT 1 FROM pg_trigger WHERE NOT tgisinternal AND tgname='jobwork_disposition_guard_v31')
 OR NOT EXISTS(SELECT 1 FROM pg_trigger WHERE NOT tgisinternal AND tgname='fabric_shrinkage_spec_guard_v31')
 THEN RAISE EXCEPTION 'ERR_F317D_F318B_GUARDS'; END IF;
 IF (SELECT provolatile FROM pg_proc WHERE proname='jobwork_part_disputed_v29' LIMIT 1)<>'v'
 THEN RAISE EXCEPTION 'ERR_F317D_STALE_DISPUTE_SNAPSHOT'; END IF;
 RAISE NOTICE 'PASS F3.17D/F3.18B: immutable signed dispositions, scoped recovery holds, provisioner-only source-approved shrinkage specification and tenant RLS';
END $$;
