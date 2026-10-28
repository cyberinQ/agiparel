-- Author: Sunil Jain, 2026-10-28.
-- Production sewing evidence admission. Separate from immutable simulation tables.
-- Prerequisites: 0019-0022 and signed cutting release 0021, PostgreSQL uuid_generate_v4().
-- Enrollment and operation approval are DBA-/provisioner-controlled, never operator-controlled.
BEGIN;
CREATE TABLE sew_prod_gateways_v10 (
 tenant_id varchar(64) NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
 gateway_id varchar(64) NOT NULL CHECK(gateway_id ~ '^[A-Za-z0-9_-]{3,64}$'),
 plant_id varchar(64) NOT NULL,
 station_id varchar(64) NOT NULL,
 public_key bytea NOT NULL CHECK(octet_length(public_key)=32),
 enabled boolean NOT NULL DEFAULT false,
 enrolled_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(tenant_id,gateway_id)
);
CREATE TABLE sew_prod_routes_v10 (
 tenant_id varchar(64) NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
 id uuid NOT NULL DEFAULT uuid_generate_v4(),
 lay_draft_id uuid NOT NULL,
 plant_id varchar(64) NOT NULL,
 operation_code varchar(64) NOT NULL CHECK(operation_code ~ '^[A-Za-z0-9_-]{2,64}$'),
 revision varchar(64) NOT NULL,
 required_parts text[] NOT NULL CHECK(cardinality(required_parts)>0),
 predecessors text[] NOT NULL DEFAULT '{}',
 station_id varchar(64) NOT NULL,
 approved_by varchar(128) NOT NULL,
 enabled boolean NOT NULL DEFAULT false,
 created_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(tenant_id,id), UNIQUE(tenant_id,lay_draft_id,plant_id,operation_code,revision),
 FOREIGN KEY(tenant_id,lay_draft_id) REFERENCES cut_lay_drafts_v8(tenant_id,id) ON DELETE RESTRICT
);
CREATE TABLE sew_prod_worker_stations_v10 (
 tenant_id varchar(64) NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
 plant_id varchar(64) NOT NULL,
 station_id varchar(64) NOT NULL,
 worker_subject varchar(128) NOT NULL,
 enabled boolean NOT NULL DEFAULT false,
 approved_by varchar(128) NOT NULL,
 approved_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(tenant_id,plant_id,station_id,worker_subject)
);
CREATE TABLE sew_prod_observations_v10 (
 tenant_id varchar(64) NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
 id uuid NOT NULL DEFAULT uuid_generate_v4(),
 event_id varchar(80) NOT NULL CHECK(event_id ~ '^[A-Za-z0-9_-]{12,80}$'),
 gateway_id varchar(64) NOT NULL,
 plant_id varchar(64) NOT NULL,
 station_id varchar(64) NOT NULL,
 lay_draft_id uuid NOT NULL,
 release_id uuid NOT NULL,
 label_code varchar(29) NOT NULL,
 epc_hex varchar(96) NOT NULL,
 marker_position integer NOT NULL,
 ply_group integer NOT NULL,
 part_code varchar(64) NOT NULL,
 observed_at timestamptz NOT NULL,
 received_at timestamptz NOT NULL DEFAULT now(),
 event_sha256 char(64) NOT NULL CHECK(event_sha256 ~ '^[0-9a-f]{64}$'),
 signature_sha256 char(64) NOT NULL CHECK(signature_sha256 ~ '^[0-9a-f]{64}$'),
 evidence_state varchar(64) NOT NULL DEFAULT 'SIGNED_GATEWAY_REPORT_NOT_INDEPENDENT_RF_PROOF'
  CHECK(evidence_state='SIGNED_GATEWAY_REPORT_NOT_INDEPENDENT_RF_PROOF'),
 PRIMARY KEY(tenant_id,id), UNIQUE(tenant_id,gateway_id,event_id),
 FOREIGN KEY(tenant_id,gateway_id) REFERENCES sew_prod_gateways_v10(tenant_id,gateway_id) ON DELETE RESTRICT,
 FOREIGN KEY(tenant_id,release_id) REFERENCES cut_sewing_releases_v8(tenant_id,id) ON DELETE RESTRICT,
 FOREIGN KEY(tenant_id,lay_draft_id,label_code) REFERENCES cut_bundle_drafts_v8(tenant_id,lay_draft_id,label_code) ON DELETE RESTRICT
);
CREATE INDEX sew_prod_observations_group_v10
 ON sew_prod_observations_v10(tenant_id,lay_draft_id,plant_id,station_id,marker_position,ply_group,received_at DESC);
