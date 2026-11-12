-- Author: Sunil Jain, 2026-11-11
-- AGIPAREL native Cut-SOP-2 add-on. Apply AFTER 0001..0004 in a development
-- database. Never rewrite v8 signed cutting/spread/release evidence.
BEGIN;

CREATE TABLE IF NOT EXISTS public.cut_sop2_target_versions_v46 (
    id uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id varchar(64) NOT NULL,
    intake_id varchar(40) NOT NULL REFERENCES public.order_intakes_v5(id),
    version int NOT NULL CHECK (version >= 1),
    po_min_extra_bp int NOT NULL CHECK (po_min_extra_bp BETWEEN 0 AND 10000),
    po_max_extra_bp int NOT NULL CHECK (po_max_extra_bp BETWEEN po_min_extra_bp AND 10000),
    source_order_sha256 char(64) NOT NULL CHECK (source_order_sha256 ~ '^[0-9a-f]{64}$'),
    request_key varchar(128) NOT NULL CHECK (request_key ~ '^[A-Za-z0-9_-]{8,128}$'),
    request_sha256 char(64) NOT NULL CHECK (request_sha256 ~ '^[0-9a-f]{64}$'),
    actor_note varchar(128) NOT NULL,
    reason varchar(1000) NOT NULL CHECK (length(btrim(reason)) >= 12),
    change_request_id uuid,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE(tenant_id,intake_id,version),
    UNIQUE(tenant_id,request_key),
    UNIQUE(tenant_id,id)
);

CREATE TABLE IF NOT EXISTS public.cut_sop2_target_skus_v46 (
    tenant_id varchar(64) NOT NULL,
    target_id uuid NOT NULL,
    color_code varchar(64) NOT NULL,
    size_code varchar(32) NOT NULL,
    ordered_qty bigint NOT NULL CHECK(ordered_qty>0),
    min_extra_bp int NOT NULL CHECK(min_extra_bp BETWEEN 0 AND 10000),
    max_extra_bp int NOT NULL CHECK(max_extra_bp BETWEEN min_extra_bp AND 10000),
    frozen_target bigint NOT NULL CHECK(frozen_target>0),
    PRIMARY KEY (tenant_id,target_id,color_code,size_code),
    FOREIGN KEY (tenant_id,target_id) REFERENCES public.cut_sop2_target_versions_v46(tenant_id,id)
);

CREATE TABLE IF NOT EXISTS public.cut_sop2_change_requests_v46 (
    id uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id varchar(64) NOT NULL,
    resource_type varchar(32) NOT NULL CHECK (resource_type IN ('PO_TARGET','STYLE_LEAD','DAILY_CUT','PANEL_NUMBERING','SEWING_PLAN')),
    resource_id varchar(128) NOT NULL,
    old_version int NOT NULL CHECK (old_version>=1),
    proposed_version int NOT NULL CHECK (proposed_version=old_version+1),
    reason_code varchar(64) NOT NULL,
    impact jsonb NOT NULL DEFAULT '{}'::jsonb,
    required_roles jsonb NOT NULL CHECK (jsonb_typeof(required_roles)='array'),
    proposed_payload_sha256 char(64) NOT NULL CHECK (proposed_payload_sha256 ~ '^[0-9a-f]{64}$'),
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE(tenant_id,id)
);

CREATE TABLE IF NOT EXISTS public.cut_sop2_change_approvals_v46 (
    tenant_id varchar(64) NOT NULL,
    change_id uuid NOT NULL,
    role varchar(64) NOT NULL,
    actor_subject varchar(128) NOT NULL,
    proof_id varchar(255) NOT NULL,
    approved_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id,change_id,role),
    FOREIGN KEY(tenant_id,change_id) REFERENCES public.cut_sop2_change_requests_v46(tenant_id,id),
    UNIQUE(tenant_id,proof_id)
);

CREATE TABLE IF NOT EXISTS public.cut_sop2_inspection_policies_v46 (
    tenant_id varchar(64) NOT NULL,
    fabric_sku varchar(128) NOT NULL,
    revision int NOT NULL CHECK(revision>0),
    min_width_mm bigint NOT NULL CHECK(min_width_mm>0),
    max_defect_points bigint NOT NULL CHECK(max_defect_points>=0),
    max_length_loss_bp int NOT NULL CHECK(max_length_loss_bp BETWEEN 0 AND 10000),
    width_samples int NOT NULL CHECK(width_samples>=2),
    relax_hours bigint NOT NULL CHECK(relax_hours>=0),
    approved_by varchar(128) NOT NULL,
    approved_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id,fabric_sku,revision)
);

CREATE TABLE IF NOT EXISTS public.cut_sop2_roll_inspections_v46 (
    id uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id varchar(64) NOT NULL,
    roll_id varchar(64) NOT NULL REFERENCES public.store_rolls(id),
    policy_sku varchar(128) NOT NULL,
    policy_revision int NOT NULL,
    ticket_length_mm bigint NOT NULL CHECK(ticket_length_mm>0),
    measured_length_mm bigint NOT NULL CHECK(measured_length_mm>0),
    usable_width_mm bigint NOT NULL CHECK(usable_width_mm>=0),
    shade varchar(32) NOT NULL,
    measurements jsonb NOT NULL,
    defects jsonb NOT NULL,
    points bigint NOT NULL CHECK(points>=0),
    decision varchar(16) NOT NULL CHECK(decision IN ('PASS','HOLD')),
    reasons jsonb NOT NULL,
    inspected_by varchar(128) NOT NULL,
    measured_at timestamptz NOT NULL DEFAULT now(),
    FOREIGN KEY(tenant_id,policy_sku,policy_revision) REFERENCES public.cut_sop2_inspection_policies_v46(tenant_id,fabric_sku,revision)
);
CREATE INDEX IF NOT EXISTS idx_sop2_inspections_roll ON public.cut_sop2_roll_inspections_v46(tenant_id,roll_id,measured_at DESC);

