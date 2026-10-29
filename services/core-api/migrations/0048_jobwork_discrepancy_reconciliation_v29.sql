-- Author: Sunil Jain, CyberinQ LLC, 2026-10-29.
-- F3.17B: immutable signed discrepancy claims for existing per-part job-work custody.
-- A reported shortage is not a proven loss; a damaged receipt is not an accepted QC result.
-- No financial charge, vendor signature, inventory correction or automatic release.
BEGIN;
-- A STABLE SQL helper can reuse a statement-start MVCC snapshot even after
-- waiting on an advisory lock. Re-evaluate F3.17A custody/replay queries with
-- the latest READ COMMITTED view after the serialization lock is acquired.
ALTER FUNCTION jobwork_part_offsite_v28(text,uuid) VOLATILE;
ALTER FUNCTION jobwork_scan_spent_v28(text,uuid) VOLATILE;
INSERT INTO workforce_chapter_actions_v17(chapter_id,permission,authority)
 VALUES(6,'jobwork.discrepancy.report','SIGNED_WRITE') ON CONFLICT DO NOTHING;

CREATE TABLE jobwork_discrepancies_v29 (
 tenant_id varchar(64) NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
 id uuid NOT NULL DEFAULT uuid_generate_v4(),
 outward_id uuid NOT NULL,
 plant_id varchar(64) NOT NULL,
 part_id uuid NOT NULL,
 category varchar(32) NOT NULL CHECK(category IN ('MISSING_AT_RECONCILIATION','DAMAGED_ON_RETURN')),
 note text NOT NULL CHECK(length(btrim(note)) BETWEEN 20 AND 1000),
 evidence_ref varchar(180) NOT NULL CHECK(length(btrim(evidence_ref)) BETWEEN 8 AND 180),
 evidence_sha256 char(64) NOT NULL CHECK(evidence_sha256 ~ '^[0-9a-f]{64}$'),
 command_id varchar(80) NOT NULL CHECK(command_id ~ '^[A-Za-z0-9_-]{12,80}$'),
 command_sha256 char(64) NOT NULL CHECK(command_sha256 ~ '^[0-9a-f]{64}$'),
 reporter_subject varchar(128) NOT NULL CHECK(length(btrim(reporter_subject))>0),
 reporter_proof_id varchar(255) NOT NULL CHECK(length(btrim(reporter_proof_id))>0),
 assertion_sha256 char(64) NOT NULL CHECK(assertion_sha256 ~ '^[0-9a-f]{64}$'),
 reported_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 authority text NOT NULL DEFAULT 'SIGNED_HUMAN_DISCREPANCY_REPORT_NOT_VENDOR_CERTIFIED_OR_QC_RELEASED'
 CHECK(authority='SIGNED_HUMAN_DISCREPANCY_REPORT_NOT_VENDOR_CERTIFIED_OR_QC_RELEASED'),
 PRIMARY KEY(tenant_id,id), UNIQUE(tenant_id,outward_id), UNIQUE(tenant_id,command_id),
 UNIQUE(tenant_id,reporter_proof_id),
 FOREIGN KEY(tenant_id,outward_id) REFERENCES jobwork_outwards_v28(tenant_id,id) ON DELETE RESTRICT,
 FOREIGN KEY(tenant_id,part_id) REFERENCES gen_cut_parts_v18(tenant_id,id) ON DELETE RESTRICT
);
CREATE INDEX jobwork_discrepancies_part_v29 ON jobwork_discrepancies_v29(tenant_id,plant_id,part_id,reported_at DESC);

-- Every discrepancy remains an unresolved hold in F3.17B. Later releases may add
-- a separate signed, evidence-verified resolution ledger; never mutate this row.
CREATE FUNCTION jobwork_part_disputed_v29(t text,p uuid) RETURNS boolean
 LANGUAGE sql VOLATILE AS $$
 SELECT EXISTS(SELECT 1 FROM jobwork_discrepancies_v29 d
  WHERE d.tenant_id=t AND d.part_id=p)
$$;

CREATE FUNCTION guard_jobwork_discrepancy_v29() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE o jobwork_outwards_v28%ROWTYPE; g uuid; lay uuid; pos integer; grp integer;
        inward_actor varchar(128);
