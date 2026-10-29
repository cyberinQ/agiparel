-- Author: Sunil Jain, 2026-10-28. F3.5 read-only schema assertions.
DO $$DECLARE n int; BEGIN
 SELECT count(*) INTO n FROM pg_class WHERE relname IN
 ('pack_carton_specs_v22','pack_size_weights_v22','pack_carton_events_v22','pack_manifest_events_v22')
 AND relrowsecurity AND relforcerowsecurity;
 IF n<>4 THEN RAISE EXCEPTION 'F3.5 missing RLS on % tables',4-n; END IF;
 SELECT count(*) INTO n FROM pg_trigger WHERE NOT tgisinternal AND tgname IN
 ('pack_carton_event_guard_v22','pack_manifest_event_guard_v22','pack_carton_immutable_v22','pack_manifest_immutable_v22',
  'pack_spec_insert_v22','pack_spec_update_v22','pack_size_rule_insert_v22','pack_size_rule_immutable_v22');
 IF n<>8 THEN RAISE EXCEPTION 'F3.5 trigger count %',n; END IF;
 SELECT count(*) INTO n FROM workforce_chapter_actions_v17 WHERE permission IN
 ('packing.pack','packing.seal','packing.manifest','packing.dispatch_stage');
 IF n<>4 THEN RAISE EXCEPTION 'F3.5 action registry incomplete'; END IF;
 IF has_table_privilege('agiparel_app','pack_carton_specs_v22','INSERT')
   OR has_table_privilege('agiparel_app','pack_size_weights_v22','UPDATE')
   OR has_table_privilege('agiparel_app','pack_manifest_events_v22','UPDATE')
 THEN RAISE EXCEPTION 'F3.5 unsafe app role grants'; END IF;
 RAISE NOTICE 'PASS F3.5: forced tenant RLS, privileged specifications, append-only carton/manifest evidence and permissions';
END $$;
