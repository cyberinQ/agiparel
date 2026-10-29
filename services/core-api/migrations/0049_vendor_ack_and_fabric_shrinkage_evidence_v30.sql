-- Author: Sunil Jain, CyberinQ LLC, 2026-10-29.
-- F3.17C + F3.18A: independent vendor signature evidence and human-attested shrinkage measurements.
-- Neither ledger grants QC release, sewing credit, cutting approval, or vendor financial settlement.
BEGIN;
INSERT INTO workforce_chapter_actions_v17(chapter_id,permission,authority) VALUES
 (6,'jobwork.vendor.ack.record','SIGNED_WRITE'),
 (4,'stores.fabric.shrinkage.record','SIGNED_WRITE') ON CONFLICT DO NOTHING;

-- Provisioner-owned vendor keys; app role can only SELECT. Revocation is prospective.
CREATE TABLE jobwork_vendor_keys_v30 (
 tenant_id varchar(64) NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
 plant_id varchar(64) NOT NULL,
 vendor_id varchar(64) NOT NULL,
 key_id varchar(64) NOT NULL CHECK(key_id ~ '^[A-Za-z0-9_-]{4,64}$'),
 public_key_hex char(64) NOT NULL CHECK(public_key_hex ~ '^[0-9a-f]{64}$'),
 enabled boolean NOT NULL DEFAULT false,
 registered_by varchar(128) NOT NULL CHECK(length(btrim(registered_by))>0),
 registered_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(tenant_id,plant_id,vendor_id,key_id),
 FOREIGN KEY(tenant_id,plant_id,vendor_id) REFERENCES jobwork_vendors_v28(tenant_id,plant_id,vendor_id) ON DELETE RESTRICT
);
CREATE TABLE jobwork_vendor_acks_v30 (
 tenant_id varchar(64) NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
 id uuid NOT NULL DEFAULT uuid_generate_v4(),
 plant_id varchar(64) NOT NULL,
 outward_id uuid NOT NULL,
 part_id uuid NOT NULL,
 job_reference varchar(64) NOT NULL,
 process_code varchar(32) NOT NULL CHECK(process_code IN ('EMBROIDERY','PRINTING','WASHING','DYEING','OTHER_FINISHING')),
 vendor_id varchar(64) NOT NULL,
 key_id varchar(64) NOT NULL,
 ack_kind varchar(32) NOT NULL CHECK(ack_kind='OUTWARD_REPORTED_RECEIVED'),
 command_id varchar(80) NOT NULL CHECK(command_id ~ '^[A-Za-z0-9_-]{12,80}$'),
 command_sha256 char(64) NOT NULL CHECK(command_sha256 ~ '^[0-9a-f]{64}$'),
 vendor_signature_hex char(128) NOT NULL CHECK(vendor_signature_hex ~ '^[0-9a-f]{128}$'),
 key_sha256 char(64) NOT NULL CHECK(key_sha256 ~ '^[0-9a-f]{64}$'),
 recorder_subject varchar(128) NOT NULL CHECK(length(btrim(recorder_subject))>0),
 recorder_proof_id varchar(255) NOT NULL CHECK(length(btrim(recorder_proof_id))>0),
 recorder_assertion_sha256 char(64) NOT NULL CHECK(recorder_assertion_sha256 ~ '^[0-9a-f]{64}$'),
 recorded_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 authority text NOT NULL DEFAULT 'SIGNED_VENDOR_ACKNOWLEDGEMENT_NOT_RF_PROOF_QC_OR_SETTLEMENT'
 CHECK(authority='SIGNED_VENDOR_ACKNOWLEDGEMENT_NOT_RF_PROOF_QC_OR_SETTLEMENT'),
 PRIMARY KEY(tenant_id,id),
 UNIQUE(tenant_id,outward_id,ack_kind),
 UNIQUE(tenant_id,command_id),
 UNIQUE(tenant_id,recorder_proof_id),
 UNIQUE(tenant_id,vendor_signature_hex),
 FOREIGN KEY(tenant_id,outward_id) REFERENCES jobwork_outwards_v28(tenant_id,id) ON DELETE RESTRICT,
 FOREIGN KEY(tenant_id,part_id) REFERENCES gen_cut_parts_v18(tenant_id,id) ON DELETE RESTRICT,
 FOREIGN KEY(tenant_id,plant_id,vendor_id,key_id) REFERENCES jobwork_vendor_keys_v30(tenant_id,plant_id,vendor_id,key_id) ON DELETE RESTRICT
);
CREATE FUNCTION guard_jobwork_vendor_ack_v30() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE o jobwork_outwards_v28%ROWTYPE; k jobwork_vendor_keys_v30%ROWTYPE;
BEGIN
 PERFORM pg_advisory_xact_lock(hashtext(NEW.tenant_id),hashtext('F317:OUTWARD:'||NEW.outward_id::text));
 SELECT * INTO o FROM jobwork_outwards_v28 WHERE tenant_id=NEW.tenant_id AND id=NEW.outward_id;
 SELECT * INTO k FROM jobwork_vendor_keys_v30 WHERE tenant_id=NEW.tenant_id
  AND plant_id=NEW.plant_id AND vendor_id=NEW.vendor_id AND key_id=NEW.key_id;
 IF o.id IS NULL OR o.plant_id<>NEW.plant_id OR o.vendor_id<>NEW.vendor_id
  OR o.part_id<>NEW.part_id OR o.process_code<>NEW.process_code OR o.job_reference<>NEW.job_reference
  OR k.key_id IS NULL OR NOT k.enabled OR
  NOT EXISTS(SELECT 1 FROM jobwork_vendors_v28 v WHERE v.tenant_id=NEW.tenant_id
   AND v.plant_id=NEW.plant_id AND v.vendor_id=NEW.vendor_id AND v.enabled)
 THEN RAISE EXCEPTION 'ERR_F317C_VENDOR_NOT_ENROLLED_OR_SCOPE'; END IF;
 IF NEW.key_sha256<>encode(digest(decode(k.public_key_hex,'hex'),'sha256'),'hex')
 THEN RAISE EXCEPTION 'ERR_F317C_KEY_HASH_MISMATCH'; END IF;
 IF NEW.recorder_subject=o.sender_subject OR EXISTS(SELECT 1 FROM jobwork_used_proofs_v28 p
  WHERE p.tenant_id=NEW.tenant_id AND p.proof_id=NEW.recorder_proof_id)
  OR EXISTS(SELECT 1 FROM jobwork_discrepancies_v29 d
   WHERE d.tenant_id=NEW.tenant_id AND d.reporter_proof_id=NEW.recorder_proof_id)
 THEN RAISE EXCEPTION 'ERR_F317C_RECORDER_NOT_INDEPENDENT_OR_PROOF_SPENT'; END IF;
 NEW.recorded_at:=clock_timestamp();
 RETURN NEW;
