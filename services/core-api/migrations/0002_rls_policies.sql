ALTER TABLE store_rolls ENABLE ROW LEVEL SECURITY;
ALTER TABLE store_rolls FORCE ROW LEVEL SECURITY;

ALTER TABLE store_issuances ENABLE ROW LEVEL SECURITY;
ALTER TABLE store_issuances FORCE ROW LEVEL SECURITY;

ALTER TABLE bundle_registry ENABLE ROW LEVEL SECURITY;
ALTER TABLE bundle_registry FORCE ROW LEVEL SECURITY;

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