-- Author: Sunil Jain, 2026-10-28.
-- F3.5: signed unit-level carton and staged-manifest genealogy. Not a shipping event.
-- Human-attested weight is NOT an independently calibrated scale measurement.
BEGIN;
INSERT INTO workforce_chapter_actions_v17(chapter_id,permission,authority) VALUES
 (8,'packing.pack','SIGNED_WRITE'),(8,'packing.seal','SIGNED_WRITE'),
 (8,'packing.manifest','SIGNED_WRITE'),(8,'packing.dispatch_stage','SIGNED_WRITE')
 ON CONFLICT DO NOTHING;

-- Privileged provisioning only. Each packing plan is tied to a signed lay and
-- immutable source document; size-weight rows are completed before activation.
CREATE TABLE pack_carton_specs_v22 (
 tenant_id varchar(64) NOT NULL REFERENCES tenants(id), id uuid NOT NULL DEFAULT uuid_generate_v4(),
 plant_id varchar(64) NOT NULL, lay_draft_id uuid NOT NULL,
 revision varchar(64) NOT NULL, source_document_id varchar(36) NOT NULL,
 source_sha256 char(64) NOT NULL CHECK(source_sha256 ~ '^[0-9a-f]{64}$'),
 min_units integer NOT NULL CHECK(min_units>0), max_units integer NOT NULL CHECK(max_units>=min_units),
 tare_g bigint NOT NULL CHECK(tare_g>=0), tolerance_g bigint NOT NULL CHECK(tolerance_g>=0),
 enabled boolean NOT NULL DEFAULT false, approved_by varchar(128) NOT NULL,
 approved_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(tenant_id,id), UNIQUE(tenant_id,lay_draft_id,revision),
 FOREIGN KEY(tenant_id,lay_draft_id) REFERENCES cut_lay_drafts_v8(tenant_id,id),
 FOREIGN KEY(tenant_id,source_document_id) REFERENCES document_sources_v65(tenant_id,id),
 CHECK(length(btrim(revision))>0 AND length(btrim(approved_by))>0)
);
CREATE UNIQUE INDEX pack_one_active_spec_v22 ON pack_carton_specs_v22(tenant_id,lay_draft_id) WHERE enabled;
CREATE TABLE pack_size_weights_v22 (
 tenant_id varchar(64) NOT NULL, spec_id uuid NOT NULL, size_code varchar(32) NOT NULL,
 expected_g bigint NOT NULL CHECK(expected_g>0),
 PRIMARY KEY(tenant_id,spec_id,size_code),
 FOREIGN KEY(tenant_id,spec_id) REFERENCES pack_carton_specs_v22(tenant_id,id)
);

CREATE FUNCTION pack_spec_insert_guard_v22() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE digest char(64);
BEGIN
 IF NEW.enabled THEN RAISE EXCEPTION 'ERR_F35_SPEC_ACTIVATE_AFTER_WEIGHTS'; END IF;
 SELECT sha256 INTO digest FROM document_sources_v65 WHERE tenant_id=NEW.tenant_id AND id=NEW.source_document_id;
 IF digest IS NULL OR digest<>NEW.source_sha256 THEN RAISE EXCEPTION 'ERR_F35_SOURCE_MISMATCH'; END IF;
 IF NOT EXISTS(SELECT 1 FROM cut_sewing_releases_v8 r WHERE r.tenant_id=NEW.tenant_id
   AND r.plant_id=NEW.plant_id AND r.lay_draft_id=NEW.lay_draft_id) THEN
  RAISE EXCEPTION 'ERR_F35_SIGNED_CUT_LAY_REQUIRED'; END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER pack_spec_insert_v22 BEFORE INSERT ON pack_carton_specs_v22
 FOR EACH ROW EXECUTE FUNCTION pack_spec_insert_guard_v22();
