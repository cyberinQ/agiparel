# F3.16B — Individual sewing operation evidence

Author: Sunil Jain, CyberinQ LLC, 2026-10-29

## Scope

Allows a worker to attest completion of a published sewing route for **one permanent garment identity**, using a fresh, unconsumed **F3.3 signed WIP gateway scan for every required cut-part code**. The SQL checks active reader/carrier binding, independent accepted cutting outcomes (including approved recuts), last signed ARRIVE custody at the route station, unheld F3.16A state, previous per-piece/legacy operation completion, and the absence of concurrent legacy whole-group credit. A 25-ply group may therefore have 24 separate signed operation records while one garment remains under an active HOLD. This does **not** automatically release the held piece or certify a 24-piece physical output.

## Endpoints

- `POST /api/v1/sewing/pieces/completions/challenge`: submit `{ "command": { "command_id": "...", "plant_id": "...", "station_id": "...", "garment_id": "<uuid>", "route_id": "<uuid>", "scan_ids": ["<uuid>"] } }`; sign the returned bound challenge using the provisioned identity provider for action `sewing.piece.complete` and role `sewing_operator`.
- `POST /api/v1/sewing/pieces/completions`: same command plus `Authorization: Bearer <fresh bound signed assertion>` (not a session token).

Requires `ENV=staging|production`, `AGIPAREL_SEWING_PRODUCTION_ENABLED=true`, `AGIPAREL_PIECE_HOLDS_ENABLED=true`, `AGIPAREL_WIP_PRODUCTION_ENABLED=true` and `AGIPAREL_CUTTING_RECONCILIATION_ENABLED=true` (existing F3.3), `AGIPAREL_SEW_PIECE_COMPLETIONS_ENABLED=true` and existing trusted IdP variables. **All feature flags default disabled.** The provisioner must independently grant the worker `sewing.piece.complete`, assign the worker to the approved station, enroll the reader and activate a route. No automatic grant or test data is created.

## SQL invariants

- Tenant forced RLS and append-only tables, privilege-limited to the application role.
- One completion per tenant/garment/route; one command and worker proof per tenant; each scan uniquely assigned to at most one piece completion.
- SQL rejects scans that are unsigned/unaccepted in upstream writers, stale, post-dated, from wrong station/plant, unbound, already spent by transfer, recovery or hold review, or earlier than the most recent ARRIVE.
- F3.16A hold/review and F3.16B completion synchronize on garment part locks and group locks; legacy group completion cannot later duplicate a piece completion for the same operation.
- Successful response states `WORKER_ATTESTED_UNIT_OPERATION_PENDING_QUALITY_NOT_PAYROLL`. No payroll, QC release, shipment, inventory or custody credit is granted. Physical sewing quality, attendance and wage credits require later governed processes.

## Deployment

Migration `0046_individual_sewing_completions_v27.sql` follows 0045, with a pre-migration backup. Run the read-only `scripts/test_sew_f316b_schema_pg.sql`, full Go test+race/vet/build and `node scripts/test_*.cjs`; then rebuild `plant-edge-os`. Do not enable production flags without commissioning and witnessed staging acceptance.

## Known boundary

Existing Chapter 09 KPIs and historical QC group-review records use the older whole-group `sew_prod_completions_v10` ledger; they must **not** silently add these piece-level attestations to their totals. A future governed aggregation/reconciliation will combine the two without duplicate output or piece-rate credit. The F3.16A group endpoint reports reserved identities and holds, not new verified output.
