-- Author: Sunil Jain, CyberinQ LLC, 2026-10-29.
-- F3.18C + F3.19A/B. Calibration association, buyer AQL sampling, and
-- needle/metal-detector incidents are evidence, never automatic production credit.
BEGIN;
INSERT INTO workforce_chapter_actions_v17(chapter_id,permission,authority) VALUES
 (4,'stores.fabric.calibration.link','SIGNED_WRITE'),
 (7,'quality.aql.inspect','SIGNED_WRITE'),
 (7,'quality.needle.report','SIGNED_WRITE')
 ON CONFLICT(permission) DO NOTHING;

-- Provisioner-owned append-only instrument calibration history. The physical
-- calibration procedure and instrument keys must be independently commissioned.
CREATE TABLE factory_instrument_calibrations_v32 (
 tenant_id varchar(64) NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
 plant_id varchar(64) NOT NULL,
 instrument_id varchar(80) NOT NULL CHECK(instrument_id ~ '^[A-Za-z0-9_.-]{4,80}$'),
 revision integer NOT NULL CHECK(revision BETWEEN 1 AND 100000),
 instrument_kind varchar(32) NOT NULL CHECK(instrument_kind IN ('DIMENSION','SCALE','METAL_DETECTOR')),
 valid_from timestamptz NOT NULL,
 valid_until timestamptz NOT NULL,
 source_document_id varchar(36) NOT NULL,
 source_sha256 char(64) NOT NULL CHECK(source_sha256 ~ '^[0-9a-f]{64}$'),
 registered_by varchar(128) NOT NULL CHECK(length(btrim(registered_by))>0),
 registered_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 enabled boolean NOT NULL DEFAULT false,
 PRIMARY KEY(tenant_id,plant_id,instrument_id,revision),
 FOREIGN KEY(tenant_id,source_document_id) REFERENCES document_sources_v65(tenant_id,id) ON DELETE RESTRICT,
 CHECK(valid_until>valid_from AND valid_until<=valid_from+interval '5 years')
);
CREATE FUNCTION guard_instrument_calibration_v32() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE h char(64); last_rev integer;
BEGIN
 SELECT sha256 INTO h FROM document_sources_v65 WHERE tenant_id=NEW.tenant_id AND id=NEW.source_document_id;
 IF h IS NULL OR h<>NEW.source_sha256 THEN RAISE EXCEPTION 'ERR_F318C_CALIBRATION_SOURCE_HASH'; END IF;
 PERFORM pg_advisory_xact_lock(hashtext(NEW.tenant_id),hashtext('F318C:CAL:'||NEW.plant_id||':'||NEW.instrument_id));
 SELECT max(revision) INTO last_rev FROM factory_instrument_calibrations_v32
 WHERE tenant_id=NEW.tenant_id AND plant_id=NEW.plant_id AND instrument_id=NEW.instrument_id;
 IF last_rev IS NOT NULL AND NEW.revision<>last_rev+1 OR last_rev IS NULL AND NEW.revision<>1 THEN
  RAISE EXCEPTION 'ERR_F318C_REVISION_SEQUENCE'; END IF;
 NEW.registered_at:=clock_timestamp(); RETURN NEW;
END $$;
CREATE TRIGGER instrument_calibration_guard_v32 BEFORE INSERT ON factory_instrument_calibrations_v32
 FOR EACH ROW EXECUTE FUNCTION guard_instrument_calibration_v32();

