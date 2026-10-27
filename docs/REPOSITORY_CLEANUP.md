# Agiparel development cleanup — October 2026

This is a deliberately destructive **development-only** cleanup. It operates on
`~/agiparel` and the Agiparel `agiparel_dev` database, not on other applications
hosted by the iMac. It does not drop Docker volumes, reset Redis, rewrite Git
history, remove migrations, or delete source files.

## Execute in order

1. Download `agiparel_repo_cleanup_overlay.zip` and place in `~/Downloads`.
2. Confirm your code includes migration `0025` and is at the correct Git branch.
3. Inflate overlay: `cd ~/agiparel && unzip -o ~/Downloads/agiparel_repo_cleanup_overlay.zip -d .`
4. Delete obsolete tracked documents, uploaded artifacts and backups:
   `bash scripts/cleanup_dev_repo.sh --apply`
5. Reset **only** Agiparel's dev database and reconstruct schema from scratch:
   `bash scripts/reset_dev_database.sh --reset-agiparel-dev`
6. Validate:
   `go test -count=1 ./... && go vet ./... && go build ./... && git diff --check`
7. Commit/push the cleanup after tests:
   `git add -A && git commit -m 'chore: prune runtime artifacts and rebuildable dev data' && git push origin main`
8. Optionally rebuild `plant-edge-os` with existing Compose command, or open `/intelligence` after step 5.

## What is deleted

- All `backups/` PostgreSQL dumps (including tracked 13 MB backups)
- Everything in `data/staging/`: runtime uploads, parsed manifests and output
- Obsolete PRD/ARD drafts, v4/v5.1/v5.2 duplicates, individual batch install notes,
  and older stage/gate explanations from `docs/`
- Obsolete root `README.txt`, standalone RFID notebook and macOS metadata

Seven small input documents previously under `data/staging/docs/` are moved to
`tests/fixtures/intake/` because `scripts/ingest_docs_test.go` actually uses them.
These test inputs are deliberately synthetic; they are not injected into DB.

## What remains

- Entire runtime, services, test suite, Go deps, Dockerfiles and Compose
- All 25 SQL migrations (schema and permission reconstruction)
- `docs/Agiparel_Master_PRD_ARD_v5.3.md`,
  `docs/Agiparel_Master_Epics_Stories_Tasks_v5.3.md`,
  `docs/Agiparel_Implementation_Plan_v5.3.md`, and this guide
- Test fixtures in `tests/fixtures/intake/` and `examples/factory-onboarding/`
- `.git/` with all committed Git history, so existing clones remain compatible

## Database operation

`reset_dev_database.sh` stops only Agiparel's `plant-edge-os` and
`document-intake` containers, verifies the Postgres container's DB name,
drops database **agiparel_dev** with FORCE (disconnecting clients to that
DB only), recreates it, reapplies the ordered 0001–0025 schema migrations,
asserts that every public table contains zero rows and restarts these services.
If a migration fails, the services remain stopped so the problem is visible.

This deliberately erases **all** tenants, readers, user-configured stations,
orders, garment genealogy, audit events, and test records. The Go integration
tests may generate disposable rows; if you require exactly zero rows *after*
tests, repeat the database reset. The schema itself is preserved/rebuilt.

## Notes on Git size

The original archive included ~47 MB of `.git/` history. Removing tracked
backups now shrinks the current checkout, but Git's committed history still
contains historical backup objects. This cleanup does not force-push rewritten
history; that is a distinct migration of every clone and is not necessary to
run the application. Avoid `git gc --prune=now` or `git filter-repo` on the
live repository without a coordinated history-rewrite plan.
