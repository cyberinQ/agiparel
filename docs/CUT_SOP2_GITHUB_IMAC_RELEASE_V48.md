<!-- Author: Sunil Jain, 2026-11-12. -->
# AGIPAREL SOP-2 v48 — MacBook GitHub publication and iMac in-place upgrade

This is a source release, not a migration of the MacBook's disposable local database. The iMac retains its own PostgreSQL data, Redis data, signed evidence, WAL, settings, and TLS/tunnel configuration.

## Trust and product boundaries

- `/demo/cutting`: opt-in **fictional public training** page when `AGIPAREL_PUBLIC_CUTTER_DEMO_ENABLED=true`; default false. Isolated browser-memory data only. Public mode does not call the SOP-2 optimizer, query factory orders, or write records. The simulation remains visibly labelled.
- `/cutting/demo`: unchanged **localhost-only development** page, with optional server advisory optimizer.
- `/cutting/sop2`: unchanged restricted manager interface. Production writes remain disabled unless development gate is explicitly satisfied; this release does not change that gate.
- Existing signed RFID/EPC, NFC, inventory and sewing release controls are not bypassed. The public demo makes no operational claims.
- The optional public demo does not expose the private `/static/` asset directory or grant access to operational `/api/` routes.

## Publish on MacBook

1. Apply the patch from the v48 ZIP after the already-installed v46 + localdev + v47 patches.
2. `cd ~/agiparel && bash scripts/ship_cut_sop2_v48_macbook.sh` verifies source, builds/tests all Go code, runs JS contracts, and stages only the reviewed release files (no push).
3. Inspect `git diff --cached --stat` and `git diff --cached --check`. If satisfied, `bash scripts/ship_cut_sop2_v48_macbook.sh --push` commits and pushes directly to `origin/main` (no force). Requires local main to equal fetched origin/main before release.
4. Check GitHub Actions `Verify Agiparel OS` for the exact SHA. Do not deploy failed CI commits.

## Upgrade on iMac

1. Confirm the iMac's **actual** running service is the repository's `compose/docker-compose.dev.yml` stack, which earlier public v43 installers used. This deployment script **refuses** another compose stack; do not point it at production compose or recreate its database.
2. In the iMac checkout, run `git fetch origin && git pull --ff-only origin main`; inspect `git log -1 --oneline`.
3. If public training at https://agiparel.empowerai.us/demo/cutting is desired, set `AGIPAREL_PUBLIC_CUTTER_DEMO_ENABLED=true` once in the existing ignored `.env`, preserving all other values. No flag is required if public training should stay off.
4. Run `bash scripts/deploy_cut_sop2_v48_imac.sh --check`.
5. Run `AGIPAREL_CONFIRM_IMAC_DEPLOY=YES bash scripts/deploy_cut_sop2_v48_imac.sh --deploy`.

Deployment builds both app and document-intake worker **before** changing the database. It verifies a custom-format PostgreSQL backup, applies the 10-table additive SOP-2 migration only if entirely absent, validates RLS, recreates only the edge app and document worker (not DB/Redis/WAL), then checks local health, home, private control tower, and public training opt-in. An existing, complete SOP-2 schema is tested without reapplying its non-idempotent policies. A partial schema stops deployment.

## Rollback boundaries

The iMac script records old container image references/IDs and the database backup in `~/agiparel-db-backups`. In case of application failure, retag old app images and use `docker compose ... up -d --no-deps --force-recreate` on the app services. The migration is additive and intentionally **not** reversed automatically; restoring the database can destroy signed production data created after the backup.

## Limits

- No automated release to a differently named Docker Compose stack, image-digest production stack, Kubernetes, or another machine; those need a separate deployment plan.
- The public domain may be routed by an upstream TLS reverse proxy/tunnel. Local HTTP success does not prove public HTTPS routing; the script reports both distinctly.
- The native cutter demo is a teaching experience, not an authenticated physical workflow. Real factory activation still requires signed inspection, verified CAD markers, approvals, identity, NFC and evidence validation.
