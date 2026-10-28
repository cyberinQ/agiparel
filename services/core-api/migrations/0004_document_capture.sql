-- Author: Sunil Jain, CyberinQ LLC
-- Additive, opt-in intake journal. No existing columns or production routes are changed.
BEGIN;

CREATE TABLE IF NOT EXISTS intake_document_capture (
    id varchar(64) PRIMARY KEY,
    tenant_id varchar(64) NOT NULL,
    filename text NOT NULL,
    content_sha256 char(64) NOT NULL,
    source_mode text NOT NULL,
    original_bytes bytea NOT NULL,
    extraction_json jsonb NOT NULL,
    status text NOT NULL DEFAULT 'PROPOSED' CHECK (status IN ('PROPOSED','SYNCED')),
    captured_at timestamptz NOT NULL DEFAULT now(),
    synced_at timestamptz
);

CREATE INDEX IF NOT EXISTS idx_intake_capture_tenant ON intake_document_capture(tenant_id,captured_at DESC);

CREATE TABLE IF NOT EXISTS intake_proposed_fact (
    capture_id varchar(64) NOT NULL REFERENCES intake_document_capture(id) ON DELETE RESTRICT,
    tenant_id varchar(64) NOT NULL,
    json_pointer text NOT NULL,
    value_json jsonb NOT NULL,
    PRIMARY KEY(capture_id,json_pointer)
);

CREATE INDEX IF NOT EXISTS idx_intake_fact_tenant ON intake_proposed_fact(tenant_id,capture_id);

ALTER TABLE intake_document_capture ENABLE ROW LEVEL SECURITY;
ALTER TABLE intake_document_capture FORCE ROW LEVEL SECURITY;
ALTER TABLE intake_proposed_fact ENABLE ROW LEVEL SECURITY;
ALTER TABLE intake_proposed_fact FORCE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS intake_capture_tenant_policy ON intake_document_capture;
CREATE POLICY intake_capture_tenant_policy ON intake_document_capture
    FOR ALL TO agiparel_app
    USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), ''))
    WITH CHECK (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), ''));

DROP POLICY IF EXISTS intake_fact_tenant_policy ON intake_proposed_fact;
CREATE POLICY intake_fact_tenant_policy ON intake_proposed_fact
    FOR ALL TO agiparel_app
    USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), ''))
    WITH CHECK (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), ''));

GRANT SELECT,INSERT,UPDATE ON intake_document_capture TO agiparel_app;
GRANT SELECT,INSERT ON intake_proposed_fact TO agiparel_app;

COMMIT;