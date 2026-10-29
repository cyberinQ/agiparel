-- Author: Sunil Jain, CyberinQ LLC, 2026-10-29.
-- F3.16B: piece-scoped worker attestations, never automatic payroll/QC or shipping credit.
-- Requires migrations 0029, 0037-0040, 0042 and corrected 0045.
BEGIN;
INSERT INTO workforce_chapter_actions_v17(chapter_id,permission,authority)
 VALUES(6,'sewing.piece.complete','SIGNED_WRITE') ON CONFLICT DO NOTHING;

CREATE TABLE sew_piece_completions_v27 (
 tenant_id varchar(64) NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
 id uuid NOT NULL DEFAULT uuid_generate_v4(),
 command_id varchar(80) NOT NULL CHECK(command_id ~ '^[A-Za-z0-9_-]{12,80}$'),
 command_sha256 char(64) NOT NULL CHECK(command_sha256 ~ '^[0-9a-f]{64}$'),
 garment_id uuid NOT NULL,
 lay_draft_id uuid NOT NULL, release_id uuid NOT NULL, route_id uuid NOT NULL,
 plant_id varchar(64) NOT NULL, station_id varchar(64) NOT NULL,
 marker_position integer NOT NULL CHECK(marker_position>0),
 ply_group integer NOT NULL CHECK(ply_group>0),
 scan_ids uuid[] NOT NULL CHECK(cardinality(scan_ids) BETWEEN 1 AND 32),
 worker_subject varchar(128) NOT NULL,
 worker_proof_id varchar(255) NOT NULL,
 assertion_sha256 char(64) NOT NULL CHECK(assertion_sha256 ~ '^[0-9a-f]{64}$'),
 recorded_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 evidence_state text NOT NULL DEFAULT 'WORKER_ATTESTED_UNIT_OPERATION_PENDING_QUALITY_NOT_PAYROLL'
  CHECK(evidence_state='WORKER_ATTESTED_UNIT_OPERATION_PENDING_QUALITY_NOT_PAYROLL'),
 PRIMARY KEY(tenant_id,id),
 UNIQUE(tenant_id,command_id), UNIQUE(tenant_id,worker_proof_id),
 UNIQUE(tenant_id,route_id,garment_id),
 FOREIGN KEY(tenant_id,garment_id) REFERENCES gen_garments_v18(tenant_id,id) ON DELETE RESTRICT,
 FOREIGN KEY(tenant_id,release_id) REFERENCES cut_sewing_releases_v8(tenant_id,id) ON DELETE RESTRICT,
 FOREIGN KEY(tenant_id,route_id) REFERENCES sew_prod_routes_v10(tenant_id,id) ON DELETE RESTRICT
);
CREATE INDEX sew_piece_completion_history_v27
 ON sew_piece_completions_v27(tenant_id,plant_id,garment_id,recorded_at DESC);

-- Normalized, exclusive scan consumption. Only a trigger can create these links.
CREATE TABLE sew_piece_used_scans_v27 (
 tenant_id varchar(64) NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
 scan_id uuid NOT NULL,
 completion_id uuid NOT NULL,
 PRIMARY KEY(tenant_id,scan_id),
 FOREIGN KEY(tenant_id,scan_id) REFERENCES wip_gateway_scans_v20(tenant_id,id) ON DELETE RESTRICT,
 FOREIGN KEY(tenant_id,completion_id) REFERENCES sew_piece_completions_v27(tenant_id,id) ON DELETE RESTRICT
);
CREATE INDEX sew_piece_scans_completion_v27 ON sew_piece_used_scans_v27(tenant_id,completion_id);

-- Lock order matches F3.16A: ALL garment-part locks, group lock, garment lock,
-- then single-use scan locks. Serializes hold/review/move/completion races.
CREATE FUNCTION guard_sew_piece_completion_v27() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE g gen_garments_v18%ROWTYPE; route sew_prod_routes_v10%ROWTYPE;
        scan wip_gateway_scans_v20%ROWTYPE;
        sid uuid; pid uuid; group_no integer; n_group integer;
        code varchar(64); slot_no integer; part_owner uuid;
        accepted boolean; replacement boolean; used_codes text[] := ARRAY[]::text[];
        prev_code text; arrival_action text; arrival_station text; arrival_at timestamptz;
