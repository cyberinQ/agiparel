BEGIN;
DO $$
DECLARE
    tbl TEXT;
    tenant_tables TEXT[] := ARRAY['store_rolls','store_issuances','cut_lay_rolls','bundle_registry','hangtag_binds','carton_pack_manifests','ocr_records','floor_issues','workers','attendance_register','machine_roster','hourly_line_metrics','styles_master','style_parts','style_assembly_dag','dynamic_work_orders'];
BEGIN
    FOREACH tbl IN ARRAY tenant_tables LOOP
        EXECUTE format('ALTER TABLE %I ENABLE ROW LEVEL SECURITY;', tbl);
        EXECUTE format('ALTER TABLE %I FORCE ROW LEVEL SECURITY;', tbl);
        EXECUTE format('DROP POLICY IF EXISTS tenant_isolation_%I ON %I;', tbl, tbl);
        EXECUTE format('CREATE POLICY tenant_isolation_%I ON %I AS PERMISSIVE FOR ALL USING (tenant_id = NULLIF(CURRENT_SETTING(''app.current_tenant_id'', true), '''')) WITH CHECK (tenant_id = NULLIF(CURRENT_SETTING(''app.current_tenant_id'', true), ''''));', tbl, tbl);
    END LOOP;
END $$;
COMMIT;
