-- Author: Sunil Jain, 2026-10-28.
-- F1.1: provisioner-owned workforce grants; never trust token tenant/role claims.
-- Additive migration; no implicit grants or access backfill.
BEGIN;
CREATE TABLE workforce_identity_grants_v15 (
 tenant_id varchar(64) NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
 plant_id varchar(64) NOT NULL,
 idp_issuer varchar(512) NOT NULL CHECK(idp_issuer LIKE 'https://%'),
 idp_subject varchar(128) NOT NULL CHECK(length(trim(idp_subject))>0),
 permission varchar(80) NOT NULL CHECK(permission ~ '^[a-z][a-z0-9_.-]{2,79}$'),
 enabled boolean NOT NULL DEFAULT false,
 approved_by varchar(128) NOT NULL CHECK(length(trim(approved_by))>0),
 approved_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(tenant_id,plant_id,idp_issuer,idp_subject,permission)
);
CREATE INDEX workforce_grant_subject_v15 ON workforce_identity_grants_v15(idp_issuer,idp_subject,tenant_id,plant_id) WHERE enabled;
ALTER TABLE workforce_identity_grants_v15 ENABLE ROW LEVEL SECURITY;
ALTER TABLE workforce_identity_grants_v15 FORCE ROW LEVEL SECURITY;
CREATE POLICY workforce_grant_tenant_v15 ON workforce_identity_grants_v15
 FOR SELECT TO agiparel_app
 USING (tenant_id=NULLIF(current_setting('app.current_tenant_id',true),''));
REVOKE ALL ON workforce_identity_grants_v15 FROM PUBLIC,agiparel_app;
GRANT SELECT ON workforce_identity_grants_v15 TO agiparel_app;
-- Changes require a separately held privileged provisioning connection.
COMMIT;
