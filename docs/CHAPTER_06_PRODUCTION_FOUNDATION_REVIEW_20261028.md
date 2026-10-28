# Agiparel Chapter 06 — Production evidence foundation

**Author: Sunil Jain, 2026-10-28.**

## Scope and integrity verdict

This increment introduces **production-only, authenticated evidence ingestion and worker-attested sewing operation records**, isolated from the Chapter 06 simulated exercises and downstream simulated QC, packing, intelligence, and payroll-like counters.

**Release classification: PILOT-ONLY. It is not a finished, certified production Factory OS.** No real gateway, IdP, administrator, cut-release or factory acceptance was provisioned in this code delivery. Test results show code contracts, not verified factory operation.

### Production-safe sequence

1. Cutting supervisor and quality officer create an independently signed release (`cut_sewing_releases_v8`) using existing Chapter 05 controls; simulation cutting draft alone is insufficient.
2. An authorized **out-of-band** administrator enrolls each gateway's Ed25519 public key into `sew_prod_gateways_v10` with exact tenant, plant, station, and `enabled` status. The device private key remains outside Agiparel.
3. A separate authorized administrator publishes an approved, revision-specific operation route in `sew_prod_routes_v10` with required parts, predecessor operations and station assignment. A route starts disabled.
4. Provision an IdP subject's plant/station assignment in `sew_prod_worker_stations_v10`; it starts disabled. No browser-entered name or station can grant itself rights.
5. The device/gateway reports a **signed** EPC observation. Server verifies the signature against the enrolled key, exact scope, clock, original cut bundle, EPC-human-witness match, and signed cutting release. One event ID is idempotent; altered replay conflicts. An observation cannot predate the signed release (2-minute clock tolerance).
6. For an operation, submit an immutable command with its unique observation IDs to `POST /api/v1/sewing/production/completions/challenge`. The IdP verifies the human using WebAuthn/FIDO2 and signs a JWT with `action=sewing.complete`, `role=sewing_operator`, challenge equal to the returned challenge, and exact `tenant_id`/`plant_id`.
7. Submit that **same** command to `POST /api/v1/sewing/production/completions` using `Authorization: Bearer <IdP-JWS>`. The server and database enforce route, plant, station assignment, release, all required parts, matching marker/group/size/color/plies, predecessors and unique non-reused observations. A duplicate command with the same content returns an idempotent replay; a changed command ID payload fails.
8. Append-only completion is labeled `WORKER_ATTESTED_PENDING_QUALITY_ACCEPTANCE`. Its `nominal_ply_count` is **not** garments inspected/accepted, verified machine runtime, payroll credit, warehouse inventory, shipment release, or realized margin.

**No operator or device can use shared development Basic Auth to write these records.** The narrow production endpoints bypass the obsolete Basic gate because they enforce their own trusted identities. All legacy APIs remain closed in a non-development deployment unless separately upgraded; no generic unauthenticated operator bypass was introduced.

### Gateway Ed25519 signed wire contract

`POST /api/v1/sewing/production/observations` accepts `{ "event": {...}, "signature_hex": "..." }` with exact event fields:

- `version`: `SEW_PRODUCTION_OBSERVATION_V1`
- `tenant_id`, `plant_id`, `gateway_id`, `station_id`, `event_id`, `lay_id`, `label_code`, `epc_hex`, `observed_at` (UTC RFC3339Nano)

The signature covers these **ten fields, in this exact order**, as UTF-8 bytes:

```
AGIPAREL_SEW_READER_V1\n
<version>\n
<tenant_id>\n
<plant_id>\n
<gateway_id>\n
<station_id>\n
<event_id>\n
<lay_id>\n
<label_code>\n
<epc_hex>\n
<observed_at>\n
```

Each `\n` above represents one actual LF byte (0x0A). The final value is LF-terminated. Field validation prevents embedded line breaks. The detached signature is the lower/upper hex encoding of a 64-byte Ed25519 signature over these bytes. The gateway's private signing key must never be sent to the server, checked into Git, or embedded in a browser.

