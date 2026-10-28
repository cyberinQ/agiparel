<!-- Author: Sunil Jain, CyberinQ LLC, 2026-10-28 -->
# F3.4 — Source-backed garment inspection, rework, finishing, quality release

**Baseline:** `4f76a9c` (F3.3). **Migration:** `0040_garment_quality_specs_and_release_v21.sql`. **Flag:** `AGIPAREL_GARMENT_QUALITY_ENABLED=true` (production/staging only; default false). This API additionally requires F3.1/F3.2/F3.3 production prerequisites, an IdP verifier, a provisioned database and authentic FIDO2/WebAuthn transaction attestations. Development stays disabled.

## What becomes authoritative

Each garment references a signed cutting-release identity (F3.1). A production-specification administrator records a Tech Pack/inspection specification **for a particular signed lay**, with exact captured `document_sources_v65` ID and SHA-256, a revision, one approved finishing station, and named approver. The specification starts **disabled**. Before the administrator activates it, they must register size-specific integer-millimetre measurement rules; the schema requires coverage for every size in that lay. Only one specification revision may be active per lay. An active revision's measurements and source cannot change. To revise, disable it and publish a new revision. A garment in progress cannot switch revision.

The schema also holds plant-scoped, administrator-controlled defect codes. An operator cannot create a valid code or specification through the production API. Document extraction is unreviewed evidence until independently approved, typed and provisioned as a specification. No automatically accepted AI-extracted dimensions.

A garment cannot start QC until **every effective cut part** has an accepted F3.2 outcome and its last F3.3 custody transaction is `ARRIVE` at the spec's approved finishing station. Rejected original parts may only be replaced by an explicit, accepted F3.2 recut link. Without a resolved replacement, QC remains blocked. This is a chain of signed attestation records, not an independent proof of physical stitch completion.

## Signed state machine and permissions

- `INSPECT_PASS` / `INSPECT_HOLD`: first decision. Inspector requires `quality.inspect`, FIDO2 role `quality_inspector`. PASS requires all and only approved size-specific measurements within tolerance. HOLD requires provisioned defect code, no fake PASS measurement.
- `REPAIR_COMPLETE`: only after HOLD; separate operator `quality.repair` and FIDO2 role `quality_repairer`, not the inspector who placed the hold.
- `REINSPECT_PASS` / `REINSPECT_HOLD`: only after repair. A different inspector from the repairer; passing measurements independently reevaluated against the pinned specification. Repeated holds and repairs permitted.
- `FINISHING_COMPLETE`: only following a passing inspection, by a different actor using `quality.finish` and `finishing_operator` role.
- `QUALITY_RELEASE`: only following finishing and by a different actor using `quality.release` and `quality_manager` role. It is a **quality-only** release, not a carton/shipping release, nor independently verified sewing completion.

The PostgreSQL insert trigger allocates strictly serial event numbers using a per-garment transaction lock, rejects illegal transitions, enforces source scope, part readiness, rework separation and inspection measurements. `command_id` is idempotent only for matching payload/actor, and proof IDs are globally unique per tenant. Events, specifications and measurement-rule revisions cannot be silently edited. After QC begins, adding original cut parts or recut links is blocked; after quality release, further WIP movements are blocked, including recut replacements. These protections are additional to the API-level IdP signature and provisioned action grants.

## Production HTTP contract

Routes (under the production-only gateway):

- `POST /api/v1/quality/garments/challenge` with `{"command":{...}}` returns the exact command-bound challenge, required permission and role. The command includes `command_id`, `plant_id`, `garment_id`, `spec_id`, `action`, `defect_code` (empty unless HOLD), `measurements_mm` (map code to integer millimetres; empty except PASS) and audit `note` (12–1000 characters).
- `POST /api/v1/quality/garments/events` with the **same** command and a short-lived bearer IdP assertion bound to the challenge, role, plant and strong authentication. Returns `event_no`, evidence type, `quality_released` (only if release action) and explicit `shipping_release:false`.
- `GET /api/v1/quality/garments/specification?garment_id=<uuid>` returns source document reference, specification revision, size, finishing station and approved measurements for `quality.read` bearer assertion with challenge `GARMENT_QUALITY_READ_V1`.
- `GET /api/v1/quality/garments/history?garment_id=<uuid>&limit=<n>&cursor=<event_no>` gives plant-scoped, keyset-paginated event history, also requiring `quality.read`.

The old Chapter 07 group QC API remains unchanged. Group inspection is **not** unit-level quality release. The old Chapter 08 readiness HOLD/packing API remains unchanged and receives no new authority from F3.4.

## Provisioning sequence (only authorized DB administrator)

1. Import and review the Tech Pack/quality document through Chapter 10; independently validate the exact source and measurements. Resolve contradictions before approval.
2. Create disabled `qc_garment_specs_v21` record with its exact tenant/plant/lay/source SHA-256/finish station and approver.
3. Insert `qc_garment_measure_rules_v21` rows for *every* size in the signed lay, with measure names and integer-mm nominal/minus/plus tolerances; configure applicable `qc_garment_defects_v21` codes separately.
4. Activate the specification by `UPDATE qc_garment_specs_v21 SET enabled=true WHERE tenant_id=... AND id=...` under an administrator role. The trigger blocks incomplete size coverage or multiple simultaneously active revisions.
5. Independently provision `quality.inspect`, `quality.repair`, `quality.finish`, `quality.release`, `quality.read` grants scoped by tenant, plant, issuer, subject. Never provision these from a browser query.
6. Verify scanner, part acceptance, cut/re-cut genealogy, station route and worker attestation on a dedicated staging factory before enabling `AGIPAREL_GARMENT_QUALITY_ENABLED=true`.

## Acceptance, limitations and next work

Run migration 0040 as an approved database owner, then the read-only `scripts/test_garment_f34_schema_pg.sql`. Run the Go full repository suite, `-race`, vet/build, all JS scripts and Docker production-config parse. Conduct **real PostgreSQL 16 multi-session tests** for concurrent QC decisions, spec revision conflicts and WIP-after-release, plus forged/replayed/expired/mismatched assertion negatives. Do not activate production until these pass.

**Explicit limitations:** no production-certified machine cycle evidence, no independent physical geometry sensor, no graded AQL sample release, no per-operation sewing completion confirmation, no carton/packing permission, no shipment/cost/payroll authority, no automatic first-pass-yield KPI or production UI. F3.4 creates a genuine signed *QC data workflow*, not certified completed-garment output. Changes to a garment's genealogy after inspection require a future governed void/correction workflow rather than silently modifying evidence. Preserve existing UI layout and design.
