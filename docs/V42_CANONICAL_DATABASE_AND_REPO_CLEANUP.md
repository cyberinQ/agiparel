# Agiparel v42: canonical PostgreSQL baseline and repository cleanup

Author: Sunil Jain · 2026-10-29

This is a **development-only, destructive database reset**. There is no production rollout or migration path for populated databases. The installer requires an explicit `AGIPAREL_V42_RESET_DEV=YES` and verifies the `agiparel_dev` name.

## Evidence-driven baseline

1. Back up the original development database with `pg_dump -Fc` and verify restore manifest.
2. Extract the immutable archive of 57 core migrations and the separate identity migration into a private temporary directory, compare each byte to the current tracked migration, replay migrations 0001–0057 and the previously separate S01/S02 identity schema in a disposable PostgreSQL 16 database.
3. Generate PostgreSQL's canonical schema plus seed-data SQL from that database. No SQL concatenation or guessed DDL.
4. Load the baseline into a second fresh scratch database. Independently normalize both sides through disposable PostgreSQL round-trip restores, then compare complete schema, privileges, and seed-data dumps (excluding only nondeterministic dump restriction tokens). Abort on any mismatch.
5. Archive the old migrations for historical source-level contract tests; perform fresh database SQL tests against the new baseline. Never use archived legacy SQL for new installations.
6. Replace legacy default credentials with `.env`-provisioned, random development-only role passwords; keep old WAL volume for recovery and provision a fresh v42 dev WAL volume.
7. Rebuild only `agiparel_dev`, run SQL and concurrency tests, Go/race/vet, JavaScript, Docker build and health, then commit/push.

The JS contract tests that inspect legacy migrations still inspect the archived historical SQL **as historical fixtures**; they do not prove active v42 DDL. PostgreSQL schema, isolation, authorization and cross-database equivalence tests exercise the active baseline.

## Important boundaries

- The historical archive includes prior development-only credential literals; treat it as **source history**, never as an installation script or secret store.
- The old PostgreSQL backup and previous development WAL volume are preserved. Do not copy old WAL entries into a reset database.
- Existing source files and API implementations are otherwise unchanged; this cleanup does not prove production commissioning, complete stock conservation, or external delivery authority.
- For recovery after a failed reset, stop the edge/worker services and restore the saved `pg_dump -Fc` archive to a recreated `agiparel_dev` before reattaching its matching WAL state.

### v42.1 correction (2026-10-29)

The previous v42 comparison failed because PostgreSQL 16 normalizes equivalent array CHECK and partial-index expressions on restore. This hotfix independently round-trips the legacy and baseline dumps through PostgreSQL, then compares full normalized DDL, grants and reference/sequence data. It does not ignore or rewrite DDL differences. The recovery launcher verifies and removes only the two untracked files created during the aborted v42 run, then reruns the guarded installer. The original dev database remains unchanged until successful equivalence verification.
