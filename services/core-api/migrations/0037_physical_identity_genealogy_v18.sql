-- Author: Sunil Jain, 2026-10-28
-- F3.1: Permanent physical-item identities, provisional cut-part lineage and
-- independent reusable RFID-carrier lifecycle. No fabricated accepted output.
-- Apply after 0036. Requires signed cutting release (0021).
BEGIN;
CREATE TABLE gen_bundles_v18 (
 tenant_id varchar(64) NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
 id uuid NOT NULL DEFAULT uuid_generate_v4(),
 plant_id varchar(64) NOT NULL,
 release_id uuid NOT NULL,
 lay_draft_id uuid NOT NULL,
 label_code varchar(29) NOT NULL,
 state text NOT NULL DEFAULT 'RELEASED_CUT_BUNDLE_REFERENCE_NOT_UNIT_ACCEPTANCE'
  CHECK(state='RELEASED_CUT_BUNDLE_REFERENCE_NOT_UNIT_ACCEPTANCE'),
 created_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY (tenant_id,id), UNIQUE(tenant_id,lay_draft_id,label_code),
 FOREIGN KEY(tenant_id,release_id) REFERENCES cut_sewing_releases_v8(tenant_id,id) ON DELETE RESTRICT,
 FOREIGN KEY(tenant_id,lay_draft_id,label_code) REFERENCES cut_bundle_drafts_v8(tenant_id,lay_draft_id,label_code) ON DELETE RESTRICT
);
CREATE TABLE gen_garments_v18 (
 tenant_id varchar(64) NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
 id uuid NOT NULL DEFAULT uuid_generate_v4(),
 plant_id varchar(64) NOT NULL,
 release_id uuid NOT NULL,
 lay_draft_id uuid NOT NULL,
 marker_position integer NOT NULL CHECK(marker_position>0),
 ply_number integer NOT NULL CHECK(ply_number>0),
 command_id varchar(80) NOT NULL CHECK(command_id ~ '^[A-Za-z0-9_-]{12,80}$'),
 command_sha256 char(64) NOT NULL CHECK(command_sha256 ~ '^[0-9a-f]{64}$'),
 reserved_by varchar(128) NOT NULL,
 proof_id varchar(255) NOT NULL,
 assertion_sha256 char(64) NOT NULL CHECK(assertion_sha256 ~ '^[0-9a-f]{64}$'),
 state text NOT NULL DEFAULT 'IDENTITY_RESERVED_NOT_PHYSICALLY_VERIFIED'
  CHECK(state='IDENTITY_RESERVED_NOT_PHYSICALLY_VERIFIED'),
 created_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(tenant_id,id),
 UNIQUE(tenant_id,lay_draft_id,marker_position,ply_number),
 UNIQUE(tenant_id,command_id), UNIQUE(tenant_id,proof_id),
 FOREIGN KEY(tenant_id,release_id) REFERENCES cut_sewing_releases_v8(tenant_id,id) ON DELETE RESTRICT,
 FOREIGN KEY(tenant_id,lay_draft_id,marker_position) REFERENCES cut_lay_positions_v8(tenant_id,lay_draft_id,marker_position) ON DELETE RESTRICT
);
CREATE INDEX gen_garments_lay_v18 ON gen_garments_v18(tenant_id,plant_id,lay_draft_id,id);
CREATE TABLE gen_cut_parts_v18 (
 tenant_id varchar(64) NOT NULL,
 id uuid NOT NULL DEFAULT uuid_generate_v4(),
 garment_id uuid NOT NULL,
 bundle_id uuid NOT NULL,
 lay_draft_id uuid NOT NULL,
 label_code varchar(29) NOT NULL,
 part_code varchar(64) NOT NULL,
 part_slot integer NOT NULL CHECK(part_slot>0),
 state text NOT NULL DEFAULT 'CUT_PART_IDENTITY_REFERENCE_NOT_ACCEPTED_PIECE'
  CHECK(state='CUT_PART_IDENTITY_REFERENCE_NOT_ACCEPTED_PIECE'),
 created_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(tenant_id,id),
 UNIQUE(tenant_id,garment_id,bundle_id,part_slot),
 FOREIGN KEY(tenant_id,garment_id) REFERENCES gen_garments_v18(tenant_id,id) ON DELETE RESTRICT,
 FOREIGN KEY(tenant_id,bundle_id) REFERENCES gen_bundles_v18(tenant_id,id) ON DELETE RESTRICT,
 FOREIGN KEY(tenant_id,lay_draft_id,label_code) REFERENCES cut_bundle_drafts_v8(tenant_id,lay_draft_id,label_code) ON DELETE RESTRICT
);
CREATE INDEX gen_parts_by_garment_v18 ON gen_cut_parts_v18(tenant_id,garment_id,id);

