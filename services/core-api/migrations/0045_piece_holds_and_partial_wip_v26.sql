-- Author: Sunil Jain, CyberinQ LLC, 2026-10-29.
-- F3.16A: hold one permanent garment/ply identity, not its entire bundle.
-- No phantom sewing completion, QC release, shipment, or payroll credit.
BEGIN;
INSERT INTO workforce_chapter_actions_v17(chapter_id,permission,authority) VALUES
 (6,'sewing.piece.hold','SIGNED_WRITE'),
 (6,'sewing.piece.review','SIGNED_WRITE') ON CONFLICT DO NOTHING;

CREATE TABLE wip_piece_holds_v26 (
 tenant_id varchar(64) NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
 id uuid NOT NULL DEFAULT uuid_generate_v4(),
 plant_id varchar(64) NOT NULL, garment_id uuid NOT NULL, station_id varchar(64) NOT NULL,
 reason text NOT NULL CHECK(reason IN('SEWING_DEFECT','MISSING_COMPONENT','OIL_STAIN','NEEDLE_DAMAGE','REWORK_REQUIRED')),
 note text NOT NULL CHECK(length(btrim(note)) BETWEEN 12 AND 500),
 command_id varchar(80) NOT NULL CHECK(command_id ~ '^[A-Za-z0-9_-]{12,80}$'),
 command_sha256 char(64) NOT NULL CHECK(command_sha256 ~ '^[0-9a-f]{64}$'),
 reporter_subject varchar(128) NOT NULL, reporter_proof_id varchar(255) NOT NULL,
 assertion_sha256 char(64) NOT NULL CHECK(assertion_sha256 ~ '^[0-9a-f]{64}$'),
 opened_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 authority text NOT NULL DEFAULT 'SIGNED_ONE_GARMENT_HOLD_NOT_PHYSICAL_DEFECT_CERTIFICATION'
  CHECK(authority='SIGNED_ONE_GARMENT_HOLD_NOT_PHYSICAL_DEFECT_CERTIFICATION'),
 PRIMARY KEY(tenant_id,id), UNIQUE(tenant_id,command_id), UNIQUE(tenant_id,reporter_proof_id),
 FOREIGN KEY(tenant_id,garment_id) REFERENCES gen_garments_v18(tenant_id,id) ON DELETE RESTRICT
);
CREATE INDEX wip_piece_holds_by_unit_v26 ON wip_piece_holds_v26(tenant_id,garment_id,opened_at DESC);

CREATE TABLE wip_piece_reviews_v26 (
 tenant_id varchar(64) NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
 id uuid NOT NULL DEFAULT uuid_generate_v4(),
 hold_id uuid NOT NULL, plant_id varchar(64) NOT NULL, garment_id uuid NOT NULL,
 station_id varchar(64) NOT NULL, scan_id uuid NOT NULL,
 review_action text NOT NULL CHECK(review_action='SUPERVISOR_REVIEWED_FOR_WIP'),
 note text NOT NULL CHECK(length(btrim(note)) BETWEEN 12 AND 500),
 command_id varchar(80) NOT NULL CHECK(command_id ~ '^[A-Za-z0-9_-]{12,80}$'),
 command_sha256 char(64) NOT NULL CHECK(command_sha256 ~ '^[0-9a-f]{64}$'),
 supervisor_subject varchar(128) NOT NULL, supervisor_proof_id varchar(255) NOT NULL,
 assertion_sha256 char(64) NOT NULL CHECK(assertion_sha256 ~ '^[0-9a-f]{64}$'),
 reviewed_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 authority text NOT NULL DEFAULT 'INDEPENDENT_SIGNED_REVIEW_AND_FRESH_GATEWAY_SCAN_NO_COMPLETION_CREDIT'
  CHECK(authority='INDEPENDENT_SIGNED_REVIEW_AND_FRESH_GATEWAY_SCAN_NO_COMPLETION_CREDIT'),
 PRIMARY KEY(tenant_id,id), UNIQUE(tenant_id,hold_id), UNIQUE(tenant_id,command_id),
 UNIQUE(tenant_id,supervisor_proof_id),
 FOREIGN KEY(tenant_id,hold_id) REFERENCES wip_piece_holds_v26(tenant_id,id) ON DELETE RESTRICT,
 FOREIGN KEY(tenant_id,scan_id) REFERENCES wip_gateway_scans_v20(tenant_id,id) ON DELETE RESTRICT,
 FOREIGN KEY(tenant_id,garment_id) REFERENCES gen_garments_v18(tenant_id,id) ON DELETE RESTRICT
);