CREATE TABLE fabric_calibration_links_v32 (
 tenant_id varchar(64) NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
 id uuid NOT NULL DEFAULT uuid_generate_v4(), plant_id varchar(64) NOT NULL,
 observation_id uuid NOT NULL,
 instrument_id varchar(80) NOT NULL,
 calibration_revision integer NOT NULL,
 method_note text NOT NULL CHECK(length(btrim(method_note)) BETWEEN 20 AND 800),
 command_id varchar(80) NOT NULL CHECK(command_id ~ '^[A-Za-z0-9_-]{12,80}$'),
 command_sha256 char(64) NOT NULL CHECK(command_sha256 ~ '^[0-9a-f]{64}$'),
 inspector_subject varchar(128) NOT NULL,
 inspector_proof_id varchar(255) NOT NULL,
 assertion_sha256 char(64) NOT NULL CHECK(assertion_sha256 ~ '^[0-9a-f]{64}$'),
 recorded_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 authority text NOT NULL DEFAULT 'HUMAN_LINKED_CALIBRATION_RECORD_NOT_DEVICE_SIGNED_READING'
 CHECK(authority='HUMAN_LINKED_CALIBRATION_RECORD_NOT_DEVICE_SIGNED_READING'),
 PRIMARY KEY(tenant_id,id), UNIQUE(tenant_id,observation_id),
 UNIQUE(tenant_id,command_id), UNIQUE(tenant_id,inspector_proof_id),
 FOREIGN KEY(tenant_id,observation_id) REFERENCES fabric_shrinkage_observations_v30(tenant_id,id) ON DELETE RESTRICT,
 FOREIGN KEY(tenant_id,plant_id,instrument_id,calibration_revision)
 REFERENCES factory_instrument_calibrations_v32(tenant_id,plant_id,instrument_id,revision) ON DELETE RESTRICT
);
CREATE FUNCTION guard_fabric_calibration_link_v32() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE o fabric_shrinkage_observations_v30%ROWTYPE; c factory_instrument_calibrations_v32%ROWTYPE; newest integer;
BEGIN
 SELECT * INTO o FROM fabric_shrinkage_observations_v30 WHERE tenant_id=NEW.tenant_id AND id=NEW.observation_id;
 SELECT * INTO c FROM factory_instrument_calibrations_v32 WHERE tenant_id=NEW.tenant_id AND plant_id=NEW.plant_id
  AND instrument_id=NEW.instrument_id AND revision=NEW.calibration_revision;
 SELECT max(revision) INTO newest FROM factory_instrument_calibrations_v32
  WHERE tenant_id=NEW.tenant_id AND plant_id=NEW.plant_id AND instrument_id=NEW.instrument_id;
 IF o.id IS NULL OR o.plant_id<>NEW.plant_id OR c.instrument_id IS NULL OR c.instrument_kind<>'DIMENSION'
 OR NOT c.enabled OR c.revision<>newest OR o.recorded_at NOT BETWEEN c.valid_from AND c.valid_until
 OR c.valid_until<clock_timestamp() OR o.tester_subject=NEW.inspector_subject THEN
  RAISE EXCEPTION 'ERR_F318C_INVALID_CALIBRATION_OR_INDEPENDENCE'; END IF;
 NEW.recorded_at:=clock_timestamp(); RETURN NEW;
END $$;
CREATE TRIGGER fabric_calibration_link_guard_v32 BEFORE INSERT ON fabric_calibration_links_v32
 FOR EACH ROW EXECUTE FUNCTION guard_fabric_calibration_link_v32();

