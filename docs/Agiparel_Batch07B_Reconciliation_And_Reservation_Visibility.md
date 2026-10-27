# Agiparel Batch 07B — Reconciliation Workbench and Signed-Ledger Visibility

**Status:** Development implementation; roll observation → review-request workflow, not authoritative issuance.

## Business outcome

Factory stores can compare a physical roll passport against the distinct protected stock ledger. Agiparel calls out missing ledger entries, reported HOLD/REJECTED observations, material/SKU and shade mismatches, width mismatches, ledger inspection/stock-state restrictions, overcommitment, and differences between today's observed measured length and the historical recorded original length. It does **not** equate remaining stock to original receipt length (issued fabric explains some differences).

The operator can write an immutable, source-linked request for independent reconciliation. The request is *not* an inspection sign-off or adjustment. The new Stores page also shows secure material reservations with exact reserved, issued, released and outstanding millimetres. No new reservation or material issue is generated from the development login.

## Added interfaces

| Method and route | Data | Privilege |
|---|---|---|
| `GET /api/v1/stores/reconciliation` | Up to 200 latest roll observations, compared with ledger and classified | Read-only |
| `POST /api/v1/stores/reconciliation` | Immutable review request, optimistic passport revision and idempotency key | Review queue only |
| `GET /api/v1/stores/reconciliation-requests` | Up to 100 stored review snapshots, newest first | Read-only |
| `GET /api/v1/stores/reservations` | Up to 200 real signed-material reservation balances | Read-only |

The existing `GET/POST /api/v1/stores/passports` and secure signed `/api/v1/material/*` APIs remain unchanged.

## Data model and rules

Migration `0016_stores_reconciliation_requests_v7.sql` adds a single append-only table with `(tenant_id,id)` and `(tenant_id,request_key)` uniqueness, a tenant/passport-revision foreign key, and a source comparison snapshot as typed JSONB with SHA-256 digest. The application's PostgreSQL role is granted **SELECT/INSERT only**, with FORCE RLS; no permission is granted to modify stock, reservations, issues or CEO controls. Request status is fixed to `PENDING_INDEPENDENT_RECONCILIATION`. Every request is tied to the source passport revision; newer observations require a new request key and a new review.

The backend reads ledger rows from `store_rolls`, `material_reservations_v5`, and `material_issues_v5` using the server-pinned development tenant `dev`. A 12–1000-character note is required. Reused idempotency keys with changed inputs fail with `409`, as do stale passport revisions. Roll-level advisory locks serialize against new B7A revisions.

## Trust boundary and work remaining

- An operator-reported PASS does not set the stock ledger to PASS.
- A matched observation is still labeled `AWAITING_SECURE_RECONCILIATION`.
- A review request **never** reserves, issues, transfers, releases or adjusts fabric.
- The Batches 02/03 signed material APIs continue to require properly configured IdP, authority scopes, independent issue witnesses and CEO-only exceptions.
- A complete authenticated inspection-to-ledger promotion mechanism is a separate forthcoming Batch 07C step; do not bypass it with the shared development login.
- Physical RFID reader commissioning and first-principles measurement remain physical pilot acceptance criteria.

## Acceptance examples

1. Passport ROLL-1001 with 98.750 m measured, no ledger row → `NOT_IN_STOCK_LEDGER`, open review case, stock remains unchanged.
2. Ledger original 100 m and measured 98.750 m → `MEASURED_VS_ORIGINAL_REVIEW`, not automatically labeled loss/theft. Existing issues displayed separately.
3. Historical ledger 100 m original, 75 m remaining after authenticated issue; same 100 m original observation → no false length-conflict based on remaining length.
4. Inspection HOLD in observation or UNVERIFIED in ledger → explicit review flags.
5. Duplicate review request with identical key and inputs returns existing record; stale version / changed request re-use denied.
6. Second tenant's requests and roll observations do not appear in the first tenant.
7. `agiparel_app` UPDATE/DELETE on reconciliation requests is denied; review cannot modify `store_rolls`.
