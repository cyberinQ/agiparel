# F3.19C + F3.19D + F3.20A — Controlled reviews and dock preflight

Author: Sunil Jain, CyberinQ LLC — 2026-10-29.
Baseline: `0a24bba`. Migration: `0052_factory_safety_aql_and_dock_readiness_v33.sql`.

## F3.19C safety clearance

A signed `SAFETY_CLEARANCE` addresses one prior negative needle/metal-detector event, never a garment in bulk. Database admission requires: same tenant/plant/garment; an actual later recorded detector PASS for that garment; independent original reporter, follow-up observer, and clearing inspector; exact source-document SHA-256; and, for needle breaks, human-accounted fragment count equal to the initially reported total. Each follow-up PASS is consumed once. Multiple incidents require separate, distinct clearances. A new negative incident reopens the hold automatically. The prior `garment_needle_safety_blocked_v32` predicate and existing QC/packing triggers continue to enforce the gate.

**Not proof of detector physics:** detector PASS, source document and counted fragments are human-signed records tied to provisioned calibration history. Factory-level instrument commissioning and direct machine-signed acquisition remain required. Clearance does not produce QC release or payroll credit.

## F3.19D internal AQL review

`buyer_aql_review_rules_v33` is provisioner-only and references the same currently buyer-approved source hash and immutable plan identity. It provides plan-specific acceptance counts. An independent `quality.aql.review` assertion is accepted only after all fixed sample garments have signed findings, counts exactly match live database values, the plan revision is current and enabled, and the document is still approved. A recommendation cannot exceed the provisioned thresholds. A `HOLD_FOR_BUYER_REVIEW` records an internal hold, not an accepted lot. The application cannot edit or create threshold rules.

**Internal recommendation is not buyer acceptance:** authenticated buyer approval, contractual AQL standards and a separate external-authority step are still outstanding.

## F3.20A dock preflight (read-only)

`GET /api/v1/packing/dock/readiness?tenant_id=...&plant_id=...&manifest_id=...` requires a scoped workforce browser session with `packing.read`. It checks staged manifest presence, the actual immutable carton list, sealed cartons, active garment QC and unresolved safety incidents, and current enabled buyer AQL plans with internal recommendation. If no active buyer plan exists, the preflight is blocked. All response flags explicitly deny actual carrier handover, ASN transmission and shipment credit.

This release does **not** integrate physical dock reader evidence, container seals, carrier-signed receipt or ASN transmission. Those remain F3.20B production tasks.

## Activation and authority

The new routes are gated by `AGIPAREL_FACTORY_RELEASE_V33_ENABLED=true` plus the existing `AGIPAREL_FACTORY_EVIDENCE_V32_ENABLED`, production/staging environment, material signed assertion verifier, and workforce identity. No grants are provisioned automatically. New actions: `quality.needle.clear`, `quality.aql.review` (independent signed rights); read-only dock uses existing `packing.read`.

## Validation and deployment

- Install the changed files over exactly `0a24bba` and a clean Git tree.
- Back up PostgreSQL. Apply migration 0052 once with `ON_ERROR_STOP=1`.
- Run `scripts/test_factory_f319c_f319d_f320a_schema_pg.sql` (read-only security contract).
- Run `go test -count=1 ./...`, `go test -race -count=1 ./...`, `go vet ./...`, `go build ./...` and all `scripts/test_*.cjs`.
- Build and deploy `plant-edge-os` to development; check process logs, worker grants and fail-closed startup.
- Commission real identity keys, inspectors, buyer sources, calibration, and gate hardware separately before activation.
- Test real PostgreSQL negative paths in a disposable database clone: cross-tenant and wrong-plant, out-of-sequence passes, reused signed proofs, insufficient fragments, reviewer equals inspector, incomplete fixed sample, stale buyer revision, revoked buyer document, concurrent reviews, new safety incidents after clearance, previously sealed carton becoming unsafe, and missing or out-of-date plan.

No SQL history is rewritten; existing incidents remain unresolved until independent evidence and review are recorded.
