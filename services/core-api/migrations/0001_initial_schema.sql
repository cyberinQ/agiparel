CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- ============================================================================
-- DOMAIN 0: TENANTS & MASTER CATALOG
-- ============================================================================
CREATE TABLE IF NOT EXISTS tenants (
    id VARCHAR(64) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS operation_master (
    id VARCHAR(64) PRIMARY KEY,
    operation_name VARCHAR(255) NOT NULL,
    default_sam NUMERIC(6,3) NOT NULL CHECK (default_sam > 0),
    machine_type VARCHAR(64) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- ============================================================================
-- DOMAIN 1: STORES & RAW MATERIAL INVENTORY (mod_stores)
-- ============================================================================
CREATE TABLE IF NOT EXISTS store_rolls (
    id VARCHAR(64) PRIMARY KEY,
    tenant_id VARCHAR(64) NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    factory_sku VARCHAR(128) NOT NULL,
    original_length_m NUMERIC(10,2) NOT NULL CHECK (original_length_m > 0),
    remaining_length_m NUMERIC(10,2) NOT NULL CHECK (remaining_length_m >= 0),
    usable_width_cm NUMERIC(6,2) NOT NULL CHECK (usable_width_cm > 0),
    shade_band VARCHAR(32) NOT NULL,
    defect_map_json JSONB NOT NULL DEFAULT '[]'::jsonb,
    bin_location VARCHAR(64) NOT NULL,
    status VARCHAR(32) NOT NULL CHECK (status IN ('AVAILABLE', 'RESERVED', 'ISSUED', 'DEPLETED')),
    reserved_order_id VARCHAR(64),
    reserved_cut_no INT CHECK (reserved_cut_no IS NULL OR reserved_cut_no > 0),
    last_rfid_sync TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_store_rolls_tenant_status ON store_rolls(tenant_id, status);

CREATE TABLE IF NOT EXISTS store_issuances (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id VARCHAR(64) NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    roll_id VARCHAR(64) NOT NULL,
    order_id VARCHAR(64) NOT NULL,
    cut_no INT NOT NULL CHECK (cut_no > 0),
    issued_length_m NUMERIC(10,2) NOT NULL CHECK (issued_length_m > 0),
    issuer_digital_id VARCHAR(64) NOT NULL,
    recipient_digital_id VARCHAR(64) NOT NULL,
    issued_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    digital_signature_hash VARCHAR(256) NOT NULL
);

-- ============================================================================
-- DOMAIN 2: CUTTING & BUNDLE SERIALIZATION (mod_cutting)
-- ============================================================================
CREATE TABLE IF NOT EXISTS bundle_registry (
    id VARCHAR(64) PRIMARY KEY,
    tenant_id VARCHAR(64) NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    order_id VARCHAR(64) NOT NULL,
    cut_no INT NOT NULL CHECK (cut_no > 0),
    sequential_bundle_no INT NOT NULL CHECK (sequential_bundle_no > 0),
    pant_position INT NOT NULL CHECK (pant_position > 0),
    piece_count INT NOT NULL CHECK (piece_count > 0),
    status VARCHAR(32) NOT NULL CHECK (status IN ('PENDING_MAP', 'IN_SEWING', 'COMPLETED', 'SCRAPPED'))
);