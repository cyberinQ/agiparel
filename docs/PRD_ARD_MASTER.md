Agiparel OS: Manufacturing Execution & Planning Substrate
Master Product Requirements Document (PRD) & Architecture Requirements Document (ARD) — Rev 1.1-PROD
Product Name: Agiparel OS (Noble Garments Plant Floor System) Edge Engine: plant-edge-os (Modular Monolithic Micro-Kernel) Author: Sunil Jain, CyberinQ LLC Release Baseline: v1.0.0-PROD Reference Site: Noble Garments, OKHLA Unit 002/02 (\ge 14 Concurrent Lines) Operational Scope: Air-Gapped Intranet Edge Execution Substrate & Cloud Multi-Tenant SaaS
Executive Vision & Core Operating Axioms
Agiparel OS transforms apparel manufacturing from an approximate, retrospective accounting exercise into an unspoofable, deterministic cyber-physical execution substrate. It enforces two foundational axioms:
"Make blame impossible by making evidence automatic." "Quality and compliance are enforced by impossibility, not by post-hoc inspection."
The platform eliminates trust-based self-reporting, manual paper tallies, retrospective ERP reconciliations, and arbitrary planning multipliers. Every physical transaction across Stores, Cutting, Sewing, Finishing, Packing, and Loading is gated by deterministic physical telemetry: high-frequency NFC tokens, long-range Live UHF RFID tags, optical barcodes, rotary quadrature encoders, physical scale telemetry, and hardware motor interlocks. A step cannot be recorded as complete until its physical prerequisites have been satisfied.
1. The S-M-O Framework
Every workflow, data schema, and station interaction satisfies three architectural criteria:
+─────────────────────────────────────────────────────────────────────────────────────────────+
|                                  THE S-M-O ARCHITECTURAL TRIAD                              |
|                                                                                             |
|   1. STANDARDIZABLE (Input & Data Invariants)                                               |
|      • Canonical Operation Master (~1,200 Ops) & Machine Master (23 Types)                  |
|      • 3-Tier Item SKU Hierarchy: Item Type + Color + Size                                  |
|      • Standardized 2D Defect Coordinates: D(Linear mm, Width mm, Class, Severity Points)   |
|      • Width Tolerance Bands: Minimum Usable Width Envelope                                 |
|      • Deterministic Cut Buffers: Clamped 2% Over-Cut Formula (Min 1, Max 15 pcs)           |
|                                     │                                                       |
|                                     ▼                                                       |
|   2. MEASURABLE (Deterministic Physical Telemetry)                                          |
|      • Rotary Pulse Wheel Encoders for Running Web Length (±0.05% Precision)                |
|      • Fixed Overhead Vision for Sub-Pixel Selvage Tracking & 2D Defect Stamping            |
|      • Carrier Segmentation: UHF for Rolls/Garments, NFC for Bundles, Barcode for Cartons   |
|      • Dual-Witness Electronic Authentication (<20s SLA)                                    |
|      • Zero-Typing Operator HMIs with Minimum Cycle Dwell Clocks                            |
|                                     │                                                       |
|                                     ▼                                                       |
|   3. OPTIMIZABLE (Closed-Loop Operational Feedback)                                         |
|      • Dynamic Defect-Aware Marker Nesting Bypassing Flaws into Waste Margins               |
|      • Layered Production Planning: Learning-Curve Ramp Profiles (Days 1–8)                 |
|      • Dynamic Worker Skill & Redundancy Mesh Rebalancing Bottlenecks                       |
|      • Closed-Loop Order Conservation: Cut = Shipped + ∑Lost + ∑Left Over                   |
+─────────────────────────────────────────────────────────────────────────────────────────────+

1.1 Standardizable
Master Taxonomies: The platform provides pre-seeded, canonical master data: ~1,200 standard garment operations, 23 machine classes, standardized defect classifications, and ANSI/ASQ Z1.4 AQL sampling tables.
Material SKU Structure: Materials are identified via a 3-tier hierarchy: Item Type ID (supplier/construction code) + Color ID + Size/Dimension ID.
Defect Coordinate Normalization: Fabric defects map to a two-dimensional physical coordinate system: linear distance from the roll head (L in mm) and transverse offset from the left selvage (W in mm).
Mathematical Cut Allowances: Over-cut allowances are standardized via a deterministic clamp formula: \text{Target Cut Qty} = \text{Order Qty} + \text{Clamp}\Big(\text{round}(0.02 \times \text{Order Qty}), \, 1, \, 15\Big)
1.2 Measurable
Decoupled Web Metrology: Length is tracked via physical quadrature rotary encoders riding the moving fabric; width is tracked via calibrated, sub-pixel edge detection algorithms.
Physical Carrier Segmentation:
Long-Range Live UHF RFID: Fabric rolls in the warehouse.
Touch NFC (HF 13.56 MHz): Sub-sub bundles at operator sewing stations.
Standard UHF EPC Gen2: Individual garments during sewing and transit.
1D/2D Optical Barcodes: Trim packages, buyer hangtags, shipping cartons, and container manifests.
Cycle Pacing: Machine cycle times are recorded automatically via station NFC/RFID touchpoints.
1.3 Optimizable
Empirical Learning Loops: System-calculated standard times (Factory SAM) adjust continuously based on rolling medians from floor scans.
Adaptive Scheduling: The Cut Plan dynamically incorporates real-world roll measurements and defect maps to bypass damaged fabric zones.
Labor Balancing: Floor attendance feeds a skill graph that matches unassigned or cross-trained operators to active line bottlenecks.
2. Operating Environment & Behavioral Guardrails
+───────────────────────────────────────────────────────────────────────────────────────────+
|                               GROUND-FLOOR REALITY PROFILE                                |
|                                                                                           |
|  [Operator Profile]              [Adversarial Threat Model]       [Labor/Audit Governance]|
|  • ≤ 5th-grade comprehension     • Piece-rate "ghost tapping"     • Append-only WORM logs |
|  • Zero manual text entry        • Cross-pant shade mixing        • Tamper-evident ledger |
|  • Spoken dialect audio prompts  • Material diversion/theft       • Audit-mode anonymized |
|  • High-contrast symbolic HMI    • 1.5s inspection dwell lock     • Zero manual redaction |
+───────────────────────────────────────────────────────────────────────────────────────────+