CREATE TABLE sew_prod_completions_v10 (
 tenant_id varchar(64) NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
 id uuid NOT NULL DEFAULT uuid_generate_v4(),
 command_id varchar(80) NOT NULL CHECK(command_id ~ '^[A-Za-z0-9_-]{12,80}$'),
 command_sha256 char(64) NOT NULL CHECK(command_sha256 ~ '^[0-9a-f]{64}$'),
 lay_draft_id uuid NOT NULL,
 release_id uuid NOT NULL,
 route_id uuid NOT NULL,
 plant_id varchar(64) NOT NULL,
 station_id varchar(64) NOT NULL,
 marker_position integer NOT NULL,
 ply_group integer NOT NULL,
 observation_ids uuid[] NOT NULL CHECK(cardinality(observation_ids)>0),
 nominal_ply_count integer NOT NULL CHECK(nominal_ply_count>0),
 worker_subject varchar(128) NOT NULL,
 worker_proof_id varchar(255) NOT NULL,
 assertion_sha256 char(64) NOT NULL CHECK(assertion_sha256 ~ '^[0-9a-f]{64}$'),
 evidence_state varchar(64) NOT NULL DEFAULT 'WORKER_ATTESTED_PENDING_QUALITY_ACCEPTANCE'
  CHECK(evidence_state='WORKER_ATTESTED_PENDING_QUALITY_ACCEPTANCE'),
 created_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(tenant_id,id), UNIQUE(tenant_id,command_id), UNIQUE(tenant_id,worker_proof_id),
 UNIQUE(tenant_id,route_id,marker_position,ply_group),
 FOREIGN KEY(tenant_id,route_id) REFERENCES sew_prod_routes_v10(tenant_id,id) ON DELETE RESTRICT,
 FOREIGN KEY(tenant_id,release_id) REFERENCES cut_sewing_releases_v8(tenant_id,id) ON DELETE RESTRICT,
 FOREIGN KEY(tenant_id,lay_draft_id,marker_position) REFERENCES cut_lay_positions_v8(tenant_id,lay_draft_id,marker_position) ON DELETE RESTRICT
);
CREATE INDEX sew_prod_completions_history_v10 ON sew_prod_completions_v10(tenant_id,plant_id,created_at DESC,id DESC);
-- Physical observation is exclusively consumed by a single committed operation.
-- This UNIQUE key, not an app-level read/check, is the concurrent-write arbiter.
CREATE TABLE sew_prod_used_observations_v10 (
 tenant_id varchar(64) NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
 observation_id uuid NOT NULL,
 completion_id uuid NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(tenant_id,observation_id),
 FOREIGN KEY(tenant_id,observation_id) REFERENCES sew_prod_observations_v10(tenant_id,id) ON DELETE RESTRICT,
 FOREIGN KEY(tenant_id,completion_id) REFERENCES sew_prod_completions_v10(tenant_id,id) ON DELETE RESTRICT
);


-- Admission invariants also apply to direct SQL inserts. The server verifies
-- the cryptographic signature; the trigger enforces immutable signed-cut lineage.
CREATE FUNCTION guard_sew_prod_observation_v10() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE rel cut_sewing_releases_v8%ROWTYPE; matched integer;
BEGIN
 IF NOT EXISTS (
  SELECT 1 FROM sew_prod_gateways_v10 g
  WHERE g.tenant_id=NEW.tenant_id AND g.gateway_id=NEW.gateway_id
    AND g.enabled AND g.plant_id=NEW.plant_id AND g.station_id=NEW.station_id) THEN
   RAISE EXCEPTION 'ERR_SEW_PROD_GATEWAY_SCOPE';
 END IF;
 SELECT * INTO rel FROM cut_sewing_releases_v8
  WHERE tenant_id=NEW.tenant_id AND id=NEW.release_id;
 IF NOT FOUND OR rel.plant_id<>NEW.plant_id OR rel.lay_draft_id<>NEW.lay_draft_id OR
    NEW.observed_at < rel.created_at - interval '2 minutes' THEN
   RAISE EXCEPTION 'ERR_SEW_PROD_CUT_RELEASE';
 END IF;
 SELECT count(*)::int INTO matched FROM cut_bundle_drafts_v8 b
 JOIN cut_bundle_scan_witnesses_v8 w
   ON w.tenant_id=b.tenant_id AND w.lay_draft_id=b.lay_draft_id AND w.label_code=b.label_code
 WHERE b.tenant_id=NEW.tenant_id AND b.lay_draft_id=NEW.lay_draft_id
  AND b.label_code=NEW.label_code AND w.epc_hex=NEW.epc_hex
  AND w.spread_report_id=rel.spread_report_id AND w.plant_id=NEW.plant_id
  AND b.marker_position=NEW.marker_position AND b.ply_group=NEW.ply_group AND b.part_code=NEW.part_code;
 IF matched<>1 THEN RAISE EXCEPTION 'ERR_SEW_PROD_BUNDLE_LINEAGE'; END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER sew_prod_observation_guard_v10 BEFORE INSERT ON sew_prod_observations_v10
 FOR EACH ROW EXECUTE FUNCTION guard_sew_prod_observation_v10();

