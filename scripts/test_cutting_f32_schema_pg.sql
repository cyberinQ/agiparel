-- Author: Sunil Jain, 2026-10-28. F3.2 read-only PostgreSQL schema acceptance checks.
DO $$
DECLARE name text; role_can_write bool;
BEGIN
 FOREACH name IN ARRAY ARRAY['cut_consumption_reconciliations_v19','cut_part_outcomes_v19','cut_recut_links_v19','cut_proof_consumptions_v19','cut_defect_codes_v19'] LOOP
  IF NOT EXISTS (SELECT 1 FROM pg_class c JOIN pg_namespace n ON c.relnamespace=n.oid
  WHERE n.nspname='public' AND c.relname=name AND c.relrowsecurity AND c.relforcerowsecurity) THEN
   RAISE EXCEPTION 'F3.2 failed RLS check for %',name;
  END IF;
  IF has_table_privilege('agiparel_app','public.'||name,'UPDATE') OR has_table_privilege('agiparel_app','public.'||name,'DELETE') THEN
   RAISE EXCEPTION 'F3.2 application has mutable evidence privilege on %',name;
  END IF;
 END LOOP;
 IF has_table_privilege('agiparel_app','public.cut_defect_codes_v19','INSERT') THEN
  RAISE EXCEPTION 'F3.2 app can self-provision defects'; END IF;
 IF NOT EXISTS (SELECT 1 FROM pg_trigger WHERE tgrelid='public.cut_consumption_reconciliations_v19'::regclass AND tgname='cut_consumption_guard_v19') OR
    NOT EXISTS (SELECT 1 FROM pg_trigger WHERE tgrelid='public.cut_part_outcomes_v19'::regclass AND tgname='cut_part_outcome_guard_v19') OR
    NOT EXISTS (SELECT 1 FROM pg_trigger WHERE tgrelid='public.cut_recut_links_v19'::regclass AND tgname='cut_recut_guard_v19') THEN
   RAISE EXCEPTION 'F3.2 lineage guards missing'; END IF;
 RAISE NOTICE 'PASS F3.2 schema: tenant RLS, append-only data, provisioner-owned defects, lineage guards';
END $$;
