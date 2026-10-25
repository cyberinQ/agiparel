# Epic 1 runtime repair — 25 October 2026

Author: Sunil Jain, 2026-10-25 — Runtime upload/AGIE root-cause analysis, changes, and smoke tests.

## What was verified

The Mac transcript supplied by the operator shows `docker compose up --build` succeeded, PostgreSQL and Redis were healthy, `go test ./...` passed, and deterministic CLI ingestion of seven supplied files committed to PostgreSQL. This establishes that the later browser upload failure is in the runtime path, not the original Go compiler issue. **No edge-daemon server error log was supplied**, so the precise failing uploaded filename and backend error are not established.

The code reveals two separate defects:

1. The HTTP upload path executed `/app/ingest-docs` without `-ollama-host`. The ingester's default is `http://localhost:11434`, which points inside the container, not the Mac. The supplied seven structured documents do not need a model, but PDFs, images and unfamiliar CSV/XLSX layouts do. Those paths can fail.
2. AGIE's chatbot tried an unconfigured default cloud URL before local Ollama; the browser assumed success even for non-JSON HTTP errors and could display a blank/undefined answer. Live-data answers also depended on a model making the appropriate tool call.

## Repairs

- Upload handler passes `OLLAMA_HOST` and `AGIE_MODEL` to the ingester; displays the per-file `[ERROR]` rather than a generic alert; serializes uploads; stages documents with rollback on failure and restores replaced originals.
- Failed extraction cannot overwrite the last committed workbook/conflict JSON/manifest. Only after successful DB synchronization are new artifacts published. **Database upsert and document-provenance commits are still separate transactions** and are not a complete atomic transaction boundary.
- Text PDFs are converted through `pdftotext` (Poppler installed in Docker). Image-only scanned PDFs still require OCR, and the UI returns a clear error rather than pretending to parse PDF binary. AI-based extraction also requires Ollama and the corresponding model to be installed and reachable.
- AGIE bypasses the LLM for the two observed questions: recorded attendance **today in Asia/Kolkata** and PO statuses marked `IN_PROGRESS`. It uses a read-only transaction and explicit tenant scope. It does not equate records from an earlier shift with today's presence, nor classify a `PROPOSED` PO as `IN_PROGRESS`.
- All other AGIE questions now go directly to local Ollama unless an explicit cloud endpoint **and** API key were configured. The generic model path rejects answers unless a `run_safe_sql` tool has successfully run; this is a basic guard, not a proof that generated language cannot misstate queried values.

## Deployment on the Mac

From the patched project root:

```sh
# Author: Sunil Jain, 2026-10-25 — Rebuild and probe the repaired Epic 1 services.
setopt interactivecomments
ollama list
# For PDF/unstructured intake and general AGIE questions, install the configured model:
ollama pull qwen3-coder:30b
# Only required when uploading standalone images:
# ollama pull qwen3-vl:8b
curl -fsS http://localhost:11434/api/tags

docker compose -f compose/docker-compose.dev.yml up --build -d
go test ./...
docker compose -f compose/docker-compose.dev.yml ps
docker compose -f compose/docker-compose.dev.yml logs --tail=120 plant-edge-os
```

Probe AGIE without its browser UI:

```sh
# Author: Sunil Jain, 2026-10-25 — Test the read-only AGIE executive API.
curl -sS -i http://localhost:18530/api/v1/agie/chat \
  -H 'Content-Type: application/json' \
  -d '{"messages":[{"role":"user","content":"how many workers present today?"}]}'
curl -sS -i http://localhost:18530/api/v1/agie/chat \
  -H 'Content-Type: application/json' \
  -d '{"messages":[{"role":"user","content":"which po is in progress?"}]}'
```

Upload a known-good structured file via HTTP:

```sh
# Author: Sunil Jain, 2026-10-25 — Smoke-test the multipart intake route.
curl -sS -i -F 'files=@data/staging/docs/02_Buyer_Purchase_Order_MK6574J9.csv' \
  http://localhost:18530/api/v1/planning/upload-docs
```

**If the previous (broken) UI upload left a failed unstructured file in `data/staging/docs`, it remains there after upgrading.** Review the failing filename in the previous edge-daemon logs, move that specific document *out* of `data/staging/docs` (retain it for diagnosis), then retry the upload. Do not delete the original six structured inputs or erase volumes with `docker compose down -v`.

## Verification boundary

Five patched Go source/test files passed standalone Go AST parsing. Four new Go unit tests (direct-question routing, ingestion error summarization, no unverified bot answer, and absent Ollama model handling) passed in an isolated standard-library-only Go 1.23 test harness. The complete Go 1.24 module and Docker/PostgreSQL integration checks **could not** be executed here because the Go 1.24 toolchain and external module downloads are unavailable in the sandbox. The Mac commands above are the final operational gate.

## Remaining Epic 1 gaps

Scanned-PDF OCR, multi-page ingestion chunking, fully transactional commit of workbook and document provenance, formally resolved conflict workflow, dynamic per-tenant timezone/identity, transactional fabric reservations, and real PDF/email dispatch are not implemented by this hotfix.
