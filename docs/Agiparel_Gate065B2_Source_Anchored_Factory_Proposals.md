# Gate 06.5-B2 — Factory data from documents, without silent authority

## Product intent

Factory-first onboarding now connects the **one document inbox** to a reviewable set of candidate machine, worker-skill, opening-stock, rate, and basic factory-profile records. It must work for incomplete data and prefer omission over guesswork. It builds on the v5.3 factory-first doctrine; it does not supersede the master PRD/ARD.

## Trust boundaries

1. Uploaded originals and extracted segments are immutable and tenant scoped (migration 0009).
2. Scan is a bounded, **deterministic heuristic** over explicitly labelled fields and tabular source headers. No generative AI, OCR, XLS legacy conversion or automatic category classification is claimed.
3. Each proposed record preserves `document_id`, `source_ordinal`, `source_locator`, typed attributes, immutable fingerprint and suggestion rationale.
4. The user can review and **Keep as provisional** or **Not relevant**. Accepted structured rows are recorded only in `factory_proposed_register_v65b2`. Accepted simple factory facts produce a **PROVISIONAL** revision in `factory_fact_events_v65b`, unless an already confirmed fact would be overwritten (rejected with 409). They never become CONFIRMED automatically.
5. The proposal tables prohibit UPDATE/DELETE for the app role, enforce per-tenant PostgreSQL RLS and do not grant the extraction worker any write permissions.
6. No material reservation, stock valuation, machine-master creation, employee payroll entry, costing approval, production release, purchasing or issuance results from reviewing proposals.
7. The page shows users the exact source location and links to the evidence inspector; unresolved facts can be entered manually at any point. Original files remain in Document Intelligence.

## Scope and limitations of B2

- Supports **ROW** (CSV/TSV) and **SHEET_ROW** (XLSX) headers for machine ID and machine type, worker ID and skill, material code and stock quantity/unit (only when source context explicitly suggests opening stock or inventory), and rate category, numeric rate and currency. Names, wages per person and other private employee information are intentionally not auto-imported on this development site.
- Supports explicitly labelled scalar values (e.g. `Sewing machines: 20`) in extracted text, PDFs and simple CSV lines. Invalid/out-of-range values and spreadsheet formulas are ignored; formula cached values are not silently treated as approved data.
- PDF/Word/complex workbook tables are **not automatically reconstructed** into row records in this release. Legacy XLS, scanned OCR, multi-row merged headers, cross-document joins, capacity and cost normalization, conflict resolution and canonical master promotion remain for later increments.
- Each source scan examines at most 1,200 extracted segments and creates at most 150 candidate suggestions. A re-scan is idempotent by fingerprint, not by file name. The current UI lists the 300 newest candidates and 200 newest proposed register records; it is not yet a full enterprise-scale register browser.
- One-time review decisions are immutable in this increment. To refine confirmed onboarding values use Factory Setup; to revise rejected interpretations, upload a corrected source or use a subsequent review workflow.

## Next implementation gate

B3: improve semantic classification and field-aware review across real PDF/Office layouts, handle mixed sheets and legacy XLS/OCR, construct candidate structured master entities with conflict handling, and make approved factory master data progressively available to planning. Never silently promote an AI guess to an authoritative store or workforce record. B/C buyer document understanding remains after the factory baseline.
