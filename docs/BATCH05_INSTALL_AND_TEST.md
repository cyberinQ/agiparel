# Agiparel Batch 05 — Source-backed order readiness

**Baseline:** Batch 04 portal and operator gate are installed; Batch 03 `materialctl` and PostgreSQL migrations 0001–0006 exist. The patch is generated relative to the **Batch 04 installed source** (not the old GitHub HEAD). It does **not** modify your previously uncommitted V5.1 backlog.

## Business delivery

- New restricted `/order-intake` HTML workbench, linked from the public Factory OS homepage (`/`). The public site **never reads/uploads source records**; it only describes the workflow and links to the operator login.
- `POST /api/v1/order-intake?action=preview`: validate strict CSV templates and compute the color/size matrix, total garments, per-material required quantity; **no DB writes**.
- `POST /api/v1/order-intake?action=stage`: stage original, unmodified buyer PO and BOM CSV bytes plus source SHA-256 and validated facts in tenant-isolated PostgreSQL. A duplicate upload returns the same receipt and does not create duplicate rows. All data remains `PROPOSED`, `HOLD`.
- `GET /api/v1/order-intake` shows up to 30 recent tenant-scoped intake receipts; `GET /api/v1/order-intake?id=OI-...` shows a source-derived matrix and BOM requirements. Original file bytes never leave via these GET endpoints.
- Fixed server-configured tenant `AGIPAREL_OPERATOR_TENANT`, used only when `ENV=development`. Do not accept tenant from client headers or query parameters. Existing Basic barrier is development only.

**This is structured CSV capture, NOT comprehensive tech-pack/OCR/AI ingestion, authoritative buyer approval, commercial costing, actual inventory availability, or released production.** The pre-existing OCR ingestion remains separate. We intentionally prioritize a deterministic, verifiable vertical slice over speculative AI extraction.

## Changed files

ADD:

- `services/core-api/migrations/0007_structured_order_intake_v5.sql`
- `services/edge-daemon/cmd/server/order_intake.go`
- `services/edge-daemon/cmd/server/order_intake_test.go`
- `services/edge-daemon/internal/web/static/order_intake.html`
- `services/edge-daemon/internal/web/static/order_intake.css`
- `services/edge-daemon/internal/web/static/order_intake.js`

MODIFY:

- `services/edge-daemon/cmd/server/main.go`
- `services/edge-daemon/internal/web/assets.go`
- `services/edge-daemon/internal/web/static/portal.html`
- `services/edge-daemon/internal/web/static/portal.css`
- `compose/docker-compose.dev.yml`

## Installation

You should already have the Batch 04 working tree. **Never unzip an entire old repository over current code.** Use one of the following options (not both):

Option A: download `BATCH05_source_backed_order_readiness.patch` into `~/Downloads`. From `~/agiparel`:

```sh
ls -lh ~/Downloads/BATCH05_source_backed_order_readiness.patch
git apply --check ~/Downloads/BATCH05_source_backed_order_readiness.patch
git apply ~/Downloads/BATCH05_source_backed_order_readiness.patch
```

If `--check` fails, STOP. Do not use `--3way`, `--reject` or `--force`; share `git status --short`, relevant file diffs and the error.

Option B: download `agiparel_batch05_changes.zip` into `~/Downloads`. It contains only the 11 files above in correct repo paths:

```sh
cd ~/agiparel
unzip -l ~/Downloads/agiparel_batch05_changes.zip
unzip -o ~/Downloads/agiparel_batch05_changes.zip -d ~/agiparel
```

**Check that the imported files exist**:

```sh
test -f services/edge-daemon/cmd/server/order_intake.go
test -f services/edge-daemon/internal/web/static/order_intake.html
test -f services/core-api/migrations/0007_structured_order_intake_v5.sql
```

## Database initialization: only Agiparel Dev PostgreSQL

Use the existing PostgreSQL Docker container; no Mac `psql` installation is necessary. Apply ONLY script 0007 after existing 0001–0006:

```sh
cd ~/agiparel
docker exec -i agiparel-postgres \
  psql -U agiparel_admin -d agiparel_dev -v ON_ERROR_STOP=1 \
  < services/core-api/migrations/0007_structured_order_intake_v5.sql
```

Expected ending: `COMMIT` (not an error). Do **not** reset database; no material transaction schema changes in this batch. Replaying script 0007 is supported for development setup but is not a migration/upgrade tool for production.

## Go tests and optional real-Postgres integration

