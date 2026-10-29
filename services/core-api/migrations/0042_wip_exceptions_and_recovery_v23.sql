-- Author: Sunil Jain, CyberinQ LLC, 2026-10-29.
-- F3.10: immutable reported exceptions, independently signed scan-backed closure,
-- and serialized WIP hold. No inferred custody movement or manufacturing credit.
BEGIN;
INSERT INTO workforce_chapter_actions_v17(chapter_id,permission,authority)
 VALUES (6,'sewing.exception.report','SIGNED_WRITE'),(6,'sewing.exception.resolve','SIGNED_WRITE')
 ON CONFLICT DO NOTHING;
CREATE TABLE wip_exception_cases_v23 (
 tenant_id varchar(64) NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
 id uuid NOT NULL DEFAULT uuid_generate_v4(),
 plant_id varchar(64) NOT NULL,
 part_id uuid NOT NULL,
 station_id varchar(64) NOT NULL,
 issue_type text NOT NULL CHECK (issue_type IN ('LOST_CARRIER','UNREADABLE_CARRIER','MISSING_WIP','FAILED_SCAN','MISROUTED_PART','REJECTED_HANDOFF')),
 note text NOT NULL CHECK(length(btrim(note)) BETWEEN 12 AND 500),
 command_id varchar(80) NOT NULL CHECK(command_id ~ '^[A-Za-z0-9_-]{12,80}$'),
 command_sha256 char(64) NOT NULL CHECK(command_sha256 ~ '^[0-9a-f]{64}$'),
 reporter_subject varchar(128) NOT NULL,
 reporter_proof_id varchar(255) NOT NULL,
 assertion_sha256 char(64) NOT NULL CHECK(assertion_sha256 ~ '^[0-9a-f]{64}$'),
 opened_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 authority text NOT NULL DEFAULT 'SIGNED_HUMAN_EXCEPTION_REPORT_NOT_PHYSICAL_PROOF'
  CHECK(authority='SIGNED_HUMAN_EXCEPTION_REPORT_NOT_PHYSICAL_PROOF'),
 PRIMARY KEY(tenant_id,id), UNIQUE(tenant_id,command_id), UNIQUE(tenant_id,reporter_proof_id),
 FOREIGN KEY(tenant_id,part_id) REFERENCES gen_cut_parts_v18(tenant_id,id) ON DELETE RESTRICT
);
CREATE INDEX wip_exception_part_v23 ON wip_exception_cases_v23(tenant_id,plant_id,part_id,opened_at DESC);
CREATE TABLE wip_exception_closures_v23 (
 tenant_id varchar(64) NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
 id uuid NOT NULL DEFAULT uuid_generate_v4(),
 case_id uuid NOT NULL,
 plant_id varchar(64) NOT NULL,
 part_id uuid NOT NULL,
 station_id varchar(64) NOT NULL,
 scan_id uuid NOT NULL,
 note text NOT NULL CHECK(length(btrim(note)) BETWEEN 12 AND 500),
 command_id varchar(80) NOT NULL CHECK(command_id ~ '^[A-Za-z0-9_-]{12,80}$'),
 command_sha256 char(64) NOT NULL CHECK(command_sha256 ~ '^[0-9a-f]{64}$'),
 supervisor_subject varchar(128) NOT NULL,
 supervisor_proof_id varchar(255) NOT NULL,
 assertion_sha256 char(64) NOT NULL CHECK(assertion_sha256 ~ '^[0-9a-f]{64}$'),
 closed_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 authority text NOT NULL DEFAULT 'SIGNED_SUPERVISOR_REVIEW_WITH_NEW_GATEWAY_SCAN_NO_CUSTODY_CHANGE'
  CHECK(authority='SIGNED_SUPERVISOR_REVIEW_WITH_NEW_GATEWAY_SCAN_NO_CUSTODY_CHANGE'),
 PRIMARY KEY(tenant_id,id), UNIQUE(tenant_id,case_id), UNIQUE(tenant_id,command_id), UNIQUE(tenant_id,supervisor_proof_id),
 FOREIGN KEY(tenant_id,case_id) REFERENCES wip_exception_cases_v23(tenant_id,id) ON DELETE RESTRICT,
 FOREIGN KEY(tenant_id,scan_id) REFERENCES wip_gateway_scans_v20(tenant_id,id) ON DELETE RESTRICT
);
-- Same per-part lock as F3.3 means creating/closing a hold and moving custody
-- are strictly serialized. The SQL trigger, not only the Go route, enforces it.
CREATE FUNCTION guard_wip_exception_open_v23() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE g gen_garments_v18%ROWTYPE; original_garment uuid;
BEGIN
 PERFORM pg_advisory_xact_lock(hashtext(NEW.tenant_id),hashtext('WIP:PART:'||NEW.part_id::text));
 SELECT g.* INTO g FROM gen_cut_parts_v18 p JOIN gen_garments_v18 g
  ON g.tenant_id=p.tenant_id AND g.id=p.garment_id
  WHERE p.tenant_id=NEW.tenant_id AND p.id=NEW.part_id;
 IF NOT FOUND OR g.plant_id<>NEW.plant_id THEN RAISE EXCEPTION 'ERR_F310_PART_SCOPE'; END IF;
 -- Serialize with quality-release transitions; a late incident cannot place an
 -- already released garment under an inconsistent new open hold.
 PERFORM pg_advisory_xact_lock(hashtext(NEW.tenant_id),hashtext('F34:GARMENT:'||g.id::text));
 IF NOT EXISTS(SELECT 1 FROM sew_prod_worker_stations_v10 w WHERE w.tenant_id=NEW.tenant_id
  AND w.plant_id=NEW.plant_id AND w.station_id=NEW.station_id AND w.worker_subject=NEW.reporter_subject AND w.enabled)
 THEN RAISE EXCEPTION 'ERR_F310_REPORTER_STATION'; END IF;
 IF EXISTS(SELECT 1 FROM qc_garment_events_v21 q WHERE q.tenant_id=NEW.tenant_id
  AND q.garment_id=g.id AND q.action='QUALITY_RELEASE') THEN RAISE EXCEPTION 'ERR_F310_ALREADY_RELEASED'; END IF;
 -- Replacement cut parts also participate in their original garment's QC.
 SELECT original.garment_id INTO original_garment FROM cut_recut_links_v19 link
  JOIN gen_cut_parts_v18 original ON original.tenant_id=link.tenant_id AND original.id=link.original_part_id
  WHERE link.tenant_id=NEW.tenant_id AND link.replacement_part_id=NEW.part_id;
 IF original_garment IS NOT NULL THEN
  PERFORM pg_advisory_xact_lock(hashtext(NEW.tenant_id),hashtext('F34:GARMENT:'||original_garment::text));
  IF EXISTS (SELECT 1 FROM qc_garment_events_v21 q WHERE q.tenant_id=NEW.tenant_id
   AND q.garment_id=original_garment AND q.action='QUALITY_RELEASE')
  THEN RAISE EXCEPTION 'ERR_F310_RECUT_ORIGINAL_ALREADY_RELEASED'; END IF;
 END IF;
 IF EXISTS(SELECT 1 FROM wip_exception_closures_v23 x WHERE x.tenant_id=NEW.tenant_id AND x.supervisor_proof_id=NEW.reporter_proof_id)
 THEN RAISE EXCEPTION 'ERR_F310_CROSS_EVENT_PROOF_REPLAY'; END IF;
 IF EXISTS(SELECT 1 FROM wip_exception_cases_v23 c WHERE c.tenant_id=NEW.tenant_id
  AND c.part_id=NEW.part_id AND NOT EXISTS(SELECT 1 FROM wip_exception_closures_v23 x
  WHERE x.tenant_id=c.tenant_id AND x.case_id=c.id)) THEN RAISE EXCEPTION 'ERR_F310_ALREADY_OPEN'; END IF;
 NEW.opened_at:=clock_timestamp();
 RETURN NEW;
