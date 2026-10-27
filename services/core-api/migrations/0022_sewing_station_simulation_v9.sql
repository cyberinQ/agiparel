-- E09 09A-09C: transactional sewing rehearsal, explicitly non-authoritative.
-- UI events look like reader reports but cannot satisfy a physical-RFID, cutting-release,
-- payment, QC, finished-goods, or shipment requirement. Never promote by changing a label.
-- Deliberately requires the 08C tables. All mutations are tenant-scoped, auditable.
BEGIN;
CREATE TABLE sew_sim_operations_v9 (
 tenant_id varchar(64) NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
 id uuid NOT NULL DEFAULT uuid_generate_v4(),
 lay_draft_id uuid NOT NULL,
 plant_id varchar(64) NOT NULL,
 operation_code varchar(64) NOT NULL CHECK(operation_code ~ '^[A-Za-z0-9_-]{2,64}$'),
 mode varchar(32) NOT NULL CHECK(mode IN ('SINGLE_TO_SINGLE','BUNDLE_TO_BUNDLE','BUNDLE_TO_SINGLE')),
 required_parts text[] NOT NULL CHECK(cardinality(required_parts) BETWEEN 1 AND 16),
 predecessors text[] NOT NULL DEFAULT '{}',
 machine_class varchar(64) NOT NULL CHECK(length(machine_class) BETWEEN 2 AND 64),
 sam_seconds numeric(10,2) NOT NULL CHECK(sam_seconds>0 AND sam_seconds<=36000),
 route_revision varchar(64) NOT NULL DEFAULT 'UI_SIMULATION_V1',
 evidence_state varchar(48) NOT NULL DEFAULT 'UNPUBLISHED_SIMULATION_CONTRACT'
   CHECK(evidence_state='UNPUBLISHED_SIMULATION_CONTRACT'),
 created_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(tenant_id,id), UNIQUE(tenant_id,lay_draft_id,operation_code),
 FOREIGN KEY(tenant_id,lay_draft_id) REFERENCES cut_lay_drafts_v8(tenant_id,id) ON DELETE RESTRICT
);
CREATE TABLE sew_sim_scans_v9 (
 tenant_id varchar(64) NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
 id uuid NOT NULL DEFAULT uuid_generate_v4(),
 event_id varchar(80) NOT NULL CHECK(event_id ~ '^[A-Za-z0-9_-]{12,80}$'),
 payload_sha256 char(64) NOT NULL CHECK(payload_sha256 ~ '^[0-9a-f]{64}$'),
 lay_draft_id uuid NOT NULL,
 plant_id varchar(64) NOT NULL,
 label_code varchar(29) NOT NULL,
 station_id varchar(64) NOT NULL,
 worker_id varchar(64) NOT NULL,
 reader_id varchar(64) NOT NULL,
 operator_clicked_at timestamptz NOT NULL,
 antenna_port integer NOT NULL CHECK(antenna_port BETWEEN 1 AND 64),
 epc_hex varchar(96),
 rssi_dbm numeric(6,2),
 received_at timestamptz NOT NULL DEFAULT now(),
 provenance varchar(48) NOT NULL DEFAULT 'UI_SIMULATED_NO_RF_OBSERVATION'
   CHECK(provenance='UI_SIMULATED_NO_RF_OBSERVATION'),
 PRIMARY KEY(tenant_id,id), UNIQUE(tenant_id,event_id),
 FOREIGN KEY(tenant_id,lay_draft_id,label_code) REFERENCES cut_bundle_drafts_v8(tenant_id,lay_draft_id,label_code) ON DELETE RESTRICT,
 CHECK(epc_hex IS NULL AND rssi_dbm IS NULL)
);
CREATE INDEX sew_sim_scans_lookup_v9 ON sew_sim_scans_v9(tenant_id,lay_draft_id,station_id,worker_id,received_at DESC);
CREATE TABLE sew_sim_cycles_v9 (
 tenant_id varchar(64) NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
 id uuid NOT NULL DEFAULT uuid_generate_v4(),
 claim_id varchar(80) NOT NULL CHECK(claim_id ~ '^[A-Za-z0-9_-]{12,80}$'),
 claim_sha256 char(64) NOT NULL CHECK(claim_sha256 ~ '^[0-9a-f]{64}$'),
 operation_id uuid NOT NULL,
 lay_draft_id uuid NOT NULL,
 plant_id varchar(64) NOT NULL,
 marker_position integer NOT NULL,
 ply_group integer NOT NULL,
 station_id varchar(64) NOT NULL,
 worker_id varchar(64) NOT NULL,
 machine_class varchar(64) NOT NULL,
 state varchar(24) NOT NULL DEFAULT 'CLAIMED' CHECK(state IN ('CLAIMED','PAUSED','COMPLETED','REVIEW_REQUIRED')),
 active_since timestamptz,
 active_ms bigint NOT NULL DEFAULT 0 CHECK(active_ms>=0),
 paused_ms bigint NOT NULL DEFAULT 0 CHECK(paused_ms>=0),
 paused_since timestamptz,
 claimed_at timestamptz NOT NULL DEFAULT now(),
 finished_at timestamptz,
 PRIMARY KEY(tenant_id,id), UNIQUE(tenant_id,claim_id),
 UNIQUE(tenant_id,operation_id,lay_draft_id,marker_position,ply_group),
 FOREIGN KEY(tenant_id,operation_id) REFERENCES sew_sim_operations_v9(tenant_id,id) ON DELETE RESTRICT,
 FOREIGN KEY(tenant_id,lay_draft_id,marker_position) REFERENCES cut_lay_positions_v8(tenant_id,lay_draft_id,marker_position) ON DELETE RESTRICT
);
CREATE TABLE sew_sim_cycle_inputs_v9 (
 tenant_id varchar(64) NOT NULL,
 cycle_id uuid NOT NULL,
 scan_id uuid NOT NULL,
 label_code varchar(29) NOT NULL,
 part_code varchar(64) NOT NULL,
 PRIMARY KEY(tenant_id,cycle_id,scan_id), UNIQUE(tenant_id,scan_id),
 UNIQUE(tenant_id,cycle_id,part_code),
 FOREIGN KEY(tenant_id,cycle_id) REFERENCES sew_sim_cycles_v9(tenant_id,id) ON DELETE RESTRICT,
 FOREIGN KEY(tenant_id,scan_id) REFERENCES sew_sim_scans_v9(tenant_id,id) ON DELETE RESTRICT
);
CREATE TABLE sew_sim_events_v9 (
 tenant_id varchar(64) NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
 id uuid NOT NULL DEFAULT uuid_generate_v4(),
 command_id varchar(80) NOT NULL CHECK(command_id ~ '^[A-Za-z0-9_-]{12,80}$'),
 payload_sha256 char(64) NOT NULL CHECK(payload_sha256 ~ '^[0-9a-f]{64}$'),
 cycle_id uuid,
 plant_id varchar(64) NOT NULL,
 action varchar(32) NOT NULL CHECK(action IN ('CLAIM','PAUSE','RESUME','COMPLETE','ISSUE')),
 station_id varchar(64) NOT NULL,
 worker_id varchar(64) NOT NULL,
 detail text NOT NULL DEFAULT '' CHECK(length(detail)<=800),
 created_at timestamptz NOT NULL DEFAULT now(),
 provenance varchar(48) NOT NULL DEFAULT 'UI_SIMULATED_NO_RF_OBSERVATION'
  CHECK(provenance='UI_SIMULATED_NO_RF_OBSERVATION'),
 PRIMARY KEY(tenant_id,id), UNIQUE(tenant_id,command_id),
 FOREIGN KEY(tenant_id,cycle_id) REFERENCES sew_sim_cycles_v9(tenant_id,id) ON DELETE RESTRICT
);
CREATE TABLE sew_sim_credits_v9 (
 tenant_id varchar(64) NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
 cycle_id uuid NOT NULL,
 operation_id uuid NOT NULL,
 lay_draft_id uuid NOT NULL,
 marker_position integer NOT NULL,
 ply_group integer NOT NULL,
 garment_count integer NOT NULL CHECK(garment_count BETWEEN 1 AND 210),
 serials text[] NOT NULL CHECK(cardinality(serials)=garment_count),
 active_ms bigint NOT NULL CHECK(active_ms>=0),
 evidence_state varchar(48) NOT NULL DEFAULT 'SIMULATED_NOT_PRODUCTION_CREDIT'
  CHECK(evidence_state='SIMULATED_NOT_PRODUCTION_CREDIT'),
 created_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(tenant_id,cycle_id),
 UNIQUE(tenant_id,operation_id,lay_draft_id,marker_position,ply_group),
 FOREIGN KEY(tenant_id,cycle_id) REFERENCES sew_sim_cycles_v9(tenant_id,id) ON DELETE RESTRICT,
 FOREIGN KEY(tenant_id,operation_id) REFERENCES sew_sim_operations_v9(tenant_id,id) ON DELETE RESTRICT
);
CREATE FUNCTION sew_sim_immutable_v9() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN RAISE EXCEPTION 'ERR_SEW_SIM_EVIDENCE_IMMUTABLE'; END $$;
DO $$ DECLARE tbl text; BEGIN
 FOREACH tbl IN ARRAY ARRAY['sew_sim_operations_v9','sew_sim_scans_v9','sew_sim_cycles_v9','sew_sim_cycle_inputs_v9','sew_sim_events_v9','sew_sim_credits_v9'] LOOP
  EXECUTE format('ALTER TABLE public.%I ENABLE ROW LEVEL SECURITY',tbl);
  EXECUTE format('ALTER TABLE public.%I FORCE ROW LEVEL SECURITY',tbl);
  EXECUTE format('CREATE POLICY sew_sim_tenant_v9 ON public.%I FOR ALL TO agiparel_app USING (tenant_id=NULLIF(current_setting(''app.current_tenant_id'',true),'''')) WITH CHECK (tenant_id=NULLIF(current_setting(''app.current_tenant_id'',true),''''))',tbl);
  EXECUTE format('REVOKE ALL ON public.%I FROM PUBLIC,agiparel_app',tbl);
  EXECUTE format('GRANT SELECT,INSERT ON public.%I TO agiparel_app',tbl);
 END LOOP;