A device report **authenticates a gateway's claim**, not independent proof of genuine RF propagation, an untampered antenna, or correct sewing. Devices require site commissioning, tamper controls, calibrated reader validation, and monitored enrollment/revocation.

### Completion request contract

Challenge and completion endpoints use the same JSON body `{"command": {"command_id": "...", "lay_id": "...", "plant_id": "...", "station_id": "...", "route_id": "...", "marker_position": 1, "ply_group": 1, "observation_ids": ["<uuid>"]}}`.

The challenge endpoint returns the exact digest to be signed by the IdP. Clients **must not** derive trust from fields they themselves typed; the IdP authenticates the human and the server checks the signed tenant/plant against the approved route and worker assignment. The completion accepts a short-lived strong-auth JWS and does not accept browser-supplied `worker_id`, `garment_count`, `production_credit`, or fabricated RSSI values.

### Configuration — default disabled

- `ENV=production` (or `staging`), never `development`.
- `AGIPAREL_SEWING_PRODUCTION_ENABLED=true` opt-in, **after migration and provisioning**.
- `AGIPAREL_IDP_ED25519_PUBLIC_KEY_B64` pinned Ed25519 public key, `AGIPAREL_IDP_KID`, `AGIPAREL_IDP_ISSUER`, `AGIPAREL_IDP_AUDIENCE`.
- `AGIPAREL_SEWING_MAX_OBSERVATION_AGE` duration such as `48h`; default is 48h, allowed 1 minute–30 days. Committed event replays can be retried later, but new stale observations cannot be admitted.
- Existing `DATABASE_URL` must use restricted application role `agiparel_app`; `LEDGER_SECRET` must be managed securely. Do not expose port 18530 publicly: use TLS termination/mTLS gateway-to-proxy, rate limits, network controls, health probes, audit forwarding, and secret management.

The optional `compose/docker-compose.production.yml` deliberately has **no development PostgreSQL, sample credentials, public-facing port or built-in IdP**. It requires an approved digest-pinned image, externally provisioned database with SSL verification, IdP keys, a localhost-bound reverse-proxy ingress, and a writable WAL volume pre-owned by UID 10001. It is a **single-instance production-pilot profile**, not evidence of high availability. The development Compose file must never be used for production.

Migration `0029_sewing_production_evidence_v10.sql` creates six independent tables, two lineage triggers, and append-only RLS for evidence. **No rewrite or deletion of Chapter 01–06 histories.** Operator role has SELECT-only access to gateway enrollment, operation routes and station approvals; it cannot approve itself. Migration must run once, by a privileged migration role, in a backup-protected database.

### Acceptance before real factory rollout

- Verify genuine reader hardware and IdP/WebAuthn integration; test gateway key rotation/revocation and physically swapped tags.
- Exercise real Postgres 16 integration including concurrent same-observation and same-command submissions, multi-tenant isolation and RLS bypass attempts, migrations and rollback backups.
- Test partition and offline replay under sustained throughput and staged network outages; quantify p95/p99 latency and database saturation across multiple plants.
- Implement production operator sign-in/HMI and supervisor correction process (append-only compensating records), distinct from the still-simulated `/sewing` UI.
- Govern approved routing versions and worker assignments through real administration workflows and periodic revocation.
- Implement authoritative QC acceptance, recut/rework, finished goods, dispatch, and finance reconciliation in later chapters. `sew_prod_completions_v10` alone must never become shipping/payroll evidence.
- Address existing upstream schema physical constraints (e.g., 210-ply, 80-marker-position assumptions) through separate versioned schema/acceptance work. This increment does **not** remove them.

### Regression status and limits

Passed: 12 existing JavaScript workflows + new production static-contract check; native Go isolated contract tests (signatures, tampering, challenge binding, opt-in, timing). The sandbox lacks Go 1.24 and PostgreSQL runtime; **full module `go test`, actual migration, and transaction integration are unverified until your iMac and staging Postgres run them**. This is why the release remains PILOT-ONLY.
