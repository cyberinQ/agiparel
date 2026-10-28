-- Author: Sunil Jain, 2026-10-28.
-- Chapter 07: independent, strongly authenticated, append-only production QC
-- of sewing-completion GROUPS. No garment-level QA, stock or shipping release.
-- Prerequisite: 0029_sewing_production_evidence_v10.sql.
BEGIN;
CREATE TABLE qc_prod_defect_catalog_v11 (
 tenant_id varchar(64) NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
 plant_id varchar(64) NOT NULL,
 code varchar(64) NOT NULL CHECK(code ~ '^[A-Za-z0-9_-]{2,64}$'),
 description text NOT NULL CHECK(length(trim(description)) BETWEEN 2 AND 240),
 active boolean NOT NULL DEFAULT false,
 approved_by varchar(128) NOT NULL,
 PRIMARY KEY (tenant_id,plant_id,code)
);
CREATE TABLE qc_prod_assignments_v11 (
 tenant_id varchar(64) NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
 plant_id varchar(64) NOT NULL,
 station_id varchar(64) NOT NULL,
 worker_subject varchar(128) NOT NULL,
 role varchar(32) NOT NULL CHECK(role IN ('quality_inspector','quality_repairer')),
 enabled boolean NOT NULL DEFAULT false,
 approved_by varchar(128) NOT NULL,
 PRIMARY KEY(tenant_id,plant_id,station_id,worker_subject,role)
);
CREATE TABLE qc_prod_group_events_v11 (
 tenant_id varchar(64) NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
 id uuid NOT NULL DEFAULT uuid_generate_v4(),
 command_id varchar(80) NOT NULL CHECK(command_id ~ '^[A-Za-z0-9_-]{12,80}$'),
 payload_sha256 char(64) NOT NULL CHECK(payload_sha256 ~ '^[0-9a-f]{64}$'),
 completion_id uuid NOT NULL,
 plant_id varchar(64) NOT NULL,
 station_id varchar(64) NOT NULL,
 action varchar(24) NOT NULL CHECK(action IN ('INSPECT_PASS','INSPECT_HOLD','REPAIR_COMPLETE','REINSPECT_PASS','REINSPECT_HOLD')),
 inspected_units integer NOT NULL CHECK(inspected_units >= 0),
 defective_units integer NOT NULL CHECK(defective_units >= 0),
 defect_code varchar(64) NOT NULL DEFAULT '',
 note varchar(1000) NOT NULL DEFAULT '',
 actor_subject varchar(128) NOT NULL,
 proof_id varchar(255) NOT NULL,
 assertion_sha256 char(64) NOT NULL CHECK(assertion_sha256 ~ '^[0-9a-f]{64}$'),
 prior_state varchar(16) NOT NULL DEFAULT '',
 next_state varchar(16) NOT NULL DEFAULT '',
 revision integer NOT NULL DEFAULT 0 CHECK(revision > 0),
 created_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(tenant_id,id), UNIQUE(tenant_id,command_id), UNIQUE(tenant_id,proof_id),
 UNIQUE(tenant_id,completion_id,revision),
 FOREIGN KEY(tenant_id,completion_id) REFERENCES sew_prod_completions_v10(tenant_id,id) ON DELETE RESTRICT
);
CREATE INDEX qc_prod_group_history_v11 ON qc_prod_group_events_v11(tenant_id,plant_id,completion_id,revision DESC);
CREATE INDEX qc_prod_group_recent_v11 ON qc_prod_group_events_v11(tenant_id,plant_id,created_at DESC,id DESC);

-- Database-level state machine prevents forged direct SQL acceptance and races.
CREATE FUNCTION guard_qc_prod_group_event_v11() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE source_row sew_prod_completions_v10%ROWTYPE;
        previous qc_prod_group_events_v11%ROWTYPE;
        bad_actor text;
        needed_role text;
        next_value text;
