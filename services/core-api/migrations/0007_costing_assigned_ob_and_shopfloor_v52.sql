-- Author: Sunil Jain, CyberinQ LLC (Generated 2026-11-24)
-- Migration 0007:
--   1. Costing with Planning Preview (Costing PRD v0.5, FR-1..FR-15, D1..D15, V1..V14)
--      with CPM = total_factory_expense / total_paid_work_minutes (D3/D3a locked: paid work minutes)
--   2. Assigned OB Lifecycle & Last-Piece Automatic Release (Costing PRD §4.3, D13-D15)
--   3. Shop-Floor Tracking extensions: Checker Stage Back-Credit, Lost-Time Stoppages,
--      Operator Skill Matrix & DHU Attribution, Outside-Process Challans & Billing Check,
--      Finishing Hangtag Replacement, and Reusable Roll RFID Tag Interval History (Shop-Floor PRD §3-§8)

BEGIN;

-- ============================================================================
-- PART 1: COSTING WITH PLANNING PREVIEW (Costing PRD v0.5 §10.1)
-- ============================================================================

CREATE TABLE IF NOT EXISTS public.cost_rates_v52 (
    id uuid PRIMARY KEY DEFAULT public.uuid_generate_v4(),
    tenant_id varchar(64) NOT NULL,
    unit_id varchar(64) NOT NULL,
    period_label varchar(32) NOT NULL, -- e.g. '2026-10' (last full closed month)
    total_factory_expense numeric(18,4) NOT NULL CHECK (total_factory_expense >= 0),
    total_paid_work_minutes numeric(18,4) NOT NULL CHECK (total_paid_work_minutes > 0),
    cpm numeric(18,6) GENERATED ALWAYS AS (total_factory_expense / total_paid_work_minutes) STORED,
    base_currency char(3) NOT NULL DEFAULT 'USD',
    default_overhead_pct numeric(8,4) NOT NULL DEFAULT 0,
    min_margin_floor_pct numeric(8,4) NOT NULL DEFAULT 8.0000,
    active boolean NOT NULL DEFAULT true,
    created_by varchar(64) NOT NULL,
    created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    UNIQUE (tenant_id, unit_id, period_label)
);

CREATE TABLE IF NOT EXISTS public.costing_sheets_v52 (
    id uuid PRIMARY KEY DEFAULT public.uuid_generate_v4(),
    tenant_id varchar(64) NOT NULL,
    unit_id varchar(64) NOT NULL,
    buyer_id varchar(64) NOT NULL,
    style_id varchar(64) NOT NULL,
    po_ref varchar(64) NOT NULL,
    currency char(3) NOT NULL DEFAULT 'USD',
    order_date date NOT NULL CHECK (order_date <= CURRENT_DATE),
    requested_ship_date date NOT NULL CHECK (requested_ship_date > order_date AND requested_ship_date <= order_date + interval '18 months'),
    tentative_confirmation_date date NOT NULL CHECK (tentative_confirmation_date >= order_date),
    quote_valid_until date NOT NULL CHECK (quote_valid_until >= order_date),
    state varchar(24) NOT NULL DEFAULT 'DRAFT'
        CHECK (state IN ('DRAFT','PREVIEWED','SUBMITTED','REJECTED','QUOTED','CONFIRMED','LOST','EXPIRED')),
    current_version_no int NOT NULL DEFAULT 1 CHECK (current_version_no >= 1),
    created_by varchar(64) NOT NULL,
    created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    updated_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    UNIQUE (tenant_id, buyer_id, po_ref, style_id),
    UNIQUE (tenant_id, id)
);

CREATE TABLE IF NOT EXISTS public.costing_versions_v52 (
    id uuid PRIMARY KEY DEFAULT public.uuid_generate_v4(),
    tenant_id varchar(64) NOT NULL,
    sheet_id uuid NOT NULL,
    version_no int NOT NULL CHECK (version_no >= 1),
    parent_version_no int,
    frozen boolean NOT NULL DEFAULT false,
    change_reason text NOT NULL DEFAULT '',
    created_by varchar(64) NOT NULL,
    created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    UNIQUE (tenant_id, sheet_id, version_no),
    UNIQUE (tenant_id, id),
    FOREIGN KEY (tenant_id, sheet_id) REFERENCES public.costing_sheets_v52(tenant_id, id) ON DELETE RESTRICT
);