-- Does not claim a defect was repaired; it simply records open administrative holds.
CREATE FUNCTION wip_piece_active_hold_v26(t varchar, gid uuid) RETURNS boolean
 LANGUAGE sql VOLATILE AS $$
 SELECT EXISTS(SELECT 1 FROM wip_piece_holds_v26 h
  WHERE h.tenant_id=t AND h.garment_id=gid
  AND NOT EXISTS(SELECT 1 FROM wip_piece_reviews_v26 r
   WHERE r.tenant_id=h.tenant_id AND r.hold_id=h.id))
$$;

CREATE FUNCTION guard_wip_piece_hold_v26() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE g gen_garments_v18%ROWTYPE; pid uuid; ply_group_no integer;
BEGIN
 SELECT * INTO g FROM gen_garments_v18 WHERE tenant_id=NEW.tenant_id AND id=NEW.garment_id;
 IF NOT FOUND OR g.plant_id<>NEW.plant_id THEN RAISE EXCEPTION 'ERR_F316_GARMENT_SCOPE'; END IF;
 -- Use exactly the WIP part locks used by F3.3/F3.10, including replacement parts.
 FOR pid IN SELECT part_id FROM (
  SELECT p.id part_id FROM gen_cut_parts_v18 p WHERE p.tenant_id=NEW.tenant_id AND p.garment_id=NEW.garment_id
  UNION SELECT link.replacement_part_id FROM cut_recut_links_v19 link
   JOIN gen_cut_parts_v18 p ON p.tenant_id=link.tenant_id AND p.id=link.original_part_id
   WHERE p.tenant_id=NEW.tenant_id AND p.garment_id=NEW.garment_id
 ) parts ORDER BY part_id LOOP
  PERFORM pg_advisory_xact_lock(hashtext(NEW.tenant_id),hashtext('WIP:PART:'||pid::text));
 END LOOP;
 FOR ply_group_no IN SELECT DISTINCT b.ply_group FROM cut_bundle_drafts_v8 b
  WHERE b.tenant_id=NEW.tenant_id AND b.lay_draft_id=g.lay_draft_id
  AND b.marker_position=g.marker_position AND g.ply_number BETWEEN b.ply_start AND b.ply_end
  ORDER BY b.ply_group LOOP
  PERFORM pg_advisory_xact_lock(hashtext(NEW.tenant_id),hashtext('F316:GROUP:'||g.lay_draft_id::text||':'||g.marker_position||':'||ply_group_no));
 END LOOP;
 PERFORM pg_advisory_xact_lock(hashtext(NEW.tenant_id),hashtext('F34:GARMENT:'||NEW.garment_id::text));
 IF NOT EXISTS(SELECT 1 FROM sew_prod_worker_stations_v10 w WHERE w.tenant_id=NEW.tenant_id
  AND w.plant_id=NEW.plant_id AND w.station_id=NEW.station_id
  AND w.worker_subject=NEW.reporter_subject AND w.enabled)
 THEN RAISE EXCEPTION 'ERR_F316_REPORTER_STATION'; END IF;
 IF EXISTS(SELECT 1 FROM qc_garment_events_v21 q WHERE q.tenant_id=NEW.tenant_id
  AND q.garment_id=NEW.garment_id AND q.action='QUALITY_RELEASE')
 THEN RAISE EXCEPTION 'ERR_F316_ALREADY_QC_RELEASED'; END IF;
 IF EXISTS(SELECT 1 FROM wip_piece_reviews_v26 x WHERE x.tenant_id=NEW.tenant_id AND x.command_id=NEW.command_id)
 THEN RAISE EXCEPTION 'ERR_F316_COMMAND_REUSE'; END IF;
 IF wip_piece_active_hold_v26(NEW.tenant_id,NEW.garment_id)
 THEN RAISE EXCEPTION 'ERR_F316_ALREADY_HELD'; END IF;
 IF EXISTS(SELECT 1 FROM wip_piece_reviews_v26 r WHERE r.tenant_id=NEW.tenant_id
   AND r.supervisor_proof_id=NEW.reporter_proof_id)
 THEN RAISE EXCEPTION 'ERR_F316_CROSS_PROOF_REPLAY'; END IF;
 NEW.opened_at:=clock_timestamp();
 RETURN NEW;
