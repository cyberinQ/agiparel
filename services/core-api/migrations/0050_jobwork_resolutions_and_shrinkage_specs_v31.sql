-- Author: Sunil Jain, CyberinQ LLC, 2026-10-29.
-- F3.17D + F3.18B. Run after 0049, once, inside one transaction.
-- Signed administrative disposition != vendor payment, physical QC release or shipment.
-- Provisioner-owned approved shrinkage limits != a cutting authorization.
BEGIN;
INSERT INTO workforce_chapter_actions_v17(chapter_id,permission,authority)
 VALUES(6,'jobwork.discrepancy.resolve','SIGNED_WRITE') ON CONFLICT(permission) DO NOTHING;

CREATE TABLE jobwork_discrepancy_resolutions_v31 (
 tenant_id varchar(64) NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
 id uuid NOT NULL DEFAULT uuid_generate_v4(),
 discrepancy_id uuid NOT NULL,
 outward_id uuid NOT NULL,
 part_id uuid NOT NULL,
 plant_id varchar(64) NOT NULL,
 resolution_kind varchar(40) NOT NULL CHECK(resolution_kind IN
 ('RETURNED_TO_PIECE_HOLD','SHORTAGE_REPORTED_REMAINS_BLOCKED','DAMAGE_REPORTED_REMAINS_BLOCKED')),
 evidence_ref varchar(180) NOT NULL CHECK(length(btrim(evidence_ref)) BETWEEN 8 AND 180),
 evidence_sha256 char(64) NOT NULL CHECK(evidence_sha256 ~ '^[0-9a-f]{64}$'),
 note text NOT NULL CHECK(length(btrim(note)) BETWEEN 20 AND 1000),
 command_id varchar(80) NOT NULL CHECK(command_id ~ '^[A-Za-z0-9_-]{12,80}$'),
 command_sha256 char(64) NOT NULL CHECK(command_sha256 ~ '^[0-9a-f]{64}$'),
 supervisor_subject varchar(128) NOT NULL CHECK(length(btrim(supervisor_subject))>0),
 supervisor_proof_id varchar(255) NOT NULL CHECK(length(btrim(supervisor_proof_id))>0),
 assertion_sha256 char(64) NOT NULL CHECK(assertion_sha256 ~ '^[0-9a-f]{64}$'),
 recorded_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 authority text NOT NULL DEFAULT 'INDEPENDENT_DISCREPANCY_DISPOSITION_NOT_QC_OR_SETTLEMENT'
  CHECK(authority='INDEPENDENT_DISCREPANCY_DISPOSITION_NOT_QC_OR_SETTLEMENT'),
 PRIMARY KEY(tenant_id,id),
 UNIQUE(tenant_id,discrepancy_id), UNIQUE(tenant_id,command_id), UNIQUE(tenant_id,supervisor_proof_id),
 FOREIGN KEY(tenant_id,discrepancy_id) REFERENCES jobwork_discrepancies_v29(tenant_id,id) ON DELETE RESTRICT,
 FOREIGN KEY(tenant_id,outward_id) REFERENCES jobwork_outwards_v28(tenant_id,id) ON DELETE RESTRICT,
 FOREIGN KEY(tenant_id,part_id) REFERENCES gen_cut_parts_v18(tenant_id,id) ON DELETE RESTRICT
);
CREATE INDEX jobwork_disposition_part_v31 ON jobwork_discrepancy_resolutions_v31(tenant_id,part_id,recorded_at DESC);

CREATE FUNCTION guard_jobwork_disposition_v31() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE d jobwork_discrepancies_v29%ROWTYPE;
        o jobwork_outwards_v28%ROWTYPE;
        ret jobwork_inwards_v28%ROWTYPE;
        garment uuid; lay uuid; pos integer; grp integer;
