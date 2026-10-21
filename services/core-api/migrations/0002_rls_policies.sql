BEGIN;

ALTER TABLE store_rolls ENABLE ROW LEVEL SECURITY;
ALTER TABLE bundle_registry ENABLE ROW LEVEL SECURITY;
ALTER TABLE cut_lay_rolls ENABLE ROW LEVEL SECURITY;
ALTER TABLE carton_pack_manifests ENABLE ROW LEVEL SECURITY;
ALTER TABLE ocr_records ENABLE ROW LEVEL SECURITY;
ALTER TABLE floor_issues ENABLE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation_store_rolls ON store_rolls
    AS RESTRICTIVE
    USING (tenant_id = NULLIF(CURRENT_SETTING('app.current_tenant_id', true), ''))
    WITH CHECK (tenant_id = NULLIF(CURRENT_SETTING('app.current_tenant_id', true), ''));

CREATE POLICY tenant_isolation_bundle_registry ON bundle_registry
    AS RESTRICTIVE
    USING (tenant_id = NULLIF(CURRENT_SETTING('app.current_tenant_id', true), ''))
    WITH CHECK (tenant_id = NULLIF(CURRENT_SETTING('app.current_tenant_id', true), ''));

CREATE POLICY tenant_isolation_cut_lay_rolls ON cut_lay_rolls
    AS RESTRICTIVE
    USING (tenant_id = NULLIF(CURRENT_SETTING('app.current_tenant_id', true), ''))
    WITH CHECK (tenant_id = NULLIF(CURRENT_SETTING('app.current_tenant_id', true), ''));

CREATE POLICY tenant_isolation_carton_pack_manifests ON carton_pack_manifests
    AS RESTRICTIVE
    USING (tenant_id = NULLIF(CURRENT_SETTING('app.current_tenant_id', true), ''))
    WITH CHECK (tenant_id = NULLIF(CURRENT_SETTING('app.current_tenant_id', true), ''));

CREATE POLICY tenant_isolation_ocr_records ON ocr_records
    AS RESTRICTIVE
    USING (tenant_id = NULLIF(CURRENT_SETTING('app.current_tenant_id', true), ''))
    WITH CHECK (tenant_id = NULLIF(CURRENT_SETTING('app.current_tenant_id', true), ''));

CREATE POLICY tenant_isolation_floor_issues ON floor_issues
    AS RESTRICTIVE
    USING (tenant_id = NULLIF(CURRENT_SETTING('app.current_tenant_id', true), ''))
    WITH CHECK (tenant_id = NULLIF(CURRENT_SETTING('app.current_tenant_id', true), ''));

COMMIT;