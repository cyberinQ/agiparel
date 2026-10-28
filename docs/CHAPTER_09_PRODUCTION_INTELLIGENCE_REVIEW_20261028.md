<!-- Author: Sunil Jain, 2026-10-28. Production evidence-first Chapter 09. -->
# Chapter 09 — Production Intelligence: operational review and acceptance

## Current state and finding

The existing `/intelligence` page and `/api/v1/intelligence/sim/*` are intentionally **development-only**. Historical plotted numbers derive from rehearsal tables. Chapters 06–08 introduced authenticated sewing-completion **group** evidence, current QC group states, and staging HOLD records. None of these constitutes individual garment output, a shipment, payroll credit or profit. Rendering simulated figures as real would mislead factory management.

## Delivered

- In production/staging, `/intelligence` serves a visually consistent **read-only production evidence** page, not the rehearsal UI. The development UI remains unchanged.
- `GET /api/v1/intelligence/production/report?since=<RFC3339>&until=<RFC3339>` requires a challenge-bound, strongly authenticated IdP `intelligence.read` / `factory_manager` assertion, enabled flag, and a separately approved tenant/plant reading assignment. No generic production Basic-Auth exemptions.
- Bounded time windows (configurable `AGIPAREL_INTELLIGENCE_MAX_WINDOW_DAYS`, default 31, max 365) protect large datasets. Existing per-plant completion timestamp and QC history indexes support the read. No N+1 browser calls.
- One repeatable-read SQL transaction provides `as_of`, cohort definition and a consistent latest-QC/current-staging snapshot. The selected cohort consists of *operation completion groups created in the reporting interval*. Its QC and staging statuses are **current at query time**, not historical period-end values.
- Explicit group counts: completion, reviewed, HOLD, reinspection, unreviewed, current staging HOLD, stale staging HOLD. These counts do not sum nominal plies or pretend operations are distinct garments. Five unsupported KPIs remain `NOT_MEASURED` (`null`, not `0`): verified garments, shipped garments, first-pass yield, labor efficiency, order margin.
- New migration 0032 adds *only* the provisioner-governed, tenant-RLS reporting-assignment table. No production history is edited or backfilled.
- The manual short-lived assertion input is intentionally neither stored nor sent to any URL except the read-only production endpoint. Integrated enterprise SSO is not yet delivered; this remains a controlled pilot rather than final production acceptance.
- No new colors, global banner changes, employee rankings, or auto-authorized production transactions.

## Factory-manager priority backlog

1. **P0 — Serial genealogy:** individual unit identity, signed cut/sew/inspection/packing evidence and acceptance gates. Without this, no garment KPIs or shipping release.
2. **P0 — Production SSO:** enterprise IdP federation, strong-auth session lifecycle, expiry/revocation, tenant/plant-switch permission, frontend identity integration. Remove manual token entry before broad rollout.
3. **P0 — Reconciled source model:** ingest first-inspection facts, shift calendars, WIP inventory, approved SAM and clock-quality envelopes. Version calculations and exemptions.
4. **P0 — Performance and scale tests:** millions of completion groups, concurrent plant reads, index/plan baselines, pagination/drill-down, tenant isolation and low-cardinality database roles.
5. **P1 — Metrics series:** governed per-period materialized rollups with formula revision, numerator/denominator, sampling, and `as_of` so trends cannot silently compare incompatible periods.
6. **P1 — Commercial economics:** actual landed cost, accepted garment count, signed shipment and buyer receipt, contract revision, overhead allocation and currency for real margin/OTIF.

## Acceptance boundary

This increment is **pilot-only**. The API is disabled by default and returns 503 without prerequisites. Release requires migration applied in a controlled environment, RLS verification under the real app role, provisioned reader assignments, external IdP, performance/concurrency testing, and real production evidence. No invented values, demo/staging cross-talk, or package-release authority.
