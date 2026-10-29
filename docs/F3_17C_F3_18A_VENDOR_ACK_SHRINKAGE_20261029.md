<!-- Author: Sunil Jain, CyberinQ LLC, 2026-10-29 -->
# F3.17C + F3.18A — Vendor Signed Acknowledgements and Fabric Shrinkage Observations

**Base commit:** `b1085cd` (F3.17B). **Schema:** migration `0049_vendor_ack_and_fabric_shrinkage_evidence_v30.sql` (apply only once after `0048`). **Release posture:** fail closed, no grants provisioned and no production events generated.

## F3.17C — Cryptographically distinct external acknowledgement

A pre-approved subcontractor may sign a *per-part outward acknowledgement* with its own Ed25519 private key. The vendor key is enrolled by a trusted provisioner in `jobwork_vendor_keys_v30`: `tenant_id, plant_id, vendor_id, key_id, public_key_hex, enabled, registered_by`. `agiparel_app` has SELECT only on this table; users must never upload vendor private keys or issue SQL inserts in production through the application role.

1. Existing `jobwork_outwards_v28` must exist for the same tenant, plant, vendor, **part ID, job reference and process**.
2. The `POST /api/v1/jobwork/vendor-ack/challenge` request carries `command` (tenant, plant, outward ID, part ID, job reference, process, vendor, key ID, command ID, `ack_kind=OUTWARD_REPORTED_RECEIVED`). Response contains the exact UTF-8 text to be signed as a detached Ed25519 signature by the **external vendor**. The signature is of the *challenge bytes*, **not** a SHA-256 digest of those bytes.
3. The internal receiving/recording supervisor signs the *same command challenge* with a separately enrolled strong assertion for `jobwork.vendor.ack.record`, role `sewing_supervisor`.
4. `POST /api/v1/jobwork/vendor-ack` contains `{ "command": {...}, "vendor_signature_hex": "..." }` and uses the signed supervisor Bearer assertion. The API checks the vendor signature against its tenant/plant/vendor/key-scoped public key and inserts an immutable record containing vendor signature, public-key fingerprint and internal assertion fingerprint. The internal recorder must not be the original outward sender.
5. The existing `GET /api/v1/jobwork/reconciliation` additionally returns `vendor_ack_evidence_count` and per-part `vendor_ack_recorded` flags, **without replacing physical outward/inward counts**.

The proof means the holder of the enrolled vendor private key acknowledged the specified outward record. It is **not independent physical RF proof**, a carrier handover guarantee, a quality result, a payment authorization, or settlement of a shortage/damage case. Existing F3.17B disputes remain blocked. **F3.17C does not implement supervised discrepancy clearance or group manifest signing; those remain outstanding.** A privileged administrator must manage real vendor key enrollment, rotation and revocation; the patch does not create keys.

## F3.18A — Human-attested physical sample dimensions

`POST /api/v1/fabric/shrinkage/challenge` generates a domain-separated challenge for a real `roll_id`, matching plant and SKU, `sample_id`, treatment (WASH, STEAM, RELAXATION, OTHER), before/after warp and weft sample lengths **in integer millimeters**, a retained document `source_document_id`, matching `source_sha256`, and a detailed method note. `POST /api/v1/fabric/shrinkage/observations` submits that exact command using a challenge-bound `stores.fabric.shrinkage.record` inspector assertion. `AGIPAREL_FABRIC_SHRINKAGE_ENABLED=true` and configured material IdP/authority are necessary.

The schema verifies the stock roll's tenant, plant and SKU, the preserved source document SHA-256, bounds and positivity of measured dimensions, exclusive sample identity and single-use signed command. PostgreSQL computes `warp_change_bp` and `weft_change_bp` as basis points of **(before − after)/before × 10,000**. Thus `500mm → 475mm` is `+500 bp` (5% shrinkage); `400mm → 408mm` is `−200 bp` (2% expansion). The saved evidence is human-reported dimensional change, not independently calibrated equipment telemetry; an original-document SHA-256 is **not buyer approval**.

**Do not gate marker selection or production release on a bare measurement.** F3.18B needs an independently approved, versioned fabric/style shrinkage specification, explicit lot-to-roll coverage, instrument calibration identity, required process conditions, and a buyer-quality assessment before enforcing an appropriate cutting-release gate. No invented testing history or pass/fail is created here.

## Safety / installation

1. Confirm exact clean commit `b1085cd`. Validate zip and file conflicts before extracting.
2. Back up the existing PostgreSQL database. Apply `0049` once via `psql -v ON_ERROR_STOP=1`; migration is atomic `BEGIN`/`COMMIT`.
3. Execute read-only `scripts/test_factory_f317c_f318a_schema_pg.sql`, then `go test -count=1 ./...`, `go test -race -count=1 ./...`, `go vet ./...`, `go build ./...`, and all `scripts/test_*.cjs`.
4. Rebuild and restart `plant-edge-os` in development only after every check passes, then commit/push the exact changed files.
5. No migration rollback is attempted by deleting append-only ledgers. Recover from a verified database backup and a tested plan if rollout must be reversed.

**Remaining live acceptance:** PostgreSQL 16 migration, full Go 1.24 integration, real provisioned vendor key/signature exchange and textile sample measurement validation. Local unit tests do not constitute physical device commissioning.
