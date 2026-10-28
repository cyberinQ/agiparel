<!-- Author: Sunil Jain, 2026-10-28 — Operational Chapter 03 review and implemented corrections. -->
# Chapter 03 — Plan Material: Source Integrity, Surplus and Working-Capital Discipline

**Scope:** `/material-planning`, its `/api/v1/material-scenario` preview/stage/report API and regression tests. **No changes** to chapters 01, 02, 04–10, live inventory, pricing authority or downstream production release. **Visual contract:** Existing violet `#7759B2` theme, single shared chapter banner, cards, typography, layout, progressive detail disclosure and responsive behavior preserved.

## Factory-management diagnosis

Material purchase planning is the most cash-sensitive step between buyer-order capture and stores. A single repeated PO, stale spreadsheet, zero/fictional price or implicitly borrowed roll can consume thin factory margins. The current engine correctly treats operator spreadsheets as **unverified claims**, maintains an immutable `PROPOSED · HOLD` scenario and applies six conditional checks. It is not a stock ledger, GRN, reservation or authorized buying engine. Management must not read a computed *net indent* as an approved requisition, or equate claimed reuse with realized savings.

## Fixes delivered now

| Risk / defect | Operational damage | Correction | Acceptance |
|---|---|---|---|
| Preview and save independently re-upload sources without exact contract | A revised sheet could be saved against an earlier visible calculation; audit lineage diverges | Stage request supplies preview SHA-256; server recalculates the digest from intake ID, canonical saved intake source hash and **original** uploaded stock/rates bytes, rejects missing/mismatched/invalid fingerprints with HTTP **409** | Wrong or missing digest cannot stage; identical source can stage; existing immutable database storage unchanged |
| Factory-paid material has zero rate accepted as actual zero cost | Understates working-capital exposure and exaggerates apparent contribution | Reject `FACTORY` unit rate `0.0000`; require a sourced, positive rate; keep zero factory rate permitted for `BUYER_OWNED` material | Zero-price factory material blocks calculation, buyer-owned supply remains excluded from factory purchase cash |
| Latest PO silently selected when no ID supplied | Reviewer may evaluate/buy against the wrong buyer order | No implicit default. Exact PO selection is required; a valid `?intake=` handoff still preselects the requested order | Opening page without handoff shows no loaded PO; switching explicitly loads that PO |
| Network responses can land after PO/evidence selection changes | Wrong buyer's details, history or proposed costs may appear on current PO | Generation-scoped asynchronous order, preview and history rendering; invalidation on PO/file change/history load; identity guard on response; one in-flight scenario action | Late responses cannot revive stale results or enable Save |
| Financial dashboard shows "available" or "savings" as if authoritative | Claims mistaken for audited inventory or confirmed budget saving | Label claims unverified, show **factory versus buyer supply** unmet-line counts, per-line rate/source/cost; clarify conditional avoided purchase cash and buyer-owned exclusion | No false live-stock claims or inferred margin; hold gate unchanged |
| Empty BOM or changed evidence leads to unhelpful stage attempts | Wasted operations and opaque failures | Client blocks empty BOM submissions; file changes invalidate preview; stage checks fingerprint and exact selected inputs | No draft staged from empty demand or a stale preview |

**Migrations:** None. No table/index/constraint changes. No new API route. One **required** field `preview_sha256` is added to `POST /api/v1/material-scenario?action=stage` (breaking for direct scripted stage clients; update them to call preview first). Existing `preview`, `GET` and stored historical scenarios remain compatible. `PROPOSED · HOLD` and tenant-scoped immutable records remain unchanged.

## Ranked architectural gaps — NOT implemented in this change

**P0: Authoritative, lot-level stock truth.** Merge goods-receipt acceptance, 4-point inspection result, usable roll length, shade/batch/lot/dye grouping, usable width, aging, location, buyer-owned/escrow constraints, reservations, allocations and material movements. Never infer availability from an exported spreadsheet alone. A source must carry its event identity, operator and timestamp. Stage against a consistent stock snapshot with reservation concurrency detection; actual issue remains Chapter 04's authority.

**P0: Accurate demand and landed cost.** BOM per style+color+size+revision, garment allowance, marker efficiency, shrinkage, wastage and usable meter conversions must be approved by IE/CAD. Quotes need supplier, effective date, valid-through, currency, MOQ, price breaks, lead-time, freight, duty, payment terms, QC rejection likelihood and approved buyer ownership. Prevent comparing different UOMs or unexplained FX. Distinguish **booked cost**, **pending purchase cash** and **hypothetical avoided procurement**. Recompute after approved PO amendments.

**P0: Pre-indent approval and scarcity policy.** When two pending POs compete for one lot, a deterministic global allocator should optimize against promised ship dates, shade continuity, lot age, cutting efficiency, release eligibility, cash financing and shortage impact. Greedy file-row ordering in the current proposal is neither optimal nor a reservation. Cross-plant transfers require transit and QA cost before assigning claimed surplus. Any authorized procurement or override must have maker/checker segregation and an immutable reason.

**P1: Actionable shortages.** For each material: factory vs buyer ownership, confirmed usable/blocked/unknown stock, quantity and UOM, latest required-by date, supplier lead time, best valid quote, required decision, owner and overdue status. Expose ranked cash/late-delivery risks, not a speculative overall 'green' score. Without authoritative dates, keep UNKNOWN.

**P1: Commercial contribution, not gross reuse value.** At order decision time, compare incremental revenue against variable fabric/trims, labor and subcontracting, processing losses, inspection/rework, freight and financing, capacity displacement and returns/chargebacks. Do not treat conditional stock reuse as incremental *profit*: inventory has an opportunity/replacement cost.

**P1: Better bulk workflow without changing look.** A source-linked picker for approved stock snapshots and buyer-specific supplier quotes can replace repeated workbook uploads when authoritative services are available, while retaining Excel/CSV offline entry and the current violet cards for small factories.

## Acceptance / operator safety checks

1. Start Chapter 03 without a handoff: no latest-order auto-selection. Use Chapter 02 handoff with `?intake=...`: only that exact order preselects.
2. Preview uploaded sheets, then change their bytes or change order: stage is blocked by client state and/or server 409. Staging requires the matching preview fingerprint. Preview itself has no side effects.
3. Confirm price `0.0000` for factory-paid material blocks a scenario; buyer-owned material remains a buyer delivery obligation, not factory cash.
4. Confirm empty BOM, two fast PO changes, delayed preview, stale history response, and file edits cannot render wrong-PO or save-enabled state.
5. Inspect all proposed lines for rate provenance, per-unit dimensions, buyer-supplied quantities and conditional factory exposure. The UI does not claim actual stock or realized savings.
6. Verify the violet palette, mobile typography and shared navigation unchanged. No downstream purchasing, inventory reservations, GRN or factory release.

## Tests / release restrictions

`node scripts/test_material_ch03_workflow.cjs` covers the browser behaviors; `go test ./services/edge-daemon/cmd/server` covers source hash and zero-price validation, and `go test ./services/edge-daemon/internal/web` checks UI/theme contracts. Full repository `go test ./...`, `go vet ./...`, `go build ./...` and Docker deployment must run on the target Go 1.24 iMac before commit. No live deployment has been performed by this code-review sandbox. Stage API clients outside the browser must implement preview-first fingerprint passing.
