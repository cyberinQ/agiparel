# F3.10 — Signed WIP exceptions and supervised recovery
Author: Sunil Jain, CyberinQ LLC, 2026-10-29

## Scope and authority

F3.10 provides a genuine operator-visible hold for a part whose carrier or WIP cannot be trusted. A worker reports one of six enumerated problems (`LOST_CARRIER`, `UNREADABLE_CARRIER`, `MISSING_WIP`, `FAILED_SCAN`, `MISROUTED_PART`, `REJECTED_HANDOFF`) using a **fresh transaction-bound FIDO2/WebAuthn IdP assertion** and an independently provisioned `sewing.exception.report` grant. The worker must have a current station assignment. The report need not contain a scanner observation: missing and unreadable evidence must remain reportable.

Opening the case creates an immutable, RLS-scoped row in `wip_exception_cases_v23`. The PostgreSQL trigger prevents any further `wip_movements_v20` INSERT for that part until closure, including bypasses that do not use Go. `QUALITY_RELEASE` is also held if that garment or an effective recut part has an unresolved exception. WIP/QC and case creation use the same existing per-part and garment advisory locking protocols. Neither prior signed movements nor historical scans are changed.

A different, assigned supervisor with a separately provisioned `sewing.exception.resolve` grant and an independent transaction-bound `sewing_supervisor` assertion may close a case only by referencing a **new, enrolled, signed scanner observation both observed and received after opening**, at the reported station, within 15 minutes, matching the current carrier-to-part binding. For `LOST_CARRIER`, the replacement binding itself must postdate the incident. Used movement scans and duplicate case closures are rejected. A closure is another immutable row, not an UPDATE of the initial report. It never performs a synthetic movement or overrides approved station routes.

Read-only exception history is available to scoped, station-assigned staff with `sewing.read`, `sewing.exception.report` or `sewing.exception.resolve`, using the existing enterprise session. The `/operator-station` screen adds a report form, open-case selection, and supervisor closure. It deliberately requires a short-lived assertion pasted from a configured IdP: site credential acquisition is outside F3.10.

## Endpoints (disabled by default)

- `POST /api/v1/wip/exceptions/challenge`: `{ "command": {...} }`, returns role-specific `WIP_EXCEPTION_V1:<sha256>` challenge.
- `POST /api/v1/wip/exceptions/events`: same signed command plus `Authorization: Bearer <transaction-bound JWT>`. `OPEN` requires `command_id`, `plant_id`, `part_id`, `station_id`, `issue_type`, `note` with `case_id`/`scan_id` empty. `CLOSE` requires `command_id`, `plant_id`, `part_id`, `station_id`, `case_id`, `scan_id`, `note` with `issue_type` empty. Both return `custody_changed:false`, `production_credit:false` and `shipping_release:false`.
- `GET /api/v1/wip/exceptions/cases?tenant_id=...&plant_id=...&part_id=...&limit=25`: tenant-scoped, bounded case history (up to 50).

The `AGIPAREL_WIP_EXCEPTIONS_ENABLED` switch is `false` by default and requires staging/production identity and F3.1–F3.3 prerequisites. The operator's F3.9 page must also be enabled for the graphical workflow. A disabled application does not remove existing database WIP holds.

## Required provisioning and acceptance

1. Install F3.9 first. Apply migration `0042` once, after `0041`. Provision `workforce_identity_grants_v15` for *specific issuer/subject/tenant/plant* pairs and both F3.10 actions. Enroll working gateways and specific station workers. A supervisor cannot be the original reporter.
2. Demonstrate that an independently accepted part, with no scanner observation, can still be reported `MISSING_WIP`. Verify the database blocks `wip_movements_v20` even when bypassing Go.
3. Reject command collisions, invalid JWT challenges, missing or cross-plant parts, insufficient role grants, unassigned station workers, second unresolved reports and reused proof IDs.
4. Reject closure by the reporter, with an old scan, a scan from another station or part, retired carrier, consumed scan or `LOST_CARRIER` without post-incident rebinding. Require a new signed scan and independent supervisor.
5. After closure, attempt the *normal* F3.3 signed handoff. It must still reject invalid route/sequence/worker and never grant QC/shipping credit. Verify full append-only case history and tenant RLS under application credentials.
6. Test live PostgreSQL 16 transactions, cross-tenant isolation, concurrent report/move/closure ordering, actual gateways and IdP MFA before any production certification. The current source package cannot exercise these physical integrations.

## Deliberate exclusions

An exception closure is not a certified search, rework, customer adjustment, carrier recovery, or physical transfer. It does not reconcile negative inventory or automatically repair a misrouted part. Offline device queue/replay, controlled rerouting and calibrated physical verification remain separate work. No demo data is inserted into an operational ledger.
