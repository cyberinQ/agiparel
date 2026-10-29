<!-- Author: Sunil Jain, CyberinQ LLC, 2026-10-29. -->
# F3.20B–D / F3.21 — Dock custody evidence, verification, CI and recovery

Source baseline: `09f79f9` (already includes 0052). Schema increment: `0053_dock_scans_custody_asn_drafts_v34.sql`. This is a **software implementation**, not a factory-production signoff.

## What is implemented

1. **F3.20B — Dock gateway scans.** Every staged manifest carton must produce one fresh Ed25519-signed scan from an independently provisioned, tenant/plant-scoped dock gateway. The physical carton code must equal the immutable carton OPEN event, and the carton must belong to the manifest. Device signatures bind tenant, plant, gateway, carton, manifest, event ID and observation timestamp. Gateway key fingerprints are retained; revocation/rekeying revokes current verification. Application-level signature verification is not independent RF-certification.
2. **F3.20C — Container seal and carrier acknowledgement.** A FIDO2-backed `dispatch_supervisor` signs an independent seal after **all** cartons have fresh scans and live QC, needle safety and AQL gates pass. A separately enrolled `carrier_agent`, not the stager/sealer, signs a receipt referencing that exact seal. Provisioning is privileged-only, with revocable carrier/reader policies. These are attestations by onboarded people; the external carrier's network/API has **not** acknowledged custody.
3. **F3.20D — ASN preparation.** A separately signed `dispatch_coordinator` binds a payload SHA-256 and ASN reference to the carrier receipt. This creates an immutable **internal draft**, not an EDI/API transmission or a carrier ACK. No shipping/inventory credit, buyer lot acceptance, transport liability or accounting posting is minted.
4. **F3.21 engineering controls.** Three new action-specific workforce grants, RLS on seven tables, immutable event ledgers, transaction advisory locks, replay-proof hardening across v32/v33/v34, scoped read-only current verification status, PostgreSQL 16 schema + tenant-isolation CI, JavaScript regressions and disabled-by-default production toggles. Docker file ownership is adjusted for non-root persistent WAL. Backup and scratch-restore drill scripts are provided; not executed in this environment.

## API contract

All new routes require `AGIPAREL_DOCK_V34_ENABLED=true` **and** prior v32/v33 toggles, a staging/production environment, signed assertion verifier, PostgreSQL app role and provisioned identities. All routes are exact matches and deny unknown payload fields.

| Endpoint | Authority | Actor | Notes |
|---|---|---|---|
| `POST /api/v1/packing/dock/scans` | Ed25519 enrolled hardware gateway | gateway | JSON `event` / `signature_hex`; `event.version=V34`, RFC3339Nano time within 15 min, max 2 min ahead; body includes tenant/plant/manifest/carton IDs, carton code, gateway and event IDs |
| `POST /api/v1/packing/dock/seals/challenge` + `/seals` | signed assertion + `packing.dock.seal` | `dispatch_supervisor` | `CONTAINER_SEAL`; container ref, physical seal code, exact count |
| `POST /api/v1/packing/dock/carrier-receipts/challenge` + `/carrier-receipts` | signed assertion + `packing.dock.carrier` | `carrier_agent` | `CARRIER_RECEIPT`; seal ID, provisioned carrier code, receipt ref |
| `POST /api/v1/packing/dock/asn-drafts/challenge` + `/asn-drafts` | signed assertion + `packing.dock.asn.prepare` | `dispatch_coordinator` | `ASN_DRAFT`; carrier receipt ID, ASN ref, payload SHA-256 |
| `GET /api/v1/packing/dock/verification?tenant_id=T&plant_id=P&manifest_id=UUID` | workforce OIDC + `packing.read` | authorized viewer | recomputes current QC/AQL/safety and device/carrier enrollment; explicitly **no** shipment credit |