BEGIN
 SELECT * INTO g FROM gen_garments_v18 WHERE tenant_id=NEW.tenant_id AND id=NEW.garment_id;
 IF NOT FOUND OR g.plant_id<>NEW.plant_id OR g.lay_draft_id<>NEW.lay_draft_id
  OR g.release_id<>NEW.release_id OR g.marker_position<>NEW.marker_position
 THEN RAISE EXCEPTION 'ERR_F316B_GARMENT_SCOPE'; END IF;
 SELECT * INTO route FROM sew_prod_routes_v10 WHERE tenant_id=NEW.tenant_id AND id=NEW.route_id;
 IF NOT FOUND OR NOT route.enabled OR route.lay_draft_id<>NEW.lay_draft_id
  OR route.plant_id<>NEW.plant_id OR route.station_id<>NEW.station_id
 THEN RAISE EXCEPTION 'ERR_F316B_ROUTE_SCOPE'; END IF;
 IF cardinality(route.required_parts)<>cardinality(NEW.scan_ids) OR
   (SELECT count(DISTINCT x) FROM unnest(route.required_parts) x)<>cardinality(route.required_parts) OR
   (SELECT count(DISTINCT x) FROM unnest(NEW.scan_ids) x)<>cardinality(NEW.scan_ids)
 THEN RAISE EXCEPTION 'ERR_F316B_REQUIRED_SCAN_SET'; END IF;

 FOR pid IN SELECT part_id FROM (
  SELECT p.id part_id FROM gen_cut_parts_v18 p WHERE p.tenant_id=NEW.tenant_id AND p.garment_id=NEW.garment_id
  UNION
  SELECT link.replacement_part_id FROM cut_recut_links_v19 link
   JOIN gen_cut_parts_v18 p ON p.tenant_id=link.tenant_id AND p.id=link.original_part_id
   WHERE p.tenant_id=NEW.tenant_id AND p.garment_id=NEW.garment_id
 ) parts ORDER BY part_id LOOP
  PERFORM pg_advisory_xact_lock(hashtext(NEW.tenant_id),hashtext('WIP:PART:'||pid::text));
 END LOOP;
 -- Share the RFID bind/retire serialization lock with F3.1/F3.3.
 PERFORM pg_advisory_xact_lock(hashtext(NEW.tenant_id),hashtext('RFID_EPC_GLOBAL_V8'));
 SELECT count(DISTINCT b.ply_group),min(b.ply_group) INTO n_group,group_no
 FROM cut_bundle_drafts_v8 b WHERE b.tenant_id=NEW.tenant_id
  AND b.lay_draft_id=NEW.lay_draft_id AND b.marker_position=NEW.marker_position
  AND g.ply_number BETWEEN b.ply_start AND b.ply_end;
 IF n_group<>1 OR group_no<>NEW.ply_group THEN RAISE EXCEPTION 'ERR_F316B_PLY_GROUP'; END IF;
 PERFORM pg_advisory_xact_lock(hashtext(NEW.tenant_id),hashtext('F316:GROUP:'||NEW.lay_draft_id::text||':'||NEW.marker_position||':'||NEW.ply_group));
 PERFORM pg_advisory_xact_lock(hashtext(NEW.tenant_id),hashtext('F34:GARMENT:'||NEW.garment_id::text));
 IF wip_piece_active_hold_v26(NEW.tenant_id,NEW.garment_id) THEN
  RAISE EXCEPTION 'ERR_F316B_PIECE_HELD'; END IF;
 IF NOT EXISTS(SELECT 1 FROM sew_prod_worker_stations_v10 w WHERE w.tenant_id=NEW.tenant_id
  AND w.plant_id=NEW.plant_id AND w.station_id=NEW.station_id
  AND w.worker_subject=NEW.worker_subject AND w.enabled) THEN
  RAISE EXCEPTION 'ERR_F316B_WORKER_NOT_ASSIGNED'; END IF;
 -- Operation codes, not only route IDs: a published revision cannot grant
 -- the same garment a second completion for the same logical operation.
 IF EXISTS(SELECT 1 FROM sew_piece_completions_v27 prior
  JOIN sew_prod_routes_v10 pr ON pr.tenant_id=prior.tenant_id AND pr.id=prior.route_id
  WHERE prior.tenant_id=NEW.tenant_id AND prior.garment_id=NEW.garment_id
   AND pr.lay_draft_id=NEW.lay_draft_id AND pr.plant_id=NEW.plant_id
   AND pr.operation_code=route.operation_code)
 THEN RAISE EXCEPTION 'ERR_F316B_OPERATION_ALREADY_COMPLETED'; END IF;
 IF EXISTS(SELECT 1 FROM sew_prod_completions_v10 whole
  JOIN sew_prod_routes_v10 prior ON prior.tenant_id=whole.tenant_id AND prior.id=whole.route_id
  WHERE whole.tenant_id=NEW.tenant_id AND whole.marker_position=NEW.marker_position
   AND whole.ply_group=NEW.ply_group AND prior.plant_id=NEW.plant_id
   AND prior.lay_draft_id=NEW.lay_draft_id AND prior.operation_code=route.operation_code)
 THEN RAISE EXCEPTION 'ERR_F316B_WHOLE_GROUP_ALREADY_COMPLETED'; END IF;
 IF EXISTS(SELECT 1 FROM qc_garment_events_v21 q WHERE q.tenant_id=NEW.tenant_id
  AND q.garment_id=NEW.garment_id AND q.action='QUALITY_RELEASE')
 THEN RAISE EXCEPTION 'ERR_F316B_ALREADY_QC_RELEASED'; END IF;

 FOR prev_code IN SELECT DISTINCT unnest(route.predecessors) LOOP
  IF NOT EXISTS(SELECT 1 FROM sew_piece_completions_v27 c
   JOIN sew_prod_routes_v10 prior ON prior.tenant_id=c.tenant_id AND prior.id=c.route_id
   WHERE c.tenant_id=NEW.tenant_id AND c.garment_id=NEW.garment_id
    AND prior.plant_id=NEW.plant_id AND prior.lay_draft_id=NEW.lay_draft_id
    AND prior.operation_code=prev_code AND prior.enabled)
  AND NOT EXISTS(SELECT 1 FROM sew_prod_completions_v10 c
   JOIN sew_prod_routes_v10 prior ON prior.tenant_id=c.tenant_id AND prior.id=c.route_id
   WHERE c.tenant_id=NEW.tenant_id AND c.marker_position=NEW.marker_position AND c.ply_group=NEW.ply_group
    AND prior.plant_id=NEW.plant_id AND prior.lay_draft_id=NEW.lay_draft_id
    AND prior.operation_code=prev_code AND prior.enabled)
  THEN RAISE EXCEPTION 'ERR_F316B_PREDECESSOR'; END IF;
 END LOOP;

 FOR sid IN SELECT unnest(NEW.scan_ids) ORDER BY 1 LOOP
  PERFORM pg_advisory_xact_lock(hashtext(NEW.tenant_id),hashtext('WIP:SCAN:'||sid::text));
  SELECT * INTO scan FROM wip_gateway_scans_v20 WHERE tenant_id=NEW.tenant_id AND id=sid;
  IF NOT FOUND OR scan.plant_id<>NEW.plant_id OR scan.station_id<>NEW.station_id
   OR scan.observed_at<clock_timestamp()-interval '15 minutes'
   OR scan.received_at<clock_timestamp()-interval '15 minutes'
   OR scan.observed_at>clock_timestamp()+interval '2 minutes'
   OR scan.received_at>clock_timestamp()
   OR scan.observed_at<route.created_at-interval '2 minutes'
  THEN RAISE EXCEPTION 'ERR_F316B_FRESH_SCOPED_SCAN_REQUIRED'; END IF;
  IF NOT EXISTS(SELECT 1 FROM sew_prod_gateways_v10 gw
   WHERE gw.tenant_id=NEW.tenant_id AND gw.gateway_id=scan.gateway_id
    AND gw.plant_id=NEW.plant_id AND gw.station_id=NEW.station_id AND gw.enabled)
  THEN RAISE EXCEPTION 'ERR_F316B_GATEWAY_DISABLED'; END IF;
  IF NOT EXISTS(SELECT 1 FROM gen_carrier_active_v18 a
    JOIN gen_carriers_v18 carrier ON carrier.tenant_id=a.tenant_id AND carrier.id=a.carrier_id
   WHERE a.tenant_id=NEW.tenant_id AND a.target_type='CUT_PART' AND a.target_id=scan.part_id
    AND a.plant_id=NEW.plant_id AND a.carrier_id=scan.carrier_id
    AND a.bound_event_id=scan.binding_event_id AND carrier.enabled AND carrier.epc_hex=scan.epc_hex)
  THEN RAISE EXCEPTION 'ERR_F316B_STALE_CARRIER'; END IF;

  -- The accepted part is either original and not replaced, or the single
  -- linked replacement of an original part belonging to this garment.
  SELECT p.part_code,p.part_slot,p.garment_id,
    EXISTS(SELECT 1 FROM cut_part_outcomes_v19 out WHERE out.tenant_id=p.tenant_id
     AND out.part_id=p.id AND out.disposition='ACCEPTED'),
    EXISTS(SELECT 1 FROM cut_recut_links_v19 link WHERE link.tenant_id=p.tenant_id AND link.replacement_part_id=p.id)
   INTO code,slot_no,part_owner,accepted,replacement
   FROM gen_cut_parts_v18 p WHERE p.tenant_id=NEW.tenant_id AND p.id=scan.part_id;
  IF NOT FOUND OR NOT accepted OR NOT (code=ANY(route.required_parts)) THEN
   RAISE EXCEPTION 'ERR_F316B_PART_NOT_ACCEPTED'; END IF;
  IF replacement THEN
   IF NOT EXISTS(SELECT 1 FROM cut_recut_links_v19 link JOIN gen_cut_parts_v18 original
      ON original.tenant_id=link.tenant_id AND original.id=link.original_part_id
     WHERE link.tenant_id=NEW.tenant_id AND link.replacement_part_id=scan.part_id
      AND original.garment_id=NEW.garment_id AND original.part_code=code AND original.part_slot=slot_no)
   THEN RAISE EXCEPTION 'ERR_F316B_RECUT_LINEAGE'; END IF;
  ELSIF part_owner<>NEW.garment_id OR EXISTS(SELECT 1 FROM cut_recut_links_v19 link
     WHERE link.tenant_id=NEW.tenant_id AND link.original_part_id=scan.part_id)
  THEN RAISE EXCEPTION 'ERR_F316B_ORIGINAL_REPLACED'; END IF;
  IF code=ANY(used_codes) THEN RAISE EXCEPTION 'ERR_F316B_DUPLICATE_PART_CODE'; END IF;
  used_codes:=array_append(used_codes,code);
  SELECT move.action,move.station_id,move.recorded_at INTO arrival_action,arrival_station,arrival_at
   FROM wip_movements_v20 move WHERE move.tenant_id=NEW.tenant_id AND move.part_id=scan.part_id
   ORDER BY move.event_no DESC LIMIT 1;
  IF NOT FOUND OR arrival_action<>'ARRIVE' OR arrival_station<>NEW.station_id
    OR scan.observed_at<arrival_at OR scan.received_at<arrival_at
  THEN RAISE EXCEPTION 'ERR_F316B_STATION_CUSTODY_REQUIRED'; END IF;
  IF EXISTS(SELECT 1 FROM wip_movements_v20 move WHERE move.tenant_id=NEW.tenant_id AND move.scan_id=sid)
   OR EXISTS(SELECT 1 FROM wip_exception_closures_v23 x WHERE x.tenant_id=NEW.tenant_id AND x.scan_id=sid)
   OR EXISTS(SELECT 1 FROM wip_piece_reviews_v26 review WHERE review.tenant_id=NEW.tenant_id AND review.scan_id=sid)
   OR EXISTS(SELECT 1 FROM sew_piece_used_scans_v27 used WHERE used.tenant_id=NEW.tenant_id AND used.scan_id=sid)
  THEN RAISE EXCEPTION 'ERR_F316B_SCAN_ALREADY_USED'; END IF;
  IF EXISTS(SELECT 1 FROM wip_exception_cases_v23 x WHERE x.tenant_id=NEW.tenant_id
    AND x.part_id=scan.part_id AND NOT EXISTS(SELECT 1 FROM wip_exception_closures_v23 cl
       WHERE cl.tenant_id=x.tenant_id AND cl.case_id=x.id))
  THEN RAISE EXCEPTION 'ERR_F316B_WIP_EXCEPTION_OPEN'; END IF;
 END LOOP;
 IF cardinality(used_codes)<>cardinality(route.required_parts)
 THEN RAISE EXCEPTION 'ERR_F316B_MISSING_REQUIRED_PART'; END IF;
 NEW.recorded_at:=clock_timestamp();
 RETURN NEW;
