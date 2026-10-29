-- Author: Sunil Jain, CyberinQ LLC, 2026-10-29.
-- F3.17A: per-part internal outward/return claims for provisioned subcontractors.
-- A signed gateway report + worker assertion is NOT a vendor signature, QC or production credit.
BEGIN;
INSERT INTO workforce_chapter_actions_v17(chapter_id,permission,authority) VALUES
 (6,'jobwork.outward','SIGNED_WRITE'), (6,'jobwork.inward','SIGNED_WRITE')
 ON CONFLICT DO NOTHING;

-- Provisioner-only vendor directory. Never infer vendor approval from a scan.
CREATE TABLE jobwork_vendors_v28 (
 tenant_id varchar(64) NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
 plant_id varchar(64) NOT NULL, vendor_id varchar(64) NOT NULL,
 display_name varchar(180) NOT NULL CHECK(length(btrim(display_name)) BETWEEN 3 AND 180),
 enabled boolean NOT NULL DEFAULT false,
 allowed_processes text[] NOT NULL DEFAULT '{}' CHECK (array_position(allowed_processes,NULL::text) IS NULL),
 approved_by varchar(128) NOT NULL, approved_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(tenant_id,plant_id,vendor_id)
);
CREATE TABLE jobwork_outwards_v28 (
 tenant_id varchar(64) NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
 id uuid NOT NULL DEFAULT uuid_generate_v4(),
 plant_id varchar(64) NOT NULL, station_id varchar(64) NOT NULL,
 part_id uuid NOT NULL, scan_id uuid NOT NULL,
 vendor_id varchar(64) NOT NULL, job_reference varchar(64) NOT NULL,
 process_code varchar(32) NOT NULL CHECK(process_code IN ('EMBROIDERY','PRINTING','WASHING','DYEING','OTHER_FINISHING')),
 note text NOT NULL CHECK(length(btrim(note)) BETWEEN 12 AND 500),
 command_id varchar(80) NOT NULL CHECK(command_id ~ '^[A-Za-z0-9_-]{12,80}$'),
 command_sha256 char(64) NOT NULL CHECK(command_sha256 ~ '^[0-9a-f]{64}$'),
 sender_subject varchar(128) NOT NULL CHECK(length(btrim(sender_subject))>0), sender_proof_id varchar(255) NOT NULL CHECK(length(btrim(sender_proof_id))>0),
 assertion_sha256 char(64) NOT NULL CHECK(assertion_sha256 ~ '^[0-9a-f]{64}$'),
 recorded_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 authority text NOT NULL DEFAULT 'SIGNED_INTERNAL_OUTWARD_NOT_VENDOR_RECEIPT'
  CHECK(authority='SIGNED_INTERNAL_OUTWARD_NOT_VENDOR_RECEIPT'),
 PRIMARY KEY(tenant_id,id), UNIQUE(tenant_id,command_id), UNIQUE(tenant_id,scan_id), UNIQUE(tenant_id,sender_proof_id),
 FOREIGN KEY(tenant_id,plant_id,vendor_id) REFERENCES jobwork_vendors_v28(tenant_id,plant_id,vendor_id) ON DELETE RESTRICT,
 FOREIGN KEY(tenant_id,part_id) REFERENCES gen_cut_parts_v18(tenant_id,id) ON DELETE RESTRICT,
 FOREIGN KEY(tenant_id,scan_id) REFERENCES wip_gateway_scans_v20(tenant_id,id) ON DELETE RESTRICT
);
CREATE INDEX jobwork_out_part_v28 ON jobwork_outwards_v28(tenant_id,plant_id,part_id,recorded_at DESC);
CREATE TABLE jobwork_inwards_v28 (
 tenant_id varchar(64) NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
 id uuid NOT NULL DEFAULT uuid_generate_v4(),
 outward_id uuid NOT NULL, plant_id varchar(64) NOT NULL,
 station_id varchar(64) NOT NULL, part_id uuid NOT NULL, scan_id uuid NOT NULL,
 note text NOT NULL CHECK(length(btrim(note)) BETWEEN 12 AND 500),
 command_id varchar(80) NOT NULL CHECK(command_id ~ '^[A-Za-z0-9_-]{12,80}$'),
 command_sha256 char(64) NOT NULL CHECK(command_sha256 ~ '^[0-9a-f]{64}$'),
 receiver_subject varchar(128) NOT NULL CHECK(length(btrim(receiver_subject))>0), receiver_proof_id varchar(255) NOT NULL CHECK(length(btrim(receiver_proof_id))>0),
 assertion_sha256 char(64) NOT NULL CHECK(assertion_sha256 ~ '^[0-9a-f]{64}$'),
 recorded_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 authority text NOT NULL DEFAULT 'SIGNED_INTERNAL_RETURN_PENDING_QC_NOT_VENDOR_CERTIFIED'
  CHECK(authority='SIGNED_INTERNAL_RETURN_PENDING_QC_NOT_VENDOR_CERTIFIED'),
 PRIMARY KEY(tenant_id,id), UNIQUE(tenant_id,outward_id), UNIQUE(tenant_id,command_id),
 UNIQUE(tenant_id,scan_id), UNIQUE(tenant_id,receiver_proof_id),
 FOREIGN KEY(tenant_id,outward_id) REFERENCES jobwork_outwards_v28(tenant_id,id) ON DELETE RESTRICT,
 FOREIGN KEY(tenant_id,part_id) REFERENCES gen_cut_parts_v18(tenant_id,id) ON DELETE RESTRICT,
 FOREIGN KEY(tenant_id,scan_id) REFERENCES wip_gateway_scans_v20(tenant_id,id) ON DELETE RESTRICT
);
CREATE INDEX jobwork_in_part_v28 ON jobwork_inwards_v28(tenant_id,plant_id,part_id,recorded_at DESC);