CREATE FUNCTION pack_spec_update_guard_v22() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE missing_sizes bigint;
BEGIN
 PERFORM pg_advisory_xact_lock(hashtext(OLD.tenant_id),hashtext('F35:SPEC:'||OLD.id::text));
 IF TG_OP='DELETE' OR (to_jsonb(OLD)-'enabled')<>(to_jsonb(NEW)-'enabled') THEN
  RAISE EXCEPTION 'ERR_F35_SPEC_IMMUTABLE'; END IF;
 IF OLD.enabled AND NOT NEW.enabled THEN RETURN NEW; END IF;
 IF NOT OLD.enabled AND NEW.enabled THEN
  SELECT count(*) INTO missing_sizes FROM (
   SELECT DISTINCT size_code FROM cut_lay_positions_v8 WHERE tenant_id=OLD.tenant_id AND lay_draft_id=OLD.lay_draft_id
  ) sizes WHERE NOT EXISTS(SELECT 1 FROM pack_size_weights_v22 w WHERE w.tenant_id=OLD.tenant_id
   AND w.spec_id=OLD.id AND w.size_code=sizes.size_code);
  IF missing_sizes<>0 OR NOT EXISTS(SELECT 1 FROM pack_size_weights_v22 w WHERE w.tenant_id=OLD.tenant_id AND w.spec_id=OLD.id)
    THEN RAISE EXCEPTION 'ERR_F35_SIZE_WEIGHT_RULES_INCOMPLETE'; END IF;
  RETURN NEW;
 END IF;
 RAISE EXCEPTION 'ERR_F35_SPEC_IMMUTABLE';
END $$;
CREATE TRIGGER pack_spec_update_v22 BEFORE UPDATE OR DELETE ON pack_carton_specs_v22
 FOR EACH ROW EXECUTE FUNCTION pack_spec_update_guard_v22();
CREATE FUNCTION pack_size_rule_guard_v22() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE spec pack_carton_specs_v22%ROWTYPE;
BEGIN
 PERFORM pg_advisory_xact_lock(hashtext(NEW.tenant_id),hashtext('F35:SPEC:'||NEW.spec_id::text));
 SELECT * INTO spec FROM pack_carton_specs_v22 WHERE tenant_id=NEW.tenant_id AND id=NEW.spec_id;
 IF spec.id IS NULL OR spec.enabled OR NOT EXISTS(SELECT 1 FROM cut_lay_positions_v8 p
   WHERE p.tenant_id=spec.tenant_id AND p.lay_draft_id=spec.lay_draft_id AND p.size_code=NEW.size_code) THEN
  RAISE EXCEPTION 'ERR_F35_RULE_REQUIRES_DRAFT_AND_VALID_SIZE'; END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER pack_size_rule_insert_v22 BEFORE INSERT ON pack_size_weights_v22
 FOR EACH ROW EXECUTE FUNCTION pack_size_rule_guard_v22();
CREATE TRIGGER pack_size_rule_immutable_v22 BEFORE UPDATE OR DELETE ON pack_size_weights_v22
 FOR EACH ROW EXECUTE FUNCTION protect_cut_draft_v8();

