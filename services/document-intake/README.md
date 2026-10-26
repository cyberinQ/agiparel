# Agiparel shared document-intake service — Gate 06.5-A

This Go service is an **internal, no-listener worker**. Every factory module can consume the same tenant-pinned PostgreSQL source records. The operator-authenticated edge gateway accepts multipart `files` uploads and queues one PostgreSQL job per byte-identical original. A separate restricted worker claims with `SKIP LOCKED`, extracts bounded, source-anchored text, and persists page/row/paragraph segments. No operational order, inventory or approval table is modified.

- `POST /api/v1/documents/uploads`: 1–10 files, max 20 MiB each, 60 MiB per request; responds 202 with saved original identifiers and per-file errors.
- `GET /api/v1/documents`: operator-tenant library with processing status.
- `GET /api/v1/documents/{id}`: up to 200 stored segments with source locators.
- `GET /api/v1/documents/{id}/source`: original byte-for-byte attachment.
- `/documents`: restricted human inspection UI; `/` is public and shows link without revealing real customer records.
- Worker formats: PDF embedded text, XLSX sheet and cell references, DOCX paragraphs, CSV/TSV rows, UTF-8 text. Scans/photos, legacy XLS, Google API links and unknown formats return explicit partial/unsupported states with original bytes preserved. PDF conversion requires `pdftotext` from `poppler-utils` in worker image.
- All assertions/approvals remain UNREVIEWED. No automatic promotion to Batch 05 orders, BOMs or Batch 06 material scenarios until Gate 06.5-B/C.

DB migration `0009_document_intelligence_v65.sql` grants document-only access to worker role; operator API's `agiparel_app` can INSERT/SELECT source and queue but not mutate extracted facts. Development role/password is static and must be replaced prior to any production use. `AGIPAREL_CAPTURE_FIRST=true` is forced for the older Planning upload path.