-- Allocated solely by AFTER INSERT security-definer triggers, not by API writers.
CREATE TABLE jobwork_used_scans_v28 (
 tenant_id varchar(64) NOT NULL REFERENCES tenants(id), scan_id uuid NOT NULL,
 kind text NOT NULL CHECK(kind IN('OUTWARD','INWARD')), entry_id uuid NOT NULL,
 PRIMARY KEY(tenant_id,scan_id),
 FOREIGN KEY(tenant_id,scan_id) REFERENCES wip_gateway_scans_v20(tenant_id,id)
);
CREATE TABLE jobwork_used_proofs_v28 (
 tenant_id varchar(64) NOT NULL REFERENCES tenants(id), proof_id varchar(255) NOT NULL,
 kind text NOT NULL CHECK(kind IN('OUTWARD','INWARD')), entry_id uuid NOT NULL,
 PRIMARY KEY(tenant_id,proof_id)
);

CREATE FUNCTION jobwork_part_offsite_v28(t text,p uuid) RETURNS boolean LANGUAGE sql STABLE AS $$
 SELECT EXISTS(SELECT 1 FROM jobwork_outwards_v28 o WHERE o.tenant_id=t AND o.part_id=p
  AND NOT EXISTS(SELECT 1 FROM jobwork_inwards_v28 i WHERE i.tenant_id=o.tenant_id AND i.outward_id=o.id))
$$;

-- Cross-ledger replay: each gateway scan is a single-use event, never a generic
-- permission to perform multiple custody/production actions.
CREATE FUNCTION jobwork_scan_spent_v28(t text,s uuid) RETURNS boolean LANGUAGE sql STABLE AS $$
 SELECT EXISTS(SELECT 1 FROM wip_movements_v20 x WHERE x.tenant_id=t AND x.scan_id=s)
 OR EXISTS(SELECT 1 FROM wip_exception_closures_v23 x WHERE x.tenant_id=t AND x.scan_id=s)
 OR EXISTS(SELECT 1 FROM wip_piece_reviews_v26 x WHERE x.tenant_id=t AND x.scan_id=s)
 OR EXISTS(SELECT 1 FROM sew_piece_used_scans_v27 x WHERE x.tenant_id=t AND x.scan_id=s)
 OR EXISTS(SELECT 1 FROM jobwork_used_scans_v28 x WHERE x.tenant_id=t AND x.scan_id=s)