BEGIN
 SELECT * INTO o FROM jobwork_outwards_v28 WHERE tenant_id=NEW.tenant_id AND id=NEW.outward_id;
 IF o.id IS NULL OR o.plant_id<>NEW.plant_id OR o.part_id<>NEW.part_id
 THEN RAISE EXCEPTION 'ERR_F317B_OUTWARD_SCOPE'; END IF;
 SELECT p.garment_id,garm.lay_draft_id,garm.marker_position INTO g,lay,pos
  FROM gen_cut_parts_v18 p JOIN gen_garments_v18 garm
   ON garm.tenant_id=p.tenant_id AND garm.id=p.garment_id
  WHERE p.tenant_id=NEW.tenant_id AND p.id=NEW.part_id;
 SELECT min(b.ply_group) INTO grp FROM cut_bundle_drafts_v8 b JOIN gen_garments_v18 x
  ON x.tenant_id=b.tenant_id AND x.lay_draft_id=b.lay_draft_id
  WHERE x.tenant_id=NEW.tenant_id AND x.id=g
   AND b.marker_position=pos AND x.ply_number BETWEEN b.ply_start AND b.ply_end;
 IF g IS NULL OR grp IS NULL THEN RAISE EXCEPTION 'ERR_F317B_PART_GENEALOGY'; END IF;
 PERFORM pg_advisory_xact_lock(hashtext(NEW.tenant_id),hashtext('F316:GROUP:'||lay::text||':'||pos||':'||grp));
 PERFORM pg_advisory_xact_lock(hashtext(NEW.tenant_id),hashtext('F34:GARMENT:'||g::text));
 PERFORM pg_advisory_xact_lock(hashtext(NEW.tenant_id),hashtext('WIP:PART:'||NEW.part_id::text));
 -- Do not backdate a disputed state into already released or packed work.
 IF EXISTS(SELECT 1 FROM qc_garment_events_v21 q
   WHERE q.tenant_id=NEW.tenant_id AND q.garment_id=g AND q.action='QUALITY_RELEASE')
 OR EXISTS(SELECT 1 FROM pack_carton_events_v22 pk
   WHERE pk.tenant_id=NEW.tenant_id AND pk.garment_id=g AND pk.action='ADD')
 THEN RAISE EXCEPTION 'ERR_F317B_ALREADY_RELEASED'; END IF;
 IF o.sender_subject=NEW.reporter_subject THEN
  RAISE EXCEPTION 'ERR_F317B_REPORTER_NOT_INDEPENDENT'; END IF;
 SELECT receiver_subject INTO inward_actor FROM jobwork_inwards_v28
  WHERE tenant_id=NEW.tenant_id AND outward_id=NEW.outward_id;
 IF (NEW.category='MISSING_AT_RECONCILIATION' AND inward_actor IS NOT NULL)
 OR (NEW.category='DAMAGED_ON_RETURN' AND inward_actor IS NULL)
 THEN RAISE EXCEPTION 'ERR_F317B_DISCREPANCY_CONTRADICTS_CUSTODY'; END IF;
 IF inward_actor IS NOT NULL AND inward_actor=NEW.reporter_subject THEN
  RAISE EXCEPTION 'ERR_F317B_REPORTER_NOT_INDEPENDENT'; END IF;
 IF EXISTS(SELECT 1 FROM jobwork_outwards_v28 later
  WHERE later.tenant_id=NEW.tenant_id AND later.part_id=NEW.part_id
   AND (later.recorded_at,later.id)>(o.recorded_at,o.id))
 THEN RAISE EXCEPTION 'ERR_F317B_NOT_LATEST_CYCLE'; END IF;
 -- Single-use evidence across the job-work family, regardless of action.
 IF EXISTS(SELECT 1 FROM jobwork_used_proofs_v28 p
   WHERE p.tenant_id=NEW.tenant_id AND p.proof_id=NEW.reporter_proof_id)
 THEN RAISE EXCEPTION 'ERR_F317B_REUSED_PROOF'; END IF;
 NEW.reported_at := clock_timestamp();
 RETURN NEW;
END $$;
CREATE TRIGGER jobwork_discrepancy_guard_v29 BEFORE INSERT ON jobwork_discrepancies_v29
 FOR EACH ROW EXECUTE FUNCTION guard_jobwork_discrepancy_v29();

