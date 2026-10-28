-- Author: Sunil Jain, 2026-10-28.
-- F3.2: dual-attested fabric allocation classification, part acceptance/rejection,
-- and non-credit recut lineage. Human attestations are NOT machine measurements.
-- Source: signed material issues -> spread allocations -> signed cutting release (0021)
-- -> provisional cut-part identity (0037). No finished goods/payroll/shipping credit.
BEGIN;
CREATE TABLE cut_consumption_reconciliations_v19 (
 tenant_id varchar(64) NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
 id uuid NOT NULL DEFAULT uuid_generate_v4(),
 plant_id varchar(64) NOT NULL,
 release_id uuid NOT NULL,
 spread_report_id uuid NOT NULL,
 used_part_mm bigint NOT NULL CHECK(used_part_mm>=0),
 trim_waste_mm bigint NOT NULL CHECK(trim_waste_mm>=0),
 damaged_mm bigint NOT NULL CHECK(damaged_mm>=0),
 command_id varchar(80) NOT NULL CHECK(command_id ~ '^[A-Za-z0-9_-]{12,80}$'),
 command_sha256 char(64) NOT NULL CHECK(command_sha256 ~ '^[0-9a-f]{64}$'),
 supervisor_subject varchar(128) NOT NULL,
 quality_subject varchar(128) NOT NULL,
 supervisor_proof_id varchar(255) NOT NULL,
 quality_proof_id varchar(255) NOT NULL,
 supervisor_assertion_sha256 char(64) NOT NULL,
 quality_assertion_sha256 char(64) NOT NULL,
 note text NOT NULL CHECK(length(btrim(note)) BETWEEN 12 AND 1000),
 evidence text NOT NULL DEFAULT 'DUAL_HUMAN_CLASSIFICATION_NOT_SENSOR_MEASUREMENT'
  CHECK(evidence='DUAL_HUMAN_CLASSIFICATION_NOT_SENSOR_MEASUREMENT'),
 created_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(tenant_id,id), UNIQUE(tenant_id,command_id),
 UNIQUE(tenant_id,spread_report_id), UNIQUE(tenant_id,release_id),
 CHECK(supervisor_subject<>quality_subject AND supervisor_proof_id<>quality_proof_id),
 FOREIGN KEY(tenant_id,release_id) REFERENCES cut_sewing_releases_v8(tenant_id,id) ON DELETE RESTRICT,
 FOREIGN KEY(tenant_id,spread_report_id) REFERENCES cut_spread_reports_v8(tenant_id,id) ON DELETE RESTRICT
);
CREATE INDEX cut_reconcile_by_plant_v19 ON cut_consumption_reconciliations_v19(tenant_id,plant_id,created_at DESC,id);

