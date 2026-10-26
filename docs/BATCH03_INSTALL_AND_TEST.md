# Agiparel Batch 03: development authority provisioning + signed HTTP/PG integration

Baseline: repository `cyberinQ/agiparel` commit `9675c19` (Batch 02 fixed). No existing files replaced and no migration needed. All fixtures are disposable developer records.

## Add three source files
- `services/edge-daemon/cmd/materialctl/main.go` — development-only privileged enrollment of CEO subjects and M3 writer leases. Requires explicit dev opt-in; **never** signs JWTs, approves variances or issues material.
- `services/edge-daemon/cmd/materialctl/main_test.go` — fail-closed configuration and validation tests.
- `services/edge-daemon/cmd/server/material_api_pg_integration_test.go` — real HTTP routes, ephemeral Ed25519 JWT signatures and restricted-role PostgreSQL. No private signing key is deployed.

## Install
From `~/agiparel`:

```bash
unzip -o ~/Downloads/agiparel_batch03_additive.zip -d ~/agiparel
gofmt -l services/edge-daemon/cmd/materialctl/*.go services/edge-daemon/cmd/server/material_api_pg_integration_test.go
go test -count=1 ./services/edge-daemon/cmd/materialctl
go test -count=1 ./services/edge-daemon/cmd/server
```

## Real PostgreSQL integration
Uses **same env setup as passed Batch 02**. No client `psql` required. Database must already have migrations 0001–0006. The test generates and cleans up unique tenant fixtures. The admin URL should be for `agiparel_admin`, app URL for `agiparel_app`; do not point at live operational data.

```bash
cd ~/agiparel
export AGIPAREL_TEST_ADMIN_DATABASE_URL="$(
  PG_SECRET="$(docker exec agiparel-postgres printenv POSTGRES_PASSWORD)" \
    python3 -c 'import os,urllib.parse; print("postgres://agiparel_admin:"+urllib.parse.quote(os.environ["PG_SECRET"],safe="")+"@127.0.0.1:15434/agiparel_dev?sslmode=disable")'
)"
export AGIPAREL_TEST_APP_DATABASE_URL="$(
  docker exec agiparel-edge-daemon printenv DATABASE_URL | sed 's/@postgres:5432/@127.0.0.1:15434/'
)"
export AGIPAREL_DESTRUCTIVE_DB_TESTS=I_UNDERSTAND_TEST_FIXTURES

go test -v -count=1 ./services/edge-daemon/cmd/server -run '^TestMaterialHTTPWithSignedIdentityAndPostgres$'
go test -v -count=1 ./services/edge-daemon/pkg/stores -run '^TestMaterialDurabilityWithPostgres$'

unset AGIPAREL_TEST_ADMIN_DATABASE_URL AGIPAREL_TEST_APP_DATABASE_URL AGIPAREL_DESTRUCTIVE_DB_TESTS
```

## Development provisioning CLI (optional, actual caller-enrolled IdP subjects required)

`materialctl` is **not needed to run its HTTP test**, because that test provisions and cleans unique admin fixtures. Only run it for your separate development tenant/order/roll after those entities exist, and after checking out-of-band the real CEO subject enrolled with the upstream IdP. Use `AGIPAREL_ADMIN_DATABASE_URL` set to the same admin DB URL; it is never printed. `--approved-by` is an audit label; not a cryptographic approval.

```bash
export AGIPAREL_ENVIRONMENT=development
export AGIPAREL_DEV_PROVISION=I_UNDERSTAND_FAKE_DATA
export AGIPAREL_ADMIN_DATABASE_URL='postgres://<admin>:<secret>@127.0.0.1:15434/agiparel_dev?sslmode=disable'

go run ./services/edge-daemon/cmd/materialctl scope \
  --tenant DEV-TENANT --order DEV-PO --plant DEV-PLANT \
  --holder edge-dev --approved-by bootstrap-admin --lease-duration 1h

go run ./services/edge-daemon/cmd/materialctl enroll-ceo \
  --tenant DEV-TENANT --subject externally-verified-ceo-subject \
  --approved-by bootstrap-admin

unset AGIPAREL_ADMIN_DATABASE_URL AGIPAREL_DEV_PROVISION
```

The matching runtime `AGIPAREL_MATERIAL_AUTHORITY_HOLDER` must equal the provisioned `--holder`. Leases expire and must be renewed. An external trusted WebAuthn/FIDO2-capable Ed25519 IdP integration is still required before enabling the real API. Keep `AGIPAREL_MATERIAL_API_ENABLED` unset/false until ready.

## Verification boundary
The ephemeral Ed25519 test signer simulates a correctly issuing IdP; it does not demonstrate a deployed, physically verified WebAuthn/FIDO2 authenticator or actual RFID scan. The tests exercise code paths and PostgreSQL persistence. This package does not alter Docker or run against real factory records.
