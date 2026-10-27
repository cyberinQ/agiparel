-- Batch 07C: immutable dual-attested roll admission records.
-- No new material issuance powers; approved receipt is mediated by signed API.
-- Requires ordered migrations 0001..0016. Apply with agiparel_admin.
BEGIN;
CREATE TABLE IF NOT EXISTS store_roll_admissions_v7 (
 tenant_id varchar(64) NOT NULL REFERENCES tenants(id),
 id uuid NOT NULL DEFAULT uuid_generate_v4(),
 request_id uuid NOT NULL,
 roll_id varchar(64) NOT NULL,
 passport_revision integer NOT NULL CHECK (passport_revision > 0),
 plant_id varchar(64) NOT NULL,
 assessment_hash char(64) NOT NULL CHECK (assessment_hash ~ '^[0-9a-f]{64}$'),
 signed_challenge_hash char(64) NOT NULL CHECK (signed_challenge_hash ~ '^[0-9a-f]{64}$'),
 admitted_mm bigint NOT NULL CHECK(admitted_mm > 0),
 manager_subject varchar(128) NOT NULL,
 inspector_subject varchar(128) NOT NULL,
 manager_proof_id varchar(255) NOT NULL,
 inspector_proof_id varchar(255) NOT NULL,
 manager_assertion_hash char(64) NOT NULL,
 inspector_assertion_hash char(64) NOT NULL,
 admitted_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(tenant_id,id),
 UNIQUE(tenant_id,request_id),
 UNIQUE(tenant_id,roll_id),
 CHECK(manager_subject <> inspector_subject),
 CHECK(manager_proof_id <> inspector_proof_id),
 FOREIGN KEY(tenant_id,request_id) REFERENCES store_reconciliation_requests_v7(tenant_id,id),
 FOREIGN KEY(tenant_id,roll_id,passport_revision) REFERENCES store_roll_passport_events_v7(tenant_id,roll_id,revision),
 FOREIGN KEY(tenant_id,roll_id) REFERENCES store_rolls(tenant_id,id)
);
CREATE TABLE IF NOT EXISTS store_roll_admission_proofs_v7 (
 tenant_id varchar(64) NOT NULL REFERENCES tenants(id),
 proof_id varchar(255) NOT NULL,
 admission_id uuid NOT NULL,
 witness_role varchar(20) NOT NULL CHECK(witness_role IN ('STORES_MANAGER','STORES_INSPECTOR')),
 recorded_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(tenant_id,proof_id),
 FOREIGN KEY(tenant_id,admission_id) REFERENCES store_roll_admissions_v7(tenant_id,id)
);
CREATE INDEX IF NOT EXISTS store_roll_admission_recorded_v7 ON store_roll_admissions_v7(tenant_id,admitted_at DESC);
DO $$ DECLARE tbl text; BEGIN
 FOREACH tbl IN ARRAY ARRAY['store_roll_admissions_v7','store_roll_admission_proofs_v7'] LOOP
  EXECUTE format('ALTER TABLE %I ENABLE ROW LEVEL SECURITY',tbl);
  EXECUTE format('ALTER TABLE %I FORCE ROW LEVEL SECURITY',tbl);
  EXECUTE format('DROP POLICY IF EXISTS stores_admission_tenant_v7 ON %I',tbl);
  EXECUTE format('CREATE POLICY stores_admission_tenant_v7 ON %I FOR ALL TO agiparel_app USING (tenant_id=NULLIF(current_setting(''app.current_tenant_id'',true),'''')) WITH CHECK (tenant_id=NULLIF(current_setting(''app.current_tenant_id'',true),''''))',tbl);
  EXECUTE format('REVOKE ALL ON %I FROM PUBLIC,agiparel_app',tbl);
  EXECUTE format('GRANT SELECT,INSERT ON %I TO agiparel_app',tbl);
  EXECUTE format('REVOKE UPDATE,DELETE,TRUNCATE ON %I FROM agiparel_app',tbl);
 END LOOP;
END $$;
-- Audit entries are append-only even if a database role was granted UPDATE later.
CREATE OR REPLACE FUNCTION protect_stores_admission_v7() RETURNS TRIGGER LANGUAGE plpgsql AS $$
BEGIN RAISE EXCEPTION 'ERR_STORES_ADMISSION_IMMUTABLE'; END $$;
DROP TRIGGER IF EXISTS store_roll_admission_immutable_v7 ON store_roll_admissions_v7;
CREATE TRIGGER store_roll_admission_immutable_v7 BEFORE UPDATE OR DELETE ON store_roll_admissions_v7
 FOR EACH ROW EXECUTE FUNCTION protect_stores_admission_v7();
DROP TRIGGER IF EXISTS store_roll_admission_proof_immutable_v7 ON store_roll_admission_proofs_v7;
CREATE TRIGGER store_roll_admission_proof_immutable_v7 BEFORE UPDATE OR DELETE ON store_roll_admission_proofs_v7
 FOR EACH ROW EXECUTE FUNCTION protect_stores_admission_v7();
-- Preserve physical roll order ownership: no cross-order borrowing, even
-- when separately requested lengths would otherwise fit on the same roll.
-- Applies to future reservation writes; audit old reservations separately.
CREATE OR REPLACE FUNCTION enforce_roll_single_order_v7() RETURNS TRIGGER LANGUAGE plpgsql AS $$
BEGIN
 PERFORM 1 FROM store_rolls WHERE tenant_id=NEW.tenant_id AND id=NEW.roll_id FOR UPDATE;
 IF NOT FOUND THEN RAISE EXCEPTION 'ERR_ROLL_NOT_FOUND'; END IF;
 IF EXISTS(SELECT 1 FROM material_reservations_v5
       WHERE tenant_id=NEW.tenant_id AND roll_id=NEW.roll_id AND order_id<>NEW.order_id)
    OR EXISTS(SELECT 1 FROM material_issues_v5
       WHERE tenant_id=NEW.tenant_id AND roll_id=NEW.roll_id AND order_id<>NEW.order_id) THEN
  RAISE EXCEPTION 'ERR_SACROSANCT_ROLL_ORDER';
 END IF;
 RETURN NEW;
END $$;
DROP TRIGGER IF EXISTS material_single_order_v7 ON material_reservations_v5;
CREATE TRIGGER material_single_order_v7 BEFORE INSERT OR UPDATE OF roll_id,order_id
 ON material_reservations_v5 FOR EACH ROW EXECUTE FUNCTION enforce_roll_single_order_v7();
COMMIT;