$$;

CREATE FUNCTION jobwork_require_scan_v28(t text,pl text,st text,part uuid,scanid uuid) RETURNS void
 LANGUAGE plpgsql AS $$
DECLARE s wip_gateway_scans_v20%ROWTYPE;
BEGIN
 PERFORM pg_advisory_xact_lock(hashtext(t),hashtext('WIP:SCAN:'||scanid::text));
 SELECT * INTO s FROM wip_gateway_scans_v20 WHERE tenant_id=t AND id=scanid;
 IF NOT FOUND OR s.part_id<>part OR s.plant_id<>pl OR s.station_id<>st
    OR s.received_at<clock_timestamp()-interval '15 minutes'
    OR s.observed_at<clock_timestamp()-interval '15 minutes'
    OR s.observed_at>clock_timestamp()+interval '2 minutes'
    OR s.received_at>clock_timestamp() THEN RAISE EXCEPTION 'ERR_F317_FRESH_SCAN_REQUIRED'; END IF;
 IF NOT EXISTS(SELECT 1 FROM gen_carrier_active_v18 a JOIN gen_carriers_v18 c
    ON c.tenant_id=a.tenant_id AND c.id=a.carrier_id
    WHERE a.tenant_id=t AND a.carrier_id=s.carrier_id AND a.bound_event_id=s.binding_event_id
      AND a.target_type='CUT_PART' AND a.target_id=part AND a.plant_id=pl
      AND c.enabled AND c.epc_hex=s.epc_hex)
 OR NOT EXISTS(SELECT 1 FROM sew_prod_gateways_v10 gw WHERE gw.tenant_id=t
   AND gw.gateway_id=s.gateway_id AND gw.plant_id=pl AND gw.station_id=st AND gw.enabled)
 OR jobwork_scan_spent_v28(t,scanid) THEN RAISE EXCEPTION 'ERR_F317_SCAN_UNTRUSTED_OR_SPENT'; END IF;
END $$;

CREATE FUNCTION guard_jobwork_outward_v28() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE g gen_garments_v18%ROWTYPE; part gen_cut_parts_v18%ROWTYPE;
 prior wip_movements_v20%ROWTYPE; group_no integer;
