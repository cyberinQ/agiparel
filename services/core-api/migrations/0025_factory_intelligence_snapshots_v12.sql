-- CyberInQ Agiparel E12/E13: audited rehearsal intelligence snapshots, NOT production closure.
-- Requires 0019..0024. No production evidence is created or upgraded.
BEGIN;
CREATE TABLE intel_sim_snapshots_v12 (
 tenant_id varchar(64) NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
 capture_id varchar(80) NOT NULL CHECK(capture_id ~ '^[A-Za-z0-9_-]{12,80}$'),
 request_sha256 char(64) NOT NULL CHECK(request_sha256 ~ '^[0-9a-f]{64}$'),
 lay_draft_id uuid NOT NULL,
 factory_timezone varchar(64) NOT NULL,
 report jsonb NOT NULL CHECK(jsonb_typeof(report)='object' AND report->>'authority'='REHEARSAL_ONLY_NOT_PRODUCTION_CLOSURE'),
 report_sha256 char(64) NOT NULL CHECK(report_sha256 ~ '^[0-9a-f]{64}$'),
 captured_at timestamptz NOT NULL DEFAULT now(),
 provenance varchar(48) NOT NULL DEFAULT 'DERIVED_FROM_SIMULATED_STAGE_EVENTS'
   CHECK(provenance='DERIVED_FROM_SIMULATED_STAGE_EVENTS'),
 PRIMARY KEY(tenant_id,capture_id),
 FOREIGN KEY(tenant_id,lay_draft_id) REFERENCES cut_lay_drafts_v8(tenant_id,id) ON DELETE RESTRICT
);
CREATE INDEX intel_sim_snapshots_by_lay_v12 ON intel_sim_snapshots_v12(tenant_id,lay_draft_id,captured_at DESC);
ALTER TABLE intel_sim_snapshots_v12 ENABLE ROW LEVEL SECURITY;
ALTER TABLE intel_sim_snapshots_v12 FORCE ROW LEVEL SECURITY;
CREATE POLICY intel_sim_snapshot_tenant_v12 ON intel_sim_snapshots_v12 FOR ALL TO agiparel_app
 USING (tenant_id=NULLIF(current_setting('app.current_tenant_id',true),''))
 WITH CHECK (tenant_id=NULLIF(current_setting('app.current_tenant_id',true),''));
REVOKE ALL ON intel_sim_snapshots_v12 FROM PUBLIC,agiparel_app;
GRANT SELECT,INSERT ON intel_sim_snapshots_v12 TO agiparel_app;
CREATE FUNCTION intel_sim_snapshot_no_edit_v12() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN RAISE EXCEPTION 'ERR_INTEL_SIM_SNAPSHOT_IMMUTABLE'; END $$;
CREATE TRIGGER intel_sim_snapshots_immutable_v12 BEFORE UPDATE OR DELETE ON intel_sim_snapshots_v12
 FOR EACH ROW EXECUTE FUNCTION intel_sim_snapshot_no_edit_v12();
COMMIT;