-- Triggers validate actual signed-release ancestry, not user-submitted field labels.
CREATE FUNCTION guard_gen_bundle_v18() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE r cut_sewing_releases_v8%ROWTYPE;
BEGIN
 SELECT * INTO r FROM cut_sewing_releases_v8 WHERE tenant_id=NEW.tenant_id AND id=NEW.release_id;
 IF NOT FOUND OR r.plant_id<>NEW.plant_id OR r.lay_draft_id<>NEW.lay_draft_id OR
  NOT EXISTS (SELECT 1 FROM cut_bundle_drafts_v8 b WHERE b.tenant_id=NEW.tenant_id AND b.lay_draft_id=NEW.lay_draft_id AND b.label_code=NEW.label_code) THEN
  RAISE EXCEPTION 'ERR_GEN_BUNDLE_RELEASE_SCOPE'; END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER gen_bundle_guard_v18 BEFORE INSERT ON gen_bundles_v18 FOR EACH ROW EXECUTE FUNCTION guard_gen_bundle_v18();
CREATE FUNCTION guard_gen_garment_v18() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE r cut_sewing_releases_v8%ROWTYPE; overlapping integer;
BEGIN
 SELECT * INTO r FROM cut_sewing_releases_v8 WHERE tenant_id=NEW.tenant_id AND id=NEW.release_id;
 IF NOT FOUND OR r.plant_id<>NEW.plant_id OR r.lay_draft_id<>NEW.lay_draft_id OR
  NOT EXISTS (SELECT 1 FROM cut_bundle_drafts_v8 b WHERE b.tenant_id=NEW.tenant_id AND b.lay_draft_id=NEW.lay_draft_id AND b.marker_position=NEW.marker_position AND NEW.ply_number BETWEEN b.ply_start AND b.ply_end) THEN
  RAISE EXCEPTION 'ERR_GEN_GARMENT_CUT_SCOPE'; END IF;
 SELECT count(*)-count(DISTINCT part_code) INTO overlapping FROM cut_bundle_drafts_v8 b
  WHERE b.tenant_id=NEW.tenant_id AND b.lay_draft_id=NEW.lay_draft_id AND b.marker_position=NEW.marker_position
    AND NEW.ply_number BETWEEN b.ply_start AND b.ply_end;
 IF overlapping<>0 THEN RAISE EXCEPTION 'ERR_GEN_AMBIGUOUS_PLY_PART'; END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER gen_garment_guard_v18 BEFORE INSERT ON gen_garments_v18 FOR EACH ROW EXECUTE FUNCTION guard_gen_garment_v18();
