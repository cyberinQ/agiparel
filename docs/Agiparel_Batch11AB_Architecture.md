# E11: Packing, Carton Genealogy & Dispatch Rehearsal

## Event chain

`Sewing simulated cycle → QC accepted → finishing complete → buyer code bound → carton open → scan garment → seal carton → load sealed carton onto manifest → dispatch rehearsal`.

Every step is backed by tenant-scoped PostgreSQL. `pack_sim_items_v11` owns the one-garment/one-carton constraint, and `pack_sim_manifest_cartons_v11` owns one-carton/one-manifest. Carton seal and manifest dispatch are one-way transitions. Database triggers enforce upstream garment eligibility, same lay/plant, and manifest count conservation independently of the UI. All event IDs and payload hashes are durable for idempotent replay.

Reader-shaped UI payload: `version=PACK_READER_V1`, `source=UI_SIMULATED`, `reader_id`, `antenna_port`, `clicked_at`, `epc_hex=null`, `rssi_dbm=null`. No hardware claim is made. Events are immutable; carton and manifest state transitions are constrained. This stage is **rehearsal only**, and does not grant shipment authorization or physical goods movement.

## Deliberate limits

The batch does not yet implement actual carrier/transport integration, OCR-confirmed carton content, signed dispatch, invoice, regulatory export documentation, physical RFID evidence, or cross-order consolidation. Those require separate evidence and authority gates. All quantities are actual relational counts, not browser counters.
