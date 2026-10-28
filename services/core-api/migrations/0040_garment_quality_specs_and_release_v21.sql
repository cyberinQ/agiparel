-- Author: Sunil Jain, 2026-10-28.
-- F3.4: provisioner-approved, source-backed garment QC requirements; immutable
-- individual-garment inspection, rework, finishing and quality-only release.
-- Never infer sewing output, AQL conformance, stock, carton or shipment credit.
BEGIN;
INSERT INTO workforce_chapter_actions_v17(chapter_id,permission,authority)
 VALUES (7,'quality.finish','SIGNED_WRITE'),(7,'quality.release','SIGNED_WRITE')
 ON CONFLICT DO NOTHING;

-- Each revision is pinned to exactly one signed lay and a saved source document.
-- Provisioning/approval is reserved to the DB provisioner, NOT the app role.
CREATE TABLE qc_garment_specs_v21 (
 tenant_id varchar(64) NOT NULL REFERENCES tenants(id),
 id uuid NOT NULL DEFAULT uuid_generate_v4(),
 plant_id varchar(64) NOT NULL,
 lay_draft_id uuid NOT NULL,
 revision varchar(64) NOT NULL,
 source_document_id varchar(36) NOT NULL,
 source_sha256 char(64) NOT NULL,
 finish_station varchar(64) NOT NULL,
 approved_by varchar(128) NOT NULL,
 approved_at timestamptz NOT NULL DEFAULT now(),
 enabled boolean NOT NULL DEFAULT false,
 inspection_basis text NOT NULL DEFAULT 'EVERY_GARMENT_NOT_AQL_SAMPLE'
  CHECK (inspection_basis='EVERY_GARMENT_NOT_AQL_SAMPLE'),
 PRIMARY KEY(tenant_id,id),
 UNIQUE(tenant_id,lay_draft_id,revision),
 FOREIGN KEY(tenant_id,lay_draft_id) REFERENCES cut_lay_drafts_v8(tenant_id,id),
 FOREIGN KEY(tenant_id,source_document_id) REFERENCES document_sources_v65(tenant_id,id),
 CHECK(length(btrim(revision))>0 AND length(btrim(approved_by))>0)
);
CREATE INDEX qc_specs_lay_v21 ON qc_garment_specs_v21(tenant_id,plant_id,lay_draft_id,enabled);
CREATE UNIQUE INDEX qc_one_active_spec_per_lay_v21 ON qc_garment_specs_v21(tenant_id,lay_draft_id) WHERE enabled;
CREATE TABLE qc_garment_measure_rules_v21 (
 tenant_id varchar(64) NOT NULL,
 spec_id uuid NOT NULL,
 size_code varchar(32) NOT NULL,
 measure_code varchar(64) NOT NULL CHECK(measure_code ~ '^[A-Za-z0-9_-]{2,64}$'),
 nominal_mm bigint NOT NULL CHECK(nominal_mm>=0),
 lower_tolerance_mm bigint NOT NULL CHECK(lower_tolerance_mm>=0),
 upper_tolerance_mm bigint NOT NULL CHECK(upper_tolerance_mm>=0),
 PRIMARY KEY(tenant_id,spec_id,size_code,measure_code),
 FOREIGN KEY(tenant_id,spec_id) REFERENCES qc_garment_specs_v21(tenant_id,id)
);
CREATE TABLE qc_garment_defects_v21 (
 tenant_id varchar(64) NOT NULL REFERENCES tenants(id),
 plant_id varchar(64) NOT NULL,
 defect_code varchar(64) NOT NULL CHECK(defect_code ~ '^[A-Za-z0-9_-]{2,64}$'),
 description varchar(512) NOT NULL CHECK(length(btrim(description))>0),
 enabled boolean NOT NULL DEFAULT false,
 PRIMARY KEY(tenant_id,plant_id,defect_code)
);
CREATE TABLE qc_garment_events_v21 (
 tenant_id varchar(64) NOT NULL REFERENCES tenants(id),
 id uuid NOT NULL DEFAULT uuid_generate_v4(),
 plant_id varchar(64) NOT NULL,
 garment_id uuid NOT NULL,
 spec_id uuid NOT NULL,
 event_no bigint NOT NULL CHECK(event_no>0),
 action varchar(24) NOT NULL CHECK(action IN (
  'INSPECT_PASS','INSPECT_HOLD','REPAIR_COMPLETE','REINSPECT_PASS',
  'REINSPECT_HOLD','FINISHING_COMPLETE','QUALITY_RELEASE')),
 defect_code varchar(64),
 measurements_mm jsonb NOT NULL DEFAULT '{}'::jsonb CHECK(jsonb_typeof(measurements_mm)='object'),
 note text NOT NULL CHECK(length(btrim(note)) BETWEEN 12 AND 1000),
 command_id varchar(80) NOT NULL CHECK(command_id ~ '^[A-Za-z0-9_-]{12,80}$'),
 command_sha256 char(64) NOT NULL CHECK(command_sha256 ~ '^[0-9a-f]{64}$'),
 actor_subject varchar(128) NOT NULL,
 proof_id varchar(255) NOT NULL,
 assertion_sha256 char(64) NOT NULL CHECK(assertion_sha256 ~ '^[0-9a-f]{64}$'),
 recorded_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 evidence text NOT NULL DEFAULT 'SIGNED_GARMENT_QC_HUMAN_ATTESTATION_NOT_SHIPPING_AUTHORITY'
  CHECK(evidence='SIGNED_GARMENT_QC_HUMAN_ATTESTATION_NOT_SHIPPING_AUTHORITY'),
 PRIMARY KEY(tenant_id,id),
 UNIQUE(tenant_id,garment_id,event_no), UNIQUE(tenant_id,command_id), UNIQUE(tenant_id,proof_id),
 FOREIGN KEY(tenant_id,garment_id) REFERENCES gen_garments_v18(tenant_id,id),
 FOREIGN KEY(tenant_id,spec_id) REFERENCES qc_garment_specs_v21(tenant_id,id),
 CHECK((action IN ('INSPECT_HOLD','REINSPECT_HOLD') AND defect_code IS NOT NULL)
   OR (action NOT IN ('INSPECT_HOLD','REINSPECT_HOLD') AND defect_code IS NULL))
);
CREATE INDEX qc_garment_history_v21 ON qc_garment_events_v21(tenant_id,plant_id,garment_id,event_no DESC);
CREATE INDEX qc_garment_status_v21 ON qc_garment_events_v21(tenant_id,plant_id,action,recorded_at);

