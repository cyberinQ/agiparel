-- CyberInQ / Agiparel E11. Rehearsal only. Requires 0023.
BEGIN;
CREATE TABLE pack_sim_cartons_v11 (
 tenant_id varchar(64) NOT NULL REFERENCES tenants(id), carton_code varchar(64) NOT NULL,
 lay_draft_id uuid NOT NULL, plant_id varchar(64) NOT NULL,
 state varchar(16) NOT NULL DEFAULT 'OPEN' CHECK(state IN ('OPEN','SEALED')),
 created_at timestamptz NOT NULL DEFAULT now(), sealed_at timestamptz,
 provenance varchar(48) NOT NULL DEFAULT 'UI_SIMULATED_NO_RF_OBSERVATION' CHECK(provenance='UI_SIMULATED_NO_RF_OBSERVATION'),
 PRIMARY KEY(tenant_id,carton_code),
 FOREIGN KEY(tenant_id,lay_draft_id) REFERENCES cut_lay_drafts_v8(tenant_id,id)
);
CREATE TABLE pack_sim_items_v11 (
 tenant_id varchar(64) NOT NULL, serial varchar(128) NOT NULL,
 carton_code varchar(64) NOT NULL, buyer_code varchar(96) NOT NULL,
 added_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(tenant_id,serial), UNIQUE(tenant_id,buyer_code),
 FOREIGN KEY(tenant_id,carton_code) REFERENCES pack_sim_cartons_v11(tenant_id,carton_code),
 FOREIGN KEY(tenant_id,serial) REFERENCES qc_sim_buyer_bindings_v10(tenant_id,serial)
);
CREATE INDEX pack_sim_items_carton_v11 ON pack_sim_items_v11(tenant_id,carton_code);
CREATE TABLE pack_sim_manifests_v11 (
 tenant_id varchar(64) NOT NULL REFERENCES tenants(id), manifest_code varchar(64) NOT NULL,
 plant_id varchar(64) NOT NULL, state varchar(24) NOT NULL DEFAULT 'DRAFT' CHECK(state IN ('DRAFT','DISPATCH_REHEARSAL')),
 created_at timestamptz NOT NULL DEFAULT now(), dispatched_at timestamptz,
 PRIMARY KEY(tenant_id,manifest_code)
);
CREATE TABLE pack_sim_manifest_cartons_v11 (
 tenant_id varchar(64) NOT NULL, manifest_code varchar(64) NOT NULL, carton_code varchar(64) NOT NULL,
 item_count integer NOT NULL CHECK(item_count>0),
 PRIMARY KEY(tenant_id,carton_code),
 FOREIGN KEY(tenant_id,manifest_code) REFERENCES pack_sim_manifests_v11(tenant_id,manifest_code),
 FOREIGN KEY(tenant_id,carton_code) REFERENCES pack_sim_cartons_v11(tenant_id,carton_code)
);
CREATE TABLE pack_sim_events_v11 (
 tenant_id varchar(64) NOT NULL REFERENCES tenants(id), event_id varchar(80) NOT NULL CHECK(event_id ~ '^[A-Za-z0-9_-]{12,80}$'),
 payload_sha256 char(64) NOT NULL, action varchar(32) NOT NULL CHECK(action IN ('OPEN_CARTON','ADD_GARMENT','REMOVE_GARMENT','SEAL_CARTON','OPEN_MANIFEST','LOAD_CARTON','DISPATCH_REHEARSAL')),
 carton_code varchar(64) NOT NULL DEFAULT '', manifest_code varchar(64) NOT NULL DEFAULT '', serial varchar(128) NOT NULL DEFAULT '',
 actor varchar(64) NOT NULL, station_id varchar(64) NOT NULL,
 reader_payload jsonb NOT NULL CHECK(jsonb_typeof(reader_payload)='object' AND reader_payload->>'version'='PACK_READER_V1' AND reader_payload->>'source'='UI_SIMULATED' AND reader_payload->>'epc_hex' IS NULL AND reader_payload->>'rssi_dbm' IS NULL),
 recorded_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(tenant_id,event_id)
);
-- Prevent direct SQL packing of held/unbound/unfininished garments.
CREATE FUNCTION pack_sim_item_guard_v11() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE u qc_sim_units_v10%ROWTYPE; b qc_sim_buyer_bindings_v10%ROWTYPE; c pack_sim_cartons_v11%ROWTYPE; steps text[];
BEGIN
 SELECT * INTO c FROM pack_sim_cartons_v11 WHERE tenant_id=NEW.tenant_id AND carton_code=NEW.carton_code FOR UPDATE;
 SELECT * INTO u FROM qc_sim_units_v10 WHERE tenant_id=NEW.tenant_id AND serial=NEW.serial FOR SHARE;
 SELECT * INTO b FROM qc_sim_buyer_bindings_v10 WHERE tenant_id=NEW.tenant_id AND serial=NEW.serial;
 SELECT r.steps INTO steps FROM qc_sim_routes_v10 r WHERE r.tenant_id=u.tenant_id AND r.lay_draft_id=u.lay_draft_id AND r.plant_id=u.plant_id;
 IF c.state<>'OPEN' OR u.qc_state<>'ACCEPTED' OR steps IS NULL OR u.finishing_count<>cardinality(steps) OR
    b.buyer_code IS NULL OR b.buyer_code<>NEW.buyer_code OR c.lay_draft_id<>u.lay_draft_id OR c.plant_id<>u.plant_id THEN
    RAISE EXCEPTION 'ERR_PACK_SIM_GARMENT_NOT_ELIGIBLE';
 END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER pack_sim_item_guard_v11 BEFORE INSERT ON pack_sim_items_v11 FOR EACH ROW EXECUTE FUNCTION pack_sim_item_guard_v11();
