# Agiparel Factory OS — Gate 06.5-A.1: Unified Navigation & Single Document Inbox

## Product principle

Customers upload any business document **once** in **Document Intelligence** (`/documents`). The system identifies its format and preserves original evidence. Customers must not guess which functional application accepts PO, tech pack, BOM, stock, GRN, worker roster, machine list, operation bulletin, packing instruction or other documents.

## This incremental release (06.5-A.1)

1. Shared operator navigation at all seven current restricted pages: persistent Home, Back (same-origin known-page referrer or Home fallback), Document Intelligence and all factory workspaces. Uses encapsulated shadow DOM to avoid changing legacy HMI styles.
2. The public demo header and sidebar expose a plain-language link to the single document inbox.
3. Document Intelligence is explicitly the **one upload starting point**, with examples of buyer, factory, people and operational documents. Upload accepts original file formats; the extractor distinguishes supported/partial/unsupported. No invented facts or implicit approval.
4. Plant Overview and Onboarding route general document-upload actions to `/documents`. The legacy planning import remains available *only* under a clearly labeled Advanced disclosure in Onboarding because the document-to-approved-facts handoff does not yet exist.
5. Order Intake and Material Planning retain specialist `.csv` forms as explicitly advanced/temporary tools so existing functional demos remain usable. Other formats belong in `/documents`, not in the specialist CSV forms.
6. The original document library remains tenant-scoped and read-only to business tables. This UI change makes **no database migration or production writes**.

## The promised seamless experience, not yet implemented

**06.5-B:** Automatic document-purpose classification and grouping (order / style / factory master / inventory / operation bulletin / quality / packing), OCR for scanned sources, legacy `.xls`, Google Docs/Sheets connectors, typed field-level evidence with page/sheet/cell locators and confidence, multi-PO separation, review screen and conflict resolution. Unknown classification must ask a helpful question, not assert a guess as fact.

**06.5-C:** One approved document review promotes versioned PO, BOM, inventory, machine and related master facts to their canonical schemas and then powers Order Intake, Material Planning and downstream workflows without another upload. Never silently import extracted values into authoritative work orders or stores. Implement idempotent, tenant-scoped, source-linked promotion contracts. Remove the advanced legacy upload forms only when equivalent review-and-execution workflows pass the actual customer corpus tests.

## Security / customer-data note

Development Basic credentials `dev` / `1212` are intended for disposable demonstrations only. Do not use sensitive employee identity, payroll or confidential customer materials until appropriate access controls are installed. The single upload hub does not change this risk.

## Acceptance tests

- Each restricted screen has a visible Home link, Back action and central document upload link without breaking existing HMI controls.
- Direct navigation to a screen has a Back action that safely falls back to Home.
- The portal advertises a clear single document-upload action at desktop and mobile widths; no API or private documents are exposed publicly.
- Onboarding's legacy uploader is disclosed as advanced/temporary, rather than masquerading as the canonical upload route.
- Specialist CSV screens remain functional and link to original-document intake.
- One original file is never represented as an approved order or automatically linked to downstream planning until 06.5-C.