CREATE FUNCTION guard_qc_spec_v21() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE doc_hash char(64);
BEGIN
 IF NEW.enabled THEN RAISE EXCEPTION 'ERR_F34_SPEC_ACTIVATE_AFTER_RULES'; END IF;
 SELECT sha256 INTO doc_hash FROM document_sources_v65
 WHERE tenant_id=NEW.tenant_id AND id=NEW.source_document_id;
 IF doc_hash IS NULL OR doc_hash<>NEW.source_sha256 THEN
  RAISE EXCEPTION 'ERR_F34_SPEC_SOURCE_MISMATCH'; END IF;
 IF NOT EXISTS(SELECT 1 FROM cut_sewing_releases_v8 r WHERE r.tenant_id=NEW.tenant_id
  AND r.lay_draft_id=NEW.lay_draft_id AND r.plant_id=NEW.plant_id) THEN
  RAISE EXCEPTION 'ERR_F34_SPEC_SIGNED_LAY_REQUIRED'; END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER qc_spec_guard_v21 BEFORE INSERT ON qc_garment_specs_v21
 FOR EACH ROW EXECUTE FUNCTION guard_qc_spec_v21();

-- Published specs and measurement tolerances are immutable. Disable, then
-- create a new revision; do not rewrite yesterday's acceptance criteria.
CREATE FUNCTION qc_spec_immutable_v21() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE missing_sizes bigint;
BEGIN
 PERFORM pg_advisory_xact_lock(hashtext(OLD.tenant_id),hashtext('F34:SPEC:'||OLD.id::text));
 IF TG_OP='DELETE' THEN RAISE EXCEPTION 'ERR_F34_IMMUTABLE_SPEC'; END IF;
 IF (to_jsonb(OLD)-'enabled')<>(to_jsonb(NEW)-'enabled') THEN
  RAISE EXCEPTION 'ERR_F34_IMMUTABLE_SPEC'; END IF;
 IF OLD.enabled AND NOT NEW.enabled THEN RETURN NEW; END IF;
 IF NOT OLD.enabled AND NEW.enabled THEN
  IF EXISTS (SELECT 1 FROM qc_garment_events_v21 e WHERE e.tenant_id=OLD.tenant_id AND e.spec_id=OLD.id) THEN
   RAISE EXCEPTION 'ERR_F34_ALREADY_USED_SPEC'; END IF;
  SELECT count(*) INTO missing_sizes FROM (
    SELECT DISTINCT size_code FROM cut_lay_positions_v8 WHERE tenant_id=OLD.tenant_id AND lay_draft_id=OLD.lay_draft_id
  ) sizes WHERE NOT EXISTS (
    SELECT 1 FROM qc_garment_measure_rules_v21 m WHERE m.tenant_id=OLD.tenant_id
      AND m.spec_id=OLD.id AND m.size_code=sizes.size_code);
  IF missing_sizes<>0 OR NOT EXISTS (SELECT 1 FROM qc_garment_measure_rules_v21 m
    WHERE m.tenant_id=OLD.tenant_id AND m.spec_id=OLD.id) THEN
   RAISE EXCEPTION 'ERR_F34_MEASUREMENT_RULES_INCOMPLETE'; END IF;
  RETURN NEW;
 END IF;
 RAISE EXCEPTION 'ERR_F34_IMMUTABLE_SPEC';
