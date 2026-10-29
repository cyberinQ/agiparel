-- Author: Sunil Jain, CyberinQ LLC, 2026-10-29.
-- Read-only RLS, privileges, immutability, signature-source and proof-guard checks.
DO $$
DECLARE n integer; t text;
BEGIN
 SELECT count(*) INTO n FROM pg_class c JOIN pg_namespace ns ON ns.oid=c.relnamespace
 WHERE ns.nspname='public' AND c.relname IN (
  'dock_gateways_v34','dock_carrier_authorizations_v34','dock_station_policies_v34',
  'dock_carton_scans_v34','dock_container_seals_v34','dock_carrier_receipts_v34','dock_asn_drafts_v34')
 AND c.relkind='r' AND c.relrowsecurity AND c.relforcerowsecurity;
 IF n<>7 THEN RAISE EXCEPTION 'ERR_F34_RLS_REQUIRED'; END IF;
 FOREACH t IN ARRAY ARRAY['dock_gateways_v34','dock_carrier_authorizations_v34','dock_station_policies_v34'] LOOP
  IF has_table_privilege('agiparel_app',t,'INSERT') OR has_table_privilege('agiparel_app',t,'UPDATE')
   OR has_table_privilege('agiparel_app',t,'DELETE') OR NOT has_table_privilege('agiparel_app',t,'SELECT')
  THEN RAISE EXCEPTION 'ERR_F34_PRIVILEGED_ENROLLMENT: %',t; END IF;
 END LOOP;
 FOREACH t IN ARRAY ARRAY['dock_carton_scans_v34','dock_container_seals_v34',
 'dock_carrier_receipts_v34','dock_asn_drafts_v34'] LOOP
  IF NOT has_table_privilege('agiparel_app',t,'INSERT') OR has_table_privilege('agiparel_app',t,'UPDATE')
   OR has_table_privilege('agiparel_app',t,'DELETE')
   OR NOT EXISTS(SELECT 1 FROM pg_trigger WHERE tgrelid=t::regclass AND tgname='dock_immutable_v34' AND NOT tgisinternal)
  THEN RAISE EXCEPTION 'ERR_F34_IMMUTABLE_LEDGER: %',t; END IF;
 END LOOP;
 IF NOT EXISTS(SELECT 1 FROM pg_trigger WHERE tgrelid='dock_carton_scans_v34'::regclass AND tgname='dock_scan_guard_v34')
 OR NOT EXISTS(SELECT 1 FROM pg_trigger WHERE tgrelid='dock_container_seals_v34'::regclass AND tgname='dock_seal_guard_v34')
 OR NOT EXISTS(SELECT 1 FROM pg_trigger WHERE tgrelid='dock_carrier_receipts_v34'::regclass AND tgname='dock_carrier_guard_v34')
 OR NOT EXISTS(SELECT 1 FROM pg_trigger WHERE tgrelid='dock_asn_drafts_v34'::regclass AND tgname='dock_asn_guard_v34')
 THEN RAISE EXCEPTION 'ERR_F34_GUARDS_MISSING'; END IF;
 IF (SELECT count(*) FROM workforce_chapter_actions_v17 WHERE chapter_id=8
  AND permission IN ('packing.dock.seal','packing.dock.carrier','packing.dock.asn.prepare')
  AND authority='SIGNED_WRITE')<>3 THEN RAISE EXCEPTION 'ERR_F34_ACTION_CATALOG'; END IF;
 IF position('dock_carrier_receipts_v34' in pg_get_functiondef('guard_factory_proof_v32()'::regprocedure))=0
 OR position('dock_asn_drafts_v34' in pg_get_functiondef('guard_factory_review_proof_v33()'::regprocedure))=0
 THEN RAISE EXCEPTION 'ERR_F34_CROSS_VERSION_REPLAY'; END IF;
 RAISE NOTICE 'PASS F3.20B-D: tenant RLS, provisioner-only authority, immutable dock evidence, signed-action guards';
END $$;