-- Buyer-approved AQL plan selects a fixed, immutable sample of actual garment
-- IDs. Approval is provisioner-only; plan revisions cannot be edited in place.
CREATE TABLE buyer_aql_plans_v32 (
 tenant_id varchar(64) NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
 id uuid NOT NULL DEFAULT uuid_generate_v4(), plant_id varchar(64) NOT NULL,
 lay_draft_id uuid NOT NULL,
 lot_code varchar(80) NOT NULL CHECK(lot_code ~ '^[A-Za-z0-9_.-]{4,80}$'),
 revision integer NOT NULL CHECK(revision BETWEEN 1 AND 100000),
 sample_garment_ids uuid[] NOT NULL,
 source_document_id varchar(128) NOT NULL,
 source_sha256 char(64) NOT NULL CHECK(source_sha256 ~ '^[0-9a-f]{64}$'),
 sampling_method varchar(80) NOT NULL CHECK(length(btrim(sampling_method))>=8),
 provisioned_by varchar(128) NOT NULL CHECK(length(btrim(provisioned_by))>0),
 enabled boolean NOT NULL DEFAULT false,
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(tenant_id,id), UNIQUE(tenant_id,plant_id,lot_code,revision),
 FOREIGN KEY(tenant_id,lay_draft_id) REFERENCES cut_lay_drafts_v8(tenant_id,id) ON DELETE RESTRICT,
 FOREIGN KEY(tenant_id,source_document_id) REFERENCES documents(tenant_id,id) ON DELETE RESTRICT,
 CHECK(array_ndims(sample_garment_ids)=1 AND cardinality(sample_garment_ids) BETWEEN 1 AND 500)
);
CREATE FUNCTION guard_buyer_aql_plan_v32() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE d documents%ROWTYPE; n integer; prior integer;
BEGIN
 SELECT * INTO d FROM documents WHERE tenant_id=NEW.tenant_id AND id=NEW.source_document_id;
 IF d.id IS NULL OR d.status<>'APPROVED' OR upper(d.issuer)<>'CUSTOMER' OR
  d.file_checksum<>NEW.source_sha256 OR
  d.document_type NOT IN ('TECH_PACK','BUYER_SPEC','BUYER_AQL_PLAN','AQL_INSPECTION_PLAN')
 THEN RAISE EXCEPTION 'ERR_F319A_BUYER_SOURCE'; END IF;
 SELECT count(DISTINCT sampled.garment_id) INTO n FROM unnest(NEW.sample_garment_ids) AS sampled(garment_id);
 IF n<>cardinality(NEW.sample_garment_ids) OR EXISTS (
   SELECT 1 FROM unnest(NEW.sample_garment_ids) AS sampled(garment_id) LEFT JOIN gen_garments_v18 g
    ON g.tenant_id=NEW.tenant_id AND g.id=sampled.garment_id
    WHERE g.id IS NULL OR g.plant_id<>NEW.plant_id OR g.lay_draft_id<>NEW.lay_draft_id)
 THEN RAISE EXCEPTION 'ERR_F319A_SAMPLE_GENEALOGY'; END IF;
 PERFORM pg_advisory_xact_lock(hashtext(NEW.tenant_id),hashtext('F319A:PLAN:'||NEW.plant_id||':'||NEW.lot_code));
 SELECT max(revision) INTO prior FROM buyer_aql_plans_v32 WHERE tenant_id=NEW.tenant_id
  AND plant_id=NEW.plant_id AND lot_code=NEW.lot_code;
 IF (prior IS NULL AND NEW.revision<>1) OR (prior IS NOT NULL AND NEW.revision<>prior+1)
 THEN RAISE EXCEPTION 'ERR_F319A_REVISION'; END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER buyer_aql_plan_guard_v32 BEFORE INSERT ON buyer_aql_plans_v32
 FOR EACH ROW EXECUTE FUNCTION guard_buyer_aql_plan_v32();

