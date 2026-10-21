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
-- SEED DATA: NOBLE GARMENTS OKHLA UNIT 002/02 GROUND TRUTH
-- ----------------------------------------------------------------------------
INSERT INTO tenants (id, name) VALUES ('TENANT_DEFAULT', 'Noble Garments Okhla Unit 002/02') ON CONFLICT (id) DO UPDATE SET name = EXCLUDED.name;

INSERT INTO workers (id, tenant_id, name, employee_code, primary_skill, skill_grade) VALUES
('W-01', 'TENANT_DEFAULT', 'Ram Kumar', 'EMP_SUPV_01', 'LINE_SUPERVISOR', 'A'),
('W-02', 'TENANT_DEFAULT', 'Sita Devi', 'EMP_A812', '4-OL_SERGING', 'A'),
('W-03', 'TENANT_DEFAULT', 'Sunita Sharma', 'EMP_A813', 'KANSAI_HEMMING', 'B'),
('W-04', 'TENANT_DEFAULT', 'Mohan Lal', 'EMP_A814', 'SNLS_BACK_RISE', 'A'),
('W-05', 'TENANT_DEFAULT', 'Ramesh Chand', 'EMP_MECH_01', 'LINE_MECHANIC', 'A')
ON CONFLICT (id) DO NOTHING;

INSERT INTO machine_roster (id, tenant_id, line_id, station_id, name, canonical_class, rated_capacity_uph, status, active_operator_id) VALUES
('M-01', 'TENANT_DEFAULT', 'Line 14', 'SUPERVISOR_DESK', 'Supervisor Terminal', 'TERMINAL', 150, 'ACTIVE', 'W-01'),
('M-02', 'TENANT_DEFAULT', 'Line 14', 'ST-02', 'Pegasus 4-Thread Overlock', '4-OL_SERGING', 120, 'ACTIVE', 'W-02'),
('M-03', 'TENANT_DEFAULT', 'Line 14', 'ST-03', 'Kansai Special Flatlock', 'KANSAI_FLATLOCK', 110, 'ACTIVE', 'W-03'),
('M-04', 'TENANT_DEFAULT', 'Line 14', 'ST-04', 'Juki DDL-9000C Lockstitch', 'SNLS_LOCKSTITCH', 100, 'ACTIVE', 'W-04'),
('M-05', 'TENANT_DEFAULT', 'Line 14', 'MAINT_BENCH', 'Maintenance Service Station', 'WORKBENCH', 60, 'ACTIVE', 'W-05')
ON CONFLICT (id) DO NOTHING;

INSERT INTO styles_master (id, tenant_id, style_code, category, fit_profile, target_sam, metadata) VALUES
('STYLE_KK524867', 'TENANT_DEFAULT', 'KK524867', 'WOVEN_TROUSER', 'REGULAR_FIT', 0.60, '{"fabric": "DENIM-STRETCH-12OZ", "tolerance_cm": 0.5}'::jsonb)
ON CONFLICT (id) DO NOTHING;

INSERT INTO dynamic_work_orders (id, tenant_id, po_number, style_id, line_id, supervisor_id, ordered_qty, color_matrix, size_matrix, packaging_bulletin, shipping_spec, status) VALUES
('WO-MK6574J9-01', 'TENANT_DEFAULT', 'MK6574J9', 'STYLE_KK524867', 'Line 14', 'W-01', 600, '{"BLK": 150, "BLU": 250, "GRN": 200}'::jsonb, '{"29/30": 600}'::jsonb, '{"box_type": "RATIO_6PC"}'::jsonb, '{"forwarder": "PERY_LOGISTICS"}'::jsonb, 'IN_PROGRESS')
ON CONFLICT (id) DO NOTHING;

INSERT INTO attendance_register (tenant_id, worker_id, shift_date, clock_in_time, status, assigned_line_id, assigned_station_id) VALUES
('TENANT_DEFAULT', 'W-01', CURRENT_DATE, CURRENT_TIMESTAMP - INTERVAL '4 hours', 'PRESENT', 'Line 14', 'SUPERVISOR_DESK'),
('TENANT_DEFAULT', 'W-02', CURRENT_DATE, CURRENT_TIMESTAMP - INTERVAL '4 hours', 'PRESENT', 'Line 14', 'ST-02'),
('TENANT_DEFAULT', 'W-03', CURRENT_DATE, NULL, 'ABSENT', 'Line 14', 'ST-03'),
('TENANT_DEFAULT', 'W-04', CURRENT_DATE, CURRENT_TIMESTAMP - INTERVAL '4 hours', 'PRESENT', 'Line 14', 'ST-04'),
('TENANT_DEFAULT', 'W-05', CURRENT_DATE, CURRENT_TIMESTAMP - INTERVAL '5 hours', 'PRESENT', 'Line 14', 'MECHANIC_FLOOR')
ON CONFLICT DO NOTHING;

INSERT INTO floor_issues (id, tenant_id, issue_type, severity, plant_id, line_id, station_id, operator_id, title, owner_id, status)
VALUES 
('ISS-1401', 'TENANT_DEFAULT', 'workforce', 'attention', 'Noble Garments', 'Line 14', 'ST-03', 'W-03', 'Sunita Sharma absent; station hem bottlenecking pace', 'W-01', 'open'),
('ISS-0201', 'TENANT_DEFAULT', 'maintenance', 'critical', 'Noble Garments', 'Line 02', 'ST-08', NULL, 'Needle timing slippage on Kansai flatlock', 'W-05', 'open')
ON CONFLICT (id) DO NOTHING;
-- ----------------------------------------------------------------------------
-- SEED DATA: LIVE FLOOR ISSUES (DOMAIN 7 / PM-02)
-- ----------------------------------------------------------------------------
INSERT INTO floor_issues (id, tenant_id, issue_type, severity, plant_id, line_id, station_id, operator_id, title, started_at, owner_id, status)
VALUES 
('ISS-1401', 'TENANT_DEFAULT', 'workforce', 'attention', 'Noble Garments', 'Line 14', 'ST-03', 'W-03', 'Sunita Sharma absent; station hem bottlenecking pace', CURRENT_TIMESTAMP - INTERVAL '2 hours', 'W-01', 'open'),
('ISS-0201', 'TENANT_DEFAULT', 'maintenance', 'critical', 'Noble Garments', 'Line 02', 'ST-08', NULL, 'Needle timing slippage on Kansai flatlock', CURRENT_TIMESTAMP - INTERVAL '45 minutes', 'W-05', 'open')
ON CONFLICT (id) DO UPDATE SET 
    title = EXCLUDED.title,
    severity = EXCLUDED.severity,
    status = EXCLUDED.status;