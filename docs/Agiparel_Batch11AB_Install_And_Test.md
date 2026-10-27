# Agiparel Batch 11A–11B: Packing & Dispatch Rehearsal

## Scope

- PostgreSQL-backed carton opening, garment scanning, removal before seal, seal, manifest creation, sealed-carton loading, dispatch rehearsal.
- Source: Batch 10 buyer-bound, QC accepted, fully finished *simulated* garment identity only.
- All clicks emit reader-shaped `PACK_READER_V1` events with `source=UI_SIMULATED`, `epc_hex=null`, `rssi_dbm=null`, immutable event history and exact-payload idempotency.
- A garment can be in at most one carton; each carton in at most one manifest; sealed cartons cannot be edited; manifest item count is derived from actual items.
- **Not** a physical RFID observation, shipping authorization, ERP shipment, real inventory deduction, or carrier handover.

## Install on the iMac (same recipe)

1. Confirm Batch 10 (`0023`) is installed and Go tests passed. Ensure Git working tree is clean or backed up; incremental ZIP intentionally overwrites 4 existing files.
2. Download `agiparel_batch11ab_changes.zip` into `~/Downloads`.
3. Inflate:

```bash
cd ~/agiparel
unzip -o ~/Downloads/agiparel_batch11ab_changes.zip -d ~/agiparel
git diff --check
```

4. Back up the database **before** applying migration:

```bash
mkdir -p backups
docker exec agiparel-postgres pg_dump -U agiparel_admin -d agiparel_dev -Fc > "backups/pre11_$(date +%Y%m%d_%H%M%S).dump"
```

5. Apply `0024` **once**, after verifying `to_regclass('public.pack_sim_cartons_v11')` is NULL:

```bash
docker exec -i agiparel-postgres psql -X -U agiparel_admin -d agiparel_dev -v ON_ERROR_STOP=1 < services/core-api/migrations/0024_packing_dispatch_simulation_v11.sql
```

6. Validate and deploy:

```bash
go test -count=1 ./... && go vet ./... && go build ./... && git diff --check
docker compose -f compose/docker-compose.dev.yml up -d --build --no-deps --force-recreate plant-edge-os
```

7. Open `/packing` (only available with `ENV=development`), and test:

- Complete sewing simulation, QC acceptance, finishing, and buyer binding first.
- Select eligible garment, enter carton code, create carton. Add two eligible garments from same lay/plant. Duplicate scan must fail. Remove one while open, re-add, seal.
- Attempt to remove after sealing: rejected. Attempt to add held, unbound, unfinished or cross-lay garment: rejected.
- Create manifest for same plant, load sealed carton. Loading it into another manifest: rejected.
- Dispatch rehearsal succeeds with a nonempty manifest; a second dispatch must fail. UI reports rehearsal only.
- Retry an identical saved event ID/payload: returns `replayed=true`; same ID/different payload: rejected.

## Safety

No schema resets, data deletions, or authoritative stock/shipment mutations. This is an additive migration and a development-only simulator. Do not expose this workbench as a live shipping workflow. Run physical-reader validation and independently authorized dispatch before real use.
