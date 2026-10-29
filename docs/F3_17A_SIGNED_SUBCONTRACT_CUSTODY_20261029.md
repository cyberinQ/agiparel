# F3.17A — Signed subcontract job-work cut-part custody

Author: Sunil Jain, CyberinQ LLC — 2026-10-29.

## Operational boundary

This increment records an **internal signed outward claim** for one permanent accepted cut-part identity, then a separate **internal signed return receipt** from a distinct worker at the same commissioned station. It is not a carrier bill of lading, a digitally signed vendor acknowledgement, a verified subcontractor processing certificate, an independent physical RFID-proof or a quality release. It does not award sewing, labor, inventory or shipment credit. The return remains pending internal quality review.

Processes: EMBROIDERY, PRINTING, WASHING, DYEING and OTHER_FINISHING. F3.17A concerns cut-parts, not the later garment-level subcontracting, multibox job-work manifests or damage/shortage settlement.

## Architecture

- Migration **0047** introduces `jobwork_vendors_v28` (provisioner-only), immutable `jobwork_outwards_v28` and `jobwork_inwards_v28`, and private cross-ledger scan/proof reservations.
- The sender signs `JOBWORK_OUT_V28:<SHA256 of canonical command JSON>` with the existing IdP/FIDO2 attestation (`jobwork.outward`, sewing_operator). Sender must be assigned to the outgoing sewing station.
- The receiver signs `JOBWORK_IN_V28:<SHA256 of canonical command JSON>` using a *different person* (`jobwork.inward`, sewing_supervisor). Both must have plant-scoped `workforce_identity_grants_v15` provisioned, not legacy grants.
- Each action requires a fresh enrolled-gateway scan of the same currently bound, accepted cut part at the same station. Outward requires prior recorded WIP ARRIVE custody; return requires a scan after outward. Distinct scan IDs cannot be spent twice across movement, exception review/closure, piece sewing and job-work actions.
- Vendor must have been approved independently in the vendor directory with `enabled=true` and explicit `allowed_processes`. Application role cannot provision vendors.
- A part with an open outward record cannot enter subsequent WIP movement, whole-group or per-piece sewing completion, garment QC or packing ADD. Existing one-piece holds, unresolved WIP exceptions, replaced cut parts and already QC-released garments cannot be sent out.
- All writes require **AGIPAREL_JOBWORK_CUSTODY_ENABLED=true**, staging/production environment, and the existing WIP/piece-hold/IdP feature dependencies. Disabled by default; no demonstration facts seeded.

## API endpoints

- `POST /api/v1/jobwork/parts/outward/challenge`
- `POST /api/v1/jobwork/parts/outward`
- `POST /api/v1/jobwork/parts/inward/challenge`
- `POST /api/v1/jobwork/parts/inward`
- `GET /api/v1/jobwork/parts/status?tenant_id=...&plant_id=...&part_id=<uuid>` (authorized OIDC browser/session and sewing.read grant)

The challenge request and subsequent signed POST use the **same** JSON object:

```json
{"command":{"command_id":"OUTBOUND_000001","plant_id":"PLANT01","station_id":"STATION01","part_id":"aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa","scan_id":"bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb","vendor_id":"VENDOR01","job_reference":"JOB_000001","process":"EMBROIDERY","note":"Signed outward transfer claim; not a vendor delivery receipt."}}
```

For an inward event remove `vendor_id`, `job_reference` and `process`, and specify the **previous** `outward_id` plus a new, independent `command_id` and `scan_id`. The example IDs above are *format examples only*, not authorized factory evidence or test records to submit to production.

## Commissioning and limits

1. DBA/provisioner creates approved vendor entries and distinct subject/action grants; the app cannot create them. Use only trusted vendor onboarding documents.
2. Commission the RFID gateway (F3.12) and independently verify cryptographic enrolment. The F3.13 reader parser must deliver real decoded EPCs.
3. Ensure existing WIP PART ARRIVE evidence and currently bound RFID carrier are present.
4. In staging, use **actual** two-person attestation and two **different** genuine signed observations, before and after physical travel. Observe outward HOLD and subsequent signed inward record.
5. Test concurrent outward attempts for one part, stale scans, replayed scans, disabled vendor or unsupported process, same-actor inward return, and missing grant. All must fail closed.
6. Independently inspect vendor paperwork, item damage, shortages, transport documents and post-process QC: F3.17A does **not** certify these. Add immutable discrepancy cases, return shortages, vendor acknowledgement and job-work manifests in F3.17B.

No legacy F3.1–F3.16 schema is replaced. This increment requires migration 0047 after 0046, and does not modify user-account, physical stock or production outcome records.