For signed commands, submit `{ "command": { "kind": "...", "command_id": "...", "plant_id": "...", "manifest_id": "...", "note": "..." /* plus kind-specific fields */ } }` to `/challenge`, then the **identical** command with a bearer FIDO2-backed assertion to the corresponding event endpoint. The signature challenge is `DOCK_HANDOVER_V34:` followed by the SHA-256 of canonical JSON. Do not manufacture an assertion or use an internal staff identity as a carrier without a legally accepted onboarding process.

Only a privileged provisioner can populate `dock_gateways_v34`, `dock_station_policies_v34`, `dock_carrier_authorizations_v34` and `workforce_identity_grants_v15`. All new admissions start **disabled**; no grants or device enrollments are seeded by the migration. A reader's public key is not an implicit proof it physically scanned a carton. Changes to provisioner-owned records require independently audited administrative procedures.

## Deployment and validation sequence

1. Work from clean `09f79f9` (or an equivalent tree that contains 0052), with separately preserved local changes. Back up PostgreSQL using `scripts/backup_factory_pg_v34.sh`; verify a **scratch restore** using `scripts/restore_factory_pg_drill_v34.sh`. An archive-format check is not a restore exercise.
2. Apply **only** migration 0053 on the existing upgraded database with `psql -X -v ON_ERROR_STOP=1 -f services/core-api/migrations/0053_dock_scans_custody_asn_drafts_v34.sql` using the privileged migrations connection, after a snapshot/backup and a maintenance window. This migration uses a transaction; investigate failures before retrying.
3. Execute `scripts/test_factory_f320b_f320d_schema_pg.sql` against the live schema (read-only) and `scripts/test_factory_v34_tenant_isolation_pg.sql` **only in a disposable test database** (writes two temporary tenant fixtures inside a rollback transaction). Validate earlier 0052 schema as well.
4. Run `go test -count=1 ./...`, `go test -race -count=1 ./...`, `go vet ./...`, `go build ./...`; run `for s in scripts/test_*.cjs; do node "$s"; done`. Run the PostgreSQL 16 migration CI job. Failure at any gate means **do not activate**.
5. Provision actual dock gateway keys, approved scanner-zone policy and carrier IdP subject mapping. Grant separate actions to appropriate humans, with no supervisor-stager overlap. Enable v32 → v33 → v34 flags **only** after tests and production change control, using image digests and workforce IdP keys. Validate a captured signed read before any supervised seal.
6. Perform real device/disconnect/replay/tamper tests and carrier custody procedures; verify physical labels, carton counts and independently re-read the manifest. Verify current status after each safety hold, revoked buyer document, reader/carrier revocation and new QC issue.

## Remaining production gates — NOT CLOSED

- Actual calibrated and enrolled readers, scales and detectors with onsite challenge/response and trusted timestamp source; hardware RF/optical physics is not proven by a signed packet.
- Authenticated external carrier/Electronic Data Interchange (EDI) transport receipt, verifiable buyer signoff, ASN transmission/acknowledgement and idempotent shipment/inventory posting. **Do not** reuse the internal ASN draft as a shipment event.
- End-to-end WIP/inventory/stock/accounting conservation under real multi-plant concurrency, delayed events, rescan conflicts and partial failures (database UNIQUE/advisory locks are defenses, not measured acceptance).
- Full authorization threat test across all older APIs, external IAM/rotation/secrets audit, independent pen test, production-load performance data and multi-region/multi-factory failover and PITR recovery proof.
- Dock/operator UI for supervised exception handling, accessibility, multilingual training, real buyer/carrier-specific workflows and documented factory UAT with responsible owners.

### Acceptance evidence log

Local source checks: JS contracts and Go formatting. **Not run here:** Go 1.24 test suite, PostgreSQL 16 migration, device commissioning, backup restoration, dynamic concurrency, full threat testing or factory UAT. CI and scripts are instrumentation to collect this evidence; their presence does not assert a passing result.
