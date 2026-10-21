-- ============================================================================
-- AGIPAREL OS: CONSOLIDATED RLS POLICIES (0002_rls_policies.sql)
-- ============================================================================

-- Helper Macro Loop for Enabling RLS and Setting Uniform Tenant Isolation
DO $$
DECLARE
    tbl text;
    tables text[] := ARRAY[
        'store_rolls',
        'store_issuances',
        'cut_lay_rolls',
        'bundle_registry',
        'hangtag_binds',
        'carton_pack_manifests',
        'ocr_records',
        'floor_issues',
        'workers',
        'attendance_register',
        'machine_roster',
        'hourly_line_metrics',
        'styles_master',
        'style_parts',
        'style_assembly_dag',
        'dynamic_work_orders'
    ];
BEGIN
    FOREACH tbl IN ARRAY tables LOOP
        EXECUTE format('ALTER TABLE %I ENABLE ROW LEVEL SECURITY;', tbl);
        EXECUTE format('ALTER TABLE %I FORCE ROW LEVEL SECURITY;', tbl);
        EXECUTE format('DROP POLICY IF EXISTS tenant_isolation_%I ON %I;', tbl, tbl);
        EXECUTE format(
            'CREATE POLICY tenant_isolation_%I ON %I FOR ALL TO PUBLIC ' ||
            'USING (tenant_id = NULLIF(current_setting(''app.current_tenant_id'', true), '''')) ' ||
            'WITH CHECK (tenant_id = NULLIF(current_setting(''app.current_tenant_id'', true), ''''));',
            tbl, tbl
        );
    END LOOP;
END $$;