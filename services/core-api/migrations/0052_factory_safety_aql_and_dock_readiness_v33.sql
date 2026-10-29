-- Author: Sunil Jain, CyberinQ LLC, 2026-10-29.
-- F3.19C/D + F3.20A: distinct physical safety clearance and internal AQL review.
-- Neither is carrier receipt, buyer acceptance, finished-garment, or shipment credit.
BEGIN;
INSERT INTO workforce_chapter_actions_v17(chapter_id,permission,authority) VALUES
 (7,'quality.needle.clear','SIGNED_WRITE'),(7,'quality.aql.review','SIGNED_WRITE')
 ON CONFLICT(permission) DO NOTHING;

CREATE TABLE needle_safety_clearances_v33 (
 tenant_id varchar(64) NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
 id uuid NOT NULL DEFAULT uuid_generate_v4(), plant_id varchar(64) NOT NULL,
 incident_id uuid NOT NULL, garment_id uuid NOT NULL, followup_pass_id uuid NOT NULL,
 accounted_fragments integer NOT NULL CHECK(accounted_fragments BETWEEN 0 AND 20),
 evidence_document_id varchar(36) NOT NULL, evidence_sha256 char(64) NOT NULL CHECK(evidence_sha256 ~ '^[0-9a-f]{64}$'),
 note text NOT NULL CHECK(length(btrim(note)) BETWEEN 20 AND 800),
 command_id varchar(80) NOT NULL CHECK(command_id ~ '^[A-Za-z0-9_-]{12,80}$'),
 command_sha256 char(64) NOT NULL CHECK(command_sha256 ~ '^[0-9a-f]{64}$'),
 reviewer_subject varchar(128) NOT NULL, reviewer_proof_id varchar(255) NOT NULL,
 assertion_sha256 char(64) NOT NULL CHECK(assertion_sha256 ~ '^[0-9a-f]{64}$'),
 recorded_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 authority text NOT NULL DEFAULT 'SUPERVISED_SAFETY_REVIEW_NOT_MACHINE_CERTIFICATION_OR_QC_RELEASE'
 CHECK(authority='SUPERVISED_SAFETY_REVIEW_NOT_MACHINE_CERTIFICATION_OR_QC_RELEASE'),
 PRIMARY KEY(tenant_id,id), UNIQUE(tenant_id,incident_id), UNIQUE(tenant_id,command_id),
 UNIQUE(tenant_id,reviewer_proof_id),
 FOREIGN KEY(tenant_id,incident_id) REFERENCES needle_safety_events_v32(tenant_id,id) ON DELETE RESTRICT,
 FOREIGN KEY(tenant_id,followup_pass_id) REFERENCES needle_safety_events_v32(tenant_id,id) ON DELETE RESTRICT,
 FOREIGN KEY(tenant_id,garment_id) REFERENCES gen_garments_v18(tenant_id,id) ON DELETE RESTRICT,
 FOREIGN KEY(tenant_id,evidence_document_id) REFERENCES document_sources_v65(tenant_id,id) ON DELETE RESTRICT
);
CREATE FUNCTION guard_needle_safety_clearance_v33() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE incident needle_safety_events_v32%ROWTYPE; followup needle_safety_events_v32%ROWTYPE; h char(64);
BEGIN
 PERFORM pg_advisory_xact_lock(hashtext(NEW.tenant_id),hashtext('F34:GARMENT:'||NEW.garment_id::text));
 SELECT * INTO incident FROM needle_safety_events_v32 WHERE tenant_id=NEW.tenant_id AND id=NEW.incident_id;
 SELECT * INTO followup FROM needle_safety_events_v32 WHERE tenant_id=NEW.tenant_id AND id=NEW.followup_pass_id;
 SELECT sha256 INTO h FROM document_sources_v65 WHERE tenant_id=NEW.tenant_id AND id=NEW.evidence_document_id;
 IF incident.id IS NULL OR incident.garment_id<>NEW.garment_id OR incident.plant_id<>NEW.plant_id
 OR incident.event_kind NOT IN('NEEDLE_BREAK','DETECTOR_CHALLENGE_FAIL','DETECTOR_REJECT')
 OR incident.actor_subject=NEW.reviewer_subject OR h IS NULL OR h<>NEW.evidence_sha256
 OR followup.id IS NULL OR followup.garment_id<>NEW.garment_id OR followup.plant_id<>NEW.plant_id
 OR followup.event_kind<>'DETECTOR_CHALLENGE_PASS' OR followup.recorded_at<=incident.recorded_at
 OR followup.actor_subject=NEW.reviewer_subject OR followup.actor_subject=incident.actor_subject
 OR NOT EXISTS(SELECT 1 FROM gen_garments_v18 g WHERE g.tenant_id=NEW.tenant_id AND g.id=NEW.garment_id AND g.plant_id=NEW.plant_id)
 OR (incident.event_kind='NEEDLE_BREAK' AND NEW.accounted_fragments<>incident.expected_fragments)
 OR (incident.event_kind<>'NEEDLE_BREAK' AND NEW.accounted_fragments<>0)
 THEN RAISE EXCEPTION 'ERR_F319C_INDEPENDENT_VERIFIED_FOLLOWUP_REQUIRED'; END IF;
 -- No reliance on client-side evidence or an arbitrary PASS. The PASS was itself
 -- admitted under the current calibrated-detector gate in F3.19B.
 IF EXISTS(SELECT 1 FROM needle_safety_clearances_v33 c WHERE c.tenant_id=NEW.tenant_id
   AND c.followup_pass_id=NEW.followup_pass_id AND c.incident_id<>NEW.incident_id)
 THEN RAISE EXCEPTION 'ERR_F319C_FOLLOWUP_ALREADY_CONSUMED'; END IF;
 NEW.recorded_at:=clock_timestamp(); RETURN NEW;
