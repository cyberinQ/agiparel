-- Author: Sunil Jain, CyberInQ, 2026-10-27
-- E10/Batch 10A-B. UI simulation of per-garment inspection, NCR/repair,
-- finishing and buyer-identifier checks. Deliberately NOT factory QA release.
-- Requires migration 0022. Does not modify any authoritative shipment, pack,
-- cutting release, payroll, or physical-reader table.
BEGIN;
CREATE TABLE qc_sim_routes_v10 (
 tenant_id varchar(64) NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
 lay_draft_id uuid NOT NULL,
 plant_id varchar(64) NOT NULL,
 profile varchar(32) NOT NULL CHECK(profile IN ('WOVEN_STANDARD','WASH_FINISH','HANGER_FINISH','KNIT_FOLD')),
 steps text[] NOT NULL CHECK(
   (profile='WOVEN_STANDARD' AND steps=ARRAY['TRIM','PRESS']::text[]) OR
   (profile='WASH_FINISH' AND steps=ARRAY['WASH','DRY','PRESS','TRIM']::text[]) OR
   (profile='HANGER_FINISH' AND steps=ARRAY['TRIM','PRESS','HANGER']::text[]) OR
   (profile='KNIT_FOLD' AND steps=ARRAY['TRIM','PRESS','FOLD']::text[])),
 evidence_state varchar(40) NOT NULL DEFAULT 'UNAPPROVED_SIMULATION_ROUTE'
   CHECK(evidence_state='UNAPPROVED_SIMULATION_ROUTE'),
 created_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(tenant_id,lay_draft_id,plant_id),
 FOREIGN KEY(tenant_id,lay_draft_id) REFERENCES cut_lay_drafts_v8(tenant_id,id) ON DELETE RESTRICT
);
CREATE TABLE qc_sim_units_v10 (
 tenant_id varchar(64) NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
 serial varchar(128) NOT NULL CHECK(serial LIKE 'SIM-G-%'),
 lay_draft_id uuid NOT NULL,
 plant_id varchar(64) NOT NULL,
 marker_position integer NOT NULL,
 color_code varchar(64) NOT NULL,
 size_code varchar(32) NOT NULL,
 qc_state varchar(24) NOT NULL DEFAULT 'PENDING' CHECK(qc_state IN ('PENDING','HOLD','REINSPECT','ACCEPTED')),
 finishing_count integer NOT NULL DEFAULT 0 CHECK(finishing_count BETWEEN 0 AND 5),
 last_defect varchar(64) NOT NULL DEFAULT '',
 fail_inspector varchar(64) NOT NULL DEFAULT '',
 repair_actor varchar(64) NOT NULL DEFAULT '',
 last_inspector varchar(64) NOT NULL DEFAULT '',
 revision integer NOT NULL DEFAULT 0 CHECK(revision>=0),
 source_state varchar(48) NOT NULL DEFAULT 'SEWING_SIMULATION_NO_PRODUCTION_CREDIT'
   CHECK(source_state='SEWING_SIMULATION_NO_PRODUCTION_CREDIT'),
 updated_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(tenant_id,serial),
 FOREIGN KEY(tenant_id,lay_draft_id,marker_position) REFERENCES cut_lay_positions_v8(tenant_id,lay_draft_id,marker_position) ON DELETE RESTRICT
);
CREATE INDEX qc_sim_units_lay_v10 ON qc_sim_units_v10(tenant_id,lay_draft_id,plant_id,qc_state);
CREATE TABLE qc_sim_events_v10 (
 tenant_id varchar(64) NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
 event_id varchar(80) NOT NULL CHECK(event_id ~ '^[A-Za-z0-9_-]{12,80}$'),
 payload_sha256 char(64) NOT NULL CHECK(payload_sha256 ~ '^[0-9a-f]{64}$'),
 serial varchar(128) NOT NULL,
 action varchar(32) NOT NULL CHECK(action IN ('ENROLL','INSPECT_PASS','INSPECT_FAIL','REPAIR_COMPLETE','REINSPECT_PASS','REINSPECT_FAIL','FINISH_STEP','BIND_BUYER')),
 actor varchar(64) NOT NULL,
 station_id varchar(64) NOT NULL,
 detail varchar(128) NOT NULL DEFAULT '',
 reader_payload jsonb NOT NULL CHECK(jsonb_typeof(reader_payload)='object' AND reader_payload->>'version'='QC_READER_V1' AND reader_payload->>'source'='UI_SIMULATED' AND reader_payload->>'epc_hex' IS NULL AND reader_payload->>'rssi_dbm' IS NULL),
 prior_state varchar(24) NOT NULL,
 next_state varchar(24) NOT NULL,
 recorded_at timestamptz NOT NULL DEFAULT now(),
 provenance varchar(48) NOT NULL DEFAULT 'UI_SIMULATED_NO_RF_OBSERVATION'
   CHECK(provenance='UI_SIMULATED_NO_RF_OBSERVATION'),
 PRIMARY KEY(tenant_id,event_id),
 FOREIGN KEY(tenant_id,serial) REFERENCES qc_sim_units_v10(tenant_id,serial) ON DELETE RESTRICT
);
CREATE INDEX qc_sim_events_unit_v10 ON qc_sim_events_v10(tenant_id,serial,recorded_at);
CREATE TABLE qc_sim_buyer_bindings_v10 (
 tenant_id varchar(64) NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
 serial varchar(128) NOT NULL,
 buyer_code varchar(96) NOT NULL CHECK(length(buyer_code) BETWEEN 4 AND 96),
 style_code varchar(96) NOT NULL,
 size_code varchar(32) NOT NULL,
 event_id varchar(80) NOT NULL,
 evidence_state varchar(48) NOT NULL DEFAULT 'UNVERIFIED_SIMULATED_BUYER_BIND'
   CHECK(evidence_state='UNVERIFIED_SIMULATED_BUYER_BIND'),
 created_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(tenant_id,serial), UNIQUE(tenant_id,buyer_code),
 FOREIGN KEY(tenant_id,serial) REFERENCES qc_sim_units_v10(tenant_id,serial) ON DELETE RESTRICT,
 FOREIGN KEY(tenant_id,event_id) REFERENCES qc_sim_events_v10(tenant_id,event_id) ON DELETE RESTRICT
);
-- Source immutability: no invented garments, counts, colors, sizes or plants.
CREATE FUNCTION qc_sim_source_guard_v10() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE matched boolean;
BEGIN
 SELECT EXISTS(
   SELECT 1 FROM sew_sim_credits_v9 cr
   JOIN sew_sim_cycles_v9 cy ON cy.tenant_id=cr.tenant_id AND cy.id=cr.cycle_id AND cy.state='COMPLETED'
   JOIN cut_lay_positions_v8 lp ON lp.tenant_id=cr.tenant_id AND lp.lay_draft_id=cr.lay_draft_id AND lp.marker_position=cr.marker_position
   WHERE cr.tenant_id=NEW.tenant_id AND NEW.serial=ANY(cr.serials)
     AND cr.lay_draft_id=NEW.lay_draft_id AND cr.marker_position=NEW.marker_position
     AND cy.plant_id=NEW.plant_id AND lp.color_code=NEW.color_code AND lp.size_code=NEW.size_code
 ) INTO matched;
 IF NOT matched OR NEW.qc_state<>'PENDING' OR NEW.finishing_count<>0 OR NEW.revision<>0 THEN
  RAISE EXCEPTION 'ERR_QC_SIM_NO_SOURCE_GENEALOGY';
 END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER qc_sim_source_guard_v10 BEFORE INSERT ON qc_sim_units_v10
  FOR EACH ROW EXECUTE FUNCTION qc_sim_source_guard_v10();