-- V9: Submitted/frozen costing versions are strictly immutable
CREATE OR REPLACE FUNCTION public.guard_costing_version_immutable_v52() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF OLD.frozen THEN
        IF TG_OP = 'DELETE' OR NEW.frozen IS DISTINCT FROM OLD.frozen
           OR NEW.change_reason IS DISTINCT FROM OLD.change_reason
           OR NEW.version_no IS DISTINCT FROM OLD.version_no THEN
            RAISE EXCEPTION 'ERR_V9_COSTING_VERSION_IMMUTABLE';
        END IF;
    END IF;
    RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS trg_costing_version_immutable_v52 ON public.costing_versions_v52;
CREATE TRIGGER trg_costing_version_immutable_v52
    BEFORE UPDATE OR DELETE ON public.costing_versions_v52
    FOR EACH ROW EXECUTE FUNCTION public.guard_costing_version_immutable_v52();

CREATE TABLE IF NOT EXISTS public.costing_order_matrix_lines_v52 (
    id uuid PRIMARY KEY DEFAULT public.uuid_generate_v4(),
    tenant_id varchar(64) NOT NULL,
    buyer_id varchar(64) NOT NULL,
    sheet_id uuid NOT NULL,
    version_id uuid NOT NULL,
    sheet_active boolean NOT NULL DEFAULT true,
    colour varchar(64) NOT NULL,
    size varchar(32) NOT NULL,
    upc varchar(64) NOT NULL,
    qty int NOT NULL CHECK (qty > 0),
    created_by varchar(64) NOT NULL,
    created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    UNIQUE (tenant_id, version_id, colour, size),
    UNIQUE (tenant_id, version_id, upc),
    FOREIGN KEY (tenant_id, version_id) REFERENCES public.costing_versions_v52(tenant_id, id) ON DELETE RESTRICT
);

-- V3: UPC unique across all open costing sheets/POs of the same buyer
CREATE UNIQUE INDEX IF NOT EXISTS idx_costing_open_buyer_upc_v52
    ON public.costing_order_matrix_lines_v52 (tenant_id, buyer_id, upc)
    WHERE sheet_active = true;

CREATE TABLE IF NOT EXISTS public.costing_bom_lines_v52 (
    id uuid PRIMARY KEY DEFAULT public.uuid_generate_v4(),
    tenant_id varchar(64) NOT NULL,
    version_id uuid NOT NULL,
    line_no int NOT NULL CHECK (line_no >= 1),
    item_id varchar(64) NOT NULL,
    category varchar(24) NOT NULL CHECK (category IN ('FABRIC','TRIM','PACKING','OTHER')),
    colour varchar(64),
    supplier_id varchar(64) NOT NULL,
    consumption numeric(18,6) NOT NULL CHECK (consumption > 0),
    uom varchar(16) NOT NULL CHECK (uom IN ('YD','M','PCS','CONE','KG','SET')),
    purchase_unit_multiple numeric(18,6) NOT NULL DEFAULT 1 CHECK (purchase_unit_multiple > 0),
    wastage_pct numeric(8,4) NOT NULL CHECK (wastage_pct >= 0 AND wastage_pct <= 100),
    unit_price numeric(18,6) NOT NULL CHECK (unit_price >= 0),
    price_currency char(3) NOT NULL DEFAULT 'USD',
    gross_qty numeric(18,4) NOT NULL,
    stock_snapshot_qty numeric(18,4) NOT NULL DEFAULT 0,
    stock_snapshot_at timestamptz,
    net_qty numeric(18,4) NOT NULL,
    line_cost_per_pc numeric(18,6) NOT NULL,
    created_by varchar(64) NOT NULL,
    created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    UNIQUE (tenant_id, version_id, line_no),
    FOREIGN KEY (tenant_id, version_id) REFERENCES public.costing_versions_v52(tenant_id, id) ON DELETE RESTRICT
);