END $$;
CREATE TRIGGER jobwork_vendor_ack_guard_v30 BEFORE INSERT ON jobwork_vendor_acks_v30
 FOR EACH ROW EXECUTE FUNCTION guard_jobwork_vendor_ack_v30();

-- Attestation is to a reported test, not a lab instrument calibration or buyer specification PASS.
CREATE TABLE fabric_shrinkage_observations_v30 (
 tenant_id varchar(64) NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
 id uuid NOT NULL DEFAULT uuid_generate_v4(),
 plant_id varchar(64) NOT NULL,
 roll_id varchar(64) NOT NULL,
 material_sku varchar(128) NOT NULL,
 sample_id varchar(80) NOT NULL CHECK(sample_id ~ '^[A-Za-z0-9_.-]{4,80}$'),
 treatment_code varchar(32) NOT NULL CHECK(treatment_code IN ('WASH','STEAM','RELAXATION','OTHER')),
 warp_before_mm integer NOT NULL CHECK(warp_before_mm BETWEEN 50 AND 10000),
 warp_after_mm integer NOT NULL CHECK(warp_after_mm BETWEEN 25 AND 15000),
 weft_before_mm integer NOT NULL CHECK(weft_before_mm BETWEEN 50 AND 10000),
 weft_after_mm integer NOT NULL CHECK(weft_after_mm BETWEEN 25 AND 15000),
 warp_change_bp integer GENERATED ALWAYS AS (round((warp_before_mm-warp_after_mm)::numeric*10000/warp_before_mm)::integer) STORED,
 weft_change_bp integer GENERATED ALWAYS AS (round((weft_before_mm-weft_after_mm)::numeric*10000/weft_before_mm)::integer) STORED,
 source_document_id varchar(36) NOT NULL,
 source_sha256 char(64) NOT NULL CHECK(source_sha256 ~ '^[0-9a-f]{64}$'),
 method_note text NOT NULL CHECK(length(btrim(method_note)) BETWEEN 20 AND 800),
 command_id varchar(80) NOT NULL CHECK(command_id ~ '^[A-Za-z0-9_-]{12,80}$'),
 command_sha256 char(64) NOT NULL CHECK(command_sha256 ~ '^[0-9a-f]{64}$'),
 tester_subject varchar(128) NOT NULL CHECK(length(btrim(tester_subject))>0),
 tester_proof_id varchar(255) NOT NULL CHECK(length(btrim(tester_proof_id))>0),
 assertion_sha256 char(64) NOT NULL CHECK(assertion_sha256 ~ '^[0-9a-f]{64}$'),
 recorded_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 authority text NOT NULL DEFAULT 'SIGNED_HUMAN_REPORTED_SHRINKAGE_NOT_CALIBRATED_OR_BUYER_APPROVED'
 CHECK(authority='SIGNED_HUMAN_REPORTED_SHRINKAGE_NOT_CALIBRATED_OR_BUYER_APPROVED'),
 PRIMARY KEY(tenant_id,id), UNIQUE(tenant_id,command_id), UNIQUE(tenant_id,tester_proof_id),
 UNIQUE(tenant_id,plant_id,roll_id,sample_id,treatment_code),
 FOREIGN KEY(tenant_id,source_document_id) REFERENCES document_sources_v65(tenant_id,id) ON DELETE RESTRICT
);
CREATE INDEX fabric_shrinkage_roll_v30 ON fabric_shrinkage_observations_v30(tenant_id,plant_id,roll_id,recorded_at DESC);
CREATE FUNCTION guard_fabric_shrinkage_v30() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE sku varchar(128); pl varchar(64); doc_hash char(64);
BEGIN
 SELECT plant_id,factory_sku INTO pl,sku FROM store_rolls
  WHERE tenant_id=NEW.tenant_id AND id=NEW.roll_id;
 IF NOT FOUND OR pl<>NEW.plant_id OR sku<>NEW.material_sku
 THEN RAISE EXCEPTION 'ERR_F318A_ROLL_SCOPE'; END IF;
 SELECT sha256 INTO doc_hash FROM document_sources_v65
  WHERE tenant_id=NEW.tenant_id AND id=NEW.source_document_id;
 IF doc_hash IS NULL OR doc_hash<>NEW.source_sha256
 THEN RAISE EXCEPTION 'ERR_F318A_SOURCE_HASH'; END IF;
 IF abs(NEW.warp_before_mm-NEW.warp_after_mm)*100 > NEW.warp_before_mm*30
 OR abs(NEW.weft_before_mm-NEW.weft_after_mm)*100 > NEW.weft_before_mm*30
 THEN RAISE EXCEPTION 'ERR_F318A_OUT_OF_BOUNDS'; END IF;
 NEW.recorded_at:=clock_timestamp();
 RETURN NEW;
