# F3.28 / v41 — Production trust isolation acceptance checks

Author: Sunil Jain, CyberinQ LLC · 2026-10-29

Baseline: `4831c15`; migration **none** (PostgreSQL schema remains 0057).

## Changes

1. **Production default-off** — `AGIPAREL_SEWING_PRODUCTION_ENABLED` changes from literal `true` to an explicit operator-controlled flag defaulting to `false`, aligning sewing with the existing dock and packing opt-in policy. Existing production environments must deliberately enable sewing after commissioning.
2. **Dock trust RLS and grant regression** — checks 10 provisioning/evidence tables for forced RLS, app-role privileges and mutable-vs-immutable authority separation, plus report-function invoker rights and explicit non-credit behavior.
3. **Parallel real app-role tenant isolation** — 12 concurrent PostgreSQL sessions (4–32 configurable) read identical gateway and signer IDs belonging to two different tenants with `SET LOCAL ROLE agiparel_app`. Every worker verifies same-tenant visibility, cross-tenant denial and absent-context denial. After a provisioner revokes one gateway, a fresh app-role transaction must see the revocation without affecting the second tenant.
4. **Regression CI + local recovery installer** — reuses v39/v40 disposable-db concurrency and rollback drills, Go/race/vet, 56+ JavaScript contracts, Docker build/deploy and GitHub push after health verification only.

## Exact trust boundary

This runs against an **ephemeral CI database or disposable restored snapshot** and does **not** bypass replication triggers. It tests real PostgreSQL tenant isolation and provisioner revocation visibility; the provisioned keys are dummy bytes and no device signs a physical observation. The preceding v39/v40 stress suite deliberately bypasses trigger checks *only* to test database uniqueness. No test currently certifies fully signed multi-factory ingress under production-like HTTP load.

This release does not implement ASN delivery, legally binding buyer acceptance, shipment/inventory/financial postings, full stock conservation or hardware commissioning. Feature switches stay off unless independently provisioned and explicitly enabled.

## Remaining acceptance campaigns

- Production-signed device and user enrollment, real-reader/needle detector/scale integration and authenticated cross-factory HTTP workload testing.
- Exact material, rework and financial-ledger conservation and ERP posting integration using balanced, idempotent journals and reconciliation.
- External ASN/EDI transport confirmation, buyer legal acceptance, site disaster-recovery (including RPO/RTO) and evidence-based factory UAT.