END $$;
CREATE TRIGGER sew_piece_completion_guard_v27 BEFORE INSERT ON sew_piece_completions_v27
 FOR EACH ROW EXECUTE FUNCTION guard_sew_piece_completion_v27();

CREATE FUNCTION reserve_sew_piece_scans_v27() RETURNS trigger
 LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog,public AS $$
BEGIN
 INSERT INTO public.sew_piece_used_scans_v27(tenant_id,scan_id,completion_id)
 SELECT NEW.tenant_id,x,NEW.id FROM unnest(NEW.scan_ids) x;
 RETURN NEW;
END $$;
REVOKE ALL ON FUNCTION reserve_sew_piece_scans_v27() FROM PUBLIC;
CREATE TRIGGER sew_piece_scans_once_v27 AFTER INSERT ON sew_piece_completions_v27
 FOR EACH ROW EXECUTE FUNCTION reserve_sew_piece_scans_v27();

-- No mixed legacy whole-group and piece-specific credit on the SAME operation.
-- F3.16A's whole-group lock fires before this trigger (alphabetical order).
CREATE FUNCTION reject_whole_sew_when_piece_v27() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF EXISTS(SELECT 1 FROM sew_piece_completions_v27 c
  JOIN gen_garments_v18 g ON g.tenant_id=c.tenant_id AND g.id=c.garment_id
  JOIN sew_prod_routes_v10 pr ON pr.tenant_id=c.tenant_id AND pr.id=c.route_id
  JOIN sew_prod_routes_v10 newroute ON newroute.tenant_id=NEW.tenant_id AND newroute.id=NEW.route_id
  WHERE c.tenant_id=NEW.tenant_id AND g.marker_position=NEW.marker_position
   AND c.ply_group=NEW.ply_group AND pr.lay_draft_id=newroute.lay_draft_id
   AND pr.operation_code=newroute.operation_code AND pr.plant_id=newroute.plant_id)
 THEN RAISE EXCEPTION 'ERR_F316B_MIXED_WHOLE_AND_PIECE_CREDIT'; END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER zzz_sew_piece_no_group_v27 BEFORE INSERT ON sew_prod_completions_v10
 FOR EACH ROW EXECUTE FUNCTION reject_whole_sew_when_piece_v27();

