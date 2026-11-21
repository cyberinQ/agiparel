-- Author: Sunil Jain, CyberinQ LLC
-- Agiparel v51: 4-Tier PO -> Style -> Tech Pack & Operation Bulletin Hierarchy,
-- Cross-Chapter PO Context, Supplier Roll Sheet Master, and PO-Independent Reusable RFID Tag Pool.

BEGIN;

-- 1. Tier 1 & 2: Anchor Purchase Orders & Styles (1 PO -> N Styles)
CREATE TABLE IF NOT EXISTS public.document_po_dossiers_v51 (
    id uuid PRIMARY KEY DEFAULT public.uuid_generate_v4(),
    tenant_id varchar(64) NOT NULL,
    po_number varchar(64) NOT NULL,
    style_code varchar(64) NOT NULL DEFAULT 'DEFAULT_STYLE',
    buyer_name varchar(128) NOT NULL DEFAULT '',
    anchor_document_id varchar(36),
    locked_machines jsonb NOT NULL DEFAULT '[]'::jsonb,
    locked_workers jsonb NOT NULL DEFAULT '[]'::jsonb,
    sub_assemblies jsonb NOT NULL DEFAULT '[]'::jsonb,
    rfid_stage_budget jsonb NOT NULL DEFAULT '{}'::jsonb,
    lock_days integer NOT NULL DEFAULT 0 CHECK (lock_days >= 0),
    notes text NOT NULL DEFAULT '',
    created_by varchar(64) NOT NULL DEFAULT 'dev',
    created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    updated_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    UNIQUE (tenant_id, po_number, style_code),
    UNIQUE (tenant_id, id),
    FOREIGN KEY (tenant_id, anchor_document_id) REFERENCES public.document_sources_v65(tenant_id, id) ON DELETE SET NULL
);

-- 2. Tier 3: Scoped Documents per (PO, Style) — Tech Packs, Bulletins, Roll Sheets, Packing
CREATE TABLE IF NOT EXISTS public.document_po_dossier_links_v51 (
    id uuid PRIMARY KEY DEFAULT public.uuid_generate_v4(),
    tenant_id varchar(64) NOT NULL,
    dossier_id uuid NOT NULL,
    document_id varchar(36) NOT NULL,
    doc_role varchar(40) NOT NULL CHECK (doc_role IN (
        'ANCHOR_PO',
        'TECH_PACK',
        'OPERATION_BULLETIN',
        'MACHINE_ROSTER',
        'WORKER_SKILL_MATRIX',
        'MATERIAL_ROLL_SHEET',
        'PACKING_SHIPPING_BULLETIN',
        'SUB_ASSEMBLY_SPEC'
    )),
    sub_assembly varchar(128) NOT NULL DEFAULT '',
    linked_by varchar(64) NOT NULL DEFAULT 'dev',
    linked_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    UNIQUE (tenant_id, dossier_id, document_id, doc_role),
    FOREIGN KEY (tenant_id, dossier_id) REFERENCES public.document_po_dossiers_v51(tenant_id, id) ON DELETE CASCADE,
    FOREIGN KEY (tenant_id, document_id) REFERENCES public.document_sources_v65(tenant_id, id) ON DELETE CASCADE
);