-- Buyer bindings cannot be forged for held, uninspected or unfinished garments.
CREATE FUNCTION qc_sim_buyer_gate_v10() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE u qc_sim_units_v10%ROWTYPE; route_steps text[]; original_style varchar(64); valid_sku boolean;
BEGIN
 SELECT * INTO u FROM qc_sim_units_v10 WHERE tenant_id=NEW.tenant_id AND serial=NEW.serial FOR SHARE;
 IF NOT FOUND OR u.qc_state<>'ACCEPTED' THEN RAISE EXCEPTION 'ERR_QC_SIM_NOT_ACCEPTED'; END IF;
 SELECT steps INTO route_steps FROM qc_sim_routes_v10 WHERE tenant_id=NEW.tenant_id AND lay_draft_id=u.lay_draft_id AND plant_id=u.plant_id;
 IF NOT FOUND OR u.finishing_count<>cardinality(route_steps) THEN RAISE EXCEPTION 'ERR_QC_SIM_NOT_FINISHED'; END IF;
 SELECT o.style_code,EXISTS(SELECT 1 FROM order_intake_skus_v5 sk WHERE sk.tenant_id=o.tenant_id AND sk.intake_id=o.id AND sk.color_code=u.color_code AND sk.size_code=u.size_code)
   INTO original_style,valid_sku FROM cut_lay_drafts_v8 d JOIN order_intakes_v5 o ON o.tenant_id=d.tenant_id AND o.id=d.intake_id
   WHERE d.tenant_id=NEW.tenant_id AND d.id=u.lay_draft_id;
 IF NOT valid_sku OR NEW.style_code<>original_style OR NEW.size_code<>u.size_code THEN RAISE EXCEPTION 'ERR_QC_SIM_BUYER_SKU_MISMATCH'; END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER qc_sim_buyer_gate_v10 BEFORE INSERT ON qc_sim_buyer_bindings_v10
 FOR EACH ROW EXECUTE FUNCTION qc_sim_buyer_gate_v10();
