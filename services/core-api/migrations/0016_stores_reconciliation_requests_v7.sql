-- Batch 07B. Immutable, tenant-scoped reconciliation REVIEW REQUESTS only.
-- No DML privilege on store_rolls, material_reservations_v5, material_issues_v5,
-- material authority leases, witness proofs, or physical ledger structures is added.
-- Requires migrations through 0015. Run only against Agiparel dev as admin.
BEGIN;
CREATE TABLE IF NOT EXISTS store_reconciliation_requests_v7 (
    id uuid NOT NULL DEFAULT uuid_generate_v4(),
    tenant_id varchar(64) NOT NULL REFERENCES tenants(id),
    roll_id varchar(64) NOT NULL,
    passport_revision integer NOT NULL CHECK(passport_revision > 0),
    request_key varchar(128) NOT NULL CHECK(length(request_key) BETWEEN 8 AND 128),
    note text NOT NULL CHECK(length(btrim(note)) BETWEEN 12 AND 1000),
    assessment jsonb NOT NULL CHECK(jsonb_typeof(assessment)='object'),
    assessment_hash char(64) NOT NULL CHECK(assessment_hash ~ '^[0-9a-f]{64}$'),
    state varchar(48) NOT NULL DEFAULT 'PENDING_INDEPENDENT_RECONCILIATION'
        CHECK(state='PENDING_INDEPENDENT_RECONCILIATION'),
    reported_by varchar(64) NOT NULL DEFAULT 'dev',
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id,id),
    UNIQUE (tenant_id,request_key),
    FOREIGN KEY(tenant_id,roll_id,passport_revision)
        REFERENCES store_roll_passport_events_v7(tenant_id,roll_id,revision)
);
CREATE INDEX IF NOT EXISTS store_reconciliation_roll_v7
    ON store_reconciliation_requests_v7(tenant_id,roll_id,created_at DESC);
ALTER TABLE store_reconciliation_requests_v7 ENABLE ROW LEVEL SECURITY;
ALTER TABLE store_reconciliation_requests_v7 FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS store_reconciliation_tenant_v7 ON store_reconciliation_requests_v7;
CREATE POLICY store_reconciliation_tenant_v7 ON store_reconciliation_requests_v7
    FOR ALL TO agiparel_app
    USING (tenant_id=NULLIF(current_setting('app.current_tenant_id',true),''))
    WITH CHECK (tenant_id=NULLIF(current_setting('app.current_tenant_id',true),''));
REVOKE ALL ON store_reconciliation_requests_v7 FROM PUBLIC,agiparel_app;
GRANT SELECT,INSERT ON store_reconciliation_requests_v7 TO agiparel_app;
REVOKE UPDATE,DELETE,TRUNCATE ON store_reconciliation_requests_v7 FROM agiparel_app;
COMMIT;