END $$;
CREATE TRIGGER qc_spec_lock_v21 BEFORE UPDATE OR DELETE ON qc_garment_specs_v21
 FOR EACH ROW EXECUTE FUNCTION qc_spec_immutable_v21();
CREATE FUNCTION qc_rule_insert_guard_v21() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE spec qc_garment_specs_v21%ROWTYPE;
BEGIN
 PERFORM pg_advisory_xact_lock(hashtext(NEW.tenant_id),hashtext('F34:SPEC:'||NEW.spec_id::text));
 SELECT * INTO spec FROM qc_garment_specs_v21 WHERE tenant_id=NEW.tenant_id AND id=NEW.spec_id;
 IF spec.id IS NULL OR spec.enabled OR NOT EXISTS (
  SELECT 1 FROM cut_lay_positions_v8 p WHERE p.tenant_id=spec.tenant_id
    AND p.lay_draft_id=spec.lay_draft_id AND p.size_code=NEW.size_code) THEN
   RAISE EXCEPTION 'ERR_F34_RULE_REQUIRES_DRAFT_SPEC_AND_VALID_SIZE'; END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER qc_rule_insert_v21 BEFORE INSERT ON qc_garment_measure_rules_v21
 FOR EACH ROW EXECUTE FUNCTION qc_rule_insert_guard_v21();
CREATE TRIGGER qc_rules_lock_v21 BEFORE UPDATE OR DELETE ON qc_garment_measure_rules_v21
 FOR EACH ROW EXECUTE FUNCTION protect_cut_draft_v8();
CREATE TRIGGER qc_events_lock_v21 BEFORE UPDATE OR DELETE ON qc_garment_events_v21
 FOR EACH ROW EXECUTE FUNCTION protect_cut_draft_v8();

-- Verify that every original cut part is accepted, or has a specific accepted
-- replacement. An unsupported/undecided component blocks QC acceptance.
-- Each effective part must have a completed WIP ARRIVE at the approved station.
CREATE FUNCTION qc_garment_parts_ready_v21(t varchar, gid uuid, finish varchar)
RETURNS boolean LANGUAGE sql STABLE AS $$
 WITH effective_parts AS (
  SELECT p.id original_id,
   CASE WHEN o.disposition='ACCEPTED' AND l.id IS NULL AND NOT EXISTS (
          SELECT 1 FROM cut_recut_links_v19 donor WHERE donor.tenant_id=p.tenant_id
            AND donor.replacement_part_id=p.id) THEN p.id
        WHEN o.disposition='REJECTED' THEN l.replacement_part_id END AS effective_id
  FROM gen_cut_parts_v18 p
  LEFT JOIN cut_part_outcomes_v19 o ON o.tenant_id=p.tenant_id AND o.part_id=p.id
  LEFT JOIN cut_recut_links_v19 l ON l.tenant_id=p.tenant_id AND l.original_part_id=p.id
  WHERE p.tenant_id=t AND p.garment_id=gid
 ), checked_parts AS (
  SELECT ep.original_id, ep.effective_id,
   (SELECT m.action='ARRIVE' AND m.station_id=finish
    FROM wip_movements_v20 m WHERE m.tenant_id=t AND m.part_id=ep.effective_id
    ORDER BY m.event_no DESC LIMIT 1) AS at_finish,
   EXISTS (SELECT 1 FROM cut_part_outcomes_v19 o WHERE o.tenant_id=t
      AND o.part_id=ep.effective_id AND o.disposition='ACCEPTED') AS accepted
  FROM effective_parts ep
 )
 SELECT count(*)>0 AND count(*)=count(effective_id)
  AND bool_and(COALESCE(at_finish,false) AND accepted)
 FROM checked_parts
