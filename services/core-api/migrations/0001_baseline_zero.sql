-- ============================================================================
-- AGIPAREL OS: BASELINE ZERO (0001_baseline_zero.sql)
-- Reference Site: Noble Garments, OKHLA Unit 002/02
-- Covers Domains 0 through 11
-- ============================================================================

CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- ----------------------------------------------------------------------------
-- DOMAIN 0: TENANTS & SYSTEM ROLES
-- ----------------------------------------------------------------------------
-- Replace lines 12-25 with properly closed DDL:
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

CREATE TABLE IF NOT EXISTS store_rolls (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    plant_id UUID NOT NULL,
    factory_sku VARCHAR(128) NOT NULL,
    original_length_m NUMERIC(10,2) NOT NULL CHECK (original_length_m > 0),
    remaining_length_m NUMERIC(10,2) NOT NULL CHECK (remaining_length_m >= 0),
    usable_width_cm NUMERIC(6,2) NOT NULL CHECK (usable_width_cm > 0),
    shade_band VARCHAR(32) NOT NULL,
    defect_map_json JSONB NOT NULL DEFAULT '[]'::jsonb,
    bin_location VARCHAR(64) NOT NULL,
    status VARCHAR(32) NOT NULL CHECK (status IN ('AVAILABLE', 'RESERVED', 'ISSUED', 'DEPLETED')),
    reserved_order_id UUID,
    reserved_cut_no INT CHECK (reserved_cut_no IS NULL OR reserved_cut_no > 0),
    aggregate_version BIGINT NOT NULL DEFAULT 0,
    last_rfid_sync TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_store_rolls_tenant_status ON store_rolls(tenant_id, status);

CREATE TABLE IF NOT EXISTS bundle_registry (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    plant_id UUID NOT NULL,
    order_id UUID NOT NULL,
    cut_no INT NOT NULL CHECK (cut_no > 0),
    sequential_bundle_no INT NOT NULL CHECK (sequential_bundle_no > 0),
    pant_position INT NOT NULL CHECK (pant_position > 0),
    part_code VARCHAR(64) NOT NULL,
    sub_bundle_id INT NOT NULL DEFAULT 1 CHECK (sub_bundle_id > 0),
    ssb_ply_group INT NOT NULL CHECK (ssb_ply_group > 0),
    part_description VARCHAR(128) NOT NULL,
    ply_start INT NOT NULL CHECK (ply_start > 0),
    ply_end INT NOT NULL CHECK (ply_end >= ply_start),
    piece_count INT NOT NULL CHECK (piece_count > 0),
    garment_serial_start INT NOT NULL CHECK (garment_serial_start > 0),
    garment_serial_end INT NOT NULL CHECK (garment_serial_end >= garment_serial_start),
    active_nfc_uid VARCHAR(128) UNIQUE,
    mapped_by_helper_id UUID,
    mapped_at TIMESTAMPTZ,
    aggregate_version BIGINT NOT NULL DEFAULT 0,
    status VARCHAR(32) NOT NULL CHECK (status IN ('PENDING_MAP', 'IN_SEWING', 'COMPLETED', 'SCRAPPED'))
);

CREATE INDEX IF NOT EXISTS idx_bundle_active_nfc ON bundle_registry(active_nfc_uid);

CREATE TABLE IF NOT EXISTS carton_pack_manifests (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    plant_id UUID NOT NULL,
    order_id UUID NOT NULL,
    box_range_start INT NOT NULL CHECK (box_range_start > 0),
    box_range_end INT NOT NULL CHECK (box_range_end >= box_range_start),
    pack_type VARCHAR(32) NOT NULL CHECK (pack_type IN ('SOLID', 'RATIO')),
    slot_rules_json JSONB NOT NULL,
    expected_gross_weight_kg NUMERIC(6,3) NOT NULL CHECK (expected_gross_weight_kg > 0),
    weight_tolerance_kg NUMERIC(4,3) NOT NULL DEFAULT 0.250,
    aggregate_version BIGINT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS dynamic_work_orders (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    plant_id UUID NOT NULL,
    po_number VARCHAR(64) NOT NULL,
    style_id UUID NOT NULL REFERENCES styles_master(id) ON DELETE RESTRICT,
    line_id UUID NOT NULL,
    supervisor_id UUID NOT NULL REFERENCES workers(id) ON DELETE RESTRICT,
    ordered_qty INT NOT NULL CHECK (ordered_qty > 0),
    color_matrix JSONB NOT NULL,
    size_matrix JSONB NOT NULL,
    packaging_bulletin JSONB NOT NULL,
    shipping_spec JSONB NOT NULL,
    aggregate_version BIGINT NOT NULL DEFAULT 0,
    status VARCHAR(32) NOT NULL CHECK (status IN ('PLANNED', 'IN_PROGRESS', 'COMPLETED', 'HOLD')),
    started_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Backward compatibility view for previous simple queries
CREATE OR REPLACE VIEW work_orders AS 
SELECT 
    dwo.id,
    dwo.tenant_id,
    dwo.id AS order_id,
    dwo.po_number,
    sm.style_code,
    dwo.line_id,
    dwo.supervisor_id,
    dwo.ordered_qty AS target_qty,
    0 AS completed_qty,
    dwo.status,
    dwo.started_at
FROM dynamic_work_orders dwo
JOIN styles_master sm ON dwo.style_id = sm.id;

-- ----------------------------------------------------------------------------
-- DEFAULT BOOTSTRAP TENANT (Populated via Pre-Boarding Workbook Import)
-- ----------------------------------------------------------------------------
INSERT INTO tenants (id, name)
VALUES ('TENANT_DEFAULT', 'Unconfigured Factory (Upload Pre-Boarding Workbook)')
ON CONFLICT (id) DO NOTHING;
-- ============================================================================
-- DOMAIN 12: REAL-WORLD WORK ORDER SKU MATRIX & TRIM BOM
-- ============================================================================
CREATE TABLE IF NOT EXISTS order_sku_matrix (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id VARCHAR(64) NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    order_id VARCHAR(64) NOT NULL REFERENCES dynamic_work_orders(id) ON DELETE CASCADE,
    color_code VARCHAR(64) NOT NULL,
    size_code VARCHAR(32) NOT NULL,
    upc VARCHAR(64) NOT NULL,
    ordered_qty INT NOT NULL CHECK (ordered_qty > 0),
    overcut_qty INT NOT NULL CHECK (overcut_qty >= 0),
    target_cut_volume INT NOT NULL CHECK (target_cut_volume > 0),
    CONSTRAINT uq_order_color_size UNIQUE (tenant_id, order_id, color_code, size_code)
);
ALTER TABLE order_sku_matrix ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation_order_sku_matrix ON order_sku_matrix FOR ALL USING (tenant_id = CURRENT_SETTING('app.current_tenant_id'));

CREATE TABLE IF NOT EXISTS bom_trim_requirements (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id VARCHAR(64) NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    style_id VARCHAR(64) NOT NULL REFERENCES styles_master(id) ON DELETE CASCADE,
    trim_code VARCHAR(64) NOT NULL,
    trim_description VARCHAR(255) NOT NULL,
    supplier_lot VARCHAR(128),
    required_qty_per_garment NUMERIC(10,3) NOT NULL CHECK (required_qty_per_garment > 0),
    uom VARCHAR(32) NOT NULL, -- e.g., 'PCS', 'YDS', 'MTRS'
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
ALTER TABLE bom_trim_requirements ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation_bom_trims ON bom_trim_requirements FOR ALL USING (tenant_id = CURRENT_SETTING('app.current_tenant_id'));

CREATE TABLE IF NOT EXISTS post_sewing_processes (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id VARCHAR(64) NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    order_id VARCHAR(64) NOT NULL REFERENCES dynamic_work_orders(id) ON DELETE CASCADE,
    process_name VARCHAR(128) NOT NULL, -- e.g., 'Enzyme Stone Wash', 'Chemical Softener'
    facility_unit VARCHAR(128) NOT NULL,
    wash_duration_mins INT NOT NULL CHECK (wash_duration_mins > 0),
    target_shrinkage_warp_pct NUMERIC(5,2) NOT NULL DEFAULT -1.50,
    target_shrinkage_weft_pct NUMERIC(5,2) NOT NULL DEFAULT -2.00,
    status VARCHAR(32) NOT NULL CHECK (status IN ('PENDING', 'IN_PROCESS', 'COMPLETED')),
    processed_at TIMESTAMPTZ
);
ALTER TABLE post_sewing_processes ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation_post_sewing ON post_sewing_processes FOR ALL USING (tenant_id = CURRENT_SETTING('app.current_tenant_id'));

CREATE TABLE IF NOT EXISTS plant_hardware_assets (
    id VARCHAR(64) PRIMARY KEY,
    tenant_id VARCHAR(64) NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    asset_class VARCHAR(64) NOT NULL, -- e.g., 'RFID_SLED', 'BENCH_SCALE', 'MODBUS_RELAY', 'UHF_PORTAL', 'ROUTER'
    serial_number VARCHAR(128) NOT NULL UNIQUE,
    ip_address VARCHAR(64),
    mac_address VARCHAR(64),
    zone VARCHAR(64) NOT NULL,
    station_id VARCHAR(64),
    status VARCHAR(32) NOT NULL CHECK (status IN ('ACTIVE', 'MAINTENANCE', 'OFFLINE')),
    last_ping_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS store_rolls (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    plant_id UUID NOT NULL,
    factory_sku VARCHAR(128) NOT NULL,
    original_length_m NUMERIC(10,2) NOT NULL CHECK (original_length_m > 0),
    remaining_length_m NUMERIC(10,2) NOT NULL CHECK (remaining_length_m >= 0),
    usable_width_cm NUMERIC(6,2) NOT NULL CHECK (usable_width_cm > 0),
    shade_band VARCHAR(32) NOT NULL,
    defect_map_json JSONB NOT NULL DEFAULT '[]'::jsonb,
    bin_location VARCHAR(64) NOT NULL,
    status VARCHAR(32) NOT NULL CHECK (status IN ('AVAILABLE', 'RESERVED', 'ISSUED', 'DEPLETED')),
    reserved_order_id UUID,
    reserved_cut_no INT CHECK (reserved_cut_no IS NULL OR reserved_cut_no > 0),
    aggregate_version BIGINT NOT NULL DEFAULT 0,
    last_rfid_sync TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS bundle_registry (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    plant_id UUID NOT NULL,
    order_id UUID NOT NULL,
    cut_no INT NOT NULL CHECK (cut_no > 0),
    sequential_bundle_no INT NOT NULL CHECK (sequential_bundle_no > 0),
    pant_position INT NOT NULL CHECK (pant_position > 0),
    part_code VARCHAR(64) NOT NULL,
    sub_bundle_id INT NOT NULL DEFAULT 1 CHECK (sub_bundle_id > 0),
    ssb_ply_group INT NOT NULL CHECK (ssb_ply_group > 0),
    part_description VARCHAR(128) NOT NULL,
    ply_start INT NOT NULL CHECK (ply_start > 0),
    ply_end INT NOT NULL CHECK (ply_end >= ply_start),
    piece_count INT NOT NULL CHECK (piece_count > 0),
    garment_serial_start INT NOT NULL CHECK (garment_serial_start > 0),
    garment_serial_end INT NOT NULL CHECK (garment_serial_end >= garment_serial_start),
    active_nfc_uid VARCHAR(128) UNIQUE,
    aggregate_version BIGINT NOT NULL DEFAULT 0,
    status VARCHAR(32) NOT NULL CHECK (status IN ('PENDING_MAP', 'IN_SEWING', 'COMPLETED', 'SCRAPPED'))
);
ALTER TABLE plant_hardware_assets ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation_hardware_assets ON plant_hardware_assets FOR ALL USING (tenant_id = CURRENT_SETTING('app.current_tenant_id'));