-- Author: Sunil Jain, 2026-10-28. Read-only migration qualification.
DO $$ DECLARE name text; BEGIN
 FOREACH name IN ARRAY ARRAY['qc_garment_specs_v21','qc_garment_measure_rules_v21','qc_garment_defects_v21','qc_garment_events_v21'] LOOP
  IF NOT EXISTS(SELECT 1 FROM pg_class WHERE oid=('public.'||name)::regclass AND relrowsecurity AND relforcerowsecurity) THEN
   RAISE EXCEPTION 'F3.4 missing forced RLS: %',name; END IF;
  IF has_table_privilege('agiparel_app','public.'||name,'DELETE') OR has_table_privilege('agiparel_app','public.'||name,'UPDATE') THEN
   RAISE EXCEPTION 'F3.4 data must be immutable: %',name; END IF;
 END LOOP;
 IF has_table_privilege('agiparel_app','public.qc_garment_specs_v21','INSERT') OR
    has_table_privilege('agiparel_app','public.qc_garment_measure_rules_v21','INSERT') OR
    has_table_privilege('agiparel_app','public.qc_garment_defects_v21','INSERT') THEN
  RAISE EXCEPTION 'F3.4 app cannot self-approve quality specifications'; END IF;
 IF NOT EXISTS (SELECT 1 FROM pg_trigger WHERE tgrelid='public.qc_garment_events_v21'::regclass AND tgname='qc_garment_guard_v21') THEN
  RAISE EXCEPTION 'F3.4 missing state-machine trigger'; END IF;
 IF NOT EXISTS (SELECT 1 FROM workforce_chapter_actions_v17 WHERE permission='quality.release' AND chapter_id=7 AND authority='SIGNED_WRITE') THEN
  RAISE EXCEPTION 'F3.4 missing quality-release action'; END IF;
 RAISE NOTICE 'PASS F3.4: tenant isolation, immutable evidence, source-backed specs, state guards';
END $$;
