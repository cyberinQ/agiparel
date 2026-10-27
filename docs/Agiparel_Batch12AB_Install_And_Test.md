# Batch 12A–12B — Factory Intelligence (simulation evidence)

## Contract
Read-only tenant-scoped, source-derived stage metrics; bounded server-timestamp interprocess intervals; distribution and operation SAM observational signals; immutable report snapshots. Never treat a UI-simulated scan as a physical reader observation or a rehearsal manifest as a commercial shipment. No production closure, scrap declaration, payroll, finance or ERP handoff.

## Prerequisites
The iMac repository must have 09A–09C, 10A–10B and 11A–11B applied, and PostgreSQL migrations through **0024** must have succeeded. Verify clean `git status` only if your previous batch changes were committed; ZIP overlays otherwise are expected. Do not overwrite uncommitted custom edits.

## Install: established ZIP → inflate → migrate → test → deploy

1. Download `agiparel_batch12ab_changes.zip` into `~/Downloads`.
2. Inflate:

```sh
cd ~/agiparel
unzip -o ~/Downloads/agiparel_batch12ab_changes.zip -d ~/agiparel
```

3. Back up before migration and check archive:

```sh
mkdir -p backups
BACKUP="backups/pre12_$(date +%Y%m%d_%H%M%S).dump"
docker exec agiparel-postgres pg_dump -U agiparel_admin -d agiparel_dev -Fc > "$BACKUP"
test -s "$BACKUP" && pg_restore -l "$BACKUP" >/dev/null
```

4. Check whether migration 0025 is already present:

```sh
docker exec agiparel-postgres psql -X -U agiparel_admin -d agiparel_dev -c "SELECT to_regclass('public.intel_sim_snapshots_v12');"
```

If the result is NULL, apply **once**:

```sh
docker exec -i agiparel-postgres psql -X -U agiparel_admin -d agiparel_dev -v ON_ERROR_STOP=1 < services/core-api/migrations/0025_factory_intelligence_snapshots_v12.sql
```

If it exists, do not rerun the migration; inspect the existing schema if it was partially applied.

5. Test:

```sh
go test -count=1 ./... && go vet ./... && go build ./... && git diff --check
```

6. Deploy only after tests pass:

```sh
docker compose -f compose/docker-compose.dev.yml up -d --build --no-deps --force-recreate plant-edge-os
```

7. Open `https://agiparel.empowerai.us/intelligence` (or local port 18530). Select a cutting lay and factory timezone, load the report, inspect residuals, intervals, operation signals, and capture an immutable snapshot. Simulation APIs are development-mode-only.

## Acceptance checks
- Source-only counts (sewing, QC states, finishing, buyer binding, packing, sealing, dispatch rehearsal), with no double counting of operations.
- Residuals explicitly distinguish open WIP/unknown from negative invariant violations; never label residual as proven scrap.
- Missing, reversed, long (>2h), or factory-calendar-day-crossing intervals are excluded as unknown. P50/P90/P95, standard deviation and CV are derived only from valid server timestamps; fewer than five samples are not rankable.
- Tenant scoping and repeatable-read snapshot consistency.
- Same capture ID + same lay/timezone returns idempotent replay; different inputs must conflict.
- Captures are immutable, include provenance and SHA-256, and never grant commercial authority.
- No simulated reader payload can authorize payroll, physical dispatch, production closure or ERP handoff.

## Known limitations
This is a rehearsal analytics workbench, not full master-plan E12 financial closure. Cut marker positions and bundle candidate groups are not equivalent to verified garment units. Shift calendars, physical reader event provenance, scrap adjudication and real finance closure remain outside this batch. PostgreSQL integration and Go 1.24 test suite must pass on the iMac before considering it deployed.