-- A scan used for piece completion cannot later be consumed as movement,
-- exception recovery or supervised piece-hold review.
CREATE FUNCTION reject_reused_sew_piece_scan_v27() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 PERFORM pg_advisory_xact_lock(hashtext(NEW.tenant_id),hashtext('WIP:SCAN:'||NEW.scan_id::text));
 IF EXISTS(SELECT 1 FROM sew_piece_used_scans_v27 used
  WHERE used.tenant_id=NEW.tenant_id AND used.scan_id=NEW.scan_id)
 THEN RAISE EXCEPTION 'ERR_F316B_SCAN_SPENT_BY_SEWING'; END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER zzz_sew_piece_scan_move_v27 BEFORE INSERT ON wip_movements_v20
 FOR EACH ROW EXECUTE FUNCTION reject_reused_sew_piece_scan_v27();
CREATE TRIGGER zzz_sew_piece_scan_exception_v27 BEFORE INSERT ON wip_exception_closures_v23
 FOR EACH ROW EXECUTE FUNCTION reject_reused_sew_piece_scan_v27();
CREATE TRIGGER zzz_sew_piece_scan_review_v27 BEFORE INSERT ON wip_piece_reviews_v26
 FOR EACH ROW EXECUTE FUNCTION reject_reused_sew_piece_scan_v27();