END $$;
CREATE TRIGGER wip_piece_open_guard_v26 BEFORE INSERT ON wip_piece_holds_v26
 FOR EACH ROW EXECUTE FUNCTION guard_wip_piece_hold_v26();

CREATE FUNCTION guard_wip_piece_review_v26() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE h wip_piece_holds_v26%ROWTYPE; s wip_gateway_scans_v20%ROWTYPE; pid uuid; g gen_garments_v18%ROWTYPE; ply_group_no integer;
BEGIN
 SELECT * INTO h FROM wip_piece_holds_v26 WHERE tenant_id=NEW.tenant_id AND id=NEW.hold_id;
 IF NOT FOUND OR h.plant_id<>NEW.plant_id OR h.garment_id<>NEW.garment_id
  OR h.station_id<>NEW.station_id THEN RAISE EXCEPTION 'ERR_F316_HOLD_SCOPE'; END IF;
 FOR pid IN SELECT part_id FROM (
  SELECT p.id part_id FROM gen_cut_parts_v18 p WHERE p.tenant_id=NEW.tenant_id AND p.garment_id=h.garment_id
  UNION SELECT link.replacement_part_id FROM cut_recut_links_v19 link
   JOIN gen_cut_parts_v18 p ON p.tenant_id=link.tenant_id AND p.id=link.original_part_id
   WHERE p.tenant_id=NEW.tenant_id AND p.garment_id=h.garment_id
 ) parts ORDER BY part_id LOOP
  PERFORM pg_advisory_xact_lock(hashtext(NEW.tenant_id),hashtext('WIP:PART:'||pid::text));
 END LOOP;
 SELECT * INTO g FROM gen_garments_v18 WHERE tenant_id=NEW.tenant_id AND id=h.garment_id;
 FOR ply_group_no IN SELECT DISTINCT b.ply_group FROM cut_bundle_drafts_v8 b
  WHERE b.tenant_id=NEW.tenant_id AND b.lay_draft_id=g.lay_draft_id
   AND b.marker_position=g.marker_position AND g.ply_number BETWEEN b.ply_start AND b.ply_end
  ORDER BY b.ply_group LOOP
  PERFORM pg_advisory_xact_lock(hashtext(NEW.tenant_id),hashtext('F316:GROUP:'||g.lay_draft_id::text||':'||g.marker_position||':'||ply_group_no));
 END LOOP;
 IF EXISTS(SELECT 1 FROM wip_piece_holds_v26 x WHERE x.tenant_id=NEW.tenant_id AND x.command_id=NEW.command_id)
 THEN RAISE EXCEPTION 'ERR_F316_COMMAND_REUSE'; END IF;
 IF NEW.supervisor_subject=h.reporter_subject THEN RAISE EXCEPTION 'ERR_F316_INDEPENDENT_REVIEW'; END IF;
 IF EXISTS(SELECT 1 FROM wip_piece_holds_v26 o WHERE o.tenant_id=NEW.tenant_id
   AND o.reporter_proof_id=NEW.supervisor_proof_id)
 THEN RAISE EXCEPTION 'ERR_F316_CROSS_PROOF_REPLAY'; END IF;
 IF NOT EXISTS(SELECT 1 FROM sew_prod_worker_stations_v10 w WHERE w.tenant_id=NEW.tenant_id
  AND w.plant_id=NEW.plant_id AND w.station_id=NEW.station_id
  AND w.worker_subject=NEW.supervisor_subject AND w.enabled)
 THEN RAISE EXCEPTION 'ERR_F316_SUPERVISOR_STATION'; END IF;
 PERFORM pg_advisory_xact_lock(hashtext(NEW.tenant_id),hashtext('WIP:SCAN:'||NEW.scan_id::text));
 SELECT * INTO s FROM wip_gateway_scans_v20 WHERE tenant_id=NEW.tenant_id AND id=NEW.scan_id;
 IF NOT FOUND OR s.plant_id<>NEW.plant_id OR s.station_id<>NEW.station_id
  OR s.received_at<=h.opened_at OR s.observed_at<=h.opened_at
  OR s.received_at>clock_timestamp() OR s.observed_at<clock_timestamp()-interval '15 minutes'
  OR s.received_at<clock_timestamp()-interval '15 minutes'
 THEN RAISE EXCEPTION 'ERR_F316_FRESH_SIGNED_SCAN_REQUIRED'; END IF;
 IF NOT EXISTS(SELECT 1 FROM gen_cut_parts_v18 p WHERE p.tenant_id=NEW.tenant_id
  AND p.garment_id=h.garment_id AND p.id=s.part_id)
 AND NOT EXISTS(SELECT 1 FROM cut_recut_links_v19 link JOIN gen_cut_parts_v18 orig
  ON orig.tenant_id=link.tenant_id AND orig.id=link.original_part_id
  WHERE orig.tenant_id=NEW.tenant_id AND orig.garment_id=h.garment_id
  AND link.replacement_part_id=s.part_id)
 THEN RAISE EXCEPTION 'ERR_F316_SCAN_NOT_IN_GARMENT'; END IF;
 IF NOT EXISTS(SELECT 1 FROM gen_carrier_active_v18 a JOIN gen_carriers_v18 c
  ON c.tenant_id=a.tenant_id AND c.id=a.carrier_id
  WHERE a.tenant_id=NEW.tenant_id AND a.plant_id=NEW.plant_id AND a.target_type='CUT_PART'
  AND a.target_id=s.part_id AND a.carrier_id=s.carrier_id
  AND a.bound_event_id=s.binding_event_id AND c.enabled AND c.epc_hex=s.epc_hex)
 THEN RAISE EXCEPTION 'ERR_F316_CARRIER_NOT_CURRENT'; END IF;
 IF EXISTS(SELECT 1 FROM wip_movements_v20 m WHERE m.tenant_id=NEW.tenant_id AND m.scan_id=NEW.scan_id)
 OR EXISTS(SELECT 1 FROM wip_exception_closures_v23 x WHERE x.tenant_id=NEW.tenant_id AND x.scan_id=NEW.scan_id)
 OR EXISTS(SELECT 1 FROM wip_piece_reviews_v26 old WHERE old.tenant_id=NEW.tenant_id AND old.scan_id=NEW.scan_id)
 THEN RAISE EXCEPTION 'ERR_F316_SCAN_ALREADY_USED'; END IF;
 IF EXISTS(SELECT 1 FROM wip_exception_cases_v23 c WHERE c.tenant_id=NEW.tenant_id
  AND c.part_id=s.part_id AND NOT EXISTS(SELECT 1 FROM wip_exception_closures_v23 x
  WHERE x.tenant_id=c.tenant_id AND x.case_id=c.id))
 THEN RAISE EXCEPTION 'ERR_F316_UNRESOLVED_WIP_EXCEPTION'; END IF;
 NEW.reviewed_at:=clock_timestamp();
 RETURN NEW;