-- No mutable carton projection: each OPEN, ADD and SEAL is an immutable signed event.
CREATE TABLE pack_carton_events_v22 (
 tenant_id varchar(64) NOT NULL REFERENCES tenants(id), id uuid NOT NULL DEFAULT uuid_generate_v4(),
 plant_id varchar(64) NOT NULL, carton_id uuid NOT NULL, event_no bigint NOT NULL CHECK(event_no>0),
 action varchar(12) NOT NULL CHECK(action IN ('OPEN','ADD','SEAL')),
 spec_id uuid NOT NULL, carton_code varchar(80), garment_id uuid,
 observed_g bigint CHECK(observed_g>=0), expected_g bigint CHECK(expected_g>=0),
 scale_reference varchar(128), note text NOT NULL CHECK(length(btrim(note)) BETWEEN 12 AND 1000),
 command_id varchar(80) NOT NULL CHECK(command_id ~ '^[A-Za-z0-9_-]{12,80}$'),
 command_sha256 char(64) NOT NULL CHECK(command_sha256 ~ '^[0-9a-f]{64}$'),
 actor_subject varchar(128) NOT NULL, proof_id varchar(255) NOT NULL,
 assertion_sha256 char(64) NOT NULL CHECK(assertion_sha256 ~ '^[0-9a-f]{64}$'),
 recorded_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 evidence text NOT NULL DEFAULT 'SIGNED_OPERATOR_PACKING_WEIGHT_NOT_CALIBRATED_NOT_SHIPPED'
  CHECK(evidence='SIGNED_OPERATOR_PACKING_WEIGHT_NOT_CALIBRATED_NOT_SHIPPED'),
 PRIMARY KEY(tenant_id,id), UNIQUE(tenant_id,carton_id,event_no),
 UNIQUE(tenant_id,command_id), UNIQUE(tenant_id,proof_id),
 FOREIGN KEY(tenant_id,spec_id) REFERENCES pack_carton_specs_v22(tenant_id,id),
 FOREIGN KEY(tenant_id,garment_id) REFERENCES gen_garments_v18(tenant_id,id),
 CHECK((action='OPEN' AND carton_code IS NOT NULL AND garment_id IS NULL AND observed_g IS NULL AND expected_g IS NULL AND scale_reference IS NULL)
    OR (action='ADD' AND carton_code IS NULL AND garment_id IS NOT NULL AND observed_g IS NULL AND expected_g IS NULL AND scale_reference IS NULL)
    OR (action='SEAL' AND carton_code IS NULL AND garment_id IS NULL AND observed_g>0 AND expected_g>0 AND scale_reference IS NOT NULL))
);
CREATE UNIQUE INDEX pack_unique_carton_code_v22 ON pack_carton_events_v22(tenant_id,plant_id,carton_code) WHERE action='OPEN';
CREATE UNIQUE INDEX pack_one_carton_per_garment_v22 ON pack_carton_events_v22(tenant_id,garment_id) WHERE action='ADD';
CREATE INDEX pack_carton_history_v22 ON pack_carton_events_v22(tenant_id,plant_id,carton_id,event_no);

CREATE FUNCTION guard_pack_carton_event_v22() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE prior pack_carton_events_v22%ROWTYPE; opener pack_carton_events_v22%ROWTYPE;
 spec pack_carton_specs_v22%ROWTYPE; garment gen_garments_v18%ROWTYPE;
 qty bigint; net_expected bigint; last_quality text;
