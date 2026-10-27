# Batch 08A · MacBook installation and acceptance

**Baseline:** Batch 07D (the code in your MacBook `~/agiparel`) and successful migrations `0001–0018`. This ZIP contains only files added/changed by Batch 08A. No reset is necessary. **Target only the MacBook's local Docker environment; do not connect to the iMac.**

## 1. Install

Download `agiparel_batch08a_cutting_lay_genealogy.zip` into `~/Downloads`.

```bash
cd ~/agiparel
unzip -o ~/Downloads/agiparel_batch08a_cutting_lay_genealogy.zip -d ~/agiparel
```

## 2. Apply migration 0019

```bash
cd ~/agiparel
docker exec -i agiparel-postgres \
  psql -X -U agiparel_admin -d agiparel_dev -v ON_ERROR_STOP=1 \
  < services/core-api/migrations/0019_cutting_lay_drafts_v8.sql
```

Expect `BEGIN`, `CREATE TABLE`, `CREATE POLICY`, `CREATE TRIGGER`, and finally `COMMIT`.

**Do not run this migration twice**; it is a one-time numbered schema migration. Do not delete the local database or any Docker volumes. If migration 0018 has not been applied on the MacBook, stop and apply it first.

## 3. Verify the schema

```bash
docker exec agiparel-postgres psql -U agiparel_admin -d agiparel_dev -c \
"SELECT to_regclass('public.cut_lay_drafts_v8') AS lays,
        to_regclass('public.cut_lay_positions_v8') AS positions,
        to_regclass('public.cut_bundle_drafts_v8') AS bundles,
        to_regclass('public.cut_lay_snapshots_v8') AS snapshots;"
```

All four columns should contain table names.

## 4. Compile and test the full repository

```bash
cd ~/agiparel
go test -count=1 ./... && \
go vet ./... && \
go build ./... && \
git diff --check
```

All four commands must succeed before deployment. Source code was statically checked and its new Go rules + server glue and embedded-website tests ran in an isolated Go 1.23 harness during packaging. The full project's required Go 1.24 build and live PostgreSQL migration were not run in the packaging container and **must run on your MacBook**.

### Optional rollback-only PostgreSQL RLS/immutability test

`TestCuttingPostgresRLSAndAppendOnly` runs only when you explicitly supply `AGIPAREL_TEST_ADMIN_DATABASE_URL` and `AGIPAREL_CUT_SQL_TEST_OPT_IN=ROLLBACK_ONLY`. It starts one transaction, inserts isolated random tenant fixtures, switches to `agiparel_app`, verifies RLS, checks update/delete immutability and always rolls back. Use only a development/test administrator connection and run:

```bash
AGIPAREL_CUT_SQL_TEST_OPT_IN=ROLLBACK_ONLY \
  go test -count=1 ./services/edge-daemon/cmd/server -run '^TestCuttingPostgresRLSAndAppendOnly$' -v
```

If `AGIPAREL_TEST_ADMIN_DATABASE_URL` has not been supplied, this test skips. If the connection lacks database superuser permissions, it skips instead of changing roles. No persistent test tenant is retained.

## 5. Start ONLY the MacBook's edge service

```bash
docker compose -f compose/docker-compose.dev.yml \
  up -d --build --no-deps --force-recreate plant-edge-os
```

Open **http://localhost:18530/cutting** and authenticate using your local development operator gate. No iMac/FQDN deployment happens from this procedure.

## 6. Browser acceptance

1. Verify the new heading **Cutting & Bundle Genealogy**, and a clear **PROPOSED / HOLD** status. The old fake denim-roll demonstration should be gone.
2. Choose a real staged buyer order. Its actual fabric SKU list and exact ordered color/size quantities should appear below the selector. If no buyer orders appear, stage one using the source-backed Order Intake/Buyer Review.
3. Fill the form from that actual source. Example: 2 marker positions with matching size/color codes, parts FRONT=2, BACK=2, WAISTBAND=1, 45 reported plies, bundle group size 20. Ensure the buyer's order quantities permit 45 units for EACH of the two selected SKUs.
4. Save. Confirm 90 planned garments, 18 draft bundle labels and 450 draft pattern pieces (when that example is supported by your source). Inspect ply groups 1–20, 21–40, 41–45.
5. Save an alternate layout as revision 2 (expected revision 1); verify the old revision persists as an immutable database row. The register should display only the latest revision for that cut.
6. Attempt more units than the buyer ordered for a color/size or across cuts. Expect an HTTP 409 rejection and no new draft.
7. Confirm the screen and API explicitly say RFID is unbound, that physical cut release is blocked, and that material stock/reservation/issuance balances do not change.

## 7. Precise regression/security boundary

The legacy simulated `POST /api/v1/cutting/lay-roll` returns HTTP 410. The read-only cutting geometry utility is a simulator, not production evidence.

Zero signed stock allocations, RFID events, material issues, warehouse adjustments or CEO exceptions are created in 08A. Migrations and source changes do not touch the iMac installation.

## 8. Troubleshooting

- `relation cut_lay_drafts_v8 does not exist` → migration 0019 not applied to the MacBook's local `agiparel_dev` database.
- `FABRIC_NOT_IN_BOM` → entered fabric SKU does not match a FABRIC line of the selected staged buyer order.
- `SKU_NOT_IN_BUYER_ORDER` → color/size combination absent from the real SKU matrix; use exactly the codes shown below the buyer selector.
- `OVERPLANNED_BUYER_SKU` or `OVERPLANNED_BUYER_ORDER` → cut draft quantity exceeds source buyer PO limits after considering the latest other cuts.
- `REVISION_CONFLICT` → refresh and set `expected_revision` to the latest saved revision for that order/cut.
- `REQUEST_KEY_REUSE` → retry key was reused with different contents; the UI generates a new key on each save attempt.

Architecture and API documentation: `docs/Agiparel_Batch08A_Cutting_Lay_And_Draft_Genealogy.md`.