-- Provisioner-owned defect codes, no development operator may modify this catalog.
CREATE TABLE cut_defect_codes_v19 (
 tenant_id varchar(64) NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
 plant_id varchar(64) NOT NULL,
 code varchar(64) NOT NULL CHECK(code ~ '^[A-Za-z0-9_-]{2,64}$'),
 description varchar(512) NOT NULL CHECK(length(btrim(description))>0),
 enabled boolean NOT NULL DEFAULT false,
 PRIMARY KEY(tenant_id,plant_id,code)
);
CREATE TABLE cut_part_outcomes_v19 (
 tenant_id varchar(64) NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
 id uuid NOT NULL DEFAULT uuid_generate_v4(),
 plant_id varchar(64) NOT NULL,
 part_id uuid NOT NULL,
 release_id uuid NOT NULL,
 disposition varchar(12) NOT NULL CHECK(disposition IN('ACCEPTED','REJECTED')),
 defect_code varchar(64),
 command_id varchar(80) NOT NULL CHECK(command_id ~ '^[A-Za-z0-9_-]{12,80}$'),
 command_sha256 char(64) NOT NULL CHECK(command_sha256 ~ '^[0-9a-f]{64}$'),
 supervisor_subject varchar(128) NOT NULL,
 quality_subject varchar(128) NOT NULL,
 supervisor_proof_id varchar(255) NOT NULL,
 quality_proof_id varchar(255) NOT NULL,
 supervisor_assertion_sha256 char(64) NOT NULL,
 quality_assertion_sha256 char(64) NOT NULL,
 note text NOT NULL CHECK(length(btrim(note)) BETWEEN 12 AND 1000),
 evidence text NOT NULL DEFAULT 'DUAL_HUMAN_PART_DISPOSITION_NOT_SENSOR_VERIFIED'
  CHECK(evidence='DUAL_HUMAN_PART_DISPOSITION_NOT_SENSOR_VERIFIED'),
 created_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(tenant_id,id), UNIQUE(tenant_id,part_id), UNIQUE(tenant_id,command_id),
 CHECK(supervisor_subject<>quality_subject AND supervisor_proof_id<>quality_proof_id),
 CHECK((disposition='ACCEPTED' AND defect_code IS NULL) OR (disposition='REJECTED' AND defect_code IS NOT NULL)),
 FOREIGN KEY(tenant_id,part_id) REFERENCES gen_cut_parts_v18(tenant_id,id) ON DELETE RESTRICT,
 FOREIGN KEY(tenant_id,release_id) REFERENCES cut_sewing_releases_v8(tenant_id,id) ON DELETE RESTRICT
);
CREATE INDEX cut_part_outcomes_by_release_v19 ON cut_part_outcomes_v19(tenant_id,plant_id,release_id,disposition);
CREATE TABLE cut_recut_links_v19 (
 tenant_id varchar(64) NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
 id uuid NOT NULL DEFAULT uuid_generate_v4(),
 plant_id varchar(64) NOT NULL,
 original_part_id uuid NOT NULL,
 replacement_part_id uuid NOT NULL,
 release_id uuid NOT NULL,
 command_id varchar(80) NOT NULL CHECK(command_id ~ '^[A-Za-z0-9_-]{12,80}$'),
 command_sha256 char(64) NOT NULL CHECK(command_sha256 ~ '^[0-9a-f]{64}$'),
 supervisor_subject varchar(128) NOT NULL,
 quality_subject varchar(128) NOT NULL,
 supervisor_proof_id varchar(255) NOT NULL,
 quality_proof_id varchar(255) NOT NULL,
 supervisor_assertion_sha256 char(64) NOT NULL,
 quality_assertion_sha256 char(64) NOT NULL,
 note text NOT NULL CHECK(length(btrim(note)) BETWEEN 12 AND 1000),
 evidence text NOT NULL DEFAULT 'IDENTIFIED_RECUT_NOT_ACCEPTED_OR_CONSUMPTION_CREDIT'
  CHECK(evidence='IDENTIFIED_RECUT_NOT_ACCEPTED_OR_CONSUMPTION_CREDIT'),
 created_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(tenant_id,id), UNIQUE(tenant_id,original_part_id),
 UNIQUE(tenant_id,replacement_part_id), UNIQUE(tenant_id,command_id),
 CHECK(original_part_id<>replacement_part_id AND supervisor_subject<>quality_subject AND supervisor_proof_id<>quality_proof_id),
 FOREIGN KEY(tenant_id,release_id) REFERENCES cut_sewing_releases_v8(tenant_id,id) ON DELETE RESTRICT,
 FOREIGN KEY(tenant_id,original_part_id) REFERENCES gen_cut_parts_v18(tenant_id,id) ON DELETE RESTRICT,
 FOREIGN KEY(tenant_id,replacement_part_id) REFERENCES gen_cut_parts_v18(tenant_id,id) ON DELETE RESTRICT
);

-- One global challenge JTI namespace across all three event types.
CREATE TABLE cut_proof_consumptions_v19 (
 tenant_id varchar(64) NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
 proof_id varchar(255) NOT NULL,
 entity_type varchar(16) NOT NULL CHECK(entity_type IN ('CONSUMPTION','PART','RECUT')),
 entity_id uuid NOT NULL,
 actor_subject varchar(128) NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(tenant_id,proof_id)
);

