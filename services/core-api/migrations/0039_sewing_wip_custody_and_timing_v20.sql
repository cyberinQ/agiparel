-- Author: Sunil Jain, 2026-10-28.
-- F3.3: signed gateway observations + worker-attested cut-part custody transitions.
-- Provisional WIP evidence only: no sensor-guaranteed presence, sewing output,
-- stock, payroll, QC, carton or shipment credit. Provisioning is DBA-controlled.
BEGIN;
INSERT INTO workforce_chapter_actions_v17(chapter_id,permission,authority)
 VALUES(6,'sewing.transfer','SIGNED_WRITE') ON CONFLICT DO NOTHING;

CREATE TABLE wip_transfer_routes_v20 (
 tenant_id varchar(64) NOT NULL REFERENCES tenants(id),
 id uuid NOT NULL DEFAULT uuid_generate_v4(),
 plant_id varchar(64) NOT NULL,
 from_station varchar(64) NOT NULL,
 to_station varchar(64) NOT NULL,
 revision varchar(64) NOT NULL,
 approved_by varchar(128) NOT NULL,
 enabled boolean NOT NULL DEFAULT false,
 created_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(tenant_id,id),
 UNIQUE(tenant_id,plant_id,from_station,to_station,revision),
 CHECK(from_station<>to_station),
 CHECK(length(btrim(revision))>0)
);
CREATE TABLE wip_gateway_scans_v20 (
 tenant_id varchar(64) NOT NULL REFERENCES tenants(id),
 id uuid NOT NULL DEFAULT uuid_generate_v4(),
 plant_id varchar(64) NOT NULL,
 station_id varchar(64) NOT NULL,
 gateway_id varchar(64) NOT NULL,
 event_id varchar(80) NOT NULL CHECK(event_id ~ '^[A-Za-z0-9_-]{12,80}$'),
 part_id uuid NOT NULL,
 carrier_id uuid NOT NULL,
 binding_event_id uuid NOT NULL,
 epc_hex varchar(96) NOT NULL,
 observed_at timestamptz NOT NULL,
 received_at timestamptz NOT NULL DEFAULT now(),
 event_sha256 char(64) NOT NULL CHECK(event_sha256 ~ '^[0-9a-f]{64}$'),
 signature_sha256 char(64) NOT NULL CHECK(signature_sha256 ~ '^[0-9a-f]{64}$'),
 evidence text NOT NULL DEFAULT 'SIGNED_GATEWAY_REPORT_NOT_INDEPENDENT_RF_PROOF'
  CHECK(evidence='SIGNED_GATEWAY_REPORT_NOT_INDEPENDENT_RF_PROOF'),
 PRIMARY KEY(tenant_id,id),
 UNIQUE(tenant_id,gateway_id,event_id),
 FOREIGN KEY(tenant_id,gateway_id) REFERENCES sew_prod_gateways_v10(tenant_id,gateway_id),
 FOREIGN KEY(tenant_id,carrier_id) REFERENCES gen_carriers_v18(tenant_id,id),
 FOREIGN KEY(tenant_id,binding_event_id) REFERENCES gen_carrier_events_v18(tenant_id,id),
 FOREIGN KEY(tenant_id,part_id) REFERENCES gen_cut_parts_v18(tenant_id,id)
);
CREATE INDEX wip_scans_part_v20 ON wip_gateway_scans_v20(tenant_id,plant_id,part_id,received_at DESC);

CREATE TABLE wip_movements_v20 (
 tenant_id varchar(64) NOT NULL REFERENCES tenants(id),
 id uuid NOT NULL DEFAULT uuid_generate_v4(),
 event_no bigint NOT NULL CHECK(event_no>0),
 plant_id varchar(64) NOT NULL,
 part_id uuid NOT NULL,
 route_id uuid NOT NULL,
 scan_id uuid NOT NULL,
 action varchar(8) NOT NULL CHECK(action IN('DEPART','ARRIVE')),
 station_id varchar(64) NOT NULL,
 command_id varchar(80) NOT NULL CHECK(command_id ~ '^[A-Za-z0-9_-]{12,80}$'),
 command_sha256 char(64) NOT NULL CHECK(command_sha256 ~ '^[0-9a-f]{64}$'),
 worker_subject varchar(128) NOT NULL,
 worker_proof_id varchar(255) NOT NULL,
 assertion_sha256 char(64) NOT NULL CHECK(assertion_sha256 ~ '^[0-9a-f]{64}$'),
 recorded_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 evidence text NOT NULL DEFAULT 'SIGNED_SCAN_AND_WORKER_CUSTODY_ATTESTATION'
  CHECK(evidence='SIGNED_SCAN_AND_WORKER_CUSTODY_ATTESTATION'),
 PRIMARY KEY(tenant_id,id),
 UNIQUE(tenant_id,part_id,event_no), UNIQUE(tenant_id,command_id),
 UNIQUE(tenant_id,worker_proof_id), UNIQUE(tenant_id,scan_id),
 FOREIGN KEY(tenant_id,part_id) REFERENCES gen_cut_parts_v18(tenant_id,id),
 FOREIGN KEY(tenant_id,route_id) REFERENCES wip_transfer_routes_v20(tenant_id,id),
 FOREIGN KEY(tenant_id,scan_id) REFERENCES wip_gateway_scans_v20(tenant_id,id)
);
CREATE INDEX wip_movements_part_v20 ON wip_movements_v20(tenant_id,plant_id,part_id,event_no DESC);
CREATE INDEX wip_movements_metrics_v20 ON wip_movements_v20(tenant_id,plant_id,route_id,recorded_at) WHERE action='ARRIVE';