END $$;
GRANT UPDATE(state,active_since,active_ms,paused_ms,paused_since,finished_at) ON sew_sim_cycles_v9 TO agiparel_app;
DO $$ DECLARE tbl text; BEGIN
 FOREACH tbl IN ARRAY ARRAY['sew_sim_operations_v9','sew_sim_scans_v9','sew_sim_cycle_inputs_v9','sew_sim_events_v9','sew_sim_credits_v9'] LOOP
  EXECUTE format('CREATE TRIGGER sew_sim_immutable_guard_v9 BEFORE UPDATE OR DELETE ON public.%I FOR EACH ROW EXECUTE FUNCTION sew_sim_immutable_v9()',tbl);
 END LOOP;
END $$;
-- Cycles alone are mutable, and only along recognized station state transitions.
CREATE FUNCTION guard_sew_sim_cycle_transition_v9() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF TG_OP='DELETE' THEN RAISE EXCEPTION 'ERR_SEW_SIM_CYCLE_DELETE'; END IF;
 IF NEW.tenant_id<>OLD.tenant_id OR NEW.id<>OLD.id OR
    NEW.claim_id<>OLD.claim_id OR NEW.claim_sha256<>OLD.claim_sha256 OR
    NEW.operation_id<>OLD.operation_id OR NEW.lay_draft_id<>OLD.lay_draft_id OR
    NEW.marker_position<>OLD.marker_position OR NEW.ply_group<>OLD.ply_group OR
    NEW.plant_id<>OLD.plant_id OR NEW.station_id<>OLD.station_id OR NEW.worker_id<>OLD.worker_id OR
    NEW.machine_class<>OLD.machine_class OR NEW.claimed_at<>OLD.claimed_at OR
    NEW.active_ms<OLD.active_ms OR NEW.paused_ms<OLD.paused_ms THEN
  RAISE EXCEPTION 'ERR_SEW_SIM_CYCLE_IDENTITY_OR_DURATION_TAMPER';
 END IF;
 IF NOT ((OLD.state='CLAIMED' AND NEW.state IN('PAUSED','COMPLETED','REVIEW_REQUIRED'))
   OR (OLD.state='PAUSED' AND NEW.state IN('CLAIMED','REVIEW_REQUIRED'))) THEN
  RAISE EXCEPTION 'ERR_SEW_SIM_INVALID_STATE_TRANSITION';
 END IF;
 IF (NEW.state='CLAIMED' AND (NEW.active_since IS NULL OR NEW.paused_since IS NOT NULL)) OR
    (NEW.state='PAUSED' AND (NEW.active_since IS NOT NULL OR NEW.paused_since IS NULL)) OR
    (NEW.state IN('COMPLETED','REVIEW_REQUIRED') AND (NEW.active_since IS NOT NULL OR NEW.paused_since IS NOT NULL)) OR
    (NEW.state='COMPLETED' AND NEW.finished_at IS NULL) THEN
  RAISE EXCEPTION 'ERR_SEW_SIM_CYCLE_CLOCK_STATE';
 END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER sew_sim_cycle_state_guard_v9 BEFORE UPDATE OR DELETE ON sew_sim_cycles_v9
 FOR EACH ROW EXECUTE FUNCTION guard_sew_sim_cycle_transition_v9();