-- Direct-SQL insertions still cannot claim unsigned-lineage material or parts.
CREATE FUNCTION guard_cut_consumption_v19() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE actual_mm bigint; allocated_mm bigint; r cut_sewing_releases_v8%ROWTYPE;
BEGIN
 SELECT * INTO r FROM cut_sewing_releases_v8 WHERE tenant_id=NEW.tenant_id AND id=NEW.release_id;
 IF NOT FOUND OR r.plant_id<>NEW.plant_id OR r.spread_report_id<>NEW.spread_report_id THEN
  RAISE EXCEPTION 'ERR_F32_RELEASE_SCOPE'; END IF;
 SELECT reported_consumption_mm INTO actual_mm FROM cut_spread_reports_v8
  WHERE tenant_id=NEW.tenant_id AND id=NEW.spread_report_id AND lay_draft_id=r.lay_draft_id;
 SELECT COALESCE(sum(allocated_mm),0) INTO allocated_mm FROM cut_spread_issue_allocations_v8
  WHERE tenant_id=NEW.tenant_id AND spread_report_id=NEW.spread_report_id;
 IF actual_mm IS NULL OR allocated_mm<>actual_mm OR
  NEW.used_part_mm>actual_mm OR NEW.trim_waste_mm>actual_mm-NEW.used_part_mm OR
  NEW.damaged_mm<>actual_mm-NEW.used_part_mm-NEW.trim_waste_mm THEN
   RAISE EXCEPTION 'ERR_F32_MATERIAL_BALANCE'; END IF;
 IF NOT EXISTS(SELECT 1 FROM cut_spread_issue_allocations_v8 a
  JOIN material_issues_v5 i ON i.tenant_id=a.tenant_id AND i.id=a.issue_id
  JOIN store_rolls sr ON sr.tenant_id=i.tenant_id AND sr.id=i.roll_id
  WHERE a.tenant_id=NEW.tenant_id AND a.spread_report_id=NEW.spread_report_id AND sr.plant_id=NEW.plant_id) THEN
   RAISE EXCEPTION 'ERR_F32_NO_SIGNED_ISSUE'; END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER cut_consumption_guard_v19 BEFORE INSERT ON cut_consumption_reconciliations_v19 FOR EACH ROW EXECUTE FUNCTION guard_cut_consumption_v19();
CREATE FUNCTION guard_cut_part_outcome_v19() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE actual_plant text; actual_release uuid;
BEGIN
 SELECT g.plant_id,g.release_id INTO actual_plant,actual_release FROM gen_cut_parts_v18 p
  JOIN gen_garments_v18 g ON g.tenant_id=p.tenant_id AND g.id=p.garment_id
  WHERE p.tenant_id=NEW.tenant_id AND p.id=NEW.part_id;
 IF actual_plant IS NULL OR actual_plant<>NEW.plant_id OR actual_release<>NEW.release_id THEN
  RAISE EXCEPTION 'ERR_F32_PART_SCOPE'; END IF;
 IF NOT EXISTS(SELECT 1 FROM cut_consumption_reconciliations_v19 c WHERE c.tenant_id=NEW.tenant_id
  AND c.release_id=NEW.release_id AND c.plant_id=NEW.plant_id) THEN
  RAISE EXCEPTION 'ERR_F32_CONSUMPTION_NOT_RECONCILED'; END IF;
 -- Serialize late reject against recut-link creation for this replacement.
 PERFORM pg_advisory_xact_lock(hashtext(NEW.tenant_id),hashtext('F32:PART:'||NEW.part_id::text));
 IF NEW.disposition='REJECTED' AND EXISTS (SELECT 1 FROM cut_recut_links_v19 l
  WHERE l.tenant_id=NEW.tenant_id AND l.replacement_part_id=NEW.part_id) THEN
  RAISE EXCEPTION 'ERR_F32_LINKED_RECUT_CANNOT_BE_REJECTED'; END IF;
 IF NEW.disposition='REJECTED' AND NOT EXISTS (
  SELECT 1 FROM cut_defect_codes_v19 d WHERE d.tenant_id=NEW.tenant_id AND d.plant_id=NEW.plant_id
  AND d.code=NEW.defect_code AND d.enabled) THEN RAISE EXCEPTION 'ERR_F32_DEFECT_NOT_APPROVED'; END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER cut_part_outcome_guard_v19 BEFORE INSERT ON cut_part_outcomes_v19 FOR EACH ROW EXECUTE FUNCTION guard_cut_part_outcome_v19();
