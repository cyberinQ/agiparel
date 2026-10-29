-- Author: Sunil Jain, CyberinQ LLC, 2026-10-29.
-- Read-only PostgreSQL sanity checks; no mock physical events.
DO $$
DECLARE t text;
BEGIN
 FOREACH t IN ARRAY ARRAY['jobwork_vendors_v28','jobwork_outwards_v28','jobwork_inwards_v28',
   'jobwork_used_scans_v28','jobwork_used_proofs_v28'] LOOP
  IF NOT EXISTS(SELECT 1 FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace
   WHERE n.nspname='public' AND c.relname=t AND c.relrowsecurity AND c.relforcerowsecurity)
  THEN RAISE EXCEPTION 'ERR_F317_RLS_%',t; END IF;
  IF has_table_privilege('agiparel_app','public.'||t,'UPDATE') OR
     has_table_privilege('agiparel_app','public.'||t,'DELETE') THEN
   RAISE EXCEPTION 'ERR_F317_IMMUTABLE_PRIVILEGE_%',t;
  END IF;
 END LOOP;
 IF has_table_privilege('agiparel_app','public.jobwork_vendors_v28','INSERT') OR
    has_table_privilege('agiparel_app','public.jobwork_used_scans_v28','INSERT') OR
    has_table_privilege('agiparel_app','public.jobwork_used_proofs_v28','INSERT') THEN
    RAISE EXCEPTION 'ERR_F317_PROVISIONER_ONLY'; END IF;
 IF NOT has_table_privilege('agiparel_app','public.jobwork_outwards_v28','INSERT') OR
    NOT has_table_privilege('agiparel_app','public.jobwork_inwards_v28','INSERT') THEN
    RAISE EXCEPTION 'ERR_F317_SIGNED_INSERT_MISSING'; END IF;
 IF (SELECT count(*) FROM pg_trigger WHERE NOT tgisinternal AND tgname LIKE 'zzz_jobwork_%')<9
 THEN RAISE EXCEPTION 'ERR_F317_CROSS_LEDGER_GUARDS'; END IF;
 IF (SELECT count(*) FROM workforce_chapter_actions_v17 WHERE permission IN ('jobwork.outward','jobwork.inward') AND authority='SIGNED_WRITE')<>2
 THEN RAISE EXCEPTION 'ERR_F317_PERMISSIONS'; END IF;
 RAISE NOTICE 'PASS F3.17A: RLS, immutable ledgers, provisioner vendors, signed rights, cross-ledger offsite and scan protections';
END $$;
