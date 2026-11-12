-- Author: Sunil Jain, 2026-11-11 — Non-mutating SOP-2 schema and isolation assertions.
DO $$
DECLARE t text; count_rows bigint;
BEGIN
    FOREACH t IN ARRAY ARRAY['cut_sop2_target_versions_v46','cut_sop2_target_skus_v46',
        'cut_sop2_change_requests_v46','cut_sop2_change_approvals_v46',
        'cut_sop2_inspection_policies_v46','cut_sop2_roll_inspections_v46',
        'cut_sop2_schedule_versions_v46','cut_sop2_nfc_bindings_v46',
        'cut_sop2_marriage_edges_v46','cut_sop2_material_reconciliations_v46'] LOOP
        IF NOT EXISTS(SELECT 1 FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace
            WHERE n.nspname='public' AND c.relname=t AND c.relkind='r' AND c.relrowsecurity AND c.relforcerowsecurity) THEN
            RAISE EXCEPTION 'Missing tenant-enforced SOP-2 relation: %',t;
        END IF;
        IF NOT EXISTS(SELECT 1 FROM pg_policies WHERE schemaname='public' AND tablename=t AND policyname='sop2_tenant') THEN
            RAISE EXCEPTION 'Missing isolation policy: %',t;
        END IF;
    END LOOP;
    IF NOT EXISTS(SELECT 1 FROM pg_proc WHERE proname='uuid_generate_v4') THEN
        RAISE EXCEPTION 'UUID extension not available';
    END IF;
    SELECT count(*) INTO count_rows FROM cut_sewing_releases_v8;
    RAISE NOTICE 'SOP-2 schema and RLS PASS; existing signed sewing releases preserved: %',count_rows;
END $$;
