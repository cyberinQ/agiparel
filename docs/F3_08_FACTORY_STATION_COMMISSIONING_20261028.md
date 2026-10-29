# F3.8 — Factory Station Commissioning Visibility

Author: Sunil Jain, CyberinQ LLC — 2026-10-28

## Scope

A production/staging-only, read-only `/commissioning` page and `/api/v1/commissioning/stations` endpoint. The service uses the F1.2 OIDC browser-session or access-token verifier, and in the same tenant-RLS-scoped read-only snapshot checks the exact provisioned `sewing.read` or `sewing.transfer` permission from F1.3. No new password system, roles, writable API, simulation authority or database migration.

### Requirements

- `ENV=staging` or `production`; `AGIPAREL_COMMISSIONING_READ_ENABLED=true` and working `AGIPAREL_WORKFORCE_OIDC_ENABLED=true` with JWKS and an independently provisioned scope permission.
- For browser access, also enable the F1.2 confidential OIDC browser flow and HTTPS cookie configuration. An access token (JWT with an authorized audience) can be used for headless GET calls; never use a client-side API key as an alternative.
- Existing PostgreSQL tables and forced tenant RLS from migrations `0029` and `0039`. No schema changes; no modification to historical evidence.

### Operational workflow

1. Open `/commissioning`, sign in with enterprise SSO and enter the actual provisioned tenant and plant IDs.
2. Click **Check station configuration**. The API returns at most 50 stations per page (maximum requested limit 100) and a cursor; page through all stations. It does not infer the factory configuration from demo records.
3. For each distinct configured station, review enabled gateway enrollment, assigned worker-station count and approved inbound/outbound transfer-route counts. The latest received signed gateway scan is reported separately as an event, not as a reader test pass.
4. Treat `CONFIGURATION_INCOMPLETE` and specific blockers as actionable provisioning issues. `CONFIGURED_NOT_PHYSICALLY_CERTIFIED` only means those database prerequisites exist. It does not certify RF field performance, scanner calibration, physical location, worker presence, correct machine/part association or end-to-end output.
5. Commission a reader only through independently approved hardware enrollment procedures, cryptographic signed observation acceptance, replay/offline recovery and a witnessed real-item physical test in isolated staging. Record calibration/witnessing outside this read-only app. F3.7 can subsequently audit a real signed lay.

### Security, accuracy and limitations

- `GET` only. No enrollment controls, grants, secrets, public-key material or worker subject identifiers are returned. Worker assignment is a count only.
- Exact tenant and plant are required; input is bounded and validated. All source queries use a read-only repeatable-read transaction under `agiparel_app`/`app.current_tenant_id` RLS and provisioned workforce permissions checked inside the same transaction.
- Pages are ordered by station ID; `next_station` only exists if there are more entries. A plant with no provisioned stations returns an empty list—not a positive readiness state.
- Signed scanner reports are not independently validated physical observations. The last scan time has no implicit freshness threshold, and the UI does not claim a reader is working now. Neither these scans nor status labels grant sewing, QC, packing, warehouse, wage or dispatch credit.
- This release has no station enrollment or physical calibration automation. It must not be described as production commissioning certification without witnessed site testing.

## Acceptance gates

Run all full Go 1.24 tests/race/vet/build and existing JavaScript tests on the operator's iMac. Run the read-only PostgreSQL schema check `scripts/test_commission_f38_schema_pg.sql` before activating staging. Test authenticated in-scope vs wrong-tenant/wrong-plant, revoked permission, disabled reader, disabled route, unassigned worker, empty factory and pagination. Production enabling remains an explicit, independently reviewed change.
