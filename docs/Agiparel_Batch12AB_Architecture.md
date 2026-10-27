# Batch 12A–12B — Factory Intelligence architecture

## 12A — Cross-stage conservation
For a selected cutting lay, read existing tables under a tenant-scoped REPEATABLE READ transaction. Unique garment serials are counted from completed simulated sewing credits, QC enrollment and dispositions, finishing routes, buyer bindings, packing items, sealed cartons and dispatch rehearsals. Report each stage's source and unit. Positive residual means WIP or unexplained difference; negative residual flags a count invariant breach. Neither means verified loss or scrap. Cut positions and cut bundle candidate groups are separate *non-comparable* counts.

## 12B — Interprocess timing, variability and bottlenecks
Compute per-garment sewing→QC enrollment and buyer identity binding→packing scan elapsed intervals from server timestamps. Reject missing endpoints, reversed timestamps, >2h gaps, and gaps crossing a local factory calendar day. Exclusions are counted by reason. Report N, unknown N, median, p90, p95, mean, standard deviation and CV. For sewing operations, compare p90 active minutes per garment to SAM only if N≥5. These are descriptive signals, not causal bottleneck attribution or worker-performance scores.

## Storage and APIs
Migration `0025_factory_intelligence_snapshots_v12.sql` creates immutable `intel_sim_snapshots_v12` with RLS, tenant-scoped lay FK, provenance, request hash, report JSON, and SHA-256 digest. The live report is read-only; capture inserts an immutable copy. Routes: `GET /api/v1/intelligence/sim/catalog`, `GET /api/v1/intelligence/sim/report?lay_id=...&timezone=...`, `POST /api/v1/intelligence/sim/capture`, `GET /api/v1/intelligence/sim/snapshots?lay_id=...`, and `/intelligence` UI. All endpoints require the existing operator gate and `ENV=development`. Do not make production decisions from these analytics.
