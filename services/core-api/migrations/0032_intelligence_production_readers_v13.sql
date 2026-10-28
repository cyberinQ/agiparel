-- Author: Sunil Jain, 2026-10-28.
-- Chapter 09: provisioner-controlled, tenant/plant-scoped production reporting access.
-- No production figures or simulated data are backfilled.
BEGIN;
CREATE TABLE intelligence_prod_readers_v13 (
 tenant_id varchar(64) NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
 plant_id varchar(64) NOT NULL,
 worker_subject varchar(128) NOT NULL,
 enabled boolean NOT NULL DEFAULT false,
 approved_by varchar(128) NOT NULL,
 approved_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(tenant_id,plant_id,worker_subject)
);
ALTER TABLE intelligence_prod_readers_v13 ENABLE ROW LEVEL SECURITY;
ALTER TABLE intelligence_prod_readers_v13 FORCE ROW LEVEL SECURITY;
CREATE POLICY intelligence_prod_reader_tenant_v13 ON intelligence_prod_readers_v13
 FOR ALL TO agiparel_app
 USING (tenant_id=NULLIF(current_setting('app.current_tenant_id',true),''))
 WITH CHECK (tenant_id=NULLIF(current_setting('app.current_tenant_id',true),''));
REVOKE ALL ON intelligence_prod_readers_v13 FROM PUBLIC,agiparel_app;
GRANT SELECT ON intelligence_prod_readers_v13 TO agiparel_app;
-- Inserts, changes and revocation require a separate privileged provisioning role.
COMMIT;
