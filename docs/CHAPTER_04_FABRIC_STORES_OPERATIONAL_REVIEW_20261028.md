# Chapter 04 — Fabric Stores: operational excellence and code review

Author: Sunil Jain, 2026-10-28

## Operating principle

A roll passport is a source-linked operator **observation**, not stock, an inspection sign-off, an inventory movement or an approved cutting allocation. The authoritative material ledger and independent signed identity workflow remain unchanged. We preserve the current gold Chapter 04 visual identity, shared nav, card layout and typography.

## Implemented in this increment

1. **Unambiguous physical-roll identity.** Selecting a passport locks `roll_id` for reinspection; a visible **New roll** action resets the form and revision to zero without writing any record. Concurrent revisions remain rejected by the existing optimistic-lock check. A previously linked document outside the short recent-inbox listing remains selectable, preserving provenance.
2. **Fabric shortage visibility without invented tolerances.** Show measured-versus-declared variance in metres **and percentage**, labeling a short roll or observed excess. Direct operators to inspect and resolve differences; neither a difference nor a reported PASS is an automatic stock adjustment.
3. **Honest bounded roll counts.** The list reads 501 rows and displays at most 500 with an explicit `has_more` signal. The three existing roll cards say *shown* and display `+` when truncated, rather than masquerading as factory-wide totals. The authoritative stock book remains separate.
4. **Prevent wrong-order material decisions.** Only orders with `status=PROPOSED` appear for draft picks, old reports clear on order changes, in-flight responses cannot replace the new order's report, and saved proposals are prominently labeled historical HOLD snapshots. Notes/order changes get fresh idempotency keys; unchanged retry keeps the original key.
5. **No invented zero-demand for unsupported UOM.** The server exposes an optional `demand_known=true` only for measurable metre demands. An unsupported fabric UOM is shown as *Unknown*, not `0.000 m`, and requires verified conversion. `omitempty` retains JSON hash compatibility with previously saved immutable proposals. Non-PROPOSED orders return an explicit `INTAKE_NOT_STAGED` conflict rather than a generic server error.
6. **Reconciliation request integrity.** Editing the review note resets its idempotency key; stale responses do not clear a newly selected roll/note. A failed unchanged retry preserves its request key.

## P0 architectural gaps — deliberately NOT implemented

- **Multi-plant isolation/transfer:** `store_rolls` is plant-tagged, but staged buyer orders lack a validated target plant in this pick path. Tenant-wide suggestions can draw candidates from another factory. The current draft is non-authoritative, but plant-scoped planning and signed interplant transfer should be introduced before any proposed pick can become an actual allocation. Until then, operators must validate physical plant and ownership manually.
- **Lot-level usable length/quality:** tie each roll to physical inspection, flaw map, salvageable metres, marker-compatible width and shrinkage; distinguish original, remaining, reserved, issued, damaged and returned/usable quantities with measurements, units and source times.
- **Supplier variance recovery:** add governed shortage/claim/credit-note workflow linking observed shortfall, accepted tolerance **from contract**, GRN, purchase order, supplier claim, recovery amount and closure; do not hardcode arbitrary tolerance.
- **Plant-scale roll search:** the current pick query scans at most 1001 tenant ledger rolls; expanding enterprises need plant- and material-filtered queries, pagination and explicit coverage proofs before decision-grade procurement.
- **Cross-order and multi-shade optimization:** use verified colorway, shade/lot/width constraints and reservation ledger locks; compare feasible roll allocations by remnant waste, transfer cost and order margin. Draft planning cannot bypass the signed release workflow.
- **Operational exception SLA:** aging unadmitted rolls, unreviewed discrepancies, supplier claims and unissued proposed picks need accountable owners, timestamps, escalation targets and a true action queue. Current reports are informational only.

## Safety and deployment

- No new migrations, database writes, grants, material movements, role changes or production authorizations.
- Existing CSS variables, Chapter 04 gold accent `#a87a25`, layout and shared navigation preserved.
- New contract: `GET /api/v1/stores/passports` includes `has_more`. `pick_material.demand_known` is optional and omitted if false for historical SHA-256 compatibility. Direct clients should treat absence of the flag as *legacy*, interpreting `UNIT_CONVERSION_REQUIRED` as unknown.
- Verification: JavaScript regression scripts, isolated Go pick-rule tests, UI static contracts. Run complete Go 1.24 test/vet/build plus Docker smoke on the iMac before commit.
