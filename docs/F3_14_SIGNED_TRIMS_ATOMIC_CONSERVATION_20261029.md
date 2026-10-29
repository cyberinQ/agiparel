# F3.14 — Signed trims inventory conservation

Author: Sunil Jain, CyberinQ LLC — 2026-10-29.

## Production contract

Migration **0043** is additive. Existing trims issuance records remain untouched; **legacy remaining balances are not automatically reconciled**. Material stock must be independently audited if historical issuance rows exist. Preflight aborts if a recorded balance exceeds its receipt.

- `POST /api/v1/material/trims/issues/challenge`: signed `material.issue.prepare` / `stores_issuer` operator generates SHA-256 challenge, binding tenant, plant, trim stock, released work order, line, integer quantity in **thousandths of the trim UOM**, and unique command ID.
- `POST /api/v1/material/trims/issues`: strong `material.issue.submit` authorization and two fresh `material.issue.witness` EdDSA assertions, issuer `stores_issuer`, recipient `stores_recipient`, signing exactly the same challenge; no actor IDs accepted from untrusted body.
- The issuer must match the signed submitting operator; recipient must be another human with a separate proof. The repository checks both provisioner-managed grants `stores.trims.issue` and `stores.trims.receive` for exact tenant, plant, IdP issuer and subject. The migration **does not grant them**.
- The serializable PostgreSQL transaction inserts the immutable issuance row. Its trusted trigger locks the lot row, verifies plant, released order and line, blocks HOLD/REJECTED/DEPLETED lots and overissue, consumes two single-use proof IDs, and decreases the remaining stock within the same commit. Unique command IDs make identical authenticated retries idempotent.
- `store_trims_v13` direct INSERT and UPDATE are revoked for `agiparel_app`; migration-owned trigger writes balances. A future authenticated trims-receipt API is needed to admit new lots without a privileged provisioning operation. The trigger independently checks evidence fields and invariants; **cryptographic identity is verified in the Go service**, not by SQL alone. A compromised application DB credential could still forge assertion hashes, so protect and rotate that credential and isolate direct DB access.
- Existing F3.10 exception and F3.3 material WIP controls remain unchanged. Trim issue is not garment completion, wage credit, or certified calibration.

## Deployment / rollback

1. Confirm F3.10 migration 0042, current F3.13/F3.15 baseline `fdac0cd`, clean tree and backup PostgreSQL with `pg_dump -Fc`.
2. Run `0043_trims_atomic_signed_issuance_v24.sql` once with `ON_ERROR_STOP=1`; run `scripts/test_trims_f314_schema_pg.sql` (read-only).
3. Run `go test -count=1 ./...`, `go test -race -count=1 ./...`, `go vet ./...`, `go build ./...`, all `scripts/test_*.cjs`, Docker development build/up.
4. Provision two distinct factory workers for the new actions. Material service remains disabled unless explicitly configured (`AGIPAREL_MATERIAL_API_ENABLED=true` plus pinned Ed25519 IdP and authority holder). Do not insert artificial proofs into live tables.
5. Commission on a **disposable PostgreSQL 16 clone** with synthetic lots: concurrent over-issue, zero-stock boundary, wrong plant, wrong order line, rejected lot, same-person and proof-ID reuse, command collision, idempotent retry, restart. Do not run synthetic stock mutations on a real factory database.

**Caution:** A database backup alone does not reverse writes made after migration. Rollback requires coordinated service shutdown and a planned forward corrective migration or restoring a disposable database. Never drop a proof ledger to hide committed inventory events.
