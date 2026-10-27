# Agiparel Gate 06.5-C2 — Tech-pack BOM evidence → reviewed material requirements

## Why this gate exists

Gate C1 accepts original buyer PO/BOM documents and allows an operator to stage a PROPOSED/HOLD order into existing Order Intake. It automatically recognizes simple labeled CSV/XLSX rows and narrow Walmart sales-order PDF layouts, but many real buyer tech packs are 25–35 page visual BOMs with multi-line components, alternate colorways, and *Quantity* cells that are not verified consumption per garment.

**C2 adds conservative, page-anchored component suggestions for a Centric-style `BOM: Colorway - Material Color` PDF layout**. The operator can choose each applicable item and enter the correct material SKU, unit and per-garment consumption; no quantity in a tech pack is automatically accepted as a factory planning rate.

## Verified against the supplied buyer samples

- Flat-front pant tech pack: **24** candidate component rows across **9** recognized BOM pages (fabric, trims, packaging and shipping sections).
- Pleated twill pant tech pack: **16** candidate component rows across **5** recognized BOM pages (fabric, trims and packaging sections).
- For both, **0** per-garment consumption rates are automatically treated as confirmed.
- In the supplied tech packs, body-fabric and pocketing lines print `0.000` in a *Quantity* column. **This is not evidence of zero fabric consumption.** Their consumption must come from verified marker/cutting rules, an authoritative consumption BOM, or explicit operator-reviewed input.
- Some files provide alternative materials for a component or distinguish retail versus `.COM` packaging. C2 intentionally does **not** assume all alternatives apply to every colorway or channel.

This recognition is **specific** to a known page-table layout. Other customer layouts can be uploaded and reviewed manually. PDF pages consisting of scans may still require OCR, and historical `.xls` files remain a separate conversion task.

## Contract and safeguards

1. Existing `/api/v1/buyer-review?po_document_id=...&bom_document_id=...` GET additionally returns `techpack_material_hints` and `techpack_bom_coverage`.
2. Hints retain component name, material description, tentative numeric code, material category, section, printed quantity, quantity-warning reason, page and *actual document-segment ordinal*.
3. Suggestions are presented in a dedicated Buyer Review section, with a direct link to Document Intelligence. Clicking **Review as BOM line** adds an **incomplete** material line: code is a suggestion; rate and unit deliberately remain empty. The browser focuses the rate field.
4. The existing C1 POST handler and `parseIntake()` require strictly positive per-garment quantities (precision <= 0.001) and a UOM; a missing or invalid rate cannot be staged. A source segment ordinal is validated against the original source.
5. C1 persists review mapping, original-document hashes, source IDs and PROPOSED/HOLD staging together in one transaction. C2 does not change its database contract, create buyer approval or issue inventory.
6. The document worker can split long PDF pages at 6,000 characters. C2 reassembles adjacent PAGE segments sharing the physical page locator before interpreting BOM tables, but records the segment where each row starts to preserve reviewer navigation.

## Acceptance path

1. Upload one Walmart PO PDF and corresponding tech-pack PDF at `/documents` and allow extraction to complete.
2. Open `/buyer-review`; select PO and tech pack and inspect. For the supplied PDF layout expect separate PO suggestions and a material evidence panel.
3. Inspect `FABRIC 1=BODY`, `FABRIC 2=POCKETING`, trim and packaging suggestions; confirm `0.000` fabric values are marked **Zero is not usage**, not exported as demand.
4. Click **Review as BOM line** for applicable material(s), inspect original source, and enter correct SKU, per-garment consumption, and UOM. Remove inapplicable options and alternatives.
5. Stage a reviewed order with a review note. It must appear in Order Intake and Material Planning only as PROPOSED/HOLD.
6. Re-running or re-staging the same reviewed mapping must follow the existing C1 digest/idempotency behavior. No automatic approval is implied.

## Deliberately deferred

Cross-colorway conditional BOMs, buyer carton rules, packaging variant eligibility, tech-pack visual diagrams, Google live import, layout-independent AI parsing, and old binary `.xls` conversion are still outstanding. The operational Batch 07 stores ledger and Batch 08 cutting material controls remain the next customer-facing priorities; this gate does not pretend to solve their physical authorizations.
