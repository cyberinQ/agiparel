<!-- Author: Sunil Jain, CyberinQ LLC, 2026-10-29. -->
# F3.23 v36 — buyer-specific dock scope, exact cartons, diagnostics

Baseline `2967cd4`, schema migration `0055`.

## Authority

- `dock_buyer_spec_bindings_v36` is provisioner-only and disabled by default. Provisioner must independently confirm the buyer is contractually assigned to that exact factory packing spec before enabling the mapping. This is a **manual, externally governed contract mapping**, not automated legal-buyer acceptance.
- An authenticated BUYER lot review for another buyer/packing spec is now rejected by the SQL insert trigger; previously written evidence will no longer qualify as current until the correct contract binding is provisioned. Carrier assertions remain independently checked against the carrier receipt.
- Carton reconciliation checks every staged carton for one OPEN, one SEAL, at least one garment, current QC/needle release, an enrolled signing gateway, exact physical code, and no missing/extra scans.
- `/api/v1/packing/dock/diagnostics` requires the existing v35 workforce identity and `packing.read` grant. Reports named blockers and read-only counts. Requires **both** v35 gates and `AGIPAREL_DOCK_DIAGNOSTICS_V36_ENABLED=true`.
- Explicitly **no EDI/ASN delivery proof, legal buyer acceptance, global materials conservation, shipment posting, title transfer, inventory credit or invoicing**. A green readiness check alone cannot authorize shipping.

## Remaining acceptance gates

1. Cross-ledger material/WIP/stock/financial conservation and concurrency/error-injection acceptance.
2. Complete authorization/privilege audit for older APIs, production secrets and identity, and independent penetration testing.
3. Real scanner, needle detector, scale, timebase and production-gateway commissioning.
4. Carrier/EDI transport and buyer contractual integration; final ledger transaction and compensation semantics (still not implemented).
5. Real multi-factory load/failover, backup point-in-time recovery, operator workflow/accessibility and customer UAT.

This release implements *conservative controls*, not external physical certification or production acceptance.
