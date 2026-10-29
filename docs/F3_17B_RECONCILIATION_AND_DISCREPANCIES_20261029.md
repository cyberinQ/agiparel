# F3.17B — Signed subcontract discrepancy and live reconciliation

Author: Sunil Jain, CyberinQ LLC, 2026-10-29.

## Scope and evidence boundaries

This increment builds on F3.17A per-cut-part outward/inward records. It does **not** assert that a vendor has accepted a shipment, signed a manifest, certified workmanship or agreed to a loss charge. It creates no payable/vendor-debit/inventory/writeoff/production/shipment or quality-release credit. The `evidence_ref` and `evidence_sha256` supplied by a worker are **fingerprints and references, not verification** of an external document. A missing-at-reconciliation report is not proof of theft or permanent loss. Damaged return is a signed human report, not an independently measured defect.

### API

- `GET /api/v1/jobwork/reconciliation?tenant_id=T&plant_id=P&vendor_id=V&job_reference=J&process=EMBROIDERY&limit=50&after=<optional outward UUID>`: authenticated `sewing.read`, tenant/plant-scoped, read-only repeatable-read snapshot. Server joins genuine F3.17A outward/inward rows and F3.17B discrepancy rows. `outward_count`, `distinct_part_count`, `repeated_part_outward_count`, `inward_count`, `not_returned_count`, `disputed_count`, `damaged_return_count` are evidence-backed totals; `items` is a capped page (up to 100). This is a **live reconciliation grouping**, not a final issued delivery challan or signed manifest.
- `POST /api/v1/jobwork/discrepancies/challenge`: exact command challenge, requires `jobwork.discrepancy.report` and `sewing_supervisor`.
- `POST /api/v1/jobwork/discrepancies`: the same JSON command and a fresh challenge-bound signed assertion. Caller must differ from outward sender and, when applicable, inward receiver. Exact plant/part/outward and category must match already-recorded custody.

Command example (syntactic values only, **never enter mock IDs as live production**):

```json
{"command":{"command_id":"DAMAGE_RPT_00001","plant_id":"PLANT01","outward_id":"aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa","part_id":"bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb","category":"DAMAGED_ON_RETURN","note":"Observed torn seam during independent return review; set aside for QC.","evidence_ref":"VENDOR_DOC_REF_0001","evidence_sha256":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}}
```

`MISSING_AT_RECONCILIATION` requires no recorded inward receipt for that outward. `DAMAGED_ON_RETURN` requires an actual signed inward. Replays require the same command hash and the same currently authorized human identity; different claims fail. A second case for the same outward is forbidden. Cases are permanently append-only, with **no release/resolution endpoint in this increment**.

### Enforcement and commissioning

Migration **0048**, after 0047, changes two F3.17A SQL predicates from STABLE to VOLATILE to refresh concurrent custody/replay visibility after advisory locking; it then creates `jobwork_discrepancies_v29` and six before-insert progress guards. A disputed cut part cannot be moved, re-sent outward, credited by group or individual sewing, QC-passed/quality-released, or added to packing. Recording a QC HOLD or REPAIR_COMPLETE remains possible for an actually returned damaged part, subject to the preexisting QC rules, but does not clear the dispute.

The new Go action is `jobwork.discrepancy.report`, provisioned separately in `workforce_identity_grants_v15`. All report writes require the F3.17A feature flag and the existing signed IdP prerequisites. No flags, vendor accounts, worker grants or synthetic production data are created. Existing ledger records and stock balances are unchanged by the migration.

**Commissioning:** First apply the migration on a backed-up staging database. Run the read-only schema test and full Go and JS regressions. In a *disposable staging database with real signed commissioning identities and scan fixtures*, verify missing vs returned category mismatch, duplicate report, revoked grant, same reporter as sender/receiver, second outward while disputed, and WIP/QC/packing blocks. Verify plant A cannot read plant B even with guessed UUIDs. Do not fabricate evidence in production to satisfy these tests.

**Follow-on F3.17C:** separately signed vendor acknowledgement, immutable issued multi-piece manifest with exact part set and reconciliation totals, and controlled discrepancy resolution supported by independent post-return QC or approved non-return loss/waste disposition. No financial charge or inventory adjustment without a separate explicit signed transaction.
