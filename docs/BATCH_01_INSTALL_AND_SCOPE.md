# Agiparel Factory OS v5.1 — Code Delivery 01

Author: Sunil Jain · 2026-10-26

## Scope and status

This delivery is a first, bounded implementation slice, NOT completion of 109 stories. It supplies drop-in Go domain logic, local tests, an additive PostgreSQL migration, a guarded existing-server patch, and test-gated CI. It does not claim a commissioned RFID plant, full CEO authentication, durable issue HTTP endpoints, an existing production-quality material inspection process, or complete quality/finishing/packing/OCR.

| Canonical story / debt | Delivered | Still blocking closure |
|---|---|---|
| E22-S01, V5-T11–T13 | Classic actual-ply bundle grouping, per-position/part genealogy, coherent serial proposal, bounded remainder grouping | Authoritative per-PO serial allocation, persisted physical cut acceptance, card printer and issuance/custody gates |
| E19-S05, V5-T06 | Six-dimensional, conservative surplus eligibility and net indent advisory | Live inventory snapshot, reservation commit/recheck, approved procurement workflow/UI |
| E23-S03, V5-T15/T18 | Typed operation contract/pairing/cardinality validator | Authenticated station, OB publication, physical events, durable anti-double-spend and credit UI |
| E19-S03, E19-S04, V5-T02/T05 | PostgreSQL serializable material reservation/issuance repository, RLS, DB over-reservation trigger, outbox, independent witness verification interface and proof replay uniqueness | Production witness verifier implementation, trusted identity middleware, service registration, live PostgreSQL migration and race/restart fixtures |
| TD03 / TD05 | Fail-closed old material-issuance HTTP route except explicitly marked development demo; no automatically seeded production roll | Remove demo handler after replacement with audited secure API; eliminate all other demo defaults |
| TD08 / TD09 | CI/test/vet/race/build gate and release workflow with tagged-commit validation | Go 1.24 CI proof, device tests, migration apply and rollback drills, service integration checks |

## File placement

Add the following files at their exact repository-relative paths:

- `services/edge-daemon/pkg/cutting/classic_bundle.go`
- `services/edge-daemon/pkg/cutting/classic_bundle_test.go`
- `services/edge-daemon/pkg/stores/surplus_gate.go`
- `services/edge-daemon/pkg/stores/surplus_gate_test.go`
- `services/edge-daemon/pkg/stores/material_repository.go`
- `services/edge-daemon/pkg/stores/material_repository_test.go`
- `services/edge-daemon/pkg/sewing/operation_contract.go`
- `services/edge-daemon/pkg/sewing/operation_contract_test.go`
- `services/core-api/migrations/0005_material_transaction_spine.sql`
- `.github/workflows/ci.yml`

Replace exactly one existing file:

- `.github/workflows/release.yml`

Apply `TD03_TD05_fail_closed.patch` from the repo root with `git apply --check <path>` then `git apply <path>`. This is a surgical patch for `services/edge-daemon/cmd/server/main.go`, not a whole repo or a wholesale main.go replacement.

Delete nothing. Existing duplicate `pkg/*` trees must be consolidated only after import-graph and integration review. Do not duplicate these new implementations into both trees: the edge daemon currently imports `services/edge-daemon/pkg/*`.

## Safeguards

1. The old `/api/v1/stores/issue-double-witness` endpoint is disabled with HTTP 503 in all environments except when BOTH `AGIPAREL_ENVIRONMENT=development` and `AGIPAREL_DEMO_ALLOW_UNAUTHENTICATED_ISSUANCE=true` are explicitly set. These variables MUST NOT be set in production.
2. A demonstration roll is seeded ONLY when `AGIPAREL_ENVIRONMENT=development` and `AGIPAREL_DEMO_SEED=true`.
3. The old `/api/v1/stores/indent-check` receives an `X-Agiparel-Authority: ADVISORY_ONLY` header. Its quantities come from the caller, so it MUST NOT authorize purchase orders.
4. `MaterialRepository.Issue` refuses a missing witness verifier. Its verifier interface must be backed by real independent authentication with freshness, challenge-binding, and verified tenant context; the test verifier is NOT a production implementation.
5. New stock inspection status defaults to `UNVERIFIED` and must be changed by an authorized GRN/inspection workflow. Do not mass-update historical stock to PASS without physical evidence.
6. All material quantities within the new transactional domain are integer millimeters, avoiding floating-point stock accounting. Existing material rolls are migrated to three decimal places in meters; audit rounding before migration.
7. The migration must be reviewed/tested against a backup of the target PostgreSQL schema. This environment did not have PostgreSQL or the repository's required Go 1.24 toolchain, so end-to-end database tests remain unverified. Roll back using a tested forward repair, not a blind drop of a ledger that contains real custody events.
8. Preserve per-order serial counter allocation in an owning database transaction. Pure `BuildClassicBundles` produces proposed cards only; it does not print them or issue tags.

## Verification performed

- `GO111MODULE=off go test -race -count=1 ./...` completed successfully for `services/edge-daemon/pkg/{cutting,stores,sewing}` with the new files copied into the exact existing source directories.
- `GO111MODULE=off go vet` completed for the standalone stores files.
- The main.go patch passed `git apply --check` against the uploaded source snapshot.
- Full `go test ./...`, PostgreSQL migrations, RFID devices, and any live station/pilot evidence were **not** verified in this runtime.

## Deployment sequencing

Run new Go domain tests and CI on a clean branch, obtain migration approval, run DB migration and tenant-isolation tests in staging, implement the authenticated witness verifier and trusted tenant middleware, then wire only the new repository into production routes. Do not re-enable the original unauthenticated issuance endpoint.