-- Even a forged direct SQL INSERT cannot create simulated credits without a completed cycle
-- covering the same operation and source lineage. Credit is never authoritative production.
CREATE FUNCTION validate_sew_sim_credit_v9() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE c sew_sim_cycles_v9%ROWTYPE; op sew_sim_operations_v9%ROWTYPE;
        counted integer; first_ply integer; last_ply integer; bad integer;
        ply_starts integer; ply_ends integer; colors integer; sizes integer; expected_serials text[];
BEGIN
 SELECT * INTO c FROM sew_sim_cycles_v9 WHERE tenant_id=NEW.tenant_id AND id=NEW.cycle_id;
 IF NOT FOUND OR c.state<>'COMPLETED' OR c.operation_id<>NEW.operation_id OR
   c.lay_draft_id<>NEW.lay_draft_id OR c.marker_position<>NEW.marker_position OR
   c.ply_group<>NEW.ply_group OR c.active_ms<>NEW.active_ms THEN
  RAISE EXCEPTION 'ERR_SEW_SIM_CREDIT_WITHOUT_COMPLETION';
 END IF;
 SELECT * INTO op FROM sew_sim_operations_v9 WHERE tenant_id=NEW.tenant_id AND id=NEW.operation_id;
 SELECT count(*)::int, min(b.ply_start),max(b.ply_end),
   count(DISTINCT b.ply_start)::int,count(DISTINCT b.ply_end)::int,
   count(DISTINCT b.color_code)::int,count(DISTINCT b.size_code)::int,
   count(*) FILTER (WHERE NOT (b.part_code=ANY(op.required_parts)) OR
       b.lay_draft_id<>NEW.lay_draft_id OR b.marker_position<>NEW.marker_position OR
       b.ply_group<>NEW.ply_group)::int
 INTO counted,first_ply,last_ply,ply_starts,ply_ends,colors,sizes,bad
 FROM sew_sim_cycle_inputs_v9 i
 JOIN cut_bundle_drafts_v8 b ON b.tenant_id=i.tenant_id AND b.label_code=i.label_code
 WHERE i.tenant_id=NEW.tenant_id AND i.cycle_id=NEW.cycle_id;
 IF counted<>cardinality(op.required_parts) OR bad<>0 OR
    ply_starts<>1 OR ply_ends<>1 OR colors<>1 OR sizes<>1 OR
   NEW.garment_count<>(last_ply-first_ply+1) THEN
  RAISE EXCEPTION 'ERR_SEW_SIM_CREDIT_INPUT_INVARIANT';
 END IF;
 SELECT array_agg(format('SIM-G-%s-%s-%s',NEW.lay_draft_id,NEW.marker_position,g.n) ORDER BY g.n)
 INTO expected_serials FROM generate_series(first_ply,last_ply) g(n);
 IF NEW.serials IS DISTINCT FROM expected_serials THEN
  RAISE EXCEPTION 'ERR_SEW_SIM_CREDIT_SERIAL_FORGERY';
 END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER sew_sim_credit_guard_v9 BEFORE INSERT ON sew_sim_credits_v9
 FOR EACH ROW EXECUTE FUNCTION validate_sew_sim_credit_v9();
COMMIT;
