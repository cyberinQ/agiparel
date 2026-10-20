Author: Sunil Jain, CyberinQ LLC

TL;DR
What was in the PRD-ARD was a high-level 16-week project roadmap (Section 10).

An MVP Sprint Execution Backlog is different: it trims the 16-week, 7-module scope down to a 14-day "Thin Thread" pilot (targeting a single cutting table and Line 14 at OKHLA Unit 002/02 with the SAMPLE DATA.xlsx order: Style KK524867, PO MK6574J9). It breaks down tasks into ready-to-code, testable Epics, User Stories, Gherkin Scenarios, and Developer Tasks formatted specifically for your VSCode markdown files.

Below is the standalone MVP Epic & User Story Backlog (MVP_EPICS_STORIES.md), ready to commit to your GitHub repository.

Markdown
# Agiparel OS: 14-Day Thin Thread MVP Backlog (`MVP_EPICS_STORIES.md`)

**Author:** Sunil Jain, CyberinQ LLC  
**Target Environment:** Local iMac (Docker Compose) + Staging Ingress via Cloudflare Tunnel (`agiparel.empowerai.us`)  
**Pilot Ground Truth:** Noble Garments, OKHLA Unit 002/02 | Pilot Line 14 | Buyer: Pery | Style: KK524867 | PO: MK6574J9  
**Pilot Volume:** 1 Cut (Cut 1: 624 pcs, 16 parts, 30 bundles) | Solid + Ratio Pack (Boxes 1–50)  

---

## EPIC 1: Air-Gapped Local Substrate & Edge Kernel

### Story 1.1: Multi-Tenant Schema & Relational RLS Core
**As a** Lead Systems Architect,  
**I want to** initialize PostgreSQL 16 with Row-Level Security on all transactional tables (`store_rolls`, `bundle_registry`, `cut_lay_rolls`, `carton_pack_manifests`, `ocr_records`),  
**So that** tenant data isolation is mathematically enforced at the database engine level with zero cross-tenant leakage.

