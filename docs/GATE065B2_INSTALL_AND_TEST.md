# Gate 06.5-B2 — Document-Sourced Factory Proposals

## Outcome

This builds **on top of Gate 06.5-B1**, which you already installed successfully. The customer continues to use ONE document inbox at `/documents`. The `/factory-setup` page now includes a document-to-factory review area: select an uploaded factory document, scan its extracted source evidence, inspect suggestions with page/sheet/row links, and either keep each result as **PROVISIONAL** or reject it. Accepted structured rows appear in a separate proposed machine, worker-skill, material-stock or costing-rate register; scalar values can populate the earlier 17-field guided onboarding form as provisional observations. No authoritative operational master, stores ledger, worker payroll, approved quote, reservation, purchasing or production release is modified.

This is **structured heuristic extraction with exact source anchors**, not comprehensive AI understanding. An incomplete document can yield zero suggestions without stopping manual factory onboarding.

## Install code (ZIP recommended, do NOT also apply patch)

Download `agiparel_gate065b2_factory_evidence.zip` into Downloads. It includes only the 20 new/changed paths and won't replace your modified `portal.go`, your `dev` login settings or Docker Compose configuration:

```bash
cd ~/agiparel
unzip -l ~/Downloads/agiparel_gate065b2_factory_evidence.zip
unzip -o ~/Downloads/agiparel_gate065b2_factory_evidence.zip -d ~/agiparel
```

Apply migration 0011 **after migration 0010**:

```bash
cd ~/agiparel
docker exec -i agiparel-postgres \
  psql -U agiparel_admin -d agiparel_dev -v ON_ERROR_STOP=1 \
  < services/core-api/migrations/0011_factory_document_candidates_v65b2.sql
```

Expect `COMMIT`. No data or volumes are reset. It only adds three new RLS-protected tables. Only the restricted `agiparel_app` role can insert proposed facts and decisions; the document extraction worker cannot approve them. Reapplying the migration creates no duplicate rows.

## Compile and test

```bash
cd ~/agiparel
go test -count=1 ./services/edge-daemon/cmd/server
go test -count=1 ./services/edge-daemon/internal/web
go test -count=1 ./...
go vet ./...
go build ./...
git diff --check
```

The optional real-PostgreSQL acceptance test is `TestFactoryCandidatesPostgresReviewBoundary`. It SKIPS unless the pre-existing environment variables `AGIPAREL_TEST_ADMIN_DATABASE_URL`, `AGIPAREL_TEST_APP_DATABASE_URL` and `AGIPAREL_DESTRUCTIVE_DB_TESTS=I_UNDERSTAND_TEST_FIXTURES` are supplied. It creates a random isolated fixture tenant, validates deduplication, tenant RLS, a one-time provisional review and role restrictions, then deletes only its own fixture records. It must use the disposable Agiparel development DB, not production.

## Deploy JUST the edge service

Your Compose development defaults remain **username `dev`, password `1212`, tenant `dev`**. No exports are needed if you already have these values or nothing exported. If old values may remain in the shell, use `unset`:

```bash
cd ~/agiparel
unset AGIPAREL_OPERATOR_USER AGIPAREL_OPERATOR_PASSWORD AGIPAREL_OPERATOR_TENANT
docker compose -f compose/docker-compose.dev.yml \
  up -d --build --no-deps --force-recreate plant-edge-os
```

Do not run `compose down`, prune images or volumes, or change other apps on your iMac. The `document-intake` worker from Gate 06.5-A remains as-is.

## HTTP smoke checks

```bash
curl -sS -o /dev/null -w 'Factory setup: %{http_code}\n' \
  -u dev:1212 http://127.0.0.1:18530/factory-setup
curl -sS -o /dev/null -w 'Candidates: %{http_code}\n' \
  -u dev:1212 http://127.0.0.1:18530/api/v1/factory-onboarding/candidates
curl -sS -o /dev/null -w 'Proposed register: %{http_code}\n' \
  -u dev:1212 http://127.0.0.1:18530/api/v1/factory-onboarding/proposed-register
curl -sS -o /dev/null -w 'Homepage: %{http_code}\n' \
  https://agiparel.empowerai.us/
```

Expect 200s when tenant is configured and schema is present.

## Visual customer acceptance demo

1. Open `https://agiparel.empowerai.us/documents` and log in as **dev / 1212**.
2. Upload **real factory documents**, or try the optional five files from `agiparel_gate065b2_sample_factory_docs.zip`. The examples are **fabricated**, not proof of real factory extraction. Wait until extraction is `EXTRACTED` or `PARTIAL`.
3. Open `https://agiparel.empowerai.us/factory-setup`, scroll to **Let your documents help complete the factory picture**.
4. Select each uploaded factory document and click **Find possible factory records**. For the five example sources you should get 3 machine rows, 3 worker-skill rows, 2 stock observations, 2 rate observations, and 4 scalar factory facts (14 total proposals). Re-scanning should insert **zero** new candidate rows.
5. Choose **Keep as provisional** for a machine; it appears in the **Your provisional registers** column. Choose **Not relevant** for another suggestion; it no longer waits for review. Use the source link to jump to the evidence location in the original document inspector.
6. Accept the scalar `Sewing machines: 3`: the 17-field factory form should display it as provisional. Confirm it separately, if valid, using the existing guided fact editor. If a field is already confirmed, auto-import does not overwrite it.
7. Verify `/api/v1/factory-onboarding` still reports `production_authorized:false`. Material issuance also remains `NOT_AUTHORIZED`.

## What this DOES NOT claim

- Scanned image OCR, direct Google Docs/Sheets connectors, legacy binary XLS conversion, PDF table reconstruction, enterprise-grade multi-row header matching, every machine/capacity/skill nuance, or extracting entire actual factory master data automatically. These need later work.
- A proposal, even one the operator elects to keep, is **not** a verified source-of-truth record. It cannot be fed as assured inventory, wages, factory machine availability or production readiness until the relevant independently validated workflow exists.
- Existing buyer PO/BOM uploads from Batches 05–06 remain staged/held; no automatic buyer-order population from PDFs is added by this gate.
- Full Go 1.24 compilation and live PostgreSQL tests are performed on your Mac; the delivery environment ran isolated stdlib tests and JavaScript checks.

## Revision-safe operations

Use `git status --short` and `git diff --check` before committing. The ZIP intentionally doesn't overwrite your local login policy or older UX test fixes. If you commit, include just these changed source, migration, examples and documentation files. Secrets, employee PII and real customer files should not go to public source control.

## Next milestone

Gate 06.5-B3: classification and stronger document understanding across messy PDFs, Office and legacy sources; human review for ambiguous fields; versioned canonical machine/skill/inventory/cost **master records** with their own approvals and staged operational readiness gates. Buyer-document semantic normalization follows once factory foundations are reliable.