-- Each scan is signed by an *enrolled* gateway; SQL validates that it was
-- referencing an actively bound carrier and an independently accepted cut part.
CREATE FUNCTION guard_wip_scan_v20() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE g gen_garments_v18%ROWTYPE;
BEGIN
 PERFORM pg_advisory_xact_lock(hashtext(NEW.tenant_id),hashtext('RFID_EPC_GLOBAL_V8'));
 SELECT garment.* INTO g FROM gen_cut_parts_v18 p JOIN gen_garments_v18 garment
  ON garment.tenant_id=p.tenant_id AND garment.id=p.garment_id
  WHERE p.tenant_id=NEW.tenant_id AND p.id=NEW.part_id;
 IF NOT FOUND OR g.plant_id<>NEW.plant_id THEN RAISE EXCEPTION 'ERR_WIP_PART_PLANT'; END IF;
 IF NOT EXISTS(SELECT 1 FROM cut_part_outcomes_v19 o WHERE o.tenant_id=NEW.tenant_id
    AND o.part_id=NEW.part_id AND o.disposition='ACCEPTED')
 OR EXISTS(SELECT 1 FROM cut_recut_links_v19 l WHERE l.tenant_id=NEW.tenant_id AND l.original_part_id=NEW.part_id)
 THEN RAISE EXCEPTION 'ERR_WIP_PART_NOT_ACCEPTED'; END IF;
 IF NOT EXISTS(SELECT 1 FROM sew_prod_gateways_v10 gw WHERE gw.tenant_id=NEW.tenant_id
  AND gw.gateway_id=NEW.gateway_id AND gw.plant_id=NEW.plant_id
  AND gw.station_id=NEW.station_id AND gw.enabled) THEN
  RAISE EXCEPTION 'ERR_WIP_GATEWAY_NOT_ENROLLED'; END IF;
 SELECT a.bound_event_id INTO NEW.binding_event_id FROM gen_carriers_v18 c JOIN gen_carrier_active_v18 a
   ON c.tenant_id=a.tenant_id AND c.id=a.carrier_id
   WHERE c.tenant_id=NEW.tenant_id AND c.id=NEW.carrier_id
   AND c.enabled AND c.epc_hex=NEW.epc_hex AND a.plant_id=NEW.plant_id
   AND a.target_type='CUT_PART' AND a.target_id=NEW.part_id;
 IF NEW.binding_event_id IS NULL THEN RAISE EXCEPTION 'ERR_WIP_CARRIER_NOT_BOUND'; END IF;
 IF NEW.observed_at>clock_timestamp()+interval '2 minutes' THEN
   RAISE EXCEPTION 'ERR_WIP_SCAN_FUTURE'; END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER wip_scan_guard_v20 BEFORE INSERT ON wip_gateway_scans_v20
 FOR EACH ROW EXECUTE FUNCTION guard_wip_scan_v20();

-- SQL enforces route and station membership as well as coherent chronological
-- DEPART/ARRIVE sequencing under per-part transaction locks. The app ALSO
-- checks Ed25519 and FIDO2 JWT cryptography before INSERT.
CREATE FUNCTION guard_wip_move_v20() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE route wip_transfer_routes_v20%ROWTYPE; scan wip_gateway_scans_v20%ROWTYPE;
 prior wip_movements_v20%ROWTYPE;
