# Agiparel Batch 10A–10B — install, migrate, test, deploy

Author: Sunil Jain / CyberInQ. 2026-10-27. Incremental overlay on Batch 09A–09C (migration `0022` already applied).

## A. The established recipe (iMac or MacBook)

**1 — Inflate the ZIP** downloaded as `~/Downloads/agiparel_batch10ab_changes.zip`:

```bash
cd ~/agiparel
unzip -t ~/Downloads/agiparel_batch10ab_changes.zip
unzip -o ~/Downloads/agiparel_batch10ab_changes.zip -d ~/agiparel
ls services/core-api/migrations/0023_quality_finishing_simulation_v10.sql
```

**2 — Back up and migrate** the *existing* database. Do not reset PostgreSQL or its volumes.

```bash
cd ~/agiparel
mkdir -p backups
docker exec agiparel-postgres pg_dump -U agiparel_admin -d agiparel_dev -Fc > "backups/pre10_$(date +%Y%m%d_%H%M%S).dump"
ls -lh backups/pre10_*.dump | tail -1
```

Read-only preflight:

```bash
docker exec agiparel-postgres psql -X -U agiparel_admin -d agiparel_dev -c "SELECT to_regclass('public.qc_sim_units_v10') AS migration_0023_table;"
```

If `migration_0023_table` is `NULL` and the DB has migration `0022`, apply **once**:

```bash
docker exec -i agiparel-postgres psql -X -U agiparel_admin -d agiparel_dev -v ON_ERROR_STOP=1 < services/core-api/migrations/0023_quality_finishing_simulation_v10.sql
```

If tables already exist, do not blindly rerun SQL; check whether it completed and investigate before proceeding.

**3 — Test**:

```bash
cd ~/agiparel
go test -count=1 ./... && go vet ./... && go build ./... && git diff --check
```

**4 — Deploy** only the application service:

```bash
docker compose -f compose/docker-compose.dev.yml up -d --build --no-deps --force-recreate plant-edge-os
```

**5 — Verify**: `https://agiparel.empowerai.us/quality` or `http://localhost:18530/quality`. Unauthenticated `curl -I` can correctly respond `401` on the public site.

## B. Operator walkthrough

1. In **Sewing** (`/sewing`), finish at least one valid simulated cycle from existing cutting bundles. Those `sew_sim_credits_v9` are the only eligible sources for QC rehearsal.
2. In **Quality** (`/quality`), select the same lay and plant (`SIM-PLANT-01` by default). The page discovers source garment serials from the database—not fabricated browser examples.
3. Choose a garment, click **Emulate garment arrival / enroll**. This creates one persistent per-garment QC aggregate.
4. Choose one of four **immutable rehearsal finishing routes**. They are **not** automatically buyer-approved style DAGs.
5. Select **Fail** on garment A: only garment A becomes `HOLD`, with a defect code. Enroll garment B separately and pass it; B stays independent.
6. For A, change `actor` to a different repair worker and click **Record repair**. Change actor again to an independent inspector and click **Independent reinspection: pass**. A becomes `ACCEPTED` exactly once.
7. Run each finishing-step button in sequence for A or B. Attempts to execute a wrong step or finish a held unit fail.
8. Enter exact buyer **style_code** from the existing PO, **size** from cutting lineage, and a new buyer serial. A bind is allowed only after successful simulated QA and all route steps. Reusing a buyer serial for another garment is rejected.
9. Check counts in the UI and `qc_sim_events_v10`. Commands are idempotent by `event_id` + exact SHA-256 payload; same ID with different payload fails.

## C. Database smoke read

```bash
docker exec agiparel-postgres psql -X -U agiparel_admin -d agiparel_dev -c "SELECT tablename FROM pg_tables WHERE schemaname='public' AND tablename LIKE 'qc_sim_%_v10' ORDER BY tablename;"
```

Expect `qc_sim_routes_v10`, `qc_sim_units_v10`, `qc_sim_events_v10`, `qc_sim_buyer_bindings_v10`.

No migration may transform simulated QC into authoritative inventory, factory QA release, payroll, or shipping evidence. The public-facing development-mode demonstration must be access restricted.

## D. Verification boundary

`gofmt`, JavaScript parse checks, ZIP integrity and source diff checks were run when packaging. Full Go `go test`, `go vet`, `go build` and live PostgreSQL tests could not be executed inside the packaging environment (Go 1.23, no Go 1.24 toolchain download or running PostgreSQL). The iMac's Go 1.24 tooling and existing DB are required for the official pass/fail. **Do not consider deployment accepted until these commands pass and the described negative cases are exercised.**