CREATE FUNCTION guard_gen_part_v18() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE g gen_garments_v18%ROWTYPE; b gen_bundles_v18%ROWTYPE; bd cut_bundle_drafts_v8%ROWTYPE;
BEGIN
 SELECT * INTO g FROM gen_garments_v18 WHERE tenant_id=NEW.tenant_id AND id=NEW.garment_id;
 SELECT * INTO b FROM gen_bundles_v18 WHERE tenant_id=NEW.tenant_id AND id=NEW.bundle_id;
 SELECT * INTO bd FROM cut_bundle_drafts_v8 WHERE tenant_id=NEW.tenant_id AND lay_draft_id=NEW.lay_draft_id AND label_code=NEW.label_code;
 IF NOT FOUND OR g.id IS NULL OR b.id IS NULL OR b.release_id<>g.release_id OR b.plant_id<>g.plant_id OR
  b.lay_draft_id<>g.lay_draft_id OR b.label_code<>NEW.label_code OR NEW.lay_draft_id<>g.lay_draft_id OR
  bd.marker_position<>g.marker_position OR NEW.part_code<>bd.part_code OR
  g.ply_number NOT BETWEEN bd.ply_start AND bd.ply_end OR NEW.part_slot>bd.pieces_per_garment THEN
  RAISE EXCEPTION 'ERR_GEN_PART_LINEAGE'; END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER gen_part_guard_v18 BEFORE INSERT ON gen_cut_parts_v18 FOR EACH ROW EXECUTE FUNCTION guard_gen_part_v18();
-- Deferred coverage check: one complete provisional part set or no unit at all.
CREATE FUNCTION check_gen_part_coverage_v18() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE expected integer; actual integer;
BEGIN
 SELECT COALESCE(sum(b.pieces_per_garment),0)::integer INTO expected FROM cut_bundle_drafts_v8 b
  WHERE b.tenant_id=NEW.tenant_id AND b.lay_draft_id=NEW.lay_draft_id AND b.marker_position=NEW.marker_position
    AND NEW.ply_number BETWEEN b.ply_start AND b.ply_end;
 SELECT count(*)::integer INTO actual FROM gen_cut_parts_v18 p WHERE p.tenant_id=NEW.tenant_id AND p.garment_id=NEW.id;
 IF expected=0 OR actual<>expected THEN RAISE EXCEPTION 'ERR_GEN_PART_COVERAGE'; END IF;
 RETURN NEW;
END $$;
CREATE CONSTRAINT TRIGGER gen_garment_coverage_v18 AFTER INSERT ON gen_garments_v18
 DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION check_gen_part_coverage_v18();

-- RFID tags are physical, reusable carriers. They are NOT item identities.
-- Enrollment is provisioner-only; currently enrolled legacy roll/bundle tags cannot
-- be silently reused without an explicit future migration/reconciliation.
CREATE TABLE gen_carriers_v18 (
 tenant_id varchar(64) NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
 id uuid NOT NULL DEFAULT uuid_generate_v4(),
 epc_hex varchar(96) NOT NULL CHECK(epc_hex ~ '^[0-9A-F]{24,96}$' AND length(epc_hex)%2=0),
 enabled boolean NOT NULL DEFAULT false,
 enrolled_by varchar(128) NOT NULL,
 enrolled_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(tenant_id,id), UNIQUE(tenant_id,epc_hex)
);
CREATE TABLE gen_carrier_events_v18 (
 event_no bigint GENERATED ALWAYS AS IDENTITY,
 tenant_id varchar(64) NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
 id uuid NOT NULL DEFAULT uuid_generate_v4(),
 command_id varchar(80) NOT NULL CHECK(command_id ~ '^[A-Za-z0-9_-]{12,80}$'),
 command_sha256 char(64) NOT NULL CHECK(command_sha256 ~ '^[0-9a-f]{64}$'),
 carrier_id uuid NOT NULL,
 plant_id varchar(64) NOT NULL,
 action varchar(12) NOT NULL CHECK(action IN('BIND','RETIRE')),
 target_type varchar(12) NOT NULL CHECK(target_type IN('GARMENT','CUT_PART','BUNDLE')),
 target_id uuid NOT NULL,
 worker_subject varchar(128) NOT NULL,
 proof_id varchar(255) NOT NULL,
 assertion_sha256 char(64) NOT NULL CHECK(assertion_sha256 ~ '^[0-9a-f]{64}$'),
 evidence text NOT NULL DEFAULT 'SIGNED_HUMAN_CARRIER_CLAIM_NOT_RF_PROOF'
  CHECK(evidence='SIGNED_HUMAN_CARRIER_CLAIM_NOT_RF_PROOF'),
 created_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(tenant_id,id), UNIQUE(tenant_id,event_no), UNIQUE(tenant_id,command_id), UNIQUE(tenant_id,proof_id),
 FOREIGN KEY(tenant_id,carrier_id) REFERENCES gen_carriers_v18(tenant_id,id) ON DELETE RESTRICT
);
CREATE INDEX gen_carrier_history_v18 ON gen_carrier_events_v18(tenant_id,plant_id,carrier_id,event_no DESC);
CREATE TABLE gen_carrier_active_v18 (
 tenant_id varchar(64) NOT NULL,
 carrier_id uuid NOT NULL,
 plant_id varchar(64) NOT NULL,
 target_type varchar(12) NOT NULL,
 target_id uuid NOT NULL,
 bound_event_id uuid NOT NULL,
 bound_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(tenant_id,carrier_id), UNIQUE(tenant_id,target_type,target_id),
 FOREIGN KEY(tenant_id,carrier_id) REFERENCES gen_carriers_v18(tenant_id,id) ON DELETE RESTRICT,
 FOREIGN KEY(tenant_id,bound_event_id) REFERENCES gen_carrier_events_v18(tenant_id,id) ON DELETE RESTRICT
);
-- Only event trigger may update the active projection (not the application role).
CREATE FUNCTION apply_gen_carrier_event_v18() RETURNS trigger LANGUAGE plpgsql SECURITY DEFINER
 SET search_path=pg_catalog,public AS $$