-- 3. Tier 4a: PO-Independent Supplier Roll Sheet & Store Inventory Master
-- Can be received into general stock (po_number = 'GENERAL_STOCK' or future PO) and later allocated to a specific PO/Style/Bulletin.
CREATE TABLE IF NOT EXISTS public.store_supplier_roll_sheets_v51 (
    id uuid PRIMARY KEY DEFAULT public.uuid_generate_v4(),
    tenant_id varchar(64) NOT NULL,
    plant_id varchar(64) NOT NULL DEFAULT 'PLANT_DEFAULT',
    po_number varchar(64) NOT NULL DEFAULT 'GENERAL_STOCK',
    allocated_po_number varchar(64) NOT NULL DEFAULT '',
    allocated_style_code varchar(64) NOT NULL DEFAULT '',
    allocated_bulletin_id varchar(64) NOT NULL DEFAULT '',
    serial_no varchar(64) NOT NULL,
    supplier_code varchar(64) NOT NULL,
    material_sku varchar(128) NOT NULL,
    fabric_description varchar(255) NOT NULL DEFAULT '',
    content varchar(255) NOT NULL DEFAULT '',
    gsm integer NOT NULL DEFAULT 0 CHECK (gsm >= 0 AND gsm <= 2000),
    construction varchar(128) NOT NULL DEFAULT '',
    color_code varchar(64) NOT NULL,
    declared_width_mm integer NOT NULL CHECK (declared_width_mm BETWEEN 100 AND 4000),
    declared_length_mm bigint NOT NULL CHECK (declared_length_mm > 0),
    shade_band varchar(32) NOT NULL,
    actual_width_mm integer,
    actual_length_mm bigint,
    defect_points integer NOT NULL DEFAULT 0 CHECK (defect_points >= 0),
    inspection_status varchar(16) NOT NULL DEFAULT 'PENDING' CHECK (inspection_status IN ('PENDING', 'PASS', 'HOLD', 'CLAIM')),
    mapped_epc_hex varchar(96),
    grn_number varchar(64),
    grn_posted_at timestamptz,
    zone_location varchar(64) NOT NULL DEFAULT 'RECEIVING_DOCK',
    balance_length_mm bigint NOT NULL CHECK (balance_length_mm >= 0),
    source_document_id varchar(36),
    recorded_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    UNIQUE (tenant_id, serial_no)
);

-- 4. Tier 4b: PO-Independent Reusable RFID Tag Pool (Mappable to Store Rolls first, then to PO/Style/Bulletin/Stage)
CREATE TABLE IF NOT EXISTS public.rfid_live_tag_pool_v51 (
    tenant_id varchar(64) NOT NULL,
    epc_hex varchar(96) NOT NULL CHECK (epc_hex ~ '^[0-9A-F]{24,96}$' AND length(epc_hex) % 2 = 0),
    nfc_uid varchar(64) NOT NULL DEFAULT '',
    human_code varchar(64) NOT NULL,
    tag_class varchar(24) NOT NULL DEFAULT 'DUAL_UHF_NFC' CHECK (tag_class IN ('LIVE_RTLS', 'PASSIVE_UHF', 'HF_NFC', 'DUAL_UHF_NFC', 'SGTIN96_RETAIL', 'SSCC96_CARTON')),
    status varchar(24) NOT NULL DEFAULT 'UNUSED' CHECK (status IN ('UNUSED', 'MAPPED', 'RELEASED', 'FAULTY_LOW_BATTERY')),
    battery_mv integer NOT NULL DEFAULT 3000 CHECK (battery_mv BETWEEN 0 AND 5000),
    target_type varchar(24) CHECK (target_type IS NULL OR target_type IN ('ROLL', 'BUNDLE', 'CUT_PART', 'GARMENT', 'CARTON', 'TROLLEY', 'MACHINE', 'FORKLIFT', 'SAMPLE_PIECE', 'OPERATOR')),
    target_id varchar(128),
    po_number varchar(64) NOT NULL DEFAULT '',
    style_code varchar(64) NOT NULL DEFAULT '',
    bulletin_code varchar(64) NOT NULL DEFAULT '',
    op_stage varchar(64) NOT NULL DEFAULT '',
    current_zone varchar(64) NOT NULL DEFAULT 'TAG_STOCK_BIN',
    current_station varchar(64) NOT NULL DEFAULT '',
    reuse_count integer NOT NULL DEFAULT 0 CHECK (reuse_count >= 0),
    last_seen_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    updated_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    PRIMARY KEY (tenant_id, epc_hex),
    UNIQUE (tenant_id, human_code)
);