BEGIN
 PERFORM pg_advisory_xact_lock(hashtext(NEW.tenant_id),hashtext('F35:CARTON:'||NEW.carton_id::text));
 NEW.event_no:=(SELECT COALESCE(max(event_no),0)+1 FROM pack_carton_events_v22
   WHERE tenant_id=NEW.tenant_id AND carton_id=NEW.carton_id);
 SELECT * INTO prior FROM pack_carton_events_v22 WHERE tenant_id=NEW.tenant_id AND carton_id=NEW.carton_id
  ORDER BY event_no DESC LIMIT 1;
 PERFORM pg_advisory_xact_lock(hashtext(NEW.tenant_id),hashtext('F35:SPEC:'||NEW.spec_id::text));
 SELECT * INTO spec FROM pack_carton_specs_v22 WHERE tenant_id=NEW.tenant_id AND id=NEW.spec_id;
 IF spec.id IS NULL OR spec.plant_id<>NEW.plant_id OR NOT spec.enabled THEN RAISE EXCEPTION 'ERR_F35_PACK_SPEC_SCOPE_OR_REVOKED'; END IF;
 IF NEW.action='OPEN' THEN
  IF prior.id IS NOT NULL OR NOT spec.enabled OR length(btrim(NEW.carton_code))<3 THEN
   RAISE EXCEPTION 'ERR_F35_OPEN_REQUIRES_APPROVED_PLAN'; END IF;
 ELSE
  SELECT * INTO opener FROM pack_carton_events_v22 WHERE tenant_id=NEW.tenant_id
   AND carton_id=NEW.carton_id AND action='OPEN';
  IF prior.id IS NULL OR opener.id IS NULL OR opener.spec_id<>NEW.spec_id OR prior.action='SEAL'
   THEN RAISE EXCEPTION 'ERR_F35_CARTON_MISSING_SEALED_OR_WRONG_SPEC'; END IF;
  IF NEW.action='ADD' THEN
   SELECT count(*) INTO qty FROM pack_carton_events_v22 WHERE tenant_id=NEW.tenant_id
      AND carton_id=NEW.carton_id AND action='ADD';
   IF qty>=spec.max_units THEN RAISE EXCEPTION 'ERR_F35_CARTON_MAX_UNITS'; END IF;
   SELECT * INTO garment FROM gen_garments_v18 WHERE tenant_id=NEW.tenant_id AND id=NEW.garment_id;
   IF garment.id IS NULL OR garment.plant_id<>NEW.plant_id OR garment.lay_draft_id<>spec.lay_draft_id THEN
    RAISE EXCEPTION 'ERR_F35_GARMENT_SCOPE_OR_LAY'; END IF;
   SELECT q.action INTO last_quality FROM qc_garment_events_v21 q WHERE q.tenant_id=NEW.tenant_id
     AND q.garment_id=NEW.garment_id ORDER BY q.event_no DESC LIMIT 1;
   IF last_quality IS DISTINCT FROM 'QUALITY_RELEASE' THEN RAISE EXCEPTION 'ERR_F35_GARMENT_NOT_RELEASED'; END IF;
   IF NOT EXISTS(SELECT 1 FROM pack_size_weights_v22 w JOIN cut_lay_positions_v8 p
     ON p.tenant_id=w.tenant_id AND p.size_code=w.size_code
     WHERE w.tenant_id=NEW.tenant_id AND w.spec_id=NEW.spec_id
       AND p.lay_draft_id=garment.lay_draft_id AND p.marker_position=garment.marker_position)
     THEN RAISE EXCEPTION 'ERR_F35_SIZE_NOT_SPECIFIED'; END IF;
  ELSIF NEW.action='SEAL' THEN
   SELECT count(*),COALESCE(sum(w.expected_g),0) INTO qty,net_expected
    FROM pack_carton_events_v22 e JOIN gen_garments_v18 g ON g.tenant_id=e.tenant_id AND g.id=e.garment_id
    JOIN cut_lay_positions_v8 p ON p.tenant_id=g.tenant_id AND p.lay_draft_id=g.lay_draft_id AND p.marker_position=g.marker_position
    JOIN pack_size_weights_v22 w ON w.tenant_id=e.tenant_id AND w.spec_id=e.spec_id AND w.size_code=p.size_code
    WHERE e.tenant_id=NEW.tenant_id AND e.carton_id=NEW.carton_id AND e.action='ADD';
   IF qty<spec.min_units OR qty>spec.max_units OR prior.action<>'ADD'
    THEN RAISE EXCEPTION 'ERR_F35_INCOMPLETE_CARTON'; END IF;
   net_expected:=net_expected+spec.tare_g;
   IF NEW.expected_g<>net_expected OR abs(NEW.observed_g-net_expected)>spec.tolerance_g
     THEN RAISE EXCEPTION 'ERR_F35_WEIGHT_OUT_OF_TOLERANCE'; END IF;
   IF EXISTS(SELECT 1 FROM pack_carton_events_v22 e WHERE e.tenant_id=NEW.tenant_id
      AND e.carton_id=NEW.carton_id AND e.actor_subject=NEW.actor_subject) OR length(btrim(NEW.scale_reference))<3
     THEN RAISE EXCEPTION 'ERR_F35_INDEPENDENT_SEAL_OR_SCALE_REF'; END IF;
  END IF;
 END IF;
 NEW.recorded_at:=clock_timestamp();
 RETURN NEW;
END $$;
CREATE TRIGGER pack_carton_event_guard_v22 BEFORE INSERT ON pack_carton_events_v22
 FOR EACH ROW EXECUTE FUNCTION guard_pack_carton_event_v22();
CREATE TRIGGER pack_carton_immutable_v22 BEFORE UPDATE OR DELETE ON pack_carton_events_v22
 FOR EACH ROW EXECUTE FUNCTION protect_cut_draft_v8();

