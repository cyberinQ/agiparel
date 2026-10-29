-- Author: Sunil Jain, 2026-10-29.
-- Provision login roles without recording passwords in tracked SQL or migration history.
\set ON_ERROR_STOP on
SELECT format('CREATE ROLE %I LOGIN', 'agiparel_app')
 WHERE NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname='agiparel_app') \gexec
SELECT format('CREATE ROLE %I LOGIN', 'agiparel_doc_worker')
 WHERE NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname='agiparel_doc_worker') \gexec
SELECT format('ALTER ROLE %I PASSWORD %L', 'agiparel_app', :'app_password') \gexec
SELECT format('ALTER ROLE %I PASSWORD %L', 'agiparel_doc_worker', :'doc_password') \gexec
SELECT format('GRANT CONNECT ON DATABASE %I TO agiparel_app, agiparel_doc_worker', current_database()) \gexec