BEGIN
 SELECT * INTO d FROM jobwork_discrepancies_v29 WHERE tenant_id=NEW.tenant_id AND id=NEW.discrepancy_id;
 IF d.id IS NULL OR d.outward_id<>NEW.outward_id OR d.part_id<>NEW.part_id OR d.plant_id<>NEW.plant_id
 THEN RAISE EXCEPTION 'ERR_F317D_DISCREPANCY_SCOPE'; END IF;
 SELECT * INTO o FROM jobwork_outwards_v28 WHERE tenant_id=NEW.tenant_id AND id=NEW.outward_id;
 IF o.id IS NULL OR o.part_id<>NEW.part_id OR o.plant_id<>NEW.plant_id
 THEN RAISE EXCEPTION 'ERR_F317D_OUTWARD_SCOPE'; END IF;
 SELECT p.garment_id,g.lay_draft_id,g.marker_position INTO garment,lay,pos
  FROM gen_cut_parts_v18 p JOIN gen_garments_v18 g ON g.tenant_id=p.tenant_id AND g.id=p.garment_id
  WHERE p.tenant_id=NEW.tenant_id AND p.id=NEW.part_id;
 SELECT min(b.ply_group) INTO grp FROM cut_bundle_drafts_v8 b JOIN gen_garments_v18 g
  ON g.tenant_id=b.tenant_id AND g.lay_draft_id=b.lay_draft_id
  WHERE g.tenant_id=NEW.tenant_id AND g.id=garment AND b.marker_position=pos
    AND g.ply_number BETWEEN b.ply_start AND b.ply_end;
 IF garment IS NULL OR grp IS NULL THEN RAISE EXCEPTION 'ERR_F317D_GENEALOGY'; END IF;
 -- Same serialization boundary as earlier piece holds and job-work transactions.
 PERFORM pg_advisory_xact_lock(hashtext(NEW.tenant_id),hashtext('F316:GROUP:'||lay::text||':'||pos||':'||grp));
 PERFORM pg_advisory_xact_lock(hashtext(NEW.tenant_id),hashtext('F34:GARMENT:'||garment::text));
 PERFORM pg_advisory_xact_lock(hashtext(NEW.tenant_id),hashtext('WIP:PART:'||NEW.part_id::text));
 IF EXISTS(SELECT 1 FROM jobwork_discrepancy_resolutions_v31 r
   WHERE r.tenant_id=NEW.tenant_id AND r.discrepancy_id=NEW.discrepancy_id)
 THEN RAISE EXCEPTION 'ERR_F317D_ALREADY_DISPOSED'; END IF;
 SELECT * INTO ret FROM jobwork_inwards_v28 WHERE tenant_id=NEW.tenant_id AND outward_id=NEW.outward_id;
 IF NEW.supervisor_subject IN (d.reporter_subject,o.sender_subject)
    OR (ret.id IS NOT NULL AND NEW.supervisor_subject=ret.receiver_subject)
 THEN RAISE EXCEPTION 'ERR_F317D_INDEPENDENCE'; END IF;
 IF EXISTS(SELECT 1 FROM jobwork_used_proofs_v28 u WHERE u.tenant_id=NEW.tenant_id AND u.proof_id=NEW.supervisor_proof_id)
 OR EXISTS(SELECT 1 FROM jobwork_discrepancies_v29 x WHERE x.tenant_id=NEW.tenant_id AND x.reporter_proof_id=NEW.supervisor_proof_id)
 OR EXISTS(SELECT 1 FROM jobwork_vendor_acks_v30 a WHERE a.tenant_id=NEW.tenant_id AND a.recorder_proof_id=NEW.supervisor_proof_id)
 OR EXISTS(SELECT 1 FROM wip_piece_holds_v26 h WHERE h.tenant_id=NEW.tenant_id AND h.reporter_proof_id=NEW.supervisor_proof_id)
 OR EXISTS(SELECT 1 FROM wip_piece_reviews_v26 h WHERE h.tenant_id=NEW.tenant_id AND h.supervisor_proof_id=NEW.supervisor_proof_id)
 THEN RAISE EXCEPTION 'ERR_F317D_PROOF_SPENT'; END IF;
 IF NEW.resolution_kind='RETURNED_TO_PIECE_HOLD' THEN
  -- Recorded inward must exist. A missing-item report requires return AFTER report;
  -- damaged-on-return has its own prior inward. The active hold is a SECOND ledger.
  IF ret.id IS NULL OR (d.category='MISSING_AT_RECONCILIATION' AND ret.recorded_at<=d.reported_at)
    OR NOT EXISTS(SELECT 1 FROM wip_piece_holds_v26 h
       WHERE h.tenant_id=NEW.tenant_id AND h.garment_id=garment AND h.opened_at>=d.reported_at
        AND NOT EXISTS(SELECT 1 FROM wip_piece_reviews_v26 r WHERE r.tenant_id=h.tenant_id AND r.hold_id=h.id))
  THEN RAISE EXCEPTION 'ERR_F317D_PHYSICAL_RETURN_AND_ACTIVE_HOLD_REQUIRED'; END IF;
 ELSIF NEW.resolution_kind='SHORTAGE_REPORTED_REMAINS_BLOCKED' THEN
  IF d.category<>'MISSING_AT_RECONCILIATION' OR ret.id IS NOT NULL
  THEN RAISE EXCEPTION 'ERR_F317D_NOT_OUTSTANDING'; END IF;
 ELSIF NEW.resolution_kind='DAMAGE_REPORTED_REMAINS_BLOCKED' THEN
  IF d.category<>'DAMAGED_ON_RETURN' OR ret.id IS NULL
  THEN RAISE EXCEPTION 'ERR_F317D_NO_DAMAGE_RETURN'; END IF;
 END IF;
 NEW.recorded_at:=clock_timestamp();
 RETURN NEW;