BEGIN
 SELECT * INTO part FROM gen_cut_parts_v18 WHERE tenant_id=NEW.tenant_id AND id=NEW.part_id;
 SELECT * INTO g FROM gen_garments_v18 WHERE tenant_id=NEW.tenant_id AND id=part.garment_id;
 IF part.id IS NULL OR g.id IS NULL OR g.plant_id<>NEW.plant_id THEN RAISE EXCEPTION 'ERR_F317_PART_SCOPE'; END IF;
 SELECT min(b.ply_group) INTO group_no FROM cut_bundle_drafts_v8 b WHERE b.tenant_id=NEW.tenant_id
  AND b.lay_draft_id=g.lay_draft_id AND b.marker_position=g.marker_position
  AND g.ply_number BETWEEN b.ply_start AND b.ply_end;
 IF group_no IS NULL THEN RAISE EXCEPTION 'ERR_F317_GROUP_MISSING'; END IF;
 PERFORM pg_advisory_xact_lock(hashtext(NEW.tenant_id),hashtext('F316:GROUP:'||g.lay_draft_id::text||':'||g.marker_position||':'||group_no));
 PERFORM pg_advisory_xact_lock(hashtext(NEW.tenant_id),hashtext('F34:GARMENT:'||g.id::text));
 PERFORM pg_advisory_xact_lock(hashtext(NEW.tenant_id),hashtext('WIP:PART:'||NEW.part_id::text));
 IF NOT EXISTS(SELECT 1 FROM jobwork_vendors_v28 v WHERE v.tenant_id=NEW.tenant_id
   AND v.plant_id=NEW.plant_id AND v.vendor_id=NEW.vendor_id AND v.enabled
   AND NEW.process_code=ANY(v.allowed_processes))
 OR NOT EXISTS(SELECT 1 FROM sew_prod_worker_stations_v10 w WHERE w.tenant_id=NEW.tenant_id
   AND w.plant_id=NEW.plant_id AND w.station_id=NEW.station_id
   AND w.worker_subject=NEW.sender_subject AND w.enabled)
 THEN RAISE EXCEPTION 'ERR_F317_VENDOR_OR_SENDER_NOT_ENROLLED'; END IF;
 IF NOT EXISTS(SELECT 1 FROM cut_part_outcomes_v19 o WHERE o.tenant_id=NEW.tenant_id
  AND o.part_id=NEW.part_id AND o.disposition='ACCEPTED')
 OR EXISTS(SELECT 1 FROM cut_recut_links_v19 l WHERE l.tenant_id=NEW.tenant_id AND l.original_part_id=NEW.part_id)
 OR wip_piece_active_hold_v26(NEW.tenant_id,g.id)
 OR EXISTS(SELECT 1 FROM wip_exception_cases_v23 x WHERE x.tenant_id=NEW.tenant_id AND x.part_id=NEW.part_id
   AND NOT EXISTS(SELECT 1 FROM wip_exception_closures_v23 cl WHERE cl.tenant_id=x.tenant_id AND cl.case_id=x.id))
 OR EXISTS(SELECT 1 FROM qc_garment_events_v21 q WHERE q.tenant_id=NEW.tenant_id AND q.garment_id=g.id AND q.action='QUALITY_RELEASE')
 OR EXISTS(SELECT 1 FROM pack_carton_events_v22 pk WHERE pk.tenant_id=NEW.tenant_id AND pk.garment_id=g.id AND pk.action='ADD')
 THEN RAISE EXCEPTION 'ERR_F317_PART_HELD_REPLACED_OR_FINISHED'; END IF;
 IF jobwork_part_offsite_v28(NEW.tenant_id,NEW.part_id) THEN RAISE EXCEPTION 'ERR_F317_ALREADY_OFFSITE'; END IF;
 SELECT * INTO prior FROM wip_movements_v20 WHERE tenant_id=NEW.tenant_id AND part_id=NEW.part_id ORDER BY event_no DESC LIMIT 1;
 IF prior.id IS NULL OR prior.action<>'ARRIVE' OR prior.station_id<>NEW.station_id
 THEN RAISE EXCEPTION 'ERR_F317_STATION_CUSTODY_REQUIRED'; END IF;
 PERFORM jobwork_require_scan_v28(NEW.tenant_id,NEW.plant_id,NEW.station_id,NEW.part_id,NEW.scan_id);
 IF NOT EXISTS(SELECT 1 FROM wip_gateway_scans_v20 x WHERE x.tenant_id=NEW.tenant_id
  AND x.id=NEW.scan_id AND x.observed_at>=prior.recorded_at AND x.received_at>=prior.recorded_at)
 THEN RAISE EXCEPTION 'ERR_F317_SCAN_PRECEDES_STATION_CUSTODY'; END IF;
 IF NEW.sender_proof_id IS NULL OR length(btrim(NEW.sender_proof_id))=0 THEN RAISE EXCEPTION 'ERR_F317_PROOF_REQUIRED'; END IF;
 NEW.recorded_at:=clock_timestamp();
 RETURN NEW;
END $$;
CREATE TRIGGER jobwork_outward_guard_v28 BEFORE INSERT ON jobwork_outwards_v28
 FOR EACH ROW EXECUTE FUNCTION guard_jobwork_outward_v28();