#### Acceptance Criteria
* **Gherkin Scenario: Tenant Isolation Enforcement**
  ```gherkin
  Given an authenticated connection under tenant "TENANT_NOBLE_OKHLA"
  When a query executes "SELECT * FROM store_rolls"
  Then only rows matching tenant_id "TENANT_NOBLE_OKHLA" are returned
  And rows belonging to "TENANT_TEST_99" are invisible even to table scans.


Check constraints strictly reject negative or zero values on piece counts and lengths.


Pre-seeded with canonical machine types (SNLS, 4-OL, Kansai) and ASTM D5430 defect classes.


Dev Tasks
[ ] Write services/core-api/migrations/0001_initial_schema.sql with tables, indexes, and foreign keys.


[ ] Write services/core-api/migrations/0002_rls_policies.sql enforcing SET app.current_tenant_id.


[ ] Add unit test script tests/integration/rls_isolation_test.go simulating cross-tenant query injection.


Story 1.2: Single-Binary Edge Kernel (plant-edge-os) with SQLite WAL
As a Floor Solutions Admin,

I want to run a compiled Go static binary on the on-premise edge gateway maintaining an embedded SQLite write-ahead log (WAL),

So that station telemetry persists and processes in 20ms even during complete factory WAN blackouts.

Acceptance Criteria
Gherkin Scenario: Air-Gapped WAN Failure Survivability



Gherkin
Given the factory WAN router link is physically severed
When 500 station scan events are received by plant-edge-os
Then all 500 events are committed to local SQLite WAL with 0 drops
And edge API returns HTTP 200 / ACK in <= 20 ms
When WAN reconnects, all records sync upstream to ClickHouse/PostgreSQL via mTLS.




Dev Tasks
[ ] Scaffold services/edge-daemon/cmd/server/main.go with Go 1.22.


[ ] Implement services/edge-daemon/internal/store/sqlite_wal.go with auto-checkpointing.


[ ] Implement upstream sync worker with exponential backoff and backpressure flow control.


Story 1.3: Cryptographic Append-Only Merkle Event Ledger
As a Compliance Auditor and Factory General Manager,

I want every floor transaction hashed sequentially into an HMAC-SHA256 audit chain,

So that neither management nor labor representatives can manipulate historical shift records, piece rates, or downtime logs.

Acceptance Criteria
Gherkin Scenario: Tamper-Evident History



Gherkin
Given a sequence of 100 historical scan events in the ledger
When an administrator manually updates a quantity row in PostgreSQL
Then the ledger verification endpoint flags a hash-chain break at that index
And blocks Order Closure Report generation.




Dev Tasks
[ ] Implement services/edge-daemon/internal/crypto/merkle_ledger.go.


[ ] Add SHA-256 genesis block generator and verification validator.


[ ] Expose /api/v1/ledger/verify health endpoint.


EPIC 2: Stores Intake & "Material Radar"
Story 2.1: Pre-PO Surplus Matching Gate
As a Sourcing Lead,

I want the system to compute NetIndent=BOMRequired-AvailableUnreserved,

So that purchase orders for gross quantities are blocked when matching materials exist in inventory.

Acceptance Criteria
Gherkin Scenario: Pre-PO Deduction (Sheet 3 Ground Truth)



Gherkin
Given BOM required pocketing fabric is 2608 yds
And unreserved stock in store is 665 yds
When the merchandiser generates a material indent
Then the system sets maximum allowed purchase indent to exactly 1943 yds
And blocks any gross PO creation without an executive override code.




Dev Tasks
[ ] Implement indent validation endpoint in services/core-api/internal/stores/indent.go.


[ ] Write integration test verifying Button (23,211 req - 1,509 stock = 21,702 indent) calculation.


Story 2.2: Double-Witness Store Issuance Checkout (<20s SLA)
As a Store Issuer and Cutting Spreader,

I want to execute a two-party digital checkout at the store window using NFC badge taps,

So that fabric roll handoffs are recorded without paper slips in under 20 seconds.

Acceptance Criteria
Gherkin Scenario: Atomic Material Issuance



Gherkin
Given Roll #7 is reserved for Cut 1 of PO MK6574J9
When Store Issuer taps badge and Cutting Spreader taps badge
Then 59 meters are deducted from Roll #7 Live RFID record
And store_issuances logs both user IDs with a cryptographic timestamp in < 20 seconds.




Dev Tasks
[ ] Implement POST /api/v1/stores/issue-double-witness endpoint.


[ ] Build mobile PWA screen STR-04 with high-contrast dual-PIN / NFC tap inputs.


EPIC 3: Cutting Floor Reality Execution & 16-Part Bundling
Story 3.1: Cut Plan Optimizer with Clamped 2% Buffer
As a Cutting Master,

I want the cut planning engine to enforce the 210-lay limit and the clamped 2% over-cut formula,

So that marker lay allocations fulfill order quotas without generating unauthorized surplus.

Acceptance Criteria
Gherkin Scenario: Cut Plan Buffer Calculation (Sheet 5 Ground Truth)



Gherkin
Given Order Qty of 600 pcs for Size 29/30
When Cut Plan optimization runs
Then clamped extra is Clamp(round(0.02 * 600), 1, 15) = 12 pcs
And target cut volume is set to exactly 612 pcs
And maximum spread lay height is capped at 210 plies.




Dev Tasks
[ ] Implement cut buffer and lay splitter in services/core-api/internal/cutting/planner.go.


[ ] Unit test against all 15 size/color variants from SAMPLE DATA.xlsx Sheet 5.


Story 3.2: Roll-to-Lay Consumption Lineage & End-Bit Ledger
As a Cutting Incharge,

I want to log the exact lay ranges produced by each roll and record remaining end-bits,

So that downstream panel defects trace back to the source roll and end-bits are routed correctly.

Acceptance Criteria
Gherkin Scenario: End-Bit Threshold Routing (Sheet 5 Ground Truth)



Gherkin
Given Roll #3 completes Lays 1 to 28 of Cut 1 with 1.3m remaining
And Roll #31 completes Lays 151 to 176 with 16.0m remaining
When the spreader finalizes the spread
Then Roll #3 (1.3m < 1.5m) is automatically flagged as SCRAP
And Roll #31 (16m > 5.0m) is flagged as STORE_RETURN and re-entered into inventory.




Dev Tasks
[ ] Create table cut_lay_rolls and write recording endpoint POST /api/v1/cutting/lay-roll.


[ ] Build tablet view CUT-02 allowing single-tap end-bit metric logging.


Story 3.3: Cursor-Driven Single-Scan Bundling Helper (CUT-04)
As a Bundling Helper,

I want to map physical recyclable NFC tags to the 16-part component list using an auto-advancing cursor,

So that bundle cards are bound to actual physical plies in sequential order ($1 \dots N$).

Acceptance Criteria
Gherkin Scenario: Cursor Auto-Advance Mapping



Gherkin
Given Cut 1 actual plies are confirmed at 208 plies (bundle size = 20)
When the bundling helper opens CUT-04
Then the active cursor highlights Row 1 (Sequential Bundle #1, Front Panel)
When an NFC tag UID "04A1B2C3D4" is scanned
Then the tag binds to Bundle #1 in < 50 ms
And the screen chime sounds, auto-advancing the cursor to Row 2.




Dev Tasks
[ ] Build bundle_registry batch allocation generator.


[ ] Build React frontend component CUT-04 with keyboard/NFC listener and auto-advancing cursor.


EPIC 4: Sewing Line Execution & Anti-Mistake Interlock
Story 4.1: In-Memory Directed Attachment DAG Engine
As a Sewing Machine Operator,

I want the edge engine to validate part pairings in 20ms against an in-memory graph,

So that attaching panels from different plies, pants, or cuts is physically blocked.

Acceptance Criteria
Gherkin Scenario: Shade-Mix Interlock Prevention



Gherkin
Given Bundle A is Cut 1, Pant 1, Left Pocket (Ply 1-20)
And Bundle B is Cut 1, Pant 2, Right Front (Ply 1-20)
When the operator scans Bundle A followed by Bundle B at Station 04
Then the in-memory DAG rejects the pairing in <= 6 ms
And returns error "ERR_SHADE_MISMATCH: Pant 1 cannot attach to Pant 2"
And disengages machine motor power via Modbus TCP relay within 15 ms.




Dev Tasks
[ ] Implement services/edge-daemon/internal/attachment/attachment_graph.go.


[ ] Implement Modbus TCP relay driver in services/edge-daemon/internal/modbus/relay.go.


[ ] Add benchmark test ensuring graph validation completes in <6ms for 10,000 tag pairs.


Story 4.2: Zero-Typing Operator HMI (SEW-01) & Anti-Ghost Tap Guard
As a Machine Sewing Operator (≤ 5th-grade reading level),

I want to tap large, high-contrast action targets with spoken dialect audio guidance,

So that I record production without typing, while the system prevents accidental double-taps.

Acceptance Criteria
Gherkin Scenario: Anti-Ghost Tap Dwell Floor



Gherkin
Given an operation with Factory SAM of 0.60 minutes (36 seconds)
And minimum cycle floor is 40% of SAM (14.4 seconds)
When the operator taps "CYCLE DONE" at 8 seconds post-cycle start
Then the tap is rejected with an audible warning chime
And zero piece-rate credit is awarded.




Dev Tasks
[ ] Build kiosk React screen SEW-01 with 80mm buttons: Green (DONE), Amber (DEFECT), Red (HELP).


[ ] Implement client-side dwell timer lockout tied to operation SAM parameters.


[ ] Integrate local audio chimes (800Hz success, 150Hz warning, Hindi/Bengali error prompts).


EPIC 5: Finishing Handoff, Multi-Slot Ratio Packing & Shipping
Story 5.1: Hangtag Helper Atomic Swap (FH-01)
As a Hangtag Helper,

I want to scan the factory internal NFC bundle tag and the buyer's serialized hangtag barcode,

So that full production genealogy transfers to the buyer UPC while the NFC tag is returned to the pool.

Acceptance Criteria
Gherkin Scenario: Atomic Hangtag Swap



Gherkin
When the helper scans factory NFC "04A1B2C3" and buyer barcode "760925382574"
Then hangtag_binds records the link atomically
And the internal NFC tag status reverts to "AVAILABLE"
And the buyer hangtag is locked to prevent duplicate usage.




Dev Tasks
[ ] Implement POST /api/v1/finishing/hangtag-bind endpoint with unique constraints.


[ ] Build rapid-scan dual barcode/NFC view FH-01.


Story 5.2: Final QC Checker HMI (FH-03) with 1.5s Dwell Guard
As a Final Quality Checker,

I want to scan hangtags and inspect garments before tapping PASS or REDO,

So that rubber-stamping is physically prevented by a mandatory inspection delay.

Acceptance Criteria
Gherkin Scenario: 1.5-Second Inspection Dwell Lock



Gherkin
Given a garment hangtag is scanned at Final QC
When the checker attempts to tap "PASS" within 400 ms of scan
Then the UI rejects the tap and displays "Inspect First"
And only enables the PASS button after exactly 1.50 seconds has elapsed.




Dev Tasks
[ ] Build FH-03 inspection HMI with hardware dwell clock enforcement.


[ ] Add rework classification picker (REDO_HANGTAG, REDO_IRON, REDO_SEWING).


Story 5.3: Multi-Slot Ratio Packing Station (PK-02)
As a Carton Packer,

I want the system to guide ratio-pack carton filling and verify gross box weight on an attached scale,

So that short-shipments and ratio errors are prevented prior to carton sealing.

Acceptance Criteria
Gherkin Scenario: Ratio Pack & Weight Enforcement (Sheet 2 Ground Truth)



Gherkin
Given Carton #1 manifest requires 2x UPC_1 + 2x UPC_2 + 2x UPC_3 (Target: 6.20 kg ± 250g)
When the packer scans 2x of UPC_1 and 2x of UPC_2
Then the carton status remains "INCOMPLETE" and label print is disabled
When the final 2x of UPC_3 are scanned and scale reads 6.22 kg
Then the system emits carton seal authorization and prints the box label.




Dev Tasks
[ ] Implement ratio-packing session state machine in services/core-api/internal/packing/carton.go.


[ ] Build PK-02 packing station tablet view with live slot counters and scale weight check.


EPIC 6: Order Closure Report (OCR) & Management Dashboard
Story 6.1: Closed-Loop Mathematical Conservation Engine
As a Factory Head and Merchandiser,

I want the system to verify mathematical closure across all departments:


Cut=Shipped+Lost+LeftOver
So that no order can be closed or invoiced while unexplained garment discrepancies exist.

Acceptance Criteria
Gherkin Scenario: Conservation Equation Validation



Gherkin
Given Cut 1 total is 624 garments
And Shipped = 612, Lost = 8, Left Over = 4
When the closure engine evaluates the order
Then 612 + 8 + 4 = 624, conservation passes
And the system enables 6-department digital sign-offs.




Dev Tasks
[ ] Implement conservation solver in services/core-api/internal/ocr/calculator.go.


[ ] Build reconciliation workspace screen OC-01.


Story 6.2: Cryptographically Sealed OCR PDF Generator
As a Buyer Compliance Auditor,

I want to download an immutable, cryptographically sealed OCR PDF certificate,

So that historical order reconciliation records cannot be altered post-closure.

Acceptance Criteria
Gherkin Scenario: Immutable PDF Certificate



Gherkin
Given an order with all 6 department sign-offs completed
When the system generates "OCR-MK6574J9-001.pdf"
Then the document hash is committed to the HMAC-SHA256 WORM ledger
And any subsequent edits require a numbered addendum document.




Dev Tasks
[ ] Integrate local PDF generation pipeline in services/core-api/internal/ocr/pdf.go.


[ ] Add digital signature verification stamp and HMAC hash watermark.


Story 6.3: Plant Manager Dashboard (PM-01) & Cloudflare Staging Ingress
As an Executive / CEO,

I want to view live 14-line pacing, DHU metrics, and bottlenecks at agiparel.empowerai.us,

So that I have real-time visibility into factory operations without exposing local OT ports to the public internet.

Acceptance Criteria
Gherkin Scenario: Secure Staging Tunnel Access



Gherkin
Given cloudflared tunnel daemon is running on the host
When navigating to "[https://agiparel.empowerai.us](https://agiparel.empowerai.us)"
Then PM-01 renders live Line 14 output bars, DHU heatmaps, and active issues
And edge API queries complete over local WebSockets in < 100 ms.




Dev Tasks
[ ] Configure compose/docker-compose.dev.yml with cloudflared tunnel routing to local NGINX.


[ ] Build desktop management view PM-01 in React with 5-second polling/WebSocket push.



---

### Verification and Execution Steps

You can now paste this backlog directly into your workspace:

1. **Create the documentation directory and save the file:**
   ```bash
   mkdir -p DOCS
   # Save the content above into DOCS/MVP_EPICS_STORIES.md

