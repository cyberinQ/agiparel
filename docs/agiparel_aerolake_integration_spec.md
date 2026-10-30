# Agiparel Sidecar Integration & Provenance Verification Specification

**Author:** Sunil Jain, CyberinQ LLC
**Scope:** services/core-api, services/aerolake-bridge, PostgreSQL 16 Migrations, Data Contracts
**Status:** Canonical Implementation Specification (Sprint 1 to 4)

---

## 1. System Responsibilities & Boundaries

Agiparel is the authoritative transactional system of record for garment manufacturing. It enforces state machines, RFID identities, and physical custody chains. Aerolake serves as an analytical and edge synchronization sidecar.

## 2. Integration Protocols & Data Contracts
- Logical Replication: PostgreSQL 16 agiparel_mesh_publication with replication slot aerolake_cdc_slot.
- Telemetry Ingestion: services/aerolake-bridge streams edge_ambient_rf_telemetry_v45 via arrow-go/v18 to Aerolake port 51051.
- Watermark Pruning: prune_ambient_rf_telemetry_v45 is gated by Aerolake confirmed durable Parquet checkpoint timestamps.
- Read-Only Query Offload: services/core-api routes dock_lineage_audit_v37 to Aerolake PGWire (:5433) with transparent fallback to PG16.
- Evidence-Backed Provenance: export_garment_story_provenance_v45 dynamically derives eu_dpp_compliant and uflpa_traceable from verified custody chains, signed via aerolake-dtn.

## 3. Four-Sprint Implementation Schedule
- Sprint 1 (Weeks 1-2): Pruning kill-switch, compose port fix (51051:51051, 5433:5432), migration updates.
- Sprint 2 (Weeks 3-4): arrow-go/v18 Flight producer, aerolake_cdc_slot provisioning, watermark pruning.
- Sprint 3 (Weeks 5-6): core-api PGWire read-only pool, lineage offload, real cryptographic signing.
- Sprint 4 (Weeks 7-8): dtn_sync_outbox_v45 draining loop, RLS tenant verification, 72h commissioning soak.
