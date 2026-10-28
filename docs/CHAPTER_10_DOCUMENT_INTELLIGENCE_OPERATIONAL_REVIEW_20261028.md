<!-- Author: Sunil Jain, 2026-10-28. Production-focused Chapter 10 operational assessment. -->
# Chapter 10 — Document Intelligence and Evidence Library

## Operating verdict
The original-source inbox is a useful shared foundation for factory onboarding, buyer orders, rolls and audits. Original bytes and SHA-256 lineage are immutable; inferred data remain provisional until separately reviewed. However, Chapter 10 is **not a production-ready document vault**. The browser's fixed 100-document and 200-segment windows silently hid older records and made scale-dependent evidence review unreliable. Its fixed development tenant is intentionally denied outside development and must not be reenabled by bypassing authorization.

## Completed in this increment
1. Version-aware document catalog now returns **bounded keyset pages** (`limit`, `cursor`, `has_more`, `next_cursor`); versions are calculated over the entire tenant before paging. Default remains 100 items for older consumers; Chapter 10 intentionally requests 50 at a time. Invalid cursors and oversized pages fail with 400. The visual total is explicitly a **loaded** count, not a factory-wide total. Sorting is explicitly local to loaded files.
2. Evidence inspection loads source segments in bounded ordinal pages (`after_ordinal`, `next_ordinal`, `has_more_segments`). Full originals remain downloadable. Searching is limited to loaded text and the UI says so.
3. Delayed responses cannot overwrite another selected document's evidence. The review dropdown receives newly loaded catalog pages without destroying in-progress record edits.
4. Existing UI palette, file table, disclosure panels, typography, routes and navigation remain unchanged. Only two quiet **Load more** controls and accurate scope labels were added.
5. In production, the document workspace itself now fails closed until a real workforce identity and tenant/plant authorization path exists; its existing APIs were already development-only.
6. Migration 0033 creates tenant-keyset and per-filename revision indexes concurrently, without changing document bytes or prior approvals.

## Required next production gates (NOT implemented)
- **P0 identity and authorization** — per-request SSO user, tenant/plant/document assignment and audit, independent of a fixed environment tenant; authenticated source downloads, privacy and retention policies.
- **P0 secure extraction** — maintained AV engine, isolated worker, resource/time budgets, queue backpressure, malicious-document testing, malware quarantine, optional object-store encryption and access review.
- **P0 transactional evidence governance** — document access logs, explicit classifications and confidentiality labels, true review/approval role separation and immutable audit of original downloads.
- **P1 cross-chapter discovery** — indexed search across *all* source segments, direct ordinal deep links, full-library filename search and sorting, not client-only filtering.
- **P1 true enterprise scale** — load-test windowed revision queries at large tenant sizes, choose materialized/current-version records if query latency becomes material, and validate partitioning/archival separately from operational metadata.
- **P1 business value** — buyer document amendment matching, fabric-shortage claims with source evidence, contract/version diffs, and exception queues for unreadable, unreviewed or superseded inputs.

## Explicit non-authority
Uploaded documents, OCR output, provisional parsed facts, and buyer source classification **never** authorize payroll, material issue, cutting release, production completion, carton packing or shipment. This increment does not change those controls.

## Production release classification
**Pilot-only / development-deployed.** No end-to-end workforce IdP or real plant authorization. Run full Go 1.24 tests, PostgreSQL migration on staging, tenant-isolation and concurrency tests, vulnerability scans, backup/restore and verified device/worker identity before production admission.

## Verification acceptance
- 18 Node scripts (including Chapter 10 bounded-page runtime regression), isolated Go cursor tests/vet with local toolchain, UI contract and change-overlay checks.
- On iMac: full `go test -count=1 ./...`, `go test -race -count=1 ./...`, `go vet ./...`, `go build ./...`, `node scripts/test_*.cjs`.
- Migration 0033 uses `CREATE INDEX CONCURRENTLY`: execute as independent statements (never inside `BEGIN`), check both indexes exist, then deploy the unchanged-for-production API in the existing development profile.
