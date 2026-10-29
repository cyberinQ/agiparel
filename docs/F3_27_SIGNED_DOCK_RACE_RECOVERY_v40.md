# F3.27 v40 — Signed dock admissions and resilience

Author: Sunil Jain · 2026-10-29

Baseline: `54dc176`. Migration: none (v38 `0057` is current).

## Changes

* Signed gateway scans and externally signed carrier/buyer assertions now set PostgreSQL `lock_timeout=2000ms` and `statement_timeout=12000ms` *inside* their admission transaction; the HTTP request still has its independent 15-second deadline. A timeout fails without generating custody or shipment credit.
* Additional Go tests verify full signed-field binding, cross-domain replay rejection, age boundaries, and strict bounded JSON envelopes.
* The v40 disposable-database drill launches 4–48 simultaneous synthetic gateway inserts for the *same* carton and simultaneous carrier assertions for the *same* manifest; verifies exactly one success, gateway-event replay rejection, tenant-specific visibility and rollback when PostgreSQL kills an uncommitted external-assertion writer. The default is 12 workers.
* Schema test asserts both uniqueness boundaries, enabled immutability triggers, provisioner-only trust-root grants.
* CI executes the drill on the ephemeral PostgreSQL-16 service; the local installer uses only a restored disposable database.

## Critical boundaries

The stress drill uses `session_replication_role=replica` inside isolated transactions to bypass *device* evidence triggers while testing **real database uniqueness and WAL behavior**. It is NOT a test of actual cryptographic device enrollment under load, RF reliability, customer delivery, physical possession, financial conservation or legal buyer acceptance.

No shipment posting, stock credit, carrier delivery or buyer approval is enabled by this release. All production feature switches remain disabled by default. Do not run the drill on real factory data; the script enforces a disposable database name and opt-in guard.

## Outstanding production readiness

The remaining acceptance campaign must cover full signed-workflow synthetic integration with a realistic enrollment fixture, authenticated multi-factory load, device commissioning, actual carrier/EDI transport receipts, full stock/cost accounting and independently reviewed factory UAT. A green CI check alone cannot substitute for these proofs.