CREATE TABLE buyer_aql_sample_events_v32 (
 tenant_id varchar(64) NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
 id uuid NOT NULL DEFAULT uuid_generate_v4(), plant_id varchar(64) NOT NULL,
 plan_id uuid NOT NULL, garment_id uuid NOT NULL,
 finding varchar(16) NOT NULL CHECK(finding IN ('PASS','MINOR','MAJOR','CRITICAL')),
 defect_code varchar(64), note text NOT NULL CHECK(length(btrim(note)) BETWEEN 20 AND 800),
 command_id varchar(80) NOT NULL CHECK(command_id ~ '^[A-Za-z0-9_-]{12,80}$'),
 command_sha256 char(64) NOT NULL CHECK(command_sha256 ~ '^[0-9a-f]{64}$'),
 inspector_subject varchar(128) NOT NULL,
 inspector_proof_id varchar(255) NOT NULL,
 assertion_sha256 char(64) NOT NULL CHECK(assertion_sha256 ~ '^[0-9a-f]{64}$'),
 recorded_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 authority text NOT NULL DEFAULT 'SIGNED_AQL_SAMPLE_FINDING_NOT_LOT_ACCEPTANCE'
 CHECK(authority='SIGNED_AQL_SAMPLE_FINDING_NOT_LOT_ACCEPTANCE'),
 PRIMARY KEY(tenant_id,id), UNIQUE(tenant_id,plan_id,garment_id),
 UNIQUE(tenant_id,command_id), UNIQUE(tenant_id,inspector_proof_id),
 FOREIGN KEY(tenant_id,plan_id) REFERENCES buyer_aql_plans_v32(tenant_id,id) ON DELETE RESTRICT,
 FOREIGN KEY(tenant_id,garment_id) REFERENCES gen_garments_v18(tenant_id,id) ON DELETE RESTRICT,
 CHECK((finding='PASS' AND defect_code IS NULL) OR
       (finding<>'PASS' AND defect_code IS NOT NULL))
);
CREATE FUNCTION guard_buyer_aql_sample_v32() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE p buyer_aql_plans_v32%ROWTYPE; last_rev integer;
BEGIN
 SELECT * INTO p FROM buyer_aql_plans_v32 WHERE tenant_id=NEW.tenant_id AND id=NEW.plan_id;
 SELECT max(revision) INTO last_rev FROM buyer_aql_plans_v32
  WHERE tenant_id=NEW.tenant_id AND plant_id=p.plant_id AND lot_code=p.lot_code;
 IF p.id IS NULL OR NOT p.enabled OR p.revision<>last_rev OR p.plant_id<>NEW.plant_id
 OR NOT (NEW.garment_id=ANY(p.sample_garment_ids)) THEN
  RAISE EXCEPTION 'ERR_F319A_SAMPLE_NOT_SELECTED'; END IF;
 IF NOT EXISTS (SELECT 1 FROM qc_garment_events_v21 q WHERE q.tenant_id=NEW.tenant_id
  AND q.plant_id=NEW.plant_id AND q.garment_id=NEW.garment_id AND q.action='QUALITY_RELEASE')
 THEN RAISE EXCEPTION 'ERR_F319A_PER_GARMENT_QC_RELEASE_REQUIRED'; END IF;
 IF NEW.defect_code IS NOT NULL AND NOT EXISTS(SELECT 1 FROM qc_garment_defects_v21 d
  WHERE d.tenant_id=NEW.tenant_id AND d.plant_id=NEW.plant_id AND d.defect_code=NEW.defect_code AND d.enabled)
 THEN RAISE EXCEPTION 'ERR_F319A_UNKNOWN_DEFECT'; END IF;
 NEW.recorded_at:=clock_timestamp(); RETURN NEW;
END $$;
CREATE TRIGGER buyer_aql_sample_guard_v32 BEFORE INSERT ON buyer_aql_sample_events_v32
 FOR EACH ROW EXECUTE FUNCTION guard_buyer_aql_sample_v32();

