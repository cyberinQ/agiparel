# Gate 06.5-B1 — Factory First / Progressive Discovery

**Apply after Gate 06.5-A and A.1.** This release adds a functional, persistence-backed factory-discovery workbench. It **does not** approve payroll, machine master records, material stock, purchase orders or production work. Existing services and unrelated Docker workloads are unaffected.

## Changed paths

**New:**

- `services/core-api/migrations/0010_factory_discovery_v65b.sql` — append-only tenant-isolated, field-typed onboarding evidence, revisions and source-document references.
- `services/edge-daemon/cmd/server/factory_discovery.go` — routed GET/POST APIs, bounded validation, revision compare-and-append, dynamic per-action readiness and conservative document relevance hints.
- `services/edge-daemon/cmd/server/factory_discovery_test.go` — value, unknown vs zero, provisional vs confirmed, method and gate tests.
- `services/edge-daemon/cmd/server/factory_discovery_pg_integration_test.go` — optional real RLS, conflict, append-only and tenant-isolation test (requires your existing test env variables).
- `services/edge-daemon/internal/web/static/factory_setup.html`, `.css`, `.js` — guided visual workbench.
- `docs/Agiparel_Master_PRD_ARD_v5.3.md`, `docs/Agiparel_Master_Epics_Stories_Tasks_v5.3.md`, `docs/Agiparel_Implementation_Plan_v5.3.md` — full retained v5.2 requirements plus factory-first amendment.

**Replace (minimal changes relative to 06.5-A.1):**

- `services/edge-daemon/cmd/server/main.go` — registers new restricted routes.
- `services/edge-daemon/internal/web/assets.go` — embeds the new UI.
- `services/edge-daemon/internal/web/static/portal.html` — promotes factory setup before document and buyer flows.
- `services/edge-daemon/internal/web/static/documents.html` — links back to guided factory setup.
- `services/edge-daemon/internal/web/static/workspace_nav.js` — shared Factory Setup navigation item.
- `services/edge-daemon/internal/web/navigation_test.go` — page navigation coverage.

The package **does not replace** `portal.go` (your `dev/1212` change), `portal_test.go`, Compose files, or any existing source documents. Existing v5.2 docs remain intact; v5.3 is additive.

## Install (recommended ZIP, do not also apply the patch)

Download `agiparel_gate065b1_factory_first.zip` to `~/Downloads`:

```bash
cd ~/agiparel
unzip -l ~/Downloads/agiparel_gate065b1_factory_first.zip
unzip -o ~/Downloads/agiparel_gate065b1_factory_first.zip -d ~/agiparel
```

### Apply the one new schema migration

```bash
cd ~/agiparel
docker exec -i agiparel-postgres \
  psql -U agiparel_admin -d agiparel_dev -v ON_ERROR_STOP=1 \
  < services/core-api/migrations/0010_factory_discovery_v65b.sql
```

Expect `COMMIT`. The migration requires Gate 06.5-A's 0009 document sources table and the `agiparel_app` role. It creates no real factory records; it does not reset any volume. It is safe to reapply due to `IF NOT EXISTS` and the recreated RLS policy, but normally run once.

### Test before deployment

```bash
cd ~/agiparel
go test -count=1 ./services/edge-daemon/cmd/server
go test -count=1 ./services/edge-daemon/internal/web
go test -count=1 ./...
go vet ./...
go build ./...
git diff --check
```

**Important:** Your previous Gate 06.5-A.1 upgrade changed the Document Intelligence page's heading. The corresponding local test (`document_intake_test.go`) needs to expect `One place for every file.`. That adjustment was already provided after your last test output; this ZIP intentionally leaves your test file alone.

**Optional isolated DB integration:** Existing `TestFactoryDiscoveryPostgresTenantAndRevisions` is skipped unless all three existing test environment variables are present: `AGIPAREL_TEST_ADMIN_DATABASE_URL`, `AGIPAREL_TEST_APP_DATABASE_URL`, `AGIPAREL_DESTRUCTIVE_DB_TESTS=I_UNDERSTAND_TEST_FIXTURES`. They must point to your **disposable** development PostgreSQL database using the admin and restricted app roles. It creates random tenant-scoped facts and removes only those fixtures. Run:

```bash
go test -v -count=1 ./services/edge-daemon/cmd/server \
  -run '^TestFactoryDiscoveryPostgresTenantAndRevisions$'
```

A `SKIP` is not a passing DB integration test.

### Deploy only Agiparel's edge service

Gate 06.5-A's Compose defaults are `AGIPAREL_OPERATOR_USER=dev`, `AGIPAREL_OPERATOR_PASSWORD=1212`, `AGIPAREL_OPERATOR_TENANT=dev`. If stale values are exported, they override defaults. To use the intended defaults without overwriting files:

```bash
cd ~/agiparel
unset AGIPAREL_OPERATOR_USER AGIPAREL_OPERATOR_PASSWORD AGIPAREL_OPERATOR_TENANT
docker compose -f compose/docker-compose.dev.yml \
  up -d --build --no-deps --force-recreate plant-edge-os
```

Do not use `docker compose down`, `docker system prune`, `docker volume prune`, or touch other applications. The `document-intake` worker is unchanged and need not be rebuilt.

### Check endpoints

```bash
curl -sS -o /dev/null -w 'Homepage %{http_code}\n' https://agiparel.empowerai.us/
curl -sS -o /dev/null -w 'Factory without login %{http_code}\n' http://127.0.0.1:18530/factory-setup
curl -sS -u dev:1212 -o /dev/null -w 'Factory with login %{http_code}\n' http://127.0.0.1:18530/factory-setup
curl -sS -u dev:1212 -o /dev/null -w 'Factory data %{http_code}\n' http://127.0.0.1:18530/api/v1/factory-onboarding
```

Expect `200`, `401`, `200`, `200` respectively. If factory data returns 503, check that migration 0010 applied, that the edge can access PostgreSQL, and that Compose tenant is `dev`.

### Manual UI acceptance

Open `https://agiparel.empowerai.us/factory-setup` and authenticate with your development account. Test these steps:

1. Enter factory name as `PROVISIONAL` (e.g., `Example Garments`); page shows a first fact and invites further discovery.
2. Enter machine count `40` as **estimate**; capacity readiness remains `NEEDS_CONFIRMATION`.
3. Edit machine count to `40` **confirmed**; prior fact stays in append-only event history. You can still lack employee count/skills.
4. Upload a machine list or material inventory file through `/documents`; return to factory setup and see suggested document domain. Choosing it as a supporting document is manual and does not validate its extracted numbers.
5. Leave cost rates blank and continue to view the sample factory story; do not see any false zero-cost quote.
6. Even after confirming fields, Material Issuance and Production Release always say `NOT_AUTHORIZED` until their independent evidence/identity systems exist.

Any unknown/ambiguous file stays `Unclassified`. A spreadsheet or employee roster is **not** automatically transformed into a machine or worker master row in this batch.

## Limitations and next B2/C increment

- This first gate stores **manual observations** under explicit review status, not source-parsed canonical machine/worker/stock records. Uploaded files are currently classified by conservative keyword suggestions only.
- Next B2: extract source-backed machine/skills/material/cost candidates from real XLSX/PDF/Word/legacy XLS, show page/cell evidence, contradictions and corrections, add privacy-safe worker models and guided AI assistance.
- Next C: controlled promotion of reviewed factory facts to normalized enterprise tables, then approved buyer orders to order-intake/material-planning, with tenant-pinned semantic AI queries.
- Basic Auth `dev/1212` remains development-only. **Do not upload confidential employee payroll, personal identifiers or production-sensitive files** to this weakly protected public test environment.

**Validation in authoring environment:** Isolated standard-library Go tests and embedded navigation tests passed; JavaScript syntax validated. Full repository Go 1.24 and live PostgreSQL were not available here and require your iMac verification.