END $$;
CREATE TRIGGER wip_piece_review_guard_v26 BEFORE INSERT ON wip_piece_reviews_v26
 FOR EACH ROW EXECUTE FUNCTION guard_wip_piece_review_v26();
CREATE UNIQUE INDEX wip_piece_scan_once_v26 ON wip_piece_reviews_v26(tenant_id,scan_id);

-- Serialize scan consumption with the existing WIP movement/exception writers.
-- A scan cannot concurrently be consumed as both a hold review and movement.
CREATE FUNCTION guard_piece_review_scan_reuse_v26() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 PERFORM pg_advisory_xact_lock(hashtext(NEW.tenant_id),hashtext('WIP:SCAN:'||NEW.scan_id::text));
 IF EXISTS(SELECT 1 FROM wip_piece_reviews_v26 r WHERE r.tenant_id=NEW.tenant_id AND r.scan_id=NEW.scan_id)
 THEN RAISE EXCEPTION 'ERR_F316_REVIEW_SCAN_ALREADY_USED'; END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER zzz_piece_scan_used_in_wip_v26 BEFORE INSERT ON wip_movements_v20
 FOR EACH ROW EXECUTE FUNCTION guard_piece_review_scan_reuse_v26();
CREATE TRIGGER zzz_piece_scan_used_in_exception_v26 BEFORE INSERT ON wip_exception_closures_v23
 FOR EACH ROW EXECUTE FUNCTION guard_piece_review_scan_reuse_v26();