DO $$ DECLARE tbl text; BEGIN
 FOREACH tbl IN ARRAY ARRAY['sew_piece_completions_v27','sew_piece_used_scans_v27'] LOOP
  EXECUTE format('ALTER TABLE public.%I ENABLE ROW LEVEL SECURITY',tbl);
  EXECUTE format('ALTER TABLE public.%I FORCE ROW LEVEL SECURITY',tbl);
  EXECUTE format('CREATE POLICY sew_piece_tenant_v27 ON public.%I FOR ALL TO agiparel_app USING (tenant_id=NULLIF(current_setting(%L,true),%L)) WITH CHECK (tenant_id=NULLIF(current_setting(%L,true),%L))',
   tbl,'app.current_tenant_id','','app.current_tenant_id','');
  EXECUTE format('REVOKE ALL ON public.%I FROM PUBLIC,agiparel_app',tbl);
  EXECUTE format('GRANT SELECT ON public.%I TO agiparel_app',tbl);
  EXECUTE format('CREATE TRIGGER sew_piece_immutable_v27 BEFORE UPDATE OR DELETE ON public.%I FOR EACH ROW EXECUTE FUNCTION protect_cut_draft_v8()',tbl);
 END LOOP;
END $$;
GRANT INSERT ON sew_piece_completions_v27 TO agiparel_app;
COMMIT;
