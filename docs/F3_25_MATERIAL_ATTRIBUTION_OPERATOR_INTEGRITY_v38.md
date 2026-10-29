# F3.25 — v38 Material Attribution and Operator Integrity

Author: Sunil Jain, CyberinQ LLC, 2026-10-29.

This release combines **three narrowly defined capabilities**:

1. `dock_material_trace_v38`: bounded current-state check of all represented cut releases, their human-classified used/trim/damaged totals, spread-to-signed-material-issue millimeter allocations, roll plant scope, and over-attribution across releases.
2. `dock_integrity_report_v38`: one nested read-only snapshot of v36 dock diagnostics, v37 cut/WIP/QC lineage, and v38 material attribution. Returns `evidence_review_complete` but **never** grants physical dispatch, shipments, inventory/financial credit, legal buyer acceptance or carrier network delivery.
3. Authenticated `GET /api/v1/packing/dock/integrity-report?tenant_id=...&plant_id=...&manifest_id=...`, using `packing.read`, strict query parsing, tenant RLS, repeatable-read, a ten-second context deadline and the opt-in `AGIPAREL_DOCK_INTEGRITY_V38_ENABLED=true`. Upstream v35 external and chapter identity gates remain mandatory; default deployment is disabled.

## Trust boundaries

- A signed material issue is evidence of human custody. Human-reported cut measurements are **not** independent physical measurements.
- Scope: up to 5,000 manifested garment memberships and 500 distinct releases. Any larger or absent manifest **fails closed** with a named blocker and is not silently sampled.
- No balance is claimed for warehouse stock, all trims, supplier feeds, finance or transport. The report's top-level authority remains `READ_ONLY_OPERATOR_EVIDENCE_NOT_SHIPMENT_RELEASE`.
- No new mutable endpoint, credential, background worker, order posting or migration-side production fixture.

## Still required

Real device commissioning, end-to-end data retention, multi-factory adversarial load, buyer/carrier EDI/transport evidence, full stock/trim financial conservation and factory UAT. Snapshot consistency is NOT a concurrency stress test. These must be measured before enabling operational credit.
