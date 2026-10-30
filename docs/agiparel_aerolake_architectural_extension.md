# Agiparel & Aerolake5: Edge-Native Provenance, Zero-Copy Mesh & Storage Governance Architecture
**Document Identifier:** AGIPAREL-AEROLAKE-ARCH-EXT-2026-REV1  
**Author:** Sunil Jain, CyberinQ LLC  
**Classification:** Production Architecture & Engineering Specification  
**Status:** Approved for Implementation  
**Target Systems:** Agiparel Core Platform (PostgreSQL 16 / Go) & Aerolake5 (`aerolake-agentd` / Rust 1.75+)

---

## 1. Executive Summary & Problem Formulation

### 1.1 Context & Motivation
Agiparel enforces deterministic, append-only provenance across the garment manufacturing lifecycle—from fabric roll intake and automated marker nesting through workstation lockstitch assembly, calibrated needle-detector challenges, AQL sampling, and scale-verified carton packing.

Every individual finished garment carries a physical identity (UHF/NFC hangtag) capable of narrating its production journey. However, in production scenarios across 10+ factory units producing tens of thousands of garments per day:
1. **Relational Table Bloat & Write Amplification:** High-frequency event logging across stations (`wip_gateway_scans_v20`, `wip_movements_v20`, `sew_piece_completions_v27`) injects millions of rows daily into PostgreSQL 16. B-tree indexes over cryptographic hashes (`event_sha256`, `signature_sha256`) trigger severe WAL bloat, high cache miss rates, and autovacuum degradation.
2. **The "Garbage Data" Trap (>99% Ephemeral Waste):** Unfiltered RFID antenna clutter, multipath reflections, and keepalive heartbeats generate massive volumes of telemetry that serve no operational or audit purpose. Hoarding this raw physical noise wastes storage, expands the attack surface, and elevates the factory's energy and carbon footprint.
3. **Statutory Traceability Mandates:** Emerging cross-border regulations—notably the **EU Digital Product Passport (DPP)** and the US **Uyghur Forced Labor Prevention Act (UFLPA)**—require verifiable, machine-readable proof linking the finished garment back to raw supplier intakes (cotton bales, yarn lots, dye chemical sheets) while strictly enforcing worker biometrics and privacy protections (GDPR / BIPA).

### 1.2 Core Architectural Thesis
1. **PostgreSQL as an Ephemeral OLTP Sequencer:** Retain PostgreSQL 16 strictly for active WIP mutexes, transaction safety, and rolling 7-to-14 day operational windows.
2. **Aerolake5 as the In-Plant L3 Compute Mesh:** Deploy `aerolake-agentd` directly on the factory LAN to consume database mutations via non-invasive Change Data Capture (CDC), store cold history in compressed Apache Iceberg v3/Parquet formats, and evaluate lineage graphs using in-memory Arrow Compressed Sparse Row (CSR) structures.
3. **Edge Ingestion Pruning:** Debounce raw RF signals at the L1/L2 hardware layer; transform transient antenna clutter into deterministic state transition proofs before permanent persistence.
4. **Decoupled Consumer Provenance Plane:** Materialize static, cryptographically signed JSON provenance summaries pushed to edge CDNs/object storage upon carton dispatch, completely isolating factory-floor transactional systems from public retail smartphone scans.

---

## 2. Garment Lineage Verification Matrix

Every sentence in the physical garment's brand narrative is bound to immutable, cryptographically verifiable database records within Agiparel:

| Garment Journey Claim | Agiparel Ledger & Table Primitives | Cryptographic & System Verifiability |
| :--- | :--- | :--- |
| **Fabric Roll & Yardage** | `store_rolls`, `store_roll_passport_events_v7`, `cut_spread_issue_allocations_v8` | Dual-witness admission proofs (`store_roll_admissions_v7`), shade-band allocations, roll EPC binding. |
| **Marker & Cut Plies** | `cut_lay_drafts_v8`, `cut_lay_positions_v8`, `cut_bundle_drafts_v8` | Marker snapshot digests (`snapshot_sha256`), calibrated ply range index bounds (`ply_start` to `ply_end`). |
| **Tech Pack & Revisions** | `documents`, `factory_reference_versions_v65b4` | Content SHA-256 validation (`file_checksum`), buyer intake approvals, non-repudiation seals. |
| **Zippers, Thread & Trims** | `store_trims_v13`, `trim_stock_admissions_v25`, `trim_issuances_v13` | Cross-ledger challenge guards (`challenge_sha256`, `manager_proof_id`, `inspector_proof_id`), supplier lot bounds. |
| **Operations & Station Routing** | `sew_piece_completions_v27`, `wip_movements_v20` | Discrete route operation codes (`operation_code`), ARRIVE/DEPART sequence ordering, station handoffs. |
| **Worker & Gateway Witnesses** | `sew_prod_worker_stations_v10`, `wip_gateway_scans_v20` | Cryptographic worker assertions (`worker_proof_id`), enrolled hardware gateway signatures (`gateway_key_sha256`). |
| **Needle Safety & Metal Detection** | `needle_safety_events_v32`, `needle_safety_clearances_v33` | Metal detector challenge audits tied to instrument calibration ledgers (`factory_instrument_calibrations_v32`). |
| **AQL Sampling & QC Release** | `buyer_aql_sample_events_v32`, `qc_garment_events_v21` | Inspector digital signatures, mm-accurate dimension arrays (`measurements_mm`), `QUALITY_RELEASE` locks. |
| **Carton Packing & Scale Tare** | `pack_carton_events_v22`, `pack_size_weights_v22` | Discrete unit `ADD` actions, carton gross/tare scale validation (`scale_reference`) verified against tolerance limits. |
| **Lineage Gate Audit** | `dock_lineage_audit_v37()` | Pre-dispatch integrity query validating cut balance, open holds, outward jobwork reconciliation, and QC clearance. |

---

## 3. High-Churn Relational Bloat Analysis

In an enterprise group running 10 factory units producing 10,000 garments/day with 20–40 discrete assembly operations, raw event volume reaches $2\times 10^6$ to $10\times 10^6$ events daily. The following Agiparel tables experience exponential growth:

```
wip_gateway_scans_v20 ─────┐
wip_movements_v20 ─────────┼──> High-Frequency Ingest ──> PostgreSQL B-Tree Index Saturation
sew_prod_observations_v10 ──┤    (event_sha256, scan_ids)    (Buffer Pool Eviction & Vacuum Locks)
sew_piece_completions_v27 ─┘
```

1. **`wip_gateway_scans_v20`:** Ingests raw antenna reads ($10^6\text{--}10^7$ rows/day). Multiple hash indexes (`event_sha256`, `signature_sha256`) saturate shared buffers and force aggressive table rewrites during autovacuum.
2. **`wip_movements_v20`:** Produces 2 records (ARRIVE, DEPART) per component part per station. Monotonically increasing sequences degrade B-tree index traversal.
3. **`sew_prod_observations_v10` & `sew_piece_used_scans_v27`:** Junction and observation ledgers joining piece UUIDs to hardware scan IDs, scaling multiplicatively ($N_{\text{garments}} \times M_{\text{parts}} \times P_{\text{operations}}$).
4. **`qc_garment_events_v21`:** Unshredded JSONB dimension arrays (`measurements_mm`) inflate table TOAST storage.

---

## 4. Target Architecture: Agiparel + Aerolake5 Mesh

To prevent database degradation and eliminate dark data while supporting sub-millisecond lineage querying, the platform moves to a three-tier hybrid architecture:

```
[ Factory Floor Edge: L1/L2 Readers & Sensors ]
                     │
                     ▼  In-Memory Windowing & Debounce (RAM)
┌────────────────────────────────────────────────────────┐
│  Tier 1: Ephemeral OLTP Kernel (PostgreSQL 16)         │
│  - Active WIP, Part Mutexes, Station Locks, RLS        │
│  - 7–14 Day Rolling Declarative Partition Retention    │
└──────────────────────────┬─────────────────────────────┘
                           │ Logical Replication (WAL / pgoutput / CDC)
                           ▼
┌────────────────────────────────────────────────────────┐
│  Tier 2: Plant L3 Data Mesh Node (aerolake-agentd)     │
│  - LocklessRingBuffer (50k batch cap) + io_uring WAL   │
│  - Embedded Apache Iceberg v3 + ZSTD Columnar Parquet  │
│  - Arrow-Native Compressed Sparse Row (CSR) Graph      │
│  - Sub-ms Lineage Traversal (OpenCypher / Arrow Flight)│
└──────────────┬──────────────────────────┬──────────────┘
               │                          │
               │ Arrow Flight / DTN (WAN) │ Stage Trigger: Build Story
               ▼                          ▼
┌──────────────────────────────┐  ┌───────────────────────────────────┐
│ Tier 3: L5 Global Enterprise │  │ Retail & Consumer Edge CDN        │
│ - Polaris Multi-Plant Catalog│  │ - Static Signed JSON Provenance   │
│ - MinIO / S3 Cold Lakehouse  │  │ - Zero Traffic on Factory Servers │
└──────────────────────────────┘  └───────────────────────────────────┘
```

