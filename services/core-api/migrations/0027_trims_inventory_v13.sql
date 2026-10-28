-- Batch 13: Trims and Accessories Receipt and Issuance Ledger
BEGIN;

CREATE TABLE IF NOT EXISTS store_trims_v13 (
    tenant_id VARCHAR(64) NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    id VARCHAR(64) NOT NULL,
    plant_id VARCHAR(64) NOT NULL DEFAULT 'PLANT_DEFAULT',
    trim_code VARCHAR(64) NOT NULL,
    description VARCHAR(255) NOT NULL,
    supplier_lot VARCHAR(128) NOT NULL,
    color_code VARCHAR(64) NOT NULL DEFAULT '',
    size_code VARCHAR(32) NOT NULL DEFAULT '',
    uom VARCHAR(32) NOT NULL,
    received_qty NUMERIC(12,3) NOT NULL CHECK (received_qty > 0),
    remaining_qty NUMERIC(12,3) NOT NULL CHECK (remaining_qty >= 0),
    status VARCHAR(32) NOT NULL DEFAULT 'AVAILABLE' CHECK (status IN ('AVAILABLE', 'DEPLETED', 'HOLD', 'REJECTED')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (tenant_id, id)
);

CREATE TABLE IF NOT EXISTS trim_issuances_v13 (
    tenant_id VARCHAR(64) NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    trim_stock_id VARCHAR(64) NOT NULL,
    order_id VARCHAR(64) NOT NULL,
    line_id VARCHAR(64) NOT NULL,
    issued_qty NUMERIC(12,3) NOT NULL CHECK (issued_qty > 0),
    issuer_id VARCHAR(64) NOT NULL,
    recipient_id VARCHAR(64) NOT NULL,
    issued_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (tenant_id, trim_stock_id) REFERENCES store_trims_v13(tenant_id, id) ON DELETE RESTRICT
);

ALTER TABLE store_trims_v13 ENABLE ROW LEVEL SECURITY;
ALTER TABLE trim_issuances_v13 ENABLE ROW LEVEL SECURITY;

CREATE POLICY store_trims_tenant_v13 ON store_trims_v13
    FOR ALL TO agiparel_app
    USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), ''))
    WITH CHECK (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), ''));

CREATE POLICY trim_issuances_tenant_v13 ON trim_issuances_v13
    FOR ALL TO agiparel_app
    USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), ''))
    WITH CHECK (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), ''));

GRANT SELECT, INSERT, UPDATE ON store_trims_v13 TO agiparel_app;
GRANT SELECT, INSERT ON trim_issuances_v13 TO agiparel_app;

COMMIT;