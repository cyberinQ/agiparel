-- Batch 07A: observed fabric receipt/inspection passports, NOT authoritative stock.
-- These tables deliberately cannot write store_rolls, reservations, material issues or the CEO registry.
-- Requires 0001, 0003, 0005, 0009. Run as agiparel_admin.
BEGIN;
CREATE TABLE IF NOT EXISTS store_roll_rfid_bindings_v7 (
 tenant_id varchar(64) NOT NULL,
 epc_hex varchar(64) NOT NULL CHECK (epc_hex ~ '^[0-9A-F]{24,64}$' AND length(epc_hex) % 2 = 0),
 roll_id varchar(64) NOT NULL CHECK (roll_id ~ '^[A-Za-z0-9][A-Za-z0-9_.-]{0,63}$'),
 bound_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(tenant_id,epc_hex), UNIQUE(tenant_id,roll_id),
 UNIQUE(tenant_id,roll_id,epc_hex)
);
CREATE TABLE IF NOT EXISTS store_roll_passport_events_v7 (
 tenant_id varchar(64) NOT NULL,
 roll_id varchar(64) NOT NULL CHECK(roll_id ~ '^[A-Za-z0-9][A-Za-z0-9_.-]{0,63}$'),
 revision integer NOT NULL CHECK(revision > 0),
 event_kind varchar(12) NOT NULL CHECK(event_kind IN ('RECEIPT','REINSPECT','HOLD')),
 epc_hex varchar(64),
 material_sku varchar(128) NOT NULL,
 supplier_lot varchar(128) NOT NULL,
 shade_band varchar(32) NOT NULL,
 bin_location varchar(64) NOT NULL,
 declared_length_mm bigint NOT NULL CHECK(declared_length_mm > 0),
 measured_length_mm bigint NOT NULL CHECK(measured_length_mm > 0),
 usable_width_mm integer NOT NULL CHECK(usable_width_mm BETWEEN 100 AND 4000),
 inspection_result varchar(12) NOT NULL CHECK(inspection_result IN ('PASS','HOLD','REJECTED')),
 defects text NOT NULL DEFAULT '',
 source_document_id varchar(36),
 source_note text NOT NULL,
 recorded_by varchar(64) NOT NULL,
 recorded_at timestamptz NOT NULL DEFAULT now(),
 evidence_class varchar(32) NOT NULL DEFAULT 'OPERATOR_REPORTED' CHECK(evidence_class='OPERATOR_REPORTED'),
 PRIMARY KEY(tenant_id,roll_id,revision),
 FOREIGN KEY(tenant_id,roll_id,epc_hex) REFERENCES store_roll_rfid_bindings_v7(tenant_id,roll_id,epc_hex),
 FOREIGN KEY(tenant_id,source_document_id) REFERENCES document_sources_v65(tenant_id,id),
 CHECK ((event_kind='HOLD' AND inspection_result='HOLD') OR event_kind<>'HOLD'),
 CHECK (length(trim(source_note)) BETWEEN 8 AND 1000),
 CHECK (length(defects)<=2000)
);
CREATE INDEX IF NOT EXISTS store_roll_passport_latest_v7 ON store_roll_passport_events_v7(tenant_id,roll_id,revision DESC);
DO $$ DECLARE tbl text; BEGIN
 FOREACH tbl IN ARRAY ARRAY['store_roll_rfid_bindings_v7','store_roll_passport_events_v7'] LOOP
  EXECUTE format('ALTER TABLE %I ENABLE ROW LEVEL SECURITY',tbl);
  EXECUTE format('ALTER TABLE %I FORCE ROW LEVEL SECURITY',tbl);
  EXECUTE format('DROP POLICY IF EXISTS store_tenant_v7 ON %I',tbl);
  EXECUTE format('CREATE POLICY store_tenant_v7 ON %I FOR ALL TO agiparel_app USING (tenant_id=NULLIF(current_setting(''app.current_tenant_id'',true),'''')) WITH CHECK (tenant_id=NULLIF(current_setting(''app.current_tenant_id'',true),''''))',tbl);
  EXECUTE format('REVOKE ALL ON %I FROM PUBLIC,agiparel_app',tbl);
  EXECUTE format('GRANT SELECT,INSERT ON %I TO agiparel_app',tbl);
  EXECUTE format('REVOKE UPDATE,DELETE,TRUNCATE ON %I FROM agiparel_app',tbl);
 END LOOP;
END $$;
COMMIT;
