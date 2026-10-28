<!-- Author: Sunil Jain, 2026-10-28 -->
# Edge daemon public package consolidation

## Source changes

- Canonical packages now live solely in `services/edge-daemon/pkg/`.
- Deleted the redundant root `pkg/` tree; the existing edge package implementations were preserved.
- Moved **all** `internal/ocr/*.go` and `internal/packing/*.go` to `pkg/ocr/` and `pkg/packing/` unchanged.
- Updated `cmd/server/main.go` imports to their new public Go import paths.
- Removed the now-invalid root `pkg/` Docker `COPY` instruction.
- Added external integration tests to guard Go public import accessibility and the OCR/packing behavior.

The public import paths are now:

- `github.com/cyberinq/agiparel/services/edge-daemon/pkg/ocr`
- `github.com/cyberinq/agiparel/services/edge-daemon/pkg/packing`

External downstream Go modules that imported the old `.../internal/ocr` or `.../internal/packing` paths must update their imports; the Go API and logic were not changed.

## Migration

**No SQL/data migration.** This is a source/package relocation only. Do not reset the database for this change.

## Test and build (Go 1.24 required)

```sh
# Author: Sunil Jain, 2026-10-28
# Validate import visibility, test behavior, vet and compile from repo root.
go test -count=1 ./...
go test -race -count=1 ./...
go vet ./...
go build ./...
docker compose -f compose/docker-compose.dev.yml build plant-edge-os document-intake
```

## Deploy (only after all checks pass)

```sh
# Author: Sunil Jain, 2026-10-28
# Recreate both services using their newly built images.
docker compose -f compose/docker-compose.dev.yml up -d --no-deps plant-edge-os document-intake
```

Deployment requires your configured target machine, Docker/Compose and service credentials. These commands do not reset Postgres or any persistent runtime data.
