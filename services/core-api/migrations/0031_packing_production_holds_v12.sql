-- Author: Sunil Jain, 2026-10-28.
-- Chapter 08: production packing *staging holds*, not finished-garment or dispatch authorizations.
-- Requires 0029 and 0030; provision assignments through privileged governance only.
BEGIN;
CREATE TABLE pack_prod_assignments_v12 (
 tenant_id varchar(64) NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
 plant_id varchar(64) NOT NULL,
 station_id varchar(64) NOT NULL,
 worker_subject varchar(128) NOT NULL,
 role varchar(32) NOT NULL CHECK(role IN ('packing_operator','packing_supervisor')),
 enabled boolean NOT NULL DEFAULT false,
 approved_by varchar(128) NOT NULL,
 PRIMARY KEY(tenant_id,plant_id,station_id,worker_subject,role)
);
CREATE TABLE pack_prod_holds_v12 (
 tenant_id varchar(64) NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
 id uuid NOT NULL DEFAULT uuid_generate_v4(),
 command_id varchar(80) NOT NULL CHECK(command_id ~ '^[A-Za-z0-9_-]{12,80}$'),
 payload_sha256 char(64) NOT NULL CHECK(payload_sha256 ~ '^[0-9a-f]{64}$'),
 completion_id uuid NOT NULL,
 qc_event_id uuid NOT NULL,
 qc_revision integer NOT NULL CHECK(qc_revision > 0),
 plant_id varchar(64) NOT NULL,
 station_id varchar(64) NOT NULL,
 nominal_group_units integer NOT NULL CHECK(nominal_group_units > 0),
 note varchar(1000) NOT NULL DEFAULT '',
 actor_subject varchar(128) NOT NULL,
 proof_id varchar(255) NOT NULL,
 assertion_sha256 char(64) NOT NULL CHECK(assertion_sha256 ~ '^[0-9a-f]{64}$'),
 state varchar(40) NOT NULL DEFAULT 'HOLD_PENDING_UNIT_TRACEABILITY'
  CHECK(state='HOLD_PENDING_UNIT_TRACEABILITY'),
 created_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(tenant_id,id), UNIQUE(tenant_id,command_id), UNIQUE(tenant_id,proof_id),
 UNIQUE(tenant_id,completion_id,qc_revision),
 FOREIGN KEY(tenant_id,completion_id) REFERENCES sew_prod_completions_v10(tenant_id,id) ON DELETE RESTRICT,
 FOREIGN KEY(tenant_id,qc_event_id) REFERENCES qc_prod_group_events_v11(tenant_id,id) ON DELETE RESTRICT
);
CREATE INDEX pack_prod_holds_recent_v12 ON pack_prod_holds_v12(tenant_id,plant_id,created_at DESC,id DESC);
-- A production-group hold is not a garment! Only a reviewed group may be staged;
-- never create a packing, warehouse or shipping release from this table.
CREATE FUNCTION guard_pack_prod_hold_v12() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE src sew_prod_completions_v10%ROWTYPE; qc qc_prod_group_events_v11%ROWTYPE;
BEGIN
 PERFORM pg_advisory_xact_lock(hashtext(NEW.tenant_id),hashtext('QC11:'||NEW.completion_id::text));
 SELECT * INTO src FROM sew_prod_completions_v10 WHERE tenant_id=NEW.tenant_id AND id=NEW.completion_id;
 IF NOT FOUND OR src.plant_id<>NEW.plant_id THEN
  RAISE EXCEPTION 'ERR_PACK_PROD_SEWING_LINEAGE';
 END IF;
 IF NOT EXISTS(SELECT 1 FROM pack_prod_assignments_v12 a WHERE a.tenant_id=NEW.tenant_id
  AND a.plant_id=NEW.plant_id AND a.station_id=NEW.station_id
  AND a.worker_subject=NEW.actor_subject AND a.role='packing_operator' AND a.enabled) THEN
  RAISE EXCEPTION 'ERR_PACK_PROD_OPERATOR_ASSIGNMENT';
 END IF;
 SELECT * INTO qc FROM qc_prod_group_events_v11 WHERE tenant_id=NEW.tenant_id
  AND completion_id=NEW.completion_id ORDER BY revision DESC LIMIT 1;
 IF NOT FOUND OR qc.next_state<>'REVIEWED' OR qc.revision<>NEW.qc_revision THEN
  RAISE EXCEPTION 'ERR_PACK_PROD_QC_NOT_CURRENTLY_REVIEWED';
 END IF;
 NEW.qc_event_id:=qc.id;
 NEW.nominal_group_units:=src.nominal_ply_count;
 RETURN NEW;
END $$;
CREATE TRIGGER pack_prod_hold_guard_v12 BEFORE INSERT ON pack_prod_holds_v12
 FOR EACH ROW EXECUTE FUNCTION guard_pack_prod_hold_v12();
CREATE TRIGGER pack_prod_hold_immutable_v12 BEFORE UPDATE OR DELETE ON pack_prod_holds_v12
 FOR EACH ROW EXECUTE FUNCTION protect_cut_draft_v8();
DO $$ DECLARE tbl text; BEGIN
 FOREACH tbl IN ARRAY ARRAY['pack_prod_assignments_v12','pack_prod_holds_v12'] LOOP
  EXECUTE format('ALTER TABLE public.%I ENABLE ROW LEVEL SECURITY',tbl);
  EXECUTE format('ALTER TABLE public.%I FORCE ROW LEVEL SECURITY',tbl);
  EXECUTE format('CREATE POLICY pack_prod_tenant_v12 ON public.%I FOR ALL TO agiparel_app USING (tenant_id=NULLIF(current_setting(''app.current_tenant_id'',true),'''')) WITH CHECK (tenant_id=NULLIF(current_setting(''app.current_tenant_id'',true),''''))',tbl);
  EXECUTE format('REVOKE ALL ON public.%I FROM PUBLIC,agiparel_app',tbl);
  EXECUTE format('GRANT SELECT ON public.%I TO agiparel_app',tbl);
 END LOOP;
END $$;
GRANT INSERT ON pack_prod_holds_v12 TO agiparel_app;
COMMIT;
