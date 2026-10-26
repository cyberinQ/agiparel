-- Gate 06.5-B4: versioned, source-backed *reference* catalog for machines, skills and rates.
-- Neither this table nor its API updates operational machines, staffing permissions,
-- payroll, stores stock, material issuance, purchase orders or approved costing.
-- Requires migrations 0009, 0011, 0012. Apply as agiparel_admin.
BEGIN;
CREATE TABLE IF NOT EXISTS factory_reference_versions_v65b4 (
 tenant_id varchar(64) NOT NULL,
 entity_type varchar(16) NOT NULL CHECK (entity_type IN ('MACHINE','SKILL','RATE')),
 entity_key varchar(256) NOT NULL CHECK (char_length(trim(entity_key)) BETWEEN 1 AND 256),
 revision integer NOT NULL CHECK (revision > 0),
 action varchar(12) NOT NULL CHECK (action IN ('PUBLISH','WITHDRAW')),
 candidate_id varchar(35) NOT NULL,
 draft_revision integer NOT NULL CHECK (draft_revision > 0),
 attributes jsonb NOT NULL CHECK (jsonb_typeof(attributes)='object'),
 rate_basis varchar(12) CHECK (rate_basis IS NULL OR rate_basis IN ('HOUR','PIECE','METER','YARD','KG','DAY','UNIT')),
 reviewer_note text NOT NULL CHECK (char_length(trim(reviewer_note)) BETWEEN 12 AND 1000),
 reviewed_by varchar(64) NOT NULL,
 recorded_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY (tenant_id,entity_type,entity_key,revision),
 FOREIGN KEY (tenant_id,candidate_id) REFERENCES factory_proposed_register_v65b2(tenant_id,candidate_id) ON DELETE RESTRICT,
 FOREIGN KEY (tenant_id,entity_type,entity_key,draft_revision)
   REFERENCES factory_draft_decisions_v65b3(tenant_id,entity_type,entity_key,revision) ON DELETE RESTRICT,
 CONSTRAINT rate_basis_required CHECK ((entity_type='RATE' AND rate_basis IS NOT NULL) OR (entity_type<>'RATE' AND rate_basis IS NULL))
);
CREATE INDEX IF NOT EXISTS factory_reference_latest_v65b4
 ON factory_reference_versions_v65b4(tenant_id,entity_type,entity_key,revision DESC);
ALTER TABLE factory_reference_versions_v65b4 ENABLE ROW LEVEL SECURITY;
ALTER TABLE factory_reference_versions_v65b4 FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS factory_reference_tenant_v65b4 ON factory_reference_versions_v65b4;
CREATE POLICY factory_reference_tenant_v65b4 ON factory_reference_versions_v65b4 FOR ALL TO agiparel_app
 USING (tenant_id=NULLIF(current_setting('app.current_tenant_id',true),''))
 WITH CHECK (tenant_id=NULLIF(current_setting('app.current_tenant_id',true),''));
REVOKE ALL ON factory_reference_versions_v65b4 FROM PUBLIC;
GRANT SELECT,INSERT ON factory_reference_versions_v65b4 TO agiparel_app;
REVOKE UPDATE,DELETE,TRUNCATE ON factory_reference_versions_v65b4 FROM agiparel_app;
COMMIT;
