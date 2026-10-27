# Engineering installation: cutting reader gateway & sewing handoff (08C)

Target **MacBook only**, `~/agiparel`, `http://localhost:18530/cutting`. Prerequisite: 08B and migration `0020` (confirmed passed in MacBook output). The iMac at `agiparel.empowerai.us` remains unaffected.

## 1. Download and apply source

```bash
cd ~/agiparel
git status --short
unzip -o ~/Downloads/agiparel_batch08c_reader_gateway_sewing_release.zip -d ~/agiparel
```

The ZIP contains changed files only. It does not reset the database or touch other Docker services/volumes. If you have independently edited these same files, compare changes before unzipping.

## 2. Apply migration 0021 (exactly once)

```bash
docker exec -i agiparel-postgres \
  psql -X -U agiparel_admin -d agiparel_dev -v ON_ERROR_STOP=1 \
  < services/core-api/migrations/0021_cutting_reader_events_and_release_v8.sql
```

Expected last line: `COMMIT`.

Check:

```bash
docker exec agiparel-postgres psql -X -U agiparel_admin -d agiparel_dev \
  -c "SELECT to_regclass('public.cut_reader_gateways_v8') AS gateways,
             to_regclass('public.cut_reader_events_v8') AS reader_events,
             to_regclass('public.cut_sewing_releases_v8') AS cutting_releases;"
```

All three should be non-null. This migration is **not idempotent**; do not rerun after success.

## 3. Test code before deployment

```bash
cd ~/agiparel
go test -count=1 ./... && go vet ./... && go build ./... && git diff --check
```

Additional opt-in **rollback-only PostgreSQL security checks** can verify migration RLS/permissions. They use the already-existing test convention; do **not** run against the iMac:

```bash
export AGIPAREL_TEST_ADMIN_DATABASE_URL='postgres://YOUR_ADMIN:YOUR_PASSWORD@127.0.0.1:15432/agiparel_dev?sslmode=disable'
AGIPAREL_CUT_SQL_TEST_OPT_IN=ROLLBACK_ONLY \
  go test -count=1 ./services/edge-daemon/cmd/server -run '^TestCuttingReaderSecurityPostgresRollbackOnly$' -v
unset AGIPAREL_TEST_ADMIN_DATABASE_URL
```

## 4. Deploy only MacBook Agiparel edge service

```bash
docker compose -f compose/docker-compose.dev.yml \
  up -d --build --no-deps --force-recreate plant-edge-os
```

Visit `http://localhost:18530/cutting`, choose a cutting lay, scroll to **Reader events & sewing handoff**. The workbench remains on HOLD until real material issuance, human RFID witnesses, enrolled hardware gateway reports and separate supervisor/quality signatures exist.

Also review `/stores`, `/documents`, `/buyer-review`, `/factory-setup`, `/material-planning` and `/order-intake`: no engineering sprint or batch numbers should be visible to the customer.

## 5. Acceptance checklist

- Source control diff contains 08C gateway ingress, signed release, migration 0021, website clean-up and tests; earlier migration files unchanged.
- Previous 08A/08B cutting tests and website tests pass.
- New Ed25519 tests reject invalid key, changed EPC, wrong station, wrong tenant, wrong event ID, stale timestamp and invalid tags.
- Browser read-only evidence page reports no spread/reader/witness/release truthfully on newly initialized DB.
- Gateway without out-of-band key enrollment gets `ERR_CUT_READER_NOT_ENROLLED` and cannot credit scans.
- Release endpoints return `ERR_MATERIAL_SECURE_API_DISABLED` until the existing strong IdP is provisioned; the shared `dev` operator is never an approver.
- If a physical reader and external IdP are not connected, **do not mark physical pilot validated**. No mock device, fictional signed witness, or synthetic release record may be promoted to real factory evidence.

## 6. Reader integrator contract and identity enrollment

See `docs/Agiparel_Batch08C_Reader_Gateway_And_Cutting_Release.md` for canonical Ed25519 JSON, gateway public-key enrollment, replay rules, signed approval scopes and limitations. The existing operator Basic gate must be replaced/supplemented with transport-level device authentication for hardware rollout; this batch does not introduce a new identity server.
