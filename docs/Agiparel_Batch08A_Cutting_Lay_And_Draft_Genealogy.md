# Agiparel Batch 08A — Source-Linked Cutting Lays and Classic Bundle Draft Genealogy

Build target: Batch 07D code on the MacBook. Migration **0019**, after 0018. Customer workspace: `http://localhost:18530/cutting`.

## What is implemented

- A new cutting HMI replaces the hard-coded simulated roll and camera-demo default page.
- Real buyer-PO/size/color/fabric BOM lookup from the immutable source-backed `order_intakes_v5`, `order_intake_skus_v5`, and `order_intake_bom_v5` tables.
- Save each marker/lay as an append-only `PROPOSED_HOLD` record, with source PO hash, material SKU, shade band, marker dimensions, operator note, reported plies, revision, timestamp, request key and snapshot hash.
- Define any 1–80 contiguous marker positions, with a color/size code per position. Define 1–24 unique part codes with 1–8 pieces per garment.
- Choose a maximum ply group size. Generate one draft sub-sub-bundle for each `(marker position, pattern part, ply group)`. For 45 plies, group size 20 gives ranges 1–20, 21–40 and 41–45. Piece count for each group = plies in group × pattern pieces per garment.
- Assign deterministic printable draft label codes (`CB08-` plus 24 uppercase hexadecimal digits); label is bound to tenant, buyer intake, cut number, revision, position, group and part. **These are NOT RFID EPCs, RFID bindings, sensor readings or clothing serials.**
- Check new or revised draft quantities against exact order color/size lines, all latest-revision cuts, and the order's overall quantity. Reject excess or unknown SKUs.
- Serialize all revisions for an order using a PostgreSQL transaction-scoped advisory lock; validate `expected_revision`; replay identical request keys and reject divergent reuse.
- Read existing double-witness material issue ledger *without moving any stock*. The issue counts remain evidence hints only; actual cutting cannot be released from this endpoint.
- Provide a browser print view of draft labels and the genealogy table. All DOM fields from the database use `textContent`, not HTML interpolation.
- Retire the legacy simulated `/api/v1/cutting/lay-roll` endpoint with HTTP 410. Earlier geometry simulation remains available via its separate endpoint but is not cutting evidence.

## Persisted tables

Migration `0019_cutting_lay_drafts_v8.sql` creates:

1. `cut_lay_drafts_v8`: buyer-linked immutable lay header and revision.
2. `cut_lay_snapshots_v8`: immutable full snapshot with SHA-256 integrity check.
3. `cut_lay_positions_v8`: the actual source-linked marker position/color/size rows.
4. `cut_bundle_drafts_v8`: each position × part × ply group, with label code, exact ply range, piece count and `UNBOUND_DRAFT_LABEL` state.

Each table has tenant-scoped RLS, forced RLS, `agiparel_app` SELECT/INSERT only, immutable UPDATE/DELETE triggers, and relational constraints. **No new privilege** on `store_rolls`, `material_reservations_v5`, `material_issues_v5`, `bundle_registry`, physical tag records, signed material proof tables or CEO variance exceptions.

## API contract

`GET /api/v1/cutting/lay-drafts`: latest revision per intake/cut, and 100 recent staged buyer orders.

`GET /api/v1/cutting/lay-drafts?intake_id=OI-...`: sourced color/size quantity matrix and fabric SKUs for one order.

`GET /api/v1/cutting/lay-drafts?draft_id=<uuid>`: one full immutable report including all draft labels, quantities, plies and source integrity digest.

`POST /api/v1/cutting/lay-drafts` (JSON, <=64 KiB):

```json
{
  "intake_id": "OI-<real-staged-order-id>",
  "cut_no": 1,
  "expected_revision": 0,
  "request_key": "cut_request_00000001",
  "marker_code": "MARKER-001",
  "fabric_sku": "FAB-001",
  "shade_band": "SHADE-A",
  "marker_length_mm": 6500,
  "usable_width_mm": 1450,
  "reported_plies": 45,
  "bundle_ply_group_size": 20,
  "positions": [
    {"position": 1, "color_code": "BLACK", "size_code": "32"},
    {"position": 2, "color_code": "BLACK", "size_code": "34"}
  ],
  "parts": [
    {"part_code": "FRONT", "pieces_per_garment": 2},
    {"part_code": "BACK", "pieces_per_garment": 2},
    {"part_code": "WAISTBAND", "pieces_per_garment": 1}
  ],
  "note": "Planning from reviewed tech pack; spread not released."
}
```

This example yields **90 planned garments** (2 positions × 45 plies), **18 draft sub-bundles** (2 positions × 3 parts × 3 ply groups) and **450 draft component pieces**. Actual source SKU quantities must permit the proposed cut; the example does not imply the factory has any physical stock.

## Boundaries and next steps

Batch 08A is a useful planning and labeling milestone, not physical manufacturing authority. No digital witness is simulated by the shared development login. No `bundle_registry` or physical RFID binding is written.

Batch 08B should connect verified per-roll issued fabric and RFID reader observations, spread/ply evidence, marker placement, cut consumption and exception handling to an authorized cut release. A subsequent step can promote draft labels into the actual bundle registry only with independent evidence and reconciliation, carrying the full part/position/ply-group genealogy through sewing.
