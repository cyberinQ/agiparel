-- Author: Sunil Jain, CyberinQ LLC, 2026-10-29.
-- Read-only schema and tenant-RLS prerequisite check (no real scan required).
DO $$
DECLARE t text;
BEGIN
 FOREACH t IN ARRAY ARRAY[
 'sew_prod_gateways_v10','gen_carriers_v18','gen_carrier_active_v18',
 'cut_part_outcomes_v19','wip_gateway_scans_v20','wip_movements_v20',
 'wip_exception_cases_v23','wip_exception_closures_v23'] LOOP
  IF NOT EXISTS(SELECT 1 FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace
      WHERE n.nspname='public' AND c.relname=t AND c.relrowsecurity AND c.relforcerowsecurity)
  THEN RAISE EXCEPTION 'F3.12 prerequisite missing forced tenant RLS for %',t; END IF;
  IF NOT has_table_privilege('agiparel_app','public.'||quote_ident(t),'SELECT')
  THEN RAISE EXCEPTION 'F3.12 application missing SELECT for %',t; END IF;
 END LOOP;
 IF NOT EXISTS (SELECT 1 FROM pg_trigger WHERE tgname='wip_scan_guard_v20' AND NOT tgisinternal)
 THEN RAISE EXCEPTION 'F3.12 signed scan trigger missing'; END IF;
 IF NOT EXISTS (SELECT 1 FROM pg_trigger WHERE tgname='zz_wip_exception_hold_v23' AND NOT tgisinternal)
 THEN RAISE EXCEPTION 'F3.12 incident WIP hold trigger missing'; END IF;
 RAISE NOTICE 'PASS F3.12: tenant RLS, signed gateway scan source and F3.10 exception hold are installed';
END $$;
