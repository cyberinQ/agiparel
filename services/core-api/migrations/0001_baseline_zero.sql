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
    roll_id VARCHAR(64) NOT NULL REFERENCES store_rolls(id),
    order_id VARCHAR(64) NOT NULL,
    cut_no INT NOT NULL CHECK (cut_no > 0),
    issued_length_m NUMERIC(10,2) NOT NULL CHECK (issued_length_m > 0),
    issuer_digital_id VARCHAR(64) NOT NULL,
    recipient_digital_id VARCHAR(64) NOT NULL,
    issued_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    digital_signature_hash VARCHAR(256) NOT NULL
);

CREATE TABLE IF NOT EXISTS cut_lay_rolls (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id VARCHAR(64) NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    order_id VARCHAR(64) NOT NULL,
    cut_no INT NOT NULL CHECK (cut_no > 0),
    roll_id VARCHAR(64) NOT NULL REFERENCES store_rolls(id),
    lay_start INT NOT NULL CHECK (lay_start > 0),
    lay_end INT NOT NULL CHECK (lay_end >= lay_start),
    plies_count INT NOT NULL CHECK (plies_count > 0),
    end_bit_length_m NUMERIC(10,2) NOT NULL CHECK (end_bit_length_m >= 0),
    end_bit_action VARCHAR(32) NOT NULL CHECK (end_bit_action IN ('SCRAP', 'RECUT_HOLD', 'STORE_RETURN')),
    recorded_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS bundle_registry (
    id VARCHAR(64) PRIMARY KEY,
    tenant_id VARCHAR(64) NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    order_id VARCHAR(64) NOT NULL,
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
    mapped_by_helper_id VARCHAR(64),
    mapped_at TIMESTAMPTZ,
    status VARCHAR(32) NOT NULL CHECK (status IN ('PENDING_MAP', 'IN_SEWING', 'COMPLETED', 'SCRAPPED'))
);
CREATE INDEX IF NOT EXISTS idx_bundle_active_nfc ON bundle_registry(active_nfc_uid);

CREATE TABLE IF NOT EXISTS hangtag_binds (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id VARCHAR(64) NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    bundle_id VARCHAR(64) NOT NULL REFERENCES bundle_registry(id),
    nfc_uid VARCHAR(128) NOT NULL,
    buyer_hangtag_barcode VARCHAR(128) NOT NULL,
    order_id VARCHAR(64) NOT NULL,
    style_id VARCHAR(64) NOT NULL,
    helper_id VARCHAR(64) NOT NULL,
    bound_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_tenant_hangtag UNIQUE (tenant_id, buyer_hangtag_barcode)
);

CREATE TABLE IF NOT EXISTS carton_pack_manifests (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id VARCHAR(64) NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    order_id VARCHAR(64) NOT NULL,
    box_range_start INT NOT NULL CHECK (box_range_start > 0),
    box_range_end INT NOT NULL CHECK (box_range_end >= box_range_start),
    pack_type VARCHAR(32) NOT NULL CHECK (pack_type IN ('SOLID', 'RATIO')),
    slot_rules_json JSONB NOT NULL,
    expected_gross_weight_kg NUMERIC(6,3) NOT NULL CHECK (expected_gross_weight_kg > 0),
    weight_tolerance_kg NUMERIC(4,3) NOT NULL DEFAULT 0.250,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS ocr_records (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id VARCHAR(64) NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    order_id VARCHAR(64) NOT NULL,
    stage VARCHAR(32) NOT NULL CHECK (stage IN ('CUT', 'SEWN', 'FINISHED', 'PACKED', 'LOADED')),
    forward_qty INT NOT NULL CHECK (forward_qty >= 0),
    lost_qty INT NOT NULL CHECK (lost_qty >= 0),
    left_over_qty INT NOT NULL CHECK (left_over_qty >= 0),
    supervisor_id VARCHAR(64) NOT NULL,
    variance_comment TEXT,
    signed_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS floor_issues (
    id VARCHAR(64) PRIMARY KEY,
    tenant_id VARCHAR(64) NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    issue_type VARCHAR(32) NOT NULL CHECK (issue_type IN ('machine', 'quality', 'output', 'workforce', 'maintenance')),
    severity VARCHAR(32) NOT NULL CHECK (severity IN ('critical', 'attention', 'info', 'resolved')),
    plant_id VARCHAR(64) NOT NULL,
    line_id VARCHAR(64) NOT NULL,
    station_id VARCHAR(64),
    machine_id VARCHAR(64),
    operator_id VARCHAR(64),
    title VARCHAR(255) NOT NULL,
    started_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    resolved_at TIMESTAMPTZ,
    owner_id VARCHAR(64) NOT NULL,
    watchers JSONB NOT NULL DEFAULT '[]'::jsonb,
    status VARCHAR(32) NOT NULL CHECK (status IN ('open', 'in_progress', 'resolved'))
);

CREATE TABLE IF NOT EXISTS workers (
    id VARCHAR(64) PRIMARY KEY,
    tenant_id VARCHAR(64) NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    name VARCHAR(255) NOT NULL,
    employee_code VARCHAR(64) NOT NULL,
    primary_skill VARCHAR(64) NOT NULL,
    skill_grade VARCHAR(8) NOT NULL CHECK (skill_grade IN ('A', 'B', 'C', 'TRAINEE')),
    hourly_rate_inr NUMERIC(10,2) NOT NULL DEFAULT 0.00,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS attendance_register (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id VARCHAR(64) NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    worker_id VARCHAR(64) NOT NULL REFERENCES workers(id) ON DELETE RESTRICT,
    shift_date DATE NOT NULL,
    clock_in_time TIMESTAMPTZ,
    clock_out_time TIMESTAMPTZ,
    status VARCHAR(32) NOT NULL CHECK (status IN ('PRESENT', 'ABSENT', 'LATE', 'ON_LEAVE')),
    assigned_line_id VARCHAR(64),
    assigned_station_id VARCHAR(64)
);

CREATE TABLE IF NOT EXISTS machine_roster (
    id VARCHAR(64) PRIMARY KEY,
    tenant_id VARCHAR(64) NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    line_id VARCHAR(64) NOT NULL,
    station_id VARCHAR(64),
    name VARCHAR(128) NOT NULL,
    canonical_class VARCHAR(64) NOT NULL,
    rated_capacity_uph INT NOT NULL CHECK (rated_capacity_uph > 0),
    status VARCHAR(32) NOT NULL CHECK (status IN ('ACTIVE', 'IDLE', 'BREAKDOWN', 'MAINTENANCE')),
    active_operator_id VARCHAR(64) REFERENCES workers(id)
);

CREATE TABLE IF NOT EXISTS hourly_line_metrics (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id VARCHAR(64) NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    line_id VARCHAR(64) NOT NULL,
    shift_date DATE NOT NULL,
    hour_window INT NOT NULL CHECK (hour_window BETWEEN 1 AND 16),
    target_units INT NOT NULL CHECK (target_units >= 0),
    actual_units INT NOT NULL DEFAULT 0 CHECK (actual_units >= 0),
    defect_units INT NOT NULL DEFAULT 0 CHECK (defect_units >= 0),
    andon_downtime_minutes INT NOT NULL DEFAULT 0 CHECK (andon_downtime_minutes >= 0),
    dhu_pct NUMERIC(5,2) GENERATED ALWAYS AS (
        CASE WHEN actual_units > 0 THEN (defect_units::numeric / actual_units::numeric) * 100 ELSE 0 END
    ) STORED,
    recorded_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_line_hour UNIQUE (tenant_id, line_id, shift_date, hour_window)
);

CREATE TABLE IF NOT EXISTS styles_master (
    id VARCHAR(64) PRIMARY KEY,
    tenant_id VARCHAR(64) NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    style_code VARCHAR(64) NOT NULL,
    category VARCHAR(64) NOT NULL,
    fit_profile VARCHAR(64),
    target_sam NUMERIC(8,2) NOT NULL CHECK (target_sam > 0),
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (tenant_id, style_code)
);

CREATE TABLE IF NOT EXISTS style_parts (
    id VARCHAR(64) PRIMARY KEY,
    tenant_id VARCHAR(64) NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    style_id VARCHAR(64) NOT NULL REFERENCES styles_master(id) ON DELETE CASCADE,
    part_code VARCHAR(64) NOT NULL,
    part_name VARCHAR(128) NOT NULL,
    sub_bundle_order INT NOT NULL CHECK (sub_bundle_order > 0),
    material_sku VARCHAR(128) NOT NULL,
    properties JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (tenant_id, style_id, part_code)
);

CREATE TABLE IF NOT EXISTS style_assembly_dag (
    id VARCHAR(64) PRIMARY KEY,
    tenant_id VARCHAR(64) NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    style_id VARCHAR(64) NOT NULL REFERENCES styles_master(id) ON DELETE CASCADE,
    parent_part_id VARCHAR(64) NOT NULL REFERENCES style_parts(id) ON DELETE RESTRICT,
    child_part_id VARCHAR(64) NOT NULL REFERENCES style_parts(id) ON DELETE RESTRICT,
    operation_id VARCHAR(64) NOT NULL,
    machine_type_req VARCHAR(64) NOT NULL,
    quality_checkpoint_rule JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS dynamic_work_orders (
    id VARCHAR(64) PRIMARY KEY,
    tenant_id VARCHAR(64) NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    po_number VARCHAR(64) NOT NULL,
    style_id VARCHAR(64) NOT NULL REFERENCES styles_master(id) ON DELETE RESTRICT,
    line_id VARCHAR(64) NOT NULL,
    supervisor_id VARCHAR(64) NOT NULL REFERENCES workers(id) ON DELETE RESTRICT,
    ordered_qty INT NOT NULL CHECK (ordered_qty > 0),
    color_matrix JSONB NOT NULL,
    size_matrix JSONB NOT NULL,
    packaging_bulletin JSONB NOT NULL,
    shipping_spec JSONB NOT NULL,
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
ALTER TABLE plant_hardware_assets ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation_hardware_assets ON plant_hardware_assets FOR ALL USING (tenant_id = CURRENT_SETTING('app.current_tenant_id'));