-- Tenant pinning via the same docTransaction used by Batches 08/09.
DO $$ DECLARE tbl text; BEGIN
 FOREACH tbl IN ARRAY ARRAY['qc_sim_routes_v10','qc_sim_units_v10','qc_sim_events_v10','qc_sim_buyer_bindings_v10'] LOOP
  EXECUTE format('ALTER TABLE public.%I ENABLE ROW LEVEL SECURITY',tbl);
  EXECUTE format('ALTER TABLE public.%I FORCE ROW LEVEL SECURITY',tbl);
  EXECUTE format('CREATE POLICY qc_sim_tenant_v10 ON public.%I FOR ALL TO agiparel_app USING(tenant_id=NULLIF(current_setting(''app.current_tenant_id'',true),'''')) WITH CHECK(tenant_id=NULLIF(current_setting(''app.current_tenant_id'',true),''''))',tbl);
  EXECUTE format('REVOKE ALL ON public.%I FROM PUBLIC,agiparel_app',tbl);
  EXECUTE format('GRANT SELECT,INSERT ON public.%I TO agiparel_app',tbl);
 END LOOP;
END $$;
GRANT UPDATE(qc_state,finishing_count,last_defect,fail_inspector,repair_actor,last_inspector,revision,updated_at) ON qc_sim_units_v10 TO agiparel_app;
CREATE FUNCTION qc_sim_no_edit_v10() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN RAISE EXCEPTION 'ERR_QC_SIM_IMMUTABLE_RECORD'; END $$;
CREATE TRIGGER qc_sim_routes_immutable_v10 BEFORE UPDATE OR DELETE ON qc_sim_routes_v10 FOR EACH ROW EXECUTE FUNCTION qc_sim_no_edit_v10();
CREATE TRIGGER qc_sim_events_immutable_v10 BEFORE UPDATE OR DELETE ON qc_sim_events_v10 FOR EACH ROW EXECUTE FUNCTION qc_sim_no_edit_v10();
CREATE TRIGGER qc_sim_buyer_immutable_v10 BEFORE UPDATE OR DELETE ON qc_sim_buyer_bindings_v10 FOR EACH ROW EXECUTE FUNCTION qc_sim_no_edit_v10();
-- Block accidental mutation of identity/genealogy even by SQL callers.
CREATE FUNCTION qc_sim_unit_guard_v10() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF TG_OP='DELETE' THEN RAISE EXCEPTION 'ERR_QC_SIM_UNIT_DELETE'; END IF;
 IF NEW.tenant_id<>OLD.tenant_id OR NEW.serial<>OLD.serial OR NEW.lay_draft_id<>OLD.lay_draft_id OR
    NEW.plant_id<>OLD.plant_id OR NEW.marker_position<>OLD.marker_position OR
    NEW.color_code<>OLD.color_code OR NEW.size_code<>OLD.size_code OR NEW.source_state<>OLD.source_state OR
    NEW.revision<>OLD.revision+1 OR NEW.finishing_count<OLD.finishing_count OR
    NEW.finishing_count>OLD.finishing_count+1 THEN
  RAISE EXCEPTION 'ERR_QC_SIM_GENEALOGY_IMMUTABLE';
 END IF;
 IF NOT ((OLD.qc_state='PENDING' AND NEW.qc_state IN ('ACCEPTED','HOLD')) OR
  (OLD.qc_state='HOLD' AND NEW.qc_state='REINSPECT') OR
  (OLD.qc_state='REINSPECT' AND NEW.qc_state IN ('ACCEPTED','HOLD')) OR
  (OLD.qc_state='ACCEPTED' AND NEW.qc_state='ACCEPTED' AND NEW.finishing_count=OLD.finishing_count+1)) THEN
  RAISE EXCEPTION 'ERR_QC_SIM_INVALID_TRANSITION';
 END IF;
 IF OLD.qc_state<>'ACCEPTED' AND NEW.finishing_count<>OLD.finishing_count THEN
  RAISE EXCEPTION 'ERR_QC_SIM_FINISH_BEFORE_ACCEPT';
 END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER qc_sim_unit_guard_v10 BEFORE UPDATE OR DELETE ON qc_sim_units_v10 FOR EACH ROW EXECUTE FUNCTION qc_sim_unit_guard_v10();
COMMIT;
