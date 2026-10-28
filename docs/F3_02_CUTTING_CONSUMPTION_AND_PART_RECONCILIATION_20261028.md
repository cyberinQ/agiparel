# F3.2 — Cutting consumption, cut-part disposition and recut links
Author: Sunil Jain, 2026-10-28

**Status:** Staging-pilot evidence foundation. Not an approved warehouse, payroll, sewing-output or shipping source. Existing Chapters 01–10 UI remains unchanged.

## Data and trust boundaries

- `cut_spread_reports_v8` and `cut_spread_issue_allocations_v8` already record source-linked consumption in integer millimetres, attributed to signed `material_issues_v5`, and signed `cut_sewing_releases_v8`. F3.2 does **not** issue additional stock, revise source measurements, create new releases, or substitute a fabricated meter reading.
- Migration `0038` introduces a **once-per-spread** two-person reconciliation that classifies the existing recorded consumption into `used_part_mm + trim_waste_mm + damaged_mm = reported_consumption_mm`. These are **dual-signed human classifications of linear-equivalent consumed mm**, not optical/camera/scale-verified measurements or a reliable fabric-area yield. The table has source-release/plant and allocation guard triggers.
- A cut-part outcome (`ACCEPTED` or `REJECTED`) requires an F3.1 reserved `gen_cut_parts_v18` identity, a reconciled signed source spread and two distinct strongly authenticated subjects. Rejections require a previously provisioned, plant-scoped enabled `cut_defect_codes_v19` value. A cut part can receive only one terminal outcome. Corrections are **not** mutable; exception reversal or reinspection needs a separate future governed workflow.
- Recut lineage (`LINK_RECUT`) associates a rejected part with a **different released lay** and a newly reserved replacement cut-part identity. Both must agree on order intake, plant, size, color, part code and slot; the original is rejected. A replacement must receive its own separately signed `ACCEPTED` outcome. Links do not create inventory or new materials; the replacement's source spread must be reconciled before its own outcome can be accepted.
- Proof IDs are unique in `cut_proof_consumptions_v19` across all three event categories. Events, proof records and all F3.1 source history are append-only. The production application cannot create its own defect codes.
- The source and outputs remain labeled `DUAL_HUMAN_ATTESTATION_NOT_SENSOR_PROOF`. Neither output-count fields nor DB rows are sufficient to assert independently verified physical production. No automatic QC garment pass, finished-goods stock, shipment, or payroll credit.

## Production APIs (feature gated)

- `POST /api/v1/cutting/production/reconciliation/challenge` accepts `{ "command": { ... } }` and returns the exact challenge and the two required roles/actions.
- `POST /api/v1/cutting/production/reconciliation/events` accepts `{ "command": { ... }, "quality_assertion": "signed JWS" }` plus a supervisor bearer JWS. Each JWS must use the challenge from the exact full command; supervisor action `cutting.release`, role `cutting_supervisor`, QA action `quality.inspect`, role `quality_inspector`; both MFA (`webauthn`/`fido2`). Distinct subjects and proof IDs, same tenant/plant. Independent database grants `cutting.release` and `quality.inspect` are mandatory on each write.
- `GET /api/v1/cutting/production/reconciliation/balance?release_id=<uuid>` returns signed-source consumption, optional human classification, expected part slots, reserved identities, accepted/rejected/undecided counts and recut gaps. Requires a challenge-bound `cutting.read` bearer token with `CUT_F32_READ_V1`.
- `GET /api/v1/cutting/production/reconciliation/defects?limit=50&cursor=...` returns a keyset page of provisioned enabled defect codes for the authorized plant (same signed read challenge). Page limits 1–100.

`command` fields: `command_id`, `plant_id`, `action`, `release_id`, `spread_id`, `part_id`, `replacement_part_id`, `used_part_mm`, `trim_waste_mm`, `damaged_mm`, `defect_code`, `note`. For `RECONCILE`, provide `spread_id` and three mm categories; for `ACCEPT_PART`/`REJECT_PART`, provide `part_id` and an optional defect code only on rejection; for `LINK_RECUT`, provide `part_id` (rejected original) and `replacement_part_id`. Non-applicable fields are absent/zero. Notes are 12–1000 characters. Command IDs are idempotent within their event type and matched against the command hash and actors.

Feature gate: `AGIPAREL_CUTTING_RECONCILIATION_ENABLED=true`; requires `AGIPAREL_GENEALOGY_PRODUCTION_ENABLED=true`, `AGIPAREL_SEWING_PRODUCTION_ENABLED=true`, `ENV=staging|production`, database/IdP and separate out-of-band provisioned workforce grants. Default **off** in development and production templates. The `mod_sewing` module registers these routes, but does not activate them without this explicit gate.

## Important remaining engineering work

1. Physical sensor/reader commissioning: calibrated length and waste measurement, label-to-part verification and independently observed cut acceptance. Signed people are not equivalent to witnessed equipment.
2. Reversible discrepancy/rework workflow: independent correction/void records, returns, partial recuts, consumable remnants and discrepancy holds. These immutable first-pass dispositions cannot be silently overwritten.
3. Scalability: replace legacy lay constraints (210 plies, 80 positions, 8 parts, 16 routes) with governed versioned factory templates; add high-volume indexed counters and query plans.
4. Physical equipment integration, independent worker terminals and production operator UX using the established design system.
5. Real PostgreSQL 16 integration: parallel disposition conflict, proof replay, inter-tenant/plant denial, source issue attribution and physical acceptance tests. The static schema check alone is **not** concurrency qualification.

## Release gates

Back up DB, apply migration 0038 once (transactional), run schema check, `go test -count=1 ./...`, `go test -race -count=1 ./...`, `go vet ./...`, `go build ./...`, `scripts/test_*.cjs`, then build/deploy the existing **development** edge. Do not flip the production gate without real infrastructure and staging tests. Verify that the migration did not change existing cutting spread, release or F3.1 genealogy records.