-- Defense in depth: SQL cannot manufacture a completion by referencing arbitrary
-- observations, simulation data, unreleased cutting, or foreign-plant parts.
CREATE FUNCTION guard_sew_prod_completion_v10() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE r sew_prod_routes_v10%ROWTYPE; rel cut_sewing_releases_v8%ROWTYPE;
    seen integer; unique_parts integer; starts integer; ends integer; colors integer; sizes integer;
    low_ply integer; high_ply integer; unmet integer; obs uuid;
BEGIN
 SELECT * INTO r FROM sew_prod_routes_v10 WHERE tenant_id=NEW.tenant_id AND id=NEW.route_id;
 IF NOT FOUND OR NOT r.enabled OR r.plant_id<>NEW.plant_id OR r.station_id<>NEW.station_id OR r.lay_draft_id<>NEW.lay_draft_id THEN
   RAISE EXCEPTION 'ERR_SEW_PROD_ROUTE_SCOPE';
 END IF;
 IF NOT EXISTS (SELECT 1 FROM sew_prod_worker_stations_v10 ws
   WHERE ws.tenant_id=NEW.tenant_id AND ws.plant_id=NEW.plant_id
     AND ws.station_id=NEW.station_id AND ws.worker_subject=NEW.worker_subject AND ws.enabled) THEN
   RAISE EXCEPTION 'ERR_SEW_PROD_WORKER_NOT_ASSIGNED';
 END IF;
 SELECT * INTO rel FROM cut_sewing_releases_v8 WHERE tenant_id=NEW.tenant_id AND id=NEW.release_id;
 IF NOT FOUND OR rel.plant_id<>NEW.plant_id OR rel.lay_draft_id<>NEW.lay_draft_id THEN
   RAISE EXCEPTION 'ERR_SEW_PROD_SIGNED_RELEASE';
 END IF;
 IF (SELECT count(DISTINCT x) FROM unnest(r.required_parts) x)<>cardinality(r.required_parts) OR
    (SELECT count(DISTINCT x) FROM unnest(NEW.observation_ids) x)<>cardinality(NEW.observation_ids) OR
    cardinality(NEW.observation_ids)<>cardinality(r.required_parts) THEN
   RAISE EXCEPTION 'ERR_SEW_PROD_PARTS';
 END IF;
 SELECT count(*)::int,count(DISTINCT o.part_code)::int,count(DISTINCT b.ply_start)::int,
   count(DISTINCT b.ply_end)::int,count(DISTINCT b.color_code)::int,count(DISTINCT b.size_code)::int,
   min(b.ply_start),max(b.ply_end)
 INTO seen,unique_parts,starts,ends,colors,sizes,low_ply,high_ply
 FROM unnest(NEW.observation_ids) ids(obs_id)
 JOIN sew_prod_observations_v10 o ON o.tenant_id=NEW.tenant_id AND o.id=ids.obs_id
 JOIN cut_bundle_drafts_v8 b ON b.tenant_id=o.tenant_id AND b.lay_draft_id=o.lay_draft_id AND b.label_code=o.label_code
 JOIN sew_prod_gateways_v10 g ON g.tenant_id=o.tenant_id AND g.gateway_id=o.gateway_id AND g.enabled
 WHERE o.lay_draft_id=NEW.lay_draft_id AND o.release_id=NEW.release_id AND
       o.plant_id=NEW.plant_id AND o.station_id=NEW.station_id AND
       o.marker_position=NEW.marker_position AND o.ply_group=NEW.ply_group AND
       o.part_code=ANY(r.required_parts) AND g.plant_id=NEW.plant_id AND g.station_id=NEW.station_id
       AND o.observed_at >= r.created_at - interval '2 minutes';
 IF seen<>cardinality(r.required_parts) OR unique_parts<>seen OR starts<>1 OR ends<>1 OR
    colors<>1 OR sizes<>1 OR NEW.nominal_ply_count<>high_ply-low_ply+1 THEN
   RAISE EXCEPTION 'ERR_SEW_PROD_INCOMPLETE_GENEALOGY';
 END IF;
 SELECT count(*)::int INTO unmet FROM unnest(r.predecessors) p(code)
 WHERE NOT EXISTS (
   SELECT 1 FROM sew_prod_routes_v10 pr JOIN sew_prod_completions_v10 c
     ON c.tenant_id=pr.tenant_id AND c.route_id=pr.id
   WHERE pr.tenant_id=NEW.tenant_id AND pr.lay_draft_id=NEW.lay_draft_id
     AND pr.plant_id=NEW.plant_id AND pr.operation_code=p.code AND pr.enabled
     AND c.marker_position=NEW.marker_position AND c.ply_group=NEW.ply_group);
 IF unmet>0 THEN RAISE EXCEPTION 'ERR_SEW_PROD_PREDECESSOR'; END IF;
 -- An RF observation is evidence for one operation at one station, not a
 -- reusable voucher. Deterministic per-observation locks avoid race/double use
 -- even when two different routes complete at the same time.
 FOR obs IN SELECT unnest(NEW.observation_ids) ORDER BY 1 LOOP
   PERFORM pg_advisory_xact_lock(hashtext(NEW.tenant_id), hashtext('SEW_OBS:'||obs::text));
 END LOOP;
 IF EXISTS (SELECT 1 FROM sew_prod_completions_v10 existing
   WHERE existing.tenant_id=NEW.tenant_id AND existing.observation_ids && NEW.observation_ids) THEN
   RAISE EXCEPTION 'ERR_SEW_PROD_OBSERVATION_ALREADY_CONSUMED';
 END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER sew_prod_completion_guard_v10 BEFORE INSERT ON sew_prod_completions_v10
 FOR EACH ROW EXECUTE FUNCTION guard_sew_prod_completion_v10();
