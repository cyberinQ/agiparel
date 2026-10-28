# F3.3 — Signed WIP custody, RFID/barcode scans and interprocess timing
Author: Sunil Jain, 2026-10-28

**Status:** production-pilot evidence plumbing, not commissioned factory instrumentation. Development UI remains unchanged. Migration 0039 follows 0038. Baseline `fe94fa8`.

## Factory operation / physical trust

1. F3.1 (`gen_cut_parts_v18`) gives each cut part a permanent identity; a reusable RFID tag (`gen_carriers_v18`) is a temporary physical carrier and must be **enrolled, enabled and actively bound to that exact CUT_PART**. Retired tags remain historical and are never permanent unit IDs. F3.2 (`cut_part_outcomes_v19`) must independently attest `ACCEPTED` and its reconciled signed material source before the part is eligible for a scanner event.
2. F3.3 `POST /api/v1/wip/production/scans` accepts an Ed25519 signature from an already-enrolled **plant/station-specific** gateway (`sew_prod_gateways_v10`). It stores source event ID, reader, carrier ID and EPC, physical cut-part ID, observed and server-received timestamps, signature hash and payload hash. No hard-coded equipment models. The database captures the exact active `binding_event_id` at scan time; later RETIRE + re-BIND cannot reuse an old scan, even if re-bound to the same part. One gateway/event ID is idempotent, and the command cannot be overwritten with different bytes.
3. `POST /api/v1/wip/production/movements/challenge` derives a transaction-bound challenge for `DEPART` or `ARRIVE`; a real operator must supply a short-lived IdP assertion with `sewing.transfer`, role `sewing_operator`, and strong `webauthn`/`fido2`. Each assertion requires an independently provisioned tenant+plant action grant, plus an active worker assignment to the exact station.
4. `POST /api/v1/wip/production/movements` requires a new signed scan associated with the same cut part and station, plus a DBA-approved enabled route (`wip_transfer_routes_v20`). `DEPART` requires the source station; `ARRIVE` requires the destination station, an outstanding departure on that same route, and **a different authenticated worker subject**. Next departure must originate at the preceding destination. Per-part SQL advisory locking, sequence checks and unique scan consumption prevent double arrivals, concurrent duplicate movement and out-of-order custody changes. Only provisioners can configure routes; no application INSERT grant.
5. `GET /api/v1/wip/production/history?part_id=<uuid>&limit=50&cursor=<event_no>` returns authenticated, tenant/plant-scoped movement history (cursor=older **per-part** sequence). `GET /api/v1/wip/production/metrics?route_id=<uuid>&since=<RFC3339>&until=<RFC3339>` returns a **31-day-or-less** window of completed handoff counts and server-recorded interval min/max/mean/sample standard deviation/p50/p95. Empty distributions return null metrics, not an invented zero duration. Reads require strong `sewing.read` and independently provisioned grant.

The server timestamps measure **time between independently submitted, signed DEPART and ARRIVE attestations**. They can approximate interprocess delay but **cannot establish actual travel/handling duration, machine cycle time, efficiency, worker productivity, payroll credit, garment acceptance or shipment credit**. In particular a registered gateway signing an EPC read does not independently prove the tag was physically at that station. Never conflate this with a calibrated, authenticated optical/RFID sensor measurement.

## API wire contracts

Signed reader event:

```json
{"event":{"version":"WIP_GATEWAY_SCAN_V1","tenant_id":"tenant-A","plant_id":"plant-A","gateway_id":"reader-A","station_id":"cutting-out","event_id":"rf-observation-0000001","part_id":"<UUID>","carrier_id":"<UUID>","epc_hex":"<CANONICAL_UPPERCASE_EPC>","observed_at":"2026-10-28T22:00:00Z"},"signature_hex":"<128_HEX_CHARS>"}
```

Sign exactly UTF-8 bytes `AGIPAREL_WIP_GATEWAY_V1\n` followed by the ten event fields in struct order, newline-separated, ending with newline. Factory gateway key provisioning occurs outside the application. Reader clock should be NTP-synchronized; late events beyond `AGIPAREL_WIP_MAX_SCAN_AGE` (default 15m, configurable 1m–24h) are rejected. Timestamp and signature validation happen before database insert; the database additionally confirms gateway scope, part acceptance, carrier binding and plant lineage.

Worker command:

```json
{"command":{"command_id":"wip-movement-00001","plant_id":"plant-A","part_id":"<UUID>","route_id":"<APPROVED_ROUTE_UUID>","scan_id":"<OBSERVATION_UUID>","action":"DEPART","station_id":"cutting-out"}}
```

Use same request body for challenge call. Issue a signed bearer JWT containing `action:sewing.transfer`, `challenge:WIP_MOVE_V1:<sha256(command_json)>`, `roles:["sewing_operator"]`, strong authentication proof, tenant, plant, user subject and unique `jti`. Use a **new** scanner event and independent worker assertion at the destination with action `ARRIVE`. The same worker cannot both hand off and receive one transfer. Interprocess variance comes from **closed** pairs only.

## Availability, migration and provisioning

- `AGIPAREL_WIP_PRODUCTION_ENABLED=true` is **off by default**, including in `compose/docker-compose.production.yml`. Requires `AGIPAREL_SEWING_PRODUCTION_ENABLED`, `AGIPAREL_GENEALOGY_PRODUCTION_ENABLED` and `AGIPAREL_CUTTING_RECONCILIATION_ENABLED` all true and `ENV=staging|production`. These are not enabled in the development Compose profile.
- Migration `0039` registers `sewing.transfer` in the global chapter policy map and adds DBA-owned routes, immutable signed gateway scans and append-only movement journal. It grants no workforce action to any user or station. Tenant RLS is forced on all three new tables, and source/transition guards run on direct SQL inserts as well.
- Provision route, enrollment, worker assignment and IdP workforce grants through a separate, audited DBA/provisioner process. Never issue universal default routes or actor permissions in migrations.
- Running this migration does **not** modify historical F3.1/F3.2 genealogy, cutting receipts or sewing completions. F3.3 writes no inventory, shipping, payroll, garment completion or QC fields.

## Remaining acceptance blockers

Physical gateway certification and site enrollment; tag-to-item binding at an inspected workstation; independently observed clock synchronization; verified shift/calendar handling; real PostgreSQL tenant and concurrent-actor integration tests; offline cache/replay conflict procedures; reversal, lost WIP and physical quarantine workflows; actual operator console wired to enterprise SSO. Since identities are cut-part-level only, a transfer is **not** proof of a completed garment or a sewing-operation completion.

## Developer release gate

Backup PostgreSQL 16, apply migration 0039 once, execute read-only `scripts/test_wip_f33_schema_pg.sql`, run full Go tests/race/vet/build and all JS tests on Go 1.24, Docker build/deploy development, then commit. Production enablement is a separate staging qualification requiring real IdP signing, reader hardware, provisioned routes, and simultaneous send/receive rejection tests.
