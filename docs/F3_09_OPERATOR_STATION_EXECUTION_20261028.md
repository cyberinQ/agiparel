# F3.9 — Station Operator Inbox & Signed Handoff
Author: Sunil Jain, CyberinQ LLC, 2026-10-28

## Purpose and exact authority

Provide a simple shop-floor HMI for a real, enrolled RFID/barcode gateway's persisted signed scan reports and for **preparing** a transaction-bound WIP handoff using the existing F3.3 writer. Never count a scan or a prepared command as production. No sample rows or implicit station/worker permissions are created.

## Implementation

- `/operator-station`: restricted enterprise workforce interface, with unchanged Agiparel styles and no browser storage.
- `GET /api/v1/operator-station/inbox?tenant_id=...&plant_id=...&station_id=...&limit=30[&scan_id=UUID]`: snapshot of the **30 most recent** signed gateway scan reports for an explicitly assigned station (hard cap 50 server-side). Optionally resolves one scan's accepted cut part, permanent garment/bundle IDs, current tag binding, last signed custody movement, and enabled next routes. It does not provide exhaustive WIP, physically verified presence, completion or shipping counts.
- The read uses a **repeatable-read, read-only** database transaction with `SET LOCAL ROLE agiparel_app`, the exact tenant RLS context, provisioned `sewing.read` or `sewing.transfer`, and the authenticated subject's enabled worker-station assignment. A user with read-only permission sees no actionable transfer options.
- Current-source checks: cutting `ACCEPTED` outcome, no replacement of the original part, current active carrier assignment with matching original binding event, scan still within the F3.3 allowed age, scan not consumed by an earlier custody event, last handoff state, distinct dispatching/receiving worker, and approved route. Read-side options are advisory; **the existing F3.3 signed write repeats authoritative checks inside a serialized transaction**.
- The HMI obtains a canonical F3.3 `WIP_MOVE_V1` challenge for the selected scan/route, then requires an **externally issued, short-lived, strongly authenticated, transaction-bound worker bearer JWT**. It submits only to the existing `POST /api/v1/wip/production/movements` with `credentials:'omit'` (not browser cookies). The JWT is never stored in cookies, localStorage or sessionStorage by the HMI; it is cleared from the input after an attempt. There is **no new production write endpoint**.
- `AGIPAREL_OPERATOR_STATION_ENABLED` defaults to false; requires staging or production, F3.1/F3.2/F3.3 gates, provisioned OIDC, a real workforce identity, approved route and gateway, workstation worker assignment, physical scanner and an independent WebAuthn/FIDO2 assertion issuer.

## Explicit limitations

A recorded signed gateway observation is not independent physical presence proof. Without a commissioned scanner gateway and a trusted IdP capable of signing the exact F3.3 challenge, the HMI can show source-backed state but **cannot complete a handoff**. It is not an enrollment screen, identity issuance mechanism, manufacturing credit engine, or a replacement for F3.7's factory-wide audit. No inferred operation completion, payroll, QC acceptance, physical shipping or auto-dispatch.

## Acceptance gates

1. `psql -v ON_ERROR_STOP=1` on `scripts/test_operator_f39_schema_pg.sql` against PG16 with migrations 0037–0041 present; confirms forced RLS and source SELECT rights.
2. `go test -count=1 ./...`, `go test -race -count=1 ./...`, `go vet ./...`, `go build ./...`; all `scripts/test_*.cjs` tests.
3. Staging: enable feature with exact tenant and station; denied identity/other-station requests fail; read-only permission cannot prepare an actionable transfer; a revoked tag and consumed/stale scan show blockers. Cross-plant report must not reveal rows.
4. Staging: enrolled gateway submits valid Ed25519 observation for an independently accepted cut part with an active bound carrier, then two *different* strongly authenticated, assigned workers sign the DEPART/ARRIVE transitions in order. Check challenge, proof consumption, receipt timing and F3.6 indicators.
5. Concurrent duplicate command submissions and carrier retire/rebind must remain fail-closed at F3.3 SQL. Verify offline/outage recovery and actual reader commissioning separately. Never substitute an example JWT or fabricate a production receipt to pass acceptance.

## Compatibility

Built against Git baseline `cb7c938` (F3.8). F3.7's read-only acceptance harness is optional and was not observed in the user's commit log. No migration; no change to existing simulation pages, previous interfaces or database records. Production and development gates remain disabled by default.
