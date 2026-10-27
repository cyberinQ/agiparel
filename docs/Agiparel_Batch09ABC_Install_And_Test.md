# Batch 09A–09C — install and test

**Starting point:** MacBook Agiparel working tree at 08C commit `8103af8`, migration `0021` applied. Keep the iMac deployment separate; do not run changes against live customer data as a simulated factory pilot.

## MacBook installation

```bash
cd ~/agiparel
git status --short
# Require a clean tree. Apply the provided Git patch from its saved location:
git apply --check /path/to/AGIPAREL_BATCH09ABC_sewing_simulation.patch
git apply /path/to/AGIPAREL_BATCH09ABC_sewing_simulation.patch
```

**Back up and inspect DB identity** (change DB name or container if your local configuration differs):

```bash
mkdir -p backups
docker exec agiparel-postgres pg_dump -U agiparel_admin -d agiparel_dev -Fc > "backups/pre09_$(date +%Y%m%d_%H%M%S).dump"
ls -lh backups/pre09_*.dump | tail -1
# Confirm this migration has not already been applied:
docker exec agiparel-postgres psql -X -U agiparel_admin -d agiparel_dev -c "SELECT to_regclass('public.sew_sim_cycles_v9');"
```

Only on a database that already has 0021 and lacks 0022:

```bash
docker exec -i agiparel-postgres psql -X -U agiparel_admin -d agiparel_dev -v ON_ERROR_STOP=1 < services/core-api/migrations/0022_sewing_station_simulation_v9.sql
```

Test and rebuild:

```bash
go test -count=1 ./...
go vet ./...
go build ./...
git diff --check
docker compose -f compose/docker-compose.dev.yml up -d --build --no-deps --force-recreate plant-edge-os
curl -I http://localhost:18530/sewing
```

Open **http://localhost:18530/sewing**; sign in with your locally configured development operator credentials. Select a cutting lay and exercise the acceptance list in `docs/Agiparel_Batch09ABC_Sewing_Simulation.md`. If no cutting drafts are present, create source-backed cutting drafts using the earlier workbench; the sewing simulator does not invent them.

Read-only SQL to inspect rehearsal evidence:

```bash
docker exec agiparel-postgres psql -X -U agiparel_admin -d agiparel_dev -c "SELECT state,count(*) FROM sew_sim_cycles_v9 GROUP BY state ORDER BY state;"
docker exec agiparel-postgres psql -X -U agiparel_admin -d agiparel_dev -c "SELECT evidence_state,count(*),coalesce(sum(garment_count),0) AS simulated_units FROM sew_sim_credits_v9 GROUP BY evidence_state;"
```

## Publish to iMac (only after testing)

Commit and push on MacBook; on iMac `git pull --ff-only origin main`; verify PostgreSQL backup and migration status; apply *only* unapplied `0022`; run Go tests and rebuild only `plant-edge-os`. Do not run 0016–0021 again. Never reset PostgreSQL or unrelated Docker services.

## Recovery / rollback

On failure before applying migration, stop and keep 08C. If migration 0022 was successfully committed but app rebuild fails, restore the 08C executable/image or revert Go code; leaving the **additive** 0022 schema in place is safe. **Do not DROP the six simulation tables if they contain audit records.** A physical DB restore requires a deliberate maintenance window and may overwrite post-backup data. Preserve any data/evidence before attempting downgrade.

## Verification limitation

This delivery was generated in a workspace with Go 1.23 and no network access to the repository-required Go 1.24 toolchain or `pgx` dependency. The newly authored Go file compiled and its isolated tests ran under Go 1.23 with type stubs for existing server functions. The static/web package tests and JavaScript syntax checks passed. Full repository build and live PostgreSQL migration/transaction verification **must be run on the MacBook** before rollout; no claim is made that they already passed.