END $$;
CREATE TRIGGER wip_exception_open_guard_v23 BEFORE INSERT ON wip_exception_cases_v23
 FOR EACH ROW EXECUTE FUNCTION guard_wip_exception_open_v23();
CREATE FUNCTION guard_wip_exception_close_v23() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE c wip_exception_cases_v23%ROWTYPE; s wip_gateway_scans_v20%ROWTYPE;
BEGIN
 SELECT * INTO c FROM wip_exception_cases_v23 WHERE tenant_id=NEW.tenant_id AND id=NEW.case_id;
 IF NOT FOUND OR c.plant_id<>NEW.plant_id OR c.part_id<>NEW.part_id OR c.station_id<>NEW.station_id THEN RAISE EXCEPTION 'ERR_F310_CASE_SCOPE'; END IF;
 PERFORM pg_advisory_xact_lock(hashtext(NEW.tenant_id),hashtext('WIP:PART:'||c.part_id::text));
 IF NEW.supervisor_subject=c.reporter_subject THEN RAISE EXCEPTION 'ERR_F310_INDEPENDENT_SUPERVISOR'; END IF;
 IF EXISTS (SELECT 1 FROM wip_exception_cases_v23 prev WHERE prev.tenant_id=NEW.tenant_id AND prev.reporter_proof_id=NEW.supervisor_proof_id)
 THEN RAISE EXCEPTION 'ERR_F310_CROSS_EVENT_PROOF_REPLAY'; END IF;
 IF NOT EXISTS(SELECT 1 FROM sew_prod_worker_stations_v10 w WHERE w.tenant_id=NEW.tenant_id
  AND w.plant_id=NEW.plant_id AND w.station_id=c.station_id AND w.worker_subject=NEW.supervisor_subject AND w.enabled)
 THEN RAISE EXCEPTION 'ERR_F310_SUPERVISOR_STATION'; END IF;
 SELECT * INTO s FROM wip_gateway_scans_v20 WHERE tenant_id=NEW.tenant_id AND id=NEW.scan_id;
 IF NOT FOUND OR s.plant_id<>NEW.plant_id OR s.part_id<>c.part_id OR s.station_id<>c.station_id
  OR s.received_at<=c.opened_at OR s.observed_at<=c.opened_at OR s.received_at>clock_timestamp()
  OR s.received_at<clock_timestamp()-interval '15 minutes' OR s.observed_at<clock_timestamp()-interval '15 minutes'
 THEN RAISE EXCEPTION 'ERR_F310_NEW_SIGNED_SCAN_REQUIRED'; END IF;
 IF NOT EXISTS(SELECT 1 FROM gen_carrier_active_v18 a JOIN gen_carriers_v18 carrier
   ON carrier.tenant_id=a.tenant_id AND carrier.id=a.carrier_id
  WHERE a.tenant_id=NEW.tenant_id AND a.plant_id=NEW.plant_id AND a.target_type='CUT_PART'
   AND a.target_id=c.part_id AND a.carrier_id=s.carrier_id AND a.bound_event_id=s.binding_event_id
   AND carrier.enabled AND carrier.epc_hex=s.epc_hex)
 THEN RAISE EXCEPTION 'ERR_F310_CARRIER_NOT_CURRENT'; END IF;
 IF c.issue_type='LOST_CARRIER' AND NOT EXISTS (SELECT 1 FROM gen_carrier_active_v18 a
  WHERE a.tenant_id=NEW.tenant_id AND a.target_type='CUT_PART' AND a.target_id=c.part_id
   AND a.bound_at>c.opened_at AND a.bound_event_id=s.binding_event_id)
 THEN RAISE EXCEPTION 'ERR_F310_REPLACEMENT_BINDING_REQUIRED'; END IF;
 IF EXISTS(SELECT 1 FROM wip_movements_v20 m WHERE m.tenant_id=NEW.tenant_id AND m.scan_id=NEW.scan_id)
 THEN RAISE EXCEPTION 'ERR_F310_RECOVERY_SCAN_ALREADY_CONSUMED'; END IF;
 NEW.closed_at:=clock_timestamp();
 RETURN NEW;