CREATE TABLE IF NOT EXISTS public.costing_ob_pins_v52 (
    id uuid PRIMARY KEY DEFAULT public.uuid_generate_v4(),
    tenant_id varchar(64) NOT NULL,
    version_id uuid NOT NULL,
    ob_id varchar(64) NOT NULL,
    ob_version int NOT NULL CHECK (ob_version >= 1),
    ob_state varchar(24) NOT NULL CHECK (ob_state = 'RELEASED'),
    total_sam numeric(18,4) NOT NULL CHECK (total_sam > 0),
    cpm_used numeric(18,6) NOT NULL CHECK (cpm_used > 0),
    cpm_period varchar(32) NOT NULL,
    cm_per_pc numeric(18,6) GENERATED ALWAYS AS (total_sam * cpm_used) STORED,
    created_by varchar(64) NOT NULL,
    created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    UNIQUE (tenant_id, version_id),
    FOREIGN KEY (tenant_id, version_id) REFERENCES public.costing_versions_v52(tenant_id, id) ON DELETE RESTRICT
);

CREATE TABLE IF NOT EXISTS public.costing_cost_lines_v52 (
    id uuid PRIMARY KEY DEFAULT public.uuid_generate_v4(),
    tenant_id varchar(64) NOT NULL,
    version_id uuid NOT NULL,
    cost_type varchar(32) NOT NULL
        CHECK (cost_type IN ('WASH','EMBELLISHMENT','TESTING','PACKING_MATERIAL','RFID_TAGS','FREIGHT','OVERHEAD','COMMISSION','MARGIN','OTHER')),
    basis varchar(16) NOT NULL CHECK (basis IN ('RATE','MANUAL')),
    rate_ref varchar(64),
    amount_per_pc numeric(18,6) NOT NULL,
    note text NOT NULL DEFAULT '',
    created_by varchar(64) NOT NULL,
    created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    -- V8: Manual cost lines require a written note
    CONSTRAINT chk_cost_line_manual_note_v52 CHECK (basis = 'RATE' OR length(btrim(note)) >= 5),
    FOREIGN KEY (tenant_id, version_id) REFERENCES public.costing_versions_v52(tenant_id, id) ON DELETE RESTRICT
);

CREATE TABLE IF NOT EXISTS public.costing_price_summaries_v52 (
    version_id uuid PRIMARY KEY,
    tenant_id varchar(64) NOT NULL,
    total_qty int NOT NULL CHECK (total_qty > 0),
    material_pc numeric(18,6) NOT NULL,
    cm_pc numeric(18,6) NOT NULL,
    other_pc numeric(18,6) NOT NULL,
    overhead_pc numeric(18,6) NOT NULL,
    margin_pct numeric(8,4) NOT NULL,
    fob_pc numeric(18,6) NOT NULL,
    order_value numeric(18,4) NOT NULL,
    base_fob_pc numeric(18,6) NOT NULL,
    base_order_value numeric(18,4) NOT NULL,
    fx_rate_ref varchar(64) NOT NULL,
    fx_rate numeric(18,8) NOT NULL CHECK (fx_rate > 0),
    computed_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    FOREIGN KEY (tenant_id, version_id) REFERENCES public.costing_versions_v52(tenant_id, id) ON DELETE RESTRICT
);

CREATE TABLE IF NOT EXISTS public.costing_scenarios_v52 (
    id uuid PRIMARY KEY DEFAULT public.uuid_generate_v4(),
    tenant_id varchar(64) NOT NULL,
    sheet_id uuid NOT NULL,
    version_no int NOT NULL,
    scenario_slot int NOT NULL CHECK (scenario_slot BETWEEN 1 AND 3),
    name varchar(64) NOT NULL,
    parameters jsonb NOT NULL DEFAULT '{}'::jsonb,
    is_quoted boolean NOT NULL DEFAULT false,
    created_by varchar(64) NOT NULL,
    created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    UNIQUE (tenant_id, sheet_id, version_no, scenario_slot)
);

