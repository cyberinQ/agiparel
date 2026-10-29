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


### F3.25 v38 material and integrity diagnostics (2026-10-29)

After migration `0057`, the opt-in `GET /api/v1/packing/dock/integrity-report` combines current dock evidence, garment-cut/WIP lineage and per-release human-classified fabric issue attribution. Set `AGIPAREL_DOCK_INTEGRITY_V38_ENABLED=true` only after upstream identity/external-gate provisioning. The report is **read only**; inventory, trim and financial conservation, physical RF, actual ASN delivery and shipment posting remain unverified. See `docs/F3_25_MATERIAL_ATTRIBUTION_OPERATOR_INTEGRITY_v38.md`.