$$;

-- A DB guard is essential: direct SQL and concurrent handlers cannot bypass
-- state sequencing, plan selection, measurements or worker separation.
CREATE FUNCTION guard_qc_garment_event_v21() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE spec qc_garment_specs_v21%ROWTYPE; g gen_garments_v18%ROWTYPE;
 prior qc_garment_events_v21%ROWTYPE; prior_inspector varchar(128);
 size_text varchar(32); rule record; val_text text; measured bigint;
 required_count bigint:=0; provided_count bigint;
BEGIN
 PERFORM pg_advisory_xact_lock(hashtext(NEW.tenant_id),hashtext('F34:GARMENT:'||NEW.garment_id::text));
 NEW.event_no := (SELECT COALESCE(max(event_no),0)+1 FROM qc_garment_events_v21
  WHERE tenant_id=NEW.tenant_id AND garment_id=NEW.garment_id);
 NEW.recorded_at := clock_timestamp();
 SELECT * INTO g FROM gen_garments_v18 WHERE tenant_id=NEW.tenant_id AND id=NEW.garment_id;
 PERFORM pg_advisory_xact_lock(hashtext(NEW.tenant_id),hashtext('F34:SPEC:'||NEW.spec_id::text));
 SELECT * INTO spec FROM qc_garment_specs_v21 WHERE tenant_id=NEW.tenant_id AND id=NEW.spec_id;
 IF g.id IS NULL OR spec.id IS NULL OR NOT spec.enabled OR g.plant_id<>NEW.plant_id
    OR spec.plant_id<>NEW.plant_id OR spec.lay_draft_id<>g.lay_draft_id THEN
  RAISE EXCEPTION 'ERR_F34_SPEC_SCOPE_OR_REVISION'; END IF;
 SELECT p.size_code INTO size_text FROM cut_lay_positions_v8 p
  WHERE p.tenant_id=NEW.tenant_id AND p.lay_draft_id=g.lay_draft_id AND p.marker_position=g.marker_position;
 IF size_text IS NULL THEN RAISE EXCEPTION 'ERR_F34_SIZE_MISSING'; END IF;
 SELECT * INTO prior FROM qc_garment_events_v21
  WHERE tenant_id=NEW.tenant_id AND garment_id=NEW.garment_id ORDER BY event_no DESC LIMIT 1;
 IF prior.id IS NOT NULL AND prior.spec_id<>NEW.spec_id THEN RAISE EXCEPTION 'ERR_F34_MIXED_SPEC_REVISIONS'; END IF;
 IF NEW.action='INSPECT_PASS' OR NEW.action='INSPECT_HOLD' THEN
  IF prior.id IS NOT NULL THEN RAISE EXCEPTION 'ERR_F34_ALREADY_INSPECTED'; END IF;
 ELSIF NEW.action='REPAIR_COMPLETE' THEN
  IF prior.id IS NULL OR prior.action NOT IN ('INSPECT_HOLD','REINSPECT_HOLD')
     OR prior.actor_subject=NEW.actor_subject THEN RAISE EXCEPTION 'ERR_F34_REPAIR_SEQUENCE'; END IF;
 ELSIF NEW.action IN ('REINSPECT_PASS','REINSPECT_HOLD') THEN
  IF prior.id IS NULL OR prior.action<>'REPAIR_COMPLETE' OR prior.actor_subject=NEW.actor_subject THEN
   RAISE EXCEPTION 'ERR_F34_REINSPECTION_SEQUENCE'; END IF;
 ELSIF NEW.action='FINISHING_COMPLETE' THEN
  IF prior.id IS NULL OR prior.action NOT IN ('INSPECT_PASS','REINSPECT_PASS')
     OR prior.actor_subject=NEW.actor_subject THEN RAISE EXCEPTION 'ERR_F34_FINISHING_SEQUENCE'; END IF;
 ELSIF NEW.action='QUALITY_RELEASE' THEN
  IF prior.id IS NULL OR prior.action<>'FINISHING_COMPLETE' OR prior.actor_subject=NEW.actor_subject THEN
   RAISE EXCEPTION 'ERR_F34_RELEASE_SEQUENCE'; END IF;
 ELSE RAISE EXCEPTION 'ERR_F34_UNKNOWN_ACTION'; END IF;
 IF NEW.action IN ('INSPECT_PASS','INSPECT_HOLD','REINSPECT_PASS','REINSPECT_HOLD','FINISHING_COMPLETE','QUALITY_RELEASE')
    AND NOT qc_garment_parts_ready_v21(NEW.tenant_id,NEW.garment_id,spec.finish_station) THEN
   RAISE EXCEPTION 'ERR_F34_PARTS_NOT_AT_FINISH_STATION'; END IF;
 IF NEW.action IN ('INSPECT_HOLD','REINSPECT_HOLD') AND NOT EXISTS (
  SELECT 1 FROM qc_garment_defects_v21 d WHERE d.tenant_id=NEW.tenant_id
   AND d.plant_id=NEW.plant_id AND d.defect_code=NEW.defect_code AND d.enabled) THEN
   RAISE EXCEPTION 'ERR_F34_DEFECT_CODE_NOT_APPROVED'; END IF;
 IF NEW.action IN ('INSPECT_PASS','REINSPECT_PASS') THEN
  SELECT count(*) INTO provided_count FROM jsonb_object_keys(NEW.measurements_mm);
  FOR rule IN SELECT measure_code,nominal_mm,lower_tolerance_mm,upper_tolerance_mm
   FROM qc_garment_measure_rules_v21 WHERE tenant_id=NEW.tenant_id
   AND spec_id=NEW.spec_id AND size_code=size_text LOOP
   required_count:=required_count+1;
   val_text:=NEW.measurements_mm->>rule.measure_code;
   IF val_text IS NULL OR val_text !~ '^[0-9]{1,12}$' THEN
    RAISE EXCEPTION 'ERR_F34_MISSING_OR_NONINTEGER_MEASUREMENT'; END IF;
   measured:=val_text::bigint;
   IF measured<rule.nominal_mm-rule.lower_tolerance_mm OR measured>rule.nominal_mm+rule.upper_tolerance_mm THEN
    RAISE EXCEPTION 'ERR_F34_MEASUREMENT_OUT_OF_TOLERANCE'; END IF;
  END LOOP;
  IF required_count=0 OR provided_count<>required_count THEN RAISE EXCEPTION 'ERR_F34_UNGOVERNED_MEASUREMENTS'; END IF;
 ELSE
  IF NEW.measurements_mm<>'{}'::jsonb THEN RAISE EXCEPTION 'ERR_F34_MEASUREMENTS_ON_NONPASS'; END IF;
 END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER qc_garment_guard_v21 BEFORE INSERT ON qc_garment_events_v21
 FOR EACH ROW EXECUTE FUNCTION guard_qc_garment_event_v21();

