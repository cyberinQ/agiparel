# Agiparel v5.3 — Factory-first implementation and release plan

**Normative user journey:** First discover and progressively model the garment factory (machines, capabilities, people/skills, raw materials/inventory, shifts, quality, costs/rates); then ingest factory buyers' POs, technical packs, drawings, schedules and shipping requirements; finally reconcile buyer demands against what the factory can truly produce. Buyer files may be captured at any time, even before factory onboarding completes, but they cannot silently authorize operations.

**Empathetic UX doctrine:** One upload inbox; one small next question; estimate now, verify later; per-fact provenance, revisions, gaps and conflicts; never treat unknown as zero; let noncritical missing details coexist with useful exploration. Gate ONLY the action that requires the information. Chat and model-derived text are proposals until reviewed. Preserve original documents. A person should not need to know “BOM”, a database schema, or an upload category merely to begin.

## Delivery snapshot

- Batches 01–06 and Gate 06.5-A/A.1: previously implemented foundation, user validates on Mac; do not infer live hardware acceptance.
- **Gate 06.5-B1 (this package):** restricted `/factory-setup` screen, per-fact `PROVISIONAL|CONFIRMED` updates, revision/audit history under tenant-scoped PostgreSQL, basic document relevance hints and per-activity discovery readiness. These are **not** operational machine/employee/inventory master records.
- **Gate 06.5-B2 (next):** unified source-to-candidate field intelligence for real machine inventories, skill rosters (privacy rules), raw materials, costs and rates. Page/cell citations; scanned/legacy XLS/cloud connectors; guided correction and conflicts.
- **Gate 06.5-C:** authorized promotion to normalized approved factory master data and buyer entities, PO approval integration with Order Intake/Material Planning, deterministic feasibility checks, semantic AI answers and tenant unification.
- **Batch 07–12:** real stores/roll custody; cutting genealogy; sewing OB/SAM; quality; packing and shipments; cross-stage conservation and verified pilot KPIs.

## Gate 06.5-B1 acceptance checklist

1. `/factory-setup` is accessible through protected navigation; homepage highlights it **before** the buyer order flow.
2. Brand-new tenant can begin with just a factory name; missing skills or rates do not block uploading documents.
3. A machine-count estimate is stored as provisional; the readiness calculator does not count it as confirmed.
4. Revising a fact appends revision `n+1` and retains prior revision; a stale write yields HTTP 409.
5. Submitted values are type-validated (nonnegative machine/workforce counts, reasonable shift hours, nonnegative exact decimal strings, valid currency code).
6. Uploaded documents are suggested as machine / skills / stores / buyer files only; classification is clearly tentative and never auto-merges.
7. Confirming opening-stock status cannot authorize issuance; RFID and independently witnessed material approvals remain separate.
8. Tenant pin comes from `AGIPAREL_OPERATOR_TENANT=dev` in development; no user-supplied tenant or cross-tenant reads.
9. Migration 0010 uses forced RLS and grants the edge app SELECT/INSERT only; document worker cannot promote facts.
10. Go unit/HTTP tests, optional real PostgreSQL integration, Go build/vet, and manual website smoke testing pass on the user's Mac.

**Explicit limitations:** No NLP agent dialogue in B1; next-question logic is deterministic. No OCR of image PDFs, no reliable classification of arbitrary file structures, no canonical machine/worker/inventory/cost normalization and no buyer-order auto-materialization in this release. Opening-stock confirmation here is *human observation*, not a ledger balance. The public demo and weak `dev`/`1212` barrier remain unsuitable for sensitive employee data or production approval.


## Gate 06.5-B4 implemented increment — reference catalog (2026-10-26)

After B3 human-reviewed conflict decisions, factories can publish source-backed **reference-only** machine identifiers/types, worker-skill observations and explicitly unit-qualified rates into an append-only per-tenant catalog (migration `0013`). Withdrawal is a new version, never an overwrite. Material opening-stock candidates remain excluded until Batch 07 physical stores verification and ledger authority. No employee authorizations, machine capabilities, approved costing, production release or buyer order data are changed by B4. UI: `/factory-setup#factory-references`. Next priority is real-document, source-linked canonical factory identities and approved buyer-document handoff (06.5-C) before operational stores.
