-- Author: Sunil Jain, CyberinQ LLC (Generated 2026-11-24)
-- Migration 0008:
--   1. Costing P1/P2 Extensions:
--      - FR-9: Cut, Bundle & RFID Tag Preview
--      - FR-10 & V11: Carton Plan Preview (enforcing cartons * pcs_per_carton = order_qty per colour x size)
--      - FR-11: Scenario Comparison (up to 3 scenarios per costing sheet, one marked is_quoted)
--      - FR-14: Sampling TNA Preview (proto, fit, size set, PP, TOP back-scheduled from plan cut date)
--      - FR-15: Costed vs Actual Variance at OCR
--   2. Shop-Floor PRD §4.13 & §7.2:
--      - 6-Stage Daily Plan & Actuals (Raw Material Arrival, Cutting, Sewing, Finishing, Packing, Shipment)
--      - Fixed-list Shortfall Reasons with Manager Note
--      - Idempotent 10-Minute Factory-to-Cloud Outbox & Sync Watermark Ledger

BEGIN;

CREATE TABLE IF NOT EXISTS public.costing_cut_previews_v53 (
    id uuid PRIMARY KEY DEFAULT public.uuid_generate_v4(),
    tenant_id varchar(64) NOT NULL,
    version_id uuid NOT NULL,
    allowance_pct numeric(8,4) NOT NULL DEFAULT 0.0200,
    min_extra_pcs int NOT NULL DEFAULT 1,
    max_extra_pcs int NOT NULL DEFAULT 15,
    max_plies int NOT NULL DEFAULT 210,
    bundle_size int NOT NULL DEFAULT 20,
    cut_parts_per_garment int NOT NULL DEFAULT 16,
    total_order_qty int NOT NULL,
    total_cut_qty int NOT NULL,
    estimated_cuts int NOT NULL,
    estimated_bundles int NOT NULL,
    garment_rfid_tags int NOT NULL,
    bundle_rfid_tags int NOT NULL,
    total_rfid_tags int NOT NULL,
    warnings jsonb NOT NULL DEFAULT '[]'::jsonb,
    breakdown jsonb NOT NULL DEFAULT '[]'::jsonb,
    computed_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    UNIQUE (tenant_id, version_id)
);

CREATE TABLE IF NOT EXISTS public.costing_carton_plans_v53 (
    id uuid PRIMARY KEY DEFAULT public.uuid_generate_v4(),
    tenant_id varchar(64) NOT NULL,
    version_id uuid NOT NULL,
    assortment_mode varchar(24) NOT NULL CHECK (assortment_mode IN ('SOLID','ASSORTED','MIXED')),
    total_order_qty int NOT NULL,
    packed_qty int NOT NULL,
    declared_cartons int NOT NULL DEFAULT 0,
    computed_cartons int NOT NULL,
    total_cbm numeric(14,4) NOT NULL,
    total_gross_weight_kg numeric(14,4) NOT NULL,
    total_carton_cost numeric(18,4) NOT NULL,
    is_complete boolean NOT NULL DEFAULT false,
    gaps jsonb NOT NULL DEFAULT '[]'::jsonb,
    lines jsonb NOT NULL DEFAULT '[]'::jsonb,
    computed_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    UNIQUE (tenant_id, version_id)
);

CREATE TABLE IF NOT EXISTS public.costing_sampling_tna_v53 (
    id uuid PRIMARY KEY DEFAULT public.uuid_generate_v4(),
    tenant_id varchar(64) NOT NULL,
    version_id uuid NOT NULL,
    order_date date NOT NULL,
    plan_cut_date date NOT NULL,
    milestones jsonb NOT NULL DEFAULT '[]'::jsonb,
    has_past_or_pre_order_flag boolean NOT NULL DEFAULT false,
    computed_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    UNIQUE (tenant_id, version_id)
);

CREATE TABLE IF NOT EXISTS public.costing_ocr_actuals_v53 (
    id uuid PRIMARY KEY DEFAULT public.uuid_generate_v4(),
    tenant_id varchar(64) NOT NULL,
    po_id varchar(64) NOT NULL,
    baseline_id uuid NOT NULL,
    costed_qty int NOT NULL,
    actual_shipped_qty int NOT NULL,
    costed_sam numeric(14,4) NOT NULL,
    actual_avg_sam numeric(14,4) NOT NULL,
    costed_cm_total numeric(18,4) NOT NULL,
    actual_cm_total numeric(18,4) NOT NULL,
    costed_material_total numeric(18,4) NOT NULL,
    actual_material_total numeric(18,4) NOT NULL,
    quoted_ex_factory date NOT NULL,
    actual_ex_factory date NOT NULL,
    cm_variance_pct numeric(10,4) NOT NULL,
    material_variance_pct numeric(10,4) NOT NULL,
    schedule_variance_days int NOT NULL,
    shipped_on_time boolean NOT NULL,
    recorded_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    UNIQUE (tenant_id, po_id)
);

