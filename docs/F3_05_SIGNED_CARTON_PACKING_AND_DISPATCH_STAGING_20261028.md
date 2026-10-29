<!-- Author: Sunil Jain, CyberinQ LLC, 2026-10-28 -->
# F3.5 — Individually Traceable Cartons, Human-Attested Weight & Staged Dispatch

Baseline: `24d0938` (F3.4). Migration: `0041_carton_packing_and_dispatch_staging_v22.sql`.

## Physical meaning and explicit limits

- A garment may be added to exactly one carton only after its **individual** F3.4 terminal `QUALITY_RELEASE`; Chapter 07 group statistics are not packing evidence.
- Each carton is constrained to a single provisioner-approved signed cutting lay and versioned packing specification linked to an immutable source document. This conservative pilot restriction is **not** a universal buyer packing requirement.
- Size-specific expected mass, carton tare, permitted absolute weight difference, and minimum/maximum units are provisioned; none are global hard-coded sizes or weights.
- `SEAL` requires an independent supervisor's transaction-bound signed assertion, a scale reference, and a reported gross weight within the approved tolerance. **A manually entered scale reference and signed reading do not establish calibration or sensor provenance**. The API explicitly returns `weight_calibrated:false`.
- Dispatch manifests record sealed carton membership and a specified destination, but the terminal action is `STAGE`. **It does not establish carrier pickup, physical custody transfer, shipment, export or financial recognition.** The API returns `shipped:false`.
- All events are append-only, tenant/plant restricted and single-use by command ID and signed proof ID. Rejected/repaired carton correction and replacement are deferred; never delete historical events.
- This increment makes no inventory or payroll credit and cannot activate earlier legacy simulation routes.

## Privileged provisioning

A privileged DB provisioner must create `pack_carton_specs_v22`, separately insert `pack_size_weights_v22` rows for **every** `cut_lay_positions_v8.size_code`, and activate the specification only after rules are complete. The source document must exist in `document_sources_v65`, with the exact source SHA-256. A signed sewing release must exist for the selected lay. The application DB role has SELECT-only rights to specifications and weights and cannot self-approve.

`pack_carton_specs_v22`: `tenant_id,plant_id,lay_draft_id,revision,source_document_id,source_sha256,min_units,max_units,tare_g,tolerance_g,approved_by,enabled`. Enable as a separate update after inserting weights. A later disable/revocation prevents new carton and manifest actions. The provisioner must have the necessary PostgreSQL row-level-security bypass privileges or another separately authorized provisioning mechanism.

Configure external signed assertions with exact tenant, plant, action, role, challenge and MFA/FIDO2. New explicit F1.3 permissions are `packing.pack`, `packing.seal`, `packing.manifest`, `packing.dispatch_stage`; read permission is existing `packing.read`. The corresponding role claims are `packing_operator`, `packing_supervisor`, `dispatch_coordinator`, `dispatch_supervisor`. The existing workforce identity grant and RLS restrictions apply in addition to the signed JWT.

## API contracts

All commands require `Content-Type: application/json` and an object `{ "command": { ... } }`. UUIDs are client-generated immutable identifiers; every command gets a distinct ID. Obtain a challenge by POSTing the exact command object to the corresponding `/challenge` path; send the **same** command JSON to `/events` with an external IdP-issued, exact-challenge `Authorization: Bearer` proof. Challenges are not authorization.

Carton endpoints:

- `POST /api/v1/packing/units/challenge` and `POST /api/v1/packing/units/events`.
- Common fields: `command_id,plant_id,carton_id,spec_id,action,note`.
- `OPEN`: adds `carton_code`; no garment or weight. `ADD`: adds `garment_id` of a F3.4-released garment. `SEAL`: adds `observed_g,expected_g,scale_reference`; the expected mass must exactly equal the sum of the approved size-weight rules plus carton tare; the observed mass must be within provisioned tolerance. Independent worker from every worker who opened or added garments to that carton.
- `GET /api/v1/packing/genealogy?kind=carton&id=<carton_uuid>&limit=50&cursor=<event_no>` returns paginated signed event history, including garment IDs and mass evidence.

Manifest endpoints:

- `POST /api/v1/packing/manifests/challenge` and `POST /api/v1/packing/manifests/events`.
- Common fields: `command_id,plant_id,manifest_id,spec_id,action,note`.
- `OPEN`: adds `manifest_ref,destination_ref`; `ADD_CARTON`: adds `carton_id` for an already sealed carton under the same approved spec; `STAGE`: independently authorizes staging of a nonempty manifest (not physical handover). No carton can belong to two manifests.
- `GET /api/v1/packing/genealogy?kind=manifest&id=<manifest_uuid>&limit=50&cursor=<event_no>` exposes linked cartons and original manifest reference/destination.
- `GET /api/v1/packing/specification?lay_id=<lay_uuid>` exposes the active source-backed spec and size weights to a signed `packing.read` actor.

GET calls require exact `packing.read` JWT challenge `PACK_UNIT_GENEALOGY_READ_V1` and `packing_supervisor` role. No bearer/session fallback for production writes. Every scope/grant check occurs inside the tenant-scoped database transaction.

## Development rollout / acceptance

1. Verify `git HEAD` is `24d0938` and the worktree is clean; check the changed-files ZIP before overwriting tracked files.
2. Back up Postgres, run migration `0041`, then `scripts/test_packing_f35_schema_pg.sql`. Do not run the migration twice; stop on any PostgreSQL error.
3. `go test -count=1 ./...`, `go test -race -count=1 ./...`, `go vet ./...`, `go build ./...` and all `scripts/test_*.cjs`.
4. Build/deploy `plant-edge-os` via the existing development Compose file. `AGIPAREL_CARTON_GENEALOGY_ENABLED=false` is the default; no live factory route is opened by development deploy.
5. In isolated staging with a properly configured IdP, signed QC release and approved packing specification, exercise valid `OPEN -> ADD -> SEAL`, duplicate garment across two cartons, overweight carton, altered JWT payload, unapproved spec, revoked spec, wrong plant, concurrent duplicate and `manifest OPEN -> ADD_CARTON -> STAGE`; all invalid cases must fail closed.
6. Real calibration, line-side scanner and warehouse/transport handover acceptance remain required before asserting shipping readiness.

## Deferred capabilities

No automated calibrated-scale gateway; no mixed-lay cartons; no buyer-defined size/color ratio matrix or carton label printer; no carton correction/unseal workflow; no physical dock scan or carrier acknowledgment; no AQL/needle test approval gate; no dispatch-as-shipped semantics; no ERP invoice/ASN/stock recognition. These require separate explicit evidence and guarded states, not inferred defaults.
