-- Author: Sunil Jain, 2026-10-28.
-- Read-only PostgreSQL 16 readiness gate for F3.6: existing tables only.
-- Requires migrations 0037–0041; no new DDL is introduced by F3.6.
DO $$
DECLARE t text; row_security boolean; force_security boolean;
BEGIN
  FOREACH t IN ARRAY ARRAY[
    'cut_part_outcomes_v19',
    'cut_consumption_reconciliations_v19',
    'qc_garment_events_v21',
    'wip_movements_v20',
    'wip_transfer_routes_v20',
    'pack_carton_events_v22',
    'pack_manifest_events_v22'
  ] LOOP
    SELECT c.relrowsecurity, c.relforcerowsecurity
      INTO row_security,force_security
    FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace
    WHERE n.nspname='public' AND c.relname=t AND c.relkind IN ('r','p');
    IF NOT FOUND OR NOT row_security OR NOT force_security THEN
      RAISE EXCEPTION 'F3.6 requires FORCE RLS for %',t;
    END IF;
    IF NOT has_table_privilege('agiparel_app', format('public.%I',t), 'SELECT') THEN
      RAISE EXCEPTION 'F3.6 read privilege missing for %',t;
    END IF;
  END LOOP;
  RAISE NOTICE 'PASS F3.6: all source tables available, forced tenant RLS, application SELECT permissions';
END $$;
