# Chapter 06 — Sewing Execution · Operational review

Author: Sunil Jain · 2026-10-28

## Operating truth and baseline

Chapter 06 currently implements a **development-only, UI-emulated sewing rehearsal**, not a certified factory execution system. `sew_sim_scans_v9` contains no physical EPC/RSSI, operation routes are unpublished, cycle timing comes from server-received operator taps, and `sew_sim_credits_v9` never earns payroll, output, quality or dispatch authorization. Do not interpret its “jobs done” number as finished garments; it counts simulated operation completions (the same garment may undergo multiple operations). The archived `/sew01` page remains a disconnected teaching simulation; its sample timers and KPIs are not production evidence.

## Chapter 06 changes made

| Priority | Risk | Implemented correction / acceptance |
|---|---|---|
| P0 | Entire tenant's sewing scans/commands serialized through one advisory lock (`"COMMANDS"`) | Event/command-key advisory locks, with the existing unique constraints and `FOR UPDATE` cycle state lock intact. Independent events no longer contend on one global key. |
| P0 | 80 cutting lays and 800 bundle rows could silently cap a factory's selectable work | Keyset-paginated lays and groups; select-group-only bundle fetch. Existing catalog consumers still receive paginated `bundles` with `bundles_next_cursor`, while the UI uses `groups_only=1`. No fixed total factory ceiling. |
| P0 | Hard-coded plant/station/worker/machine/SAM values | No invented identities, station, machine class or SAM. Explicit valid inputs required before any simulated event; the old training page is labeled non-integrated. |
| P0 | Retry events shared across every station in one browser | Tenant-browser/local device retry queues scoped to plant+station+worker, preserving existing event IDs. One-time migration from v1 pending records (deduplicated) without discarding valid unsent events. Idempotent server replies remain authoritative. |
| P0 | Offline tap could be presented as scanned and later replayed under a different operator | Only server-committed/replayed scan events get UI ACK; no scan event earns real RFID evidence. Duplicate/pending taps blocked. Cross-station replay prevented. An explicitly confirmed **Discard oldest local retry** action recovers from permanently rejected unsent demo events; no server record is deleted or reversed. |
| P0 | Switching lay, group or worker could overwrite the new view with an old request | Monotonic view/status tokens and actor checks discard old results; work context and controls reset until reloaded. |
| P1 | “Saved work” list silently stopped at 250 rows; recovering an open job beyond that page failed | `cycles_has_more` labels recent-history scope, while accurate DB aggregate remains independent. A separate station/worker-scoped active-cycle query supports recovery; multiple active jobs are flagged as ambiguous and fail closed. |
| P1 | Large-tenant catalogue and status queries underindexed | Additive migration `0028_sewing_catalog_query_indexes_v9.sql`: five tenant-scoped query indexes. No data, RLS, identity, event hashes, immutable source or production authorization changes. |

UI preservation: Chapter 06's existing blue palette, cards, form controls, worker-first steps, shared workspace header, responsive layout and typography are unchanged. Only two small paging buttons, one truthful history scope, and visible station guidance are added.

### API compatibility and pagination

- `GET /api/v1/sewing/sim/catalog` returns a page of 80 lays and `lays_next_cursor` (opaque; send as `lay_cursor`).
- `GET .../catalog?lay_id=<uuid>` preserves the existing `bundles` field, now with a `bundles_next_cursor` for keyset continuation; `groups` and `groups_next_cursor` are also returned. Existing clients that only read the first page still receive the original first 800 bundle rows.
- `GET .../catalog?groups_only=1&lay_id=<uuid>` returns 100 bundle groups per page; send `group_cursor` to continue without loading every piece label.
- `GET .../catalog?lay_id=<uuid>&bundle_position=<n>&bundle_group=<n>` fetches every part of exactly one selected group, without silently dropping parts.
- `GET .../status?lay_id=<uuid>&plant_id=<code>&station_id=<code>&worker_id=<code>` returns up to 250 recent cycles, `cycles_has_more`, the **all-history** simulated-operation-unit total, and one active cycle (or `active_conflict`).
- Page sizes (80/100/800/250) are per-request throughput guards, **not factory-size or production-capacity parameters**. Clients follow opaque cursors. Multiple operations on one garment are not a garment yield total.

## Unresolved architecture gaps — not represented as completed

1. **P0: Actual authenticated sewing.** Bind worker and station identity to a verifiable authority; ingest signed physical RFID observations, approved operation bulletin revisions, machine telemetry, quality acceptance and handoff custody. Reject UI simulation data as production evidence.
2. **P0: Factory/site-scoped work authorization.** The cutting draft's `plant_id` is not governed as a multi-site allocation at source. Bind work orders, bundle issue, approved routes, stations, machine classes and staff assignment to a plant; require approved transfer evidence for cross-site moves.
3. **P0: No universal hard-coded physical envelope.** Cutting's schema still has specific ply limits (210), marker positions (80), parts per garment (8), and sewing route parts (16). These constraints originate upstream and require an explicit, backwards-compatible domain model/schema migration coordinated with Chapter 05; silently expanding only sewing would violate cut-to-sew conservation.
4. **P0: Sewing-floor exception management.** Missing part, shade mismatch, damage, machine failure, rework, line balancing, no-work WIP and late-order risks need typed, accountable holds with plant/shift/line scope and closure SLAs. Current free-text `ISSUE` is only a simulation hold.
5. **P1: Quantified profitability and throughput.** Real SAM vs observed operation time, approved labor rates, changeover/idle time, workstation capacity, defects/rework and due dates are required for contribution margin, capacity and bottleneck optimization. Current simulation cannot support OEE, FPY, payroll or efficiency claims.
6. **P1: Production-scale operation selection.** Approved route templates should be product/style/version-scoped rather than manually invented per cutting lay. Filtered station work queues, machine compatibility, shift and allocation rules are needed to serve thousands of jobs without scrolling.
7. **P1: Migration rollout at scale.** The additive index migration is suitable for this development deployment. For a large live PostgreSQL cluster, use a planned online (`CREATE INDEX CONCURRENTLY`) variant and verify query plans, lock times and throughput under load.

## Validation and deployment

- Browser regression `scripts/test_sewing_ch06_workflow.cjs` covers dynamic factory identities, lay/group pagination, loaded-only selected parts, all-history totals, offline no-ACK, cross-station isolation, replay and stale response isolation.
- Go cursor tests validate stable keyset encoding and malformed cursor rejection; Go UI contract test verifies retained design hooks and no sample defaults.
- All 12 JavaScript regressions run in this sandbox; Go 1.23 isolated cursor tests run. The full Go 1.24 module tests/build, SQL application and Docker deployment must run on the iMac.
- **Migration required:** `services/core-api/migrations/0028_sewing_catalog_query_indexes_v9.sql` (additive indexes). Apply once before starting the new edge binary. Do not blindly replay older migrations.