END $$;
CREATE TRIGGER jobwork_disposition_guard_v31 BEFORE INSERT ON jobwork_discrepancy_resolutions_v31
 FOR EACH ROW EXECUTE FUNCTION guard_jobwork_disposition_v31();

-- Only an independently supervised, physically returned part routed into
-- an ACTIVE F3.16 piece HOLD stops being a job-work dispute. WIP/QC remains
-- blocked by the independent piece hold until its own scan-backed review.
CREATE OR REPLACE FUNCTION jobwork_part_disputed_v29(t text,p uuid) RETURNS boolean
 LANGUAGE sql VOLATILE AS $$
 SELECT EXISTS(SELECT 1 FROM jobwork_discrepancies_v29 d
  WHERE d.tenant_id=t AND d.part_id=p AND NOT EXISTS(
    SELECT 1 FROM jobwork_discrepancy_resolutions_v31 r
    WHERE r.tenant_id=d.tenant_id AND r.discrepancy_id=d.id
      AND r.resolution_kind='RETURNED_TO_PIECE_HOLD'))
$$;
-- Restore the F3.17B whole-group guard to consult the effective open dispute,
-- not merely the immutable historical discrepancy row.
CREATE OR REPLACE FUNCTION guard_jobwork_disputed_progress_v29() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE garment uuid; part uuid; lay uuid;
BEGIN
 IF TG_TABLE_NAME IN ('jobwork_outwards_v28','wip_movements_v20') THEN
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
      AND g.marker_position=NEW.marker_position AND b.ply_group=NEW.ply_group
      AND jobwork_part_disputed_v29(d.tenant_id,d.part_id))
  THEN RAISE EXCEPTION 'ERR_F317B_DISPUTED_GROUP_BLOCKED'; END IF;
 ELSIF TG_TABLE_NAME IN ('sew_piece_completions_v27','qc_garment_events_v21','pack_carton_events_v22') THEN
  IF TG_TABLE_NAME='pack_carton_events_v22' AND NEW.action<>'ADD' THEN RETURN NEW; END IF;
  IF TG_TABLE_NAME='qc_garment_events_v21' AND NEW.action IN ('INSPECT_HOLD','REINSPECT_HOLD','REPAIR_COMPLETE') THEN RETURN NEW; END IF;
  garment:=NEW.garment_id;
  PERFORM pg_advisory_xact_lock(hashtext(NEW.tenant_id),hashtext('F34:GARMENT:'||garment::text));
  IF EXISTS(SELECT 1 FROM gen_cut_parts_v18 p WHERE p.tenant_id=NEW.tenant_id AND p.garment_id=garment
    AND jobwork_part_disputed_v29(p.tenant_id,p.id))
  THEN RAISE EXCEPTION 'ERR_F317B_DISPUTED_GARMENT_BLOCKED'; END IF;
 END IF;
 RETURN NEW;
END $$;

