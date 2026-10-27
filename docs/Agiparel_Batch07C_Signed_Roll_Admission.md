# Agiparel Factory OS — Batch 07C: Signed Roll Admission

**Author:** CyberinQ LLC / Sunil Jain | **Basis:** customer-uploaded `agiparel(3).zip` (Batch 07B + migrations 0001–0016).

## Operational result

1. Stores operators record a roll passport with an immutable original document and RFID EPC identity in the Batch 07A screen. An EPC typed into the UI is a registration claim, not physical reader telemetry.
2. An operator raises an immutable reconciliation request (Batch 07B).
3. `/stores` displays **read-only** admission readiness: `NOT_READY`, `READY_FOR_SIGNED_REVIEW`, or `ADMITTED_TO_LEDGER`, with explicit blockers. This works with existing development Basic login.
4. **Only with an externally enrolled IdP and strong user authentication:** a real Stores Manager obtains the challenge for that precise request using `GET /api/v1/material/roll-admissions/challenge?request_id=...` with an independent manager bearer token asserting action `material.roll.admit.prepare`. The IdP produces two FIDO2/WebAuthn-backed, fresh, Ed25519 signed JWTs (manager and *different* inspector) with action `material.roll.admit.witness` and the exact challenge hash.
5. The manager submits the two attestations to `POST /api/v1/material/roll-admissions` with their manager bearer token asserting action `material.roll.admit.submit`. The server independently checks external signature, role, subject, tenant, plant, freshness, action and challenge for both witnesses, then atomically **inserts a NEW** `store_rolls` row, immutable `store_roll_admissions_v7` audit record, unique witness-proof records, and outbox event. There is **no** endpoint to overwrite a pre-existing ledger row or adjust an existing stock balance.

## Trust / boundaries

- This gate does **not** delegate CEO exception approval, material issuance, purchase/order reservations, or legacy writer fencing. Those retain the existing separately scoped authenticated material APIs. In particular, `dev / 1212` cannot sign an admission; IdP signing credentials are **not** minted by Agiparel.
- `AGIPAREL_MATERIAL_API_ENABLED` must explicitly be `true`, and external public key, key ID, issuer, audience and material authority holder must be configured or all material roll-admission writes return HTTP **503**.
- The manager and inspector must have different signed subjects and proof IDs, be authenticated with WebAuthn/FIDO2, and both be in the same tenant and plant as the submitting manager. The canonical SHA-256 challenge covers tenant, plant, request ID, passport revision, assessment hash, SKU, lot, shade, bin, EPC, source document, length, width, and defects.
- A PASS entered by a shared operator is not sufficient. Admission also requires latest revision, immutable request snapshot matching the source, a nonblank document linked in the same tenant, EPC binding, and absent authoritative ledger roll. Verification *depends on the enrolled humans physically checking that evidence*; it is not an automated physical inspection or RFID reader measurement.
- Ledger admission uses exact integer-mm to NUMERIC metres conversion, preserving three decimal places. `usable_width_cm` is from width mm divided by 10 (two decimal places).
- Admission audit tables have tenant RLS, INSERT/SELECT-only app grants, and hard immutable triggers. Duplicate admission per request or roll, and proof-ID reuse across roles within this admission system, are rejected by UNIQUE constraints. Outbox publication remains the established outbox responsibility.
- Migration 0017 also prevents **new** cross-order reservations of the same physical roll, including where older reservations or issues exist. It does not rewrite historical records or treat two orders' materials as fungible.

## Routes added

| API | Auth | Effect |
|---|---|---|
| `GET /api/v1/stores/admissions` | Existing restricted operator access | List 100 recent review requests, readiness, reasons, recorded admission time; **no writes** |
| `GET /api/v1/material/roll-admissions/challenge?request_id=UUID` | External signed bearer, strong-auth `stores_manager`, action `material.roll.admit.prepare` | Returns challenge hash for a currently eligible request |
| `POST /api/v1/material/roll-admissions` | External signed bearer `stores_manager`, action `material.roll.admit.submit`; two distinct signed `material.roll.admit.witness` proofs | One atomic **new-roll** admission only |

POST body:
```json
{
  "request_id": "12345678-1234-1234-1234-123456789abc",
  "manager_attestation": "<fresh-externally-signed-manager-JWT>",
  "inspector_attestation": "<fresh-externally-signed-inspector-JWT>"
}
```
Do not use sample JWTs or invented actors. An external IdP must issue the attestations; no developer bypass is provided. Retry after a completed admission will not create more stock; API rejects already-admitted requests rather than silently duplicating them. Review completed admission via the read-only Stores queue.

## Database

Migration `0017_stores_signed_roll_admission_v7.sql` creates `store_roll_admissions_v7`, `store_roll_admission_proofs_v7`, append-only triggers and a single-order-per-roll reservation guard. It **requires 0001–0016 applied**. No reset. No changes to pre-existing ledgers.

## Important limitations

- This code needs a working external human identity provider to **exercise actual signed approval**. The development Compose file intentionally does not configure one; readiness UI works but physical stock admissions remain disabled. That is expected and safe.
- No hardware RFID reader telemetry or calibrated length-measuring instrument is connected by this patch.
- Only **new roll initial admission** is implemented; existing-roll loss/surplus adjustments still need the separate CEO-authorized variance process. Secure issuance already exists but requires independent IdP custody witnesses and a valid scope lease.
- Current baseline `store_rolls.id` is globally unique across tenants; avoid reusing the same roll IDs across tenant factories until schema scope is tightened in a separately staged, backward-compatible migration.
- PostgreSQL and end-to-end MacBook tests require the user's local Docker PostgreSQL and Go 1.24. Container-side verification here is isolated Go stdlib / web UI only; do not represent the real factory or external IdP as validated.