BEGIN
 PERFORM pg_advisory_xact_lock(hashtext(NEW.tenant_id),hashtext('QC11:'||NEW.completion_id::text));
 SELECT * INTO source_row FROM sew_prod_completions_v10
 WHERE tenant_id=NEW.tenant_id AND id=NEW.completion_id;
 IF NOT FOUND OR source_row.plant_id<>NEW.plant_id THEN
  RAISE EXCEPTION 'ERR_QC_PROD_SEWING_LINEAGE';
 END IF;
 IF NEW.action='REPAIR_COMPLETE' THEN needed_role:='quality_repairer';
 ELSE needed_role:='quality_inspector'; END IF;
 IF NOT EXISTS(SELECT 1 FROM qc_prod_assignments_v11 a
  WHERE a.tenant_id=NEW.tenant_id AND a.plant_id=NEW.plant_id
   AND a.station_id=NEW.station_id AND a.worker_subject=NEW.actor_subject
   AND a.role=needed_role AND a.enabled) THEN
  RAISE EXCEPTION 'ERR_QC_PROD_ASSIGNMENT';
 END IF;
 SELECT * INTO previous FROM qc_prod_group_events_v11
  WHERE tenant_id=NEW.tenant_id AND completion_id=NEW.completion_id
  ORDER BY revision DESC LIMIT 1;
 IF FOUND THEN
  NEW.prior_state:=previous.next_state;
  NEW.revision:=previous.revision+1;
 ELSE
  NEW.prior_state:='PENDING'; NEW.revision:=1;
 END IF;
 IF NEW.action IN ('INSPECT_PASS','REINSPECT_PASS') THEN
  IF NEW.inspected_units<>source_row.nominal_ply_count OR NEW.defective_units<>0 OR NEW.defect_code<>'' THEN
   RAISE EXCEPTION 'ERR_QC_PROD_FULL_INSPECTION_REQUIRED';
  END IF;
 ELSIF NEW.action IN ('INSPECT_HOLD','REINSPECT_HOLD') THEN
  IF NEW.inspected_units<1 OR NEW.inspected_units>source_row.nominal_ply_count OR
    NEW.defective_units<1 OR NEW.defective_units>NEW.inspected_units OR NEW.defect_code='' OR
    NOT EXISTS(SELECT 1 FROM qc_prod_defect_catalog_v11 c WHERE
       c.tenant_id=NEW.tenant_id AND c.plant_id=NEW.plant_id
       AND c.code=NEW.defect_code AND c.active) THEN
   RAISE EXCEPTION 'ERR_QC_PROD_DEFECT_OR_COUNTS';
  END IF;
 ELSE
  IF NEW.inspected_units<>0 OR NEW.defective_units<>0 OR NEW.defect_code<>'' OR length(trim(NEW.note))=0 THEN
   RAISE EXCEPTION 'ERR_QC_PROD_REPAIR_EVIDENCE';
  END IF;
 END IF;
 IF NEW.prior_state='PENDING' AND NEW.action IN ('INSPECT_PASS','INSPECT_HOLD') THEN
  IF NEW.action='INSPECT_PASS' THEN next_value:='REVIEWED'; ELSE next_value:='HOLD'; END IF;
 ELSIF NEW.prior_state='REVIEWED' AND NEW.action='INSPECT_HOLD' THEN
  next_value:='HOLD';
 ELSIF NEW.prior_state='HOLD' AND NEW.action='REPAIR_COMPLETE' AND NEW.actor_subject<>previous.actor_subject THEN
  next_value:='REINSPECT';
 ELSIF NEW.prior_state='REINSPECT' AND NEW.action IN ('REINSPECT_PASS','REINSPECT_HOLD') THEN
  SELECT e.actor_subject INTO bad_actor FROM qc_prod_group_events_v11 e
  WHERE e.tenant_id=NEW.tenant_id AND e.completion_id=NEW.completion_id
   AND e.revision=previous.revision-1 AND e.next_state='HOLD';
  IF NEW.actor_subject=previous.actor_subject OR NEW.actor_subject=bad_actor OR bad_actor IS NULL THEN
   RAISE EXCEPTION 'ERR_QC_PROD_INDEPENDENT_REINSPECTION';
  END IF;
  IF NEW.action='REINSPECT_PASS' THEN next_value:='REVIEWED'; ELSE next_value:='HOLD'; END IF;
 ELSE
  RAISE EXCEPTION 'ERR_QC_PROD_STATE_TRANSITION';
 END IF;
 NEW.next_state:=next_value;
 RETURN NEW;
END $$;
CREATE TRIGGER qc_prod_group_guard_v11 BEFORE INSERT ON qc_prod_group_events_v11
 FOR EACH ROW EXECUTE FUNCTION guard_qc_prod_group_event_v11();
CREATE TRIGGER qc_prod_group_immutable_v11 BEFORE UPDATE OR DELETE ON qc_prod_group_events_v11
 FOR EACH ROW EXECUTE FUNCTION protect_cut_draft_v8();
DO $$ DECLARE tbl text; BEGIN
 FOREACH tbl IN ARRAY ARRAY['qc_prod_defect_catalog_v11','qc_prod_assignments_v11','qc_prod_group_events_v11'] LOOP
  EXECUTE format('ALTER TABLE public.%I ENABLE ROW LEVEL SECURITY',tbl);
  EXECUTE format('ALTER TABLE public.%I FORCE ROW LEVEL SECURITY',tbl);
  EXECUTE format('CREATE POLICY qc_prod_tenant_v11 ON public.%I FOR ALL TO agiparel_app USING (tenant_id=NULLIF(current_setting(''app.current_tenant_id'',true),'''')) WITH CHECK (tenant_id=NULLIF(current_setting(''app.current_tenant_id'',true),''''))',tbl);
  EXECUTE format('REVOKE ALL ON public.%I FROM PUBLIC,agiparel_app',tbl);
  EXECUTE format('GRANT SELECT ON public.%I TO agiparel_app',tbl);
 END LOOP;
END $$;
-- Only privileged provisioning can approve personnel and defect codes.
GRANT INSERT ON qc_prod_group_events_v11 TO agiparel_app;
COMMIT;