DECLARE c public.gen_carriers_v18%ROWTYPE; active public.gen_carrier_active_v18%ROWTYPE; actual_plant text;
BEGIN
 -- Shares the original roll/bundle EPC namespace lock; both old and new
 -- writers participate, preventing a concurrent cross-table double binding.
 PERFORM pg_advisory_xact_lock(hashtext(NEW.tenant_id),hashtext('RFID_EPC_GLOBAL_V8'));
 SELECT * INTO c FROM public.gen_carriers_v18 WHERE tenant_id=NEW.tenant_id AND id=NEW.carrier_id FOR UPDATE;
 IF NOT FOUND OR (NEW.action='BIND' AND NOT c.enabled) THEN RAISE EXCEPTION 'ERR_GEN_CARRIER_NOT_ENROLLED'; END IF;
 CASE NEW.target_type
  WHEN 'GARMENT' THEN SELECT plant_id INTO actual_plant FROM public.gen_garments_v18 WHERE tenant_id=NEW.tenant_id AND id=NEW.target_id;
  WHEN 'CUT_PART' THEN SELECT g.plant_id INTO actual_plant FROM public.gen_cut_parts_v18 p JOIN public.gen_garments_v18 g ON g.tenant_id=p.tenant_id AND g.id=p.garment_id WHERE p.tenant_id=NEW.tenant_id AND p.id=NEW.target_id;
  WHEN 'BUNDLE' THEN SELECT plant_id INTO actual_plant FROM public.gen_bundles_v18 WHERE tenant_id=NEW.tenant_id AND id=NEW.target_id;
 END CASE;
 IF actual_plant IS NULL OR actual_plant<>NEW.plant_id THEN RAISE EXCEPTION 'ERR_GEN_CARRIER_TARGET_SCOPE'; END IF;
 SELECT * INTO active FROM public.gen_carrier_active_v18 WHERE tenant_id=NEW.tenant_id AND carrier_id=NEW.carrier_id;
 IF NEW.action='BIND' THEN
  IF active.carrier_id IS NOT NULL OR
   EXISTS (SELECT 1 FROM public.cut_bundle_scan_witnesses_v8 w WHERE w.tenant_id=NEW.tenant_id AND w.epc_hex=c.epc_hex) OR
   EXISTS (SELECT 1 FROM public.store_roll_rfid_bindings_v7 r WHERE r.tenant_id=NEW.tenant_id AND r.epc_hex=c.epc_hex)
  THEN RAISE EXCEPTION 'ERR_GEN_CARRIER_ALREADY_ASSIGNED_OR_LEGACY'; END IF;
  INSERT INTO public.gen_carrier_active_v18(tenant_id,carrier_id,plant_id,target_type,target_id,bound_event_id)
   VALUES(NEW.tenant_id,NEW.carrier_id,NEW.plant_id,NEW.target_type,NEW.target_id,NEW.id);
 ELSE
  IF active.carrier_id IS NULL OR active.target_type<>NEW.target_type OR active.target_id<>NEW.target_id OR active.plant_id<>NEW.plant_id THEN
   RAISE EXCEPTION 'ERR_GEN_CARRIER_RETIRE_MISMATCH'; END IF;
  DELETE FROM public.gen_carrier_active_v18 WHERE tenant_id=NEW.tenant_id AND carrier_id=NEW.carrier_id;
 END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER gen_carrier_apply_v18 AFTER INSERT ON gen_carrier_events_v18
 FOR EACH ROW EXECUTE FUNCTION apply_gen_carrier_event_v18();
