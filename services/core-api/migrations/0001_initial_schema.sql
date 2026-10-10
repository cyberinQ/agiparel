-- 0001_initial_schema.sql
-- Agiparel OS Master Relational Schema (PostgreSQL 16)
-- Author: Sunil Jain, CyberinQ LLC

CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- 1. TENANT ENTITY REGISTRY
CREATE TABLE tenants (
    id VARCHAR(64) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- 2. CANONICAL OPERATION MASTER (~1,200 operations)
CREATE TABLE operation_master (
    op_id VARCHAR(32) PRIMARY KEY,
    op_name VARCHAR(128) NOT NULL,
    section VARCHAR(64) NOT NULL, -- Collar, Sleeve, Front, Back, Assembly, Finishing
    default_machine_type VARCHAR(32) NOT NULL,
    default_skill VARCHAR(16) NOT NULL CHECK (default_skill IN ('Trainee', 'Skilled', 'Expert')),
    attachment_type VARCHAR(32) NOT NULL CHECK (attachment_type IN ('Single-to-Single', 'Bundle-to-Bundle', 'Bundle-to-Single'))
);

-- 3. CANONICAL MACHINE MASTER (23 types)
CREATE TABLE machine_master (
    machine_type VARCHAR(32) PRIMARY KEY,
    description VARCHAR(128) NOT NULL,
    requires_relay_interlock BOOLEAN NOT NULL DEFAULT FALSE
);

-- 4. FABRIC STORE ROLLS WITH LIVE RFID & METROLOGY
CREATE TABLE store_rolls (
    id VARCHAR(64) PRIMARY KEY,
    tenant_id VARCHAR(64) NOT NULL REFERENCES tenants(id),
    factory_sku VARCHAR(128) NOT NULL,
    original_length_m NUMERIC(10,2) NOT NULL CHECK (original_length_m > 0),
    remaining_length_m NUMERIC(10,2) NOT NULL CHECK (remaining_length_m >= 0),
    usable_width_cm NUMERIC(6,2) NOT NULL CHECK (usable_width_cm > 0),
    shade_band VARCHAR(32) NOT NULL,
    defect_map_json JSONB NOT NULL DEFAULT '[]'::jsonb,
    bin_location VARCHAR(64) NOT NULL,
    status VARCHAR(32) NOT NULL CHECK (status IN ('AVAILABLE', 'RESERVED', 'ISSUED', 'DEPLETED')),
    reserved_order_id VARCHAR(64),
    reserved_cut_no INT,
    last_rfid_sync TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_store_rolls_tenant_sku ON store_rolls(tenant_id, factory_sku);
CREATE INDEX idx_store_rolls_reservation ON store_rolls(tenant_id, reserved_order_id, reserved_cut_no);

-- 5. DOUBLE-WITNESS STORE ISSUANCE TRANSACTIONS
CREATE TABLE store_issuances (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id VARCHAR(64) NOT NULL REFERENCES tenants(id),
    roll_id VARCHAR(64) NOT NULL REFERENCES store_rolls(id),
    order_id VARCHAR(64) NOT NULL,
    cut_no INT NOT NULL,
    issued_length_m NUMERIC(10,2) NOT NULL CHECK (issued_length_m > 0),
    issuer_digital_id VARCHAR(64) NOT NULL,
    recipient_digital_id VARCHAR(64) NOT NULL,
    issued_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    digital_signature_hash VARCHAR(256) NOT NULL
);
CREATE INDEX idx_store_issuances_order ON store_issuances(tenant_id, order_id, cut_no);

-- 6. ROLL-TO-LAY CONSUMPTION LINEAGE & END-BIT ROUTING
CREATE TABLE cut_lay_rolls (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id VARCHAR(64) NOT NULL REFERENCES tenants(id),
    order_id VARCHAR(64) NOT NULL,
    cut_no INT NOT NULL,
    roll_id VARCHAR(64) NOT NULL REFERENCES store_rolls(id),
    lay_start INT NOT NULL CHECK (lay_start > 0),
    lay_end INT NOT NULL CHECK (lay_end >= lay_start),
    plies_count INT NOT NULL CHECK (plies_count > 0 AND plies_count <= 210),
    end_bit_length_m NUMERIC(10,2) NOT NULL CHECK (end_bit_length_m >= 0),
    end_bit_action VARCHAR(32) NOT NULL CHECK (end_bit_action IN ('SCRAP', 'RECUT_HOLD', 'STORE_RETURN')),
    recorded_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_cut_lay_rolls_order ON cut_lay_rolls(tenant_id, order_id, cut_no);

-- 7. POST-CUT 16-PART SUB-SUB BUNDLE REGISTRY
CREATE TABLE bundle_registry (
    id VARCHAR(64) PRIMARY KEY, -- Form: PO-Cut-Sequential (e.g., MK6574J9-C1-047)
    tenant_id VARCHAR(64) NOT NULL REFERENCES tenants(id),
    order_id VARCHAR(64) NOT NULL,
    cut_no INT NOT NULL,
    sequential_bundle_no INT NOT NULL CHECK (sequential_bundle_no > 0),
    pant_position INT NOT NULL CHECK (pant_position >= 1 AND pant_position <= 8),
    sub_bundle_id INT NOT NULL CHECK (sub_bundle_id >= 1 AND sub_bundle_id <= 18),
    ssb_ply_group INT NOT NULL CHECK (ssb_ply_group >= 1 AND ssb_ply_group <= 5),
    part_description VARCHAR(128) NOT NULL,
    ply_start INT NOT NULL CHECK (ply_start > 0),
    ply_end INT NOT NULL CHECK (ply_end >= ply_start),
    piece_count INT NOT NULL CHECK (piece_count >= 13 AND piece_count <= 25),
    garment_serial_start INT NOT NULL CHECK (garment_serial_start > 0),
    garment_serial_end INT NOT NULL CHECK (garment_serial_end >= garment_serial_start),
    active_nfc_uid VARCHAR(128) UNIQUE,
    mapped_by_helper_id VARCHAR(64),
    mapped_at TIMESTAMPTZ,
    status VARCHAR(32) NOT NULL CHECK (status IN ('PENDING_MAP', 'IN_SEWING', 'COMPLETED', 'SCRAPPED'))
);
CREATE INDEX idx_bundle_registry_tenant_nfc ON bundle_registry(tenant_id, active_nfc_uid);
CREATE INDEX idx_bundle_registry_order_seq ON bundle_registry(tenant_id, order_id, sequential_bundle_no);

-- 8. CARTON PACKING RATIO & SOLID MANIFESTS
CREATE TABLE carton_pack_manifests (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id VARCHAR(64) NOT NULL REFERENCES tenants(id),
    order_id VARCHAR(64) NOT NULL,
    box_range_start INT NOT NULL CHECK (box_range_start > 0),
    box_range_end INT NOT NULL CHECK (box_range_end >= box_range_start),
    pack_type VARCHAR(32) NOT NULL CHECK (pack_type IN ('SOLID', 'RATIO')),
    slot_rules_json JSONB NOT NULL,
    expected_gross_weight_kg NUMERIC(6,3) NOT NULL CHECK (expected_gross_weight_kg > 0),
    weight_tolerance_kg NUMERIC(4,3) NOT NULL DEFAULT 0.250,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_carton_manifest_order ON carton_pack_manifests(tenant_id, order_id);

-- 9. HANGTAG HELPER ATOMIC BINDING
CREATE TABLE hangtag_binds (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id VARCHAR(64) NOT NULL REFERENCES tenants(id),
    bundle_id VARCHAR(64) NOT NULL REFERENCES bundle_registry(id),
    nfc_uid VARCHAR(128) NOT NULL,
    buyer_hangtag_barcode VARCHAR(128) NOT NULL,
    order_id VARCHAR(64) NOT NULL,
    style_id VARCHAR(64) NOT NULL,
    helper_id VARCHAR(64) NOT NULL,
    bound_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_tenant_hangtag UNIQUE (tenant_id, buyer_hangtag_barcode)
);
CREATE INDEX idx_hangtag_binds_order ON hangtag_binds(tenant_id, order_id);

-- 10. FIRST-CLASS FLOOR ISSUES OBJECT
CREATE TABLE floor_issues (
    id VARCHAR(64) PRIMARY KEY,
    tenant_id VARCHAR(64) NOT NULL REFERENCES tenants(id),
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
CREATE INDEX idx_floor_issues_active ON floor_issues(tenant_id, plant_id, status);

-- 11. ORDER CLOSURE REPORT (OCR) RECONCILIATION LEDGER
CREATE TABLE ocr_records (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id VARCHAR(64) NOT NULL REFERENCES tenants(id),
    order_id VARCHAR(64) NOT NULL,
    stage VARCHAR(32) NOT NULL CHECK (stage IN ('CUT', 'SEWN', 'FINISHED', 'PACKED', 'LOADED')),
    forward_qty INT NOT NULL CHECK (forward_qty >= 0),
    lost_qty INT NOT NULL CHECK (lost_qty >= 0),
    left_over_qty INT NOT NULL CHECK (left_over_qty >= 0),
    supervisor_id VARCHAR(64) NOT NULL,
    variance_comment TEXT,
    signed_at TIMESTAMPTZ
);
CREATE INDEX idx_ocr_records_order ON ocr_records(tenant_id, order_id);
