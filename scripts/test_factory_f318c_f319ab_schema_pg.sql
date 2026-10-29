-- Author: Sunil Jain, CyberinQ LLC, 2026-10-29.
-- Read-only schema/privilege acceptance after migration 0051; no test data.
DO $$ DECLARE t text; BEGIN
 FOREACH t IN ARRAY ARRAY[
  'factory_instrument_calibrations_v32','fabric_calibration_links_v32',
  'buyer_aql_plans_v32','buyer_aql_sample_events_v32','needle_safety_events_v32'] LOOP
  IF to_regclass('public.'||t) IS NULL THEN RAISE EXCEPTION 'Missing relation: %',t; END IF;
  IF NOT (SELECT relrowsecurity AND relforcerowsecurity FROM pg_class WHERE oid=to_regclass('public.'||t))
  THEN RAISE EXCEPTION 'Missing forced RLS: %',t; END IF;
  IF NOT has_table_privilege('agiparel_app','public.'||t,'SELECT')
  THEN RAISE EXCEPTION 'Missing scoped read: %',t; END IF;
  IF NOT EXISTS(SELECT 1 FROM pg_trigger WHERE tgrelid=to_regclass('public.'||t)
   AND tgname='immutable_factory_v32' AND NOT tgisinternal)
  THEN RAISE EXCEPTION 'Missing immutable guard: %',t; END IF;
 END LOOP;
 IF has_table_privilege('agiparel_app','factory_instrument_calibrations_v32','INSERT')
 OR has_table_privilege('agiparel_app','buyer_aql_plans_v32','INSERT')
 THEN RAISE EXCEPTION 'Application can self-approve plan or calibration'; END IF;
 IF NOT has_table_privilege('agiparel_app','fabric_calibration_links_v32','INSERT')
 OR NOT has_table_privilege('agiparel_app','buyer_aql_sample_events_v32','INSERT')
 OR NOT has_table_privilege('agiparel_app','needle_safety_events_v32','INSERT')
 THEN RAISE EXCEPTION 'Missing signed evidence insert rights'; END IF;
 IF NOT EXISTS(SELECT 1 FROM pg_trigger WHERE tgname='needle_safety_qc_gate_v32' AND NOT tgisinternal)
 OR NOT EXISTS(SELECT 1 FROM pg_trigger WHERE tgname='needle_safety_packing_gate_v32' AND NOT tgisinternal)
 THEN RAISE EXCEPTION 'Missing garment safety blockers'; END IF;
 RAISE NOTICE 'PASS F3.18C/F3.19A/B: instrument source, provisioner-only plans, signed immutable evidence, forced RLS, and QC/packing safety gates';
END $$;