CREATE FUNCTION pack_sim_item_delete_guard_v11() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE state_now varchar(16);
BEGIN
 SELECT state INTO state_now FROM pack_sim_cartons_v11 WHERE tenant_id=OLD.tenant_id AND carton_code=OLD.carton_code FOR UPDATE;
 IF state_now<>'OPEN' THEN RAISE EXCEPTION 'ERR_PACK_SIM_SEALED_CARTON_IMMUTABLE'; END IF;
 RETURN OLD;
END $$;
CREATE TRIGGER pack_sim_item_delete_guard_v11 BEFORE DELETE ON pack_sim_items_v11 FOR EACH ROW EXECUTE FUNCTION pack_sim_item_delete_guard_v11();
CREATE FUNCTION pack_sim_manifest_guard_v11() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE c pack_sim_cartons_v11%ROWTYPE; m pack_sim_manifests_v11%ROWTYPE; actual integer;
BEGIN
 SELECT * INTO c FROM pack_sim_cartons_v11 WHERE tenant_id=NEW.tenant_id AND carton_code=NEW.carton_code FOR UPDATE;
 SELECT * INTO m FROM pack_sim_manifests_v11 WHERE tenant_id=NEW.tenant_id AND manifest_code=NEW.manifest_code FOR UPDATE;
 SELECT count(*)::integer INTO actual FROM pack_sim_items_v11 WHERE tenant_id=NEW.tenant_id AND carton_code=NEW.carton_code;
 IF c.state<>'SEALED' OR m.state<>'DRAFT' OR c.plant_id<>m.plant_id OR actual<>NEW.item_count OR actual=0 THEN
   RAISE EXCEPTION 'ERR_PACK_SIM_MANIFEST_CONSERVATION';
 END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER pack_sim_manifest_guard_v11 BEFORE INSERT ON pack_sim_manifest_cartons_v11 FOR EACH ROW EXECUTE FUNCTION pack_sim_manifest_guard_v11();
CREATE FUNCTION pack_sim_immutable_v11() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'ERR_PACK_SIM_IMMUTABLE'; END $$;
CREATE TRIGGER pack_sim_events_immutable_v11 BEFORE UPDATE OR DELETE ON pack_sim_events_v11 FOR EACH ROW EXECUTE FUNCTION pack_sim_immutable_v11();
CREATE TRIGGER pack_sim_manifest_cartons_immutable_v11 BEFORE UPDATE OR DELETE ON pack_sim_manifest_cartons_v11 FOR EACH ROW EXECUTE FUNCTION pack_sim_immutable_v11();
CREATE FUNCTION pack_sim_carton_state_v11() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF NEW.tenant_id<>OLD.tenant_id OR NEW.carton_code<>OLD.carton_code OR NEW.lay_draft_id<>OLD.lay_draft_id OR NEW.plant_id<>OLD.plant_id OR
  OLD.state<>'OPEN' OR NEW.state<>'SEALED' OR NEW.sealed_at IS NULL THEN RAISE EXCEPTION 'ERR_PACK_SIM_CARTON_TRANSITION'; END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER pack_sim_carton_state_v11 BEFORE UPDATE ON pack_sim_cartons_v11 FOR EACH ROW EXECUTE FUNCTION pack_sim_carton_state_v11();
CREATE FUNCTION pack_sim_manifest_state_v11() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF NEW.tenant_id<>OLD.tenant_id OR NEW.manifest_code<>OLD.manifest_code OR NEW.plant_id<>OLD.plant_id OR
  OLD.state<>'DRAFT' OR NEW.state<>'DISPATCH_REHEARSAL' OR NEW.dispatched_at IS NULL THEN RAISE EXCEPTION 'ERR_PACK_SIM_MANIFEST_TRANSITION'; END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER pack_sim_manifest_state_v11 BEFORE UPDATE ON pack_sim_manifests_v11 FOR EACH ROW EXECUTE FUNCTION pack_sim_manifest_state_v11();
DO $$ DECLARE t text; BEGIN
 FOREACH t IN ARRAY ARRAY['pack_sim_cartons_v11','pack_sim_items_v11','pack_sim_manifests_v11','pack_sim_manifest_cartons_v11','pack_sim_events_v11'] LOOP
 EXECUTE format('ALTER TABLE public.%I ENABLE ROW LEVEL SECURITY',t);
 EXECUTE format('ALTER TABLE public.%I FORCE ROW LEVEL SECURITY',t);
 EXECUTE format('CREATE POLICY pack_sim_tenant_v11 ON public.%I FOR ALL TO agiparel_app USING (tenant_id=NULLIF(current_setting(''app.current_tenant_id'',true),'''')) WITH CHECK (tenant_id=NULLIF(current_setting(''app.current_tenant_id'',true),''''))',t);
 EXECUTE format('REVOKE ALL ON public.%I FROM PUBLIC,agiparel_app',t);
 EXECUTE format('GRANT SELECT,INSERT ON public.%I TO agiparel_app',t);
 END LOOP;
END $$;
GRANT DELETE ON pack_sim_items_v11 TO agiparel_app;
GRANT UPDATE(state,sealed_at) ON pack_sim_cartons_v11 TO agiparel_app;
GRANT UPDATE(state,dispatched_at) ON pack_sim_manifests_v11 TO agiparel_app;
COMMIT;
