-- Enable and FORCE RLS on all tenant-segregated transactional tables
ALTER TABLE store_rolls ENABLE ROW LEVEL SECURITY;
ALTER TABLE store_rolls FORCE ROW LEVEL SECURITY;

ALTER TABLE store_issuances ENABLE ROW LEVEL SECURITY;
ALTER TABLE store_issuances FORCE ROW LEVEL SECURITY;

ALTER TABLE cut_lay_rolls ENABLE ROW LEVEL SECURITY;
ALTER TABLE cut_lay_rolls FORCE ROW LEVEL SECURITY;

ALTER TABLE bundle_registry ENABLE ROW LEVEL SECURITY;
ALTER TABLE bundle_registry FORCE ROW LEVEL SECURITY;

ALTER TABLE carton_pack_manifests ENABLE ROW LEVEL SECURITY;
ALTER TABLE carton_pack_manifests FORCE ROW LEVEL SECURITY;

ALTER TABLE hangtag_binds ENABLE ROW LEVEL SECURITY;
ALTER TABLE hangtag_binds FORCE ROW LEVEL SECURITY;

ALTER TABLE floor_issues ENABLE ROW LEVEL SECURITY;
ALTER TABLE floor_issues FORCE ROW LEVEL SECURITY;

ALTER TABLE ocr_records ENABLE ROW LEVEL SECURITY;
ALTER TABLE ocr_records FORCE ROW LEVEL SECURITY;

-- Dynamic Tenant Isolation Policies (Permissive by default)
DROP POLICY IF EXISTS tenant_isolation_store_rolls ON store_rolls;
CREATE POLICY tenant_isolation_store_rolls ON store_rolls
    FOR ALL
    TO PUBLIC
    USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), ''))
    WITH CHECK (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), ''));

DROP POLICY IF EXISTS tenant_isolation_store_issuances ON store_issuances;
CREATE POLICY tenant_isolation_store_issuances ON store_issuances
    FOR ALL
    TO PUBLIC
    USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), ''))
    WITH CHECK (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), ''));

DROP POLICY IF EXISTS tenant_isolation_cut_lay_rolls ON cut_lay_rolls;
CREATE POLICY tenant_isolation_cut_lay_rolls ON cut_lay_rolls
    FOR ALL
    TO PUBLIC
    USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), ''))
    WITH CHECK (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), ''));

DROP POLICY IF EXISTS tenant_isolation_bundle_registry ON bundle_registry;
CREATE POLICY tenant_isolation_bundle_registry ON bundle_registry
    FOR ALL
    TO PUBLIC
    USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), ''))
    WITH CHECK (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), ''));

DROP POLICY IF EXISTS tenant_isolation_carton_pack_manifests ON carton_pack_manifests;
CREATE POLICY tenant_isolation_carton_pack_manifests ON carton_pack_manifests
    FOR ALL
    TO PUBLIC
    USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), ''))
    WITH CHECK (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), ''));

DROP POLICY IF EXISTS tenant_isolation_hangtag_binds ON hangtag_binds;
CREATE POLICY tenant_isolation_hangtag_binds ON hangtag_binds
    FOR ALL
    TO PUBLIC
    USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), ''))
    WITH CHECK (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), ''));

DROP POLICY IF EXISTS tenant_isolation_floor_issues ON floor_issues;
CREATE POLICY tenant_isolation_floor_issues ON floor_issues
    FOR ALL
    TO PUBLIC
    USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), ''))
    WITH CHECK (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), ''));

DROP POLICY IF EXISTS tenant_isolation_ocr_records ON ocr_records;
CREATE POLICY tenant_isolation_ocr_records ON ocr_records
    FOR ALL
    TO PUBLIC
    USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), ''))
    WITH CHECK (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), ''));
