<!-- Author: Sunil Jain, 2026-10-28 — Operational review and delivered Chapter 02 change contract. -->
# Chapter 02 — Buyer Order: Operations, Controls and Economics

**Scope:** `/buyer-review` (normal source-linked buyer-order review), its document evidence mapper and handoff to `/material-planning`; `/order-intake` is the existing advanced spreadsheet alternative, audited but not restyled. **Excluded:** Chapter 01 and Chapters 03–10; inventory release, buying authority and factory-wide margin allocation. **Visual contract:** Retain existing buyer-blue accent (`#326DB3`), cards, tables, page structure, navigation, responsive layout and fonts; add only compact in-panel information.

## Operational diagnosis

An order is not a single number. A production-worthy buyer commitment requires a coherent set of **buyer + PO/amendment + style + color/size/UPC + quantity + latest approved tech pack/BOM + delivery windows + price/payment + packing/labeling + compliance**. This repository intentionally stages unapproved evidence as `PROPOSED · HOLD`. That boundary is correct and must not be bypassed. The highest immediate risks are incorrect quantities, lost reviewer corrections, and misattributed source documents. Those cause under-purchasing, missed shipments, rework or chargebacks even when software reports success.

## Fixes implemented now

| Risk/defect | Manufacturing consequence | Change/acceptance |
|---|---|---|
| Changing a source file after analysis leaves a visible review of old evidence | Saves corrected PO/BOM numbers against a different source document, corrupting lineage | Source selection clears previous preview; saved request must match analyzed document IDs; late network replies cannot revive obsolete review; analysis can restart after an in-flight switch |
| Switching between POs (often multiple in one buyer PDF) discards edited quantity, BOM, reviewer note and acknowledgement | Lost corrections lead to wrong order matrix and materials | Independent **in-memory**, per-PO review drafts; restore edits when switching back; fresh POs are not automatically acknowledged; no client-side storage of buyer evidence |
| Changing the style code quietly replaces the edited material grid | Corrected material lines lost or silently linked to wrong style | Keep material grid on style edit and carry each original material's `style_code` to the existing backend guard; mismatched materials are blocked with a specific explanation |
| Incomplete extracted PO/BOM rows disappear from automatic suggestions | Understates ordered quantities and material needs | Server reports missing PO/style/color/size/quantity and missing/invalid BOM category/rate/UOM as source-segment warnings; does not invent data or auto-import PDF estimates |
| Blind draft submission with malformed/duplicate quantities | Wasteful failed requests, confusing error feedback | Browser checks integer pieces and 100M cap, duplicate color/size, duplicate material/type/unit, positive consumption with <=3 decimals, material style mismatch; authoritative server validation remains unchanged |
| Review panel lacks a single concise quantity control | Supervisor must manually sum size lines and count manual-source corrections | Small inline summary of SKU count, total ordered pieces, material line count and manually sourced lines; clear warnings for missing consumption, units or fabric |
| Commercial commitments not distinguished from a drafted PO | Operator may confuse a save with a viable, profitable order | Single-line reminder to verify revisions, ship date, currency/price, terms, approved sample, packing, shipping; existing **PROPOSED · HOLD** persists |

**Schema / migration:** None. GET `/api/v1/buyer-review` retains identical shape; only adds warning strings to its existing `issues` array. POST contracts, Go package paths, idempotent handoff, RLS/tenant checks and document authenticity gates are unchanged. Advanced `/order-intake` remains available.

## Decisions/controls not implemented — ranked backlog

**P0 — Buyer commitment and amendment identity.** Introduce tenant-scoped buyer/customer records, canonical buyer PO ID **plus explicit revision and effective date**, supersession/withdrawal semantics, approved samples, sales contract status, signature/authorized reviewer, and buyer-supplied style/color/size/UPC identity; block promotion of superseded order lines. A PDF with multiple POs is a set of distinct commitments, not one factory order. Requires schema, migration, role control and cross-chapter tests.

**P0 — Commercial truth before accepting an order.** Record currency, sale price basis (FOB/EXW/CIF/etc.), Incoterms, freight/insurance obligations, payment terms, discount, expected penalties/chargebacks, taxes, customer-required compliance, ship windows, minimum order/pack rules, and committed delivery per PO line. Compute **incremental contribution**, not headline markup: net realized revenue minus variable fabric/trims, cutting/sewing/packing labor, subcontracting, incremental freight, defect/rework, financing and opportunity cost. If missing, show **UNKNOWN**, not a green profitability label.

**P0 — Source completeness and governed approvals.** Reconcile extracted aggregate quantities against document-declared totals (per PO/color/size), tech-pack components and versions, and buyer-specified packing/label/UPC matrices. Route conflicts to named authorized buyer merchandiser/QA/industrial-engineering owners; preserve revisions and override reasons. Existing `source_ordinal=0` is a truthful *manual* entry, **not a validated buyer fact**. Source-segment warnings require human resolution; no AI quantity fabrication.

**P0 — Feasible delivery across plants.** A confirmed buyer PO still does not prove plant capacity, material availability, QA capability or achievable lead time. Model constraints per plant: approved route SAM, qualified skills, effective shifts, bottleneck station, rework allowance, material arrival, cutting utilization, customs/freight calendars and order priority. Only an authorized scheduler may commit ship dates. This needs factory/site schema and permission work planned in Chapter 01.

**P1 — Packaging/chargeback prevention.** Link buyer-specific label/UPC, carton assortment/ratio, country-of-origin, polybag, casepack, ASN/EDI and routing requirements to QA/packing acceptance; compare packing output to approved amendment and record any deviations. Prevent accepting the wrong revision of carton or UPC instructions.

**P1 — Merchandising control tower.** With governed data, show by buyer/PO: requested vs confirmed dates, amendment count, risk of shortage/delay, unverified cost basis, pending decisions/owner/age and projected contribution. Do not present guessed margins or fictional on-time-in-full metrics.

**P1 — Advanced spreadsheet parity.** `/order-intake` validates the PO and BOM pair and stages originals, but remains a separate advanced path; eventually unify per-line evidence/approval provenance and the same warning/readiness semantics with `/buyer-review`, while keeping bulk-upload throughput and responsive UI intact. Do not merge drafts in a way that loses source identity.

## Acceptance checklist

1. Analyze PO A and BOM B, then change either document. Old review disappears; a stage attempt is rejected; a late GET result cannot restore it; re-analysis remains enabled.
2. Analyze a document with two POs. Correct quantity, BOM, note and check acknowledgement on PO 1; inspect PO 2; switch back and recover exactly the PO 1 changes.
3. Manually edit a style code; corrected BOM rows remain visible. A source material for a different style is rejected before staging and by the original Go server guard.
4. Remove/re-enter a size; duplicate color/size and invalid quantities are rejected with an explicit stage message, before a POST.
5. A source PO row missing a quantity and a BOM row missing a usage rate both produce segment-specific warnings; extracted quantities are not silently treated as complete.
6. Panel's existing blue cards/table hierarchy, Aggie/navigation, mobile layout and HOLD badge remain unchanged. No new page or color theme.
7. Draft staging stays `PROPOSED · HOLD`, with no stock reservations, buyer approval, production release, or imagined financial calculations.

## Test/deploy limits

Node behavioral regression `scripts/test_buyer_ch02_review.cjs` exercises the browser control flow with mocked DOM/network. Existing JavaScript suites and isolated Go extractor tests passed; static Go `internal/web` tests passed. Full Go 1.24 module tests/build and Docker integration require your iMac: this sandbox has Go 1.23.2 without access to the Go 1.24 distribution/dependency proxy. Do not claim live deployment or commit from this build workspace.
