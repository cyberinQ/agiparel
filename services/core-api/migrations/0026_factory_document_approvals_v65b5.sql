-- Author: Sunil Jain, 2026-10-27
-- Append-only, tenant-isolated field decisions and atomic manager-approved reference snapshots.
-- These references NEVER authorize payroll, inventory, stock issuance, production or costing.
BEGIN;
CREATE TABLE IF NOT EXISTS factory_candidate_review_versions_v65b5 (
 tenant_id varchar(64) NOT NULL,
 candidate_id varchar(35) NOT NULL,
 revision integer NOT NULL CHECK (revision>0),
 action varchar(8) NOT NULL CHECK (action IN ('KEEP','SKIP')),
 attributes jsonb NOT NULL CHECK (jsonb_typeof(attributes)='object'),
 checked_fields jsonb NOT NULL CHECK (jsonb_typeof(checked_fields)='array'),
 reviewer varchar(64) NOT NULL,
 reviewed_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY (tenant_id,candidate_id,revision),
 FOREIGN KEY (tenant_id,candidate_id) REFERENCES factory_document_candidates_v65b2(tenant_id,candidate_id) ON DELETE RESTRICT
);
CREATE TABLE IF NOT EXISTS factory_document_submissions_v65b5 (
 tenant_id varchar(64) NOT NULL,
 document_id varchar(36) NOT NULL,
 submission_id varchar(35) NOT NULL,
 approved_count integer NOT NULL CHECK(approved_count>=0),
 skipped_count integer NOT NULL CHECK(skipped_count>=0),
 reviewer varchar(64) NOT NULL,
 approved_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(tenant_id,document_id),
 UNIQUE(tenant_id,submission_id),
 FOREIGN KEY(tenant_id,document_id) REFERENCES document_sources_v65(tenant_id,id) ON DELETE RESTRICT
);
CREATE TABLE IF NOT EXISTS factory_approved_reference_versions_v65b5 (
 tenant_id varchar(64) NOT NULL,
 entity_type varchar(16) NOT NULL CHECK (entity_type IN ('MACHINE','SKILL','MATERIAL','RATE','FIELD')),
 entity_key varchar(256) NOT NULL,
 revision integer NOT NULL CHECK(revision>0),
 candidate_id varchar(35) NOT NULL,
 document_id varchar(36) NOT NULL,
 review_revision integer NOT NULL,
 attributes jsonb NOT NULL CHECK(jsonb_typeof(attributes)='object'),
 approved_by varchar(64) NOT NULL,
 approved_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(tenant_id,entity_type,entity_key,revision),
 UNIQUE(tenant_id,candidate_id),
 FOREIGN KEY(tenant_id,document_id) REFERENCES factory_document_submissions_v65b5(tenant_id,document_id) ON DELETE RESTRICT,
 FOREIGN KEY(tenant_id,candidate_id,review_revision)
  REFERENCES factory_candidate_review_versions_v65b5(tenant_id,candidate_id,revision) ON DELETE RESTRICT
);
CREATE INDEX IF NOT EXISTS factory_approved_by_document_v65b5 ON factory_approved_reference_versions_v65b5(tenant_id,document_id);
DO $$ DECLARE tbl text; BEGIN
 FOR tbl IN SELECT unnest(ARRAY['factory_candidate_review_versions_v65b5','factory_document_submissions_v65b5','factory_approved_reference_versions_v65b5']) LOOP
  EXECUTE format('ALTER TABLE %I ENABLE ROW LEVEL SECURITY',tbl);
  EXECUTE format('ALTER TABLE %I FORCE ROW LEVEL SECURITY',tbl);
  EXECUTE format('DROP POLICY IF EXISTS factory_tenant_v65b5 ON %I',tbl);
  EXECUTE format('CREATE POLICY factory_tenant_v65b5 ON %I FOR ALL TO agiparel_app USING (tenant_id=NULLIF(current_setting(''app.current_tenant_id'',true),'''')) WITH CHECK (tenant_id=NULLIF(current_setting(''app.current_tenant_id'',true),''''))',tbl);
  EXECUTE format('REVOKE ALL ON %I FROM PUBLIC',tbl);
  EXECUTE format('GRANT SELECT,INSERT ON %I TO agiparel_app',tbl);
  EXECUTE format('REVOKE UPDATE,DELETE,TRUNCATE ON %I FROM agiparel_app',tbl);
 END LOOP;
END $$;
COMMIT;
