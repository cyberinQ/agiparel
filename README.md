<!-- Author: Sunil Jain, CyberinQ LLC -->
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