CREATE FUNCTION guard_cut_recut_v19() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE oldpart record; newpart record;
BEGIN
 PERFORM pg_advisory_xact_lock(hashtext(NEW.tenant_id),hashtext('F32:PART:'||NEW.replacement_part_id::text));
 SELECT p.part_code,p.part_slot,g.release_id,g.plant_id,g.lay_draft_id,d.intake_id,d.fabric_sku,d.shade_band,d.source_order_sha256,
  pos.size_code,pos.color_code INTO oldpart
 FROM gen_cut_parts_v18 p JOIN gen_garments_v18 g ON g.tenant_id=p.tenant_id AND g.id=p.garment_id
 JOIN cut_lay_drafts_v8 d ON d.tenant_id=g.tenant_id AND d.id=g.lay_draft_id
 JOIN cut_lay_positions_v8 pos ON pos.tenant_id=g.tenant_id AND pos.lay_draft_id=g.lay_draft_id AND pos.marker_position=g.marker_position
 WHERE p.tenant_id=NEW.tenant_id AND p.id=NEW.original_part_id;
 IF NOT FOUND THEN RAISE EXCEPTION 'ERR_F32_ORIGINAL_NOT_FOUND'; END IF;
 SELECT p.part_code,p.part_slot,g.release_id,g.plant_id,g.lay_draft_id,d.intake_id,d.fabric_sku,d.shade_band,d.source_order_sha256,
  pos.size_code,pos.color_code INTO newpart
 FROM gen_cut_parts_v18 p JOIN gen_garments_v18 g ON g.tenant_id=p.tenant_id AND g.id=p.garment_id
 JOIN cut_lay_drafts_v8 d ON d.tenant_id=g.tenant_id AND d.id=g.lay_draft_id
 JOIN cut_lay_positions_v8 pos ON pos.tenant_id=g.tenant_id AND pos.lay_draft_id=g.lay_draft_id AND pos.marker_position=g.marker_position
 WHERE p.tenant_id=NEW.tenant_id AND p.id=NEW.replacement_part_id;
 IF NOT FOUND OR oldpart.plant_id<>NEW.plant_id OR newpart.plant_id<>NEW.plant_id OR
  oldpart.release_id<>NEW.release_id OR oldpart.lay_draft_id=newpart.lay_draft_id OR oldpart.intake_id<>newpart.intake_id OR
  oldpart.part_code<>newpart.part_code OR oldpart.part_slot<>newpart.part_slot OR
  oldpart.size_code<>newpart.size_code OR oldpart.color_code<>newpart.color_code OR
  oldpart.fabric_sku<>newpart.fabric_sku OR oldpart.shade_band<>newpart.shade_band OR
  oldpart.source_order_sha256<>newpart.source_order_sha256 THEN
   RAISE EXCEPTION 'ERR_F32_RECUT_LINEAGE'; END IF;
 IF NOT EXISTS(SELECT 1 FROM cut_part_outcomes_v19 d WHERE d.tenant_id=NEW.tenant_id
  AND d.part_id=NEW.original_part_id AND d.disposition='REJECTED') THEN
   RAISE EXCEPTION 'ERR_F32_ORIGINAL_NOT_REJECTED'; END IF;
 IF EXISTS(SELECT 1 FROM cut_part_outcomes_v19 d WHERE d.tenant_id=NEW.tenant_id
  AND d.part_id=NEW.replacement_part_id AND d.disposition='REJECTED') THEN
   RAISE EXCEPTION 'ERR_F32_REPLACEMENT_REJECTED'; END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER cut_recut_guard_v19 BEFORE INSERT ON cut_recut_links_v19 FOR EACH ROW EXECUTE FUNCTION guard_cut_recut_v19();

DO $$ DECLARE tbl text; BEGIN
 FOREACH tbl IN ARRAY ARRAY['cut_consumption_reconciliations_v19','cut_defect_codes_v19','cut_part_outcomes_v19','cut_recut_links_v19','cut_proof_consumptions_v19'] LOOP
  EXECUTE format('ALTER TABLE public.%I ENABLE ROW LEVEL SECURITY',tbl);
  EXECUTE format('ALTER TABLE public.%I FORCE ROW LEVEL SECURITY',tbl);
  EXECUTE format('CREATE POLICY cut_tenant_v19 ON public.%I FOR ALL TO agiparel_app USING(tenant_id=NULLIF(current_setting(''app.current_tenant_id'',true),'''')) WITH CHECK(tenant_id=NULLIF(current_setting(''app.current_tenant_id'',true),''''))',tbl);
  EXECUTE format('REVOKE ALL ON public.%I FROM PUBLIC,agiparel_app',tbl);
  EXECUTE format('GRANT SELECT ON public.%I TO agiparel_app',tbl);
 END LOOP;
END $$;
GRANT INSERT ON cut_consumption_reconciliations_v19,cut_part_outcomes_v19,cut_recut_links_v19,cut_proof_consumptions_v19 TO agiparel_app;
DO $$ DECLARE tbl text; BEGIN
 FOREACH tbl IN ARRAY ARRAY['cut_consumption_reconciliations_v19','cut_part_outcomes_v19','cut_recut_links_v19','cut_proof_consumptions_v19'] LOOP
  EXECUTE format('CREATE TRIGGER cut_immutable_v19 BEFORE UPDATE OR DELETE ON public.%I FOR EACH ROW EXECUTE FUNCTION protect_cut_draft_v8()',tbl);
 END LOOP;
END $$;
COMMIT;
