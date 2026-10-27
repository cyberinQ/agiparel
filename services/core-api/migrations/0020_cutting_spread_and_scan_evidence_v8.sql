-- Agiparel Batch 08B. The records below are append-only EVIDENCE, not a
-- signed cutting release, stock adjustment, or proof of hardware RF capture.
-- Requires migrations 0001..0019, especially material_issues_v5 and 08A lays.
BEGIN;
-- The legacy issue ledger keys by global UUID; add composite identity for tenant-safe FKs.
CREATE UNIQUE INDEX IF NOT EXISTS material_issues_tenant_identity_v8 ON material_issues_v5(tenant_id,id);
CREATE TABLE cut_spread_reports_v8 (
 id uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
 tenant_id varchar(64) NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
 lay_draft_id uuid NOT NULL,
 request_key varchar(128) NOT NULL CHECK(request_key ~ '^[A-Za-z0-9_-]{8,128}$'),
 input_sha256 char(64) NOT NULL CHECK(input_sha256 ~ '^[0-9a-f]{64}$'),
 draft_snapshot_sha256 char(64) NOT NULL CHECK(draft_snapshot_sha256 ~ '^[0-9a-f]{64}$'),
 reported_actual_plies integer NOT NULL CHECK(reported_actual_plies BETWEEN 1 AND 210),
 measured_marker_length_mm bigint NOT NULL CHECK(measured_marker_length_mm BETWEEN 100 AND 100000000),
 reported_consumption_mm bigint NOT NULL CHECK(reported_consumption_mm > 0),
 evidence_note text NOT NULL CHECK(length(btrim(evidence_note)) BETWEEN 12 AND 1000),
 state varchar(48) NOT NULL DEFAULT 'REPORTED_PENDING_INDEPENDENT_VERIFICATION'
  CHECK(state='REPORTED_PENDING_INDEPENDENT_VERIFICATION'),
 created_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE(tenant_id,id), UNIQUE(tenant_id,lay_draft_id), UNIQUE(tenant_id,request_key),
 FOREIGN KEY(tenant_id,lay_draft_id) REFERENCES cut_lay_drafts_v8(tenant_id,id) ON DELETE RESTRICT
);
CREATE UNIQUE INDEX cut_spread_report_lineage_v8 ON cut_spread_reports_v8(tenant_id,id,lay_draft_id);
CREATE UNIQUE INDEX cut_bundle_label_lineage_v8 ON cut_bundle_drafts_v8(tenant_id,lay_draft_id,label_code);

CREATE TABLE cut_spread_issue_allocations_v8 (
 tenant_id varchar(64) NOT NULL,
 spread_report_id uuid NOT NULL,
 issue_id uuid NOT NULL,
 allocated_mm bigint NOT NULL CHECK(allocated_mm > 0),
 PRIMARY KEY(tenant_id,spread_report_id,issue_id),
 FOREIGN KEY(tenant_id,spread_report_id) REFERENCES cut_spread_reports_v8(tenant_id,id) ON DELETE RESTRICT,
 FOREIGN KEY(tenant_id,issue_id) REFERENCES material_issues_v5(tenant_id,id) ON DELETE RESTRICT
);
CREATE INDEX cut_spread_allocation_by_issue_v8 ON cut_spread_issue_allocations_v8(tenant_id,issue_id);

-- Database-side guard: the application role cannot invent consumption in a
-- different buyer/cut/shade or allocate one signed issue twice across cuts.
CREATE FUNCTION validate_cut_issue_attribution_v8() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE signed_mm bigint; already_mm bigint; okay boolean;
BEGIN
 SELECT i.length_mm,
  (i.cut_no=d.cut_no AND (i.order_id=d.intake_id OR
     (i.order_id=o.po_number AND
       (SELECT count(*) FROM order_intakes_v5 oo WHERE oo.tenant_id=d.tenant_id AND oo.po_number=o.po_number)=1))
    AND sr.factory_sku=d.fabric_sku AND sr.shade_band=d.shade_band)
 INTO signed_mm,okay
 FROM material_issues_v5 i
 JOIN store_rolls sr ON sr.tenant_id=i.tenant_id AND sr.id=i.roll_id
 JOIN cut_spread_reports_v8 s ON s.tenant_id=NEW.tenant_id AND s.id=NEW.spread_report_id
 JOIN cut_lay_drafts_v8 d ON d.tenant_id=s.tenant_id AND d.id=s.lay_draft_id
 JOIN order_intakes_v5 o ON o.tenant_id=d.tenant_id AND o.id=d.intake_id
 WHERE i.tenant_id=NEW.tenant_id AND i.id=NEW.issue_id FOR UPDATE OF i;
 IF NOT FOUND OR NOT okay THEN RAISE EXCEPTION 'ERR_CUT_ISSUE_SCOPE_MISMATCH'; END IF;
 SELECT COALESCE(sum(allocated_mm),0) INTO already_mm
 FROM cut_spread_issue_allocations_v8 WHERE tenant_id=NEW.tenant_id AND issue_id=NEW.issue_id;
 IF already_mm>signed_mm OR NEW.allocated_mm>signed_mm-already_mm THEN
  RAISE EXCEPTION 'ERR_CUT_ISSUE_DOUBLE_ATTRIBUTION';
 END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER cut_issue_attribution_guard_v8 BEFORE INSERT ON cut_spread_issue_allocations_v8
 FOR EACH ROW EXECUTE FUNCTION validate_cut_issue_attribution_v8();