CREATE TABLE IF NOT EXISTS public.cut_sop2_schedule_versions_v46 (
    id uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id varchar(64) NOT NULL,
    factory_date date NOT NULL,
    version int NOT NULL CHECK(version>0),
    source_sewing_plan_sha256 char(64) NOT NULL CHECK(source_sewing_plan_sha256 ~ '^[0-9a-f]{64}$'),
    style_lead_version varchar(64) NOT NULL,
    capacity_snapshot_sha256 char(64) NOT NULL CHECK(capacity_snapshot_sha256 ~ '^[0-9a-f]{64}$'),
    accepted_plan jsonb NOT NULL CHECK(jsonb_typeof(accepted_plan)='array'),
    status varchar(16) NOT NULL DEFAULT 'PROPOSED' CHECK(status IN ('PROPOSED','FROZEN')),
    created_by varchar(128) NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE(tenant_id,factory_date,version)
);

CREATE TABLE IF NOT EXISTS public.cut_sop2_nfc_bindings_v46 (
    id uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id varchar(64) NOT NULL,
    label_code varchar(29) NOT NULL,
    nfc_uid varchar(128) NOT NULL,
    draft_id uuid NOT NULL REFERENCES public.cut_lay_drafts_v8(id),
    event_type varchar(12) NOT NULL CHECK(event_type IN ('BIND','UNBIND')),
    change_request_id uuid,
    evidence_sha256 char(64) NOT NULL CHECK(evidence_sha256 ~ '^[0-9a-f]{64}$'),
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_sop2_nfc_label ON public.cut_sop2_nfc_bindings_v46(tenant_id,label_code,created_at DESC);

CREATE TABLE IF NOT EXISTS public.cut_sop2_marriage_edges_v46 (
    tenant_id varchar(64) NOT NULL,
    draft_id uuid NOT NULL REFERENCES public.cut_lay_drafts_v8(id),
    label_left varchar(29) NOT NULL,
    label_right varchar(29) NOT NULL,
    family_key varchar(100) NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY(tenant_id,draft_id,label_left,label_right),
    CHECK(label_left<label_right)
);

CREATE TABLE IF NOT EXISTS public.cut_sop2_material_reconciliations_v46 (
    tenant_id varchar(64) NOT NULL,
    spread_id uuid NOT NULL REFERENCES public.cut_spread_reports_v8(id),
    issued_mm bigint NOT NULL CHECK (issued_mm>=0),
    plies_mm bigint NOT NULL CHECK (plies_mm>=0),
    reserve_used_mm bigint NOT NULL CHECK (reserve_used_mm>=0),
    reserve_returned_mm bigint NOT NULL CHECK (reserve_returned_mm>=0),
    endbits_mm bigint NOT NULL CHECK (endbits_mm>=0),
    defects_mm bigint NOT NULL CHECK (defects_mm>=0),
    difference_mm bigint GENERATED ALWAYS AS
      (issued_mm-plies_mm-reserve_used_mm-reserve_returned_mm-endbits_mm-defects_mm) STORED,
    recorded_by varchar(128) NOT NULL,
    recorded_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY(tenant_id,spread_id)
);

-- Defense in depth: each new table is tenant-scoped even for SELECT.
DO $$
DECLARE tbl text;
BEGIN
    FOREACH tbl IN ARRAY ARRAY[
      'cut_sop2_target_versions_v46','cut_sop2_target_skus_v46',
      'cut_sop2_change_requests_v46','cut_sop2_change_approvals_v46',
      'cut_sop2_inspection_policies_v46','cut_sop2_roll_inspections_v46',
      'cut_sop2_schedule_versions_v46','cut_sop2_nfc_bindings_v46',
      'cut_sop2_marriage_edges_v46','cut_sop2_material_reconciliations_v46'
    ] LOOP
        EXECUTE format('ALTER TABLE public.%I ENABLE ROW LEVEL SECURITY',tbl);
        EXECUTE format('ALTER TABLE public.%I FORCE ROW LEVEL SECURITY',tbl);
        EXECUTE format('CREATE POLICY %I ON public.%I TO agiparel_app USING (tenant_id = nullif(current_setting(''app.current_tenant_id'',true),'''')) WITH CHECK (tenant_id = nullif(current_setting(''app.current_tenant_id'',true),''''))','sop2_tenant',tbl);
        EXECUTE format('GRANT SELECT, INSERT ON public.%I TO agiparel_app',tbl);
    END LOOP;
END $$;

-- A frozen target and its line items are append-only for agiparel_app. To modify
-- a frozen plan, create a new version with independently verified approvals.
COMMIT;
