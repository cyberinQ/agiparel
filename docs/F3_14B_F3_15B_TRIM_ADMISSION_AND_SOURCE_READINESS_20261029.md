# F3.14B + F3.15B — Signed Trim Stock Admission and Source-Evidence Readiness

Author: Sunil Jain, CyberinQ LLC — 2026-10-29.

## F3.14B: New trim-stock admission only

Migration `0044_signed_trim_stock_admission_v25.sql` builds on already-applied F3.14 `0043`. It does not recalculate, replenish or modify historical stock. Direct runtime `INSERT/UPDATE` of `store_trims_v13` stays revoked. An INSERT into append-only `trim_stock_admissions_v25` atomically creates ONE new stock lot with `remaining_qty = received_qty`; conflicting lot IDs, proof IDs or command IDs fail.

`POST /api/v1/material/trims/admissions/challenge`: signed `stores_manager` prepares an exact-stock, quantity (milli of UOM), bin, approved-document and source hash challenge. No stock is changed.

`POST /api/v1/material/trims/admissions`: requires an authenticated `stores_manager` caller and two independently verified signed assertions, `stores_manager` and `stores_inspector`, with action `material.trims.admit.witness` and exact challenge. Identity and granted actions `stores.trims.admit`, `stores.trims.inspect` must be provisioned under the same tenant and plant. Human-reporting witnesses must be genuinely independent. A replay with the identical command and proof IDs returns the original receipt; conflicting reuse fails.

**Source requirement:** a tenant-matching `documents` record whose status is `APPROVED`, SHA-256 matches, and `document_type` is `TRIM_RECEIPT`, `DELIVERY_CHALLAN`, `SUPPLIER_INVOICE`, or `PACKING_LIST`. Do not fabricate an approval or document in order to admit stock. The source is document review evidence, **not a calibrated scale measurement, QC pass, or independently verified supplier fulfillment**. Returnable/adjustment/ad hoc stock changes are not covered.

The DB enforces source checks, strong witness grants, immutability, tenant RLS, one-time proofs (including cross-ledger reuse from F3.14), non-editable balances, and atomic stock creation. Cryptographic Ed25519/JWT proof verification is done by the hardened Go service, not by SQL. The app DB role remains a privileged trust boundary; direct DB access must be restricted to the service.

**Production gate:** API remains off until `AGIPAREL_MATERIAL_API_ENABLED=true`, a real IdP is configured, and the independently provisioned role-action grants exist. Never rely on dev impersonation.

## F3.15B: Read-only evidence comparison

`GET /api/v1/material/readiness/source?work_order_id=...` requires authenticated strong `stores_manager`. The DB transaction is read-only, repeatable-read and tenant/plant scoped through the actual production work order. It loads the exact PO and style's source-preserved **PROPOSED** BOM quantities (or reports ambiguous/missing proposals), and reports only downstream cutting consumption tied to signed cutting releases. It distinguishes no reported consumption from a real zero.

This endpoint is **not** a BOM approval process, procurement authorization, stock allocation or cutting release. `production_release=false` always. There is no speculative marker yield, no simulated stock, no currency conversion, and no new KPI credit. Different source order quantities, duplicate candidate intakes or absent source documents are surfaced rather than guessed away.

## Commissioning

1. Apply migration `0044` once, after a consistent `pg_dump -Fc` backup; run the safe `scripts/test_trim_f314b_schema_pg.sql` check.
2. Run full Go (`test`, race, vet, build) and all JS regression scripts; rebuild the development edge image.
3. In isolated staging, provision real approved trim source document and independent manager/inspector grants. Issue a genuine challenge and both signed assertions; verify the stock appears once, and deduplication, overlong input, mismatched source, stale proof and cross-tenant cases fail.
4. Run the source-readiness endpoint for one actual released work order and check it never presents the PROPOSED BOM as buyer-approved.
5. Do not apply the earlier draft `0042_unified_identity...` migration. `0042` and `0043` are already occupied.

PostgreSQL triggers, real OIDC/FIDO2 and physical stock admission must be exercised on commissioned infrastructure before any production authority claims.