CREATE TABLE IF NOT EXISTS public.costing_preview_runs_v52 (
    id uuid PRIMARY KEY DEFAULT public.uuid_generate_v4(),
    tenant_id varchar(64) NOT NULL,
    version_id uuid NOT NULL,
    scenario_id uuid,
    run_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    input_snapshot_hash char(64) NOT NULL,
    input_snapshot jsonb NOT NULL,
    verdict varchar(12) NOT NULL CHECK (verdict IN ('GREEN','AMBER','RED')),
    reasons jsonb NOT NULL DEFAULT '[]'::jsonb,
    latest_plan_cut_date date NOT NULL,
    sewing_start_date date NOT NULL,
    sewing_end_date date NOT NULL,
    finishing_end_date date NOT NULL,
    earliest_feasible_ex_factory date NOT NULL,
    slack_days int NOT NULL,
    is_stale boolean NOT NULL DEFAULT false,
    stale_reason text NOT NULL DEFAULT '',
    UNIQUE (tenant_id, id),
    FOREIGN KEY (tenant_id, version_id) REFERENCES public.costing_versions_v52(tenant_id, id) ON DELETE RESTRICT
);

CREATE TABLE IF NOT EXISTS public.costing_material_checks_v52 (
    id uuid PRIMARY KEY DEFAULT public.uuid_generate_v4(),
    tenant_id varchar(64) NOT NULL,
    run_id uuid NOT NULL,
    bom_line_id uuid NOT NULL,
    order_by date NOT NULL,
    lead_time_days int NOT NULL,
    expected_in_house date NOT NULL,
    needed_by date NOT NULL,
    slack_days int NOT NULL,
    also_counted_by_open_costings int NOT NULL DEFAULT 0, -- C2 transparency
    verdict varchar(12) NOT NULL CHECK (verdict IN ('GREEN','AMBER','RED')),
    FOREIGN KEY (tenant_id, run_id) REFERENCES public.costing_preview_runs_v52(tenant_id, id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS public.costing_machine_checks_v52 (
    id uuid PRIMARY KEY DEFAULT public.uuid_generate_v4(),
    tenant_id varchar(64) NOT NULL,
    run_id uuid NOT NULL,
    week_start date NOT NULL,
    machine_type_id varchar(64) NOT NULL,
    required_count int NOT NULL,
    available_count int NOT NULL,
    shortage_count int NOT NULL,
    unowned_type boolean NOT NULL DEFAULT false,
    verdict varchar(12) NOT NULL CHECK (verdict IN ('GREEN','AMBER','RED')),
    FOREIGN KEY (tenant_id, run_id) REFERENCES public.costing_preview_runs_v52(tenant_id, id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS public.costing_skill_checks_v52 (
    id uuid PRIMARY KEY DEFAULT public.uuid_generate_v4(),
    tenant_id varchar(64) NOT NULL,
    run_id uuid NOT NULL,
    skill_grade varchar(32) NOT NULL,
    is_critical_op boolean NOT NULL DEFAULT false,
    required_count int NOT NULL,
    qualified_available_count int NOT NULL,
    shortage_count int NOT NULL,
    verdict varchar(12) NOT NULL CHECK (verdict IN ('GREEN','AMBER','RED')),
    FOREIGN KEY (tenant_id, run_id) REFERENCES public.costing_preview_runs_v52(tenant_id, id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS public.costing_capacity_checks_v52 (
    id uuid PRIMARY KEY DEFAULT public.uuid_generate_v4(),
    tenant_id varchar(64) NOT NULL,
    run_id uuid NOT NULL,
    unit_id varchar(64) NOT NULL,
    work_date date NOT NULL,
    available_minutes int NOT NULL,
    committed_minutes int NOT NULL,
    held_minutes int NOT NULL,
    required_minutes int NOT NULL,
    free_minutes int NOT NULL,
    verdict varchar(12) NOT NULL CHECK (verdict IN ('GREEN','AMBER','RED')),
    FOREIGN KEY (tenant_id, run_id) REFERENCES public.costing_preview_runs_v52(tenant_id, id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS public.costing_daily_ramps_v52 (
    id uuid PRIMARY KEY DEFAULT public.uuid_generate_v4(),
    tenant_id varchar(64) NOT NULL,
    version_id uuid NOT NULL,
    day_index int NOT NULL CHECK (day_index >= 1),
    planned_pcs int NOT NULL CHECK (planned_pcs > 0),
    agreed_by_supervisor varchar(64) NOT NULL,
    agreed_by_ie varchar(64) NOT NULL,
    agreed_by_manager varchar(64) NOT NULL,
    agreed_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    UNIQUE (tenant_id, version_id, day_index)
);

-- D2: Quote capacity holds until tentative_confirmation_date
CREATE TABLE IF NOT EXISTS public.costing_capacity_holds_v52 (
    id uuid PRIMARY KEY DEFAULT public.uuid_generate_v4(),
    tenant_id varchar(64) NOT NULL,
    sheet_id uuid NOT NULL,
    version_no int NOT NULL,
    unit_id varchar(64) NOT NULL,
    hold_date date NOT NULL,
    held_minutes int NOT NULL CHECK (held_minutes > 0),
    machine_type_id varchar(64) NOT NULL DEFAULT 'ALL',
    machine_count int NOT NULL DEFAULT 0,
    skill_grade varchar(32) NOT NULL DEFAULT 'ALL',
    operator_count int NOT NULL DEFAULT 0,
    hold_until date NOT NULL,
    status varchar(16) NOT NULL DEFAULT 'HELD' CHECK (status IN ('HELD','CONVERTED','RELEASED')),
    released_reason text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    updated_at timestamptz NOT NULL DEFAULT clock_timestamp()
);

-- D5: Joint re-plan record (no Amber/Red override)
CREATE TABLE IF NOT EXISTS public.costing_replan_records_v52 (
    id uuid PRIMARY KEY DEFAULT public.uuid_generate_v4(),
    tenant_id varchar(64) NOT NULL,
    sheet_id uuid,
    po_id varchar(64),
    trigger_kind varchar(24) NOT NULL CHECK (trigger_kind IN ('AMBER','RED','BOTTLENECK','TARGET_MISSED')),
    participants jsonb NOT NULL, -- must include merchandiser, planning_mgr, ie, prod_mgr
    changes jsonb NOT NULL,
    reason text NOT NULL CHECK (length(btrim(reason)) >= 20),
    recorded_at timestamptz NOT NULL DEFAULT clock_timestamp()
);

CREATE TABLE IF NOT EXISTS public.costing_approvals_v52 (
    id uuid PRIMARY KEY DEFAULT public.uuid_generate_v4(),
    tenant_id varchar(64) NOT NULL,
    version_id uuid NOT NULL,
    approval_level int NOT NULL CHECK (approval_level >= 1),
    approver_id varchar(64) NOT NULL,
    decision varchar(16) NOT NULL CHECK (decision IN ('APPROVED','REJECTED')),
    reason text NOT NULL DEFAULT '',
    device varchar(24) NOT NULL DEFAULT 'DESKTOP',
    decided_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    CONSTRAINT chk_reject_reason_v52 CHECK (decision = 'APPROVED' OR length(btrim(reason)) >= 20)
);

-- FR-13: Immutable baseline handoff to Planning, Sourcing, Cutting, Packing & OCR
CREATE TABLE IF NOT EXISTS public.planning_baselines_v52 (
    id uuid PRIMARY KEY DEFAULT public.uuid_generate_v4(),
    tenant_id varchar(64) NOT NULL,
    po_id varchar(64) NOT NULL,
    sheet_id uuid NOT NULL,
    version_id uuid NOT NULL,
    run_id uuid NOT NULL,
    payload jsonb NOT NULL,
    frozen_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    UNIQUE (tenant_id, po_id),
    UNIQUE (tenant_id, version_id)
);

-- V10: Strict append-only audit log
CREATE TABLE IF NOT EXISTS public.costing_audit_events_v52 (
    id uuid PRIMARY KEY DEFAULT public.uuid_generate_v4(),
    tenant_id varchar(64) NOT NULL,
    entity varchar(48) NOT NULL,
    entity_id varchar(64) NOT NULL,
    actor_id varchar(64) NOT NULL,
    role varchar(32) NOT NULL,
    action varchar(32) NOT NULL,
    before_state jsonb,
    after_state jsonb,
    reason text NOT NULL DEFAULT '',
    recorded_at timestamptz NOT NULL DEFAULT clock_timestamp()
);

CREATE OR REPLACE FUNCTION public.reject_audit_mutation_v52() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'ERR_V10_AUDIT_LOG_APPEND_ONLY';
END;
$$;

DROP TRIGGER IF EXISTS trg_audit_append_only_v52 ON public.costing_audit_events_v52;
CREATE TRIGGER trg_audit_append_only_v52
    BEFORE UPDATE OR DELETE ON public.costing_audit_events_v52
    FOR EACH ROW EXECUTE FUNCTION public.reject_audit_mutation_v52();

-- ============================================================================
-- PART 2: ASSIGNED OB LIFECYCLE & AUTO-RELEASE ON LAST PIECE (Costing PRD §4.3, D13-D15)
-- ============================================================================

CREATE TABLE IF NOT EXISTS public.ob_assignments_v52 (
    id uuid PRIMARY KEY DEFAULT public.uuid_generate_v4(),
    tenant_id varchar(64) NOT NULL,
    unit_id varchar(64) NOT NULL,
    po_id varchar(64) NOT NULL,
    ob_id varchar(64) NOT NULL,
    ob_version int NOT NULL,
    plan_version int NOT NULL DEFAULT 1 CHECK (plan_version >= 1),
    operation_id varchar(64) NOT NULL,
    operation_seq int NOT NULL,
    sam numeric(12,4) NOT NULL CHECK (sam > 0),
    target_qty int NOT NULL CHECK (target_qty > 0),
    completed_qty int NOT NULL DEFAULT 0 CHECK (completed_qty >= 0),
    operator_id varchar(64) NOT NULL,
    operator_name varchar(128) NOT NULL,
    machine_id varchar(64) NOT NULL,
    machine_no varchar(64) NOT NULL,
    machine_name varchar(128) NOT NULL,
    block_from date NOT NULL,
    block_to date NOT NULL,
    status varchar(16) NOT NULL DEFAULT 'ASSIGNED' CHECK (status IN ('ASSIGNED','SUPERSEDED','RELEASED')),
    released_at timestamptz,
    changed_by varchar(64) NOT NULL,
    changer_role varchar(32) NOT NULL
        CHECK (changer_role IN ('PLANNING_MANAGER','IE','FACTORY_MANAGER','SUPERVISOR','SYSTEM')),
    change_reason text NOT NULL DEFAULT '',
    changed_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    UNIQUE (tenant_id, po_id, plan_version, operation_id)
);

-- D13: Automatically release operator & machine block when last piece of operation scans out
CREATE OR REPLACE FUNCTION public.auto_release_ob_assignment_v52() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF NEW.status = 'ASSIGNED' AND NEW.completed_qty >= NEW.target_qty THEN
        NEW.status := 'RELEASED';
        NEW.released_at := clock_timestamp();
    END IF;
    RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS trg_auto_release_ob_assignment_v52 ON public.ob_assignments_v52;
CREATE TRIGGER trg_auto_release_ob_assignment_v52
    BEFORE INSERT OR UPDATE OF completed_qty ON public.ob_assignments_v52
    FOR EACH ROW EXECUTE FUNCTION public.auto_release_ob_assignment_v52();

-- ============================================================================
-- PART 3: SHOP-FLOOR TRACKING EXTENSIONS (Shop-Floor PRD §3-§8)
-- ============================================================================

-- Reusable Roll Tag Interval History (§3.7, §6.2) replacing permanent 1:1 roll lock
CREATE TABLE IF NOT EXISTS public.roll_tag_mappings_v52 (
    id uuid PRIMARY KEY DEFAULT public.uuid_generate_v4(),
    tenant_id varchar(64) NOT NULL,
    plant_id varchar(64) NOT NULL,
    epc_hex varchar(96) NOT NULL,
    tag_type varchar(16) NOT NULL CHECK (tag_type IN ('ACTIVE','NFC','UHF')),
    target_type varchar(24) NOT NULL CHECK (target_type IN ('ROLL','SUB_BUNDLE','BUNDLE','GARMENT')),
    target_id varchar(128) NOT NULL,
    mapped_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    released_at timestamptz,
    mapped_by varchar(64) NOT NULL,
    release_reason varchar(64) NOT NULL DEFAULT ''
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_active_tag_mapping_v52
    ON public.roll_tag_mappings_v52 (tenant_id, epc_hex)
    WHERE released_at IS NULL;

-- Automated Lost-Time Stoppages (§5.7)
CREATE TABLE IF NOT EXISTS public.workstation_stoppages_v52 (
    id uuid PRIMARY KEY DEFAULT public.uuid_generate_v4(),
    tenant_id varchar(64) NOT NULL,
    plant_id varchar(64) NOT NULL,
    line_id varchar(64) NOT NULL,
    station_id varchar(64) NOT NULL,
    operator_id varchar(64) NOT NULL,
    operation_id varchar(64) NOT NULL,
    category varchar(24) NOT NULL
        CHECK (category IN ('MACHINE_BREAKDOWN','NO_WIP','OPERATOR_OWN_ISSUE')),
    started_at timestamptz NOT NULL,
    ended_at timestamptz,
    supervisor_alerted_at timestamptz,
    manager_escalated_at timestamptz,
    resolved_by varchar(64) NOT NULL DEFAULT ''
);

-- Operator-Attributed Defects & Open Alterations (§5.6)
CREATE TABLE IF NOT EXISTS public.operator_defect_ledger_v52 (
    id uuid PRIMARY KEY DEFAULT public.uuid_generate_v4(),
    tenant_id varchar(64) NOT NULL,
    plant_id varchar(64) NOT NULL,
    line_id varchar(64) NOT NULL,
    po_id varchar(64) NOT NULL,
    style_id varchar(64) NOT NULL,
    garment_id varchar(128) NOT NULL,
    operation_id varchar(64) NOT NULL,
    responsible_operator_id varchar(64) NOT NULL,
    checker_id varchar(64) NOT NULL,
    defect_code varchar(64) NOT NULL,
    status varchar(16) NOT NULL DEFAULT 'OPEN' CHECK (status IN ('OPEN','CLOSED')),
    opened_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    closed_at timestamptz
);

-- Outside-Process Challans, SLA Turnaround & Vendor Bill Reconciliation (§4.9)
CREATE TABLE IF NOT EXISTS public.outside_process_challans_v52 (
    id uuid PRIMARY KEY DEFAULT public.uuid_generate_v4(),
    tenant_id varchar(64) NOT NULL,
    plant_id varchar(64) NOT NULL,
    lot_number varchar(64) NOT NULL,
    challan_barcode varchar(96) NOT NULL,
    vendor_id varchar(64) NOT NULL,
    process_code varchar(32) NOT NULL,
    po_id varchar(64) NOT NULL,
    style_id varchar(64) NOT NULL,
    color_code varchar(64) NOT NULL,
    sent_qty int NOT NULL CHECK (sent_qty > 0),
    received_qty int NOT NULL DEFAULT 0,
    short_qty int NOT NULL DEFAULT 0,
    rejected_qty int NOT NULL DEFAULT 0,
    tags_sent_count int NOT NULL DEFAULT 0,
    tags_returned_count int NOT NULL DEFAULT 0,
    agreed_turnaround_hours int NOT NULL DEFAULT 48,
    dispatched_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    received_at timestamptz,
    vendor_billed_qty int,
    bill_verified_at timestamptz,
    UNIQUE (tenant_id, lot_number),
    UNIQUE (tenant_id, challan_barcode)
);

-- RLS Policies for Tenant Isolation (V12)
DO $$
DECLARE
    tbl text;
BEGIN
    FOR tbl IN SELECT unnest(ARRAY[
        'cost_rates_v52','costing_sheets_v52','costing_versions_v52','costing_order_matrix_lines_v52',
        'costing_bom_lines_v52','costing_ob_pins_v52','costing_cost_lines_v52','costing_price_summaries_v52',
        'costing_scenarios_v52','costing_preview_runs_v52','costing_material_checks_v52',
        'costing_machine_checks_v52','costing_skill_checks_v52','costing_capacity_checks_v52',
        'costing_daily_ramps_v52','costing_capacity_holds_v52','costing_replan_records_v52',
        'costing_approvals_v52','planning_baselines_v52','costing_audit_events_v52',
        'ob_assignments_v52','roll_tag_mappings_v52','workstation_stoppages_v52',
        'operator_defect_ledger_v52','outside_process_challans_v52'
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
