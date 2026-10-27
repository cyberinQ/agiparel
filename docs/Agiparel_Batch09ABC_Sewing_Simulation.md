# Agiparel — E09 sewing station execution, 09A–09C

**Author:** Sunil Jain / CyberInQ LLC · **Date:** 2026-10-27

## Why emulated reader clicks work

The UI button is a transport adapter for the station event protocol. It sends a server-persisted *simulated reader* message with a unique event ID, station, worker alias, cutting bundle label, lay identity, antenna port, and explicit `source=UI_SIMULATED`. `epc_hex` and `rssi_dbm` MUST be null because they were not measured. The eventual hardware integration supplies real reader observations and authenticated gateway provenance through a separately verified adapter; it cannot turn a stored simulation event into hardware evidence by rewriting a flag.

This delivers a functioning **simulation workstream**, not independently verified manufacturing. No simulated row is eligible to satisfy cutting release, payroll, QA, physical provenance, packing, or dispatch authority. The route is intentionally available only when `ENV=development`; public exposure still requires appropriate access protection.

## Traceability and features

- **09A / E09-S01:** Open real cutting lay and part-bundle drafts; configure unpublished simulated operation route with style (source lay), required part set, cardinality mode, predecessor list, machine class and SAM reference; click `Emulate RFID scan` for every required part. Scan idempotency is on the event ID and a canonical JSON payload hash.
- **09B / E09-S01 and E23-S03:** Atomic `CLAIM` consumes fresh unconsumed station-scoped scan IDs. The operation's parts and garment serial ranges are derived **by the server** from actual cutting genealogy; inputs must agree on lay/marker/ply group/color/size/ply interval. A single operation/group can be claimed/credited only once. Prerequisites must already have simulated completion in the same group. Completion generates simulated operation-unit counts and deterministic `SIM-G-<lay>-<position>-<ply>` identities; **production credit remains zero**.
- **09C / E09-S02–04:** `PAUSE`, `RESUME`, `ISSUE`, `COMPLETE` state machine. Active segments count server-observed time only; paused time excluded. Active segments above 4 hours enter review hold rather than accruing time. An issue irreversibly holds the simulated cycle; it does not generate credit. Commands use stable IDs and SHA-256 fingerprints; replay of an identical command is read-only, replay with changed data conflicts. The UI retains outstanding requests in origin-scoped browser localStorage; replay runs sequentially after connection. Browser-local buffering is *not* hardware-grade offline WAL.

### API map (all gated to development)

| Path | Method | Behavior |
|---|---|---|
| `/sewing` | GET | Operator workbench, simulated source explicitly marked |
| `/sew01` | GET | Redirect from retired browser-generated phantom tags |
| `/api/v1/sewing/sim/catalog[?lay_id=<uuid>]` | GET | Source cutting draft metadata and scoped bundle labels |
| `/api/v1/sewing/sim/operations?lay_id=<uuid>&plant_id=<code>` | GET | Configured unpublished operations |
| `/api/v1/sewing/sim/operations` | POST | Add simulation operation; immutable afterward |
| `/api/v1/sewing/sim/scan` | POST | Persist reader-shaped but non-RF event |
| `/api/v1/sewing/sim/command` | POST | CLAIM/PAUSE/RESUME/ISSUE/COMPLETE |
| `/api/v1/sewing/sim/status?lay_id=<uuid>&plant_id=<code>` | GET | Source-backed cycles and simulated unit totals |

**Scan example** (use real label + UUID from catalog):

```json
{
  "event_id":"SEW-1234567890123456",
  "lay_id":"12345678-1234-1234-1234-123456789abc",
  "plant_id":"SIM-PLANT-01",
  "label_code":"CB08-AAAAAAAAAAAAAAAAAAAAAAAA",
  "station_id":"SEW-ST04",
  "worker_id":"SIM-OP01",
  "reader":{
    "version":"SEW_READER_V1",
    "source":"UI_SIMULATED",
    "reader_id":"UI-READER-01",
    "clicked_at":"2026-10-27T11:00:00Z",
    "antenna_port":1,
    "epc_hex":null,
    "rssi_dbm":null
  }
}
```

For genuine hardware, the adapter must provide a physically observed EPC/RSSI/antenna and authenticated reader identity, a replay-protected nonce or device event ID, bounded clock uncertainty, cryptographic gateway attestation, workforce authorization, approved route/machine/skills, and cutting release. **None of that is claimed by the simulator.**

## Persistence and isolation

New migration `0022_sewing_station_simulation_v9.sql` creates:

- `sew_sim_operations_v9`: immutable unpublished simulation operation definitions.
- `sew_sim_scans_v9`: immutable click-to-scan reader payloads; SQL forbids pretending EPC or RSSI was observed.
- `sew_sim_cycles_v9`: DB-ledger state with explicit allowed transitions and timing.
- `sew_sim_cycle_inputs_v9`: append-only references from a claim to its actual simulated scan IDs.
- `sew_sim_events_v9`: append-only, replay-keyed command journal.
- `sew_sim_credits_v9`: immutable **simulated** operation units, not production credit, with deterministic serial/lineage guard.

All tables have tenant row-level security and forced RLS. Operations, scans, events, and cycles carry plant IDs; source identities are scoped by tenant, cutting lay, station and worker. A tenant-scoped PostgreSQL advisory lock serializes commands for deterministic retries; unique keys independently guard duplicates. Tables cannot mutate cutting, stores, real garment/WIP, or payroll ledgers.

## Acceptance tests

1. Start with a genuine cutting **draft** containing one or more part bundles; choose a lay and marker/ply group. Its signed release status displays separately.
2. Configure `BUNDLE_TO_BUNDLE` for one known part, and optionally `BUNDLE_TO_SINGLE` for two or more same-shade and same-ply source parts. Never invent a bundle label.
3. Click simulated scan for each required part, then claim; operation cannot claim if any part is missing, if station/worker/plant differs, or if scan is stale or already consumed.
4. Pause, wait, resume. Confirm `paused_ms` is recorded separately and not added to `active_ms`.
5. Complete. Confirm `SIMULATED_NOT_PRODUCTION_CREDIT` and count equal to number of plies, not total component pieces.
6. Replay exactly the same command ID+payload: response `replayed=true`, no extra cycle/credit. Reuse ID with a changed payload: 409 conflict. Repeat the group under a different claim: 409.
7. Switch to an operation with prerequisite: claim fails until predecessor has a simulated completion in the same source group.
8. Report an issue in a new cycle. Confirm `REVIEW_REQUIRED`; completing the held cycle fails; zero credit.
9. Disconnect temporarily before a new scan or command, verify pending queue is retained; reconnect/replay; refresh shows PostgreSQL-committed state, not optimistic local counters.
10. Confirm `/sew01` redirects to `/sewing`, and production-mode ENV disables the simulator.

**Not yet validated by physical pilot:** real operator identities and skills, real machine class, approved operation bulletin versions, authenticated RFID, real defect outcomes, approved sewing release, and physical/signed production credit. These are explicit promotion gates; no UI emulation can prove them.