-- The spread claim and per-issue allocations are inserted atomically by the
-- Go handler. Deferred end-of-transaction assertion forbids orphaned or
-- partially attributed consumption even from a direct SQL writer.
CREATE FUNCTION check_cut_spread_coverage_v8() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE covered_mm bigint;
BEGIN
 SELECT COALESCE(sum(a.allocated_mm),0) INTO covered_mm
 FROM cut_spread_issue_allocations_v8 a
 WHERE a.tenant_id=NEW.tenant_id AND a.spread_report_id=NEW.id;
 IF covered_mm<>NEW.reported_consumption_mm THEN
   RAISE EXCEPTION 'ERR_CUT_SPREAD_CONSUMPTION_NOT_COVERED';
 END IF;
 RETURN NEW;
END $$;
CREATE CONSTRAINT TRIGGER cut_spread_full_coverage_v8
 AFTER INSERT ON cut_spread_reports_v8 DEFERRABLE INITIALLY DEFERRED
 FOR EACH ROW EXECUTE FUNCTION check_cut_spread_coverage_v8();


-- Trust boundary: every scan is witnessed by a strong externally signed human
-- identity (not a dev login). This does NOT prove a physical reader antenna
-- observed RF energy; a reader-gateway attestation is a later integration.
CREATE TABLE cut_bundle_scan_witnesses_v8 (
 id uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
 tenant_id varchar(64) NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
 plant_id varchar(64) NOT NULL,
 spread_report_id uuid NOT NULL,
 lay_draft_id uuid NOT NULL,
 label_code varchar(29) NOT NULL,
 epc_hex varchar(96) NOT NULL CHECK(epc_hex ~ '^[0-9A-F]{24,96}$' AND length(epc_hex)%2=0),
 scanner_subject varchar(128) NOT NULL,
 scanner_proof_id varchar(255) NOT NULL,
 scanner_assertion_hash char(64) NOT NULL CHECK(scanner_assertion_hash ~ '^[0-9a-f]{64}$'),
 signed_challenge_hash char(64) NOT NULL CHECK(signed_challenge_hash ~ '^[0-9a-f]{64}$'),
 witness_state varchar(48) NOT NULL DEFAULT 'SIGNED_HUMAN_WITNESS_NOT_HARDWARE_ATTESTED'
  CHECK(witness_state='SIGNED_HUMAN_WITNESS_NOT_HARDWARE_ATTESTED'),
 created_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE(tenant_id,epc_hex), UNIQUE(tenant_id,lay_draft_id,label_code),
 UNIQUE(tenant_id,scanner_proof_id),
 FOREIGN KEY(tenant_id,spread_report_id,lay_draft_id) REFERENCES cut_spread_reports_v8(tenant_id,id,lay_draft_id) ON DELETE RESTRICT,
 FOREIGN KEY(tenant_id,lay_draft_id,label_code) REFERENCES cut_bundle_drafts_v8(tenant_id,lay_draft_id,label_code) ON DELETE RESTRICT
);
CREATE INDEX cut_bundle_scans_by_spread_v8 ON cut_bundle_scan_witnesses_v8(tenant_id,spread_report_id);
-- Share one EPC namespace between physical roll tags and bundle tags.
-- Per-tenant transaction advisory locking prevents simultaneous writers from
-- successfully introducing the same EPC in different tables.
CREATE FUNCTION prevent_cross_type_epc_v8() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 PERFORM pg_advisory_xact_lock(hashtext(NEW.tenant_id),hashtext('RFID_EPC_GLOBAL_V8'));
 IF TG_TABLE_NAME='store_roll_rfid_bindings_v7' THEN
   IF EXISTS(SELECT 1 FROM cut_bundle_scan_witnesses_v8 WHERE tenant_id=NEW.tenant_id AND epc_hex=NEW.epc_hex) THEN
     RAISE EXCEPTION 'ERR_EPC_ALREADY_BOUND_TO_CUT_BUNDLE';
   END IF;
 ELSE
   IF EXISTS(SELECT 1 FROM store_roll_rfid_bindings_v7 WHERE tenant_id=NEW.tenant_id AND epc_hex=NEW.epc_hex) THEN
     RAISE EXCEPTION 'ERR_EPC_ALREADY_BOUND_TO_FABRIC_ROLL';
   END IF;
 END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER epc_namespace_roll_v8 BEFORE INSERT ON store_roll_rfid_bindings_v7
 FOR EACH ROW EXECUTE FUNCTION prevent_cross_type_epc_v8();
CREATE TRIGGER epc_namespace_bundle_v8 BEFORE INSERT ON cut_bundle_scan_witnesses_v8
 FOR EACH ROW EXECUTE FUNCTION prevent_cross_type_epc_v8();

DO $$ DECLARE tbl text; BEGIN
 FOREACH tbl IN ARRAY ARRAY['cut_spread_reports_v8','cut_spread_issue_allocations_v8','cut_bundle_scan_witnesses_v8'] LOOP
  EXECUTE format('ALTER TABLE public.%I ENABLE ROW LEVEL SECURITY',tbl);
  EXECUTE format('ALTER TABLE public.%I FORCE ROW LEVEL SECURITY',tbl);
  EXECUTE format('CREATE POLICY cutting_evidence_tenant_v8 ON public.%I FOR ALL TO agiparel_app USING(tenant_id=NULLIF(current_setting(''app.current_tenant_id'',true),'''')) WITH CHECK(tenant_id=NULLIF(current_setting(''app.current_tenant_id'',true),''''))',tbl);
  EXECUTE format('REVOKE ALL ON public.%I FROM PUBLIC,agiparel_app',tbl);
  EXECUTE format('GRANT SELECT,INSERT ON public.%I TO agiparel_app',tbl);
  EXECUTE format('CREATE TRIGGER cutting_evidence_immutable_v8 BEFORE UPDATE OR DELETE ON public.%I FOR EACH ROW EXECUTE FUNCTION protect_cut_draft_v8()',tbl);
 END LOOP;
END $$;
COMMIT;
