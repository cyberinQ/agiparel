<<<<<<< HEAD
-- Author: Sunil Jain, 2026-10-25 — Persist intake document provenance for Epic 1.
-- ============================================================================
-- AGIPAREL OS: BASELINE ZERO (0001_baseline_zero.sql) - REPAIRED & UNIFIED
-- Reference Site: Noble Garments, OKHLA Unit 002/02
-- Covers Domains 0 through 12 + All RLS Tables required by 0002_rls_policies.sql
=======
-- ============================================================================
-- AGIPAREL OS: BASELINE ZERO (0001_baseline_zero.sql)
-- Reference Site: Noble Garments, OKHLA Unit 002/02
-- Covers Domains 0 through 11
>>>>>>> 62b87176970aae981bc0876d77b7cab60eca07b1
-- ============================================================================

CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- ----------------------------------------------------------------------------
<<<<<<< HEAD
-- DOMAIN 0: TENANTS & MASTER OPERATIONS
-- ----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS tenants (
    id VARCHAR(64) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    unit_code VARCHAR(64) NOT NULL DEFAULT 'Unit 001',
    shift_start_hour INT NOT NULL DEFAULT 8 CHECK (shift_start_hour BETWEEN 0 AND 23),
    default_bundle_size INT NOT NULL DEFAULT 20 CHECK (default_bundle_size > 0),
=======
-- DOMAIN 0: TENANTS & SYSTEM ROLES
-- ----------------------------------------------------------------------------
-- Replace lines 12-25 with properly closed DDL:
CREATE TABLE IF NOT EXISTS tenants (
    id VARCHAR(64) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
>>>>>>> 62b87176970aae981bc0876d77b7cab60eca07b1
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS operation_master (
    id VARCHAR(64) PRIMARY KEY,
    operation_name VARCHAR(255) NOT NULL,
    default_sam NUMERIC(6,3) NOT NULL CHECK (default_sam > 0),
    machine_type VARCHAR(64) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

<<<<<<< HEAD
-- ----------------------------------------------------------------------------
-- DOMAIN 1: WORKFORCE, ATTENDANCE & MACHINES
-- ----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS workers (
    id VARCHAR(64) PRIMARY KEY,
    tenant_id VARCHAR(64) NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    name VARCHAR(255) NOT NULL,
    employee_code VARCHAR(64) NOT NULL,
    primary_skill VARCHAR(64) NOT NULL,
    secondary_skill VARCHAR(64) NOT NULL DEFAULT '',
    skill_grade VARCHAR(16) NOT NULL DEFAULT 'A',
    hourly_rate_inr NUMERIC(10,2) NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS attendance_register (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id VARCHAR(64) NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    worker_id VARCHAR(64) NOT NULL REFERENCES workers(id) ON DELETE CASCADE,
    shift_date DATE NOT NULL DEFAULT CURRENT_DATE,
    clock_in_time TIMESTAMPTZ,
    status VARCHAR(32) NOT NULL CHECK (status IN ('PRESENT', 'ABSENT', 'LATE', 'ON_LEAVE')),
    assigned_line_id VARCHAR(64) NOT NULL DEFAULT '',
    assigned_station_id VARCHAR(64) NOT NULL DEFAULT '',
    UNIQUE (tenant_id, worker_id, shift_date)
);

CREATE TABLE IF NOT EXISTS machine_roster (
    id VARCHAR(64) PRIMARY KEY,
    tenant_id VARCHAR(64) NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    line_id VARCHAR(64) NOT NULL,
    station_id VARCHAR(64) NOT NULL,
    name VARCHAR(255) NOT NULL,
    canonical_class VARCHAR(64) NOT NULL,
    rated_capacity_uph INT NOT NULL DEFAULT 100 CHECK (rated_capacity_uph > 0),
    status VARCHAR(32) NOT NULL DEFAULT 'ACTIVE' CHECK (status IN ('ACTIVE', 'IDLE', 'BREAKDOWN', 'MAINTENANCE')),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- ----------------------------------------------------------------------------
-- DOMAIN 2: STYLES, PARTS, BOM & ASSEMBLY DAG
-- ----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS styles_master (
    id VARCHAR(64) PRIMARY KEY,
    tenant_id VARCHAR(64) NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    style_code VARCHAR(64) NOT NULL,
    category VARCHAR(64) NOT NULL DEFAULT 'GARMENT',
    fit_profile VARCHAR(64) NOT NULL DEFAULT 'REGULAR_FIT',
    target_sam NUMERIC(6,3) NOT NULL DEFAULT 0.60 CHECK (target_sam > 0),
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (tenant_id, style_code)
);

CREATE TABLE IF NOT EXISTS style_parts (
    id VARCHAR(128) PRIMARY KEY,
    tenant_id VARCHAR(64) NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    style_id VARCHAR(64) NOT NULL REFERENCES styles_master(id) ON DELETE CASCADE,
    part_code VARCHAR(64) NOT NULL,
    part_name VARCHAR(128) NOT NULL,
    sub_bundle_order INT NOT NULL DEFAULT 1 CHECK (sub_bundle_order > 0),
    material_sku VARCHAR(128) NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS style_assembly_dag (
    id VARCHAR(128) PRIMARY KEY,
    tenant_id VARCHAR(64) NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    style_id VARCHAR(64) NOT NULL REFERENCES styles_master(id) ON DELETE CASCADE,
    parent_part_id VARCHAR(128) NOT NULL REFERENCES style_parts(id) ON DELETE CASCADE,
    child_part_id VARCHAR(128) NOT NULL REFERENCES style_parts(id) ON DELETE CASCADE,
    operation_id VARCHAR(64) NOT NULL REFERENCES operation_master(id) ON DELETE RESTRICT,
    machine_type_req VARCHAR(64) NOT NULL
);

CREATE TABLE IF NOT EXISTS bom_trim_requirements (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id VARCHAR(64) NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    style_id VARCHAR(64) NOT NULL REFERENCES styles_master(id) ON DELETE CASCADE,
    trim_code VARCHAR(64) NOT NULL,
    trim_description VARCHAR(255) NOT NULL,
    supplier_lot VARCHAR(128),
    required_qty_per_garment NUMERIC(10,3) NOT NULL CHECK (required_qty_per_garment > 0),
    uom VARCHAR(32) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- ----------------------------------------------------------------------------
-- DOMAIN 3: STORES, FABRIC ROLLS & ISSUANCES
-- ----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS store_rolls (
    id VARCHAR(64) PRIMARY KEY,
    tenant_id VARCHAR(64) NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    plant_id VARCHAR(64) NOT NULL DEFAULT 'PLANT_DEFAULT',
=======
CREATE TABLE IF NOT EXISTS store_rolls (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    plant_id UUID NOT NULL,
>>>>>>> 62b87176970aae981bc0876d77b7cab60eca07b1
    factory_sku VARCHAR(128) NOT NULL,
    original_length_m NUMERIC(10,2) NOT NULL CHECK (original_length_m > 0),
    remaining_length_m NUMERIC(10,2) NOT NULL CHECK (remaining_length_m >= 0),
    usable_width_cm NUMERIC(6,2) NOT NULL CHECK (usable_width_cm > 0),
    shade_band VARCHAR(32) NOT NULL,
    defect_map_json JSONB NOT NULL DEFAULT '[]'::jsonb,
    bin_location VARCHAR(64) NOT NULL,
<<<<<<< HEAD
    status VARCHAR(32) NOT NULL CHECK (status IN ('AVAILABLE', 'RESERVED', 'ISSUED', 'DEPLETED', 'QUARANTINE')),
    reserved_order_id VARCHAR(64),
    reserved_cut_no INT CHECK (reserved_cut_no IS NULL OR reserved_cut_no >= 0),
=======
    status VARCHAR(32) NOT NULL CHECK (status IN ('AVAILABLE', 'RESERVED', 'ISSUED', 'DEPLETED')),
    reserved_order_id UUID,
    reserved_cut_no INT CHECK (reserved_cut_no IS NULL OR reserved_cut_no > 0),
>>>>>>> 62b87176970aae981bc0876d77b7cab60eca07b1
    aggregate_version BIGINT NOT NULL DEFAULT 0,
    last_rfid_sync TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_store_rolls_tenant_status ON store_rolls(tenant_id, status);

<<<<<<< HEAD
CREATE TABLE IF NOT EXISTS store_issuances (
    id VARCHAR(64) PRIMARY KEY,
    tenant_id VARCHAR(64) NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    roll_id VARCHAR(64) NOT NULL REFERENCES store_rolls(id) ON DELETE RESTRICT,
    order_id VARCHAR(64) NOT NULL,
    cut_no INT NOT NULL CHECK (cut_no > 0),
    issued_length_m NUMERIC(10,2) NOT NULL CHECK (issued_length_m > 0),
    remaining_length_m NUMERIC(10,2) NOT NULL CHECK (remaining_length_m >= 0),
    issuer_id VARCHAR(64) NOT NULL,
    recipient_id VARCHAR(64) NOT NULL,
    digital_signature_hash VARCHAR(128) NOT NULL,
    issued_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- ----------------------------------------------------------------------------
-- DOMAIN 4: WORK ORDERS & SKU MATRIX
-- ----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS dynamic_work_orders (
    id VARCHAR(64) PRIMARY KEY,
    tenant_id VARCHAR(64) NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    plant_id VARCHAR(64) NOT NULL DEFAULT 'PLANT_DEFAULT',
    po_number VARCHAR(64) NOT NULL,
    style_id VARCHAR(64) NOT NULL REFERENCES styles_master(id) ON DELETE RESTRICT,
    line_id VARCHAR(64) NOT NULL,
    supervisor_id VARCHAR(64) NOT NULL REFERENCES workers(id) ON DELETE RESTRICT,
    ordered_qty INT NOT NULL CHECK (ordered_qty > 0),
    color_matrix JSONB NOT NULL DEFAULT '{}'::jsonb,
    size_matrix JSONB NOT NULL DEFAULT '{}'::jsonb,
    packaging_bulletin JSONB NOT NULL DEFAULT '{}'::jsonb,
    shipping_spec JSONB NOT NULL DEFAULT '{}'::jsonb,
    aggregate_version BIGINT NOT NULL DEFAULT 0,
    status VARCHAR(32) NOT NULL CHECK (status IN ('PROPOSED', 'PLANNED', 'LOCKED', 'IN_PROGRESS', 'COMPLETED', 'HOLD')),
    started_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS order_sku_matrix (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id VARCHAR(64) NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    order_id VARCHAR(64) NOT NULL REFERENCES dynamic_work_orders(id) ON DELETE CASCADE,
    color_code VARCHAR(64) NOT NULL,
    color_name VARCHAR(128) NOT NULL DEFAULT '',
    size_code VARCHAR(32) NOT NULL,
    upc VARCHAR(64) NOT NULL,
    ordered_qty INT NOT NULL CHECK (ordered_qty > 0),
    overcut_qty INT NOT NULL CHECK (overcut_qty >= 0),
    target_cut_volume INT NOT NULL CHECK (target_cut_volume > 0),
    unit_price_usd NUMERIC(10,2) NOT NULL DEFAULT 0.00,
    CONSTRAINT uq_order_color_size UNIQUE (tenant_id, order_id, color_code, size_code)
);

-- Backward compatibility view for simple queries
CREATE OR REPLACE VIEW work_orders AS
SELECT
=======
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
>>>>>>> 62b87176970aae981bc0876d77b7cab60eca07b1
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
<<<<<<< HEAD
-- DOMAIN 5: CUTTING, BUNDLES & SEWING FLOOR EXECUTION
-- ----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS cut_lay_rolls (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id VARCHAR(64) NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    roll_id VARCHAR(64) NOT NULL REFERENCES store_rolls(id) ON DELETE RESTRICT,
    order_id VARCHAR(64) NOT NULL,
    cut_no INT NOT NULL CHECK (cut_no > 0),
    lay_start INT NOT NULL CHECK (lay_start > 0),
    lay_end INT NOT NULL CHECK (lay_end >= lay_start),
    plies_count INT NOT NULL CHECK (plies_count BETWEEN 1 AND 210),
    end_bit_length_m NUMERIC(10,2) NOT NULL CHECK (end_bit_length_m >= 0),
    end_bit_action VARCHAR(32) NOT NULL CHECK (end_bit_action IN ('SCRAP', 'RECUT_HOLD', 'STORE_RETURN')),
    recorded_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
=======
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
>>>>>>> 62b87176970aae981bc0876d77b7cab60eca07b1
);

CREATE TABLE IF NOT EXISTS bundle_registry (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
<<<<<<< HEAD
    tenant_id VARCHAR(64) NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    plant_id VARCHAR(64) NOT NULL DEFAULT 'PLANT_DEFAULT',
    order_id VARCHAR(64) NOT NULL,
=======
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    plant_id UUID NOT NULL,
    order_id UUID NOT NULL,
>>>>>>> 62b87176970aae981bc0876d77b7cab60eca07b1
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
<<<<<<< HEAD
    mapped_by_helper_id VARCHAR(64),
    mapped_at TIMESTAMPTZ,
    aggregate_version BIGINT NOT NULL DEFAULT 0,
    status VARCHAR(32) NOT NULL CHECK (status IN ('PENDING_MAP', 'IN_SEWING', 'COMPLETED', 'SCRAPPED'))
);

CREATE INDEX IF NOT EXISTS idx_bundle_active_nfc ON bundle_registry(active_nfc_uid);

-- ----------------------------------------------------------------------------
-- DOMAIN 6: FINISHING, PACKING, METRICS, ISSUES & OCR CLOSURE
-- ----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS hangtag_binds (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id VARCHAR(64) NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    order_id VARCHAR(64) NOT NULL,
    garment_serial INT NOT NULL,
    upc VARCHAR(64) NOT NULL,
    epc_sgtin96 VARCHAR(64) NOT NULL UNIQUE,
    bound_by_worker_id VARCHAR(64),
    bound_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS carton_pack_manifests (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id VARCHAR(64) NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    plant_id VARCHAR(64) NOT NULL DEFAULT 'PLANT_DEFAULT',
    order_id VARCHAR(64) NOT NULL,
    box_range_start INT NOT NULL CHECK (box_range_start > 0),
    box_range_end INT NOT NULL CHECK (box_range_end >= box_range_start),
    pack_type VARCHAR(32) NOT NULL CHECK (pack_type IN ('SOLID', 'RATIO')),
    color_code VARCHAR(64) NOT NULL DEFAULT '',
    slot_rules_json JSONB NOT NULL DEFAULT '[]'::jsonb,
    expected_gross_weight_kg NUMERIC(6,3) NOT NULL CHECK (expected_gross_weight_kg > 0),
    weight_tolerance_kg NUMERIC(4,3) NOT NULL DEFAULT 0.250,
    aggregate_version BIGINT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS post_sewing_processes (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id VARCHAR(64) NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    order_id VARCHAR(64) NOT NULL REFERENCES dynamic_work_orders(id) ON DELETE CASCADE,
    process_name VARCHAR(128) NOT NULL,
    facility_unit VARCHAR(128) NOT NULL,
    wash_duration_mins INT NOT NULL CHECK (wash_duration_mins > 0),
    target_shrinkage_warp_pct NUMERIC(5,2) NOT NULL DEFAULT -1.50,
    target_shrinkage_weft_pct NUMERIC(5,2) NOT NULL DEFAULT -2.00,
    status VARCHAR(32) NOT NULL CHECK (status IN ('PENDING', 'IN_PROCESS', 'COMPLETED')),
    processed_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS hourly_line_metrics (
    tenant_id VARCHAR(64) NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    line_id VARCHAR(64) NOT NULL,
    shift_date DATE NOT NULL DEFAULT CURRENT_DATE,
    hour_window INT NOT NULL CHECK (hour_window BETWEEN 1 AND 24),
    target_units INT NOT NULL DEFAULT 75,
    actual_units INT NOT NULL DEFAULT 0,
    defect_units INT NOT NULL DEFAULT 0,
    andon_downtime_minutes INT NOT NULL DEFAULT 0,
    PRIMARY KEY (tenant_id, line_id, shift_date, hour_window)
);

CREATE TABLE IF NOT EXISTS floor_issues (
    id VARCHAR(64) PRIMARY KEY,
    tenant_id VARCHAR(64) NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    issue_type VARCHAR(64) NOT NULL,
    severity VARCHAR(32) NOT NULL CHECK (severity IN ('info', 'attention', 'critical')),
    plant_id VARCHAR(128) NOT NULL,
    line_id VARCHAR(64) NOT NULL,
    station_id VARCHAR(64),
    machine_id VARCHAR(64),
    operator_id VARCHAR(64),
    title TEXT NOT NULL,
    started_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    resolved_at TIMESTAMPTZ,
    owner_id VARCHAR(64) NOT NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'open' CHECK (status IN ('open', 'in_progress', 'resolved'))
);

CREATE TABLE IF NOT EXISTS ocr_records (
    id VARCHAR(128) PRIMARY KEY,
    tenant_id VARCHAR(64) NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    order_id VARCHAR(64) NOT NULL,
    cut_no INT NOT NULL CHECK (cut_no > 0),
    cut_qty INT NOT NULL,
    shipped_qty INT NOT NULL,
    lost_qty INT NOT NULL,
    left_over_qty INT NOT NULL,
    variance INT NOT NULL,
    variance_comment TEXT,
    signatures_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    document_hash VARCHAR(128),
    is_sealed BOOLEAN NOT NULL DEFAULT false,
    sealed_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS plant_hardware_assets (
    id VARCHAR(64) PRIMARY KEY,
    tenant_id VARCHAR(64) NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    asset_class VARCHAR(64) NOT NULL,
    serial_number VARCHAR(128) NOT NULL UNIQUE,
    ip_address VARCHAR(64),
    mac_address VARCHAR(64),
    zone VARCHAR(64) NOT NULL,
    station_id VARCHAR(64),
    status VARCHAR(32) NOT NULL CHECK (status IN ('ACTIVE', 'MAINTENANCE', 'OFFLINE')),
    last_ping_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- ----------------------------------------------------------------------------
-- DEFAULT BOOTSTRAP TENANT
-- ----------------------------------------------------------------------------
INSERT INTO tenants (id, name, unit_code, shift_start_hour, default_bundle_size)
VALUES ('TENANT_DEFAULT', 'Noble Garments', 'OKHLA Unit 002/02', 8, 20)
ON CONFLICT (id) DO NOTHING;
-- Epic 1 intake provenance: immutable checksum and review state for every uploaded file.
CREATE TABLE IF NOT EXISTS documents (
    id VARCHAR(128) PRIMARY KEY,
    tenant_id VARCHAR(64) NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    filename TEXT NOT NULL,
    file_checksum CHAR(64) NOT NULL,
    source_mode VARCHAR(32) NOT NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'PROPOSED',
    conflicts_json JSONB NOT NULL DEFAULT '[]'::jsonb,
    imported_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (tenant_id, filename, file_checksum)
);
=======
    aggregate_version BIGINT NOT NULL DEFAULT 0,
    status VARCHAR(32) NOT NULL CHECK (status IN ('PENDING_MAP', 'IN_SEWING', 'COMPLETED', 'SCRAPPED'))
);
ALTER TABLE plant_hardware_assets ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation_hardware_assets ON plant_hardware_assets FOR ALL USING (tenant_id = CURRENT_SETTING('app.current_tenant_id'));
>>>>>>> 62b87176970aae981bc0876d77b7cab60eca07b1