-- Provisioner-only versioned customer-approved spec. The file-checksum is
-- validated against the EXISTING approved document on every insertion.
CREATE TABLE fabric_shrinkage_specs_v31 (
 tenant_id varchar(64) NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
 plant_id varchar(64) NOT NULL,
 material_sku varchar(128) NOT NULL,
 treatment_code varchar(32) NOT NULL CHECK(treatment_code IN ('WASH','STEAM','RELAXATION','OTHER')),
 revision integer NOT NULL CHECK(revision BETWEEN 1 AND 100000),
 warp_min_bp integer NOT NULL CHECK(warp_min_bp BETWEEN -3000 AND 3000),
 warp_max_bp integer NOT NULL CHECK(warp_max_bp BETWEEN -3000 AND 3000),
 weft_min_bp integer NOT NULL CHECK(weft_min_bp BETWEEN -3000 AND 3000),
 weft_max_bp integer NOT NULL CHECK(weft_max_bp BETWEEN -3000 AND 3000),
 minimum_samples integer NOT NULL CHECK(minimum_samples BETWEEN 1 AND 32),
 source_document_id varchar(128) NOT NULL,
 source_sha256 char(64) NOT NULL CHECK(source_sha256 ~ '^[0-9a-f]{64}$'),
 provisioned_by varchar(128) NOT NULL CHECK(length(btrim(provisioned_by))>0),
 approved_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 enabled boolean NOT NULL DEFAULT false,
 CHECK(warp_min_bp<=warp_max_bp AND weft_min_bp<=weft_max_bp),
 PRIMARY KEY(tenant_id,plant_id,material_sku,treatment_code,revision),
 FOREIGN KEY(tenant_id,source_document_id) REFERENCES documents(tenant_id,id) ON DELETE RESTRICT
);
-- Append-only revisions: highest revision controls; enabled=false is an audited revocation.
CREATE INDEX fabric_shrinkage_latest_v31 ON fabric_shrinkage_specs_v31(tenant_id,plant_id,material_sku,treatment_code,revision DESC);
CREATE FUNCTION guard_fabric_shrinkage_spec_v31() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE src documents%ROWTYPE;
BEGIN
 SELECT * INTO src FROM documents WHERE tenant_id=NEW.tenant_id AND id=NEW.source_document_id;
 IF src.id IS NULL OR src.status<>'APPROVED' OR upper(src.issuer)<>'CUSTOMER'
    OR src.file_checksum<>NEW.source_sha256
    OR src.document_type NOT IN ('TECH_PACK','BUYER_SPEC','FABRIC_SHRINKAGE_SPEC','FABRIC_TEST_SPEC')
 THEN RAISE EXCEPTION 'ERR_F318B_APPROVED_CUSTOMER_SPEC_REQUIRED'; END IF;
 NEW.approved_at:=clock_timestamp();
 RETURN NEW;
END $$;
CREATE TRIGGER fabric_shrinkage_spec_guard_v31 BEFORE INSERT ON fabric_shrinkage_specs_v31
 FOR EACH ROW EXECUTE FUNCTION guard_fabric_shrinkage_spec_v31();

DO $$ DECLARE tbl text; BEGIN
 FOREACH tbl IN ARRAY ARRAY['jobwork_discrepancy_resolutions_v31','fabric_shrinkage_specs_v31'] LOOP
  EXECUTE format('ALTER TABLE public.%I ENABLE ROW LEVEL SECURITY',tbl);
  EXECUTE format('ALTER TABLE public.%I FORCE ROW LEVEL SECURITY',tbl);
  EXECUTE format('CREATE POLICY factory_tenant_v31 ON public.%I FOR ALL TO agiparel_app USING (tenant_id=NULLIF(current_setting(%L,true),%L)) WITH CHECK (tenant_id=NULLIF(current_setting(%L,true),%L))',tbl,'app.current_tenant_id','','app.current_tenant_id','');
  EXECUTE format('REVOKE ALL ON public.%I FROM PUBLIC,agiparel_app',tbl);
  EXECUTE format('GRANT SELECT ON public.%I TO agiparel_app',tbl);
  EXECUTE format('CREATE TRIGGER immutable_factory_v31 BEFORE UPDATE OR DELETE ON public.%I FOR EACH ROW EXECUTE FUNCTION protect_cut_draft_v8()',tbl);
 END LOOP;
END $$;
GRANT INSERT ON jobwork_discrepancy_resolutions_v31 TO agiparel_app;
-- No application INSERT/UPDATE/DELETE on buyer-spec catalog. Provisioner must
-- add a NEW approved revision rather than editing an existing spec.
COMMIT;
