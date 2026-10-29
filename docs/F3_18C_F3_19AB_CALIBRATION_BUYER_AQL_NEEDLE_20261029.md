# F3.18C + F3.19A/B: Calibration Linkage, Buyer Sample Audits, and Needle Safety Evidence

Author: Sunil Jain, CyberinQ LLC — 2026-10-29. Baseline `74c6e53`. Migration `0051_calibration_aql_needle_evidence_v32.sql` follows 0050.

## Production boundaries

1. **F3.18C:** Append-only, provisioner-owned instrument calibration revisions reference source-preserved SHA-256 documents and validity periods. A second, different worker signs the association between a human-reported fabric shrinkage sample and the current in-period `DIMENSION` calibration. This association is **NOT** a device-signed reading, machine calibration certificate verification, or cutting release. The existing shrinkage readiness includes `human_linked_calibration_samples` but preserves its uncalibrated/uncertified readiness classification.
2. **F3.19A:** Provisioner-owned buyer AQL plans require an approved CUSTOMER document and immutable, explicit selected garment UUIDs from one lay. A unique separately signed human sample finding (`PASS`, `MINOR`, `MAJOR`, or `CRITICAL`) requires that garment's preceding 100% QC `QUALITY_RELEASE`. Only selected, current-plan garments can be recorded; defects must be enrolled. AQL counts **DO NOT** perform accept/reject-number evaluation or grant buyer acceptance, production credit, or shipping authority. No stochastic or invented sample selection.
3. **F3.19B:** Separately signed human reports of needle breakage and metal-detector checks reference actual garment genealogy and source-preserved documents. Detector events require a provisioned currently valid `METAL_DETECTOR` calibration. Any reported needle break, challenge failure, or reject causes a database-level block on subsequent `QUALITY_RELEASE` or carton `ADD` for that garment. These **cannot** be cleared through this release; independent safety investigation/clearance must come in a later controlled increment. Existing releases are not retroactively rewritten.

All writer endpoints fail closed unless `AGIPAREL_FACTORY_EVIDENCE_V32_ENABLED=true`, `AGIPAREL_MATERIAL_API_ENABLED=true`, a trusted Ed25519 IdP and authority holder are configured, and `ENV` is `staging` or `production`. Rights are independently provisioned: `stores.fabric.calibration.link`, `quality.aql.inspect`, `quality.needle.report` (respectively `stores_inspector` / `quality_inspector`). Readers additionally require existing OIDC and `quality.read`. This release grants nobody a role and seeds no inventory, workers, instruments, samples, or factory observations.

## API

* `POST /api/v1/factory/evidence/challenge`: `{ "command": { ... } }`, exact challenge for one of `CALIBRATION_LINK`, `AQL_SAMPLE`, `NEEDLE_SAFETY`.
* `POST /api/v1/factory/evidence/events`: same command plus bearer strong signed identity assertion (not a self-issued token).
* `GET /api/v1/quality/aql/sampling?tenant_id=...&plant_id=...&plan_id=...`: counts sampled, uninspected, PASS/MINOR/MAJOR/CRITICAL; explicitly no buyer lot acceptance.
* `GET /api/v1/quality/needle/incidents?tenant_id=...&plant_id=...&garment_id=...`: signed human incident counts; no safety clearance.
* Existing `GET /api/v1/fabric/shrinkage/readiness` now also returns `human_linked_calibration_samples`; it never claims physical-device certification.

All writes are append-only, command-idempotent with exact payload and signer match, tenant/plant scoped under forced RLS, and verified through IdP assertions. Database triggers enforce current instrument rev, approved sampling, current genealogy, proof uniqueness across this increment, and unresolved safety gate. Admin-approved plans/calibrations are provisioner-only.

## Acceptance sequence

1. Before changes, confirm clean Git HEAD `74c6e53` and back up the DB.
2. Install changed files and apply SQL migration 0051 **once**, in one transaction. The schema test is read-only and does not fabricate data.
3. Run `scripts/test_factory_f318c_f319ab_schema_pg.sql`, `go test -count=1 ./...`, `go test -race -count=1 ./...`, `go vet ./...`, `go build ./...`, and all `scripts/test_*.cjs`.
4. Rebuild/restart `plant-edge-os` and inspect logs. Do not activate production features without instrument/provisioned-buyer-source acceptance, access tests, and real staff enrollment.
5. Only after all checks succeed, commit/push the changed files listed in the ZIP.

## Future acceptance

Needs device-signed measurement ingestion, traceable calibration authority validation, statistical buyer AQL accept/reject rule evaluation and independent buyer auditor sign-off, needle-room fragment reconciliation and independent safety clearance, and multi-user PostgreSQL replay/concurrency tests against a disposable plant clone. These are intentionally **not** claimed in this increment.
