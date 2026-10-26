# Gate 06.5-C1 — Buyer-source review to proposed Order Intake

## Product intent

This is the last narrowly scoped integration gate before physical Stores Batch 07. Agiparel's central inbox remains the only customer-facing general upload destination. After factory-first onboarding, the operator selects any extracted buyer PO and BOM/tech-pack source, confirms mappings, and stages one reviewed order at a time. Material Planning immediately sees this source-linked, PROPOSED/HOLD order via the existing Batch 05 read model.

## Implementation

- GET `/buyer-review` and GET `/api/v1/buyer-review?po_document_id=DOC-...&bom_document_id=DOC-...` read tenant-isolated source segments and suggest PO/BOM rows only when explicit schema cues are present. An explicit source-specific YSAG/Walmart sales-order page reader recognizes repeated REP groups only after size-column totals reconcile. It is **not** a universal PDF interpretation engine.
- Browser can correct recognized rows or manually transcribe missing fields while viewing up to 35 in-page excerpts and opening exact PDF page/spreadsheet row snippets in `/documents`.
- POST `/api/v1/buyer-review` validates explicit acknowledgement, source segment numbers, PO/style identity, positive quantities, unique size lines, material categories, units, quantity precision, and at least one fabric requirement. It reconstructs validated canonical CSV **internally** from user-reviewed typed facts; the customer never has to prepare CSV.
- A *single PostgreSQL transaction* inserts the existing `order_intakes_v5` proposal and its `buyer_order_handoffs_v65c1` source lineage. The originals remain in immutable `document_sources_v65`. Replays with identical input are idempotent.
- The handoff is source-linked **reviewed transcription**, not buyer PO approval, material authorization, machine capability validation or production release.

## Current format coverage

| Input | Behavior in this gate |
|---|---|
| Well-formed CSV and XLSX tables | Deterministic header-driven PO/BOM row suggestions |
| YSAG/Walmart sales order PDF supplied in project | Narrow, page-specific multi-PO size/quantity suggestions after printed-total check |
| Other PDFs, DOCX or irregular spreadsheets | Original source excerpts are shown for guided manual mapping; no fabricated structured facts |
| Image-only PDF, images, binary XLS | Await extraction/OCR conversion before review |
| Multi-style BOM | Each row carries optional style; selecting one style must not pull a different style into its proposed requirements |

## Evidence classes

- `source_ordinal > 0`: reviewer links a row to an existing extracted source segment in the selected document. Link existence is verified against the tenant's document segments.
- `source_ordinal = 0`: explicitly manual value. Reviewer's explanation and original document ID are preserved, but source line is not claimed.
- Source hashes, validated canonical row values, row reference numbers, note and reviewer login are captured as a versioned handoff record (immutable proposal; a changed mapping produces a new source digest).
- `order_intakes_v5.status=PROPOSED`, `readiness=HOLD`; subsequent material calculation remains hypothetical only.

## Acceptance evidence and limitations

The two provided Walmart PO PDF texts were inspected with `pdftotext -layout` and the new deterministic segment parser: flat-front REP3 = 57,600, REP4 = 72,000; pleated REP1 = 40,000, REP2 = 89,000. All seven size values in each matched the explicitly printed totals. No assertion is made that a buyer verified these, or that their corresponding tech-pack BOMs are fully parsed.

The new Go parser and validation tests and UI navigation test passed in isolated Go 1.23 harnesses, with Node JS syntax checked. PostgreSQL transaction, role/RLS and idempotency integration test was written as an *opt-in* Mac test; not run in the assistant's environment. Full Go 1.24 compilation and live browser testing remain Mac acceptance.

## Next releases (bounded roadmap)

- **06.5-C2 (only if needed for pilot documents):** extract/normalize complex, multi-sheet buyer BOM and versioned buyer approval requirements using source-linked human review. Otherwise proceed to Batch 07 once the factory can stage at least one real order safely.
- **07:** inspected roll passport, conservation ledger, eligible stock, independently witnessed issuance and CEO exceptions.
- **08:** physical cutting markers, actual ply ranges, classic sub-bundles and RFID genealogy.
- **09:** validated OB, operation-level SAM/skills and RFID sewing HMI state machine.
- **10:** quality, repair, finishing and garment hangtag identity.
- **11:** packing, polybags, cartons, shipment and order closure.
- **12:** cross-stage conservation, cycle/queue-time evidence, AI semantic accuracy and pilot acceptance on real readers.

Legacy uploads remain retired; factory-first progressive onboarding doctrine remains unchanged. Never promote source-extracted values to approved operational truth without explicit domain gates.