CREATE FUNCTION guard_jobwork_inward_v28() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE o jobwork_outwards_v28%ROWTYPE; g uuid; group_no integer; lay uuid; pos integer;
BEGIN
 SELECT * INTO o FROM jobwork_outwards_v28 WHERE tenant_id=NEW.tenant_id AND id=NEW.outward_id;
 IF o.id IS NULL OR o.plant_id<>NEW.plant_id OR o.part_id<>NEW.part_id OR o.station_id<>NEW.station_id
   OR o.sender_subject=NEW.receiver_subject THEN RAISE EXCEPTION 'ERR_F317_OUTWARD_OR_INDEPENDENCE'; END IF;
 SELECT p.garment_id,garm.lay_draft_id,garm.marker_position INTO g,lay,pos
  FROM gen_cut_parts_v18 p JOIN gen_garments_v18 garm ON garm.tenant_id=p.tenant_id AND garm.id=p.garment_id
  WHERE p.tenant_id=NEW.tenant_id AND p.id=NEW.part_id;
 SELECT min(b.ply_group) INTO group_no FROM cut_bundle_drafts_v8 b JOIN gen_garments_v18 garm
  ON garm.tenant_id=b.tenant_id AND garm.lay_draft_id=b.lay_draft_id
  WHERE garm.tenant_id=NEW.tenant_id AND garm.id=g
  AND b.marker_position=pos AND garm.ply_number BETWEEN b.ply_start AND b.ply_end;
 PERFORM pg_advisory_xact_lock(hashtext(NEW.tenant_id),hashtext('F316:GROUP:'||lay::text||':'||pos||':'||group_no));
 PERFORM pg_advisory_xact_lock(hashtext(NEW.tenant_id),hashtext('F34:GARMENT:'||g::text));
 PERFORM pg_advisory_xact_lock(hashtext(NEW.tenant_id),hashtext('WIP:PART:'||NEW.part_id::text));
 IF NOT jobwork_part_offsite_v28(NEW.tenant_id,NEW.part_id) OR
  EXISTS(SELECT 1 FROM jobwork_inwards_v28 i WHERE i.tenant_id=NEW.tenant_id AND i.outward_id=NEW.outward_id)
 THEN RAISE EXCEPTION 'ERR_F317_OUTWARD_NOT_OPEN'; END IF;
 IF NOT EXISTS(SELECT 1 FROM sew_prod_worker_stations_v10 w WHERE w.tenant_id=NEW.tenant_id
  AND w.plant_id=NEW.plant_id AND w.station_id=NEW.station_id
  AND w.worker_subject=NEW.receiver_subject AND w.enabled)
 THEN RAISE EXCEPTION 'ERR_F317_RECEIVER_NOT_ENROLLED'; END IF;
 PERFORM jobwork_require_scan_v28(NEW.tenant_id,NEW.plant_id,NEW.station_id,NEW.part_id,NEW.scan_id);
 IF NOT EXISTS(SELECT 1 FROM wip_gateway_scans_v20 x WHERE x.tenant_id=NEW.tenant_id
  AND x.id=NEW.scan_id AND x.observed_at>=o.recorded_at AND x.received_at>=o.recorded_at)
 THEN RAISE EXCEPTION 'ERR_F317_RETURN_SCAN_PRECEDES_OUTWARD'; END IF;
 NEW.recorded_at:=clock_timestamp();
 RETURN NEW;
END $$;
CREATE TRIGGER jobwork_inward_guard_v28 BEFORE INSERT ON jobwork_inwards_v28
 FOR EACH ROW EXECUTE FUNCTION guard_jobwork_inward_v28();

CREATE FUNCTION jobwork_reserve_evidence_v28() RETURNS trigger
 LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog,public AS $$
DECLARE kind text; proof text;
BEGIN
 IF TG_TABLE_NAME='jobwork_outwards_v28' THEN kind:='OUTWARD';proof:=NEW.sender_proof_id;
 ELSE kind:='INWARD';proof:=NEW.receiver_proof_id; END IF;
 INSERT INTO public.jobwork_used_scans_v28(tenant_id,scan_id,kind,entry_id) VALUES(NEW.tenant_id,NEW.scan_id,kind,NEW.id);
 INSERT INTO public.jobwork_used_proofs_v28(tenant_id,proof_id,kind,entry_id) VALUES(NEW.tenant_id,proof,kind,NEW.id);
 RETURN NEW;
END $$;
REVOKE ALL ON FUNCTION jobwork_reserve_evidence_v28() FROM PUBLIC;
CREATE TRIGGER jobwork_out_evidence_v28 AFTER INSERT ON jobwork_outwards_v28
 FOR EACH ROW EXECUTE FUNCTION jobwork_reserve_evidence_v28();
