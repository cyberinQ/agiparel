<!-- Author: Sunil Jain, CyberinQ LLC, 2026-10-29 -->
# F3.17D + F3.18B — Signed Job-Work Disposition and Shrinkage-Spec Readiness

**Required git baseline:** `ba1cd5f` (F3.17C/F3.18A). **Migration:** `0050_jobwork_resolutions_and_shrinkage_specs_v31.sql` after `0049`. Both capabilities are OFF unless the existing F3.17 and F3.18 secure-read/write prerequisites have been provisioned. No dummy suppliers, buyers, equipment, physical observations, worker grants or finished goods are created.

## F3.17D — Audited discrepancy dispositions

The new `POST /api/v1/jobwork/dispositions/challenge` provides a domain-separated challenge for an existing F3.17B discrepancy. `POST /api/v1/jobwork/dispositions` requires the *same* command bound to a strong, fresh `webauthn` or `fido2` assertion from an independently enrolled `sewing_supervisor` with `jobwork.discrepancy.resolve` permission. The claim, exact plant, actual discrepancy, outward and part IDs are checked under tenant RLS. Signer must differ from the original sender, return receiver (if any), and discrepancy reporter. Proof, command and discrepancy are single-use. No old evidence is updated or erased.

Command fields: `command_id`, `plant_id`, `discrepancy_id` (UUID), `outward_id` (UUID), `part_id` (UUID), `resolution_kind`, `evidence_ref`, `evidence_sha256`, `note` (20–1000 characters). Allowed dispositions:

- `RETURNED_TO_PIECE_HOLD`: requires a real F3.17A inward return, and an **active independent F3.16A garment hold opened at/after the discrepancy**. A part reported missing requires its inward return to have been recorded **after** that missing report. Only then may the F3.17B job-work-dispute guard yield to the separate F3.16A hold; the piece is not QC-approved, not production-credited, not financially settled. WIP still needs its own fresh signed hold-review evidence.
- `SHORTAGE_REPORTED_REMAINS_BLOCKED`: only a still-unreturned missing part. Acknowledges an unresolved administrative shortage, not a proven write-off, insurance claim or vendor payment deduction.
- `DAMAGE_REPORTED_REMAINS_BLOCKED`: only a reported damaged inward return. Stores an administrative disposition while retaining the database dispute gate. Recut, independent QC and any finance decision are separate workflows.

The existing GET `/api/v1/jobwork/reconciliation` includes `disposition_record_count` and per-row `disposition`/`status`. Outward/inward counts are still computed from the original immutable custody events. The database helper `jobwork_part_disputed_v29` and all six F3.17B downstream protection triggers still block unresolved outcomes; only properly recorded recovery transferred to an independent hold is no longer an *open vendor dispute*.

## F3.18B — Source-approved specification, versioned and immutable

`fabric_shrinkage_specs_v31` is provisioner-owned. The application role can SELECT but cannot INSERT/UPDATE/DELETE specifications. Every revision belongs to tenant, plant, material SKU and treatment (WASH/STEAM/RELAXATION/OTHER) and stores the approved customer document ID and exact SHA-256, independent warp/weft minimum and maximum in signed basis points, number of required samples, author and time. A before-insert trigger checks the existing `documents` row is `APPROVED`, `issuer=CUSTOMER`, hash-equal and of a relevant technical-specification type. Revisions are append-only; the **highest revision** takes effect. A new highest revision with `enabled=false` revokes the spec. If the customer source becomes superseded, the assessment reports no current approved spec.

`GET /api/v1/fabric/shrinkage/readiness?tenant_id=...&plant_id=...&roll_id=...&treatment_code=WASH` requires authenticated tenant/plant-scoped `stores.read`, a forced-RLS **read-only repeatable-read** transaction, and the existing material/shrinkage feature prerequisites. It reports one of:

- `NO_CURRENT_APPROVED_SPEC` — no trustworthy currently provisioned customer specification, including after revocation;
- `INSUFFICIENT_REPORTED_SAMPLES` — fewer distinct F3.18A recorded test samples than required;
- `OUTSIDE_SPEC_HOLD_FOR_REVIEW` — at least one recorded sample outside the independent warp or weft bounds;
- `WITHIN_LIMITS_REPORTED_UNCALIBRATED` — all the *reported*, uncalibrated observations fall within the provisioned limits;
- `ROLL_INSPECTION_NOT_PASS` — stored fabric lot has not passed its separate inventory inspection.

The result **never** authorizes cutting, marker choice, garment QC, shipment or payroll. F3.18A measurements are human-reported and are not yet checked against calibrated instruments, laboratory method, fabric lot sampling coverage, buyer size/style restrictions or marker release. This is a truthful pre-cutting *evidence readiness* assessment, not a cutting-state transition. A future explicit guarded approval can require calibrated instruments and buyer-quality sign-off.

## Deploy / acceptance

1. Install only on a clean `ba1cd5f` checkout. Verify ZIP integrity and untracked filename conflicts before extraction. Save a `pg_dump -Fc` of the PostgreSQL database before migration. Run `0050` **once** with `-v ON_ERROR_STOP=1`, and then the **read-only** `scripts/test_factory_f317d_f318b_schema_pg.sql`.
2. Run full `go test -count=1 ./...`, `go test -race -count=1 ./...`, `go vet ./...`, `go build ./...`, then all `scripts/test_*.cjs`. Rebuild and restart `plant-edge-os` only if all checks pass.
3. Commission with actual independent supervisors, externally provisioned vendor identities, accepted physical return scans, distinct strong assertions, and existing appropriate piece holds. Confirm duplicate/disputed command, invalid cross-plant scope, unrelated part, reused signature, no inward event and missing hold are denied by database. Do not fabricate events to test a live factory.
4. Provision real customer-approved spec revisions using the controlled database-provisioning workflow, not by permitting application INSERT on spec table. Verify absent spec, disabled later revision, document supersession, missing samples and out-of-range samples all fail closed.
5. Commit and push the changed-files ZIP **only after** iMac PostgreSQL 16 + Go 1.24 integration checks succeed. Do not reapply a partially failed deployment blindly; a migration error inside the transaction rolls back database DDL but not already extracted files.

**Remaining work:** multi-piece vendor-signed dispatch manifests, actual financial loss settlement, calibrated scale/sensor ingestion, buyer-approved cutting gate, and explicit instrument calibration/certification. None is claimed by F3.17D/F3.18B.
