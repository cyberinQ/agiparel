# Agiparel Gate 06.5-B4.1 — Evidence and ingestion integrity

**Release**: B4.1 (hardening bridge between versioned factory reference catalog and advanced business-document understanding)
**Dependency**: Gate B4 and database migration 0013.
**Factory-first doctrine**: Accept incomplete source material, preserve the actual evidence, but never infer operationally authoritative numbers or mark incomplete records ready.

## P0 defects closed in this release

1. **Legacy web ingestion retired:** `/api/v1/onboarding/upload` (CSV direct database provisioning) and POST `/api/v1/planning/upload-docs` no longer write factory tables or start the legacy import process. Both return HTTP 410 with the central `/documents` alternative. Document Intelligence and B2/B3/B4 review APIs are unchanged.
2. **Legacy plan release retired:** `/api/v1/planning/approve` returns HTTP 409 without changing records, even if an operator provides a manager PIN. The cumulative-workbook readiness API is no longer presented as production evidence (`/api/v1/planning/assessment` returns 410). Order Intake and Material Planning remain proposed/HOLD as designed.
3. **Direct workbook writer disabled:** `onboarding.ApplyToDatabase` always returns `ERR_LEGACY_IMPORT_RETIRED`. Retaining its type signature is a compatibility aid, not a bypass.
4. **Compatibility CLI capture-only:** `scripts/ingest_docs.go` rejects `-capture-first=false` and `-apply-db=false`, pins tenant to a trusted flag/environment value instead of uploaded content, and persists original bytes plus extracted proposed facts in one database transaction per source. It does not merge into a cumulative workbook or touch machine/workers/rolls/workorders production tables. The separate `/documents` worker remains the preferred intake.
5. **No invented numeric values:** Malformed or missing machine UPH, worker rate, roll length/width, operation SAM or PO quantity cannot become a valid typed row in the deterministic compatibility parser. Instead it emits `validation_issues` with row, optional sheet ordinal, raw field and suggested resolution. It never generates 100 UPH, 14 stations, assumed 40% dwell, a 14.85-minute style SAM or a fit classification from a PO. A confirmed `0` worker rate remains distinguishable from a missing rate because a missing rate does not yield a WorkerRow.
6. **Malformed source protection:** Damaged CSV or XLSX no longer falls back to an AI reinterpretation that might conceal XML/tokenization errors. Well-formed but *unrecognized* layouts may still be proposed to the LLM for later review.
7. **Durable compatibility manifest:** Corrupted manifests are not silently treated as empty. Writes use a temp file, sync, atomic rename and directory sync; an interrupted after-database capture can be safely retried, because source capture is content-identified and idempotent.
8. **Legacy UI reconciliation:** Historic Planning page explicitly indicates that its calculator is not production release. Navigation points to Order Intake, Material Planning and Factory Setup; Plant Overview directs unconfigured users toward progressive factory setup rather than the old sheet upload.

## Operational semantics

- A document can be partially understood without being rejected by the factory onboarding journey. All source originals in the current document library remain preserved; the independent worker continues marking text/layout extraction statuses. These compatibility-parser `validation_issues` are persisted in the **legacy capture JSON** where that CLI is used. They are not yet an integrated, unified visual issue dashboard across all input types.
- Only verified, source-backed and explicitly published factory references can progress beyond the B4 review boundary. **This release does not enable** physical stores stock, machine availability or factory production release.
- There is **no migration** and no destructive database reset. Existing data is **not retroactively corrected**. Previously imported fake/default values must be independently audited before factory reliance.
- The independent Document Intelligence worker is not replaced. Real multi-format OCR, layout understanding, canonical buyer PO/BOM parsing and per-field extraction-coverage reporting remain the next gate.

## Acceptance contracts

- POST legacy factory and Planning upload endpoints return `410`; old plan approval returns `409`; no DB dependency is needed for these responses.
- An invalid machine capacity `garbage` yields a field/row issue, **not** 100 UPH.
- An invalid worker rate yields an issue, **not** an employee at zero hourly wage.
- Unknown roll dimensions do not produce an available stock record.
- A PO may have a known quantity and target, yet must not create a factory line with assumed station count or an invented SAM value.
- The seven historical staging input fixtures continue to parse, with no regression to valid quantities or conflict detection.
- `go test -count=1 ./...`, `go vet ./...` and `go build ./...` pass on the Mac with Go 1.24.

## The road to the twelve batches

- **B4.1 (this release):** harden the evidence boundary and retire all legacy auto-write paths.
- **B5 / 06.5-C:** real messy factory documents to canonical, versioned and reviewed factory masters; PDF/OCR/XLS support and extraction coverage; stage buyer POs, BOMs and technical packs from originals, not CSV conversion.
- **07:** physical stores ledger, rolls, buyer ownership, reservations, authenticated issuance.
- **08–11:** RFID cutting, sewing, quality/finishing and shipment conservation.
- **12:** physical validation, end-to-end conservation and source-grounded AI question accuracy.

Never treat a green parser test as validation of a physical factory workflow.
