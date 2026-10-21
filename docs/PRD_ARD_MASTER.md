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

Physical Carrier Segmentation & Flow Architecture:
The platform enforces a 4-Tier physical carrier model matching the factory lifecycle[cite: 5]:
1. Tag Type 1 — Roll Core Tag: RAIN UHF (860–960 MHz), writable, reusable, mounted on the fabric roll core[cite: 5]. Encodes factory SKU, measured length, minimum usable width, 2D flaw coordinates, shade lot, and remaining length[cite: 5]. Interrogated by Overhead UHF antennas and Handheld UHF Sleds (Chainway C72)[cite: 5, 7]. Removed and redeployed when the roll is depleted[cite: 5].
2. Tag Type 2 — Sub-Bundle Disc Tag: Encapsulated ABS HF/NFC button disc (13.56 MHz, NTAG213 / ICODE SLIX), 100% closed-loop reusable[cite: 5, 7]. Encodes sequential bundle ID, Cut #, Pant #, Part description, and physical Ply Range (e.g., 1–20)[cite: 5, 7]. Interrogated via flatbed USB NFC pads at bundling tables and station phone NFC pads[cite: 5].
3. Tag Type 3 — Permanent Garment Tag: Washable fabric inlay RAIN UHF (860–960 MHz, EPC Gen2), permanently stitched into the care label or waistband[cite: 5]. Encodes SGTIN-96 (Buyer GTIN/UPC + sequential garment serial number)[cite: 5]. Survives 50–100 industrial wash/dry cycles[cite: 5]. Interrogated by near-field UHF/barcode scanners at hangtag helper stations, ring scanners at final QC, and bulk packing tunnels[cite: 5].
4. Tag Type 4 — Carton Shipping Tag: RAIN UHF label + GS1-128 / SSCC barcode, single-use, applied on outer cartons[cite: 5]. Encodes Carton ID, buyer manifest contents, and gross weight verification[cite: 5]. Interrogated by packing tunnels and dock door portals[cite: 5].

Process Telemetry Engine:
"Every RFID read is an event --> Correlated events become process measurements --> Process measurements become actionable intelligence."[cite: 14]
By correlating tag identities, reader locations, and timestamps, the edge kernel reconstructs the production journey and continuously measures:
a) Cycle time: Net duration of each manufacturing operation against Factory SAM[cite: 14].
b) Interprocess latency: Time elapsed between successive operations across workstations[cite: 14].
c) Variability: Mean, standard deviation, and outliers across operators, machines, and shifts[cite: 14].
d) Bottlenecks: Real-time detection of where WIP accumulates and line velocity degrades[cite: 14].
e) Traceability: Complete garment genealogy from raw fabric roll to shipping container[cite: 14].

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
Invariant 5: Directed Sub-Bundle Attachment DAG & Marriage Retirement Protocol
Garment assembly pairings are verified at scan time using an in-memory directed graph[cite: 3]. In any joining operation (e.g., Pocket joining Front Panel), the larger component is designated as the Master Carrier[cite: 5].
1. At a marriage station, the operator taps Secondary Sub-Bundle Tag A on the fixed station NFC pad, followed by Master Sub-Bundle Tag B[cite: 5].
2. The edge engine evaluates compatibility in <= 20 ms:
   Scan Status = ACCEPT  if (B_source, B_target) ∈ E  AND  Pant(B_source) == Pant(B_target)  AND  PlyRange(B_source) == PlyRange(B_target)
3. On MATCH: Tag B absorbs Tag A's complete piece lineage in the database; Tag A is retired and physically dropped into the station recovery bin to be cleaned, re-encoded, and returned to the Cutting Dept pool[cite: 5].
4. On MISMATCH: The edge engine immediately triggers a 150Hz warning tone, blocks piece credit, and disengages machine motor power via a Modbus TCP relay[cite: 3, 5].

