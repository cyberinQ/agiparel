# Epic 1 implementation repair and verification

Author: Sunil Jain — 2026-10-25

## What changed

- Fixed the missing closing brace in `handlePlanningApprove` and the missing `planning` import. Restored `/planning` route.
- Added an explicit dependency on `nats.go` v1.49.0 (Go 1.24 compatible), and Docker now resolves Go module dependencies after copying source code.
- Moved file intake to a **real multipart upload** route `POST /api/v1/planning/upload-docs`; files are copied into the local bind-mounted staging directory before invoking the same deterministic ingestion binary used by the CLI. Uploads are serialized on each edge server. **This is a pilot approach, not the final S03 standalone document service.**
- Replaced false-positive heuristic XLSX/CSV parsing with header-based parsers for the supplied operation bulletin, PO, machine roster, worker attendance, GRN rolls, and packing bulletin. Preserves sparse XLSX column offsets; doesn't invent default operations, workers, machine types, roll inventory, or order quantities.
- Added a cumulative order conflict gate: no silent overwrite when the 2022 sample's `12,000` pieces disagree with the 2026 PO's `600`. Packing bulletin reports `70 cartons * 12 = 840`, inconsistent with 600. These are flagged in `data/staging/output/conflicts.json`; they prevent plan approval.
- Parses and preserves the source shift date `2026-10-21` rather than silently claiming historical attendance happened today.
- Rebuilt the database workflow: `-apply-db=true` fails early if PostgreSQL is unreachable. The manifest reports `SUCCESS` only **after** workbook and intake provenance are committed. Explicit `-apply-db=false` produces `ARTIFACT_ONLY` and `db_applied=false`. A new `documents` table stores SHA-256 and conflict metadata.
- Reworked approval to validate a configured manager PIN, verify one exact work order changed in a transaction, reject RED/YELLOW/incomplete assessments and unresolved document conflicts. Removed the dangerous mock quick fixes that fabricated inventory, attendance and machine health. Disabled fake email `SENT` reporting.
- Scoped the ingestion tenant with **transaction-local** `set_config`, expanded RLS, corrected the duplicate identity `EnforcePermission` method, and made PostgreSQL-dependent tests explicitly opt-in.

## Verified extraction from original 7 files

| Item | Count / result |
|---|---|
| Operation bulletin operations | 14 |
| Machine roster entries | 16 |
| Worker roster entries | 17 |
| GRN fabric rolls | 10 |
| Current order | MK6574J9, 600 pieces |
| Conflicts | packing 840 vs 600; legacy sample 12,000 vs 600 |
| Offline validation | All 7 parsed deterministically, no Ollama needed |

Regression tests validate all seven files and conflict handling. Local Go 1.23 testing used a **temporary, non-delivered NATS stub** and omitted the PostgreSQL driver to verify compilation and offline logic. It does **not** demonstrate a real Docker build, Go 1.24 dependency installation, PostgreSQL execution, or SMTP; those require testing on the development machine.

## Run on the Mac from the repository root

```zsh
setopt interactivecomments
# Author: Sunil Jain, 2026-10-25 — Build and run the repaired Epic 1 intake without destroying existing volumes.
export MANAGER_PIN=1234

docker compose -f compose/docker-compose.dev.yml up --build -d
docker compose -f compose/docker-compose.dev.yml ps

go mod tidy
go test ./...
go run scripts/ingest_docs.go -force -apply-db=true
cat data/staging/output/.processed_manifest.json
cat data/staging/output/conflicts.json
curl -s 'http://localhost:18530/api/v1/planning/assessment?po=MK6574J9'
```

**Important:** `MANAGER_PIN=1234` is only a throwaway local-development example. Set a different PIN before real use; four digits alone are **not adequate production-grade authentication**. The server currently lacks a complete session/RBAC enforcement mechanism. Do **not expose this pilot application publicly**.

Open `http://localhost:18530/planning`. The files in `data/staging/docs` are bind-mounted at `/app/data/staging/docs` inside the container; changes made through the upload UI are visible to the CLI and vice versa.

Only if this is a disposable database and you explicitly intend to erase all database volumes, use `docker compose -f compose/docker-compose.dev.yml down -v` before building. Normally **do not** use `-v`: it deletes the PostgreSQL data volume. Existing initialized PostgreSQL volumes do not rerun changed `0001` migration files; use a proper forward migration before production deployment.

When intentionally validating offline processing, run `go run scripts/ingest_docs.go -force -apply-db=false`. The manifest will **correctly** use `ARTIFACT_ONLY`, never `SUCCESS`.

## Still open before Epic 1 acceptance

- `E1-S1`: HTTP upload currently invokes an ingestion subprocess rather than importing a reusable intake library. Provenance has SHA and local filenames, but not fully versioned document assertions or immutable object storage.
- `E1-S2`: field-level conflict coverage is currently focused on PO quantity and carton-conservation contradictions. Full buyer-technical precedence, approved revision graph, supplier attachment data, and conflict-resolution workflow remain to be built.
- `E1-S3`: readiness still uses the cumulative staged workbook, not authoritative live DB queries or a measured marker/BOM consumption calculator. The existing `1.25m/piece` cloth demand is explicitly an estimate. Daily output and completion estimates are **not** a constrained APS solver.
- `E1-S4`: the management UI now uploads actual files and reports verified readiness but lacks an authenticated, auditable conflict-resolution editor and manager-approved reassignments.
- `E1-S5`: the DB status lock does **not** transactionally reserve the exact physical rolls; PDF generation, SMTP transport, and a DB-transactional outbox remain incomplete. Email route correctly returns HTTP 501 rather than reporting a fake dispatch.
- `E0`: identity tenant UUID data is separate from legacy factory tenant string IDs, and complete end-to-end RLS, NATS, and PostgreSQL integration tests have not been executed here.

Do not label the entire Epic 1 as production-complete until those acceptance gates pass. In particular, the sample data's source mismatch and the carton-count mismatch need a documented human decision.
