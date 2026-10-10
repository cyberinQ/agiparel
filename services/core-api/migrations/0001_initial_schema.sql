CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- ============================================================================
-- DOMAIN 0: TENANT & MASTER CATALOG
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
-- DOMAIN 1: STORES & RAW MATERIAL INVENTORY (MODULE: mod_stores)
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
CREATE INDEX IF NOT EXISTS idx_store_rolls_reservation ON store_rolls(tenant_id, reserved_order_id, reserved_cut_no);

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

CREATE INDEX IF NOT EXISTS idx_store_issuances_tenant_roll ON store_issuances(tenant_id, roll_id);

-- ============================================================================
-- DOMAIN 2: CUTTING & BUNDLE SERIALIZATION (MODULE: mod_cutting)
-- ============================================================================
CREATE TABLE IF NOT EXISTS cut_lay_rolls (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id VARCHAR(64) NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    order_id VARCHAR(64) NOT NULL,
    cut_no INT NOT NULL CHECK (cut_no > 0),
    roll_id VARCHAR(64) NOT NULL,
    lay_start INT NOT NULL CHECK (lay_start > 0),
    lay_end INT NOT NULL CHECK (lay_end >= lay_start),
    plies_count INT NOT NULL CHECK (plies_count > 0 AND plies_count <= 210),
    end_bit_length_m NUMERIC(10,2) NOT NULL CHECK (end_bit_length_m >= 0),
    end_bit_action VARCHAR(32) NOT NULL CHECK (end_bit_action IN ('SCRAP', 'RECUT_HOLD', 'STORE_RETURN')),
    recorded_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_cut_lay_rolls_tenant_order_cut ON cut_lay_rolls(tenant_id, order_id, cut_no);
CREATE INDEX IF NOT EXISTS idx_cut_lay_rolls_roll ON cut_lay_rolls(tenant_id, roll_id);

CREATE TABLE IF NOT EXISTS bundle_registry (
    id VARCHAR(64) PRIMARY KEY,
    tenant_id VARCHAR(64) NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    order_id VARCHAR(64) NOT NULL,
    cut_no INT NOT NULL CHECK (cut_no > 0),
    sequential_bundle_no INT NOT NULL CHECK (sequential_bundle_no > 0),
    pant_position INT NOT NULL CHECK (pant_position > 0),
    sub_bundle_id INT NOT NULL CHECK (sub_bundle_id > 0),
    ssb_ply_group INT NOT NULL CHECK (ssb_ply_group > 0),
    part_description VARCHAR(128) NOT NULL,
    ply_start INT NOT NULL CHECK (ply_start > 0),
    ply_end INT NOT NULL CHECK (ply_end >= ply_start),
    piece_count INT NOT NULL CHECK (piece_count > 0),
    garment_serial_start INT NOT NULL CHECK (garment_serial_start > 0),
    garment_serial_end INT NOT NULL CHECK (garment_serial_end >= garment_serial_start),
    active_nfc_uid VARCHAR(128) UNIQUE,
    mapped_by_helper_id VARCHAR(64),
    mapped_at TIMESTAMPTZ,
    status VARCHAR(32) NOT NULL CHECK (status IN ('PENDING_MAP', 'IN_SEWING', 'COMPLETED', 'SCRAPPED'))
);

CREATE INDEX IF NOT EXISTS idx_bundle_registry_tenant_order_cut ON bundle_registry(tenant_id, order_id, cut_no);
CREATE INDEX IF NOT EXISTS idx_bundle_registry_nfc ON bundle_registry(tenant_id, active_nfc_uid);

-- ============================================================================
-- DOMAIN 3: FINISHING & RATIO PACKING (MODULE: mod_packing)
-- ============================================================================
CREATE TABLE IF NOT EXISTS carton_pack_manifests (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id VARCHAR(64) NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    order_id VARCHAR(64) NOT NULL,
    box_range_start INT NOT NULL CHECK (box_range_