Invariant 6: Permanent Tag Swap & SGTIN-96 Binding
At the exit of the sewing floor (Hangtag Helper station FH-01), the final Master Sub-Bundle NFC tag is removed and retired to the central recyclable pool[cite: 5, 10]. An atomic database transaction binds the internal WIP tracking record to the garment's permanent washable RAIN UHF inlay (Tag Type 3) and buyer hangtag barcode[cite: 3, 5]:
   PermanentBind = <Internal Bundle ID, Garment Serial, Buyer GTIN/UPC, SGTIN-96 EPC, Helper ID, t_timestamp>[cite: 3, 5]
This preserves complete piece-level history through final inspection, multi-slot carton packing, warehouse logistics, and retail checkout[cite: 3, 5].

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

Operational Persona | Device Form Factor | Attached Readers & Peripherals | Primary Operational Workflow
-------------------|-------------------|--------------------------------|-----------------------------
Stores Clerk / Receiver | Handheld Mobile Computer (Chainway C72)[cite: 3] | Integrated Long-range UHF antenna + 2D scanner[cite: 3] | Inspect incoming rolls, write measured length & 2D flaw map to Tag Type 1, zoned inventory Geiger search, double-witness issuance[cite: 3, 5].
Cutting Helper | 10" Rugged Tablet[cite: 3] | Flatbed USB NFC Reader + Industrial Card Printer[cite: 3] | Cursor-driven auto-advancing single-scan binding of 20-piece Sub-Bundle Disc Tags (Tag Type 2)[cite: 3, 5].
Sewing Machine Operator | 6.5" Android Smartphone[cite: 3] | Machine C-clamp mount with integrated Phone NFC Pad[cite: 3, 5] | Tap bundle NFC tag on station pad; evaluate marriage/absorb sub-tags (<20ms); anti-ghost dwell lock (40% SAM)[cite: 3, 5].
Line Quality Checker | 6.5" Rugged Phone[cite: 3] | Integrated camera + Bluetooth ring scanner[cite: 3] | In-line inspection, defect tagging, repair routing[cite: 3].
Hangtag Helper | 6.5" Phone / 10" Tablet[cite: 3] | Dual Reader: USB/Phone NFC Pad + Near-field UHF / 1D-2D Barcode scanner[cite: 3, 5] | Atomic swap: scan final Master NFC Tag, bind to permanent washable UHF label (SGTIN-96) and buyer hangtag, recycle NFC tag[cite: 3, 5].
Final Quality Checker | 10" Rugged Tablet[cite: 3] | Bluetooth 2D Ring Scanner / Near-field UHF pad[cite: 3, 5] | Scan permanent garment UHF tag; enforce 1.5s inspection dwell lock; PASS or REDO routing[cite: 3, 5].
Carton Packer | 10" Tablet[cite: 3] | Benchtop Certified Scale + Bulk UHF Packing Tunnel / Scanner[cite: 3, 5] | Scan carton manifest, multi-slot ratio pack verification, instantaneous multi-tag scan of closed carton, gross weight balance[cite: 3, 5].
Dock Container Loader | 10" Rugged Tablet[cite: 3] | Overhead Dock Door UHF Portal Antennas + Handheld Scanner[cite: 3, 5] | Decremental loading scan against container manifest; rejects unverified cartons[cite: 3].
Plant Manager / Executive | Desktop Workstation[cite: 3] | Situation Room Display (PM-01)[cite: 3] | 14-line telemetry overview, live process latency & bottleneck monitoring, Agie ReAct AI drawer[cite: 3].

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


Core Architectural Principles Moving ForwardZero Hardcoded Garment Limits: Fluid assembly DAGs, flexible part counts, dynamic BOMs, and arbitrary buyer packing specifications stored as queryable JSONB and relational graphs.No "Toy" LLM Stubs: Agie operates as an autonomous ReAct query agent equipped with deterministic tool execution (run_safe_sql, catalog introspection)—no static string stuffing in Go.   Unified Baseline Migrations: Clean DDL migrations, automated via CI/CD, eliminating runtime DDL side-effects.Exact Marvel Screen Fidelity: Stop inventing new UI layouts; map the back-end APIs directly to the wireframe mockups provided by Noble Garments.