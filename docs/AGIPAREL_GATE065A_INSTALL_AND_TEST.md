# Agiparel Gate 06.5-A — install and acceptance on your Mac

**Base required:** Batch 06 repository and migration `0008_material_scenarios_v6.sql` already applied.
**Result:** `/documents` live, PDF/XLSX/DOCX/CSV/TSV/TXT original upload, tenant-`dev` queue, database-preserved originals, background extraction, source-anchored review UI. **No automated PO/BOM business approval yet.**

## 1. Download and install **only the ZIP**

Download `agiparel_gate065a_source.zip` to `~/Downloads`.

```bash
cd ~/agiparel
unzip -l ~/Downloads/agiparel_gate065a_source.zip
unzip -o ~/Downloads/agiparel_gate065a_source.zip -d ~/agiparel
```

There are **16 changed or added files**. The ZIP deliberately does **not** overwrite `portal.go` or `portal_test.go`, preserving your four-character development-login change. Do not also apply the patch.

## 2. Apply the additive, tenant-safe migration

```bash
docker exec -i agiparel-postgres \
  psql -U agiparel_admin -d agiparel_dev -v ON_ERROR_STOP=1 \
  < services/core-api/migrations/0009_document_intelligence_v65.sql
```

Expect `COMMIT`. This adds four `document_*_v65` tables and an internal, **document-only** DB worker role; no operating data tables are reset. `document_sources_v65.original` is immutable under the application role. `document_jobs_v65` is durable. DB RLS uses the operator tenant `dev` and composite tenant keys. The migration's embedded worker secret is **disposable development-only**.

## 3. Full test suite, then deploy **two Agiparel services**

The Compose file now defaults to username **`dev`**, password **`1212`**, tenant **`dev`**. You do **not** need to export them for new shells. If you previously exported different credentials in this shell, `unset AGIPAREL_OPERATOR_USER AGIPAREL_OPERATOR_PASSWORD AGIPAREL_OPERATOR_TENANT` once to use the Compose defaults.

```bash
cd ~/agiparel
go test -count=1 ./services/document-intake/...
go test -count=1 ./services/edge-daemon/cmd/server
go test -count=1 ./...
go vet ./...
go build ./...
git diff --check

docker compose -f compose/docker-compose.dev.yml \
  up -d --build --no-deps --force-recreate plant-edge-os document-intake

docker compose -f compose/docker-compose.dev.yml ps
```

Only `plant-edge-os` and the new `document-intake` container are recreated. The new worker listens on **no public port**, and no unrelated iMac applications, containers or volumes are affected. The older Planning uploader now has `AGIPAREL_CAPTURE_FIRST=true` to prevent legacy auto-merge.

## 4. Smoke-test endpoints

```bash
curl -sS -o /dev/null -w 'Public homepage: %{http_code}\n' http://127.0.0.1:18530/
curl -sS -o /dev/null -w 'Documents without login: %{http_code}\n' http://127.0.0.1:18530/documents
curl -sS -o /dev/null -w 'Documents as dev: %{http_code}\n' -u dev:1212 http://127.0.0.1:18530/documents
curl -fsS -u dev:1212 http://127.0.0.1:18530/api/v1/documents

docker logs --tail=35 agiparel-document-intake
```

Expected HTTP `200`, `401`, `200`; the API returns `{"items":[], "tenant":"dev"...}` before uploads. Worker logs should show `document-intake worker: ready` and no repeated permission errors.

## 5. Run the **real customer demonstration**

Open **https://agiparel.empowerai.us/documents** and sign in as `dev` / `1212`. Drag in the original files directly from your customer ZIP (unzip them first on your Mac). You can select PDFs, XLSX, the two old XLS files, and any other files together. Upload all 14 customer documents with the built-in batching, and see per-file processing status. Click a document to inspect its page number, sheet/row, paragraph or CSV line evidence, and use Search for `REP3`, `REP4`, `PO`, `SAM`, `FABRIC` or `UPC`.

Based on the isolated extractor test against the **same 14-file corpus**:
- **7 PDFs:** `EXTRACTED`, page text segments preserved.
- **5 XLSX:** `EXTRACTED`, sheet/row/cell strings preserved, formulas retain their expression and any cached values.
- **2 binary XLS:** `UNSUPPORTED`, originals retained for conversion in 06.5-B.

`EXTRACTED` means *text evidence was captured*, **not** that Agiparel has automatically understood the complete business relationships or that orders exist in Batch 05 tables. Scanned PDFs/photos are captured but require OCR in 06.5-B. Native Google Docs/Sheets link import requires a connected, authorized export adapter in 06.5-B; downloaded exported originals work in the current UI.

You can also upload a file from Terminal, for example:

```bash
curl -fsS -u dev:1212 \
  -F 'files=@/path/to/Flat Front Pant WM-GM09Y094_PO Sheet.pdf' \
  http://127.0.0.1:18530/api/v1/documents/uploads
```

The `@/path/to/...` is a placeholder for your local extracted customer file. Use the browser if easier.

## 6. Validate persistence and isolation

```bash
docker exec -i agiparel-postgres psql -U agiparel_admin -d agiparel_dev \
 -c "SELECT tenant_id,format,count(*) FROM document_sources_v65 GROUP BY tenant_id,format ORDER BY tenant_id,format;"

docker exec -i agiparel-postgres psql -U agiparel_admin -d agiparel_dev \
 -c "SELECT status,count(*),sum(segment_count) AS segments FROM document_jobs_v65 GROUP BY status ORDER BY status;"
```

Recreate **only the worker**, reload `/documents` and confirm original files and extraction state persist:

```bash
docker compose -f compose/docker-compose.dev.yml \
  up -d --no-deps --force-recreate document-intake
```

## 7. Commit changes

```bash
git status --short
git add compose/docker-compose.dev.yml \
  services/core-api/migrations/0009_document_intelligence_v65.sql \
  services/document-intake \
  services/edge-daemon/cmd/server/document_intake.go \
  services/edge-daemon/cmd/server/document_intake_test.go \
  services/edge-daemon/cmd/server/main.go \
  services/edge-daemon/internal/web/assets.go \
  services/edge-daemon/internal/web/static/documents.* \
  services/edge-daemon/internal/web/static/order_intake.html \
  services/edge-daemon/internal/web/static/portal.html

git diff --cached --check
git diff --cached --stat
# Review, then:
git commit -m "feat: universal source document intake worker and evidence library"
git push origin main
```

## Current feature boundary / work explicitly still ahead

**Gate 06.5-B (next, still before Batch 07):** legacy XLS, OCR for scanned inputs, improved PDF tables, semantic extractors for multiple POs/style revisions/BOM/tech pack/OB/packing with field-level provenance, conflict review, Google authorized import. **Gate 06.5-C:** approval of reviewed structured entities creates real staged orders, connects to the Batch 06 scenario selector, and exposes source-grounded natural-language queries. Nothing in Gate A automatically advances a physical order or releases procurement/production.

### Verification performed in this environment

Standard-library Go tests passed for the extraction engine and isolated HTTP handler harness. `node --check` passed. All 14 attached customer documents were processed with 12 text extracted and 2 explicit unsupported XLS; ZIP/patch integrity and clean application were tested. **Not verified here:** full repository Go 1.24 build, live PostgreSQL migration/role policy execution and live browser-to-worker workflow; those are verified with the Mac commands above.