-- Manifest genealogy remains STAGED, never 'shipped'. A shipment requires
-- independently authenticated dock and carrier custody evidence (future gate).
CREATE TABLE pack_manifest_events_v22 (
 tenant_id varchar(64) NOT NULL REFERENCES tenants(id), id uuid NOT NULL DEFAULT uuid_generate_v4(),
 plant_id varchar(64) NOT NULL, manifest_id uuid NOT NULL, event_no bigint NOT NULL CHECK(event_no>0),
 action varchar(16) NOT NULL CHECK(action IN ('OPEN','ADD_CARTON','STAGE')),
 spec_id uuid NOT NULL, manifest_ref varchar(80), destination_ref varchar(128), carton_id uuid,
 note text NOT NULL CHECK(length(btrim(note)) BETWEEN 12 AND 1000),
 command_id varchar(80) NOT NULL CHECK(command_id ~ '^[A-Za-z0-9_-]{12,80}$'),
 command_sha256 char(64) NOT NULL CHECK(command_sha256 ~ '^[0-9a-f]{64}$'),
 actor_subject varchar(128) NOT NULL, proof_id varchar(255) NOT NULL,
 assertion_sha256 char(64) NOT NULL CHECK(assertion_sha256 ~ '^[0-9a-f]{64}$'),
 recorded_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 evidence text NOT NULL DEFAULT 'STAGED_DISPATCH_MANIFEST_NOT_CARRIER_HANDOVER_OR_SHIPMENT'
   CHECK(evidence='STAGED_DISPATCH_MANIFEST_NOT_CARRIER_HANDOVER_OR_SHIPMENT'),
 PRIMARY KEY(tenant_id,id), UNIQUE(tenant_id,manifest_id,event_no),
 UNIQUE(tenant_id,command_id), UNIQUE(tenant_id,proof_id),
 FOREIGN KEY(tenant_id,spec_id) REFERENCES pack_carton_specs_v22(tenant_id,id),
 CHECK((action='OPEN' AND manifest_ref IS NOT NULL AND destination_ref IS NOT NULL AND carton_id IS NULL)
  OR (action='ADD_CARTON' AND manifest_ref IS NULL AND destination_ref IS NULL AND carton_id IS NOT NULL)
  OR (action='STAGE' AND manifest_ref IS NULL AND destination_ref IS NULL AND carton_id IS NULL))
);
CREATE UNIQUE INDEX pack_unique_manifest_ref_v22 ON pack_manifest_events_v22(tenant_id,plant_id,manifest_ref) WHERE action='OPEN';
CREATE UNIQUE INDEX pack_one_manifest_per_carton_v22 ON pack_manifest_events_v22(tenant_id,carton_id) WHERE action='ADD_CARTON';
CREATE INDEX pack_manifest_history_v22 ON pack_manifest_events_v22(tenant_id,plant_id,manifest_id,event_no);
CREATE FUNCTION guard_pack_manifest_event_v22() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE prior pack_manifest_events_v22%ROWTYPE; opener pack_manifest_events_v22%ROWTYPE;
 carton_open pack_carton_events_v22%ROWTYPE; carton_last pack_carton_events_v22%ROWTYPE;
 qty bigint;
