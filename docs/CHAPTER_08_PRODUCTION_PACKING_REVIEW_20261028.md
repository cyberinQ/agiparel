<!-- Author: Sunil Jain, 2026-10-28. Operational, scalability and production safety review for Chapter 08. -->
# Chapter 08 — Packing and Dispatch: Production Integrity Review

## Executive judgement

The existing `/packing` page and `pack_sim_*_v11` data are a **development rehearsal**, not shipped garments. Chapter 06 establishes authenticated **sewing-completion groups**; Chapter 07 independently reviews those groups. Neither chapter verifies individual garment identity, accepted garment quantity, buyer packing specification, finished-goods receipt, physical carton contents, weight, custody, or carrier pickup. Therefore *no actual packing/dispatch authority may be inferred* from a QC group `REVIEWED` state.

## Implemented in this package

1. `0031_packing_production_holds_v12.sql` introduces a **separate production HOLD ledger** with tenant row-level security (FORCE RLS), provisioner-controlled plant/station/worker roles, cryptographic proof ID uniqueness and an immutable ledger. Simulation history remains separate.
2. `POST /api/v1/packing/production/holds/challenge` produces a hash-bound command challenge; `POST /api/v1/packing/production/holds` requires an external IdP FIDO2/WebAuthn-backed signed assertion with `packing.stage` action, `packing_operator` role and matching tenant/plant scope. The worker must also have an active plant/station assignment in the database. No self-enrollment endpoint is added.
3. The database trigger serializes with Chapter 07's group QC writes and requires the **latest QC event** to be `REVIEWED` with the expected QC revision. It resolves source nominal group quantity and QC event ID server-side. A new reviewed QC revision can be staged again, but an existing revision cannot be staged twice. A staged hold always has `HOLD_PENDING_UNIT_TRACEABILITY` state and is **not** finished inventory, a carton, or a shipment.
4. `GET /api/v1/packing/production/readiness` requires `packing.read`, role `packing_supervisor`, a valid IdP token and an active plant assignment; it uses plant-scoped, keyset-paginated reads and reports source group QC, current QC revision, hold evidence and blockers. It expressly reports `garment_units_verified=false`, `carton_release=false` and `dispatch_release=false` for every row.
5. Earlier legacy in-memory packing start/scan/seal actions are explicitly **development-only**. Enabling packing on a real server cannot turn a memory-only UPC scan into a production carton.
6. The existing Chapter 08 interface retains its visual design and navigation; its sample worker/station/reader identity values are no longer prepopulated, and dispatch remains clearly labelled a demo.
7. The production ingress route allowlist includes only the three new exact IdP-secured paths; production API is disabled unless both real sewing and real QC production APIs are enabled and `AGIPAREL_PACKING_PRODUCTION_ENABLED=true` (off by default).

## Production API contract

`POST /api/v1/packing/production/holds/challenge` (JSON):

```json
{"command":{"command_id":"PACK-STAGE-20261028-00001","completion_id":"12345678-1234-1234-1234-123456789abc","plant_id":"FACTORY_X","station_id":"PACK_QA_7","expected_qc_revision":2,"note":"Hold for individual garment verification"}}
```

This returns an exact transaction challenge, required IdP role, and `HOLD_ONLY` effect. Post the same JSON to `/holds` with the short-lived bearer attestation over the challenge. Replay with matching command/payload/actor returns HTTP 200; a new hold returns HTTP 201. The API never returns a shipment release.

`GET /api/v1/packing/production/readiness?limit=50&cursor=...` uses the fixed IdP challenge `PACK_PROD_READINESS_V1` and returns groups with one of `QC_NOT_REVIEWED`, `NEEDS_STAGING_HOLD`, `INDIVIDUAL_UNIT_TRACEABILITY_MISSING`, `QC_CHANGED_AFTER_STAGING`. Default page size 50, max 100. This is a **source-limited page**, not an entire factory total. A QC change after staging is surfaced as a blocker immediately; prior hold records are preserved.

## Mandatory P0 features before real cartons or dispatch

- **Unit-level genealogy:** uniquely commissioned physical garment IDs linked to genuine bundle/ply, measurements and item-level quantity reconciliation. Count splits, recuts, replacements, scrap and rework without double claims.
- **QC release and finishing:** explicit unit-level disposition with independent approved signatories, buyer/version-specific inspection plans, finishing requirements and quarantine custody. Group review alone is insufficient.
- **Pack plan and buyer contract:** versioned buyer PO, SKU/color/size, carton assortment, tolerance, customer label format, accessories/packing BOM, MOQ and capacity derived from governed configurable data, not code constants.
- **Real carton build:** authenticated physical scans at enrolled devices, unit-to-one-open-carton exclusivity, weight verification/calibrated scales where required, seal evidence, reopen/void dual authorization, tamper detection and durable offline replay.
- **Shipping authorization:** finished-goods receipt and custody, carton-to-manifest reconciliation, export documentation, carrier/vehicle identity, independent release, actual loading/gate-out proof, financial and stock accounting. Do not equate a saved manifest with dispatch.
- **Production operator HMI:** real OIDC + FIDO2 flow and protected site session; the present packing page is a development-only simulation, not a production operator console.

## Scale, security and release gates

No hard-coded factory IDs, station IDs, worker IDs, buyer specifications, sample SKU assumptions or fixed production totals. Only bounded API *page sizes* and constrained security fields are used. Keyset paging avoids silently truncated global results. Each tenant/plant must have provisioned roles with no default personnel.

Database checks required in staging: run migration 0031 after 0029 and 0030, verify app role cannot provision users or update/delete holds, enforce tenant RLS and cross-plant rejection, concurrent QC HOLD vs staging, same-revision duplicate staging, replay and proof reuse, and a new QC revision restage. Load-test read pagination with two factories and many completion groups. Validate a real IdP and production TLS/mTLS ingress. Do not activate production packing during development deployment.

**Classification:** pilot-only packing-readiness and immutable staging HOLD. Physical carton and shipment authorization remain unimplemented and **must remain blocked**.