END $$;
CREATE TRIGGER fabric_shrinkage_guard_v30 BEFORE INSERT ON fabric_shrinkage_observations_v30
 FOR EACH ROW EXECUTE FUNCTION guard_fabric_shrinkage_v30();

DO $$ DECLARE tbl text; BEGIN
 FOREACH tbl IN ARRAY ARRAY['jobwork_vendor_keys_v30','jobwork_vendor_acks_v30','fabric_shrinkage_observations_v30'] LOOP
  EXECUTE format('ALTER TABLE public.%I ENABLE ROW LEVEL SECURITY',tbl);
  EXECUTE format('ALTER TABLE public.%I FORCE ROW LEVEL SECURITY',tbl);
  EXECUTE format('CREATE POLICY f317c_f318a_tenant_v30 ON public.%I FOR ALL TO agiparel_app USING (tenant_id=NULLIF(current_setting(%L,true),%L)) WITH CHECK (tenant_id=NULLIF(current_setting(%L,true),%L))',tbl,'app.current_tenant_id','','app.current_tenant_id','');
  EXECUTE format('REVOKE ALL ON public.%I FROM PUBLIC,agiparel_app',tbl);
  EXECUTE format('GRANT SELECT ON public.%I TO agiparel_app',tbl);
 END LOOP;
END $$;
GRANT INSERT ON jobwork_vendor_acks_v30,fabric_shrinkage_observations_v30 TO agiparel_app;
CREATE TRIGGER jobwork_vendor_ack_immutable_v30 BEFORE UPDATE OR DELETE ON jobwork_vendor_acks_v30
 FOR EACH ROW EXECUTE FUNCTION protect_cut_draft_v8();
CREATE TRIGGER fabric_shrinkage_immutable_v30 BEFORE UPDATE OR DELETE ON fabric_shrinkage_observations_v30
 FOR EACH ROW EXECUTE FUNCTION protect_cut_draft_v8();
COMMIT;