-- A one-piece hold blocks only that garment's original and recut WIP movements.
CREATE FUNCTION guard_wip_piece_move_hold_v26() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 PERFORM pg_advisory_xact_lock(hashtext(NEW.tenant_id),hashtext('WIP:PART:'||NEW.part_id::text));
 IF EXISTS(SELECT 1 FROM gen_cut_parts_v18 p WHERE p.tenant_id=NEW.tenant_id
  AND p.id=NEW.part_id AND wip_piece_active_hold_v26(NEW.tenant_id,p.garment_id))
 OR EXISTS(SELECT 1 FROM cut_recut_links_v19 l JOIN gen_cut_parts_v18 orig
  ON orig.tenant_id=l.tenant_id AND orig.id=l.original_part_id
  WHERE l.tenant_id=NEW.tenant_id AND l.replacement_part_id=NEW.part_id
  AND wip_piece_active_hold_v26(NEW.tenant_id,orig.garment_id))
 THEN RAISE EXCEPTION 'ERR_F316_PIECE_HOLD_MOVEMENT'; END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER zz_wip_piece_hold_v26 BEFORE INSERT ON wip_movements_v20
 FOR EACH ROW EXECUTE FUNCTION guard_wip_piece_move_hold_v26();

CREATE FUNCTION guard_qc_piece_hold_v26() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF NEW.action IN ('INSPECT_PASS','REINSPECT_PASS','FINISHING_COMPLETE','QUALITY_RELEASE') AND wip_piece_active_hold_v26(NEW.tenant_id,NEW.garment_id)
 THEN RAISE EXCEPTION 'ERR_F316_PIECE_HOLD_QC'; END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER zz_qc_piece_hold_v26 BEFORE INSERT ON qc_garment_events_v21
 FOR EACH ROW EXECUTE FUNCTION guard_qc_piece_hold_v26();

-- Legacy sewing production completion credits whole ply groups. Do NOT permit
-- whole-group credit during a partial-piece hold, until unit completion exists.
CREATE FUNCTION guard_sew_completion_piece_hold_v26() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 PERFORM pg_advisory_xact_lock(hashtext(NEW.tenant_id),hashtext('F316:GROUP:'||NEW.lay_draft_id::text||':'||NEW.marker_position||':'||NEW.ply_group));
 IF EXISTS(SELECT 1 FROM gen_garments_v18 g JOIN wip_piece_holds_v26 h
   ON h.tenant_id=g.tenant_id AND h.garment_id=g.id
  WHERE g.tenant_id=NEW.tenant_id AND g.lay_draft_id=NEW.lay_draft_id
   AND g.marker_position=NEW.marker_position AND h.plant_id=NEW.plant_id
   AND NOT EXISTS(SELECT 1 FROM wip_piece_reviews_v26 r WHERE r.tenant_id=h.tenant_id AND r.hold_id=h.id)
   AND EXISTS(SELECT 1 FROM cut_bundle_drafts_v8 b WHERE b.tenant_id=g.tenant_id
    AND b.lay_draft_id=g.lay_draft_id AND b.marker_position=g.marker_position
    AND b.ply_group=NEW.ply_group AND g.ply_number BETWEEN b.ply_start AND b.ply_end))
 THEN RAISE EXCEPTION 'ERR_F316_NO_FULL_BUNDLE_COMPLETION_DURING_PIECE_HOLD'; END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER zz_sew_completion_piece_hold_v26 BEFORE INSERT ON sew_prod_completions_v10
 FOR EACH ROW EXECUTE FUNCTION guard_sew_completion_piece_hold_v26();

DO $$ DECLARE tbl text; BEGIN
 FOREACH tbl IN ARRAY ARRAY['wip_piece_holds_v26','wip_piece_reviews_v26'] LOOP
  EXECUTE format('ALTER TABLE public.%I ENABLE ROW LEVEL SECURITY',tbl);
  EXECUTE format('ALTER TABLE public.%I FORCE ROW LEVEL SECURITY',tbl);
  EXECUTE format('CREATE POLICY wip_piece_tenant_v26 ON public.%I FOR ALL TO agiparel_app USING (tenant_id=NULLIF(current_setting(%L,true),%L)) WITH CHECK (tenant_id=NULLIF(current_setting(%L,true),%L))', tbl, 'app.current_tenant_id', '', 'app.current_tenant_id', '');
  EXECUTE format('REVOKE ALL ON public.%I FROM PUBLIC,agiparel_app',tbl);
  EXECUTE format('GRANT SELECT,INSERT ON public.%I TO agiparel_app',tbl);
  EXECUTE format('CREATE TRIGGER immutable_piece_v26 BEFORE UPDATE OR DELETE ON public.%I FOR EACH ROW EXECUTE FUNCTION protect_cut_draft_v8()',tbl);
 END LOOP;
END $$;
COMMIT;