2.1 Low-Literacy Interface Standard (\le 5th-Grade Comprehension)
Zero Floor Typing: Sewing machine operators, helpers, ironers, and checkers never type text or navigate nested menus.
High-Contrast Touch Targets (\ge 80\text{mm}):
Large Green Circle: CYCLE DONE / PASS
Large Amber Triangle: DEFECT TAG (pictorial flaw picker: Hole, Stain, Broken Stitch)
Large Red Octagon: ANDON / HELP (calls Line Mechanic or Float Operator)
Multimodal Audio Cues: Stations emit distinct hardware chimes:
Success: 800Hz confirmation tone.
Mismatch / Error: 150Hz low-frequency alert tone followed by native audio guidance in the operator's dialect (Hindi, Bengali, Tamil, Khmer).
2.2 Anti-Gaming & Floor Governance
Anti-Ghost Tapping: To prevent gaming piece rates by tapping CYCLE DONE without working, the action button remains disabled until an empirical cycle floor expires: t_{\text{min}} = \text{Factory SAM} \times 0.40 Early taps trigger an alert tone and are discarded.
1.5-Second Inspection Dwell Guard: Final Quality Checkers cannot rapidly pass garments. The PASS action remains disabled for 1.5\text{ seconds} following a hangtag scan. Early taps reset the timer.
Anti-Shade Mixing Gate: When an operator scans an incoming bundle, the edge engine validates part compatibility in \le 20\text{ ms}. If an operator attempts to marry parts from different plies, pants, or cuts, the system disallows credit, triggers an alert, and disengages the sewing machine motor via a Modbus TCP relay.
2.3 Union-Management Transparency & Audit Integrity
Append-Only Cryptographic Ledger: System transactions (scans, overrides, transfers, and sign-offs) are written to an HMAC-SHA256 hash-chained log: \text{Hash}_n = \text{HMAC-SHA256}(\text{Payload}_n \parallel \text{Hash}_{n-1}, \, K_{\text{plant}}) Neither management nor labor representatives can alter timestamps, piece rates, or downtime logs.
Audit-Safe Privacy Mode: Public line monitors show station-level throughput and aggregate metrics. Individual operator names are replaced with tokens (Worker #A814) on shared screens to comply with buyer social audits (Sedex, BSCI).
2.4 The Executive Cognitive Substrate & GM Query Engine
The platform must satisfy natural language queries from executive leadership (GM, Plant Head, CEO) 
with sub-second, deterministic precision by pairing LLM semantic comprehension with direct SQL/time-series verification.
1. Deterministic Grounding (Zero Hallucination):
   Agie never estimates or extrapolates operational ground truth. Every numerical answer regarding 
   headcount, pacing, output, scrap, or machine availability must be backed by a verifiable SQL query 
   or Merkle Ledger block hash.
2. Dynamic Temporal Windowing:
   All operational events (scans, dwell passes, defect logs, Andon alarms) are indexed into continuous 
   hourly time buckets (T-0 to T-Shift) to answer trend, acceleration, and deceleration queries.
3. Biometric & Skill Interlock:
   Line allocations link to daily attendance punches. Absenteeism immediately updates line pacing 
   models and flags bottleneck stations before morning line startup.
3. System Architecture & Bimodal Topology
Agiparel OS separates the cloud management plane from the shop-floor edge execution plane using a Bimodal Modular Architecture.
+─────────────────────────────────────────────────────────────────────────────────────────────+
|                                    CLOUD MANAGEMENT PLANE                                   |
|                        (Independently Scalable SaaS Microservices)                          |
|                                                                                             |
|  [Sourcing Svc]    [Planning Svc]    [Costing Svc]    [Analytics Svc]    [OCR Billing Svc]  |
|  • Independent DB  • Independent DB  • Independent DB • ClickHouse OLAP  • Independent DB   |
+───────────────────────────────────────────────┬─────────────────────────────────────────────+
                                                │ mTLS Event Mesh / Async Sync
                                                ▼
+─────────────────────────────────────────────────────────────────────────────────────────────+
|                                 FACTORY ON-PREM EDGE APPLIANCE                              |
|                                (Modular Monolith / Micro-Kernel)                            |
|                                                                                             |
|  +---------------------------------------------------------------------------------------+  |
|  |                            SINGLE STATIC BINARY: plant-edge-os                        |  |
|  |                                                                                       |  |
|  |   [Appetite License Enforcer] ──► Reads active feature flags from license.jwt         |  |
|  |                                                                                       |  |
|  |   +-------------------+  +-------------------+  +-------------------+  +------------+ |  |
|  |   | mod_stores        |  | mod_cutting       |  | mod_sewing        |  | mod_packing| |  |
|  |   | (Material Radar)  |  | (16-Part Nesting) |  | (In-Memory DAG)   |  | (Ratio Box)| |  |
|  |   +-------------------+  +-------------------+  +-------------------+  +------------+ |  |
|  |             │                      │                      │                   │       |  |
|  |             └──────────────────────┴──────────┬───────────┴───────────────────┘       |  |
|  |                                               ▼                                       |  |
|  |                         Zero-Copy In-Process Event Ring (Go Channels)                 |  |
|  |                                               │                                       |  |
|  |                                               ▼                                       |  |
|  |                         Shared Local Storage (Embedded SQLite WAL)                    |  |
|  +---------------------------------------------------------------------------------------+  |
+─────────────────────────────────────────────────────────────────────────────────────────────+

3.1 Cloud Management Plane (Microservices)
Upstream planning, costing, global telemetry aggregation, and multi-facility compliance run as independently deployed, containerized microservices behind an API Gateway. Each microservice manages its own transactional boundary, scaling horizontally based on compute demand:
Transactional Relational Store (PostgreSQL 16): Isolates factory tenants via PostgreSQL Row-Level Security (tenant_id = CURRENT_SETTING('app.current_tenant_id')). Manages style definitions, bills of materials, purchase orders, material allocations, and finalized order closure records.
High-Frequency Telemetry Store (ClickHouse): Ingests raw RFID scan streams without locking operational tables. Computes empirical operation times, line pacing trends, and quality metrics across the factory.
3.2 Factory Shop-Floor Plane (plant-edge-os Micro-Kernel)
On the factory floor, network-distributed microservices (HTTP/gRPC service meshes over virtual bridges) are strictly prohibited. The on-premise gateway runs as a single compiled Go/Rust static binary (plant-edge-os). Functional modules (mod_stores, mod_cutting, mod_sewing, mod_finishing, mod_packing, mod_ocr) execute as in-process packages communicating across zero-copy Go memory channels, guaranteeing sub-millisecond execution and eliminating network serialization jitter:
Hardware Profile: Runs on a dedicated on-premise fanless industrial PC (Intel N100, 16GB RAM, 512GB NVMe SSD, \le 15\text{W}) with Linux and auto-restarting systemd services.
Guaranteed Execution Latency: Evaluates in-memory assembly graphs, validates bundle pairings, and manages sensor debouncing in \le 20\text{ ms}.
Offline Operation: If external internet connectivity drops, floor operations continue without interruption. The edge node allocates bundle serials from pre-leased numeric blocks, writes events to an embedded SQLite WAL, and queues updates for upstream synchronization once network connectivity returns.
Process Isolation: The entire edge engine runs within a bounded memory footprint (\le 450\text{ MB} RAM excluding local SLM inference), fitting commodity hardware without requiring multi-node orchestration.
4. Appetite-Based Consumption & Module Packaging
Agiparel OS supports selective adoption, allowing factories to license and activate specific modules based on operational maturity without requiring architectural rewrites.
+─────────────────────────────────────────────────────────────────────────────────────────────+
|                                  MODULAR ADOPTION PROFILES                                  |
|                                                                                             |
|  [TIER A: STORE RADAR ONLY]       [TIER B: CUT & SEW SHIELD]     [TIER C: FULL SUITE]       |
|  • mod_stores                     • mod_cutting                  • All 7 Modules            |
|  • Live RFID roll tracking        • mod_sewing                   • End-to-end telemetry     |
|  • Double-witness issuance        • In-memory DAG interlocks     • Closed-loop OCR PDF      |
|  • Pre-PO surplus gate            • Anti-ghost dwell gating      • Margin reconciliation    |
+─────────────────────────────────────────────────────────────────────────────────────────────+

4.1 Standalone Module Boundaries
Module Identifier
Functional Boundary
Standalone Input Interfaces
Standalone Output Artifacts
Primary Value Driver
mod_stores (Material Radar)
Godown raw material intake, inspection metrology, and warehouse storage.
Supplier invoices, packing lists, inspection rotary encoder feeds.
Verified roll master, Live RFID length tags, double-witness issuance log.
Eliminates fabric shrinkage, unrecorded stock, and accidental deadstock purchases.
mod_cutting (Reality Bundling)
Cut planning, spreading, table execution, and bundle serialization.
Ingested CAD DXF markers, manual table lay tallies.
Sequential bundle cards (1 \dots N), 16-part component tags, lay lineage logs.
Prevents ply-count discrepancies and eliminates downstream phantom garment tags.
mod_sewing (Assembly Gate)
Machine-level workstation assembly and piece-rate validation.
Bundle RFID/NFC scans, Operation Bulletins (OB), operator logins.
Pass/fail relay interlocks, piece credits, empirical Factory SAM metrics.
Makes shade-mixing physically impossible and prevents piece-rate gaming.
mod_finishing (Inspection Guard)
Garment inspection, alteration tracking, and buyer tag association.
Internal factory RFID tags, buyer UPC barcodes.
Atomic hangtag bindings, first-pass yield logs, alteration dispatch tickets.
Guarantees 1.5s visual review and links factory genealogy to buyer barcodes.
mod_packing (Ratio Pack & Ship)
Carton packaging, AQL audit, and container dock loading.
Buyer pack manifests, box UPC scans, digital scale telemetry.
Ratio-verified carton labels, gross weight certificates, decremental load manifests.
Prevents short-shipments, mixed-ratio penalties, and unauthorized carton loading.
mod_ocr (Reconciliation Ledger)
Cross-department conservation balance and order closure.
Stage counts from any active upstream modules.
Cryptographically sealed OCR PDF, multi-department digital sign-offs.
Enforces mathematical order closure: \text{Cut} = \text{Shipped} + \sum\text{Lost} + \sum\text{Left Over}.

4.2 Dynamic License Attestation (license.jwt)
Cryptographic Feature Gating: Modules are enabled or disabled via an ed25519-signed JWT license file placed in /etc/agiparel/license.jwt.
Runtime Verification: Upon boot, plant-edge-os inspects the active license payload:
{
  "tenant_id": "NOBLE_OKHLA",
  "plant_id": "UNIT_002",
  "licensed_modules": ["mod_cutting", "mod_sewing", "mod_ocr"],
  "max_active_lines": 14,
  "valid_until": "2027-12-31T23:59:59Z"
}


Dynamic UI Mounting: The edge web server dynamically mounts only licensed API endpoints. The frontend PWA reads this configuration on hydration, rendering only licensed workspace routes while pruning inactive modules from the UI bundle.
5. End-to-End System Invariants
Invariant 1: Pre-PO Surplus Matching Gate
Before an indent can be issued to procurement, the system calculates the net purchase requirement against unreserved inventory:
\text{Net Indent Qty} = \text{BOM Required Qty} - \text{Available Unreserved Qty}
The system blocks the creation of purchase orders for gross quantities unless an explicit override reason code is recorded.
Invariant 2: Sacrosanct Reservations & Zero-Borrowing
Once a fabric roll is allocated to an Order \times Cut in the Cut Plan, its state locks to RESERVED. The system prohibits borrowing materials across orders. Supplier shortfalls route to the Top Management Council (CEO, Sourcing, Production, IE, Finance) for an official replan.
Invariant 3: Collaborative Non-Repudiable Planning Lock
Order commitments require independent baseline sign-offs from Merchandising, IE, Sourcing, and Production. A plan freezes only when all four leads submit cryptographic PIN confirmations. Mid-order modifications generate formal amendments that require unanimous digital re-acceptance before schedule updates take effect.
Invariant 4: Post-Cut 1 Reality Numbering
Sequential bundle numbering and RFID tag bindings generate strictly after Cut 1 finishes spreading and cutting. Numbering plans reflect actual physical plies counted rather than CAD projections, preventing phantom garment tags from reaching the sewing floor.
Invariant 5: Directed Sub Bundle Attachment DAG
Garment assembly pairings are verified at scan time using an in-memory directed graph:
\text{Scan Status} = \begin{cases} \mathbf{ACCEPT}, & \text{if } (B_{\text{source}}, B_{\text{target}}) \in \mathcal{E} \land \text{Pant}(B_{\text{source}}) = \text{Pant}(B_{\text{target}}) \\ \mathbf{REJECT}, & \text{otherwise} \end{cases}
If an operator scans panels from different pants or cuts, the edge engine rejects the pairing, cuts machine motor power via Modbus TCP relays, and withholds piece credits.
Invariant 6: Hangtag Helper Atomic Swap
At sewing exit, the internal factory RFID tag is removed and recycled. An atomic database transaction binds the internal RFID tracking record to the buyer's serialized Hangtag barcode:
\text{HangtagBind} = \langle \text{Internal RFID ID}, \, \text{Buyer Hangtag Barcode}, \, \text{SKU Details}, \, \text{Helper ID}, \, t_{\text{timestamp}} \rangle
This preserves complete piece-level history through downstream packing and logistics.
Invariant 7: Closed-Loop Order Conservation
Material flows must balance across every stage of the manufacturing pipeline:
\text{Inflow}_{\text{stage}} = \text{Forward} + \text{Lost} + \text{Left Over}
The complete order must balance before the Order Closure Report (OCR) PDF can be generated:
\mathbf{Cut} = \mathbf{Shipped} + \sum \mathbf{Lost} + \sum \mathbf{Left Over}
Any non-zero variance requires explanatory comments from the owning supervisor and digital sign-offs from all six department heads.
6. Functional Modules Specification
[Part 1: Costing] ─────────► [Part 2: Planning] ─────────► [Part 3: Sourcing & Stores]
  Empirical SAM Quote           8-Stream T&A Synch            Mandatory Surplus Pre-PO Gate
  Capacity-Aware CMT            Ramp Learning Curve           Material Radar & Roll Lay Log
       │                             │                                  │
       ▼                             ▼                                  ▼
[Part 4: Cutting] ─────────► [Part 5 & 6: Sewing/Finishing] ──► [Part 7: OCR Closure]
  210 Lay & 16-Part DAG         Hangtag Helper Atomic Bind      Inflow = Forward+Lost+Left
  End-Bit Tier Routing          Multi-Slot Ratio Packing        Immutable Reconciled Ledger

6.1 Costing (Part 1)
Empirical Factory SAM: Quotes use real operational time studies derived from workstation telemetry rather than theoretical GSD benchmarks. SAM values carry confidence scores: RFID·H (\ge 500 runs), IE·M (100–500 runs), and Provisional / Analogy (<100 runs).
Capacity-Aware CMT Engine: Calculates Cost-Make-Trim from operation-level SAM and Cost Per Minute (CPM) rates. Identifies machine and skill shortages during initial quoting.
Progressive Margin Tracking: Monitors margins across four baselines: Quoted \rightarrow Booked \rightarrow Projected Realized \rightarrow Final Realized (OCR).
6.2 Planning (Part 2)
8 Synchronized T&A Streams: Balances Fabric, Trims, Cutting, Sewing (Bottleneck), Finishing, Packing Trims, Machine Plan, and Skill Plan. Schedule slips automatically update downstream dependencies.
Sampling-to-Production Stage Gates: Enforces strict sequential sign-offs before bulk production release: \text{Proto 1} \rightarrow \text{Proto 2} \rightarrow \text{Ref Sample} \rightarrow \text{Pre-Production (PP)} \rightarrow \text{Size Set} \rightarrow \text{TOP Sample} \rightarrow \text{Bulk Cut}
Learning-Curve Production Ramp (DPP Layer 1): Replaces flat daily targets with an empirical ramp profile across the 8-day production window:
Day 1: Line setup and tooling verification (0 pcs).
Day 2: Initial pilot output (30% capacity).
Day 3: Acceleration (60% capacity).
Days 4–7: Steady-state run (100% capacity).
Day 8: Balance runout and line clean-down.
Automated Pacing Alerts: Calculates line pace deviations: \text{Variance}_{\text{hour}} = \frac{\text{DPP Adjusted Target} - \text{Actual Good Scans}}{\text{DPP Adjusted Target}}
\text{Variance} \in [11\%, 15\%]: Triggers Level 1 Alert on Line Supervisor tablet (SEW-03).
\text{Variance} > 15\%: Triggers Level 2 Issue on Line Quality and Mechanic consoles.
\text{Variance} > 20\% (or 2 consecutive hours behind): Triggers Level 3 Escalation on Plant Manager desktop (PM-01/PM-02).
6.3 Sourcing & Stores (Part 3)
Material Radar: Warehouse personnel query inventory remotely via handheld UHF readers. A directional audio tone indicates physical proximity to targeted rolls.
Dimensional Tolerance Normalization: Inspects fabric rolls at three physical positions: Head, Middle, and Tail. The minimum usable width becomes the binding geometric boundary for CAD marker matching: W_{\text{usable}} = \min(W_{\text{head}}, W_{\text{mid}}, W_{\text{tail}}) - \delta_{\text{selvage}}
Inspection Rig Metrology: Tracks fabric length using a physical rotary encoder and logs 2D defect coordinates via an overhead camera mounted on a rigid gantry. Measured values commit directly to the roll's Live RFID tag.
Double-Witness Digital Issuance: Material checkouts require dual authentication (Issuer ID + Recipient ID) completed in <20\text{ seconds}, decrementing tag memory dynamically.
6.4 Cutting Floor (Part 4)
Physical Spreading & Cut Constraints:
Maximum Lay Limit: Spread length capped at 210 lays per table.
Bundle Size Bounds: 20 \pm 2 pieces per sub-sub bundle.
Part Complexity: Enforces a 16-part component graph per trouser unit.
Dynamic Defect Bypass & Optimization: Ingests CAD markers (universal DXF, Optitex, Lectra, Gerber). Projects the 2D defect map across the planned spread: defects falling into inter-panel gaps are flagged as benign, avoiding unnecessary cut stops.
Roll-to-Lay Lineage (cut_lay_rolls): Explicitly maps each roll to its spread lay range (e.g., Roll #7 \rightarrow Lays 29 to 59). Any ply defect logged downstream traces back to the source fabric roll.
End-Bit Disposition Thresholds:
L_{\text{end}} < 1.5\text{m}: Written off as cutting scrap.
1.5\text{m} \le L_{\text{end}} \le 5.0\text{m}: Retained at the cutting table for panel recuts.
L_{\text{end}} > 5.0\text{m}: Returned to Stores inventory with updated Live RFID tags for future orders.
Pre-Sewing Balance Gate: If fabric consumption deviates by more than \pm 2\%, bundles cannot release to sewing lines without digital CEO authorization.
6.5 Sewing Floor Execution (Part 5)
Operation Bulletin (OB) Data Contract: Standardizes operation parameters across distinct operational columns:
Unit Mode: Bundle (batch credit on stack completion) vs. Single (piece-rate credit per unit read).
Tracking Mode: Scanned (hardware-verified) vs. Unscanned (manual steps inheriting credit from the next scanned operation).
Operator Multiplicity: Single (1:1 station mapping) vs. Multi-Operator (shared stations balancing tact times).
Zero-Typing Station HMI: Operators interact with high-contrast touch targets (CYCLE DONE, DEFECT TAG, ANDON) paired with animated 3D assembly instructions.
In-Line Checker HMI: Checkers verify garments and log results via dedicated [PASS] and [REJECT] tabs, routing rework pieces directly to alteration lines.
6.6 Finishing, Packing & Pre-Ship Logistics (Part 6)
Execution Principle: Scan = Identity, Tap = Decision.
Inspection & Alteration Routing: Checkers inspect garments under the 1.5-second dwell guardrail. Rejections route by failure type: REDO_HANGTAG (Hangtag Helper), REDO_IRON (Ironing Station), or REDO_SEWING (Alteration Operator). All repaired garments must receive a final PASS tap before entering packing queues.
Multi-Slot Ratio Packing Engine (PK-02): Supports complex mixed cartons:
Cartons 1–50: Ratio pack (e.g., 2 \times \text{UPC}_1 + 2 \times \text{UPC}_2 + 2 \times \text{UPC}_3 = 6\text{ pcs/box}; Target Gross Weight: 6.2 kg \pm 250\text{g}).
Cartons 51–150: Solid pack (12 \times \text{UPC}_4; Target Gross Weight: 7.8 kg \pm 250\text{g}).
Enforcement: The tablet interface renders decrementing slot targets, locking the carton seal and barcode printer until all slots are filled and scale weight matches specification.
Dock Container Loader: Dock loaders execute decremental scans against shipping manifests, rejecting unverified cartons.
6.7 Order Closure Report (Part 7)
Closed-Loop Reconciliation: Reconciles piece counts across all six departments. Once verified against the conservation equation, the system generates an immutable, numbered PDF (e.g., OCR-MK6574J9-001). Post-closure adjustments must be issued as numbered addenda.
7. Screen Taxonomy
The user interface layer contains 31 production-floor screens and ~70 system-wide target views:
+─────────────────────────────────────────────────────────────────────────────────────────────+
|                                    OPERATIONAL SCREEN TIERS                                 |
|                                                                                             |
|  [Tier 1: Desktop Situation Rooms]  ──► Management, IE Masters, Planning Gantt, OCR         |
|  [Tier 2: 10" Mobile Tablets]       ──► Line Supervisors, Quality Leads, Dock Loaders      |
|  [Tier 3: Handheld / Fixed HMIs]    ──► Machine Operators, Checkers, Hangtag Helpers        |
+─────────────────────────────────────────────────────────────────────────────────────────────+

Module / Tier
Screen ID
Dashboard / Screen Name
Primary Persona
Form Factor
Functional Scope & Key Invariants
Plant Management
PM-01
Plant Overview
Plant Manager / CEO
Desktop
Live 14-line grid, output achieved vs target, plant DHU, active issues.


PM-02
Live Issues Feed
Plant Manager / Director
Desktop
Plant-wide issue ticker with "who is on it" ownership stamps.


PM-03
Line Drill-In
Plant Manager / IE Head
Desktop / Tablet
Single-line digital twin: station throughput, hourly pacing, active calls.


PM-04
Workforce & Attendance
Plant Manager / HR Lead
Desktop
Line attendance, skill coverage gaps, float reallocations.


PM-05
Day Performance & Plan
Plant Manager
Desktop
End-of-day output projections, next-day planning, pending approvals.
Line Quality
QS-01
Line Quality Dashboard
Line Quality Supervisor
Tablet (Portrait)
Single-line live DHU, pass/defect totals, checker status.


QS-02
Live Defect Feed
Line Quality Supervisor
Tablet (Portrait)
Real-time stream of defects logged by checkers with photo attachments.


QS-03
Defect Heatmap
Line Quality Supervisor
Tablet (Portrait)
Operator \times Operation matrix identifying where to route technical help.


QS-04
Repair Queue
Line Quality Supervisor
Tablet (Portrait)
Alteration tracking with aging countdowns and 30-min SLA alerts.


QS-05
Audits & Style Setup
Line Quality Supervisor
Tablet (Portrait)
Shift audit calendar (inline/end-of-line), AQL parameters, risk callouts.
Multi-Line Quality
QM-01
Plant Quality Overview
Multi-line Quality Manager
Desktop
Plant DHU, lines ranked best-to-worst, 4-hour sparklines, escalations.


QM-02
Defect Pareto
Multi-line Quality Manager
Desktop
Plant-wide top 10 defect Pareto, worst operation/style drilldowns.


QM-03
Style-wise Quality
Multi-line Quality Manager
Desktop
Active style DHU, risk tiers, pre-production risk mitigation.


QM-04
Quality Trends
Multi-line Quality Manager
Desktop
30-day plant DHU trends, shift analysis, improving/slipping lines.


QM-05
Audit Calendar & Compliance
Multi-line Quality Manager
Desktop
Plant compliance matrix, missed audits, buyer customer audit calendar.
Sewing Floor
SEW-01
Machine Operator HMI
Sewing Operator
Phone / 10" Tablet
Zero-typing: NFC login, RFID queue, DWI, Cycle Done / Defect / Andon.


SEW-02
In-Line Checker HMI
In-Line Checker
Phone / Scanner
Fast defect tagging, pass/repair routing, bundle checkpoint logging.


SEW-03
Line Supervisor Console
Line Supervisor
10" Tablet (Mobile)
Hourly pacing (bars 1–8), bottleneck detection, float-operator dispatch.


SEW-04
Mechanic Terminal
Line Mechanic
Phone
Breakdown queue, accept/resolve calls, preventive maintenance jobs.


SEW-05
Chief Mechanic Console
Chief Mechanic
Phone / Tablet
Machine availability, mechanic allocations, spare parts requests.
Finishing
FH-00
Finishing Flow Map
Finishing Supervisor / IE
Desktop
Visual pieces/hour throughput balance across finishing lines.


FH-01
Hangtag Helper Station
Hangtag Helper
Phone / Tablet
Atomic swap: scan RFID, scan Buyer Hangtag, validate SKU, recycle tag.


FH-02
Ironer Station
Ironing Operator
Phone
Large "+1 PRESSED" tap target, hourly pacing bars, iron-redo feedback.


FH-03
Final Checker HMI
Final Checker
Phone / Tablet
"Scan = ID, Tap = Decision" with 1.5s dwell guard; PASS or REDO.


FH-04
Alteration Operator HMI
Alteration Operator
Phone
Sewing defect queue, original context, DONE tap, scan-back validation.


FH-05
Finishing Supervisor Console
Finishing Supervisor
Tablet / Desktop
Live station tracking, Final Pass Rate (\ge 98\%), alteration queue depth.
Packing & Shipping
PK-00
Pre-Ship Pipeline Overview
Logistics Lead / Merchandiser
Desktop
Visual carton staging, packing rate, container fill statuses.


PK-01
Pack Plan Setup
Merchandiser
Desktop
Buyer carton ranges, solid/ratio pack setup, AQL settings, CSV upload.


PK-02
Carton Packing Station
Packer
Tablet + Scanner
Scan carton UPC, multi-slot ratio pack verification, weight match.


PK-03
Final Audit Inspector
Warehouse AQL Auditor
Tablet + Scanner
Random carton sample validation, defect scoring, pass/fail verdict.


PK-04
Container Dock Loader
Loading Operator
Tablet + Scanner
Decremental loading countdown; blocks unauthorized cartons.


PK-05
Pack & Ship Supervisor
Logistics Supervisor
Desktop
Active packers, audit queue, repack line, truck ETDs, manifest seals.
Order Closure (OCR)
OC-01
Reconciliation Working Desk
Merchandiser / Factory Head
Desktop
7-stage balance sheet; checks \text{Cut} = \text{Shipped} + \sum\text{Lost} + \sum\text{Left Over}.


OC-02
Order Closure Report Document
Merchandiser / Buyer / Auditor
Desktop / Printable PDF
Immutable, 6-department signed reconciliation certificate.
Engineering & Costing
OB-01
OB Authoring (Costing Stage)
Industrial Engineer (IE)
Desktop
Operation sequence builder, Machine Master assignment, Factory SAM auto-fill.


OB-02
OB Execution Allocation
Factory Head / IE
Desktop
Frozen operator assignment, workstation/machine IDs, reason-coded shifts.


CST-01
Enquiry & Costing Studio
Merchandiser / Costing Exec
Desktop
BOM pricing, dynamic CMT from empirical SAM, margin protection gates.


CST-02
Order Book & Margin Pipeline
Sales Manager / CEO
Desktop
Quoted vs. Booked vs. Projected Realized margin tracking.
Planning & Scheduling
PLN-01
8-Stream Master T&A Console
Merchandiser Head / IE Head
Desktop
Multi-order Gantt chart synchronizing Fabric, Trims, Cut, Sew, Finish, Pack, Machine, Skill.


PLN-02
Dynamic Line Loading & Capacity
Master Scheduler / IE Head
Desktop
Machine/line allocations, capacity calendar, bottleneck resolution.


PLN-03
3-Layer DPP Workspace
Factory Head
Desktop / Tablet
Ramp curve setup: IE Baseline \rightarrow Adjusted (with reason codes) \rightarrow Actual.


PLN-04
Skill Demand & Forward Pipeline
HR Head / IE Head
Desktop
Skill matrix, forward gap solver (Train/Hire/Reassign), mobility heatmap.


PLN-05
Critical Order Watchlist & Replan
COO / CEO
Desktop
Top Management Council replan workspace for material shortfalls.
Sourcing & Stores
STR-01
Indent Pipeline & Pre-PO Gate
Sourcing Head / Merchandiser
Desktop
Surplus check: \text{Net Indent} = \text{BOM Required} - \text{Unreserved Stock}.


STR-02
Gate Intake & Fabric Inspection
Receiving / Quality Inspector
Tablet + Scanner
GRN logging, 4-point defect mapping, usable width capture, Live RFID tagging.


STR-03
Live Inventory & Roll Ping Map
Store Manager (Fabric)
Desktop / Tablet
Zoned bin hierarchy, Available vs. Reserved state, handheld Geiger ping.


STR-04
Double-Witness Issuance Station
Store Issuer
Tablet (Mobile)
Scan roll RFID, dual digital authentication (Issuer + Recipient PIN) in <20\text{ s}.


STR-05
Trims & Packing Stores Console
Store Manager (Trims)
Desktop
Barcode SKU balances, issuance queues, MOQ tracking, dispatch links.


STR-06
Surplus & Deadstock Aging Studio
Sourcing Head / CFO
Desktop
Aging ledger for end-bits and over-runs (Re-allocate, Sell, Write-off, Extras).
Cutting Floor
CUT-01
Cut Plan Optimization & Tweak
Cutting Master / Production Head
Desktop
Roll-to-cut allocation, CAD marker ingestion, fabric utilization solver.


CUT-02
Table Spreading & Cut Execution
Spreader / Cutting Operator
Station Tablet
Actual plies captured, 210-lay bounds, CAM telemetry, roll-lay tracking.


CUT-03
Post-Cut Reality Numbering
Cutting Incharge
Desktop / Tablet
Post-Cut 1 sequential ID generation (1 \dots N) based on actual plies.


CUT-04
Cutting Helper Bundling Station
Cutting Dept Helper
Station Tablet + RFID Reader
Cursor-driven single-scan workflow mapping 16-part bundles (20 \pm 2).


CUT-05
Pre-Sewing Fabric Balance Gate
Cutting Incharge / Factory Head
Desktop
Net variance check; blocks bundle release if variance > \pm 2\% without CEO sign-off.


CUT-06
Cutting KPI & Multi-Table Console
Cutting Master / IE Head
Desktop
4–12 table queue status, fabric utilization, end-bit %, tweak rate.
Executive Governance
EXEC-01
CEO Consumption Variance Desk
Chief Executive Officer
Desktop / Mobile Web
Mandatory CEO authorization with narrative "actual story" for variances.


EXEC-02
Top Management Council Cockpit
CEO, COO, Sourcing, IE Heads
Desktop
Critical material shortfall replan workspace enforcing zero-borrowing.

8. Concrete Technical Architecture & Contracts
8.1 Relational Data Models (schema.sql)
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- TENANT ENTITY SPECIFICATION
CREATE TABLE tenants (
    id VARCHAR(64) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);

-- FABRIC ROLL LEDGER WITH EMBEDDED DEFECT MAPS
CREATE TABLE store_rolls (
    id VARCHAR(64) PRIMARY KEY,
    tenant_id VARCHAR(64) NOT NULL REFERENCES tenants(id),
    factory_sku VARCHAR(128) NOT NULL,
    original_length_m NUMERIC(10,2) NOT NULL,
    remaining_length_m NUMERIC(10,2) NOT NULL,
    usable_width_cm NUMERIC(6,2) NOT NULL,
    shade_band VARCHAR(32) NOT NULL,
    defect_map_json JSONB DEFAULT '[]'::jsonb,
    bin_location VARCHAR(64) NOT NULL,
    status VARCHAR(32) NOT NULL CHECK (status IN ('AVAILABLE', 'RESERVED', 'ISSUED', 'DEPLETED')),
    reserved_order_id VARCHAR(64),
    reserved_cut_no INT,
    last_rfid_sync TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);
ALTER TABLE store_rolls ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation_store_rolls ON store_rolls 
    FOR ALL USING (tenant_id = CURRENT_SETTING('app.current_tenant_id'));

-- DOUBLE-WITNESS STORE ISSUANCE TRANSACTIONS
CREATE TABLE store_issuances (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id VARCHAR(64) NOT NULL REFERENCES tenants(id),
    roll_id VARCHAR(64) NOT NULL REFERENCES store_rolls(id),
    order_id VARCHAR(64) NOT NULL,
    cut_no INT NOT NULL,
    issued_length_m NUMERIC(10,2) NOT NULL,
    issuer_digital_id VARCHAR(64) NOT NULL,
    recipient_digital_id VARCHAR(64) NOT NULL,
    issued_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    digital_signature_hash VARCHAR(256) NOT NULL
);
ALTER TABLE store_issuances ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation_store_issuances ON store_issuances 
    FOR ALL USING (tenant_id = CURRENT_SETTING('app.current_tenant_id'));

-- ROLL-TO-LAY CONSUMPTION LINEAGE
CREATE TABLE cut_lay_rolls (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id VARCHAR(64) NOT NULL REFERENCES tenants(id),
    order_id VARCHAR(64) NOT NULL,
    cut_no INT NOT NULL,
    roll_id VARCHAR(64) NOT NULL REFERENCES store_rolls(id),
    lay_start INT NOT NULL,
    lay_end INT NOT NULL,
    plies_count INT NOT NULL,
    end_bit_length_m NUMERIC(10,2) NOT NULL,
    end_bit_action VARCHAR(32) NOT NULL CHECK (end_bit_action IN ('SCRAP', 'RECUT_HOLD', 'STORE_RETURN')),
    recorded_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);
ALTER TABLE cut_lay_rolls ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation_cut_lay_rolls ON cut_lay_rolls 
    FOR ALL USING (tenant_id = CURRENT_SETTING('app.current_tenant_id'));

-- POST-CUT SUB-SUB BUNDLE REGISTRY
CREATE TABLE bundle_registry (
    id VARCHAR(64) PRIMARY KEY, -- Form: PO-Cut-Sequential
    tenant_id VARCHAR(64) NOT NULL REFERENCES tenants(id),
    order_id VARCHAR(64) NOT NULL,
    cut_no INT NOT NULL,
    sequential_bundle_no INT NOT NULL, -- Sequential Integer 1..N across PO
    pant_position INT NOT NULL,        -- Garment position in marker
    sub_bundle_id INT NOT NULL,        -- 16-part component ID
    ssb_ply_group INT NOT NULL,        -- Physical ply group (size 20 ± 2)
    part_description VARCHAR(128) NOT NULL,
    ply_start INT NOT NULL,
    ply_end INT NOT NULL,
    piece_count INT NOT NULL,
    garment_serial_start INT NOT NULL,
    garment_serial_end INT NOT NULL,
    active_nfc_uid VARCHAR(128) UNIQUE,
    mapped_by_helper_id VARCHAR(64),
    mapped_at TIMESTAMPTZ,
    status VARCHAR(32) NOT NULL CHECK (status IN ('PENDING_MAP', 'IN_SEWING', 'COMPLETED', 'SCRAPPED'))
);
ALTER TABLE bundle_registry ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation_bundle_registry ON bundle_registry 
    FOR ALL USING (tenant_id = CURRENT_SETTING('app.current_tenant_id'));

-- CARTON PACK RATIO MANIFEST
CREATE TABLE carton_pack_manifests (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id VARCHAR(64) NOT NULL REFERENCES tenants(id),
    order_id VARCHAR(64) NOT NULL,
    box_range_start INT NOT NULL,
    box_range_end INT NOT NULL,
    pack_type VARCHAR(32) NOT NULL CHECK (pack_type IN ('SOLID', 'RATIO')),
    slot_rules_json JSONB NOT NULL, -- Array of {upc, target_qty}
    expected_gross_weight_kg NUMERIC(6,3) NOT NULL,
    weight_tolerance_kg NUMERIC(4,3) NOT NULL DEFAULT 0.250,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);
ALTER TABLE carton_pack_manifests ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation_carton_pack_manifests ON carton_pack_manifests 
    FOR ALL USING (tenant_id = CURRENT_SETTING('app.current_tenant_id'));

-- ATOMIC HANGTAG BINDING TRANSACTION
CREATE TABLE hangtag_binds (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id VARCHAR(64) NOT NULL REFERENCES tenants(id),
    bundle_id VARCHAR(64) NOT NULL REFERENCES bundle_registry(id),
    nfc_uid VARCHAR(128) NOT NULL,
    buyer_hangtag_barcode VARCHAR(128) NOT NULL,
    order_id VARCHAR(64) NOT NULL,
    style_id VARCHAR(64) NOT NULL,
    helper_id VARCHAR(64) NOT NULL,
    bound_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_tenant_hangtag UNIQUE (tenant_id, buyer_hangtag_barcode)
);
ALTER TABLE hangtag_binds ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation_hangtag_binds ON hangtag_binds 
    FOR ALL USING (tenant_id = CURRENT_SETTING('app.current_tenant_id'));

-- SHARED FIRST-CLASS FLOOR ISSUE OBJECT
CREATE TABLE floor_issues (
    id VARCHAR(64) PRIMARY KEY,
    tenant_id VARCHAR(64) NOT NULL REFERENCES tenants(id),
    issue_type VARCHAR(32) NOT NULL CHECK (issue_type IN ('machine', 'quality', 'output', 'workforce', 'maintenance')),
    severity VARCHAR(32) NOT NULL CHECK (severity IN ('critical', 'attention', 'info', 'resolved')),
    plant_id VARCHAR(64) NOT NULL,
    line_id VARCHAR(64) NOT NULL,
    station_id VARCHAR(64),
    machine_id VARCHAR(64),
    operator_id VARCHAR(64),
    title VARCHAR(255) NOT NULL,
    started_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    resolved_at TIMESTAMPTZ,
    owner_id VARCHAR(64) NOT NULL,
    watchers JSONB DEFAULT '[]'::jsonb,
    status VARCHAR(32) NOT NULL CHECK (status IN ('open', 'in_progress', 'resolved'))
);
ALTER TABLE floor_issues ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation_floor_issues ON floor_issues 
    FOR ALL USING (tenant_id = CURRENT_SETTING('app.current_tenant_id'));

-- ORDER CLOSURE REPORT (OCR) RECONCILIATION LEDGER
CREATE TABLE ocr_records (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id VARCHAR(64) NOT NULL REFERENCES tenants(id),
    order_id VARCHAR(64) NOT NULL,
    stage VARCHAR(32) NOT NULL CHECK (stage IN ('CUT', 'SEWN', 'FINISHED', 'PACKED', 'LOADED')),
    forward_qty INT NOT NULL,
    lost_qty INT NOT NULL,
    left_over_qty INT NOT NULL,
    supervisor_id VARCHAR(64) NOT NULL,
    variance_comment TEXT,
    signed_at TIMESTAMPTZ,
    CONSTRAINT chk_positive_quantities CHECK (forward_qty >= 0 AND lost_qty >= 0 AND left_over_qty >= 0)
);
ALTER TABLE ocr_records ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation_ocr_records ON ocr_records 
    FOR ALL USING (tenant_id = CURRENT_SETTING('app.current_tenant_id'));

-- ============================================================================
-- DOMAIN 8: WORKFORCE, ATTENDANCE & SKILL MATRIX
-- ============================================================================

CREATE TABLE IF NOT EXISTS workers (
    id VARCHAR(64) PRIMARY KEY,
    tenant_id VARCHAR(64) NOT NULL REFERENCES tenants(id),
    name VARCHAR(255) NOT NULL,
    employee_code VARCHAR(64) NOT NULL,
    primary_skill VARCHAR(64) NOT NULL, -- e.g., 'KANSAI_HEMMING', 'COLLAR_ATTACH'
    skill_grade VARCHAR(8) NOT NULL CHECK (skill_grade IN ('A', 'B', 'C', 'TRAINEE')),
    hourly_rate_inr NUMERIC(10,2) NOT NULL DEFAULT 0.00,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);
ALTER TABLE workers ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation_workers ON workers FOR ALL USING (tenant_id = CURRENT_SETTING('app.current_tenant_id'));

CREATE TABLE IF NOT EXISTS attendance_register (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id VARCHAR(64) NOT NULL REFERENCES tenants(id),
    worker_id VARCHAR(64) NOT NULL REFERENCES workers(id),
    shift_date DATE NOT NULL,
    clock_in_time TIMESTAMPTZ,
    clock_out_time TIMESTAMPTZ,
    status VARCHAR(32) NOT NULL CHECK (status IN ('PRESENT', 'ABSENT', 'LATE', 'ON_LEAVE')),
    assigned_line_id VARCHAR(64),
    assigned_station_id VARCHAR(64)
);
ALTER TABLE attendance_register ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation_attendance ON attendance_register FOR ALL USING (tenant_id = CURRENT_SETTING('app.current_tenant_id'));

-- ============================================================================
-- DOMAIN 9: MACHINE ROSTER & HEALTH
-- ============================================================================

CREATE TABLE IF NOT EXISTS machine_roster (
    id VARCHAR(64) PRIMARY KEY,
    tenant_id VARCHAR(64) NOT NULL REFERENCES tenants(id),
    line_id VARCHAR(64) NOT NULL,
    station_id VARCHAR(64),
    name VARCHAR(128) NOT NULL,
    canonical_class VARCHAR(64) NOT NULL, -- SNLS, 4-OL, KANSAI_FLATLOCK, etc.
    rated_capacity_uph INT NOT NULL CHECK (rated_capacity_uph > 0),
    status VARCHAR(32) NOT NULL CHECK (status IN ('ACTIVE', 'IDLE', 'BREAKDOWN', 'MAINTENANCE')),
    active_operator_id VARCHAR(64) REFERENCES workers(id)
);
ALTER TABLE machine_roster ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation_machine_roster ON machine_roster FOR ALL USING (tenant_id = CURRENT_SETTING('app.current_tenant_id'));

-- ============================================================================
-- DOMAIN 10: HOURLY PRODUCTION VELOCITY (TIME-SERIES AGGREGATE)
-- ============================================================================

CREATE TABLE IF NOT EXISTS hourly_line_metrics (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id VARCHAR(64) NOT NULL REFERENCES tenants(id),
    line_id VARCHAR(64) NOT NULL,
    shift_date DATE NOT NULL,
    hour_window INT NOT NULL CHECK (hour_window BETWEEN 1 AND 12),
    target_units INT NOT NULL,
    actual_units INT NOT NULL DEFAULT 0,
    defect_units INT NOT NULL DEFAULT 0,
    andon_downtime_minutes INT NOT NULL DEFAULT 0,
    dhu_pct NUMERIC(5,2) GENERATED ALWAYS AS (
        CASE WHEN actual_units > 0 THEN (defect_units::numeric / actual_units::numeric) * 100 ELSE 0 END
    ) STORED,
    recorded_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_line_hour UNIQUE (tenant_id, line_id, shift_date, hour_window)
);
ALTER TABLE hourly_line_metrics ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation_hourly_metrics ON hourly_line_metrics FOR ALL USING (tenant_id = CURRENT_SETTING('app.current_tenant_id'));

8.2 In-Memory Directed Attachment DAG Engine (attachment_graph.go)
package main

import (
	"errors"
	"fmt"
	"sync"
)

type SubBundleNode struct {
	SequentialID int    `json:"sequential_id"`
	CutNo        int    `json:"cut_no"`
	PantPosition int    `json:"pant_position"`
	SubBundleID  int    `json:"sub_bundle_id"` // 16-part component ID
	PartName     string `json:"part_name"`
	PlyStart     int    `json:"ply_start"`
	PlyEnd       int    `json:"ply_end"`
}

type AttachmentEngine struct {
	sync.RWMutex
	validEdges   map[string]map[int]map[int]bool // Style -> SourcePart -> TargetPart -> true
	bundleLookup map[string]SubBundleNode        // NFC_UID -> Node
}

func NewAttachmentEngine() *AttachmentEngine {
	return &AttachmentEngine{
		validEdges:   make(map[string]map[int]map[int]bool),
		bundleLookup: make(map[string]SubBundleNode),
	}
}

func (e *AttachmentEngine) ValidatePair(styleID, sourceNFC, targetNFC string) error {
	e.RLock()
	defer e.RUnlock()

	src, okSrc := e.bundleLookup[sourceNFC]
	tgt, okTgt := e.bundleLookup[targetNFC]
	if !okSrc || !okTgt {
		return errors.New("ERR_UNREGISTERED_TAG: Tag missing from active floor pool")
	}

	// Invariant: Cross-cut assembly is prohibited
	if src.CutNo != tgt.CutNo {
		return fmt.Errorf("ERR_CROSS_CUT: Cannot attach Cut %d to Cut %d", src.CutNo, tgt.CutNo)
	}

	// Invariant: Cross-pant assembly (shade-mix) is prohibited
	if src.PantPosition != tgt.PantPosition {
		return fmt.Errorf("ERR_SHADE_MISMATCH: Bundle %d (Pant %d) cannot marry Bundle %d (Pant %d)",
			src.SequentialID, src.PantPosition, tgt.SequentialID, tgt.PantPosition)
	}

	// Invariant: Exact ply-group alignment required
	if src.PlyStart != tgt.PlyStart || src.PlyEnd != tgt.PlyEnd {
		return fmt.Errorf("ERR_PLY_MISMATCH: Ply group mismatch [%d-%d] vs [%d-%d]",
			src.PlyStart, src.PlyEnd, tgt.PlyStart, tgt.PlyEnd)
	}

	// Invariant: Directed edge validation
	if !e.validEdges[styleID][src.SubBundleID][tgt.SubBundleID] {
		return fmt.Errorf("ERR_INVALID_ATTACHMENT: Part %s cannot attach to %s", src.PartName, tgt.PartName)
	}

	return nil
}

8.3 Multi-Slot Ratio Packing Engine (carton_packer.go)
package packing

import (
	"errors"
	"fmt"
	"sync"
)

type RatioSlot struct {
	UPC       string `json:"upc"`
	TargetQty int    `json:"target_qty"`
	PackedQty int    `json:"packed_qty"`
}

type CartonPackingSession struct {
	sync.Mutex
	BoxID            int          `json:"box_id"`
	PackType         string       `json:"pack_type"` // "SOLID" or "RATIO"
	Slots            []*RatioSlot `json:"slots"`
	ExpectedWeightKg float64      `json:"expected_weight_kg"`
	ToleranceKg      float64      `json:"tolerance_kg"`
	IsSealed         bool         `json:"is_sealed"`
}

func (s *CartonPackingSession) ScanItemUPC(upc string) error {
	s.Lock()
	defer s.Unlock()

	if s.IsSealed {
		return errors.New("ERR_CARTON_SEALED: Carton session already completed")
	}

	matched := false
	for _, slot := range s.Slots {
		if slot.UPC == upc {
			matched = true
			if slot.PackedQty >= slot.TargetQty {
				return fmt.Errorf("ERR_SLOT_OVERFLOW: Target quantity (%d) for UPC %s already satisfied",
					slot.TargetQty, upc)
			}
			slot.PackedQty++
			return nil
		}
	}

	if !matched {
		return fmt.Errorf("ERR_INVALID_UPC: UPC %s does not belong to this carton manifest", upc)
	}
	return nil
}

func (s *CartonPackingSession) VerifyAndSeal(scaleWeightKg float64) error {
	s.Lock()
	defer s.Unlock()

	// Verify all slot quotas are fulfilled
	for _, slot := range s.Slots {
		if slot.PackedQty != slot.TargetQty {
			return fmt.Errorf("ERR_UNFINISHED_SLOT: UPC %s has %d of %d pieces packed",
				slot.UPC, slot.PackedQty, slot.TargetQty)
		}
	}

	// Verify scale weight bounds
	minWeight := s.ExpectedWeightKg - s.ToleranceKg
	maxWeight := s.ExpectedWeightKg + s.ToleranceKg
	if scaleWeightKg < minWeight || scaleWeightKg > maxWeight {
		return fmt.Errorf("ERR_WEIGHT_MISMATCH: Measured %.3f kg outside valid bounds [%.3f - %.3f] kg",
			scaleWeightKg, minWeight, maxWeight)
	}

	s.IsSealed = true
	return nil
}

8.4 Cryptographic Event Chaining Engine (crypto_ledger.go)
package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"
	"time"
)

type AuditEvent struct {
	Index        uint64 `json:"index"`
	Timestamp    int64  `json:"timestamp"`
	StationID    string `json:"station_id"`
	OperatorID   string `json:"operator_id"`
	EventType    string `json:"event_type"`
	PayloadHash  string `json:"payload_hash"`
	PreviousHash string `json:"prev_hash"`
	EntryHash    string `json:"entry_hash"`
}

type MerkleLedger struct {
	sync.Mutex
	secretKey    []byte
	currentIndex uint64
	lastHash     string
}

func NewMerkleLedger(seedKey string) *MerkleLedger {
	h := sha256.Sum256([]byte("GENESIS_BLOCK_AGIPAREL_REV11"))
	return &MerkleLedger{
		secretKey:    []byte(seedKey),
		currentIndex: 0,
		lastHash:     hex.EncodeToString(h[:]),
	}
}

func (l *MerkleLedger) AppendEvent(stationID, operatorID, eventType, rawPayload string) (AuditEvent, error) {
	l.Lock()
	defer l.Unlock()

	l.currentIndex++
	now := time.Now().UTC().UnixMilli()

	payloadSha := sha256.Sum256([]byte(rawPayload))
	payloadHashStr := hex.EncodeToString(payloadSha[:])

	mac := hmac.New(sha256.New, l.secretKey)
	claim := fmt.Sprintf("%d:%d:%s:%s:%s:%s:%s",
		l.currentIndex, now, stationID, operatorID, eventType, payloadHashStr, l.lastHash)
	mac.Write([]byte(claim))
	entryHashStr := hex.EncodeToString(mac.Sum(nil))

	event := AuditEvent{
		Index:        l.currentIndex,
		Timestamp:    now,
		StationID:    stationID,
		OperatorID:   operatorID,
		EventType:    eventType,
		PayloadHash:  payloadHashStr,
		PreviousHash: l.lastHash,
		EntryHash:    entryHashStr,
	}

	l.lastHash = entryHashStr
	return event, nil
}

9. Role-Based Hardware Assignment Matrix
+───────────────────────────────────────────────────────────────────────────────────────────+
|                                    WORKER-LEVEL HARDWARE                                  |
|                                                                                           |
|  [Sewing Operator]          [Store & Cut Helper]           [Finishing & Packers]          |
|  • Phone / 10" Tablet Mount • Handheld RFID Reader Sled    • Phone / Tablet Kiosk         |
|  • NFC Wristband Login      • NFC Reader / Barcode Scanner • Bluetooth Ring 2D Scanner    |
|  • Machine Relay Interlock  • Industrial Label/Card Printer• Benchtop Weighing Scale      |
+───────────────────────────────────────────────────────────────────────────────────────────+

Operational Persona
Device Form Factor
Physical Mounting / Carrying
Attached Peripherals
Primary Operational Workflow
Sewing Machine Operator
6.5" Phone / 10" Tablet
Fixed C-clamp machine arm
NFC wristband / badge
Large touch targets (DONE, DEFECT, HELP); animated assembly guides.
Cutting Helper
10" Rugged Tablet
Fixed bundling table stand
Flatbed USB NFC reader
Cursor-driven auto-advancing single-tag assignment (20 \pm 2 units).
Store Clerk / Issuer
Handheld Mobile Computer
Holster / Pistol grip
Long-range UHF antenna
Directional inventory search and double-witness issuance (<20\text{ s}).
Line Mechanic
6.5" Rugged Phone
Drop-resistant belt holster
Internal vibration/sound alerts
Real-time breakdown queue, acceptance, and resolution stamps.
In-Line Quality Checker
6.5" Rugged Phone
Neck lanyard / Holster
Integrated camera
Defect logging, photo capture, and immediate routing to alteration.
Hangtag Helper
6.5" Phone / 10" Tablet
Station mount at line exit
Optical barcode reader
Detaches factory RFID, binds buyer hangtag, and recycles tag.
Final Quality Checker
10" Rugged Tablet
Inspection table mount
Wearable ring scanner
1.5s dwell lock, verification, and PASS / REDO routing.
Packer
10" Tablet
Packing station mount
2D ring scanner + scale
Multi-slot ratio pack verification and gross box weight check.
Dock Container Loader
10" Rugged Tablet
Handheld with rear strap
Integrated laser barcode scanner
Decremental count as cartons load; blocks unauthorized boxes.
Line Supervisor
10" Android Tablet
Shoulder sling / Elastic strap
Drop bumper casing
Pacing bars 1–8, bottleneck identification, and float redeployment.
Plant Manager
24"–27" Monitor + Desktop
Office desk
High-resolution display
14-line plant status grid (PM-01), executive overrides, and daily review.
Solution Administrator
Laptop / Workstation
Office desk / Server rack
Secondary diagnostic sled
System configuration, schema setup, edge monitoring, and backup routines.

10. 16-Week Agile Sprint Plan
+─────────────────────────────────────────────────────────────────────────────────+
|                  16-WEEK AGILE SPRINT EXECUTION ROADMAP                         |
|                                                                                 |
|  [ SPRINT 1-2 ] ──► [ SPRINT 3-4 ] ──► [ SPRINT 5-6 ] ──► [ SPRINT 7-8 ]        |
|    Core Engine        Stores & Cut        Sewing &           OCR Freeze,        |
|    & Edge Node        Plan Release       Finishing Bind    Pilot Integration    |
+─────────────────────────────────────────────────────────────────────────────────+

Sprint 1 & 2: Edge Infrastructure & Core Multi-Tenancy Backbone
Epic 1: Multi-Tenant Data Core & API Gateway
Task 1.1: Configure PostgreSQL with dynamic tenant RLS policies on schemas (store_rolls, bundle_registry, floor_issues).
Task 1.2: Deploy ClickHouse cluster and implement telemetry_scans table partitioned by month.
Task 1.3: Stand up Redpanda/Kafka pipeline fanning raw scans into ClickHouse.
Epic 2: On-Premise Plant Edge Daemon (plant-edge-os)
Task 2.1: Implement native Go LLRP protocol driver reading Impinj and Zebra readers over TCP.
Task 2.2: Build edge sliding-window RSSI filter and temporal debouncer suppressing cross-station antenna bleed.
Task 2.3: Implement embedded SQLite WAL for caching cut plans and offline store-and-forward spooling.
Sprint 3 & 4: Sourcing, Fabric Stores & Cutting Reality Execution
Epic 3: Sourcing & Store Governance (Part 3)
Task 3.1: Build the pre-PO surplus allocation gate: \text{Net Indent} = \text{BOM Required} - \text{Unreserved Stock}.
Task 3.2: Implement double-witness issuance endpoint validating active reservations with digital signatures in <20\text{ s}.
Task 3.3: Build CEO consumption-variance authorization view: block issuance on excess draw, capture mandatory narrative story.
Epic 4: Cutting Optimization & Reality Serialization (Part 4)
Task 4.1: Implement CAD marker metadata parser enforcing the 210-lay limit and 16-part component graph.
Task 4.2: Build post-cut actuals capture screen recording physical plies and applying the clamped 2% over-cut buffer.
Task 4.3: Build cursor-driven Cutting Helper UI mapping 16-part Sub-sub Bundles (20 \pm 2 pieces).
Task 4.4: Implement cut_lay_rolls recording and end-bit routing (<1.5\text{m}, 1.5\text{–}5\text{m}, >5\text{m}).
Task 4.5: Enforce pre-sewing fabric balance check: block bundle release if variance > \pm 2\% without CEO approval.
Sprint 5 & 6: Assembly Floor Gating, Finishing Handoff & Pre-Ship
Epic 5: Sewing Execution & Anti-Mistake Gating (Part 5)
Task 5.1: Implement the in-memory directed Attachment Map in plant-edge-os returning <20\text{ ms} ACCEPT/REJECT responses.
Task 5.2: Build Modbus TCP relay actuator to disengage machine motor power upon detected attachment mismatches.
Task 5.3: Implement real-time WebSocket issue broadcasting across floor displays.
Epic 6: Finishing Swap & Multi-Slot Packing (Part 6)
Task 6.1: Build Hangtag Helper station: atomic binding of buyer UPC to internal piece history while returning RFID tag to pool.
Task 6.2: Implement Final Checker UI enforcing a 1.5\text{ s} inspection dwell guardrail.
Task 6.3: Build Packer UI with multi-slot ratio pack decrementing and scale weight verification.
Task 6.4: Implement dock loading decremental scanner linked to container shipping manifests.
Sprint 7 & 8: Order Closure Report (OCR), Integration & Pilot Hardening
Epic 7: Order Closure Report & Reconciliation (Part 7)
Task 7.1: Build reconciliation engine enforcing \text{Inflow} = \text{Forward} + \text{Lost} + \text{Left Over} across all stages.
Task 7.2: Build multi-department digital sign-off workflow enforcing written root-cause explanations on non-zero variances.
Task 7.3: Implement PDF generator outputting immutable, cryptographically sealed OCR certificates.
Epic 8: Field Hardening at OKHLA Unit 002/02 & Go-Live Acceptance Gate
Task 8.1: Execute 4-hour simulated WAN disconnect during peak production across 14 virtual lines (70,000 daily scans). Verify zero data loss and clean catch-up via store-and-forward gRPC.
Task 8.2: Complete 5-shift continuous production run on Pilot Line 14: verify zero cross-shade bundle marriages, double-witness issuance times < 20\text{ s}, and 100% balance reconciliation across all generated OCR records.
11. Operational Acceptance Matrix
Operational Subsystem
Verification Test Scenario
Target SLA / Operational Bound
Acceptance Pass Condition
Edge Hardware Driver
100 bundle tags swept past a fixed dock antenna array
Processing latency \le 20\text{ ms} at edge node
System debounces reads to exactly 100 unique events; zero missed reads.
Material Issuance
Double-witness checkout of fabric roll at store counter
Total execution time \le 20\text{ s}
Issuer and recipient IDs recorded; remaining roll length decrements.
Anti-Mistake Hardware Gate
Operator scans Left Pocket (Pant 1) + Right Front (Pant 1)
Response time \le 20\text{ ms}
Attachment Map rejects pairing; machine motor relay disengages.
Anti-Borrowing Lock
Issuer attempts check-out of roll reserved for Order A to Order B
Programmatic block
Action blocked; prompts for CEO override with written narrative.
Cycle Pacing Guard
Operator attempts CYCLE DONE at 20\% of Factory SAM
Minimum dwell floor: 0.40 \times \text{SAM}
Screen rejects tap with warning chime; zero piece credit awarded.
Inspection Dwell Lock
Checker scans hangtag and attempts PASS within 200\text{ ms}
Fixed inspection dwell: 1.5\text{ s}
Screen displays "Inspect First"; garment cannot enter packing queue.
Multi-Slot Ratio Packing
Packer scans wrong UPC or attempts seal outside \pm 250\text{g}
Instant terminal lockout
UI flags mismatch; carton label generation disabled until weight and slots balance.
Pre-Ship Validation
Loader scans carton not assigned to active PO manifest
Immediate visual/audio warning
Dock tablet rejects scan; container countdown counter does not decrement.
Order Closure (OCR)
Reconcile completed PO: 12,240 cut, 12,000 shipped, 90 lost, 150 surplus
Complete conservation balance
\text{Cut} = \text{Shipped} + \text{Lost} + \text{Left Over}; immutable signed PDF emitted.


