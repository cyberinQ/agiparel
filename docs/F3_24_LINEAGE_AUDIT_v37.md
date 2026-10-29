<!-- Author: Sunil Jain, CyberinQ LLC, 2026-10-29. -->
# F3.24 v37 — Cut-to-packing lineage and operational diagnostics

Baseline: `4bfecce`. Migration `0056`. All v36 controls retained.

## Combined changes

1. **Conservative cross-ledger cut/WIP audit:** per-manifest counts trace staged cartons into unique garment identities, signed cut-consumption records, effective accepted/reworked parts at the finishing station, latest quality release, open needle holds, individual WIP holds, offsite jobwork and disputes. Real-time *read-only* recomputation, not an event projection or shipment event.
2. **Operator exception diagnostics:** `GET /api/v1/packing/dock/lineage-audit?tenant_id=...&plant_id=...&manifest_id=...` uses the existing workforce identity, `packing.read` grant, tenant-scoped RLS and REPEATABLE READ. Named blockers; requests cannot close or waive a hold. The report caps cardinality at 5,000 cartons or garments and **fails closed** when larger.
3. **Adversarial regression gates:** PostgreSQL privilege and tenant-isolation checks, negative missing-manifest test, Go feature-toggle test, CI/release schema tests and one-command installer with disposable restore before deploy.

## Critical distinction

A complete `lineage_evidence_complete` flag proves **only** that selected cut-to-packing evidence predicates agree in the current database snapshot. It does not prove physical textile mass conservation, raw fabric inventory balance, trim stock, carrier network ASN delivery, independent sensor performance, external contractual acceptance, title transfer, financial posting or final shipment credit. All those remain **false** in the response. This endpoint is disabled by default, requires the v35 gate and has no mutation route.

## Remaining engineering after v37

Global material/trims/financial conservation; end-to-end privilege audit, secrets, multi-factory load/failure injection; operator exception UI and accessibility; hardware/reader/scale/needle detector commissioning; carrier/EDI and factory UAT. Do not enable a shipping ledger writer until those critical controls and independently evidenced acceptance are satisfied.
