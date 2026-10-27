# Agiparel Batch 07A — Stores Roll Receipts and Inspection Passports

**Release baseline:** Uploaded `agiparel(2).zip` (contains Gate 06.5-C2). **Status:** Code ready for Mac acceptance; NOT production material authority.

## What the customer can do now

- Open `/stores`, add physical roll receipts, measured fabric length (exact 0.001 m), supplier lot, fabric SKU, shade band, usable width, rack/bin and defect observations.
- Optionally record a physical tag's 96-bit or longer uppercase-hex EPC. EPC-to-roll binding is persistent and unique per tenant. A tag bound to one roll cannot silently become another roll's tag.
- Link an original uploaded GRN, roll card, inspection record or similar to each observation through `/documents`.
- Reinspect/recount a roll without rewriting prior events. The UI shows every recorded revision and original-document links.
- Review measured-versus-declared fabric variance, protected stock ledger inspection status, protected stock remaining length, and the open reservation quantity, without granting this page any stock-mutating authority.

## Invariants

1. Source observations use tenant-isolated, append-only PostgreSQL tables `store_roll_passport_events_v7` and `store_roll_rfid_bindings_v7`.
2. A manual `PASS` result means the operator **reported** passing their observation. It is not an independently authenticated inspection. Every result has `evidence_class = OPERATOR_REPORTED`.
3. This release **never** writes `store_rolls`, `material_reservations_v5`, `material_issues_v5`, `material_write_scopes_v5`, `material_ceo_principals_v5`, or `material_variances_v5`.
4. Existing production reservation and two-witness issuance APIs stay disabled unless their separate Ed25519 IdP, signed scope/lease and identity restrictions are genuinely configured.
5. A roll can be recorded without a tag and receive an EPC later; a bound EPC cannot be reused or reassigned in this release.
6. Each revision is optimistic (`expected_revision`) and serialized per roll via transaction-level advisory lock. Concurrent EPC conflicts are prevented by a unique database constraint.
7. A document ID must be real and belong to the active tenant. Invalid numerical dimensions or unknown IDs are rejected instead of defaulted.
8. All document uploads still occur at the single `/documents` inbox, never at `/stores`.
9. The development tenant and operator remain `dev`, with local development password `1212`. This is not production operator identity.

## Routes

- `GET /stores` — restricted workspace.
- `GET /api/v1/stores/passports` — latest operator observations with a read-only comparison to the protected stock ledger.
- `GET /api/v1/stores/passports?roll_id=ROLL-1001` — immutable observation history for one roll.
- `POST /api/v1/stores/passports` — append one observed receipt or reinspection revision.
- `GET /api/v1/stores/authority` — tenant-scoped counts of protected stock entries, PASS inspections and open reservations; never a grant of authority.

## Batch 07 remainder

Batch 07B: integrate source-linked GRNs and authenticated, inspection-controlled promotion into `store_rolls`, safely reconciling identities, ownership and available millimetres. Batch 07C: operator website for reservation and two-witness material issuance using independently verified identities (never the shared development Basic password). CEO-only exception remains mandatory. Then Batch 08 cutting consumes physically issued material and creates classic sub-bundle genealogy.

**Caution:** Existing stock rows may be marked `PASS` from prior test fixtures. The UI distinguishes those protected rows from new unverified observations, and does not imply they are pilot-verified.