CREATE FUNCTION reserve_sew_prod_observations_v10() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 INSERT INTO sew_prod_used_observations_v10(tenant_id,observation_id,completion_id)
 SELECT NEW.tenant_id,x,NEW.id FROM unnest(NEW.observation_ids) x;
 RETURN NEW;
END $$;
CREATE TRIGGER sew_prod_consume_observations_v10 AFTER INSERT ON sew_prod_completions_v10
 FOR EACH ROW EXECUTE FUNCTION reserve_sew_prod_observations_v10();

-- Gateway and route enrollment is performed only by privileged provisioning.
-- Application role may read enrollment and insert immutable observations/completions.
DO $$ DECLARE tbl text; BEGIN
 FOREACH tbl IN ARRAY ARRAY['sew_prod_gateways_v10','sew_prod_routes_v10','sew_prod_worker_stations_v10','sew_prod_observations_v10','sew_prod_completions_v10','sew_prod_used_observations_v10'] LOOP
  EXECUTE format('ALTER TABLE public.%I ENABLE ROW LEVEL SECURITY',tbl);
  EXECUTE format('ALTER TABLE public.%I FORCE ROW LEVEL SECURITY',tbl);
  EXECUTE format('CREATE POLICY sew_prod_tenant_v10 ON public.%I FOR ALL TO agiparel_app USING (tenant_id=NULLIF(current_setting(''app.current_tenant_id'',true),'''')) WITH CHECK (tenant_id=NULLIF(current_setting(''app.current_tenant_id'',true),''''))',tbl);
  EXECUTE format('REVOKE ALL ON public.%I FROM PUBLIC,agiparel_app',tbl);
  EXECUTE format('GRANT SELECT ON public.%I TO agiparel_app',tbl);
 END LOOP;
END $$;
GRANT INSERT ON sew_prod_observations_v10,sew_prod_completions_v10,sew_prod_used_observations_v10 TO agiparel_app;
CREATE TRIGGER sew_prod_observation_immutable_v10 BEFORE UPDATE OR DELETE ON sew_prod_observations_v10
 FOR EACH ROW EXECUTE FUNCTION protect_cut_draft_v8();
CREATE TRIGGER sew_prod_completion_immutable_v10 BEFORE UPDATE OR DELETE ON sew_prod_completions_v10
 FOR EACH ROW EXECUTE FUNCTION protect_cut_draft_v8();
CREATE TRIGGER sew_prod_used_observation_immutable_v10 BEFORE UPDATE OR DELETE ON sew_prod_used_observations_v10
 FOR EACH ROW EXECUTE FUNCTION protect_cut_draft_v8();
COMMIT;
