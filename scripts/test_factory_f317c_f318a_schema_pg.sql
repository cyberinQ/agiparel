-- Author: Sunil Jain, CyberinQ LLC, 2026-10-29.
-- Run read-only as admin AFTER migration 0049, no synthetic plant rows.
DO $$ DECLARE t text; BEGIN
 FOREACH t IN ARRAY ARRAY['jobwork_vendor_keys_v30','jobwork_vendor_acks_v30','fabric_shrinkage_observations_v30'] LOOP
  IF NOT EXISTS(SELECT 1 FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace
    WHERE n.nspname='public' AND c.relname=t AND c.relrowsecurity AND c.relforcerowsecurity)
  THEN RAISE EXCEPTION 'ERR_F317C_F318A_RLS_%',t; END IF;
  IF has_table_privilege('agiparel_app',format('public.%I',t),'UPDATE') OR
     has_table_privilege('agiparel_app',format('public.%I',t),'DELETE')
  THEN RAISE EXCEPTION 'ERR_F317C_F318A_MUTATION_PRIV_%',t; END IF;
 END LOOP;
 IF has_table_privilege('agiparel_app','public.jobwork_vendor_keys_v30','INSERT')
 THEN RAISE EXCEPTION 'ERR_F317C_VENDOR_KEY_NOT_PROVISIONED'; END IF;
 IF NOT has_table_privilege('agiparel_app','public.jobwork_vendor_acks_v30','INSERT') OR
    NOT has_table_privilege('agiparel_app','public.fabric_shrinkage_observations_v30','INSERT')
 THEN RAISE EXCEPTION 'ERR_F317C_F318A_WRITER_PRIV'; END IF;
 IF NOT EXISTS(SELECT 1 FROM pg_trigger WHERE tgname='jobwork_vendor_ack_guard_v30' AND NOT tgisinternal)
 OR NOT EXISTS(SELECT 1 FROM pg_trigger WHERE tgname='fabric_shrinkage_guard_v30' AND NOT tgisinternal)
 THEN RAISE EXCEPTION 'ERR_F317C_F318A_GUARDS'; END IF;
 IF NOT EXISTS(SELECT 1 FROM workforce_chapter_actions_v17 WHERE permission='jobwork.vendor.ack.record' AND chapter_id=6 AND authority='SIGNED_WRITE')
 OR NOT EXISTS(SELECT 1 FROM workforce_chapter_actions_v17 WHERE permission='stores.fabric.shrinkage.record' AND chapter_id=4 AND authority='SIGNED_WRITE')
 THEN RAISE EXCEPTION 'ERR_F317C_F318A_PERMISSIONS'; END IF;
 RAISE NOTICE 'PASS F3.17C/F3.18A: tenant RLS, provisioned vendor keys, independent signature ledgers, immutable source-linked measurements';
END $$;
