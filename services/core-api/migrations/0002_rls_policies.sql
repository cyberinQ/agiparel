<<<<<<< HEAD
-- Author: Sunil Jain, 2026-10-25 — Tenant-scoped RLS policies.
=======
>>>>>>> 62b87176970aae981bc0876d77b7cab60eca07b1
BEGIN;
DO $$
DECLARE
    tbl TEXT;
<<<<<<< HEAD
    tenant_tables TEXT[] := ARRAY['store_rolls','store_issuances','cut_lay_rolls','bundle_registry','hangtag_binds','carton_pack_manifests','ocr_records','floor_issues','workers','attendance_register','machine_roster','hourly_line_metrics','styles_master','style_parts','style_assembly_dag','dynamic_work_orders','documents','order_sku_matrix','bom_trim_requirements','post_sewing_processes','plant_hardware_assets'];
=======
    tenant_tables TEXT[] := ARRAY['store_rolls','store_issuances','cut_lay_rolls','bundle_registry','hangtag_binds','carton_pack_manifests','ocr_records','floor_issues','workers','attendance_register','machine_roster','hourly_line_metrics','styles_master','style_parts','style_assembly_dag','dynamic_work_orders'];
>>>>>>> 62b87176970aae981bc0876d77b7cab60eca07b1
BEGIN
    FOREACH tbl IN ARRAY tenant_tables LOOP
        EXECUTE format('ALTER TABLE %I ENABLE ROW LEVEL SECURITY;', tbl);
        EXECUTE format('ALTER TABLE %I FORCE ROW LEVEL SECURITY;', tbl);
        EXECUTE format('DROP POLICY IF EXISTS tenant_isolation_%I ON %I;', tbl, tbl);
        EXECUTE format('CREATE POLICY tenant_isolation_%I ON %I AS PERMISSIVE FOR ALL USING (tenant_id = NULLIF(CURRENT_SETTING(''app.current_tenant_id'', true), '''')) WITH CHECK (tenant_id = NULLIF(CURRENT_SETTING(''app.current_tenant_id'', true), ''''));', tbl, tbl);
    END LOOP;
END $$;
COMMIT;