END $$;
CREATE TRIGGER needle_safety_clearance_guard_v33 BEFORE INSERT ON needle_safety_clearances_v33
 FOR EACH ROW EXECUTE FUNCTION guard_needle_safety_clearance_v33();
CREATE UNIQUE INDEX needle_followup_once_v33 ON needle_safety_clearances_v33(tenant_id,followup_pass_id);
CREATE OR REPLACE FUNCTION garment_needle_safety_blocked_v32(t text,g uuid) RETURNS boolean
 LANGUAGE sql VOLATILE AS $$
 SELECT EXISTS(SELECT 1 FROM needle_safety_events_v32 e
 WHERE e.tenant_id=t AND e.garment_id=g AND e.event_kind IN('NEEDLE_BREAK','DETECTOR_CHALLENGE_FAIL','DETECTOR_REJECT')
 AND NOT EXISTS(SELECT 1 FROM needle_safety_clearances_v33 c WHERE c.tenant_id=e.tenant_id AND c.incident_id=e.id))
$$;

-- Approval rules are provisioned, buyer-document-bound, append-only.
-- They evaluate samples, but an internal review is NOT external buyer acceptance.
CREATE TABLE buyer_aql_review_rules_v33 (
 tenant_id varchar(64) NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
 plant_id varchar(64) NOT NULL, plan_id uuid NOT NULL,
 max_minor integer NOT NULL CHECK(max_minor>=0),
 max_major integer NOT NULL CHECK(max_major>=0),
 max_critical integer NOT NULL DEFAULT 0 CHECK(max_critical=0),
 source_document_id varchar(128) NOT NULL,
 source_sha256 char(64) NOT NULL CHECK(source_sha256 ~ '^[0-9a-f]{64}$'),
 provisioned_by varchar(128) NOT NULL CHECK(length(btrim(provisioned_by))>0),
 enabled boolean NOT NULL DEFAULT false,
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(tenant_id,plan_id),
 FOREIGN KEY(tenant_id,plan_id) REFERENCES buyer_aql_plans_v32(tenant_id,id) ON DELETE RESTRICT,
 FOREIGN KEY(tenant_id,source_document_id) REFERENCES documents(tenant_id,id) ON DELETE RESTRICT
);
CREATE FUNCTION guard_buyer_aql_rules_v33() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE p buyer_aql_plans_v32%ROWTYPE; d documents%ROWTYPE;
BEGIN
 SELECT * INTO p FROM buyer_aql_plans_v32 WHERE tenant_id=NEW.tenant_id AND id=NEW.plan_id;
 SELECT * INTO d FROM documents WHERE tenant_id=NEW.tenant_id AND id=NEW.source_document_id;
 IF p.id IS NULL OR p.plant_id<>NEW.plant_id OR p.source_document_id<>NEW.source_document_id
 OR p.source_sha256<>NEW.source_sha256 OR d.id IS NULL OR d.status<>'APPROVED'
 OR upper(d.issuer)<>'CUSTOMER' OR d.file_checksum<>NEW.source_sha256
 OR NEW.max_minor+NEW.max_major>=cardinality(p.sample_garment_ids)
 THEN RAISE EXCEPTION 'ERR_F319D_AQL_RULES_SOURCE_OR_LIMITS'; END IF;
 NEW.created_at:=clock_timestamp(); RETURN NEW;
