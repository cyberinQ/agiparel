# F3.6 — Evidence-Based Factory Performance Dashboard

Author: Sunil Jain, CyberinQ LLC — 2026-10-28

**Base source:** F3.5, after migration `0041_carton_packing_and_dispatch_staging_v22.sql`.
**Scope:** Chapter 09 production read-only report; no writes, simulations, schema migrations or changes to production release rules.
**Feature gate:** Existing `AGIPAREL_INTELLIGENCE_PRODUCTION_ENABLED=true` in staging/production, subject to all Chapter 09 prerequisites. Default development profile stays disabled.

## Source-backed measures

All new metrics appear in the `factory_evidence` field of the existing authenticated `GET /api/v1/intelligence/production/report?since=...&until=...` response. The legacy `metrics` field remains unchanged and continues to describe **sewing-completion groups**, which must not be confused with individual garments.

The new `factory_evidence` authority is `SIGNED_UNIT_AND_CUSTODY_EVENTS_NOT_PHYSICAL_CERTIFICATION` and `production_release` is always `false`. In one tenant-bound repeatable-read snapshot, each source is filtered by exact tenant, plant, and the half-open `[since,until)` period of **its own event timestamp**:

| Metric | Authoritative source | Exact basis |
|---|---|---|
| Cut parts accepted/rejected | `cut_part_outcomes_v19` | Dual-human-signed terminal decisions; recuts are separate parts |
| Cut-part acceptance (%) | Same | Accepted / (accepted + rejected) decisions in the period |
| Garments initially inspected | `qc_garment_events_v21` | `INSPECT_PASS` and `INSPECT_HOLD` only; no reinspections |
| Garment first-pass yield (%) | Same | Initial `INSPECT_PASS` / all initial inspections in period |
| Garments quality released | Same | `QUALITY_RELEASE` events; **not shipped units** |
| Reported consumption length / classified-use ratio | `cut_consumption_reconciliations_v19` | Dual-human-attested used, trim, damaged millimetres. **Not CAD marker-area utilization** |
| Completed handoffs, P50/P95 | `wip_movements_v20` | Matching adjacent signed DEPART/ARRIVE events for same part and approved route; database timestamps |
| Ten slowest observed routes | `wip_transfer_routes_v20` and handoffs | Highest P95 handoff duration, sorted and capped; not machine cycle speed |
| Garments added to cartons | `pack_carton_events_v22` | `ADD` events, regardless of whether carton subsequently sealed |
| Cartons sealed | Same | `SEAL` events; weight checked against human-attested readings, not calibrated instrumentation |
| Manifests staged | `pack_manifest_events_v22` | `STAGE` events, **not physical handover or shipments** |

Unsupported measures remain `value: null`, `status: NOT_MEASURED`: physical shipment units, OEE, labor efficiency, and true fabric-area utilization. A rate with no denominator is `value: null`, `status: NO_DENOMINATOR`; **zero measured output with a nonzero denominator is 0%**. Rates are transmitted in basis points (`10000 = 100.00%`) to avoid ambiguous float formatting. Durations are signed-recorded intervals in milliseconds, never ideal machine cycles.

**Cross-stage percentages are intentionally absent.** Cutting, QC, packing and dispatch stage counts use independent event cohorts; dividing them would create a false conversion or yield story.

## Security and operational controls

- Existing short-lived signed pilot assertion or verified OIDC access token / confidential browser SSO session, plus `intelligence.read` chapter grant **and** independent `intelligence_prod_readers_v13` assignment.
- Same tenant-bound repeatable-read database transaction for group and unit evidence; no arbitrary SQL, no client-supplied role, no simulation source.
- Bounded report time window (default 31 days); timestamps RFC3339 with exclusive end. Route diagnostics capped at ten; no individual worker performance ranking.
- Cache-Control `no-store`, DOM `textContent` only, no browser storage, unsupported metrics fail closed, and session-mode cookies never attached to explicit bearer reads.
- No new database objects, migrations or seed data. Use `scripts/test_intelligence_f36_schema_pg.sql` as the read-only PostgreSQL readiness gate.

## Test and acceptance gates

1. Prerequisite: F3.5 code and committed migration 0041; inspect Git history and DB migration state before overlay.
2. Run PostgreSQL readiness check against your own PostgreSQL 16 instance with `ON_ERROR_STOP=1`.
3. Run all `go test -count=1 ./...`, `go test -race -count=1 ./...`, `go vet ./...`, `go build ./...` and `scripts/test_*.cjs` regression scripts on Go 1.24.
4. In staging with a real IdP: validate tenant denial, plant denial, revoked reader grant, source-created records, zero denominator and valid observed zero; compare select counts against SQL for the same window.
5. Simultaneous inserts and readers must not leak rows across snapshots. Verify latency at target event volumes, real index usage and any query-plan improvements before production certification.
6. Retain physical device acceptance, calibrated instrumentation, genuine carrier handover and actual performance standards as separate future work.

The Chapter 09 visual system remains unchanged: the existing group cards and table are retained, and a new unit-evidence table and bounded slowest-route table are added below them.