BEGIN
 PERFORM pg_advisory_xact_lock(hashtext(NEW.tenant_id),hashtext('WIP:PART:'||NEW.part_id::text));
 -- Sequence and receipt time are allocated only AFTER the per-part lock.
 -- Global identity sequences assigned before lock would reorder concurrent moves.
 NEW.event_no := (SELECT COALESCE(max(event_no),0)+1 FROM wip_movements_v20
  WHERE tenant_id=NEW.tenant_id AND part_id=NEW.part_id);
 NEW.recorded_at := clock_timestamp();
 PERFORM pg_advisory_xact_lock(hashtext(NEW.tenant_id),hashtext('RFID_EPC_GLOBAL_V8'));
 SELECT * INTO route FROM wip_transfer_routes_v20 WHERE tenant_id=NEW.tenant_id AND id=NEW.route_id;
 IF NOT FOUND OR NOT route.enabled OR route.plant_id<>NEW.plant_id THEN RAISE EXCEPTION 'ERR_WIP_ROUTE'; END IF;
 SELECT * INTO scan FROM wip_gateway_scans_v20 WHERE tenant_id=NEW.tenant_id AND id=NEW.scan_id;
 IF NOT FOUND OR scan.part_id<>NEW.part_id OR scan.plant_id<>NEW.plant_id OR scan.station_id<>NEW.station_id
    OR scan.received_at>NEW.recorded_at OR scan.observed_at>NEW.recorded_at+interval '2 minutes' THEN
   RAISE EXCEPTION 'ERR_WIP_SCAN_MISMATCH'; END IF;
 IF NOT EXISTS(SELECT 1 FROM cut_part_outcomes_v19 o WHERE o.tenant_id=NEW.tenant_id AND o.part_id=NEW.part_id AND o.disposition='ACCEPTED') THEN RAISE EXCEPTION 'ERR_WIP_PART_NOT_ACCEPTED'; END IF;
 IF NOT EXISTS(SELECT 1 FROM gen_carrier_active_v18 a WHERE a.tenant_id=NEW.tenant_id
   AND a.carrier_id=scan.carrier_id AND a.bound_event_id=scan.binding_event_id
   AND a.target_type='CUT_PART' AND a.target_id=NEW.part_id
   AND a.plant_id=NEW.plant_id) THEN RAISE EXCEPTION 'ERR_WIP_CARRIER_RETIRED'; END IF;
 IF NOT EXISTS(SELECT 1 FROM sew_prod_worker_stations_v10 w WHERE w.tenant_id=NEW.tenant_id
   AND w.plant_id=NEW.plant_id AND w.station_id=NEW.station_id
   AND w.worker_subject=NEW.worker_subject AND w.enabled) THEN RAISE EXCEPTION 'ERR_WIP_WORKER_STATION'; END IF;
 SELECT * INTO prior FROM wip_movements_v20 WHERE tenant_id=NEW.tenant_id AND part_id=NEW.part_id
  ORDER BY event_no DESC LIMIT 1;
 IF NEW.action='DEPART' THEN
  IF NEW.station_id<>route.from_station OR (prior.id IS NOT NULL AND
      (prior.action<>'ARRIVE' OR prior.station_id<>route.from_station)) THEN
      RAISE EXCEPTION 'ERR_WIP_DEPART_STATE'; END IF;
 ELSE
  IF NEW.station_id<>route.to_station OR prior.id IS NULL OR prior.action<>'DEPART' OR
     prior.route_id<>NEW.route_id OR prior.station_id<>route.from_station OR
     prior.worker_subject=NEW.worker_subject OR NEW.recorded_at<prior.recorded_at THEN
     RAISE EXCEPTION 'ERR_WIP_ARRIVE_STATE'; END IF;
 END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER wip_move_guard_v20 BEFORE INSERT ON wip_movements_v20
 FOR EACH ROW EXECUTE FUNCTION guard_wip_move_v20();

DO $$ DECLARE tbl text; BEGIN
 FOREACH tbl IN ARRAY ARRAY['wip_transfer_routes_v20','wip_gateway_scans_v20','wip_movements_v20'] LOOP
  EXECUTE format('ALTER TABLE public.%I ENABLE ROW LEVEL SECURITY',tbl);
  EXECUTE format('ALTER TABLE public.%I FORCE ROW LEVEL SECURITY',tbl);
  EXECUTE format('CREATE POLICY wip_tenant_v20 ON public.%I FOR ALL TO agiparel_app USING (tenant_id=NULLIF(current_setting(''app.current_tenant_id'',true),'''')) WITH CHECK(tenant_id=NULLIF(current_setting(''app.current_tenant_id'',true),''''))',tbl);
  EXECUTE format('REVOKE ALL ON public.%I FROM PUBLIC,agiparel_app',tbl);
  EXECUTE format('GRANT SELECT ON public.%I TO agiparel_app',tbl);
 END LOOP;
END $$;
GRANT INSERT ON wip_gateway_scans_v20,wip_movements_v20 TO agiparel_app;
DO $$ DECLARE tbl text; BEGIN
 FOREACH tbl IN ARRAY ARRAY['wip_gateway_scans_v20','wip_movements_v20'] LOOP
  EXECUTE format('CREATE TRIGGER wip_immutable_v20 BEFORE UPDATE OR DELETE ON public.%I FOR EACH ROW EXECUTE FUNCTION protect_cut_draft_v8()',tbl);
 END LOOP;
END $$;
COMMIT;
