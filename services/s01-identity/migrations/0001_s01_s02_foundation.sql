-- Author: Sunil Jain, CyberinQ LLC
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE SCHEMA IF NOT EXISTS s01_identity;
CREATE SCHEMA IF NOT EXISTS s02_device;

-- S01: Tenants & Physical Plants (VARCHAR(64) IDs for cross-domain compatibility with UUID defaults)
CREATE TABLE IF NOT EXISTS s01_identity.tenants (
    id VARCHAR(64) PRIMARY KEY DEFAULT gen_random_uuid()::text,
    name VARCHAR(255) NOT NULL,
    slug VARCHAR(64) UNIQUE NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS s01_identity.plants (
    id VARCHAR(64) PRIMARY KEY DEFAULT gen_random_uuid()::text,
    tenant_id VARCHAR(64) NOT NULL REFERENCES s01_identity.tenants(id) ON DELETE CASCADE,
    code VARCHAR(64) NOT NULL,
    name VARCHAR(255) NOT NULL,
    timezone VARCHAR(64) NOT NULL DEFAULT 'UTC',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_plant_tenant_code UNIQUE (tenant_id, code)
);

CREATE TABLE IF NOT EXISTS s01_identity.users (
    id VARCHAR(64) PRIMARY KEY DEFAULT gen_random_uuid()::text,
    tenant_id VARCHAR(64) NOT NULL REFERENCES s01_identity.tenants(id) ON DELETE CASCADE,
    email VARCHAR(255) NOT NULL,
    full_name VARCHAR(255) NOT NULL,
    pin_hash VARCHAR(256),
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_user_email UNIQUE (tenant_id, email)
);

CREATE TABLE IF NOT EXISTS s01_identity.roles (
    id VARCHAR(64) PRIMARY KEY DEFAULT gen_random_uuid()::text,
    tenant_id VARCHAR(64) NOT NULL REFERENCES s01_identity.tenants(id) ON DELETE CASCADE,
    name VARCHAR(64) NOT NULL,
    description TEXT,
    CONSTRAINT uq_role_tenant_name UNIQUE (tenant_id, name)
);

CREATE TABLE IF NOT EXISTS s01_identity.role_permissions (
    role_id VARCHAR(64) NOT NULL REFERENCES s01_identity.roles(id) ON DELETE CASCADE,
    permission VARCHAR(64) NOT NULL,
    PRIMARY KEY (role_id, permission)
);

CREATE TABLE IF NOT EXISTS s01_identity.user_role_grants (
    id VARCHAR(64) PRIMARY KEY DEFAULT gen_random_uuid()::text,
    tenant_id VARCHAR(64) NOT NULL REFERENCES s01_identity.tenants(id) ON DELETE CASCADE,
    user_id VARCHAR(64) NOT NULL REFERENCES s01_identity.users(id) ON DELETE CASCADE,
    role_id VARCHAR(64) NOT NULL REFERENCES s01_identity.roles(id) ON DELETE CASCADE,
    plant_id VARCHAR(64) REFERENCES s01_identity.plants(id) ON DELETE CASCADE,
    granted_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_user_grant UNIQUE (user_id, role_id, plant_id)
);

-- S02: Hardware Device Trust & Attestation Registry
CREATE TABLE IF NOT EXISTS s02_device.registered_devices (
    id VARCHAR(64) PRIMARY KEY DEFAULT gen_random_uuid()::text,
    tenant_id VARCHAR(64) NOT NULL REFERENCES s01_identity.tenants(id) ON DELETE CASCADE,
    plant_id VARCHAR(64) NOT NULL REFERENCES s01_identity.plants(id) ON DELETE CASCADE,
    device_uid VARCHAR(128) NOT NULL,
    device_type VARCHAR(64) NOT NULL CHECK (device_type IN ('PHONE_HMI', 'TABLET_KIOSK', 'RFID_SLED', 'BT_RING_SCANNER', 'DIGITAL_SCALE', 'EDGE_SERVER')),
    cert_fingerprint VARCHAR(128) NOT NULL,
    public_key_pem TEXT NOT NULL,
    station_id VARCHAR(64),
    status VARCHAR(32) NOT NULL DEFAULT 'ENROLLED' CHECK (status IN ('ENROLLED', 'ACTIVE', 'REVOKED', 'OFFLINE')),
    last_ping_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_device_uid UNIQUE (tenant_id, device_uid)
);

ALTER TABLE s01_identity.plants ENABLE ROW LEVEL SECURITY;
ALTER TABLE s01_identity.users ENABLE ROW LEVEL SECURITY;
ALTER TABLE s01_identity.roles ENABLE ROW LEVEL SECURITY;
ALTER TABLE s01_identity.user_role_grants ENABLE ROW LEVEL SECURITY;
ALTER TABLE s02_device.registered_devices ENABLE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS tenant_isolation_plants ON s01_identity.plants;
CREATE POLICY tenant_isolation_plants ON s01_identity.plants
    FOR ALL USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), ''));

DROP POLICY IF EXISTS tenant_isolation_users ON s01_identity.users;
CREATE POLICY tenant_isolation_users ON s01_identity.users
    FOR ALL USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), ''));

DROP POLICY IF EXISTS tenant_isolation_roles ON s01_identity.roles;
CREATE POLICY tenant_isolation_roles ON s01_identity.roles
    FOR ALL USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), ''));

DROP POLICY IF EXISTS tenant_isolation_grants ON s01_identity.user_role_grants;
CREATE POLICY tenant_isolation_grants ON s01_identity.user_role_grants
    FOR ALL USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), ''));

DROP POLICY IF EXISTS tenant_isolation_devices ON s02_device.registered_devices;
CREATE POLICY tenant_isolation_devices ON s02_device.registered_devices
    FOR ALL USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), ''));