CREATE TABLE IF NOT EXISTS public.factory_stage_daily_plan_v53 (
    id uuid PRIMARY KEY DEFAULT public.uuid_generate_v4(),
    tenant_id varchar(64) NOT NULL,
    factory_id varchar(64) NOT NULL,
    factory_name varchar(128) NOT NULL,
    po_id varchar(64) NOT NULL,
    buyer_id varchar(64) NOT NULL,
    style_id varchar(64) NOT NULL,
    plan_date date NOT NULL,
    stage varchar(32) NOT NULL CHECK (stage IN (
        'RAW_MATERIAL_ARRIVAL',
        'CUTTING',
        'SEWING',
        'FINISHING',
        'PACKING',
        'SHIPMENT'
    )),
    planned_qty int NOT NULL CHECK (planned_qty >= 0),
    actual_qty int NOT NULL DEFAULT 0 CHECK (actual_qty >= 0),
    shortfall_reason_code varchar(40) NOT NULL DEFAULT '' CHECK (shortfall_reason_code IN (
        '',
        'MATERIAL_LATE',
        'MACHINE_BREAKDOWN',
        'OPERATOR_ABSENTEEISM',
        'QUALITY_HOLD_REWORK',
        'OUTSIDE_VENDOR_DELAY',
        'POWER_OR_UTILITY_OUTAGE',
        'BUYER_CHANGE_HOLD'
    )),
    shortfall_note text NOT NULL DEFAULT '',
    shortfall_recorded_by varchar(64) NOT NULL DEFAULT '',
    updated_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    UNIQUE (tenant_id, factory_id, po_id, plan_date, stage)
);

CREATE TABLE IF NOT EXISTS public.cloud_sync_outbox_v53 (
    event_id uuid PRIMARY KEY,
    tenant_id varchar(64) NOT NULL,
    factory_id varchar(64) NOT NULL,
    stream_name varchar(48) NOT NULL,
    aggregate_id varchar(128) NOT NULL,
    payload jsonb NOT NULL,
    occurred_at timestamptz NOT NULL,
    enqueued_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    synced_at timestamptz
);

CREATE TABLE IF NOT EXISTS public.cloud_sync_factory_status_v53 (
    tenant_id varchar(64) NOT NULL,
    factory_id varchar(64) NOT NULL,
    factory_name varchar(128) NOT NULL,
    last_sync_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    pending_events int NOT NULL DEFAULT 0,
    last_batch_count int NOT NULL DEFAULT 0,
    sync_status varchar(24) NOT NULL DEFAULT 'HEALTHY' CHECK (sync_status IN ('HEALTHY','DELAYED','OFFLINE_QUEUED')),
    PRIMARY KEY (tenant_id, factory_id)
);

DO $$
DECLARE
    tbl text;
BEGIN
    FOR tbl IN SELECT unnest(ARRAY[
        'costing_cut_previews_v53',
        'costing_carton_plans_v53',
        'costing_sampling_tna_v53',
        'costing_ocr_actuals_v53',
        'factory_stage_daily_plan_v53',
        'cloud_sync_outbox_v53',
        'cloud_sync_factory_status_v53'
    ]) LOOP
        EXECUTE format('ALTER TABLE public.%I ENABLE ROW LEVEL SECURITY;', tbl);
        EXECUTE format('ALTER TABLE public.%I FORCE ROW LEVEL SECURITY;', tbl);
        EXECUTE format('DROP POLICY IF EXISTS tenant_iso_%I ON public.%I;', tbl, tbl);
        EXECUTE format(
            'CREATE POLICY tenant_iso_%I ON public.%I USING (tenant_id = current_setting(''app.current_tenant_id'', true)) WITH CHECK (tenant_id = current_setting(''app.current_tenant_id'', true));',
            tbl, tbl
        );
    END LOOP;
END $$;

COMMIT;
