# Agiparel Batch 07D — Surplus-first order-to-roll planning proposals

Owner: CyberinQ LLC · factory operating model: Agiparel Factory OS.

## Why now

Gate 06.5-C2 ingests original buyer evidence; Batch 05 creates a source-backed proposed order and explicit BOM; Batch 07A–C record roll passports, discrepancy evidence, secure stock admission and actual reservations. Batch 07D bridges the gap between buyer demand and secured roll stock **without issuing, reserving, assigning stock or granting material authority**. It is useful while the physical identity provider remains unconfigured.

## Operator journey

At `http://localhost:18530/stores`, section F:

1. Select a staged original-source buyer order with a **positive** fabric BOM quantity (`M`, `METRE`, or `METER`).
2. Record why a planning comparison is needed; click **Calculate & save proposal**.
3. Inspect gross demand, tentative same-shade roll coverage, remaining uncovered gap, excluded rolls and nonwaivable review blockers.
4. View immutable, idempotent snapshots in history. A new comparison gets a new request key and captures current ledger data; historical snapshots are never silently refreshed.
5. Independent shade/colorway, marker width, lot matching, source release, order ownership and stores/cutting approval remain mandatory before a real reservation. Production authorization uses the existing `material.reserve` and two-independent-witness `material.issue` secured APIs, not this workbench.

**After a MacBook database reset:** staged buyer orders and physical stock may both be empty. This is expected. Import original source orders via `/documents` and `/buyer-review` before running a proposal. Do **not** create fictitious signed inventory to make the demo appear complete.

## API

`GET /api/v1/stores/pick-proposals` lists up to 100 staged buyer orders and the latest 50 historical planning snapshots. The response authority is `PROPOSAL_ONLY`.

`POST /api/v1/stores/pick-proposals`, authenticated by the existing development/operator gate (not by a material-signing identity), accepts:

```json
{"intake_id":"ORDER-123", "request_key":"pick_20261027_001", "note":"Review colorway, shade, and marker width with cutting."}
```

It returns HTTP 201 with an immutable `PROPOSED_HOLD` snapshot, or HTTP 200 for an identical idempotent replay. Reusing a key for a different order or explanation returns 409. Invalid input returns 400. Input JSON is size-limited and unknown fields are refused.

No new endpoints mutate `store_rolls`, `material_reservations_v5`, `material_issues_v5`, `store_roll_admissions_v7`, or authority records.

## Deterministic rules

- Fabric demand: `ordered_qty * per_garment_milli` with checked integer multiplication, expressed as **millimetres**. Do not convert yards, kg, pieces, or uncertain BOM units to metres. Unknown units are explicitly blocked.
- Candidate supply comes exclusively from the existing tenant-scoped `store_rolls` ledger, not manually reported roll passports or planning suggestions.
- Ignore rolls without `PASS` in the authoritative ledger; ignore status other than AVAILABLE or RESERVED, bad widths, blank shades or overcommitted lengths.
- A legacy `reserved_order_id`, open reservation or **historical issuance** for another order excludes the roll; neither an open nor a consumed cross-order reservation is treated as free inventory.
- Existing open reservations are subtracted from remaining physical balance.
- Do not blend different shade bands. Choose the shade with the most potentially eligible length, breaking ties deterministically. Every nonchosen shade is listed as an exclusion.
- Results are always `PROPOSED_HOLD`, even if the numerical gap is zero. A match in SKU alone cannot verify buyer colorway, fabric lot, marker suitability or physical stores acceptance.
- Snapshots are capped at 1,000 ledger rolls and 250 fabric demands. Instead of silent truncation, oversized inventories fail closed. A larger factory needs a properly paginated/scoped selection architecture.

## Schema and controls

Migration `0018_stores_pick_proposals_v7.sql` creates only `store_pick_proposals_v7` with:

- tenant-scoped FK to original staged order;
- client idempotency key unique per tenant;
- source order SHA-256, canonical planning snapshot JSON and computed snapshot SHA-256;
- PostgreSQL FORCE RLS, restricted app SELECT/INSERT privileges and UPDATE/DELETE prevention trigger.

Tenant identity comes from server-pinned development configuration as in Batches 05–07; a caller cannot submit a tenant ID. No bearer identity, role or unsigned browser argument is considered material authority.

## Candid boundaries

This release **does not make reservations, issue cloth, assert that a physical RFID reader scanned a roll, or prove that a proposed buyer colorway matches a fabric shade**. Formal signed material release remains disabled until an external IdP, genuine separate people, scope and audit processes are configured. The numeric shortage displayed is a planning gap, not an automatically approved procurement indent.

## Testing

The added Go rule, input-contract, missing-DB, and embedded website tests pass in an isolated Go 1.23 harness. Static JS passes `node --check`. The full Go 1.24 suite and the live PostgreSQL migration need confirmation on the MacBook. The opt-in PostgreSQL integration test uses a single SQL transaction with final rollback and tests tenant RLS and immutable updates without changing persistent records.