CREATE TRIGGER jobwork_in_evidence_v28 AFTER INSERT ON jobwork_inwards_v28
 FOR EACH ROW EXECUTE FUNCTION jobwork_reserve_evidence_v28();

-- Block all other consumers of a job-work scan, even after return.
CREATE FUNCTION reject_jobwork_scan_reuse_v28() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE ids uuid[]; sid uuid;
BEGIN
 IF TG_TABLE_NAME='sew_piece_completions_v27' THEN ids:=NEW.scan_ids;
 ELSE ids:=ARRAY[NEW.scan_id]; END IF;
 FOREACH sid IN ARRAY ids LOOP
  PERFORM pg_advisory_xact_lock(hashtext(NEW.tenant_id),hashtext('WIP:SCAN:'||sid::text));
  IF EXISTS(SELECT 1 FROM jobwork_used_scans_v28 j WHERE j.tenant_id=NEW.tenant_id AND j.scan_id=sid)
  THEN RAISE EXCEPTION 'ERR_F317_SCAN_ALREADY_USED_FOR_JOBWORK'; END IF;
 END LOOP;
 RETURN NEW;
END $$;
CREATE TRIGGER zzz_jobwork_scan_move_v28 BEFORE INSERT ON wip_movements_v20
 FOR EACH ROW EXECUTE FUNCTION reject_jobwork_scan_reuse_v28();
CREATE TRIGGER zzz_jobwork_scan_exception_v28 BEFORE INSERT ON wip_exception_closures_v23
 FOR EACH ROW EXECUTE FUNCTION reject_jobwork_scan_reuse_v28();
CREATE TRIGGER zzz_jobwork_scan_piece_review_v28 BEFORE INSERT ON wip_piece_reviews_v26
 FOR EACH ROW EXECUTE FUNCTION reject_jobwork_scan_reuse_v28();
CREATE TRIGGER zzz_jobwork_scan_piece_complete_v28 BEFORE INSERT ON sew_piece_completions_v27
 FOR EACH ROW EXECUTE FUNCTION reject_jobwork_scan_reuse_v28();

CREATE FUNCTION guard_jobwork_offsite_v28() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE garment uuid; part uuid; lay uuid; pos int; group_no int;
BEGIN
 IF TG_TABLE_NAME='wip_movements_v20' THEN
  part:=NEW.part_id;
  PERFORM pg_advisory_xact_lock(hashtext(NEW.tenant_id),hashtext('WIP:PART:'||part::text));
  IF jobwork_part_offsite_v28(NEW.tenant_id,part) THEN RAISE EXCEPTION 'ERR_F317_OFFSITE_NO_WIP_MOVE'; END IF;
 ELSIF TG_TABLE_NAME='sew_prod_completions_v10' THEN
  SELECT r.lay_draft_id INTO lay FROM sew_prod_routes_v10 r WHERE r.tenant_id=NEW.tenant_id AND r.id=NEW.route_id;
  PERFORM pg_advisory_xact_lock(hashtext(NEW.tenant_id),hashtext('F316:GROUP:'||lay::text||':'||NEW.marker_position||':'||NEW.ply_group));
  IF EXISTS(SELECT 1 FROM jobwork_outwards_v28 o JOIN gen_cut_parts_v18 p ON p.tenant_id=o.tenant_id AND p.id=o.part_id
    JOIN gen_garments_v18 g ON g.tenant_id=p.tenant_id AND g.id=p.garment_id
    JOIN cut_bundle_drafts_v8 b ON b.tenant_id=g.tenant_id AND b.lay_draft_id=g.lay_draft_id
     AND b.marker_position=g.marker_position AND g.ply_number BETWEEN b.ply_start AND b.ply_end
    WHERE o.tenant_id=NEW.tenant_id AND g.lay_draft_id=lay AND g.marker_position=NEW.marker_position
     AND b.ply_group=NEW.ply_group AND jobwork_part_offsite_v28(o.tenant_id,o.part_id))
  THEN RAISE EXCEPTION 'ERR_F317_OFFSITE_NO_GROUP_SEWING'; END IF;
 ELSIF TG_TABLE_NAME='sew_piece_completions_v27' OR TG_TABLE_NAME='qc_garment_events_v21'
 OR TG_TABLE_NAME='pack_carton_events_v22' THEN
  IF TG_TABLE_NAME='pack_carton_events_v22' THEN
   IF NEW.action<>'ADD' THEN RETURN NEW; END IF;
  END IF;
  garment:=NEW.garment_id;
  PERFORM pg_advisory_xact_lock(hashtext(NEW.tenant_id),hashtext('F34:GARMENT:'||garment::text));
  IF EXISTS(SELECT 1 FROM gen_cut_parts_v18 p WHERE p.tenant_id=NEW.tenant_id AND p.garment_id=garment
   AND jobwork_part_offsite_v28(p.tenant_id,p.id))
  THEN RAISE EXCEPTION 'ERR_F317_OFFSITE_NO_SEWING_QC_PACKING'; END IF;
 END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER zzz_jobwork_offsite_move_v28 BEFORE INSERT ON wip_movements_v20
 FOR EACH ROW EXECUTE FUNCTION guard_jobwork_offsite_v28();
