# Agiparel Batch 08B — Issue-backed cutting spread evidence and signed bundle EPC witnesses

Baseline: Batch 08A after migrations 0001–0019, including Batch 07C hotfix. Apply migration **0020** before deploying new server code.

## Scope and trust boundaries

This is a **physical-execution evidence workbench**, not an unconditional release to sewing. It provides:

1. A source-linked cutting draft with immutable per-position / part / ply-group label genealogy from 08A.
2. A single immutable operator-reported spread record for that exact lay revision. The report stores observed actual plies, measured marker length, reported fabric consumption in **integer millimetres**, explanatory evidence, and allocated signed material issue UUIDs.
3. A transactionally enforced conservation check: every attributed issue must match the same tenant, buyer order, cut number, fabric SKU, and shade. Reused or over-attributed issue millimetres are rejected, including concurrent requests. A legacy PO-number issue is eligible only if that PO-number uniquely identifies one staged buyer intake in the tenant.
4. Frozen lay revisions after spread evidence; an obsolete revision cannot silently rewrite physical sub-bundle genealogy. A mismatch between observed plies and the planned ply count requires correcting the draft **before** recording spread evidence.
5. A signed EPC-witness API (strong FIDO2/WebAuthn identity assertions from a configured external identity provider) for each exact generated sub-bundle code and EPC. The witness confirms a human-signed assertion about that code/EPC, **not** hardware-origin, antenna reads, or RF capture. It does **not** authorize any material movement, cutting release, sewing credit, or garment serialization.
6. A PostgreSQL guard prevents one RFID EPC from being bound to both a material roll and a bundle in the same tenant, even by concurrent writers.

A common dev operator password can record **reported** spread evidence, but cannot issue stock or create authenticated EPC witnesses. The material-signing APIs are disabled unless an external IdP is explicitly configured. The browser makes no claim that labels are observed RFID reads.

## Endpoints

### Restricted operator: `GET /api/v1/cutting/spreads?draft_id=<uuid>`

Returns `spreads`, `matching_signed_issues` with issued/attributed/open millimetres, `bundle_witnesses`, `authority: OPERATOR_REPORTED_ONLY_NO_RELEASE`, `hardware_rfid_proven: false`. Without `draft_id`, lists up to 100 evidence reports. Restricted to development operator tenant using the established operator gate; production without provider identity fails closed.

### Restricted operator: `POST /api/v1/cutting/spreads`

`Content-Type: application/json`, at most 32 KiB, exactly one JSON object, no unknown fields. Example **structure**, not demo production truth:

```json
{
  "draft_id": "<actual cut_lay_drafts_v8 UUID>",
  "request_key": "spread_20261027_measured_ply_001",
  "actual_plies": 20,
  "measured_marker_length_mm": 6000,
  "reported_consumption_mm": 125000,
  "issue_allocations": [
    {"issue_id": "<first actual material_issues_v5 UUID>", "allocated_mm": 100000},
    {"issue_id": "<second actual material_issues_v5 UUID>", "allocated_mm": 25000}
  ],
  "evidence_note": "Operator observed spread and marker measurement; physical release remains pending."
}
```

Returns HTTP 201 with `spread` or 200 for exact request-key replay. Conflicting replay, insufficient signed issue coverage, wrong shade/material/cut/order, stale or altered genealogy, and duplicate reports fail closed. Underlying signed issue rows already reflect stores issuance, so **this transaction does not decrement stock a second time**. Report state is permanently `REPORTED_PENDING_INDEPENDENT_VERIFICATION`.

**Database guards:** immutable evidence; tenant-scoped RLS and restricted `SELECT, INSERT`; a deferred coverage assertion ensures the sum of allocations equals reported consumption; a locking allocation trigger enforces the signed issue upper bound and verifies source order/cut/SKU/shade independently of the HTTP handler.

### Secure API: `GET /api/v1/cutting/bundle-scans/challenge`

Query `draft_id`, `label_code`, and `epc_hex` with `Authorization: Bearer <external strong JWT>` for action `cut.bundle.scan.prepare`, role `cutting_reader`. Returns a SHA-256 challenge binding tenant, plant, spread report, immutable draft snapshot, exact label, and EPC. Material API environment must be enabled and trusted IdP configured. Every associated issued roll must match the asserted plant.

### Secure API: `POST /api/v1/cutting/bundle-scans`

Bearer assertion action `cut.bundle.scan.submit` role `cutting_reader`; JSON `draft_id`, `label_code`, `epc_hex`, `witness_attestation`. Witness attestation requires action `cut.bundle.scan.witness`, role `cutting_reader`, FIDO2/WebAuthn, and the exact prepared challenge. Witness subject/tenant/plant must match authorized actor. Unique proof JTI prevents replay; unique EPC and label prevent duplicate binding. Response state is `SIGNED_HUMAN_WITNESS_NOT_HARDWARE_ATTESTED`, with `production_released: false`.

This only supports a human-attested scan claim until a **trusted RFID reader gateway** provides cryptographically authenticated hardware observation and timestamp proof in the next cutting increment. Do not treat a typed EPC or an external IdP human assertion as an independent RF read.

## Status and intended continuation

- **Completed in 08B code:** issue-backed spread reporting, consumption conservation, append-only report history, immutable bundle identity alignment, secured witness ingestion, shared EPC collision checks, customer cutting screen.
- **Not completed:** independently certified actual spread/ply evidence, reader-originated RF evidence, actual cutting release or sewing handoff, per-piece QC/count confirmation, verified production pilot.
- **Next:** Batch 08C physical-reader ingestion and signed cutting release, then Batch 09 station sewing operations with typed sewing credit and offline recovery.
