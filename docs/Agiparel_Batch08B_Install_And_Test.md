# Batch 08B — MacBook localhost installation and acceptance

Target: **MacBook** `~/agiparel`, local `http://localhost:18530/cutting`, container `agiparel-postgres`, tenant `dev`. The iMac at `https://agiparel.empowerai.us` remains unchanged.

**Prerequisite:** Batch 08A code and migration `0019` applied (as confirmed by the Oct 27 MacBook logs). The ZIP contains changed source only, not a full project.

## 1. Apply code

Download `agiparel_batch08b_cutting_execution_evidence.zip` to `~/Downloads`:

```bash
cd ~/agiparel
unzip -o ~/Downloads/agiparel_batch08b_cutting_execution_evidence.zip -d ~/agiparel
```

If there are uncommitted local modifications, inspect `git status --short` beforehand; do not overwrite independent edits unknowingly.

## 2. Apply migration 0020 (once only)

```bash
docker exec -i agiparel-postgres \
  psql -X -U agiparel_admin -d agiparel_dev -v ON_ERROR_STOP=1 \
  < services/core-api/migrations/0020_cutting_spread_and_scan_evidence_v8.sql
```

Expected final message `COMMIT`. `0020` is **not designed for repeated manual application**; if it has already committed, skip it. Do not drop the database or volumes. This migration creates three new tenant-isolated tables and audit/conservation triggers; it does not edit signed material issuances or roll balances.

Verify:

```bash
docker exec agiparel-postgres psql -X -U agiparel_admin -d agiparel_dev -c \
"SELECT to_regclass('public.cut_spread_reports_v8') AS spread_reports,
        to_regclass('public.cut_spread_issue_allocations_v8') AS issue_allocations,
        to_regclass('public.cut_bundle_scan_witnesses_v8') AS bundle_witnesses;"
```

## 3. Run complete Go regression suite

```bash
cd ~/agiparel
go test -count=1 ./... && go vet ./... && go build ./... && git diff --check
```

All commands should exit 0 before deployment. The 08A tests must continue to pass. If any fails, share the **first FAIL** and the surrounding lines.

## 4. Deploy only the local cutting service

```bash
docker compose -f compose/docker-compose.dev.yml \
  up -d --build --no-deps --force-recreate plant-edge-os
```

Open **http://localhost:18530/cutting**, scroll to **Section 4 — Batch 08B physical spread reporting & signed-issue reconciliation**. Select a saved cutting draft. The table displays matching signed material-issue UUIDs, issued lengths, previously attributed lengths, and remaining attribution capacity.

If the recently reset MacBook database has no independently witnessed issues, this list will correctly be empty and reported consumption will be blocked. **Do not forge witness entries merely to make the demo green.** A real external IdP is required for actual material issuance and signed EPC witnesses.

## 5. Optional rollback-only SQL integration test (disposable local DB)

This test starts a single SQL transaction, inserts synthetic sample records as superuser, switches to the restricted app role, checks tenant isolation, immutable evidence, EPC collision protection, and duplicate issue attribution, then **rolls back the transaction**. It never promotes its synthetic fixtures to production records.

Requires `AGIPAREL_TEST_ADMIN_DATABASE_URL` pointing to a nonproduction DB where migrations `0001–0020` exist. Example (adapt host/credentials; do not paste secrets into chat):

```bash
export AGIPAREL_TEST_ADMIN_DATABASE_URL='postgres://YOUR_ADMIN:YOUR_PASSWORD@127.0.0.1:15432/agiparel_dev?sslmode=disable'
AGIPAREL_CUT_SQL_TEST_OPT_IN=ROLLBACK_ONLY \
  go test -count=1 ./services/edge-daemon/cmd/server -run '^TestCuttingSpreadPGConstraintsRollbackOnly$' -v
unset AGIPAREL_TEST_ADMIN_DATABASE_URL
```

This test **requires a PostgreSQL superuser** to set local role and uses rollback-only fixtures. If your normal database admin is not a superuser, it skips; do not grant superuser to the web app.

## 6. Secure EPC witness onboarding (not enabled by dev login)

The secure cutting endpoints reuse existing `AGIPAREL_MATERIAL_API_ENABLED`, external Ed25519 IdP public key, issuer/audience/KID, and material authority configuration. The `cutting_reader` role and actions `cut.bundle.scan.prepare`, `cut.bundle.scan.submit`, `cut.bundle.scan.witness` must be provisioned by your trusted IdP. This release does not generate private signing keys or provide a fictitious reader integration.

A signed human witness is deliberately marked **NOT HARDWARE ATTESTED**. There is no cutting release, bundle movement authority, or sewing credit in 08B.
