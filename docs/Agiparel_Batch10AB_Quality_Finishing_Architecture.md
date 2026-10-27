# Agiparel E10 — Batch 10A/B Quality & Finishing Rehearsal

Author: Sunil Jain / CyberInQ · 2026-10-27

## Sources and authority

The operator workbench is based on `sew_sim_credits_v9` serials from the completed 09A-C sewing simulations. Exact cutting ancestry and color/size come from `cut_lay_positions_v8`. A caller cannot mint a garment: both backend queries and the SQL insert trigger verify its lineage. Unique `(tenant_id,serial)` represents one inspected item; each failed event holds only that row.

**Reader-shaped UI report**: `{version: QC_READER_V1, source: UI_SIMULATED, reader_id, antenna_port, clicked_at, epc_hex: null, rssi_dbm: null}`. The backend requires this shape; event history stores the complete report, reception time and immutable source classification. These clicks are **never** physical UHF read evidence or authenticated worker actions.

**Migration 0023** creates:

- `qc_sim_units_v10`: one tenant-scoped garment state, source-derived color/size, defect and independent repair identities, finishing step count, monotonically increasing version.
- `qc_sim_events_v10`: append-only, unique command/event ID and payload SHA-256. Replay is idempotent; altered replay is rejected. Indexed by garment serial.
- `qc_sim_routes_v10`: immutable, one profile per source lay/plant. Four fixed *unapproved* simulation routes, with SQL constraints matching the route catalog.
- `qc_sim_buyer_bindings_v10`: append-only buyer identity, unique across garments within tenant, only after accepted QC and all finishing steps. Exact PO style and color/size SKU must match. SQL trigger independently enforces gates.

All four tables have forced PostgreSQL RLS and app tenant-policy grants. Immutable event/bind records have defensive SQL triggers. Identity and source-lineage cannot be mutated by UPDATE. All operations use the established `docTransaction` tenant pinning and per-serial advisory transaction locks.

## API, station simulator and control plane

- `GET /api/v1/quality/sim/catalog[?lay_id=UUID&plant_id=ID]`: source-derived candidates, per-serial QA states, immutable route.
- `POST /api/v1/quality/sim/route`: pick a fixed rehearsal route for a source lay/plant. Cannot overwrite existing route.
- `POST /api/v1/quality/sim/event`: source enrollment, inspect pass/fail, repair, independent reinspection, one finishing step, or simulated buyer bind.
- `GET /quality`: restricted quality workbench with UI reader buttons and per-unit counts.

Reject operations without recognized development mode (`ENV=development`). Postgres is authoritative for *simulation state*, not for physical production. The UI uses the same endpoint on click that a future device adapter could call, but no future physical event should enter this `/sim` namespace: physical readers need a separately authenticated gateway and device-captured evidence contract.

## State machine

`PENDING → ACCEPTED` (pass) or `PENDING → HOLD` (fail with defect). `HOLD → REINSPECT` requires a repair actor different from the failing inspector. `REINSPECT → ACCEPTED|HOLD` requires a distinct inspector from both repair worker and failing inspector. `ACCEPTED` can complete *one next step* of the immutable route at a time, then bind a unique buyer identity, with exact style, color and size checked against the PO source. Already-bound garments cannot re-enter the workflow.

There is no synthetic production-credit transfer, packing eligibility, stock increment, unverified defect auto-clear, bulk pass from a scan, or silent forgiveness of duplicates.

## Acceptance and remaining gaps

Logic tests cover reader-source rejection, invented EPC/RSSI rejection, defect requirement, actor independence, invalid transitions, route catalog and content-bound event replay. Database uniqueness, RLS and triggers are in the migration. Live PG, concurrency, restart, browser/manual acceptance are still required on the iMac.

What this batch does **not** certify: approved style-specific finishing DAGs, signed QC inspectors, calibrated defect photography/metrology, device-reader authentication, real hangtag RFID swaps, real factory QA release, finished-goods stock, packing or dispatch. Those require independent authoritative workflows and hardware evaluation; E10 production verification remains open.