BEGIN
 PERFORM pg_advisory_xact_lock(hashtext(NEW.tenant_id),hashtext('F35:MANIFEST:'||NEW.manifest_id::text));
 PERFORM pg_advisory_xact_lock(hashtext(NEW.tenant_id),hashtext('F35:SPEC:'||NEW.spec_id::text));
 IF NOT EXISTS(SELECT 1 FROM pack_carton_specs_v22 s WHERE s.tenant_id=NEW.tenant_id
   AND s.id=NEW.spec_id AND s.plant_id=NEW.plant_id AND s.enabled) THEN
   RAISE EXCEPTION 'ERR_F35_MANIFEST_SPEC_REVOKED_OR_SCOPE'; END IF;
 NEW.event_no:=(SELECT COALESCE(max(event_no),0)+1 FROM pack_manifest_events_v22 WHERE tenant_id=NEW.tenant_id AND manifest_id=NEW.manifest_id);
 SELECT * INTO prior FROM pack_manifest_events_v22 WHERE tenant_id=NEW.tenant_id AND manifest_id=NEW.manifest_id ORDER BY event_no DESC LIMIT 1;
 IF NEW.action='OPEN' THEN
  IF prior.id IS NOT NULL OR length(btrim(NEW.manifest_ref))<3 OR length(btrim(NEW.destination_ref))<3
    OR NOT EXISTS(SELECT 1 FROM pack_carton_specs_v22 s WHERE s.tenant_id=NEW.tenant_id
      AND s.id=NEW.spec_id AND s.plant_id=NEW.plant_id AND s.enabled) THEN
   RAISE EXCEPTION 'ERR_F35_MANIFEST_REQUIRES_APPROVED_SPEC'; END IF;
 ELSE
  SELECT * INTO opener FROM pack_manifest_events_v22 WHERE tenant_id=NEW.tenant_id AND manifest_id=NEW.manifest_id AND action='OPEN';
  IF prior.id IS NULL OR opener.id IS NULL OR opener.spec_id<>NEW.spec_id OR opener.plant_id<>NEW.plant_id OR prior.action='STAGE' THEN
   RAISE EXCEPTION 'ERR_F35_MANIFEST_NOT_OPEN'; END IF;
  IF NEW.action='ADD_CARTON' THEN
   SELECT * INTO carton_open FROM pack_carton_events_v22 e WHERE e.tenant_id=NEW.tenant_id
    AND e.carton_id=NEW.carton_id AND e.action='OPEN';
   SELECT * INTO carton_last FROM pack_carton_events_v22 e WHERE e.tenant_id=NEW.tenant_id
    AND e.carton_id=NEW.carton_id ORDER BY event_no DESC LIMIT 1;
   IF carton_open.id IS NULL OR carton_open.spec_id<>NEW.spec_id OR carton_open.plant_id<>NEW.plant_id
    OR carton_last.action<>'SEAL' THEN RAISE EXCEPTION 'ERR_F35_UNSEALED_OR_WRONG_CARTON'; END IF;
  ELSIF NEW.action='STAGE' THEN
   SELECT count(*) INTO qty FROM pack_manifest_events_v22 e WHERE e.tenant_id=NEW.tenant_id
    AND e.manifest_id=NEW.manifest_id AND e.action='ADD_CARTON';
   IF qty=0 OR EXISTS(SELECT 1 FROM pack_manifest_events_v22 e WHERE e.tenant_id=NEW.tenant_id
      AND e.manifest_id=NEW.manifest_id AND e.actor_subject=NEW.actor_subject) THEN
    RAISE EXCEPTION 'ERR_F35_EMPTY_OR_NOT_INDEPENDENT_STAGING'; END IF;
  END IF;
 END IF;
 NEW.recorded_at:=clock_timestamp();
 RETURN NEW;
END $$;
CREATE TRIGGER pack_manifest_event_guard_v22 BEFORE INSERT ON pack_manifest_events_v22
 FOR EACH ROW EXECUTE FUNCTION guard_pack_manifest_event_v22();
CREATE TRIGGER pack_manifest_immutable_v22 BEFORE UPDATE OR DELETE ON pack_manifest_events_v22
 FOR EACH ROW EXECUTE FUNCTION protect_cut_draft_v8();
DO $$ DECLARE t text; BEGIN
 FOREACH t IN ARRAY ARRAY['pack_carton_specs_v22','pack_size_weights_v22','pack_carton_events_v22','pack_manifest_events_v22'] LOOP
  EXECUTE format('ALTER TABLE public.%I ENABLE ROW LEVEL SECURITY',t);
  EXECUTE format('ALTER TABLE public.%I FORCE ROW LEVEL SECURITY',t);
  EXECUTE format('CREATE POLICY packing_v22_tenant ON public.%I FOR ALL TO agiparel_app USING (tenant_id=NULLIF(current_setting(''app.current_tenant_id'',true),'''')) WITH CHECK(tenant_id=NULLIF(current_setting(''app.current_tenant_id'',true),''''))',t);
  EXECUTE format('REVOKE ALL ON public.%I FROM PUBLIC,agiparel_app',t);
  EXECUTE format('GRANT SELECT ON public.%I TO agiparel_app',t);
 END LOOP;
END $$;
GRANT INSERT ON pack_carton_events_v22,pack_manifest_events_v22 TO agiparel_app;
COMMIT;