CREATE TRIGGER zzz_jobwork_offsite_whole_v28 BEFORE INSERT ON sew_prod_completions_v10
 FOR EACH ROW EXECUTE FUNCTION guard_jobwork_offsite_v28();
CREATE TRIGGER zzz_jobwork_offsite_piece_v28 BEFORE INSERT ON sew_piece_completions_v27
 FOR EACH ROW EXECUTE FUNCTION guard_jobwork_offsite_v28();
CREATE TRIGGER zzz_jobwork_offsite_qc_v28 BEFORE INSERT ON qc_garment_events_v21
 FOR EACH ROW EXECUTE FUNCTION guard_jobwork_offsite_v28();
CREATE TRIGGER zzz_jobwork_offsite_pack_v28 BEFORE INSERT ON pack_carton_events_v22
 FOR EACH ROW EXECUTE FUNCTION guard_jobwork_offsite_v28();

DO $$ DECLARE tbl text; BEGIN
 FOREACH tbl IN ARRAY ARRAY['jobwork_vendors_v28','jobwork_outwards_v28','jobwork_inwards_v28',
  'jobwork_used_scans_v28','jobwork_used_proofs_v28'] LOOP
  EXECUTE format('ALTER TABLE public.%I ENABLE ROW LEVEL SECURITY',tbl);
  EXECUTE format('ALTER TABLE public.%I FORCE ROW LEVEL SECURITY',tbl);
  EXECUTE format('CREATE POLICY jobwork_tenant_v28 ON public.%I FOR ALL TO agiparel_app USING (tenant_id=NULLIF(current_setting(%L,true),%L)) WITH CHECK (tenant_id=NULLIF(current_setting(%L,true),%L))',
   tbl,'app.current_tenant_id','','app.current_tenant_id','');
  EXECUTE format('REVOKE ALL ON public.%I FROM PUBLIC,agiparel_app',tbl);
  EXECUTE format('GRANT SELECT ON public.%I TO agiparel_app',tbl);
 END LOOP;
END $$;
GRANT INSERT ON jobwork_outwards_v28,jobwork_inwards_v28 TO agiparel_app;
CREATE TRIGGER jobwork_out_immutable_v28 BEFORE UPDATE OR DELETE ON jobwork_outwards_v28
 FOR EACH ROW EXECUTE FUNCTION protect_cut_draft_v8();
CREATE TRIGGER jobwork_in_immutable_v28 BEFORE UPDATE OR DELETE ON jobwork_inwards_v28
 FOR EACH ROW EXECUTE FUNCTION protect_cut_draft_v8();
CREATE TRIGGER jobwork_scan_immutable_v28 BEFORE UPDATE OR DELETE ON jobwork_used_scans_v28
 FOR EACH ROW EXECUTE FUNCTION protect_cut_draft_v8();
CREATE TRIGGER jobwork_proof_immutable_v28 BEFORE UPDATE OR DELETE ON jobwork_used_proofs_v28
 FOR EACH ROW EXECUTE FUNCTION protect_cut_draft_v8();
COMMIT;