REVOKE ALL ON FUNCTION apply_gen_carrier_event_v18() FROM PUBLIC;
-- Prevent a new legacy roll/bundle EPC record from arriving after this ledger
-- has bound the same tag. The legacy ingress already takes this advisory lock.
CREATE FUNCTION guard_legacy_carrier_collision_v18() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 PERFORM pg_advisory_xact_lock(hashtext(NEW.tenant_id),hashtext('RFID_EPC_GLOBAL_V8'));
 IF EXISTS (
  SELECT 1 FROM gen_carrier_active_v18 a JOIN gen_carriers_v18 c
   ON c.tenant_id=a.tenant_id AND c.id=a.carrier_id
  WHERE a.tenant_id=NEW.tenant_id AND c.epc_hex=NEW.epc_hex
 ) THEN RAISE EXCEPTION 'ERR_GEN_CARRIER_ALREADY_ACTIVE'; END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER gen_no_legacy_roll_collision_v18 BEFORE INSERT ON store_roll_rfid_bindings_v7
 FOR EACH ROW EXECUTE FUNCTION guard_legacy_carrier_collision_v18();
CREATE TRIGGER gen_no_legacy_bundle_collision_v18 BEFORE INSERT ON cut_bundle_scan_witnesses_v8
 FOR EACH ROW EXECUTE FUNCTION guard_legacy_carrier_collision_v18();

-- RLS and grants; no direct writes to carrier registry or active projection.
DO $$ DECLARE tbl text; BEGIN
 FOREACH tbl IN ARRAY ARRAY['gen_bundles_v18','gen_garments_v18','gen_cut_parts_v18','gen_carriers_v18','gen_carrier_events_v18','gen_carrier_active_v18'] LOOP
  EXECUTE format('ALTER TABLE public.%I ENABLE ROW LEVEL SECURITY',tbl);
  EXECUTE format('ALTER TABLE public.%I FORCE ROW LEVEL SECURITY',tbl);
  EXECUTE format('CREATE POLICY gen_tenant_v18 ON public.%I FOR ALL TO agiparel_app USING (tenant_id=NULLIF(current_setting(''app.current_tenant_id'',true),'''')) WITH CHECK (tenant_id=NULLIF(current_setting(''app.current_tenant_id'',true),''''))',tbl);
  EXECUTE format('REVOKE ALL ON public.%I FROM PUBLIC,agiparel_app',tbl);
  EXECUTE format('GRANT SELECT ON public.%I TO agiparel_app',tbl);
 END LOOP;
END $$;
GRANT INSERT ON gen_bundles_v18,gen_garments_v18,gen_cut_parts_v18,gen_carrier_events_v18 TO agiparel_app;
DO $$ DECLARE tbl text; BEGIN
 FOREACH tbl IN ARRAY ARRAY['gen_bundles_v18','gen_garments_v18','gen_cut_parts_v18','gen_carrier_events_v18'] LOOP
  EXECUTE format('CREATE TRIGGER gen_immutable_v18 BEFORE UPDATE OR DELETE ON public.%I FOR EACH ROW EXECUTE FUNCTION protect_cut_draft_v8()',tbl);
 END LOOP;
END $$;
COMMIT;
