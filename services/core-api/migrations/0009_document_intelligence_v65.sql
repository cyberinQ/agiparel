-- Gate 06.5-A: source-preserving, tenant-scoped document intake.
-- Run ONCE as agiparel_admin after migration 0008; creates a narrowly privileged internal worker.
BEGIN;
DO $$ BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname='agiparel_doc_worker') THEN
    CREATE ROLE agiparel_doc_worker LOGIN PASSWORD 'doc_worker_dev_only_change_in_production';
  END IF;
END $$;

CREATE TABLE IF NOT EXISTS document_sources_v65 (
  tenant_id varchar(64) NOT NULL,
  id varchar(36) NOT NULL,
  filename varchar(255) NOT NULL,
  media_type varchar(100) NOT NULL,
  format varchar(24) NOT NULL,
  byte_length integer NOT NULL CHECK (byte_length BETWEEN 1 AND 20971520),
  sha256 char(64) NOT NULL,
  original bytea NOT NULL,
  evidence_state varchar(20) NOT NULL DEFAULT 'UNREVIEWED' CHECK (evidence_state='UNREVIEWED'),
  captured_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (tenant_id,id),
  UNIQUE (tenant_id,sha256)
);
CREATE TABLE IF NOT EXISTS document_jobs_v65 (
  tenant_id varchar(64) NOT NULL,
  document_id varchar(36) NOT NULL,
  status varchar(24) NOT NULL DEFAULT 'QUEUED' CHECK (status IN ('QUEUED','PROCESSING','EXTRACTED','PARTIAL','UNSUPPORTED','ERROR')),
  attempts integer NOT NULL DEFAULT 0 CHECK (attempts BETWEEN 0 AND 3),
  outcome text NOT NULL DEFAULT '',
  segment_count integer NOT NULL DEFAULT 0 CHECK (segment_count >= 0),
  started_at timestamptz,
  finished_at timestamptz,
  updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (tenant_id,document_id),
  FOREIGN KEY (tenant_id,document_id) REFERENCES document_sources_v65(tenant_id,id) ON DELETE RESTRICT
);
CREATE INDEX IF NOT EXISTS document_jobs_v65_queue ON document_jobs_v65(status, updated_at,document_id);
CREATE TABLE IF NOT EXISTS document_segments_v65 (
  tenant_id varchar(64) NOT NULL,
  document_id varchar(36) NOT NULL,
  ordinal integer NOT NULL CHECK (ordinal BETWEEN 1 AND 10000),
  kind varchar(32) NOT NULL,
  locator jsonb NOT NULL,
  content text NOT NULL,
  extracted_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (tenant_id,document_id,ordinal),
  FOREIGN KEY (tenant_id,document_id) REFERENCES document_sources_v65(tenant_id,id) ON DELETE RESTRICT,
  CHECK(octet_length(content) <= 16000)
);
CREATE TABLE IF NOT EXISTS document_assertions_v65 (
  tenant_id varchar(64) NOT NULL,
  document_id varchar(36) NOT NULL,
  assertion_id varchar(36) NOT NULL,
  field_path text NOT NULL,
  value_json jsonb NOT NULL,
  source_ordinal integer NOT NULL,
  review_state varchar(20) NOT NULL DEFAULT 'UNREVIEWED' CHECK(review_state='UNREVIEWED'),
  PRIMARY KEY(tenant_id,document_id,assertion_id),
  FOREIGN KEY(tenant_id,document_id,source_ordinal) REFERENCES document_segments_v65(tenant_id,document_id,ordinal) ON DELETE RESTRICT
);

DO $$ DECLARE tbl text; BEGIN
 FOR tbl IN SELECT unnest(ARRAY['document_sources_v65','document_jobs_v65','document_segments_v65','document_assertions_v65']) LOOP
  EXECUTE format('ALTER TABLE %I ENABLE ROW LEVEL SECURITY',tbl);
  EXECUTE format('ALTER TABLE %I FORCE ROW LEVEL SECURITY',tbl);
  EXECUTE format('DROP POLICY IF EXISTS document_tenant_v65 ON %I',tbl);
  EXECUTE format('CREATE POLICY document_tenant_v65 ON %I FOR ALL TO agiparel_app USING(tenant_id=NULLIF(current_setting(''app.current_tenant_id'',true),'''')) WITH CHECK(tenant_id=NULLIF(current_setting(''app.current_tenant_id'',true),''''))',tbl);
 END LOOP;
END $$;
CREATE POLICY document_worker_sources_v65 ON document_sources_v65 FOR SELECT TO agiparel_doc_worker USING(true);
CREATE POLICY document_worker_jobs_v65 ON document_jobs_v65 FOR ALL TO agiparel_doc_worker USING(true) WITH CHECK(true);
CREATE POLICY document_worker_segments_v65 ON document_segments_v65 FOR ALL TO agiparel_doc_worker USING(true) WITH CHECK(true);

REVOKE ALL ON document_sources_v65,document_jobs_v65,document_segments_v65,document_assertions_v65 FROM PUBLIC;
GRANT SELECT,INSERT ON document_sources_v65,document_jobs_v65 TO agiparel_app;
GRANT SELECT ON document_segments_v65,document_assertions_v65 TO agiparel_app;
REVOKE UPDATE,DELETE,TRUNCATE ON document_sources_v65,document_jobs_v65,document_segments_v65,document_assertions_v65 FROM agiparel_app;
GRANT USAGE ON SCHEMA public TO agiparel_doc_worker;
GRANT SELECT ON document_sources_v65 TO agiparel_doc_worker;
GRANT SELECT ON document_jobs_v65 TO agiparel_doc_worker;
GRANT UPDATE(status,attempts,outcome,segment_count,started_at,finished_at,updated_at) ON document_jobs_v65 TO agiparel_doc_worker;
GRANT SELECT,INSERT ON document_segments_v65 TO agiparel_doc_worker;
-- Neither the edge process nor worker can change the immutable originals, promote facts or write factory tables.
COMMIT;
