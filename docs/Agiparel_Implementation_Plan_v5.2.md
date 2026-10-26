# Agiparel — Executable v5.2 Plan (decision log + next deliverables)

**Status:** Plan updated, **not** new source code. **Date:** 2026-10-26. Use with both v5.2 master docs.

## Decisions confirmed by customer

1. **Dev convention**: `AGIPAREL_OPERATOR_USER=dev`, `AGIPAREL_OPERATOR_PASSWORD=1212`, `AGIPAREL_OPERATOR_TENANT=dev`. No `DEV-TENANT` or `dev-operator` in future instructions. Tenant ID is a routing/isolation key, not a second username/password. Development PIN does not authorize production business actions.
2. **Universal document UX**: customer supplies PDFs, XLS/XLSX, Word, CSV, Google Docs/Sheets, images, and other formats **as received**. They should not first convert to CSV. A single Document Intake service is shared by all factory modules.
3. **Internal source of truth**: immutable originals + normalized typed assertion JSON and canonical PostgreSQL domain entities. Optional CSV export for integrations; **not** a lossy intermediate requiring all documents to be tabular.
4. **No silent facts**: source cell/page/region provenance, format support classification, conflicts and human approval remain mandatory. Staging/capture is not production authorization.
5. **Next increment is Gate 06.5**, before releasing verified stores custody in Batch 07. Ship website improvements in each gate, not just backend.

## Current real-world problem and remedy

- Batch 06's material-planning dropdown shows only Batch 05 staged records. The user should not need to stage a reformatted PO/BOM CSV; approving an extraction should create a canonical buyer order that populates the selector automatically.
- Existing Planning import and Batch 05 structured CSV intake are separate paths. Fold both into the shared service and block auto-merges until reviewed.
- Tenant mismatch between `TENANT_DEFAULT`, `DEV-TENANT`, and `dev` must be resolved explicitly in queries, migrations, and tests; do not unilaterally rewrite historical rows without a reviewed migration.
- The real-file corpus exposes multi-PO PDFs, rich tech-pack BOMs/packing, XLSX/XLS operation bulletins, size/color matrices and stock/gross/net naming ambiguities. Extraction success must be measured at *fact and relationship* granularity, not 'file converted'.

## Next implementation: Gate 06.5-A, first demonstrable release

**Customer UI**: Add "Documents" to left navigation and `/documents` or `/order-intake/documents` (choose one canonical route). Multi-select upload, drag/drop, file cards, progress states, source preview/download, per-document extracted facts with anchors, and *review required* alerts.

**Backend**: One `document-intake` service, durable ingestion job queue in PostgreSQL, immutable source storage, pdf text + OCR as needed, spreadsheet sheet/cell extractor, DOCX tables/sections, CSV fast path, MIME/signature validation, extraction adapters, typed assertion v1. Preserve worker isolation, cost/time limits and tenant pins. Go edge gateway authenticates and routes; service-to-service requests cannot mint authority by inventing headers. No single-microservice-per-format proliferation.

**Migrations**: source artifacts, versions, jobs, renditions, segments, assertions, review decisions; tenant-safe composite keys. Existing `intake_document_capture` and `order_intakes_v5` remain readable during migration.

**Acceptance**: 14 customer files uploaded with explicit per-file supported/partial/error outcomes, byte-identical sources and SHA, durable restart, 4 Walmart POs extracted *or* flagged for review rather than collapsed, no auto-merge, no cross-tenant answer, no production-side writes, useful website screen. Later 06.5-B/C gates raise semantic coverage and connected Google docs.

**API direction**: `/api/v1/documents/uploads`, `/api/v1/documents/jobs/{id}`, `/api/v1/documents/{id}/assertions`, `/api/v1/documents/reviews`, typed approved entity reads. Define OpenAPI and status codes before coding.

## Subsequent delivery sequence

- **06.5-B:** universal adapters and cloud exports, page/cell citations, rich PO/BOM/OB/packing normalization, review/diff/version controls.
- **06.5-C:** canonical PO approval populates Order Intake and Material Planning; semantic AI query API; >=40 gold-source questions and no-answer checks.
- **07:** real stores rolls, inventory commitments, reservations, witnessed issuance/CEO rules.
- **08:** cutting and tagged physical bundle genealogy.
- **09:** OB/SAM and RFID sewing station events.
- **10:** quality/finishing/hangtags.
- **11:** bag/carton/manifest/dispatch and OCR conservation.
- **12:** measured end-to-end times, data/accounting reconciliation, grounded AI QA and witnessed plant acceptance.

## What the next update should NOT do

- Do not force the customer to clean/convert every file to CSV.
- Do not claim "all formats understood" just because the endpoint accepts their bytes.
- Do not allow LLM text or successful SQL alone to become approved facts.
- Do not flatten two distinct POs sharing the same style into one order.
- Do not silently assume every 2-piece bag is an individual garment RFID.
- Do not invent formula outputs where Excel cached values are absent.
- Do not rewrite other Docker stacks or perform global Docker cleanup.

## Plan artifacts in this ZIP

- `docs/Agiparel_Master_PRD_ARD_v5.2.md` — the new normative addendum plus full retained v5.1 PRD/ARD.
- `docs/Agiparel_Master_Epics_Stories_Tasks_v5.2.md` — prioritized DGI-01..DGI-12 and 06.5-A/B/C work + complete v5.1 backlog.
- `docs/Agiparel_Implementation_Plan_v5.2.md` — short operational decision log and roadmap (this file).