### 4.1 Tier 1: Ephemeral OLTP Kernel (PostgreSQL 16)
* Maintains active transactional consistency, unique key uniqueness checks, and row-level locking (`pg_advisory_xact_lock`) while garments are on the line.
* High-churn event tables implement declarative range partitioning: `PARTITION BY RANGE (recorded_at)` in weekly increments.
* Partitions older than 14 days whose carton manifests are sealed and shipped are detached and dropped, keeping PostgreSQL lean and operating entirely within RAM cache.

### 4.2 Tier 2: Local Plant Data Mesh (Aerolake5 `aerolake-agentd`)
* Runs as a zero-GC, single native Rust binary on the plant server.
* Ingests PostgreSQL WAL stream mutations via `aerolake_cdc::PgLogParser` into a lockless ring buffer.
* Flushes data every 60 seconds into ZSTD-compressed Apache Iceberg v3 Parquet partitions in `./parquet_data/`.
* Builds an in-memory **Apache Arrow Compressed Sparse Row (CSR) graph** connecting rolls $\to$ plies $\to$ bundles $\to$ pieces $\to$ cartons. Evaluates multi-hop lineage queries (`dock_lineage_audit_v37`) in $<6\text{ ms}$ using OpenCypher pattern matching without executing recursive SQL joins on hot OLTP tables.

### 4.3 Tier 3: Global Aggregation & Disruption-Tolerant Networking (DTN)
* In network-challenged facilities (DIL regimes), `aerolake-agentd` operates autonomously offline.
* Synchronizes compacted Parquet deltas upstream to central object storage (MinIO/S3) using **Apache Arrow Flight** over encrypted QUIC, cutting WAN bandwidth consumption by up to 90% compared to JSON/REST replication.
* Reconciles cross-partition state via deterministic join-semilattice merges with delete-wins dominance.

---

## 5. Garbage Data Elimination & Carbon Footprint Reduction

Over 99% of raw factory sensor output is transient electromagnetic noise. Storing raw ticks perpetually waste megawatts of energy in unnecessary SSD manufacture, server compute, cooling, and network transit.

```
[ Raw Antennas: 40-80 reads/sec/tag ] ──> 100% Volume (Physical Chatter)
                     │
                     ▼ In-Memory Sliding Window (500 ms)
[ Debounced Edge Stream ] ──────────────> ~5% Volume (Station Boundary Events)
                     │
                     ▼ Aerolake5 State Prover (L3)
[ Cryptographic Execution Block ] ──────> <1% Volume (Verified State Transitions)
  - 1 Merkle Hash per Station Handoff
  - Discard raw RF pings after 72h
```

1. **L1/L2 In-Memory Debouncing:** Fixed readers and workstation gateways execute a 500 ms sliding window in RAM. An operator manipulating a garment in front of an antenna for 8 seconds yields a single debounced interaction record `(epc, start_time, end_time, peak_rssi, read_count)` rather than 400 separate rows.
2. **Cryptographic State Distillation:** Rather than persisting granular antenna micro-reads, Aerolake rolls an operation’s inputs (`scan_ids`, `worker_token`, `station_id`, `tolerances`) into an **Execution Block** sealed with a cryptographic hash (BLAKE3/SHA-256). The hash proves execution integrity; the raw physical noise is discarded.
3. **Tiered TTL Policy:**
   * *Ephemeral Raw Telemetry:* 24- to 72-hour TTL (ring buffer / local scratch space for antenna diagnostics), automatically reclaimed.
   * *Active WIP Ledger:* 14-day rolling window in PostgreSQL.
   * *Permanent Provenance:* Compacted, immutable Parquet partitions storing state transitions and cryptographic proofs.

---

## 6. Cross-Border & In-Country Compliance

### 6.1 EU Digital Product Passport (DPP) Mandates
* The platform models the garment's identity according to open standards (GS1 Digital Link and W3C Verifiable Credentials).
* Structured attributes (recycled fiber percentage, OEKO-TEX dye certification, repairability index) are stored as typed columnar arrays in Iceberg v3, ready for machine-readable extraction by European customs authorities and consumers.

