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
