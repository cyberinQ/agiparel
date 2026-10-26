-- Agiparel Batch 06: immutable, PROPOSED-only sourcing scenarios, never inventory authority.
-- Requires 0007; runtime may INSERT/SELECT, never UPDATE/DELETE.
BEGIN;
CREATE TABLE IF NOT EXISTS material_scenarios_v6 (
 id varchar(40) PRIMARY KEY,
 tenant_id varchar(64) NOT NULL,
 intake_id varchar(40) NOT NULL,
 source_sha256 char(64) NOT NULL,
 scenario_sha256 char(64) NOT NULL,
 stock_filename varchar(255) NOT NULL,
 prices_filename varchar(255) NOT NULL,
 stock_original bytea NOT NULL,
 prices_original bytea NOT NULL,
 report_json jsonb NOT NULL,
 status varchar(20) NOT NULL DEFAULT 'PROPOSED' CHECK(status='PROPOSED'),
 release_gate varchar(10) NOT NULL DEFAULT 'HOLD' CHECK(release_gate='HOLD'),
 created_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE(tenant_id,id),
 UNIQUE(tenant_id,intake_id,scenario_sha256),
 FOREIGN KEY(tenant_id,intake_id) REFERENCES order_intakes_v5(tenant_id,id) ON DELETE RESTRICT
);
CREATE INDEX IF NOT EXISTS material_scenarios_recent_v6 ON material_scenarios_v6(tenant_id,intake_id,created_at DESC);
ALTER TABLE material_scenarios_v6 ENABLE ROW LEVEL SECURITY;
ALTER TABLE material_scenarios_v6 FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation_v6 ON material_scenarios_v6;
CREATE POLICY tenant_isolation_v6 ON material_scenarios_v6
 USING(tenant_id=NULLIF(current_setting('app.current_tenant_id',true),''))
 WITH CHECK(tenant_id=NULLIF(current_setting('app.current_tenant_id',true),''));
GRANT SELECT,INSERT ON material_scenarios_v6 TO agiparel_app;
REVOKE UPDATE, DELETE, TRUNCATE ON material_scenarios_v6 FROM agiparel_app;
COMMIT;
