-- ============================================================================
-- AGIPAREL OS: APPLICATION ROLE PROVISIONING (0003_app_role.sql)
-- ============================================================================
DO $$ BEGIN
    IF NOT EXISTS (SELECT FROM pg_catalog.pg_roles WHERE rolname = 'agiparel_app') THEN
        CREATE ROLE agiparel_app WITH LOGIN PASSWORD 'app_secure_runtime_pass';
    END IF;
END $$;

GRANT CONNECT ON DATABASE agiparel_dev TO agiparel_app;
GRANT USAGE ON SCHEMA public TO agiparel_app;
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO agiparel_app;
GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO agiparel_app;
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO agiparel_app;
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT USAGE, SELECT ON SEQUENCES TO agiparel_app;