END $$;
CREATE TRIGGER wip_exception_close_guard_v23 BEFORE INSERT ON wip_exception_closures_v23
 FOR EACH ROW EXECUTE FUNCTION guard_wip_exception_close_v23();
CREATE FUNCTION guard_wip_exception_hold_v23() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 PERFORM pg_advisory_xact_lock(hashtext(NEW.tenant_id),hashtext('WIP:PART:'||NEW.part_id::text));
 IF EXISTS (SELECT 1 FROM wip_exception_cases_v23 c WHERE c.tenant_id=NEW.tenant_id
  AND c.plant_id=NEW.plant_id AND c.part_id=NEW.part_id
  AND NOT EXISTS(SELECT 1 FROM wip_exception_closures_v23 x
  WHERE x.tenant_id=c.tenant_id AND x.case_id=c.id))
 THEN RAISE EXCEPTION 'ERR_F310_UNRESOLVED_EXCEPTION_WIP_HOLD'; END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER zz_wip_exception_hold_v23 BEFORE INSERT ON wip_movements_v20
 FOR EACH ROW EXECUTE FUNCTION guard_wip_exception_hold_v23();
-- Newly opened cases for a garment freeze quality release as well, not just movement.
CREATE FUNCTION guard_qc_exception_hold_v23() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF NEW.action='QUALITY_RELEASE' AND EXISTS(
  SELECT 1 FROM gen_cut_parts_v18 p JOIN wip_exception_cases_v23 c
   ON c.tenant_id=p.tenant_id AND c.part_id=p.id
  WHERE p.tenant_id=NEW.tenant_id AND p.garment_id=NEW.garment_id
   AND NOT EXISTS(SELECT 1 FROM wip_exception_closures_v23 x
     WHERE x.tenant_id=c.tenant_id AND x.case_id=c.id))
 THEN RAISE EXCEPTION 'ERR_F310_UNRESOLVED_EXCEPTION_QC_HOLD'; END IF;
 IF NEW.action='QUALITY_RELEASE' AND EXISTS (
  SELECT 1 FROM gen_cut_parts_v18 original
   JOIN cut_recut_links_v19 link ON link.tenant_id=original.tenant_id AND link.original_part_id=original.id
   JOIN wip_exception_cases_v23 c ON c.tenant_id=link.tenant_id AND c.part_id=link.replacement_part_id
   WHERE original.tenant_id=NEW.tenant_id AND original.garment_id=NEW.garment_id
    AND NOT EXISTS(SELECT 1 FROM wip_exception_closures_v23 x WHERE x.tenant_id=c.tenant_id AND x.case_id=c.id))
 THEN RAISE EXCEPTION 'ERR_F310_UNRESOLVED_RECUT_EXCEPTION_QC_HOLD'; END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER zz_qc_exception_hold_v23 BEFORE INSERT ON qc_garment_events_v21
 FOR EACH ROW EXECUTE FUNCTION guard_qc_exception_hold_v23();
DO $$ DECLARE tbl text; BEGIN
 FOREACH tbl IN ARRAY ARRAY['wip_exception_cases_v23','wip_exception_closures_v23'] LOOP
  EXECUTE format('ALTER TABLE public.%I ENABLE ROW LEVEL SECURITY',tbl);
  EXECUTE format('ALTER TABLE public.%I FORCE ROW LEVEL SECURITY',tbl);
  EXECUTE format('CREATE POLICY wip_exception_tenant_v23 ON public.%I FOR ALL TO agiparel_app USING (tenant_id=NULLIF(current_setting(''app.current_tenant_id'',true),'''')) WITH CHECK (tenant_id=NULLIF(current_setting(''app.current_tenant_id'',true),''''))',tbl);
  EXECUTE format('REVOKE ALL ON public.%I FROM PUBLIC,agiparel_app',tbl);
  EXECUTE format('GRANT SELECT,INSERT ON public.%I TO agiparel_app',tbl);
  EXECUTE format('CREATE TRIGGER wip_exception_immutable_v23 BEFORE UPDATE OR DELETE ON public.%I FOR EACH ROW EXECUTE FUNCTION protect_cut_draft_v8()',tbl);
 END LOOP;
END $$;
COMMIT;
