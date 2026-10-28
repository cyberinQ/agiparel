<!-- Author: Sunil Jain, 2026-10-28. Operational architecture and acceptance review. -->
# Chapter 07 — Production quality and finishing: operational review

## Current implementation and truth boundaries

- Chapter 07 currently has **development-only** browser-simulated garment inspections, predefined simulated finishing profiles, simulated buyer labels, and simulated repairs. These data are not real RF observations or QA/packing releases.
- Chapter 06's new production evidence (migration `0029`) contains signed reader observations and worker-attested **sewing completion groups**. A group has a nominal ply count but **not an individually identified garment or verified physical output count**.
- Consequently, an operation can record a *group-level inspection attestation* but must not claim individual pieces were accepted, finished, tagged, or shipped. No cross-table promotion from `qc_sim_*` to production is allowed.

## Delivered in Chapter 07

1. Migration `0030` creates separate tenant-RLS, provisioner-controlled quality assignments and a plant-specific defect code catalog. No factory, defect, or station defaults are embedded in production logic.
2. An **append-only group quality event ledger**, referencing only `sew_prod_completions_v10`, with database-enforced state transitions and row-level isolation; inspection count must match full nominal ply group for a PASS.
3. Authentication uses the existing pinned Ed25519 IdP and **exact-command challenge-bound FIDO2/WebAuthn** assertions. Roles split into `quality_inspector` and `quality_repairer`, with explicit enabled station assignments and plant scope.
4. `PENDING → REVIEWED` or `HOLD → REINSPECT → REVIEWED/HOLD`, with different people for hold assessment, repair, and subsequent reinspection. A later detected defect can return `REVIEWED → HOLD`.
5. Safe command retry (same command, payload, worker) and no reuse of an IdP proof for different work. One DB advisory transaction lock per tenant + completion group supports concurrent factory lines.
6. Inspector-authenticated, plant-filtered keyset catalog (`limit` up to 100, next-cursor) with **nominal group counts and no inferred garment totals**.
7. Feature disabled by default, even when sewing production is enabled. Separate ingress identity from the development Basic Auth gate. Development UI remains visually consistent and labels demo behavior explicitly.

## HTTP contract (staging/production only)

- `POST /api/v1/quality/production/events/challenge` `{ "command": { ... } }` gives `challenge`, `action` and required IdP role.
- `POST /api/v1/quality/production/events` same JSON plus `Authorization: Bearer <short-lived IdP assertion>`; IdP assertion signs the exact challenge and includes `tenant_id`, `plant_id`, action, role and strong authentication claim.
- Command: `command_id`, `completion_id`, `plant_id`, `station_id`, `action`, `inspected_units`, `defective_units`, `defect_code`, `note`.
- Actions: `INSPECT_PASS`, `INSPECT_HOLD`, `REPAIR_COMPLETE`, `REINSPECT_PASS`, `REINSPECT_HOLD`. Successful replay returns HTTP 200; first accepted event HTTP 201.
- `GET /api/v1/quality/production/catalog?limit=50&cursor=…` requires the same IdP with action `quality.read`, challenge `QUALITY_PROD_CATALOG_V1`, role `quality_inspector`, and active assignment.
- Every response declares quality evidence **not a garment, packing, or shipping release**.

## Critical production acceptance gaps — not implemented

**P0: Garment-level genealogy.** A sewing completion group is *not* equivalent to counted, independently labeled garments. To authorize any packing or buyer identity, establish unit serials/cut-part lineage and preserve genuine observed production counts. Reconcile rework, scrap, recuts, replacements and split groups without double counting.

**P0: Inspectable source evidence.** Gateway signature establishes the enrolled source, not physical RF authenticity. Commission real scanners, calibrate stations, verify tag-to-physical-item pairing and secure reader keys. QC actor attests an inspection; there is no camera/metrology independent check in this increment.

**P0: End-user identity and operator console.** Implement OIDC login/FIDO2 initiation and challenge signing without manually copying tokens. The current `/quality` HMI remains development-only. No production QC HMI is represented as completed.

**P0: Material disposition.** `REVIEWED` is not approved finished goods. Add segregated quarantine custody, explicit scrap accounting, disposition approvals and unambiguous QC-to-finishing-to-packing release events, with independent authorization.

**P1: Dynamic buyer-specific quality plans.** Versioned inspection plans and sample sizes, AQL requirements where contracted, buyer tolerances, configurable finishing route revisions, calibrated measurements, sampling provenance, severity, objective hold criteria and acceptance authorization.

**P1: Factory economics.** Count defect units vs inspected units with denominators, avoid conflating defects with defective garments, and capture repair time, cycle time, repeat-fail, material scrap, margin exposure and time-to-clear holds. No estimate based on simulated output should flow into FPY or contribution margin.

## Release gates

- Run migration 0030 after verified 0029; seed no default users, codes or stations. Only an authorized provisioner may enable a role/defect code.
- Validate two distinct factory scopes, forbidden cross-plant writes, conflicting commands, same-proof reuse, expired IdP tokens, independent reinspection and directly attempted SQL bypass.
- Go full-suite/race/vet/build and all JS regression scripts; exercise real PostgreSQL/RLS and transaction-race tests in staging; check backup and restore before production activation.
- `AGIPAREL_QUALITY_PRODUCTION_ENABLED=false` by default. Do not enable in live production until real IdP, provisioning, device traceability, production UI and end-to-end pilot results are approved.
