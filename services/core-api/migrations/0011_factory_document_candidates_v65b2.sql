-- Gate 06.5-B2: source-anchored, non-authoritative factory candidate evidence.
-- Apply AFTER 0010 with the agiparel_admin role. This does not change any
-- operational master, payroll, cost ledger, inventory, reservation or PO table.
BEGIN;
CREATE TABLE IF NOT EXISTS factory_document_candidates_v65b2 (
 tenant_id varchar(64) NOT NULL,
 candidate_id varchar(35) NOT NULL CHECK(candidate_id ~ '^FC-[0-9a-f]{32}$'),
 document_id varchar(36) NOT NULL,
 source_ordinal integer NOT NULL,
 source_locator jsonb NOT NULL CHECK(jsonb_typeof(source_locator)='object'),
 entity_type varchar(16) NOT NULL CHECK(entity_type IN ('MACHINE','SKILL','MATERIAL','RATE','FIELD')),
 entity_key varchar(256) NOT NULL,
 field_key varchar(80) NOT NULL DEFAULT '',
 proposed_value text NOT NULL CHECK(char_length(proposed_value) BETWEEN 1 AND 1000),
 attributes jsonb NOT NULL CHECK(jsonb_typeof(attributes)='object'),
 explanation text NOT NULL CHECK(char_length(explanation) BETWEEN 1 AND 500),
 fingerprint char(64) NOT NULL,
 captured_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(tenant_id,candidate_id),
 UNIQUE(tenant_id,fingerprint),
 FOREIGN KEY(tenant_id,document_id,source_ordinal)
  REFERENCES document_segments_v65(tenant_id,document_id,ordinal) ON DELETE RESTRICT
);
CREATE INDEX IF NOT EXISTS factory_candidates_by_document_v65b2
 ON factory_document_candidates_v65b2(tenant_id,document_id,captured_at DESC);
CREATE TABLE IF NOT EXISTS factory_candidate_reviews_v65b2 (
 tenant_id varchar(64) NOT NULL,
 candidate_id varchar(35) NOT NULL,
 decision varchar(24) NOT NULL CHECK(decision IN ('ACCEPT_PROVISIONAL','REJECT')),
 review_note text NOT NULL DEFAULT '' CHECK(char_length(review_note)<=1000),
 reviewed_by varchar(64) NOT NULL,
 reviewed_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(tenant_id,candidate_id),
 FOREIGN KEY(tenant_id,candidate_id)
  REFERENCES factory_document_candidates_v65b2(tenant_id,candidate_id) ON DELETE RESTRICT
);
-- Structured register is an evidentiary observation, not a factory master.
-- Each accepted row retains exact source and proposal identity.
CREATE TABLE IF NOT EXISTS factory_proposed_register_v65b2 (
 tenant_id varchar(64) NOT NULL,
 candidate_id varchar(35) NOT NULL,
 entity_type varchar(16) NOT NULL CHECK(entity_type IN ('MACHINE','SKILL','MATERIAL','RATE')),
 entity_key varchar(256) NOT NULL,
 attributes jsonb NOT NULL,
 review_state varchar(16) NOT NULL DEFAULT 'PROVISIONAL' CHECK(review_state='PROVISIONAL'),
 recorded_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(tenant_id,candidate_id),
 FOREIGN KEY(tenant_id,candidate_id)
  REFERENCES factory_document_candidates_v65b2(tenant_id,candidate_id) ON DELETE RESTRICT
);
DO $$ DECLARE tbl text; BEGIN
 FOR tbl IN SELECT unnest(ARRAY['factory_document_candidates_v65b2','factory_candidate_reviews_v65b2','factory_proposed_register_v65b2']) LOOP
  EXECUTE format('ALTER TABLE %I ENABLE ROW LEVEL SECURITY',tbl);
  EXECUTE format('ALTER TABLE %I FORCE ROW LEVEL SECURITY',tbl);
  EXECUTE format('DROP POLICY IF EXISTS factory_candidates_tenant_v65b2 ON %I',tbl);
  EXECUTE format('CREATE POLICY factory_candidates_tenant_v65b2 ON %I FOR ALL TO agiparel_app USING(tenant_id=NULLIF(current_setting(''app.current_tenant_id'',true),'''')) WITH CHECK(tenant_id=NULLIF(current_setting(''app.current_tenant_id'',true),''''))',tbl);
 END LOOP;
END $$;
REVOKE ALL ON factory_document_candidates_v65b2,factory_candidate_reviews_v65b2,factory_proposed_register_v65b2 FROM PUBLIC;
GRANT SELECT,INSERT ON factory_document_candidates_v65b2,factory_candidate_reviews_v65b2,factory_proposed_register_v65b2 TO agiparel_app;
REVOKE UPDATE,DELETE,TRUNCATE ON factory_document_candidates_v65b2,factory_candidate_reviews_v65b2,factory_proposed_register_v65b2 FROM agiparel_app;
-- Document worker cannot approve extracted content or write factory entities.
COMMIT;