-- Freeze canonical part/recut genealogy once garment QC has begun. Use
-- the same lock as QC so concurrent late changes cannot pass an inspection.
CREATE FUNCTION qc_freeze_garment_parts_v21() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 PERFORM pg_advisory_xact_lock(hashtext(NEW.tenant_id),hashtext('F34:GARMENT:'||NEW.garment_id::text));
 IF EXISTS(SELECT 1 FROM qc_garment_events_v21 e WHERE e.tenant_id=NEW.tenant_id
   AND e.garment_id=NEW.garment_id) THEN
  RAISE EXCEPTION 'ERR_F34_GENEALOGY_FROZEN_AFTER_QC'; END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER qc_freeze_parts_v21 BEFORE INSERT ON gen_cut_parts_v18
 FOR EACH ROW EXECUTE FUNCTION qc_freeze_garment_parts_v21();
CREATE FUNCTION qc_freeze_recut_v21() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE original_garment uuid; replacement_garment uuid;
BEGIN
 SELECT p.garment_id INTO original_garment FROM gen_cut_parts_v18 p
  WHERE p.tenant_id=NEW.tenant_id AND p.id=NEW.original_part_id;
 SELECT p.garment_id INTO replacement_garment FROM gen_cut_parts_v18 p
  WHERE p.tenant_id=NEW.tenant_id AND p.id=NEW.replacement_part_id;
 IF original_garment IS NULL OR replacement_garment IS NULL THEN
  RAISE EXCEPTION 'ERR_F34_RECUT_PART_UNKNOWN'; END IF;
 -- Locks are acquired in stable lexical order to avoid cross-garment cycles.
 IF original_garment::text < replacement_garment::text THEN
  PERFORM pg_advisory_xact_lock(hashtext(NEW.tenant_id),hashtext('F34:GARMENT:'||original_garment::text));
  PERFORM pg_advisory_xact_lock(hashtext(NEW.tenant_id),hashtext('F34:GARMENT:'||replacement_garment::text));
 ELSE
  PERFORM pg_advisory_xact_lock(hashtext(NEW.tenant_id),hashtext('F34:GARMENT:'||replacement_garment::text));
  PERFORM pg_advisory_xact_lock(hashtext(NEW.tenant_id),hashtext('F34:GARMENT:'||original_garment::text));
 END IF;
 IF EXISTS (SELECT 1 FROM qc_garment_events_v21 e WHERE e.tenant_id=NEW.tenant_id
   AND e.garment_id IN(original_garment,replacement_garment)) THEN
  RAISE EXCEPTION 'ERR_F34_RECUT_AFTER_INSPECTION'; END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER qc_freeze_recut_v21 BEFORE INSERT ON cut_recut_links_v19
 FOR EACH ROW EXECUTE FUNCTION qc_freeze_recut_v21();

