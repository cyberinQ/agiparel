# F3.1 — Permanent Garment Identity and RFID Carrier Lifecycle

Author: Sunil Jain, 2026-10-28

## Scope and provenance

This increment adds an append-only, tenant-scoped **identity reservation**, not a physical garment production claim. A signed Chapter 05 release is a precondition; each reservation identifies precisely one marker position and one ply. Each part reference belongs to an existing cut bundle whose lay/part/ply range matches that unit. A deferred database constraint requires the full part-slot set atomically, so a partial reservation fails its transaction.

A cut-part reference is a reservation for a cut piece, not proof that the cutting machine produced or accepted it. Current Chapter 05 cut bundles and sewing completions remain group-based; no garment-level QC, finishing, inventory or shipment credit is created by these APIs. Individual-piece physical admission and QC disposition are F3.2/F3.4 gates.

## Trusted production APIs

All routes are registered only when `mod_sewing` is enabled; they are **fail-closed** until all of `ENV=staging|production`, `AGIPAREL_SEWING_PRODUCTION_ENABLED=true`, `AGIPAREL_GENEALOGY_PRODUCTION_ENABLED=true`, DB and existing Ed25519 attestation IdP configuration are valid.

- `POST /api/v1/genealogy/units/challenge`: body `{"command":{"command_id":"PlyClaimABC123","plant_id":"plant-A","release_id":"<uuid>","lay_id":"<uuid>","marker_position":1,"ply_number":1}}`. Returns canonical signed challenge (`GENEALOGY_RESERVE_V1:<sha256>`).
- `POST /api/v1/genealogy/units`: same JSON + `Authorization: Bearer <transaction-bound signed IdP assertion>`. Requires action `cutting.release`, role `cutting_supervisor`, MFA and independent F1.3 plant permission. Returns permanent garment UUID, number of cut-part references and `IDENTITY_RESERVED_NOT_PHYSICALLY_VERIFIED`; explicitly no production credit. Same command+subject+payload replays idempotently; changing it returns conflict.
- `GET /api/v1/genealogy/units?id=<uuid>&limit=50&cursor=<uuid>`: requires an independently signed assertion for `cutting.read`, challenge `GENEALOGY_READ_V1`, role `cutting_supervisor`, plus provisioned `cutting.read`. Returns scoped unit ancestry and a bounded, keyset-paginated list of part identities (`has_more`, `next_cursor`), never silently clips a part set.
- `POST /api/v1/genealogy/carriers/challenge`: body `{"command":{"command_id":"CarrierClaimABC123","plant_id":"plant-A","carrier_id":"<uuid>","action":"BIND","target_type":"GARMENT","target_id":"<uuid>"}}`.
- `GET /api/v1/genealogy/carriers/history?carrier_id=<uuid>&limit=50&cursor=<event_no>`: independently signed `cutting.read` assertion, scoped to the caller’s plant, returning append-only BIND/RETIRE history with stable descending event-sequence pagination. Legacy bindings are never misreported as this new history.
- `POST /api/v1/genealogy/carriers/events`: same JSON, signed assertion (`cutting.release`, `cutting_supervisor`), provisioned permission. `BIND` creates an active assignment; `RETIRE` releases it without deleting history. Event reports `SIGNED_HUMAN_CARRIER_CLAIM_NOT_RF_PROOF`; neither a human assertion nor a bound EPC is proof of actual RF capture.

All requests must be sent over production TLS through the authenticated ingress. Browser session cookies alone cannot authorize these production writes.

## Carrier provisioning and the legacy boundary

`gen_carriers_v18` has no application INSERT grant. A privileged provisioning workflow must register each physical tag and enable it after verification; do not add self-enrollment to this API. Existing legacy roll and cutting EPCs are *not silently migrated into reusable inventory*. They are quarantined from new assignment until a separately audited transfer/reconciliation process proves that their prior binding was retired. Existing historic scans remain unchanged.

The `gen_carrier_events_v18` table is append-only. The active assignment table is a mutable **projection maintained solely by a SECURITY DEFINER database trigger** with a fixed search path. Per-carrier row locking prevents two concurrent BINDs from succeeding. A unique constraint additionally prevents two carriers from simultaneously identifying the same target. Admin-level or physical commissioning errors still require an independently audited correction process.

## Configuration and scalability

There are no hardcoded plant names, style capacities, number of garment parts, ply counts, or bundle sizes in F3.1. It uses database source parts and `generate_series(1,pieces_per_garment)`, bounded HTTP input and paginated output. The inherited Chapter 05 schema still restricts 210 plies / 80 marker positions / 8 part slots; removing those inherited physical limits is a separate, backward-compatible migration needed before broad production qualification.

## Acceptance gates (not claimed complete)

1. Apply migration 0037 on PostgreSQL 16 and run concurrent reserve/BIND/RETIRE, rollback and cross-tenant RLS integration tests.
2. Test real enrolled device/worker MFA assertions and plant-restricted permissions; verify no unauthorized SQL writes to carrier enrollment/active projections.
3. Validate physical source bundle and cut-part admission with a factory. **Neither identity reservation nor human tag binding is a physical-acceptance event.**
4. Test very large configurable lays/bundle compositions and validate query plans and index usage.
5. Verify that legacy EPC overlap is blocked and any future migration of retired tags requires explicit reconciliation.
6. Do not enable this feature in the current development Compose environment. Production API remains off unless explicitly configured in staged deployment.

F3.1 intentionally does not implement garment-level status promotion, accepted output quantities, payroll, packing or shipping. Those require independent measured evidence in subsequent increments.