-- Human-reported safety observations. BREAK/FAIL records are permanent; an
-- independent follow-up clearance/release is a separate future transition.
CREATE TABLE needle_safety_events_v32 (
 tenant_id varchar(64) NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
 id uuid NOT NULL DEFAULT uuid_generate_v4(), plant_id varchar(64) NOT NULL,
 garment_id uuid NOT NULL, instrument_id varchar(80),
 event_kind varchar(32) NOT NULL CHECK(event_kind IN
  ('NEEDLE_BREAK','DETECTOR_CHALLENGE_PASS','DETECTOR_CHALLENGE_FAIL','DETECTOR_REJECT')),
 expected_fragments integer NOT NULL CHECK(expected_fragments BETWEEN 0 AND 20),
 recovered_fragments integer NOT NULL CHECK(recovered_fragments BETWEEN 0 AND 20),
 evidence_document_id varchar(36) NOT NULL, evidence_sha256 char(64) NOT NULL
 CHECK(evidence_sha256 ~ '^[0-9a-f]{64}$'),
 note text NOT NULL CHECK(length(btrim(note)) BETWEEN 20 AND 800),
 command_id varchar(80) NOT NULL CHECK(command_id ~ '^[A-Za-z0-9_-]{12,80}$'),
 command_sha256 char(64) NOT NULL CHECK(command_sha256 ~ '^[0-9a-f]{64}$'),
 actor_subject varchar(128) NOT NULL, proof_id varchar(255) NOT NULL,
 assertion_sha256 char(64) NOT NULL CHECK(assertion_sha256 ~ '^[0-9a-f]{64}$'),
 recorded_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 authority text NOT NULL DEFAULT 'SIGNED_HUMAN_SAFETY_REPORT_NOT_MACHINE_ATTESTATION_OR_RELEASE'
 CHECK(authority='SIGNED_HUMAN_SAFETY_REPORT_NOT_MACHINE_ATTESTATION_OR_RELEASE'),
 PRIMARY KEY(tenant_id,id), UNIQUE(tenant_id,command_id), UNIQUE(tenant_id,proof_id),
 FOREIGN KEY(tenant_id,garment_id) REFERENCES gen_garments_v18(tenant_id,id) ON DELETE RESTRICT,
 FOREIGN KEY(tenant_id,evidence_document_id) REFERENCES document_sources_v65(tenant_id,id) ON DELETE RESTRICT,
 CHECK(recovered_fragments<=expected_fragments),
 CHECK(event_kind<>'NEEDLE_BREAK' OR expected_fragments>0)
);
CREATE FUNCTION guard_needle_safety_event_v32() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE g gen_garments_v18%ROWTYPE; h char(64);
BEGIN
 SELECT * INTO g FROM gen_garments_v18 WHERE tenant_id=NEW.tenant_id AND id=NEW.garment_id;
 SELECT sha256 INTO h FROM document_sources_v65 WHERE tenant_id=NEW.tenant_id AND id=NEW.evidence_document_id;
 PERFORM pg_advisory_xact_lock(hashtext(NEW.tenant_id),hashtext('F34:GARMENT:'||NEW.garment_id::text));
 IF g.id IS NULL OR g.plant_id<>NEW.plant_id OR h IS NULL OR h<>NEW.evidence_sha256
 THEN RAISE EXCEPTION 'ERR_F319B_GARMENT_OR_EVIDENCE'; END IF;
 IF NEW.event_kind LIKE 'DETECTOR_%' AND (NEW.instrument_id IS NULL OR NOT EXISTS(
  SELECT 1 FROM factory_instrument_calibrations_v32 c WHERE c.tenant_id=NEW.tenant_id
   AND c.plant_id=NEW.plant_id AND c.instrument_id=NEW.instrument_id AND c.instrument_kind='METAL_DETECTOR'
   AND c.revision=(SELECT max(revision) FROM factory_instrument_calibrations_v32 x
    WHERE x.tenant_id=c.tenant_id AND x.plant_id=c.plant_id AND x.instrument_id=c.instrument_id)
   AND c.enabled AND clock_timestamp() BETWEEN c.valid_from AND c.valid_until)) THEN
  RAISE EXCEPTION 'ERR_F319B_DETECTOR_CALIBRATION_REQUIRED'; END IF;
 NEW.recorded_at:=clock_timestamp(); RETURN NEW;
END $$;
CREATE TRIGGER needle_safety_guard_v32 BEFORE INSERT ON needle_safety_events_v32
 FOR EACH ROW EXECUTE FUNCTION guard_needle_safety_event_v32();

-- Serialization and proof-ID uniqueness extend across all three new ledgers.
CREATE FUNCTION guard_factory_proof_v32() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE proof text;
BEGIN
 IF TG_TABLE_NAME='needle_safety_events_v32' THEN proof:=NEW.proof_id;
 ELSE proof:=NEW.inspector_proof_id; END IF;
 PERFORM pg_advisory_xact_lock(hashtext(NEW.tenant_id),hashtext('F32:PROOF:'||proof));
 IF EXISTS(SELECT 1 FROM fabric_calibration_links_v32 x WHERE x.tenant_id=NEW.tenant_id AND x.inspector_proof_id=proof)
 OR EXISTS(SELECT 1 FROM buyer_aql_sample_events_v32 x WHERE x.tenant_id=NEW.tenant_id AND x.inspector_proof_id=proof)
 OR EXISTS(SELECT 1 FROM needle_safety_events_v32 x WHERE x.tenant_id=NEW.tenant_id AND x.proof_id=proof)
 THEN RAISE EXCEPTION 'ERR_F32_PROOF_REUSED'; END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER calibration_proof_guard_v32 BEFORE INSERT ON fabric_calibration_links_v32
 FOR EACH ROW EXECUTE FUNCTION guard_factory_proof_v32();