-- A released garment is immutable in sewing WIP custody. The added
-- per-garment lock serializes this check against QC release itself.
CREATE FUNCTION guard_wip_quality_release_v21() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE garment uuid; original_garment uuid;
BEGIN
 SELECT p.garment_id INTO garment FROM gen_cut_parts_v18 p
  WHERE p.tenant_id=NEW.tenant_id AND p.id=NEW.part_id;
 IF garment IS NULL THEN RAISE EXCEPTION 'ERR_F34_WIP_GARMENT_MISSING'; END IF;
 -- A recut part may belong to another reserved garment identity; also check
 -- the original garment whose missing part it replaces.
 PERFORM pg_advisory_xact_lock(hashtext(NEW.tenant_id),hashtext('F34:GARMENT:'||garment::text));
 IF EXISTS(SELECT 1 FROM qc_garment_events_v21 q WHERE q.tenant_id=NEW.tenant_id
   AND q.garment_id=garment AND q.action='QUALITY_RELEASE') THEN
  RAISE EXCEPTION 'ERR_F34_RELEASED_GARMENT_WIP_LOCKED'; END IF;
 SELECT original.garment_id INTO original_garment FROM cut_recut_links_v19 l
    JOIN gen_cut_parts_v18 original ON original.tenant_id=l.tenant_id AND original.id=l.original_part_id
    WHERE l.tenant_id=NEW.tenant_id AND l.replacement_part_id=NEW.part_id;
 IF original_garment IS NOT NULL THEN
  PERFORM pg_advisory_xact_lock(hashtext(NEW.tenant_id),hashtext('F34:GARMENT:'||original_garment::text));
  IF EXISTS (SELECT 1 FROM qc_garment_events_v21 q WHERE q.tenant_id=NEW.tenant_id
    AND q.garment_id=original_garment AND q.action='QUALITY_RELEASE') THEN
   RAISE EXCEPTION 'ERR_F34_RELEASED_RECUT_WIP_LOCKED'; END IF;
 END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER wip_quality_release_fence_v21 BEFORE INSERT ON wip_movements_v20
 FOR EACH ROW EXECUTE FUNCTION guard_wip_quality_release_v21();

DO $$ DECLARE tbl text; BEGIN
 FOREACH tbl IN ARRAY ARRAY['qc_garment_specs_v21','qc_garment_measure_rules_v21','qc_garment_defects_v21','qc_garment_events_v21'] LOOP
  EXECUTE format('ALTER TABLE public.%I ENABLE ROW LEVEL SECURITY',tbl);
  EXECUTE format('ALTER TABLE public.%I FORCE ROW LEVEL SECURITY',tbl);
  EXECUTE format('CREATE POLICY qc_v21_tenant ON public.%I FOR ALL TO agiparel_app USING (tenant_id=NULLIF(current_setting(''app.current_tenant_id'',true),'''')) WITH CHECK(tenant_id=NULLIF(current_setting(''app.current_tenant_id'',true),''''))',tbl);
  EXECUTE format('REVOKE ALL ON public.%I FROM PUBLIC,agiparel_app',tbl);
  EXECUTE format('GRANT SELECT ON public.%I TO agiparel_app',tbl);
 END LOOP;
END $$;
GRANT INSERT ON qc_garment_events_v21 TO agiparel_app;
COMMIT;