END $$;
CREATE TRIGGER buyer_aql_rule_guard_v33 BEFORE INSERT ON buyer_aql_review_rules_v33
 FOR EACH ROW EXECUTE FUNCTION guard_buyer_aql_rules_v33();

CREATE TABLE buyer_aql_reviews_v33 (
 tenant_id varchar(64) NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
 id uuid NOT NULL DEFAULT uuid_generate_v4(), plant_id varchar(64) NOT NULL,
 plan_id uuid NOT NULL, decision varchar(30) NOT NULL CHECK(decision IN ('INTERNAL_RECOMMEND_ACCEPT','HOLD_FOR_BUYER_REVIEW')),
 pass_count integer NOT NULL, minor_count integer NOT NULL, major_count integer NOT NULL, critical_count integer NOT NULL,
 note text NOT NULL CHECK(length(btrim(note)) BETWEEN 20 AND 800),
 command_id varchar(80) NOT NULL CHECK(command_id ~ '^[A-Za-z0-9_-]{12,80}$'),
 command_sha256 char(64) NOT NULL CHECK(command_sha256 ~ '^[0-9a-f]{64}$'),
 reviewer_subject varchar(128) NOT NULL, reviewer_proof_id varchar(255) NOT NULL,
 assertion_sha256 char(64) NOT NULL CHECK(assertion_sha256 ~ '^[0-9a-f]{64}$'),
 recorded_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 authority text NOT NULL DEFAULT 'INTERNAL_AQL_REVIEW_NOT_BUYER_LOT_ACCEPTANCE_OR_SHIPMENT'
 CHECK(authority='INTERNAL_AQL_REVIEW_NOT_BUYER_LOT_ACCEPTANCE_OR_SHIPMENT'),
 PRIMARY KEY(tenant_id,id), UNIQUE(tenant_id,plan_id), UNIQUE(tenant_id,command_id), UNIQUE(tenant_id,reviewer_proof_id),
 FOREIGN KEY(tenant_id,plan_id) REFERENCES buyer_aql_plans_v32(tenant_id,id) ON DELETE RESTRICT
);
CREATE FUNCTION guard_buyer_aql_review_v33() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE p buyer_aql_plans_v32%ROWTYPE; r buyer_aql_review_rules_v33%ROWTYPE;
 pass_count bigint; minor_count bigint; major_count bigint; critical_count bigint; total bigint; latest integer;
BEGIN
 PERFORM pg_advisory_xact_lock(hashtext(NEW.tenant_id),hashtext('F319D:AQL:'||NEW.plan_id::text));
 SELECT * INTO p FROM buyer_aql_plans_v32 WHERE tenant_id=NEW.tenant_id AND id=NEW.plan_id;
 SELECT * INTO r FROM buyer_aql_review_rules_v33 WHERE tenant_id=NEW.tenant_id AND plan_id=NEW.plan_id;
 SELECT max(revision) INTO latest FROM buyer_aql_plans_v32 WHERE tenant_id=NEW.tenant_id AND plant_id=p.plant_id AND lot_code=p.lot_code;
 SELECT count(*),count(*) FILTER(WHERE finding='PASS'),count(*) FILTER(WHERE finding='MINOR'),
 count(*) FILTER(WHERE finding='MAJOR'),count(*) FILTER(WHERE finding='CRITICAL')
 INTO total,pass_count,minor_count,major_count,critical_count FROM buyer_aql_sample_events_v32
 WHERE tenant_id=NEW.tenant_id AND plan_id=NEW.plan_id;
 IF p.id IS NULL OR NOT p.enabled OR p.revision<>latest OR p.plant_id<>NEW.plant_id
 OR r.plan_id IS NULL OR NOT r.enabled OR total<>cardinality(p.sample_garment_ids)
 OR NOT EXISTS(SELECT 1 FROM documents d WHERE d.tenant_id=p.tenant_id AND d.id=p.source_document_id
  AND d.status='APPROVED' AND upper(d.issuer)='CUSTOMER' AND d.file_checksum=p.source_sha256)
 OR EXISTS(SELECT 1 FROM buyer_aql_sample_events_v32 e WHERE e.tenant_id=NEW.tenant_id
  AND e.plan_id=NEW.plan_id AND e.inspector_subject=NEW.reviewer_subject)
 OR NEW.pass_count<>pass_count OR NEW.minor_count<>minor_count OR NEW.major_count<>major_count
 OR NEW.critical_count<>critical_count
 OR (NEW.decision='INTERNAL_RECOMMEND_ACCEPT' AND
   (minor_count>r.max_minor OR major_count>r.max_major OR critical_count>r.max_critical))
 THEN RAISE EXCEPTION 'ERR_F319D_REVIEW_INCOMPLETE_OR_UNAUTHORIZED'; END IF;
 NEW.recorded_at:=clock_timestamp(); RETURN NEW;