CREATE TABLE IF NOT EXISTS public.rfid_live_tag_events_v51 (
    id uuid PRIMARY KEY DEFAULT public.uuid_generate_v4(),
    tenant_id varchar(64) NOT NULL,
    plant_id varchar(64) NOT NULL DEFAULT 'PLANT_DEFAULT',
    epc_hex varchar(96) NOT NULL,
    action varchar(24) NOT NULL CHECK (action IN ('ENROLL', 'MAP', 'ALLOCATE_PO', 'SCAN_SIGHTING', 'STAGE_ARRIVE', 'STAGE_DEPART', 'RELEASE', 'RETURN_TO_UNUSED', 'MARK_FAULTY')),
    stage_no integer NOT NULL DEFAULT 0 CHECK (stage_no BETWEEN 0 AND 20),
    stage_name varchar(64) NOT NULL DEFAULT '',
    target_type varchar(24) NOT NULL DEFAULT '',
    target_id varchar(128) NOT NULL DEFAULT '',
    po_number varchar(64) NOT NULL DEFAULT '',
    style_code varchar(64) NOT NULL DEFAULT '',
    bulletin_code varchar(64) NOT NULL DEFAULT '',
    gateway_id varchar(64) NOT NULL DEFAULT '',
    station_id varchar(64) NOT NULL DEFAULT '',
    zone_code varchar(64) NOT NULL DEFAULT '',
    rssi_dbm integer,
    actor_subject varchar(128) NOT NULL DEFAULT 'system',
    detail jsonb NOT NULL DEFAULT '{}'::jsonb,
    recorded_at timestamptz NOT NULL DEFAULT clock_timestamp()
);

CREATE INDEX IF NOT EXISTS idx_rfid_tag_events_v51_lookup
    ON public.rfid_live_tag_events_v51 (tenant_id, po_number, style_code, recorded_at DESC);

CREATE TABLE IF NOT EXISTS public.rfid_rtls_sightings_v51 (
    id uuid PRIMARY KEY DEFAULT public.uuid_generate_v4(),
    tenant_id varchar(64) NOT NULL,
    plant_id varchar(64) NOT NULL DEFAULT 'PLANT_DEFAULT',
    gateway_id varchar(64) NOT NULL,
    reader_mode varchar(16) NOT NULL DEFAULT 'UHF_REMOTE' CHECK (reader_mode IN ('UHF_REMOTE', 'NFC_NEAR', 'LIVE_RTLS', 'BARCODE_2D')),
    epc_hex varchar(96) NOT NULL,
    zone_code varchar(64) NOT NULL,
    rack_bin varchar(64) NOT NULL DEFAULT '',
    station_id varchar(64) NOT NULL DEFAULT '',
    rssi_dbm integer NOT NULL DEFAULT -55,
    battery_mv integer,
    observed_at timestamptz NOT NULL DEFAULT clock_timestamp()
);

DO $$
DECLARE tbl text;
BEGIN
    FOREACH tbl IN ARRAY ARRAY[
        'document_po_dossiers_v51',
        'document_po_dossier_links_v51',
        'store_supplier_roll_sheets_v51',
        'rfid_live_tag_pool_v51',
        'rfid_live_tag_events_v51',
        'rfid_rtls_sightings_v51'
    ] LOOP
        EXECUTE format('ALTER TABLE public.%I ENABLE ROW LEVEL SECURITY', tbl);
        EXECUTE format('ALTER TABLE public.%I FORCE ROW LEVEL SECURITY', tbl);
        EXECUTE format('DROP POLICY IF EXISTS rls_%I ON public.%I', tbl, tbl);
        EXECUTE format(
            'CREATE POLICY rls_%I ON public.%I FOR ALL TO agiparel_app USING (tenant_id = NULLIF(current_setting(''app.current_tenant_id'', true), '''')) WITH CHECK (tenant_id = NULLIF(current_setting(''app.current_tenant_id'', true), ''''))',
            tbl, tbl
        );
        EXECUTE format('GRANT SELECT, INSERT, UPDATE, DELETE ON public.%I TO agiparel_app', tbl);
    END LOOP;
END $$;

COMMIT;