<!-- Author: Sunil Jain, CyberinQ LLC, 2026-10-29. -->
# F3.22 — External signed counterparty assertions and conservative dock reconciliation (v35)

Baseline: `90dff56` (v34/migration 0053 present). New migration: `0054_external_acknowledgement_and_dock_acceptance_v35.sql`.

## Delivered

- Independently provisioned Ed25519 trust roots for carrier and buyer systems. Trust enrollment is tenant/plant/party scoped, begins disabled, and is provisioner-only (application can only read the trusted keys). App ingress verifies signatures over a fixed, versioned, domain-separated JSON message.
- Immutable, nonce/event-ID protected external carrier ASN acknowledgement (`CARRIER_ASN_ACK`) and buyer lot review (`BUYER_LOT_REVIEW` with `REVIEWED_ACCEPT` or `REVIEWED_REJECT`). SQL independently matches the immutable ASN draft, digest, manifest, plant, signer identity, clock window and current v34 dock safety.
- Idempotent same-payload retries; mismatched event-ID retries conflict. At most one assertion of each type per manifest. Historical record is never overwritten, including buyer rejection. Later key revocation, new QC holds and lapsed safety gates invalidate **current** reporting without erasing historic records.
- Conservative manifest/carton conservation: staged carton membership, one opening and seal, contents, current per-garment QC release, exactly matching enrolled gateway scans. This is **not** material/financial/WIP/warehouse inventory conservation.
- New scoped read-only verification endpoint using workforce OIDC and `packing.read`, a repeatable-read tenant-isolated database snapshot, separate current/stale flags and explicit zero shipment credit.
- PostgreSQL 16 RLS/role/immutability/negative tests; Go signature tamper tests; JavaScript contracts; CI schema coverage; one-command, backup-first release installer.

## API

| Endpoint | Method | Required authority | Result |
|---|---|---|---|
| `/api/v1/packing/dock/external-assertions` | POST | Independently signed, provisioned external Ed25519 event | Append-only external counterparty assertion |
| `/api/v1/packing/dock/external-verification?tenant_id=T&plant_id=P&manifest_id=UUID` | GET | Workforce OIDC and provisioned `packing.read` | Current evidence flags, no credit |

The POST payload is `{ "event": { ... }, "signature_hex": "<128 hex digits>" }`. The event requires `version: "V35"`, tenant/plant/manifest IDs, `asn_draft_id`, signer/party IDs, `assertion_kind`, `decision`, unique `event_id`, `external_reference`, **exact** `asn_reference` and 64-character `payload_sha256` from the v34 ASN draft, plus an RFC3339Nano `issued_at`. The signed bytes are `AGIPAREL_EXTERNAL_ASSERTION_V35\n` followed by compact JSON encoded in the **declared Go struct field order**. Signers must use the published canonical field order exactly; integration tests with real vendor payloads are mandatory.

**The POST is disabled unless `AGIPAREL_DOCK_EXTERNAL_V35_ENABLED=true` and all required v32/v33/v34 gates are also enabled.** This release does not automatically set any flag, trust key or authorization grant. HTTPS/mTLS/IP allowlisting, key ownership evidence, clock synchronization, key rotation, provider contracts and retry behavior require real integration commissioning.

## Critical authority boundaries

- A signed carrier acknowledgment from a provisioned counterparty is a recorded assertion, **not proof that an ASN was transmitted by this application, nor a verified network delivery receipt**. No EDI/API transport adapter is bundled.
- A signed buyer lot review is not automatically a legally valid final buyer acceptance. Party identity is provisioned but **not independently tied to an order's buyer contract** by this patch.
- External assertions do **not** post shipments, mark garments shipped, remove stock, create invoices, transfer legal title or book revenue. No complete global inventory/WIP conservation algorithm or dynamic multi-factory load proof exists yet.
- The current-state response can become false when a signer, scanner or buyer rule is revoked, or safety/QC evidence changes. Historical append-only evidence remains auditable.
- Production rollout requires security review for event ingress, independent counterparties, dedicated credentials, HMI/operator exception handling, real buyer/carrier integration, simulated concurrency/load and actual site UAT. **No production release authority is conferred by a green unit test.**

## Validation and deployment

From clean commit `90dff56`, use `scripts/install_release_v35.sh` from the patch ZIP, with the release ZIP's published SHA-256. The installer fails closed on dirty baseline, backs up before schema migration, applies 0054 once, backs up again, restores a disposable database, executes v33/v34/v35 schema tests, executes full Go/race/vet/JS tests, builds, deploys, health-checks, stages only patch files, commits and pushes. Failed migration/validation leaves the existing database for inspection; **do not rerun blindly**. No real counterparty enrollment or acceptance is performed automatically.

Additional acceptance matrix to conduct onsite: two tenants × two plants; key rotation/revocation; exact replay vs changed-payload replay; concurrent duplicate submissions; stale timestamps; buyer rejection; removed QC release; new needle incident; AQL-rule revocation; carton rescan/duplicate; carrier cross-plant spoofing; interrupted network; backup PITR and disaster recovery drills. Capture independent signed evidence, human owner, timestamp and result for each case.
