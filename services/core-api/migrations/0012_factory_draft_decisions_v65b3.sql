-- Gate 06.5-B3: append-only, tenant-isolated resolutions of competing factory evidence.
-- This stores *onboarding decisions only*. No production machine, payroll,
-- materials ledger, factory approval or costing master is modified.
BEGIN;
CREATE TABLE IF NOT EXISTS factory_draft_decisions_v65b3 (
  tenant_id varchar(64) NOT NULL,
  entity_type varchar(16) NOT NULL CHECK (entity_type IN ('MACHINE','SKILL','MATERIAL','RATE')),
  entity_key varchar(256) NOT NULL,
  revision integer NOT NULL CHECK (revision > 0),
  candidate_id varchar(35) NOT NULL,
  decision varchar(24) NOT NULL CHECK (decision IN ('VERIFY_DRAFT','NEEDS_CLARIFICATION')),
  review_note text NOT NULL CHECK (char_length(review_note) <= 1000),
  reviewed_by varchar(64) NOT NULL,
  reviewed_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (tenant_id,entity_type,entity_key,revision),
  FOREIGN KEY (tenant_id,candidate_id)
    REFERENCES factory_proposed_register_v65b2(tenant_id,candidate_id) ON DELETE RESTRICT
);
CREATE INDEX IF NOT EXISTS factory_draft_decisions_latest_v65b3
  ON factory_draft_decisions_v65b3(tenant_id,entity_type,entity_key,revision DESC);
ALTER TABLE factory_draft_decisions_v65b3 ENABLE ROW LEVEL SECURITY;
ALTER TABLE factory_draft_decisions_v65b3 FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS factory_draft_tenant_v65b3 ON factory_draft_decisions_v65b3;
CREATE POLICY factory_draft_tenant_v65b3 ON factory_draft_decisions_v65b3
  FOR ALL TO agiparel_app
  USING (tenant_id=NULLIF(current_setting('app.current_tenant_id',true),''))
  WITH CHECK (tenant_id=NULLIF(current_setting('app.current_tenant_id',true),''));
REVOKE ALL ON factory_draft_decisions_v65b3 FROM PUBLIC;
GRANT SELECT,INSERT ON factory_draft_decisions_v65b3 TO agiparel_app;
REVOKE UPDATE,DELETE,TRUNCATE ON factory_draft_decisions_v65b3 FROM agiparel_app;
COMMIT;
