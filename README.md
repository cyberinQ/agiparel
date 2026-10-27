# Agiparel Factory OS

Go 1.24 / PostgreSQL 16 / Redis / Docker-based garment manufacturing and RFID-emulation platform.

## Development

From this repository root:

```sh
docker compose -f compose/docker-compose.dev.yml up -d --build
# Restricted operator app: http://localhost:18530

go test -count=1 ./...
go vet ./...
go build ./...
```

On a fresh PostgreSQL volume, the Compose initialization scripts create schema migrations 0001–0003.
Run **remaining** SQL migrations `0004`–`0025` in ascending order for a brand-new database.
For an existing database, apply only missing migrations; there is no migration ledger.

For an intentionally empty Agiparel development DB rebuilt through all existing migrations, use `scripts/reset_dev_database.sh --reset-agiparel-dev` (destroys all Agiparel DB contents; does not touch other Docker services or databases).

## Structure

- `services/` — API, sewing, cutting, stores, quality, packing, document intake, and web UI
- `services/core-api/migrations/` — complete ordered PostgreSQL schema history
- `pkg/`, `scripts/`, `tests/` — shared code, tooling, regression tests
- `tests/fixtures/intake/`, `examples/` — deliberate test fixtures (never production inventory)
- `docs/` — current v5.3 master requirements, epics and implementation plan
- `compose/` — development stack; `data/` and `backups/` are runtime-only, ignored

User-interface button scans are explicitly **simulated**; they do not constitute physical RFID evidence or production-authoritative releases. The default Compose credentials are development-only and must not be used for real customer data.