### 6.2 Uyghur Forced Labor Prevention Act (UFLPA) Tracing
* US Customs mandates unbroken custody tracing down to the raw cotton bale.
* Agiparel maintains immutable hash linkages from the finished garment piece (`gen_cut_parts_v18`) back to raw fabric roll admission documents (`document_sources_v65`, `file_checksum`). Because raw document checksums are preserved in the cold lakehouse, full legal rebuttals can be produced without retaining millions of intermediate RF scans.

### 6.3 Worker Privacy & Labor Compliance (GDPR Article 9 / BIPA)
* Persisting direct worker identities (`worker_subject`) in permanent public audit logs violates employee monitoring regulations.
* The data pipeline tokenizes operator identities at the plant boundary using a keyed cryptographic salt:
  $$\text{WorkerProxy} = \text{HMAC-SHA256}(\text{worker\_subject}, \text{PlantSalt})$$
* Consumer-facing story endpoints render: *"Hand-stitched by Certified Artisan #88 at Station 12"*, ensuring non-repudiation and traceability while safeguarding individual worker privacy.

---

## 7. End-to-End Physical Hardware Topology

The platform deploys across three distinct physical tiers designed for harsh industrial textile environments:

```
┌────────────────────────────────────────────────────────────────────────┐
│ L1 / L2: Plant Floor & Workstation Edge                                │
│ ├─ Overhead Portals: Impinj Speedway R420/R700 or Zebra FX9600 (PoE)   │
│ ├─ Antennas: Circularly polarized 6-9 dBi panels at packing gates     │
│ ├─ Workstations: NXP i.MX8M Plus / CM4 gateways + near-field pads      │
│ ├─ Instruments: Conveyor metal detectors & RS-232 digital check scales │
│ └─ Tags: Reusable UHF WIP carriers + dual-frequency RAIN/NFC hangtags  │
└───────────────────────────────────┬────────────────────────────────────┘
                                    │ Isolated Industrial GbE PoE VLAN
                                    ▼
┌────────────────────────────────────────────────────────────────────────┐
│ L3: Local Plant Server Appliance (1 per Factory Unit)                  │
│ ├─ Form Factor: Ruggedized 1U rackmount or fanless industrial PC (IP50)│
│ ├─ Processor: 8–16 Cores x86_64 / aarch64 (AMD EPYC / Ampere Altra)    │
│ ├─ Memory: 32 GB – 64 GB DDR4/DDR5 ECC                                │
│ ├─ Storage: 2x 1TB NVMe SSDs in RAID-1 (High DWPD write endurance)    │
│ └─ Software Stack: Agiparel core-api + PostgreSQL 16 + aerolake-agentd │
└───────────────────────────────────┬────────────────────────────────────┘
                                    │ Encrypted WAN (Arrow Flight / QUIC)
                                    ▼
┌────────────────────────────────────────────────────────────────────────┐
│ L4 / L5: Regional Sovereign Hub & Global Consumer Edge                 │
│ ├─ Multi-Tenant Lakehouse: Apache Polaris Catalog + MinIO / S3 Object  │
│ ├─ Compute: Stateless cloud instances (4-8 vCPU) for cross-plant BI    │
│ └─ Consumer Edge: Cloudflare Workers / CloudFront CDN (Static JSON KV) │
└────────────────────────────────────────────────────────────────────────┘
```

---

## 8. Implementation Roadmap & Scope

Remediation requires no modifications to Agiparel core transactional business logic. Implementation spans three discrete phases over approximately **2.5 to 3.5 engineering weeks**:

1. **Phase 1: PostgreSQL Range Partitioning & TTL (3–4 Days)**
   * Partition 8 high-churn tables by `RANGE (recorded_at)`.
   * Implement automated partition dropping for closed/shipped manifests older than 14 days.
2. **Phase 2: Aerolake5 Plant CDC Integration (6–8 Days)**
   * Configure PostgreSQL `wal_level = logical`.
   * Deploy `aerolake-agentd` alongside `core-api` on the L3 plant server.
   * Wire `aerolake_cdc::PgLogParser` to stream mutations into local compacted Parquet files.
3. **Phase 3: Lineage Graph Offload & Static Provenance Export (4–6 Days)**
   * Route heavy diagnostic lineage queries (`dock_lineage_audit_v37`) to Aerolake’s in-memory Arrow CSR graph.
   * Add a carton-staging trigger that renders a signed, static provenance JSON payload (`garment_story_<epc>.json`) and pushes it to the public CDN bucket.