CREATE TRIGGER buyer_aql_proof_guard_v32 BEFORE INSERT ON buyer_aql_sample_events_v32
 FOR EACH ROW EXECUTE FUNCTION guard_factory_proof_v32();
CREATE TRIGGER needle_safety_proof_guard_v32 BEFORE INSERT ON needle_safety_events_v32
 FOR EACH ROW EXECUTE FUNCTION guard_factory_proof_v32();

-- Negative incidents have no automatic clearance. Once a break/reject/fail is
-- reported, affected garment QC release and carton addition remain blocked.
-- Future F3.19C must add independent source-backed physical safety clearance.
CREATE FUNCTION garment_needle_safety_blocked_v32(t text,g uuid) RETURNS boolean
 LANGUAGE sql VOLATILE AS $$
 SELECT EXISTS(SELECT 1 FROM needle_safety_events_v32 e
  WHERE e.tenant_id=t AND e.garment_id=g AND e.event_kind IN
   ('NEEDLE_BREAK','DETECTOR_CHALLENGE_FAIL','DETECTOR_REJECT'))
$$;
CREATE FUNCTION guard_needle_safety_progress_v32() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF (TG_TABLE_NAME='qc_garment_events_v21' AND NEW.action<>'QUALITY_RELEASE') OR
    (TG_TABLE_NAME='pack_carton_events_v22' AND NEW.action<>'ADD') THEN RETURN NEW; END IF;
 PERFORM pg_advisory_xact_lock(hashtext(NEW.tenant_id),hashtext('F34:GARMENT:'||NEW.garment_id::text));
 IF garment_needle_safety_blocked_v32(NEW.tenant_id,NEW.garment_id) THEN
  RAISE EXCEPTION 'ERR_F319B_UNRESOLVED_GARMENT_SAFETY_HOLD'; END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER needle_safety_qc_gate_v32 BEFORE INSERT ON qc_garment_events_v21
 FOR EACH ROW EXECUTE FUNCTION guard_needle_safety_progress_v32();
CREATE TRIGGER needle_safety_packing_gate_v32 BEFORE INSERT ON pack_carton_events_v22
 FOR EACH ROW EXECUTE FUNCTION guard_needle_safety_progress_v32();

DO $$ DECLARE tbl text; BEGIN
 FOREACH tbl IN ARRAY ARRAY[
 'factory_instrument_calibrations_v32','fabric_calibration_links_v32',
 'buyer_aql_plans_v32','buyer_aql_sample_events_v32','needle_safety_events_v32'] LOOP
  EXECUTE format('ALTER TABLE public.%I ENABLE ROW LEVEL SECURITY',tbl);
  EXECUTE format('ALTER TABLE public.%I FORCE ROW LEVEL SECURITY',tbl);
  EXECUTE format('CREATE POLICY factory_tenant_v32 ON public.%I FOR ALL TO agiparel_app USING (tenant_id=NULLIF(current_setting(%L,true),%L)) WITH CHECK (tenant_id=NULLIF(current_setting(%L,true),%L))',tbl,'app.current_tenant_id','','app.current_tenant_id','');
  EXECUTE format('REVOKE ALL ON public.%I FROM PUBLIC,agiparel_app',tbl);
  EXECUTE format('GRANT SELECT ON public.%I TO agiparel_app',tbl);
  EXECUTE format('CREATE TRIGGER immutable_factory_v32 BEFORE UPDATE OR DELETE ON public.%I FOR EACH ROW EXECUTE FUNCTION protect_cut_draft_v8()',tbl);
 END LOOP;
END $$;
GRANT INSERT ON fabric_calibration_links_v32,buyer_aql_sample_events_v32,needle_safety_events_v32 TO agiparel_app;
-- AQL plan and instrument calibration INSERT remain restricted to provisioner.
COMMIT;