END $$;
CREATE TRIGGER buyer_aql_review_guard_v33 BEFORE INSERT ON buyer_aql_reviews_v33
 FOR EACH ROW EXECUTE FUNCTION guard_buyer_aql_review_v33();

-- Share the F3.19B proof lock namespace to prevent reusing an older
-- signed JWT assertion as a new clearance or AQL review.
CREATE FUNCTION guard_factory_review_proof_v33() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE proof text;
BEGIN
 proof:=NEW.reviewer_proof_id;
 PERFORM pg_advisory_xact_lock(hashtext(NEW.tenant_id),hashtext('F32:PROOF:'||proof));
 IF EXISTS(SELECT 1 FROM needle_safety_events_v32 e WHERE e.tenant_id=NEW.tenant_id AND e.proof_id=proof)
 OR EXISTS(SELECT 1 FROM buyer_aql_sample_events_v32 e WHERE e.tenant_id=NEW.tenant_id AND e.inspector_proof_id=proof)
 OR EXISTS(SELECT 1 FROM fabric_calibration_links_v32 e WHERE e.tenant_id=NEW.tenant_id AND e.inspector_proof_id=proof)
 OR EXISTS(SELECT 1 FROM needle_safety_clearances_v33 e WHERE e.tenant_id=NEW.tenant_id AND e.reviewer_proof_id=proof)
 OR EXISTS(SELECT 1 FROM buyer_aql_reviews_v33 e WHERE e.tenant_id=NEW.tenant_id AND e.reviewer_proof_id=proof)
 THEN RAISE EXCEPTION 'ERR_F33_ASSERTION_REUSED'; END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER safety_review_proof_v33 BEFORE INSERT ON needle_safety_clearances_v33
 FOR EACH ROW EXECUTE FUNCTION guard_factory_review_proof_v33();
CREATE TRIGGER aql_review_proof_v33 BEFORE INSERT ON buyer_aql_reviews_v33
 FOR EACH ROW EXECUTE FUNCTION guard_factory_review_proof_v33();

DO $$ DECLARE tbl text; BEGIN
 FOREACH tbl IN ARRAY ARRAY['needle_safety_clearances_v33','buyer_aql_review_rules_v33','buyer_aql_reviews_v33'] LOOP
  EXECUTE format('ALTER TABLE public.%I ENABLE ROW LEVEL SECURITY',tbl);
  EXECUTE format('ALTER TABLE public.%I FORCE ROW LEVEL SECURITY',tbl);
  EXECUTE format('CREATE POLICY factory_tenant_v33 ON public.%I FOR ALL TO agiparel_app USING (tenant_id=NULLIF(current_setting(%L,true),%L)) WITH CHECK(tenant_id=NULLIF(current_setting(%L,true),%L))',tbl,'app.current_tenant_id','','app.current_tenant_id','');
  EXECUTE format('REVOKE ALL ON public.%I FROM PUBLIC,agiparel_app',tbl);
  EXECUTE format('GRANT SELECT ON public.%I TO agiparel_app',tbl);
  EXECUTE format('CREATE TRIGGER immutable_factory_v33 BEFORE UPDATE OR DELETE ON public.%I FOR EACH ROW EXECUTE FUNCTION protect_cut_draft_v8()',tbl);
 END LOOP;
END $$;
GRANT INSERT ON needle_safety_clearances_v33,buyer_aql_reviews_v33 TO agiparel_app;
-- Provisioner alone can insert review rules; no application DML on rules.
COMMIT;
