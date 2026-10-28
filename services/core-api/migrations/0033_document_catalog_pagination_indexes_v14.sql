-- Author: Sunil Jain, 2026-10-28.
-- Chapter 10: tenant-local evidence catalog paging and revision lineage indexes.
-- Deliberately no BEGIN: CONCURRENTLY avoids blocking writes on large plants.
-- Run with psql -v ON_ERROR_STOP=1; rerunning is safe.
CREATE INDEX CONCURRENTLY IF NOT EXISTS document_sources_v65_tenant_time_keyset_v14
 ON document_sources_v65 (tenant_id, captured_at DESC, id DESC);
CREATE INDEX CONCURRENTLY IF NOT EXISTS document_sources_v65_tenant_filename_revision_v14
 ON document_sources_v65 (tenant_id, lower(filename), captured_at, id);
