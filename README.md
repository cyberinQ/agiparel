<!-- Author: Sunil Jain, CyberinQ LLC, 2026-10-29. -->
# Agiparel Factory OS

Go 1.24 / PostgreSQL 16 / Redis / Docker-based garment manufacturing and RFID evidence platform. Physical hardware, buyer approval and shipment credit are **not** inferred from software tests.

## Development

From this repository root:

```sh
docker compose -f compose/docker-compose.dev.yml up -d --build
# Restricted operator app: http://localhost:18530
go test -count=1 ./...
go test -race -count=1 ./...
go vet ./...
go build ./...
for script in scripts/test_*.cjs; do node "$script"; done
```

## Production acceptance

Start with [Production Readiness Plan](docs/Production_Readiness_Plan.md), then [F3.20B–D dock and F3.21 engineering controls](docs/F3_20BCD_F3_21_DOCK_AND_OPERATIONS_20261029.md). New production controls are disabled by default. Migration 0054 follows 0053. See [F3.22 external counterparty assertions and carton reconciliation](docs/F3_22_EXTERNAL_ACK_CARTON_RECONCILIATION_20261029.md). Complete identity/tenant, physical evidence, concurrency, recovery and onsite acceptance gates before authorizing real shipments.

F3.23 v36: spec-bound buyer authorization, exact dock carton reconciliation and read-only diagnostics; see `docs/F3_23_BUYER_CONTRACT_CARTON_DIAGNOSTICS_v36.md`. No shipment posting.

### F3.24 v37 — live cut/WIP lineage audit (read-only)

New `GET /api/v1/packing/dock/lineage-audit` reuses the existing tenant/plant-scoped workforce `packing.read` grant and repeatable-read transaction. Requires both v35 external gate and `AGIPAREL_DOCK_LINEAGE_AUDIT_V37_ENABLED=true`. It conservatively checks packed identities, signed cut reconciliation, latest effective-part/QC evidence, piece holds and jobwork states, with bounded manifest size and named blockers. It is **not** global material/trim/financial conservation, ASN delivery, shipment posting or physical acceptance. See `docs/F3_24_LINEAGE_AUDIT_v37.md`.


### F3.25 v38 material and integrity diagnostics (2026-10-29)

After migration `0057`, the opt-in `GET /api/v1/packing/dock/integrity-report` combines current dock evidence, garment-cut/WIP lineage and per-release human-classified fabric issue attribution. Set `AGIPAREL_DOCK_INTEGRITY_V38_ENABLED=true` only after upstream identity/external-gate provisioning. The report is **read only**; inventory, trim and financial conservation, physical RF, actual ASN delivery and shipment posting remain unverified. See `docs/F3_25_MATERIAL_ATTRIBUTION_OPERATOR_INTEGRITY_v38.md`.


### F3.26 v39 database race and recovery verification (2026-10-29)

No migration: existing migrations 0041–0057 contain the relevant packing uniqueness and tenant-isolation controls. v39 adds concurrent double-pack prevention tests against the **actual** partial unique index, PostgreSQL backend-termination rollback verification, separate-tenant key namespace tests, trust-root privilege checks, bounded dock-report SQL execution, and production dock verification flags (all default false). It also removes duplicate CI checks and wires drills into schema/release jobs. Test harness uses `session_replication_role=replica` under an isolated superuser solely to inject test records without signing; this verifies **database uniqueness and rollback only**, not end-to-end signed transaction semantics. Never run against a live or shared database. See `docs/F3_26_CONCURRENCY_RECOVERY_v39.md`.

### F3.27 v40 signed dock admission hardening

The v40 release adds database-side 2-second lock and 12-second statement limits to signed gateway and external-acknowledgement writes, an adversarial Go envelope/signature suite, and a scratch-only 12-worker PostgreSQL concurrency/replay/crash drill. It does not change migrations or authorize stock/shipment postings. See `docs/F3_27_SIGNED_DOCK_RACE_RECOVERY_v40.md` for evidence boundaries and commissioning gaps.


### F3.28 v41 — Production trust isolation

Sewing production is opt-in (disabled by default). See `docs/F3_28_V41_PRODUCTION_TRUST_ISOLATION.md` for forced-RLS privilege checks, parallel app-role tenant isolation, key revocation and release limits. No migration or shipment posting.

## v42 database baseline (development only)

The canonical PostgreSQL 16 schema is `services/core-api/migrations/0001_v42_baseline.sql` (generated and equivalence-tested on the development host at v42 cutover). Bootstrap credentials live in ignored `.env`; the app/worker role SQL is `scripts/bootstrap_roles_v42.sql`. Historical migrations and release notes are compressed source fixtures, **not** execution inputs. The automated v42 cutover requires explicit destructive-reset consent. See `docs/V42_CANONICAL_DATABASE_AND_REPO_CLEANUP.md`.

## v43 public Factory Home and engineer operations

The public home at `https://agiparel.empowerai.us/` includes an optional three-question explanation of the product. Restricted `/engineer-ops` provides a live read-only operational overview after independent admin authentication. The website must be deployed to the host behind its TLS reverse proxy; a localhost Docker health check alone does not establish live publication.

For the v43 patch, deployment gate and platform-access boundaries see [V43 Public Demo and Engineer Ops](docs/V43_PUBLIC_DEMO_AND_ENGINEER_OPS.md). No schema migration or shipment authority is introduced.