Commit and push to GitHub:


Basgit add DOCS/MVP_EPICS_STORIES.md
git commit -m "docs(mvp): define 14-day thin thread epics, user stories, and acceptance criteria"
git push origin main




Begin Development:
Start implementation with Epic 1, Story 1.1 (scaffolding the multi-tenant PostgreSQL schema with Row-Level Security).


Markdown
## EPIC 7: Executive Intelligence & Semantic Ground-Truth Querying

### Story 7.1: Semantic NL-to-SQL Diagnostic Tool for Agie
**As a** Factory General Manager,  
**I want to** ask conversational questions ("Who is absent on Line 14?", "Which station has the most breakdowns?") in natural language,  
**So that** Agie translates intent into parameterized, safe SQL queries and provides factually verified answers in < 3 seconds.

#### Acceptance Criteria
* **Gherkin Scenario: Real-Time Attendance Query**
  ```gherkin
  Given 2 operators on Line 14 have status 'ABSENT' in attendance_register
  When the GM asks Agie: "who is absent today on Line 14?"
  Then Agie executes safe query against attendance_register
  And answers with exact names, stations, and the downstream pacing impact.

Story 7.2: Rolling Hourly Trend & Defect Velocity Engine
As an Operations Director,
I want Agie to compare morning vs. afternoon pacing and defect rates across hourly buckets,
So that I receive proactive notifications of pacing drops before the shift concludes.
Acceptance Criteria
Gherkin Scenario: Velocity Comparison
Gherkin
Given Line 14 completed 58 pcs/hr in Hours 1-3 and 41 pcs/hr in Hour 4
When the GM asks: "are we producing faster or slower than this morning?"
Then Agie calculates the -29.3% velocity delta
And cites the Station 03 thread tension Andon event as the primary root cause.




Story 7.3: Dynamic Executive Memo & Action Dispatch (PM-02)
As a Plant General Manager,
I want Agie to extract structured action directives when I say "remember this" or "make a note",
So that operational decisions made during conversations persist as actionable tasks assigned to specific department heads.
Acceptance Criteria
Gherkin Scenario: Memo to Action Delegation
Gherkin
When the GM tells Agie: "make a note: assign float mechanic Ramesh to fix tension on Station 03"
Then Agie records an Executive Directive in floor_issues
And tags severity='attention', target='LINE14_ST03', owner='MECH_RAMESH'
And renders confirmation in the PM-02 Live Issues Feed.





---

### 4. Implementation Readiness Check

With this updated framing, your architecture now provides symmetric coverage across both tiers:

| Operational Layer | Bottom-Up Cyber-Physical (Original)[cite: 6] | Top-Down Cognitive (New Addition) |
| :--- | :--- | :--- |
| **Material Stores**[cite: 6] | Double-witness issuance & Pre-PO surplus gate[cite: 5, 6] | *"How many days of twill pocketing do we have left?"* |
| **Cutting Floor**[cite: 6] | Clamped 2% buffer & 210-lay limit[cite: 5, 6] | *"Are we on track to finish Cut 1 before 2 PM?"* |
| **Sewing Floor**[cite: 6] | In-memory DAG & 40% SAM dwell floor[cite: 5, 6] | *"Who is bottlenecking Line 14 right now?"* |
| **Workforce / HR**[cite: 6] | NFC wristband logins[cite: 5, 6] | *"Who is absent and how does it hurt capacity?"* |
| **Quality / OCR**[cite: 6] | 1.5s dwell lock & Conservation equation[cite: 5, 6] | *"Show me our DHU trend across the last 3 hours."* |

Updating `docs/PRD_ARD_MASTER.md` and `docs/mvp_epics_stories.md` with these definitions aligns the codebase to support automated floor operations as well as real-time conversational oversight for executive management[cite: 5, 6].






# Sizing the MVP BOM based on the 14-day thin thread pilot scope
# Pilot setup: 1 Cutting Table, Pilot Line 14 (6 Sewing Stations), 1 Store/Issue, 1 Finishing/Packing, 1 Supervisor
items = [
    # Hardware
    {"category": "Compute / Network", "item": "Host Server (Owner/Dev Laptop or Mini PC N100)", "qty": 1, "unit_usd": 0, "source": "Existing BYOD / Amazon"},
    {"category": "Compute / Network", "item": "Wi-Fi 6 Router (TP-Link Archer AX23 / AX1800)", "qty": 1, "unit_usd": 65, "source": "Amazon / Local IT Vendor"},
    {"category": "Operator Displays", "item": "Android Smartphones 6.5\" (Redmi / Realme / Moto Budget tier)", "qty": 6, "unit_usd": 75, "source": "Amazon / Croma / Flipkart"},
    {"category": "Station Tablets", "item": "10\" Android Tablets (Lenovo Tab M10 / Samsung A9 3GB/32GB)", "qty": 3, "unit_usd": 110, "source": "Amazon / Reliance Digital"},
    {"category": "Auto-ID Hardware", "item": "Handheld Bluetooth UHF RFID + 2D Sled (Chainway C72 / C5 or Hopeland)", "qty": 1, "unit_usd": 550, "source": "Chainway direct / Indiamart / AliExpress"},
    {"category": "Auto-ID Hardware", "item": "Bluetooth 2D Ring Scanners (Eyoyo / Netum)", "qty": 2, "unit_usd": 45, "source": "Amazon"},
    {"category": "Accessories", "item": "Gooseneck C-Clamp Machine Mounts", "qty": 9, "unit_usd": 12, "source": "Amazon"},
    {"category": "Accessories", "item": "Compact Bench Scale 30kg USB/Bluetooth", "qty": 1, "unit_usd": 50, "source": "Amazon / Indiamart"},
    # Consumables
    {"category": "Consumables", "item": "Recyclable NFC Hard Tags / Wristbands (NTAG213 / ICODE HF 13.56MHz)", "qty": 500, "unit_usd": 0.15, "source": "Indiamart / AliExpress"},
    {"category": "Consumables", "item": "Adhesive UHF RFID Roll Labels (EPC Gen2 Monza R6/Alien)", "qty": 200, "unit_usd": 0.06, "source": "Indiamart / Avery Dennison vendor"},
    {"category": "Consumables", "item": "Pre-printed Perforated Heavy Cardstock Bundle Cards", "qty": 1000, "unit_usd": 0.005, "source": "Local Print Shop"}
]

total_usd = sum(x["qty"] * x["unit_usd"] for x in items)
usd_to_inr = 83.0
total_inr = total_usd * usd_to_inr

print(f"Total MVP BOM Cost: ${total_usd:,.2f} USD (approx. Rs. {total_inr:,.0f} INR)")


Code output
Total MVP BOM Cost: $1,735.00 USD (approx. Rs. 144,005 INR)


Author: Sunil Jain, CyberinQ LLC

TL;DR
For the 14-Day Thin Thread MVP (targeting 1 Cutting Table, 6 Sewing Workstations on Line 14, 1 Store Issue checkpoint, 1 Finishing/Packing station, and 1 Supervisor console):

Total Hardware & Consumables Capex: $1,735 USD (approx. ₹1.44 Lakhs INR).


Procurement Channels: All items are off-the-shelf commodity consumer hardware and standard auto-ID inventory sourced via Amazon, IndiaMART, Croma/Reliance Digital, or direct OEM distributors (Chainway/Avery Dennison).


The Alpha Value Promise: "In 14 days on a single line, we will physically eliminate shade-mixing, eliminate paper bundle tickets, automate fabric roll issue-deductions in under 20 seconds, and mathematically prove where every cut garment goes before it leaves your dock—without touching your existing ERP or requiring an active internet connection."


Part I: MVP Bill of Materials (BOM), Sources & Cost Breakdown
The physical deployment map assigns minimal, rugged hardware to each station:

+─────────────────────────────────────────────────────────────────────────────────────────────+
|                                    MVP SHOP-FLOOR HARDWARE MAP                              |
|                                                                                             |
|   [Stores & Intake]            [Cutting Table]              [Line 14 Sewing (6 Ops)]        |
|   • 1 Handheld RFID Sled       • 1 10" Tablet (Stand)       • 6 Budget Android Phones       |
|   • Live UHF Roll Labels       • Desktop USB NFC Reader     • 6 Gooseneck C-Clamp Mounts    |
|                                • 500 Recycled NFC Tags      • NFC Wristband Logins          |
|                                                                                             |
|   [Finishing & Final QC]       [Carton Packing]             [Host Server & Admin]           |
|   • 1 10" Tablet               • 1 Bluetooth Ring Scanner   • 1 Dev Laptop (Docker Compose) |
|   • 1 Bluetooth Ring Scanner   • 1 30kg Bench Scale         • 1 Wi-Fi 6 Router (Air-Gapped) |
+─────────────────────────────────────────────────────────────────────────────────────────────+

Subsystem / Item
Exact Specification
Qty
Unit Cost (USD)
Total Cost (USD)
Approx. Cost (INR)
Where to Procure
Edge Server Host
Existing Laptop / Mini PC (Intel Core i5/i7 or Apple M-series, 16GB RAM) running Docker
1
$0 (BYOD)
$0
₹0
Existing development machine or owner laptop
Air-Gapped OT Router
TP-Link Archer AX23 / AX1800 Wi-Fi 6 Gigabit Router (WPA3-PSK)
1
$65
$65
₹5,400
Amazon / Local IT hardware distributor
Sewing Station HMIs
6.5" Android Smartphone (Redmi 13C / Realme Narzo / Moto G-series, 4GB/64GB)
6
$75
$450
₹37,350
Amazon / Croma / Reliance Digital / Flipkart
Workstation Tablets
10" Android Tablet (Lenovo Tab M10 Gen 3 / Samsung Galaxy Tab A9, Wi-Fi, 3GB/32GB)
3
$110
$330
₹27,400
Amazon / Local authorized electronics distributor
Handheld RFID Sled
Chainway C72 or C5 Android Mobile Computer (Integrated UHF RFID 9dBi + 2D Imager)
1
$550
$550
₹45,650
Chainway India / IndiaMART / AliExpress
Wearable 2D Scanners
Eyoyo / Netum 2D Bluetooth Ring Barcode Scanner (for Hangtag Swap & Packing)
2
$45
$90
₹7,470
Amazon / IndiaMART
Machine Mounts
Heavy-Duty Gooseneck C-Clamp Tablet & Phone Mounts (Vibration-resistant)
9
$12
$108
₹8,960
Amazon / Industrial hardware supply
Packing Scale
30kg Compact Digital Benchtop Scale with USB/Bluetooth serial output
1
$50
$50
₹4,150
Amazon / Local metrology scale vendor
Recyclable NFC Tags
NTAG213 / ICODE SLIX HF 13.56 MHz Encapsulated ABS Disk / Button Tags (for bundles)
500
$0.15
$75
₹6,225
IndiaMART (RFID card vendors) / Smartrac / HID
Adhesive UHF Roll Tags
Alien Higgs-3 / Impinj Monza R6 Printable UHF EPC Gen2 Adhesive Labels (for rolls)
200
$0.06
$12
₹1,000
Avery Dennison vendor / IndiaMART
Bundle Cardstock
Heavy 250 GSM Perforated Cardstock with pre-printed sequential integer layouts
1,000
$0.005
$5
₹415
Local commercial print shop
TOTAL INITIAL MVP CAPEX






$1,735
₹1,44,020
All-inclusive upfront capital expenditure

Part II: Procurement Playbook (Step-by-Step)
Consumer Electronics (Phones, Tablets, Router, Mounts):



Where: Amazon or local electronics retail distribution (Delhi/NCR Okhla, Nehru Place, or regional tech hub).


Lead Time: 24 to 48 hours.


Setup: Lock Android devices into Dedicated Kiosk Mode using Android Enterprise PIN or Fully Managed Device mode (pins the Chrome browser or Agiparel PWA to full screen, disables notifications, gestures, and settings access).


Handheld UHF Reader (Chainway C72 / Zebra RFD8500):



Where: Contact domestic Chainway or Zebra auto-ID distributors via IndiaMART or direct OEM sales reps in Delhi/Gurugram/Bangalore.


Lead Time: 3 to 5 business days.


Setup: Pair over Bluetooth or run the native Android data collection wedge pointing directly to 192.168.10.10:8080.


Tags & Auto-ID Consumables (NFC Buttons & UHF Labels):



Where: Order from specialized RFID converter suppliers (e.g., Bartronics, Syrma SGS, or local IndiaMART tag converters).


Lead Time: 3 to 7 business days.


Spec to request: "High-frequency 13.56 MHz ISO 14443A 25mm PVC/ABS round disc tags with center hole" for recyclable bundle attachment, and "UHF 860–960 MHz EPC Gen2 paper roll labels".


Part III: What You Can Concrete-Promise an Alpha Customer
When pitching factory owners (e.g., Vinod Kumar / Ram at Noble Garments), do not sell abstract software or theoretical AI. Promise concrete operational guarantees that directly protect their floor margins:

1. "Zero Shade-Mixing Guarantee on Line 14"
The Promise: "We will make cross-ply, cross-pant, and cross-cut panel marriages physically impossible on your pilot line."


The Mechanism: An operator cannot marry a pocket from Pant 1 with a front panel from Pant 2. The edge Directed Attachment DAG flags mismatches within 20ms and blocks piece-rate credit, eliminating fabric rejection chargebacks from buyers.


2. "Fabric Roll Issue & Verification in Under 20 Seconds"
The Promise: "Your cutting spreaders will never wait 10 minutes at the store counter for manual yardage measurement or paper gate passes."


The Mechanism: The store issuer and cutting spreader tap their NFC badges at the store window. The system validates reservations and updates the roll's Live RFID tag in under 20 seconds, eliminating unrecorded material slippage.


3. "Elimination of Lost Bundle Paper Tickets"
The Promise: "No more missing paper bundle tickets, no manual paper tally registers, and zero 'ghost tapping' of piece rates."


The Mechanism: Recyclable NFC tags map to bundles with a single tap. Workstation screens use minimum cycle dwell clocks ($40\%$ of Factory SAM), making it impossible for operators to game the system by rapid screen-tapping.


4. "100% Closed-Loop Mathematical Reconciled Order Closure (OCR)"
The Promise: "When Cut 1 is completed and packed, you will know the exact mathematical disposition of every single garment cut before the truck leaves your gate: Cut=Shipped+Lost+LeftOver."


The Mechanism: Packing stations verify exact ratio packs (e.g., Boxes 1–50) with scale weight bounds (250g). The system automatically generates a cryptographically signed, immutable Order Closure Report PDF signed by all six department leads.


5. "Zero IT Friction & Zero Internet Dependency"
The Promise: "We do not need access to your factory internet, and we do not touch your existing ERP. If your power stays on, our system runs at 100% capacity."


The Mechanism: The entire platform runs over an isolated, air-gapped Wi-Fi router on our own hardware. If the plant internet connection drops for 5 days, shop-floor production continues without missing a beat.


The Alpha Pilot Agreement (The 14-Day Wedge)
Offer this low-risk operational trial:

"Give us 1 cutting table and Line 14 for 14 days on a single cut of Style KK524867 (PO MK6574J9). We will supply the hardware and run side-by-side with your existing paper tallies. If we do not stop assembly errors, accelerate roll handoffs, and give you exact garment reconciliation by day 14, we pack up our kit and walk away at zero cost to you."