```sh
cd ~/agiparel
gofmt -l services/edge-daemon/cmd/server/order_intake*.go
go test -count=1 ./services/edge-daemon/cmd/server
go test -count=1 ./...
go vet ./...
go build ./...
git diff --check
```

The `TestOrderIntakePreviewAndPostgres` test skips unless the SAME existing Batch 02/03 test environment vars are set:

```sh
export AGIPAREL_TEST_ADMIN_DATABASE_URL="$(
  PG_SECRET="$(docker exec agiparel-postgres printenv POSTGRES_PASSWORD)" \
  python3 -c 'import os,urllib.parse; print("postgres://agiparel_admin:"+urllib.parse.quote(os.environ["PG_SECRET"],safe="")+"@127.0.0.1:15434/agiparel_dev?sslmode=disable")'
)"
export AGIPAREL_TEST_APP_DATABASE_URL="$(
  docker exec agiparel-edge-daemon printenv DATABASE_URL | sed 's/@postgres:5432/@127.0.0.1:15434/'
)"
export AGIPAREL_DESTRUCTIVE_DB_TESTS=I_UNDERSTAND_TEST_FIXTURES

go test -v -count=1 ./services/edge-daemon/cmd/server \
  -run '^TestOrderIntakePreviewAndPostgres$'

unset AGIPAREL_TEST_ADMIN_DATABASE_URL AGIPAREL_TEST_APP_DATABASE_URL AGIPAREL_DESTRUCTIVE_DB_TESTS
```

Expected: PASS, not SKIP. This test checks preview without DB writes, stage, duplicate replay, retained original bytes, read-back material arithmetic, cross-tenant isolation, and a failed attempt to modify source evidence with the runtime DB role. The database contains only disposable test records.

## Development-only operator access

The public Factory OS remains open; `/order-intake` and its API **require the existing Batch 04 operator gate**. On a development environment, configure the gate **and pin the tenant** before starting or recreating the container.

```sh
cd ~/agiparel
export ENVIRONMENT=development
export AGIPAREL_OPERATOR_TENANT=DEV-TENANT
export AGIPAREL_OPERATOR_USER=dev-operator
# To avoid a plaintext password in shell history:
read -rs 'AGIPAREL_OPERATOR_PASSWORD?Development operator password (>=16 chars): '
echo
export AGIPAREL_OPERATOR_PASSWORD

docker compose -f compose/docker-compose.dev.yml \
  up -d --build --no-deps --force-recreate plant-edge-os
```

The sample `DEV-TENANT` exists from your Batch 03 provisioning. If it doesn't exist, use an existing development tenant, or create a disposable one in the **Agiparel-only** dev DB.

For persistent settings, securely configure `AGIPAREL_OPERATOR_*` in your normal deployment process. Remember: development Basic authorization **is not** a workforce identity provider and must never be the authority for CEO approvals, issuance or live production operations.

## Smoke test

```sh
curl -sS -o /dev/null -w 'Public portal: %{http_code}\n' http://127.0.0.1:18530/
curl -sS -o /dev/null -w 'Unauthenticated intake: %{http_code}\n' http://127.0.0.1:18530/order-intake
curl -sS -o /dev/null -w 'Unauthenticated intake API: %{http_code}\n' http://127.0.0.1:18530/api/v1/order-intake
curl -sS -o /dev/null -w 'Public domain: %{http_code}\n' https://agiparel.empowerai.us/
```

Expected: public `200`; unauthenticated operator route **`401` if credentials are configured** or **`503` if not configured**. After successful operator Basic login, open `https://agiparel.empowerai.us/order-intake` and use **Download PO template**, **Download BOM template**, **Validate & preview**, **Stage source evidence**. The sample templates should stage an order of **600 pieces**, with fabric demand **1110.000 M** and zipper demand **600.000 PCS**; status remains `PROPOSED`, `HOLD`. Staging the same files twice should return the same receipt.

**Do not run the restricted workbench for a real factory order**. Genuine buyer source documents require verification and approval, while the project remains development-only.

## Scope/priority and remaining gates

This batch earns its place in the 12-batch pilot plan because the exact order/SKU/BOM foundation makes the next material requirement/net indent and cutting stages computable. Not included: document classifiers and scans, confidence extraction, prices, multi-style/multi-PO uploads, buyer approvals, 8-stream T&A, production release, physically verified stock or RFID reader capture. Those get built only when their operational dependencies are ready.
