-- Gate 06.5-B1: progressive factory discovery, human-confirmed, tenant-pinned facts.
-- Run as agiparel_admin after 0009.  No operational inventory/employee/machine
-- records are created or changed by this table.  This is an append-only evidence log.
BEGIN;
CREATE TABLE IF NOT EXISTS factory_fact_events_v65b (
  tenant_id varchar(64) NOT NULL,
  field_key varchar(80) NOT NULL,
  revision integer NOT NULL CHECK (revision BETWEEN 1 AND 2147483647),
  value_kind varchar(16) NOT NULL CHECK (value_kind IN ('TEXT','INTEGER','DECIMAL','CURRENCY','CHOICE')),
  value_text text NOT NULL CHECK (char_length(value_text) BETWEEN 1 AND 2000),
  normalized_value jsonb NOT NULL CHECK (jsonb_typeof(normalized_value) = 'object'),
  review_state varchar(16) NOT NULL CHECK (review_state IN ('PROVISIONAL','CONFIRMED')),
  source_document_id varchar(36),
  review_note text NOT NULL DEFAULT '' CHECK (char_length(review_note) <= 1000),
  recorded_by varchar(64) NOT NULL,
  recorded_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY(tenant_id,field_key,revision),
  FOREIGN KEY(tenant_id,source_document_id)
    REFERENCES document_sources_v65(tenant_id,id) ON DELETE RESTRICT
);
CREATE INDEX IF NOT EXISTS factory_fact_events_latest_v65b
  ON factory_fact_events_v65b (tenant_id,field_key,revision DESC);
ALTER TABLE factory_fact_events_v65b ENABLE ROW LEVEL SECURITY;
ALTER TABLE factory_fact_events_v65b FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS factory_fact_tenant_v65b ON factory_fact_events_v65b;
CREATE POLICY factory_fact_tenant_v65b ON factory_fact_events_v65b FOR ALL TO agiparel_app
 USING(tenant_id=NULLIF(current_setting('app.current_tenant_id',true),''))
 WITH CHECK(tenant_id=NULLIF(current_setting('app.current_tenant_id',true),''));
REVOKE ALL ON factory_fact_events_v65b FROM PUBLIC;
GRANT SELECT,INSERT ON factory_fact_events_v65b TO agiparel_app;
REVOKE UPDATE,DELETE,TRUNCATE ON factory_fact_events_v65b FROM agiparel_app;
-- Worker cannot write human confirmations.  No grants to agiparel_doc_worker.
COMMIT;
