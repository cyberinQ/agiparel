# Agiparel — Batch 06 material planning: installation, acceptance, scope

**Baseline:** Batch 05 installed and running on the Agiparel iMac. This delivery is **12 changes/additions only**, over the Batch 05 source; not a whole repository. Earlier Batch 04/05 files may remain uncommitted. Do not apply both the ZIP and the patch.

## Delivered business capability

- New **restricted** `/material-planning` website and `GET/POST /api/v1/material-scenario` API, linked from the homepage and `/order-intake`.
- Source from one immutable, tenant-scoped, staged Batch 05 buyer PO + BOM, **not** an invented order.
- Upload candidate fabric surplus CSV and supplier unit-price CSV. Evidence remains explicitly `OPERATOR_SUBMITTED_CSV_UNVERIFIED`; original files stored byte-for-byte with the calculated SHA-256 input digest.
- Six-dimensional fabric scenario check: exact spec + operator-submitted GSM tolerance, shade delta-E, usable versus marker width, inspection status, age versus limit, buyer escrow + non-borrowing. Failed/unknown claims **never** reduce proposed indent. A zero-surplus header-only CSV is supported.
- Deterministic exact fixed-point quantities and price extensions (four decimal places; HALF UP); no floating-point money or margin fabrication.
- For factory-owned inputs: **gross BOM need – conditionally matched surplus = net factory indent**, clamped to zero. Buyer-owned materials are instead a **buyer supply gap** and **never factory purchase cash**.
- Immutable PostgreSQL scenarios, idempotent resubmission, per-tenant RLS, no runtime update/delete privilege, read-back and revision list; scenarios always `PROPOSED / HOLD`. No PO, roll reservation, buyer authorization, accepted price, complete cost quote, accounting transaction or production release.

## ADD (6 files)

- `services/core-api/migrations/0008_material_scenarios_v6.sql`
- `services/edge-daemon/cmd/server/material_scenario.go`
- `services/edge-daemon/cmd/server/material_scenario_test.go`
- `services/edge-daemon/internal/web/static/material_planning.html`
- `services/edge-daemon/internal/web/static/material_planning.css`
- `services/edge-daemon/internal/web/static/material_planning.js`

## REPLACE (6 files)

- `services/edge-daemon/cmd/server/main.go`
- `services/edge-daemon/internal/web/assets.go`
- `services/edge-daemon/internal/web/static/order_intake.html`
- `services/edge-daemon/internal/web/static/order_intake.css`
- `services/edge-daemon/internal/web/static/order_intake.js`
- `services/edge-daemon/internal/web/static/portal.html`

No `portal.go`, `portal_test.go`, compose file, older SQL migrations, existing physical transaction services, other Docker stacks or shared volumes are modified. Your dev login configuration remains yours.

## Install: recommended ZIP path

Download **`agiparel_batch06_changes.zip`** into Mac `~/Downloads`:

```sh
cd ~/agiparel
git status --short
unzip -l ~/Downloads/agiparel_batch06_changes.zip
unzip -o ~/Downloads/agiparel_batch06_changes.zip -d ~/agiparel
```

This may overwrite changes made *locally* to the six REPLACE files since Batch 05. If you've edited those files yourself, save/commit the changes before installing. Do **not** also `git apply` the alternative patch. Conversely, you may use the patch *instead* and stop if `git apply --check` fails.

Apply the **new** Agiparel-only PostgreSQL migration (no reset; PostgreSQL container already running):

```sh
docker exec -i agiparel-postgres \
  psql -U agiparel_admin -d agiparel_dev -v ON_ERROR_STOP=1 \
  < services/core-api/migrations/0008_material_scenarios_v6.sql
```

Expect a terminal `COMMIT`, not an error. This is a development migration, NOT a production schema-upgrade/rollback contract.

## Test and deploy

```sh
cd ~/agiparel
go test -count=1 ./services/edge-daemon/cmd/server
go test -count=1 ./...
go vet ./...
go build ./...
git diff --check
```

**Before recreating the container**: Docker Compose takes credentials from your *current shell*, not a previously created container. Preserve/enter your chosen development user and tenant:

```sh
export ENVIRONMENT=development
export AGIPAREL_OPERATOR_USER=dev
export AGIPAREL_OPERATOR_TENANT=DEV-TENANT
read -rs 'AGIPAREL_OPERATOR_PASSWORD?Development operator password: '
echo
export AGIPAREL_OPERATOR_PASSWORD

docker compose -f compose/docker-compose.dev.yml \
  up -d --build --no-deps --force-recreate plant-edge-os
```

For your locally modified four-character minimum, `1212` may be entered at the hidden prompt. The server-pinned tenant is `DEV-TENANT` (not the username `dev`); confirm it exists in Agiparel's `tenants` table. The existing temporary development Basic-auth barrier protects operational routes; do not mistake its existence for authorization of material movements or approved procurement.

Smoke tests:

```sh
curl -sS -o /dev/null -w 'home: %{http_code}\n' http://127.0.0.1:18530/
curl -sS -o /dev/null -w 'planning without login: %{http_code}\n' http://127.0.0.1:18530/material-planning
curl -sS -o /dev/null -w 'public home: %{http_code}\n' https://agiparel.empowerai.us/
```

Expected `200`, `401` if credentials configured, `200` respectively. For the new restricted app use `https://agiparel.empowerai.us/material-planning` and your existing development login. If the login is not configured, `503` is expected rather than `401`.

## Customer demonstration: four sample CSVs

Download **`agiparel_batch06_example_csvs.zip`** and unzip into a scratch directory, not your source tree:

```sh
mkdir -p /tmp/agiparel-batch06-examples
unzip -o ~/Downloads/agiparel_batch06_example_csvs.zip \
  -d /tmp/agiparel-batch06-examples
```

1. If you already staged the Batch 05 `PO-DEMO-01` sample, select it in `/material-planning`. Otherwise, visit `/order-intake` and stage files `01_sample_buyer_po.csv` and `02_sample_style_bom.csv` first.
2. Select the staged order in `/material-planning`; upload `03_sample_surplus_unverified.csv` and `04_sample_prices_unverified.csv`. Click **Evaluate six checks**, inspect each lot, then **Save immutable proposal**.
3. Click the resulting entry in **Scenario revision history**; it must display the same quantities after a page reload. Upload exactly the same inputs again; no duplicate row should be created.

**Expected SAMPLE output:**

| Metric | Expected |
| --- | ---: |
| Buyer PO | PO-DEMO-01 |
| Ordered garments | 600 |
| Gross cotton twill need | 1110.000 M |
| EXAMPLE-ROLL-101 conditionally matched | 500.000 M |
| EXAMPLE-ROLL-102 held for OTHER-PO | 0.000 M applied |
| Net fabric factory indent | 610.000 M |
| Net zipper factory indent | 600.000 PCS |
| Gross factory material exposure | USD 4915.5000 |
| Conditional fabric reuse value | USD 2125.0000 |
| Conditional net purchase exposure | USD 2790.5000 |
| Workflow authority | PROPOSED / HOLD |

These fabricated assertions are not inspection findings, buyer approval, available-to-promise warehouse stock, commercial quotations or cash commitments.

## Optional **real PostgreSQL** integration and RLS test

The test `TestMaterialScenarioPostgresPersistenceAndTenantRLS` is explicitly skipped without test URLs; ordinary `go test ./...` cannot prove it has run. It **inserts then removes disposable test tenants and rows**, so run it against **only your Agiparel dev database**:

```sh
export AGIPAREL_TEST_ADMIN_DATABASE_URL="$(
  PG_SECRET="$(docker exec agiparel-postgres printenv POSTGRES_PASSWORD)" \
  python3 -c 'import os,urllib.parse; print("postgres://agiparel_admin:"+urllib.parse.quote(os.environ["PG_SECRET"],safe="")+"@127.0.0.1:15434/agiparel_dev?sslmode=disable")'
)"
export AGIPAREL_TEST_APP_DATABASE_URL="$(
  docker exec agiparel-edge-daemon printenv DATABASE_URL \
  | sed 's/@postgres:5432/@127.0.0.1:15434/'
)"
export AGIPAREL_DESTRUCTIVE_DB_TESTS=I_UNDERSTAND_TEST_FIXTURES

go test -v -count=1 ./services/edge-daemon/cmd/server \
  -run '^TestMaterialScenarioPostgresPersistenceAndTenantRLS$'

unset AGIPAREL_TEST_ADMIN_DATABASE_URL AGIPAREL_TEST_APP_DATABASE_URL AGIPAREL_DESTRUCTIVE_DB_TESTS
```

The explicit test verifies idempotency, tenant RLS and that the restricted app role cannot overwrite stored proposals. Passing it requires `PASS` rather than `SKIP`.

## Release notes / limits

- This is a deterministic what-if calculation driven by operator-submitted evidence, not an authoritative stores netting gate. Until Batch 07 has independently inspected/verified stock and concurrency-safe reservations, no physical surplus is claimable as actual stock.
- Real price offers, supplier ETA, marker CAD provenance, buyer tolerance authority and consumption revision approval are NOT verified here.
- The four canonical Part 1 costing models, Factory SAM/CMT operations, margins, quote authority and full commercial cost waterfall remain future business gates. The cost values shown are material-only indicative prices, with buyer-owned items excluded from factory procurement cash.
- New controls do not bypass the earlier CEO-only exception or double-witness material transaction paths.
- **Local validation performed by assistant**: `gofmt`, isolated Go standard-library harness unit tests (includes all six criterion failures, price/decimal errors and buyer-owned exclusion), `go vet` harness, JS syntax checks, generated Git patch applies cleanly to Batch 05 baseline and yields byte-identical ZIP files. Full repository Go 1.24 test and live PostgreSQL execution must be performed on your iMac.