-- Prevent a reported damaged return from silently continuing in the factory.
-- A missing part already remains offsite under F3.17A; the additional gate handles
-- both reported categories and blocks a repeat outward for damaged returns.
CREATE FUNCTION guard_jobwork_disputed_progress_v29() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE garment uuid; part uuid; lay uuid;
BEGIN
 IF TG_TABLE_NAME='jobwork_outwards_v28' OR TG_TABLE_NAME='wip_movements_v20' THEN
  part:=NEW.part_id;
  PERFORM pg_advisory_xact_lock(hashtext(NEW.tenant_id),hashtext('WIP:PART:'||part::text));
  IF jobwork_part_disputed_v29(NEW.tenant_id,part) THEN
    RAISE EXCEPTION 'ERR_F317B_DISPUTED_PART_BLOCKED'; END IF;
 ELSIF TG_TABLE_NAME='sew_prod_completions_v10' THEN
  SELECT r.lay_draft_id INTO lay FROM sew_prod_routes_v10 r
   WHERE r.tenant_id=NEW.tenant_id AND r.id=NEW.route_id;
  PERFORM pg_advisory_xact_lock(hashtext(NEW.tenant_id),hashtext('F316:GROUP:'||lay::text||':'||NEW.marker_position||':'||NEW.ply_group));
  IF EXISTS(SELECT 1 FROM jobwork_discrepancies_v29 d
    JOIN gen_cut_parts_v18 p ON p.tenant_id=d.tenant_id AND p.id=d.part_id
    JOIN gen_garments_v18 g ON g.tenant_id=p.tenant_id AND g.id=p.garment_id
    JOIN cut_bundle_drafts_v8 b ON b.tenant_id=g.tenant_id AND b.lay_draft_id=g.lay_draft_id
     AND b.marker_position=g.marker_position AND g.ply_number BETWEEN b.ply_start AND b.ply_end
    WHERE d.tenant_id=NEW.tenant_id AND g.lay_draft_id=lay
      AND g.marker_position=NEW.marker_position AND b.ply_group=NEW.ply_group)
  THEN RAISE EXCEPTION 'ERR_F317B_DISPUTED_GROUP_BLOCKED'; END IF;
 ELSIF TG_TABLE_NAME IN ('sew_piece_completions_v27','qc_garment_events_v21','pack_carton_events_v22') THEN
  IF TG_TABLE_NAME='pack_carton_events_v22' AND NEW.action<>'ADD' THEN RETURN NEW; END IF;
  -- A quality HOLD or REPAIR_COMPLETE may be recorded to document a defect;
  -- passing/finishing/release is forbidden while the external dispute is unresolved.
  IF TG_TABLE_NAME='qc_garment_events_v21' AND NEW.action IN ('INSPECT_HOLD','REINSPECT_HOLD','REPAIR_COMPLETE') THEN RETURN NEW; END IF;
  garment:=NEW.garment_id;
  PERFORM pg_advisory_xact_lock(hashtext(NEW.tenant_id),hashtext('F34:GARMENT:'||garment::text));
  IF EXISTS(SELECT 1 FROM gen_cut_parts_v18 p
    WHERE p.tenant_id=NEW.tenant_id AND p.garment_id=garment
     AND jobwork_part_disputed_v29(p.tenant_id,p.id))
  THEN RAISE EXCEPTION 'ERR_F317B_DISPUTED_GARMENT_BLOCKED'; END IF;
 END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER zzz_jobwork_disputed_out_v29 BEFORE INSERT ON jobwork_outwards_v28
 FOR EACH ROW EXECUTE FUNCTION guard_jobwork_disputed_progress_v29();
CREATE TRIGGER zzz_jobwork_disputed_move_v29 BEFORE INSERT ON wip_movements_v20
 FOR EACH ROW EXECUTE FUNCTION guard_jobwork_disputed_progress_v29();
CREATE TRIGGER zzz_jobwork_disputed_group_v29 BEFORE INSERT ON sew_prod_completions_v10
 FOR EACH ROW EXECUTE FUNCTION guard_jobwork_disputed_progress_v29();
CREATE TRIGGER zzz_jobwork_disputed_piece_v29 BEFORE INSERT ON sew_piece_completions_v27
 FOR EACH ROW EXECUTE FUNCTION guard_jobwork_disputed_progress_v29();
CREATE TRIGGER zzz_jobwork_disputed_qc_v29 BEFORE INSERT ON qc_garment_events_v21
 FOR EACH ROW EXECUTE FUNCTION guard_jobwork_disputed_progress_v29();
CREATE TRIGGER zzz_jobwork_disputed_pack_v29 BEFORE INSERT ON pack_carton_events_v22
 FOR EACH ROW EXECUTE FUNCTION guard_jobwork_disputed_progress_v29();

ALTER TABLE jobwork_discrepancies_v29 ENABLE ROW LEVEL SECURITY;
ALTER TABLE jobwork_discrepancies_v29 FORCE ROW LEVEL SECURITY;
CREATE POLICY jobwork_discrepancy_tenant_v29 ON jobwork_discrepancies_v29
 FOR ALL TO agiparel_app
 USING (tenant_id=NULLIF(current_setting('app.current_tenant_id',true),''))
 WITH CHECK (tenant_id=NULLIF(current_setting('app.current_tenant_id',true),''));
REVOKE ALL ON jobwork_discrepancies_v29 FROM PUBLIC,agiparel_app;
GRANT SELECT,INSERT ON jobwork_discrepancies_v29 TO agiparel_app;
CREATE TRIGGER jobwork_discrepancy_immutable_v29 BEFORE UPDATE OR DELETE ON jobwork_discrepancies_v29
 FOR EACH ROW EXECUTE FUNCTION protect_cut_draft_v8();
COMMIT;
