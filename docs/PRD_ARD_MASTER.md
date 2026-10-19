# Agiparel OS: Master Product & Architecture Requirements Document (PRD/ARD)
**Author:** Sunil Jain, CyberinQ LLC  
**Product Baseline:** v1.1.0-PROD  
**Target Environment:** Local Apple Silicon iMac (Docker Compose) + Staging Ingress via Cloudflare Tunnel (`agiparel.empowerai.us`)  
**Pilot Ground Truth:** Noble Garments, OKHLA Unit 002/02 | Pilot Line 14 | Buyer: Pery | Style: KK524867 | PO: MK6574J9  

---

## 1. Executive Vision & Core Operating Axioms
Agiparel OS transforms apparel manufacturing from an approximate, retrospective accounting exercise into an **unspoofable, deterministic cyber-physical execution substrate**. It enforces two foundational axioms:
1. **"Make blame impossible by making evidence automatic."**
2. **"Quality and compliance are enforced by impossibility, not by post-hoc inspection."**

Every physical transaction across Stores, Cutting, Sewing, Finishing, Packing, and Loading is gated by **deterministic physical telemetry**: high-frequency NFC tokens, long-range Live UHF RFID tags, optical barcodes, quadrature rotary encoders, physical scale telemetry, and hardware motor interlocks.

---

## 2. Bimodal Architecture & The "Agie" AI Onboarding Engine
### 2.1 Bimodal Topology
- **CyberinQ Central Cloud Control Plane:** Manages tenant billing (Stripe/Razorpay), multi-tenant SaaS routing, and issues cryptographically signed time-boxed license tokens (`license.jwt`) that unlock specific modules (`mod_stores`, `mod_cutting`, `mod_sewing`, `mod_packing`, `mod_ocr`).
- **Factory Edge Binaries (`plant-edge-os`):** Self-contained, compiled Go static binaries running on an on-premise industrial mini-PC (Intel N100, fanless, 15W) inside an isolated Docker container with an embedded SQLite Write-Ahead Log (WAL) and HMAC-SHA256 Merkle WORM ledger. Operates 100% air-gapped with zero WAN dependency.

### 2.2 "Agie": The Autonomous Factory Onboarding Agent
Instead of manual IT configuration, an AI agent named **Agie** conducts audio/text interviews with the Factory GM, Line Supervisors, and Store Managers:
- **Conversational Discovery:** Agie gathers shift timings, machine classes, bottleneck frequencies, and operation bulletins in local languages.
- **Dynamic Schema Extension ("DB-Smith"):** Safely provisions custom table attributes and JSONB fields on the fly without breaking core WORM ledgers.
- **Workflow & Rule Mapping ("Logic-Weaver"):** Translates verbal rules into in-memory directed attachment DAGs and dwell-time interlocks.
- **Auto-Documentation ("Doc-Scribe"):** Generates 5th-grade plain-English user manuals and multi-lingual audio prompt scripts.
- **Human-in-the-Loop Gate:** Agie presents its configuration plan on a clean review dashboard, requiring a single-click **"Approve & Go Live"** from the Factory Head.

---

## 3. Physical & Behavioral Guardrails (Zero-Trust Floor)
- **Zero Floor Typing ($\le$ 5th-Grade Comprehension):** Operators never type text. They interact with large high-contrast touch targets ($\ge 80\text{mm}$): **Green (CYCLE DONE)**, **Amber (DEFECT TAG)**, and **Red (HELP / ANDON)**.
- **Anti-Ghost Tap Dwell Floor ($t_{\text{min}}$):** Action buttons remain locked until an empirical cycle floor expires ($t_{\text{min}} = \text{SAM} \times 0.40$), preventing piece-rate gaming.
- **Anti-Shade Mixing Interlock:** The in-memory directed attachment DAG validates panel compatibility in $\le 20\text{ ms}$. Mismatches trigger an instant alert and disengage sewing machine motors via Modbus TCP relays.
- **1.5-Second Inspection Dwell Guard:** Final Quality Checkers cannot rubber-stamp garment passes; the PASS button remains locked for 1.5 seconds following a hangtag scan.

---

## 4. Supported RBAC Logins, Machines & Materials
- **RBAC Roles:** Sewing Machine Operator, Cutting Helper, Store Clerk/Issuer, Line Mechanic, In-Line Checker, Hangtag Helper, Final Quality Checker, Packer, Dock Loader, Line Supervisor, Quality Lead, Plant Manager, Top Management Council, 6 Order Closure Department Heads (Merchandising, IE, Sourcing, Production, Quality, Finance), and Solution Administrator.
- **Machine Classes:** 23 canonical types pre-seeded (featuring SNLS, 4-OL, Kansai, CAM-Cutter, Fusing Press, Boiler Iron, etc.).
- **Materials (3-Tier Hierarchy):** Fabrics (Denim, Twill, Pocketing with 4-point defect coordinates), Trims (Zippers, Buttons, Elastics, Thread, Hangtags), and Packaging (Solid/Ratio cartons, NFC hard tags, UHF roll labels).

---

## 5. Disaster Recovery & Business Continuity (RTO / RPO)
- **Zero-Loss WAL Spooling (RPO = 0):** Every scan and cycle commit writes instantly to local SQLite WAL files before ACKs.
- **Container Self-Healing:** Docker Compose manages local lifecycles (`restart: unless-stopped`), automatically restarting `plant-edge-os` in <1 second upon failure and replaying WAL logs.
- **Fast Recovery (RTO < 15 mins):** Daily automated encrypted snapshots to local NAS or USB ensure rapid hardware replacement recovery.