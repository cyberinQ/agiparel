-- 0002_rls_policies.sql
-- Enforce Row-Level Security across all transactional entities
-- Author: Sunil Jain, CyberinQ LLC

-- Enable RLS on all tenant-bound tables
ALTER TABLE store_rolls ENABLE ROW LEVEL SECURITY;
ALTER TABLE store_issuances ENABLE ROW LEVEL SECURITY;
ALTER TABLE cut_lay_rolls ENABLE ROW LEVEL SECURITY;
ALTER TABLE bundle_registry ENABLE ROW LEVEL SECURITY;
ALTER TABLE carton_pack_manifests ENABLE ROW LEVEL SECURITY;
ALTER TABLE hangtag_binds ENABLE ROW LEVEL SECURITY;
ALTER TABLE floor_issues ENABLE ROW LEVEL SECURITY;
ALTER TABLE ocr_records ENABLE ROW LEVEL SECURITY;

-- Tenant Isolation Policies via app.current_tenant_id session parameter
CREATE POLICY rls_store_rolls ON store_rolls
    FOR ALL USING (tenant_id = CURRENT_SETTING('app.current_tenant_id', true));

CREATE POLICY rls_store_issuances ON store_issuances
    FOR ALL USING (tenant_id = CURRENT_SETTING('app.current_tenant_id', true));

CREATE POLICY rls_cut_lay_rolls ON cut_lay_rolls
    FOR ALL USING (tenant_id = CURRENT_SETTING('app.current_tenant_id', true));

CREATE POLICY rls_bundle_registry ON bundle_registry
    FOR ALL USING (tenant_id = CURRENT_SETTING('app.current_tenant_id', true));

CREATE POLICY rls_carton_pack_manifests ON carton_pack_manifests
    FOR ALL USING (tenant_id = CURRENT_SETTING('app.current_tenant_id', true));

CREATE POLICY rls_hangtag_binds ON hangtag_binds
    FOR ALL USING (tenant_id = CURRENT_SETTING('app.current_tenant_id', true));

CREATE POLICY rls_floor_issues ON floor_issues
    FOR ALL USING (tenant_id = CURRENT_SETTING('app.current_tenant_id', true));

CREATE POLICY rls_ocr_records ON ocr_records
    FOR ALL USING (tenant_id = CURRENT_SETTING('app.current_tenant_id', true));
