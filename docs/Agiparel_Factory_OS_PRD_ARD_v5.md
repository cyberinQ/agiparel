# AGIPAREL FACTORY OS — PRODUCT REQUIREMENTS & ARCHITECTURE DESIGN v5.0

**Brownfield-Compatible · RFID-Mandatory · Founder-Governed · Evidence-Backed · Closed-Loop Factory OS**  
**Product/engineering owner:** Sunil Jain, CyberinQ LLC  
**Founder's source doctrine:** Rajesh Aggarwal, Agiparel Part 1–4 working PRDs and Part 5 Plant Floor/Quality PRD  
**Date:** 26 October 2026  
**Status:** Proposed consolidated engineering baseline; founder/plant/legal/IT/security acceptance pending  
**Supersession:** This v5 normative specification precedes the full retained v4 and inherited v3. v4/v3 details remain binding unless explicitly modified by v5. A specification conflict unresolved here is a controlled design hold, never an implicit override.

## 0. Executive product contract

Agiparel deploys alongside existing ERP, CAD, documents and operating habits while making RFID **mandatory** for graduating into authoritative production execution. Initial shadow ingestion need not stop plant operations, replace existing software or impose immediate write authority. RFID infrastructure qualification is a production gate, **not an optional permanent operating mode**. The platform ties approved buyer commitments to physical garments, evidence, operations, costs, decisions, recovery and shipment. It prevents invalid production credits and material transfers by policy, promotes no-blame intervention, and learns from measured interventions rather than merely collecting dashboards.

### 0.1 Normative language and traceability

`MUST/SHALL` is a requirement; `SHOULD` a recommended default requiring documented deviation; `MAY` optional. `P0` blocks production correctness or governance; `P1` required for full operational rollout; `P2` strategic extension. Source classes: **F** founder-locked source rule, **S** stated source requirement, **V4** inherited v4/v3, **D** new v5 design proposal (requires approval). Evidence stages: **VALIDATED** observed in signed pilot evidence; **DESIGNED** specified/illustrated but not demonstrated; **PROPOSED** not yet approved; **FUTURE** research or deferred. A screen mockup or prior PRD does not prove operational validation. Initial statuses are DESIGNED or PROPOSED unless a recorded acceptance report proves otherwise.

### 0.2 Binding reconciliation decisions (new in v5)

1. **RFID production mandate [F/user-confirmed]:** Operational production adoption requires commissioned RFID capture at prescribed receiving/store, cutting, sewing and finishing/control stations. Brownfield M0/M1 shadow/explain can use legacy observation feeds while rollout proceeds; RFID-dependent M2 experiment or M3/M4 authoritative credit requires station-level qualified coverage and traceable fallback/quarantine policies. A missing RFID read is not proof that work did not occur; it is incomplete evidence and triggers recovery rather than invented production credit.
2. **Founder CEO authority [F/user-confirmed]:** A consumption-overrun exception, prohibited reservation change, or founder-locked material override requires a directly authenticated CEO approval and written actual story. It cannot be delegated by ordinary RBAC or circumvented by scoped write-authority leases. Urgent supplier shortfall/rejection requires Top Management Council replan. Configure per-tenant identity and incumbent; the rule itself is immutable unless the founder formally changes policy in a new signed baseline.
3. **Physical bundle profile [F/user-confirmed]:** The Part 4 sub-sub-bundle method is an exact, testable **Classic Cut-Bundle Profile**. Data and event contracts MAY also support separately approved variable-quantity/split/merge profiles; no profile can relax conservation, part compatibility, genealogy or authorization.
4. **Evidence maturity [F/user-confirmed]:** Existing Part 5 layouts/contracts, v3/v4 proposed features, operational claims and validated deployments are separately labeled. Never label designed screens as “pilot validated” without screenshots plus signed test runs, plant and date.

### 0.3 Architecture boundaries retained from v4

Retain v4's legacy coexistence states M0–M4, authoritative aggregate leases, append-only provenance, planning snapshots and human questions, CAD-authoritative geometry, roll/lay optimization, UHF/HF hierarchy, variable physical membership, service/event topology, security, telemetry, replay, 29 proposed services and complete routed screen inventory. The named Gerber integration in v4 is the initial CAD adapter, **not** a vendor lock-in; CAD adapters must conform to the same validated marker interchange contract. No optimization service issues safety-critical machine commands. Physical transaction truth and accepted financial/accounting state remain independently governed.

## V5-A. Founder governance, material custody and irreversible decisions

### A1. Hard no-borrowing and scoped reservation state [F, P0]

Inventory must distinguish `physical_on_hand`, `quality_hold`, `available_unreserved`, `reserved_for_order`, `issued_to_process`, `work_in_process`, `returned_endbit`, `scrap`, `shipped/consumed` and `disputed`. Ownership (factory vs buyer) and buyer eligibility are separate dimensions. A reservation for PO A cannot be moved to PO B by urgency, manager preference, a bulk allocation optimizer or a legacy integration import. Reject unsupported cross-PO transfers atomically. In a shortage, freeze impacted allocations, calculate feasibility and emit an exception for council decision. In execution consumption variance, capture CEO-authorized delta before releasing blocked downstream inventory. Integration in M0/M1 records unauthorized legacy actions as discrepancy, not retroactively “approved” transactions.

### A2. Required exception and approval matrix [F, P0]

| Trigger | Hard gate | Decision owner | Evidence before unlock |
|---|---|---|---|
| Supplier under-shipment / fabric or trim quality rejection | Block plan-consumption of missing/rejected qty | Top Management Council (CEO, Sales, Sourcing, Production, IE, Finance; attendance/quorum requires founder confirmation) | Supplier GRN/inspection, shortage, alternate plans, council decision/version |
| Consumption greater than authorized issue, or fabric variance outside policy | Block affected release until review | CEO personally | Measurement and confidence, written **actual story**, signed timestamp, impacted PO/roll/cut, amount, revised ledger |
| Frozen cut plan materially altered beyond permitted tweaks, or roll re-reservation | Reject mutation | CEO for reservation exception; Planning for new approved plan | Before/after state, affected order, exact override authorization |
| Routine in-tolerance Cutting Master tweak | Permitted without unfreezing | Cutting Master | Reason and versioned delta; eligibility validation |
| Tech pack/buyer contractual change | Freeze impacted downstream publication | Authorized commercial/buyer approval chain | Signed revised buyer document and dependency re-evaluation |

Default denied. Approval carries actor's authenticated CEO identity, signing context, document hash, effective scope, prior/current revision and revocation history. Separation of duties applies wherever source policy permits, but never delegates CEO-specific decisions. Authorization alone cannot make a physically impossible allocation feasible.

### A3. Double-witness custody [F, P0]

Every **physical issuance or transfer** of fabric, sewing trims and packing trims shall bind: unique movement ID, immutable material identity, quantity+unit, from/to locations, issuer and recipient independent authenticated digital identities, scan evidence, device, signed acceptance times, reason, reservations and reversal references. The two actors may not be silently substituted with a single service account. A refused/missing recipient creates `PENDING_RECEIPT` rather than completed transfer. Controlled emergency/offline handling queues evidence for later independent attestation; it cannot post a final authoritative issuance without required witness validation. Retries are idempotent; reversal is compensating, never destructive.

### A4. Decisions and causality ledger [D, P1]

`DecisionRecord` fields: policy_version, hypothesis, observed facts/unknowns, feasible alternatives, cost/delivery/quality estimates with uncertainty, approver, signature, chosen action, counterfactual baseline, outcome window, actual effects, unresolved confounders. Distinguish causal attribution from mere temporal association; report “unknown/contested” rather than automatically blaming last operator. Tie to v4's existing experiment and issue models without creating a second authority ledger.

## V5-B. Commercial costing, quote promises, and margin realization

### B1. Commercial master [S, P1]

Buyer → buying house/agent → office → contacts, scoped payment terms (`LC/TT/OA/mixed`), Incoterms (`FOB/CIF/LDP/DDP` where contractually applicable), agent commissions, negotiated discounts, currency/exchange assumptions, freight/duty/payment financing, compliance requirements, nominated inspection/packing, claim history, buyer-constrained sourcing, realized margin and payment behavior. Date-effectivity and original sources mandatory. Buyer trust scores are advisory and cannot bypass contract or founder-locked gates.

### B2. Costing modes and configurable policy [S, P0]

Preserve Part 1's four distinct approved costing **model variants** as typed policies imported under their exact agreed names and methods during founder review; do not invent formula equivalence. Per model, separately expose raw materials, wastage allowances, cutting, making, finishing, compliance/testing, QA/rework, trims and packing, overhead and indirects, freight, duties, finance, commissions/agent fees and margin. Distinguish factory-owned FABRIC from buyer-owned CMT materials; never charge buyer-owned fabric to factory procurement. Support different quote currencies, rounding and the buyer-specific quote model. All assumptions are frozen in a versioned `CostModelRevision`.

### B3. QuoteAssumptionLedger [D/S, P0]

For each RFQ/quote: tech-pack/BOM/OB revision, Factory SAM with confidence, style category/complexity, marker yield source, anticipated line mix and achievable capacity, supplier pricing/ETA, skill and machine readiness, waste/rejection allowances, currency, quantity mix, shipment splits, base margin, margin floor, override authority and approval expiration. Model `Quoted → Negotiated → Booked → ProjectedRealized → FinalRealized`; explain deltas as quantity/mix, SAM drift, machine/skill gap, supplier/material variance, downtime, scrap/rework, freight/compliance, packing and commercial concessions. Trigger re-cost/approval when governing revisions change. Preserve immutable negotiation rounds and loss reasons. A quote requiring unavailable capabilities must be flagged or blocked according to approved policy, not silently treated as feasible.

### B4. Commercial acceptance [P0]

- Revised OB changes Factory SAM: approved quote unchanged in history; recomputed candidate exposes incremental cost/margin and affected delivery, with approval gate.
- Costing for buyer-supplied fabric excludes fabric purchase cash while retaining buyer-stock custody, loss/claim exposure and processing costs.
- Two negotiation rounds produce independently reproducible landed/FOB prices and margin waterfalls; tenant currency/rounding cannot silently rewrite historical quotes.

## V5-C. Planning, skills, machine capacity and time economics

### C1. Eight synchronized streams [S, P0]

Name and assign: Fabric T&A (Sourcing), Sewing Trims T&A (Sourcing), Cutting T&A (IE/Cutting), Sewing T&A (IE/Production, initial planning bottleneck), Finishing T&A (Finishing), **Packing Trims T&A** (Sourcing, independent of sewing trims), Machine Plan (IE/Maintenance), Skill Plan (IE/HR). Retain sample-development milestones as explicitly scoped adjacent stream if authorized. Backward schedule upstream from sewing readiness and forward schedule finishing/dispatch; compute joint dependencies, float/negative float, capacity contention and cascade on any event. Sewing is a default primary bottleneck **hypothesis**, re-evaluated from observed constraint evidence rather than assumed permanent physical truth.

### C2. Three immutable DPP tiers [S, P0]

`IE_Default_DPP`: approved SAM/capacity-derived baseline, revision and assumptions. `FactoryHead_Adjusted_DPP`: dated, reason-coded realistic signed commitment, daily cutoff, operator availability and shift assumptions. `Observed_DPP`: timestamped evidenced accepted output and stage-specific confidence. Compare IE vs adjusted (estimation optimism), adjusted vs actual (execution gap), IE vs actual (calibration). Preserve supersession and amendment history; never overwrite yesterday's published expectation with today's achieved numbers. Operational scan counts are not automatically accepted/quality-cleared garment counts.

### C3. Factory-level labor doctrine [F/S, P1]

Operators belong to the factory, with soft home-line/shift affiliations. IE proposes transfers using style, skill certification, proven operation performance, absences, transfer overhead, safety, labor/union/contract restrictions, compliance and donating-line impact; authorized factory leadership approves. Maintain per-worker transfer frequency, acclimatization, return plan, reassignment reasons and immutable assignment audit for buyer social compliance. Maximize plant-wide **accepted** contribution within deadline and labor guardrails, not local raw scan throughput. Do not use operator leaderboards for public punishment.

### C4. Forward procurement and external-capacity decisions [S, P1]

Line × machine × order × day × shift capacity is reserved with collision checks. Expose borrow from idle line (not borrowed materials), rental, purchase/capex, overtime, work method redesign and subcontracting alternatives with lead times, installation/commissioning, total landed cost and expected effect. Training/hiring work items start from predicted skill shortage and required qualification date. Govern subcontracting (qualified vendor, compliance, material custody, milestones, return inspection, commercial effect); recurring outsourcing for a style/IE model raises an estimation-calibration issue, not an automatic disciplinary fault.

### C5. Interprocess time as first-class evidence [D/V4, P1]

Use v4's event timestamps, event/processing-time distinction, confidence and WIP genealogy to measure queue distributions (`p50,p90,p99`), variance, transfer time, searching, setup, starvation and blocked time. Attribute work intervals to machine, labor, material, quality, routing or unknown only if supported by evidence. Model cost of delay and counterfactual throughput impact; do not double-count overlapping intervals or fabricate scan-based labor duration.

## V5-D. Stores, fabric allocation, and deterministic cut/bundle execution

### D1. Material identity and eligibility [S, P0]

Preserve Item Type + Color + Size/physical variant for fabric, sewing trims and packing trims; supplier catalog aliases resolve to one tenant factory item master, with original supplier SKU preserved. Buyer-supplied materials are legally and physically segregated. Eligibility is `spec × color/shade × measured usable width × quality PASS × age/condition × owner/buyer permission × reservation availability × traceable lot`. Functional equivalence does not authorize buyer cross-use. Preserve exact dimensions, GSM, weave/construction, shrinkage/nap, defect maps, roll segments and calibrated measurements; do not collapse nominal and usable width.

### D2. Surplus-first mandatory pre-PO gate [F/S, P0]

Before indent/PO issuance, search `AVAILABLE_UNRESERVED` and eligible returned surplus, excluding holds, pledged/contract-restricted stock, expired/degraded material and missing inspections. Emit evidence-backed match/rejection reasons and `net_procurement_requirement = max(0, required_including_approved_allowance − eligible_available_unreserved)`. Reserve selected surplus atomically or recheck on approval to prevent race. Exceptions require source-authorized decision; never silently double procure. Track opportunity cost, redeployment benefit and future order commitments.

### D3. Cutting Master acceptance and plan freeze [F, P0]

Optimizer proposes cut plan with source CAD marker revision and yield, roll/segment, lay, plies, table/slot, expected end-bits, size/part output and bounded solver diagnostics. Cutting Master may change **sequence, roll assignment (within exact shade/width/quality/reservation), plies (within material/machine/stack and coverage), table**. Marker geometry/dimensions, out-of-pool roll usage, changes to approved quantity/contract cannot be accepted as tweaks. Freeze plan after approval; only authorized Planning replan or CEO-governed material exception may reopen frozen inputs. Track each recommendation, tweak, reason and realized utilization, with acceptance rate interpreted jointly with economic outcome and operator engagement; never use tweak rate alone as personnel fault.

### D4. Classic Cut-Bundle Profile [F, P0]

After **actual** cut capture, derive garment serial for each `(cut, marker_position, actual_ply)` in the approved PO/style-color-size/lot policy. Under Classic Profile, each standard bundle contains `N` components of **one part type** and **one marker position** from `N` consecutive actual plies with compatible shade and physical part attributes. More generally: `bundle_count = Σ_(cut, marker_position, part_type) count(valid consecutive ply groups)`; formula `garments-per-marker × parts-per-garment × groups` applies only for uniform component counts and valid ply groups. Dynamic profiles must declare split/merge conservation and may not assert a uniform count. Configure target/min/max bundle quantity; balance last groups without crossing hard boundaries or manufacturing phantom pieces. `101 plies → [25,25,25,13,13]` is a valid arithmetic example only where group constraints allow. No bundle/serial/tag is issued for unverified planned-only plies.

**Dual identity:** human-visible PO-sequential bundle number `1…N` (scoped and immutable after issue) vs canonical IDs and detailed `cut, marker_position, part_type, ply_start/end, garment_serial_set, source_roll_segments, shade, route_revision, tag_binding`. A number is not globally unique without its PO/issue context.

**Commissioning:** generate/print physical cards after actual cut; operator selects physical bundle in an explicit ordered queue; scanner binds next verified reusable tag to exactly one active carrier, confirms read-back/reader health, and advances cursor only after an idempotently committed binding. Mismatch, duplicate/EPC reuse, unreadable tag, offline uncertainty or expired identity causes safe stop and exception. Reprints do not mint duplicates.

### D5. Assembly compatibility and replacement lineage [F/D, P0]

OB's `OperationExecutionContract` declares required part types, source cardinality, allowed combinations, prior work, scan mode and credible evidence. The approved `SubBundleAttachmentMap` determines compatible component pairing by garment serial or authorized set mapping, style revision, size, shade, nap/orientation, variant and route. Reject incompatible scans and refuse output credit. If physically preventing wrong assembly is safety/quality-critical, use fixture/poka-yoke or independently verified presence; software rejection alone does not guarantee physical prevention. Recut rejected panels from qualified remnant/surplus with a **new physical component identity** associated with the **same logical garment serial**, with explicit `replaces`/`superseded_by`, defect disposition and no double-consumption. Retain historical evidence; do not overwrite old component identity.

### D6. Per-cut conservation and release gate [F/S, P0]

Measure in a common unit and signed inventory sense: `Issued/Drawn = Used_in_accepted_or_rework_panels + Returned_endbits + Measured_scrap + Measured_remaining_in_process + Unexplained_variance` with roll-by-roll, cut-by-cut and order-level reconciliation. The pasted shortcut `Reserved − Consumed − EndbitReturned − Scrapped` is not a universal physical conservation equation; reserved is a commitment, not necessarily issued quantity. Define explicit tolerances by measuring method, calibrated uncertainty, fabric class and buyer rule; **±2% is illustrative, not validated threshold**. Block affected cut-bundle release when policy tolerance or identity/quantity checks fail. CEO directly signs qualifying consumption deviation with written actual story before authorized release; no retrospective silent adjustment.

### D7. CAD interchange profile [S/V4, P1]

Keep source CAD sovereign over marker geometry/cutter instructions. Vendor-neutral normalized `MarkerRevision` with hash, origin CAD, width, length, sizes, orientation, multiplicity, efficiency, source file and approved conversions. Build capability-negotiated adapters for approved DXF dialect, Gerber AccuMark and prioritized Optitex/Lectra; later Style3D/CLO/Browzwear subject to real test files. File extensions alone never prove interchange correctness. Require test vectors for units, notches/grain, contours, revision mapping, marker counts, size-set and simulated vs CAD-calculated yield. No fabricated marker efficiencies.

## V5-E. Operation Bulletin, SAM confidence and production-credit engine

### E1. One progressively enriched OB [S, P0]

Maintain one versioned Operation Bulletin aggregate, not three loosely linked spreadsheets. Costing stage (IE): operation master/reference, sequence/DAG, type/machine, **Factory SAM** and evidence class, scan Y/N, bundle/single mode, skill/quality requirements. Planning stage (IE/Production): line/shift, expected PPH, workstation count, approved alternative resources. Execution stage (Factory Head/authorized lead): operator assignment, authenticated operator ID, workstation, specific machine instance, shift lock and reason-coded reassignment. Fields carry effective intervals and approver; a downstream edit to route/SAM can force re-cost/replan/republish. A runtime assignment change does not mutate the historical costing baseline.

### E2. Factory SAM evidence contract [S/D, P0]

Distinguish `Standard/IE_Time`, `Measured_Factory_SAM`, `Quoted_SAM`, `Planned_SAM`, `Achieved_Runtime`. For measurement preserve operation/style/size/machine/operator cohort, observations, timestamp/censoring, breaks/wait/rework exclusion policy, confidence interval and bias. Original source thresholds (`<100`, `100–500`, `>500`) are configurable **illustrative sample bins**, not an automatic statistical guarantee. Sampling variation, operator learning and factory bottleneck context must be visible. The source's preference for factory empiricism does not justify replacing uncertain empirical measures with unjustified precision.

### E3. Typed OperationExecutionContract [S/D, P0]

Declare input identities and cardinalities, output identity/quantity, scan mode (`Single→Single`, `Bundle→Bundle`, `Bundle→Single`, plus approved additional profiles), quality prerequisite, allowed predecessors, compatible materials/components, expected part consumption, credited unit and cycle evidence. `production_credit` requires idempotency key, legal/physical identity matches, non-duplicate consumption, verified route/version and sufficient station evidence. Reject invalid credit atomically; quarantine unresolved scans and never infer output from tag presence alone. Explicitly model partial bundle completion, rework and cancelled work without overstating accepted garments.

### E4. Learning from expertise [S/D, P1]

Capture every recommendation, cutting/planning/quality human adjustment, rationale, prior metrics, outcome and confounders. Never train a policy solely to reproduce human choices or minimize tweak-rate: compare actual first-pass accepted throughput, margin, waste, latency, fatigue and operator burden. All optimizer updates versioned, reviewed, replayable and reversible. Experiments inherit v4 A/B rigor.

## V5-F. Finishing, packing, shipping and order closure

### F1. Internal RFID → buyer identity [S/V4, P0]

At approved finishing/hangtag station, record an atomic `HangtagBind` linking immutable logical garment ID to buyer-required serialized hangtag/barcode/RFID identifier (distinguish UPC/SKU from uniquely serialized identifiers). Unbind retired internal carrier only after verifying full mapping and transfer; return reusable tag through quarantine/inspection/clear before pool reissue. Customer hangtag errors route through restricted unbind/rebind with audit and revocation. A generic UPC is not proof of unique garment identity.

### F2. Typed finishing rework [S, P1]

Defect categories route: hangtag error → Hangtag Helper; pressing error → Ironing; sewing/construction defect → Alteration or qualified sewing repair; other damage → approved disposition. Every redo has issue ID, original context, assignment, clock, attempted fix, re-inspection, success/failure and work-cost. `RescueRate = accepted_on_reinspection / eligible_attempted_repairs` with stated population, time window and repeat-repair policy; do not imply independently checked pieces were all repaired. Separate first-pass yield and cumulative recovery.

### F3. Carton and manifest integrity [S/V4, P0]

Packing requires buyer PO-line/size/color/pack-ratio/serial rule, validated unique garment identity and count. Invalid/misscanned piece raises immediate station warning but **does not mutate carton membership**; log operator coaching separately from material defects. Reopen sealed carton into `REPACK_AUDIT`; record original member identity set, rescanned set and authorized replacement deltas, enforce exact membership constraints (or approved discrepancy disposition) before reseal. Never rely on an aggregate UPC count as unique serial membership. Loading starts from accepted manifest cartons, decrements on each distinct matched scanned carton, rejects duplicates and foreign manifest entries; final dispatch requires `remaining == 0` plus seal/carrier/document approvals. Offline stations retain append-only local events but cannot silently double-consume serials.

### F4. OCR immutable balanced certificate [S/V4, P0]

Present human equation `Cut = Shipped + Lost + Leftover` only when definitions are aligned (piece equivalent, buyer order boundary, full stage reconciliations). Compute typed stage balance separately for cut, sewing, finishing, packing, audited repack, dispatch, residual WIP, scrap, quality hold and approved transfers; no double counting. Require six source-specified departmental signoffs: Cutting, Sewing, Finishing, Pack & Ship, Quality, Plant Manager. Every signature binds immutable ledger snapshot ID, signature hash, variance explanations and approval identity. A late event opens an OCR amendment workflow, never silently edits a signed closure.

### F5. Issue and quality collaboration [S/V4, P1]

Shared `Issue` is first-class: detected_by, impacted entity/stage, severity, opened/observed time, response owner, watchers, responders, status, SLA, corrective action, verification and recurrence link. Support checker→line quality→mechanic/line supervisor→multi-line quality escalation. Quality risk callouts propagate with effective style/lot/window; audit plans cover inline/endline/final/buyer AQL and escalation of missed cadence. Charts must distinguish DHU (defects/100 inspected units), garment rejection rate, FPY, rework rate, rescue rate, audit sampling coverage and repeated checks; comparisons conditioned on style mix and denominator. UX policy: calm by default, one primary action, intervention-first, no public individual-output leaderboards. Requirements inherited from Part 5 screens remain DESIGNed until independently validated.

## V5-G. Mandatory RFID rollout, tag economics, localization and delivery assurance

### G1. Brownfield adoption compatibility gates [user-confirmed/V4, P0]

| Mode | Legacy coexistence | RFID requirement | Permitted authority |
|---|---|---|---|
| M0 Listen/shadow | Files, APIs, scanners, read replicas; no replacement | RFID site assessment and tagged-process design; may initially be installing | No production writes |
| M1 Explain/reconcile | Map legacy IDs and physical claims | Commission representative gates and evidence coverage measurement | Reconciliation/advice, no authoritative credit |
| M2 Controlled pilot | Pre-registered treatment/control; rollback | **Mandatory validated RFID coverage** on all pilot RFID-dependent handoffs; explicit exclusion areas | Limited pilot decisions per authorized scope |
| M3 Scoped production | Legacy still authoritative for undelegated aggregates | **Mandatory validated RFID** for each transferred execution transaction type | Per-aggregate, signed single-writer delegation |
| M4 System of record | Legacy data retained/exportable | Production RFID operational and audited, defined fail-safe/offline policy | Declared full scopes only |

Capture commissioning certification (reader/tag compatibility, read coverage, antenna cross-read, identity collision, environmental constraints, manual exceptions), confidence and station migration dependencies. Legacy report reconciliation may discover gaps but cannot substitute for mandatory operational RFID on mature production scope. Never allow production safety functions to depend on RFID inference.

### G2. Reusable tag circular inventory [D, P1]

Model lifecycle: `AVAILABLE → BOUND → IN_CIRCULATION → RETURNED → INSPECTION → AVAILABLE` with `LOST`, `FAILED`, `RETIRED`, `QUARANTINED`. Preserve durable physical tag EPC/TID (where stable), association history and versioned relabel events; hard block simultaneous binding. Plan pool quantity from throughput × observed turnaround + reserve and failure uncertainty. Original example 20k–70k/day, three–four rotation days and 2% loss alert is **a hypothesis to configure**, not an acceptance target. Monitor cycle-time p90/p99, attrition per 1,000 issues, unreturned age, false binding, reread/reuse rate, cost per successful cycle, stockouts and replenishment lead time.

### G3. Multi-locale and low-end devices [S, P1]

Retain full Unicode, native-script input, stable terminology and independent localization resources, and logical RTL layouts for Urdu/Arabic. Initial candidate languages derived from Part 1: English, Hindi, Bengali, Tamil, Marathi, Gujarati, Punjabi, Telugu, Kannada, Urdu, Vietnamese, Bahasa Indonesia, Spanish, Simplified Chinese; locale activation subject to signed linguistic and device testing. Support buyer-specific wording/units, dates, currencies and time zones; label/card print engines must prove barcode/RFID encoding + human-script fonts and legibility on actual printers. No machine translation for safety-critical machine/garment terms without reviewer approval. Define offline mode and low-end Android scanner responsiveness through measured station-level tests.

### G4. Measured production SLOs and security [V4/D, P0]

Inherit v4 numeric targets as **proposed**, not demonstrated. For every latency promise (e.g. misscan UI feedback example of 200 ms), define start/end measurement, p95/p99, offline behavior, client device, load and authoritative commit semantics. Tenant isolation; encryption/least privilege; secure CEO approvals; append-only evidence chain; chain-of-custody on source docs; tamper-evident auditable export. Privacy: skill and quality coaching data access restricted and retention/legal rules reviewed. Include labor compliance, reasonable accommodation and no punitive automated employment decision without governed review.

### G5. Pilot validation maturity and rollout [user-confirmed, P0]

Every capability/screen/event/solver decision declares `source`, `requirement_owner`, `maturity`, `acceptance_test`, `pilot_plant`, `tested_on`, `signed_by`, `evidence_uri`, `defect_status`, `rollout_eligibility`. A mockup/legacy PRD is DESIGNed, not VALIDATED. Prohibit promotion without independent physical tally/replay, recovery drill, operator usability test in target language, and signed finance/quality/plant acceptance as applicable. V4 acceptance tests are inherited; new tests are appended below.

## V5-H. Normative domain model, service mapping and events

**Avoid new deployables by default:** these are aggregates/contracts residing in inherited v4 services until scale or ownership measurements justify extraction. `DecisionRecord`, `MaterialMovementWitness`, `MaterialEligibilityResult`, `QuoteAssumptionLedger`, `DPPBaselineRevision`, `WorkforceTransferDecision`, `MachineGapOption`, `SubcontractJob`, `ClassicBundleProfile`, `BundleIssueCursor`, `AssemblyCompatibilityContract`, `OperationExecutionContract`, `FactorySAMObservation`, `ReworkCase`, `HangtagBind`, `RepackAudit`, `OCRCertificate`, `TagPoolCycle`, `PilotEvidenceRegister`.

**Canonical events (versioned, source/tenant/plant/idempotency/effective + observed timestamps):** `PolicyExceptionRaised`, `CEOApprovalSigned`, `CouncilReplanApproved`, `CustodyTransferProposed`, `CustodyTransferWitnessed`, `SurplusMatchApproved`, `QuoteAssumptionsFrozen`, `QuoteRecostRequired`, `DPPBaselinePublished`, `DPPCommitmentPublished`, `WorkforceTransferApproved`, `MachineGapDetected`, `CutPlanFrozen`, `ActualCutAccepted`, `ClassicBundleCreated`, `CarrierBound`, `BundleReleaseBlocked`, `PanelReplaced`, `OperationCreditAccepted/Rejected`, `HangtagBound`, `InternalCarrierReturned`, `RedoRouted`, `CartonMisscanRejected`, `CartonRepackStarted/Verified`, `ManifestCartonLoaded`, `OCRSigned/Amended`, `TagCycleClosed`, `PilotGatePassed/Failed`.

**Minimum command surfaces** (logical routes, service boundary per inherited v4 registry):

| Command | Hard invariant |
|---|---|
| `POST /v1/material-movements/{id}/witness` | Two distinct authenticated counterparties; immutable signed movement; idempotent |
| `POST /v1/decisions/{id}/ceo-sign` | Direct CEO authentication; actual story; no RBAC delegation |
| `POST /v1/procurement/indents/{id}/eligibility-check` | Available/unreserved six-dimension material eligibility; atomic recheck at commit |
| `POST /v1/quotes/{id}/freeze-assumptions` | Quote references immutable evidence and effective model |
| `POST /v1/dpp/{id}/adjust` | Factory Head authorized; reason and date; baseline retained |
| `POST /v1/cutting/cuts/{id}/accept-actual` | Actual plies, calibrated consumption and approved cut revision |
| `POST /v1/bundles/{id}/bind-carrier` | One active carrier binding; cursor read-back; preserved bundle profile |
| `POST /v1/operations/{id}/credit` | Typed operation valid; no double-consumption; correct compatible input |
| `POST /v1/garments/{id}/hangtag-bind` | Logical garment immutable; buyer uniqueness semantics; tag retirement custody |
| `POST /v1/cartons/{id}/repack-verify` | Reconciled membership and authorized deltas before reseal |
| `POST /v1/ocr/{id}/sign` | Snapshot hash fixed and source-specific signatory complete |

Responses must distinguish `accepted`, `rejected`, `pending_evidence`, `awaiting_ceo`, `awaiting_council`, `quarantined`; version conflicts 409, policy denials 403 or typed domain 422, transient offline retry explicit. No silent automatic corrections. Existing v4 route names may differ; implementation must maintain a canonical route registry and alias/deprecation map, not duplicate competing authoritative endpoints.

## V5-I. Acceptance tests — release blockers

| ID | Requirement | Given / When / Then evidence |
|---|---|---|
| V5-T01 | RFID mandatory + brownfield | Legacy ERP imports in M0 without replacement; M3 RFID-dependent cut/credit denied until commissioned station coverage signed |
| V5-T02 | No borrowing | Roll reserved PO A cannot move to PO B via UI, API, optimizer, backfill or retry; rejection audited |
| V5-T03 | CEO-only | Non-CEO admin cannot sign executive consumption exception; CEO signs actual story with independent auth and matching hash |
| V5-T04 | Council shortage | Supplier under-ships; impacted plan quarantined; approved multi-head replan creates new revision, no fabricated stock |
| V5-T05 | Double witness | Issuer scan alone does not complete release; distinct recipient validates; duplicate retransmission does not double issue |
| V5-T06 | Surplus-first PO | Eligible surplus reduces indent; wrong shade/age/buyer ownership/quality HOLD cannot qualify; concurrent reservation rechecked |
| V5-T07 | Quote versioning | OB SAM revision causes new candidate cost, preserves accepted historical quote and discloses margin drift |
| V5-T08 | Three DPP tiers | Factory Head reason-code edit changes commitment only; actual progress preserves baseline, edit and timestamps |
| V5-T09 | Workforce optimization | Suggested transfer includes donor-line lost output, transfer time and labor eligibility; unauthorized reassignment denied |
| V5-T10 | Frozen cut tweaks | In-tolerance sequence/table/plies/roll change succeeds; marker mutation or reserved-roll borrowing denied |
| V5-T11 | Actual-derived serials | Planned 100 plies, actual 97: issue exactly actual-eligible serials, no phantom identities |
| V5-T12 | Bundle balance | 101 plies under compatible layout group to 25/25/25/13/13; infeasible boundary reports explicit exception |
| V5-T13 | PO floor numbering | Human # unique within PO; canonical bundle IDs unique across plants; cut/ply/part lineage reconstructable |
| V5-T14 | RFID cursor | Unreadable/duplicate tag does not advance cursor; successful bind advances once, survives scanner retry |
| V5-T15 | Assembly mismatch | Different serial, size, shade, route or invalid predecessor rejected with zero credit; evidence retained |
| V5-T16 | Panel replacement | Recut physical panel supersedes rejected panel, links original logical garment, consumes remnant once |
| V5-T17 | Cut conservation | Reconcile issued/returned/scrap/WIP panel equivalents; failed tolerance blocks bundle release until CEO-authorized exception if applicable |
| V5-T18 | Typed operation credit | Bundle-to-single partial conversion yields exact permitted output, no duplicate aggregate or phantom credit |
| V5-T19 | Hangtag identity | Buyer serialized identity binds to original garment before internal reusable tag is made available for reissue |
| V5-T20 | Miscan | Wrong garment produces bounded UI warning and audit, carton membership unchanged |
| V5-T21 | Repack | Unseal for AQL then omit a garment: reseal blocked until original identity accounting or authorized disposition |
| V5-T22 | Manifest load | Duplicate/foreign carton rejected; decrement valid remaining by one, dispatch only at zero + approvals |
| V5-T23 | OCR six signers | Signing snapshot hash freezes stage balances; late correction yields amendment, not mutation of signed certificate |
| V5-T24 | Tag pool | Lost/unreadable tag quarantined, cannot rebind; pool forecast reacts to measured return latency |
| V5-T25 | Source maturity | Part 5 screen without signed pilot verification stays DESIGNed, never VALIDATED |
| V5-T26 | Multi-language | RTL, non-Latin input, printed card, scan-and-error text tested on target floor device and printer |
| V5-T27 | Real-world downtime | Reader/network loss creates bounded offline queue and uncertain evidence; no inferred production credit or dual writer |
| V5-T28 | CAD contract | Two CAD-origin markers with same declared width but different revision/geometry fail interchange consistency unless validated and reapproved |

**Pilot gate:** Each test records independent physical fixture, versioned source documents, pass/fail threshold, raw device logs, replay result, timing distribution and named signoff. The inherited v4 tests remain mandatory where compatible; provenance and meaningful denominator required for any “efficiency improved” claim.

## V5-J. Decision holds, dependencies and release sequencing

### Founder / plant signoff holds

- **H1:** Confirm exact commercial names/formulas for Part 1's four costing models, rate allocations and margin authority matrix directly against canonical Part 1 source and founder. No invented formulas.
- **H2:** Confirm council quorum, CEO deputy policy for true incapacity (currently **no delegation**), and legal/operational response to a time-critical shutdown while CEO unreachable.
- **H3:** Confirm double-witness policy for every class of movement, including internal warehouse relocations, stock counts, return-to-store and automated conveyors; retain strong gate for issuances while designing exception evidence.
- **H4:** Approve physical bundle minimum, max, shift-related exceptions, multiple-component-per-position and serial assignment for defective/replaced panels.
- **H5:** Confirm RFID type/read topology and secure enrollment at each plant's real machines; per-reader event credibility thresholds and fail-safe continuation of production.
- **H6:** Validate operational 200ms warnings, ±2% illustrative fabric reconciliation, SAM sample bins and tag-pool sizes with real plant measurements. Do not treat examples as signed SLOs.
- **H7:** Audit Part 5's mockups and “already designed” roles versus actual production deployment; attach verified evidence per screen/flow.

### Suggested staged implementation

**Phase 0 — Govern and instrument:** signed founder locks, source traceability, RFID survey, golden-order fixtures, legacy imports/read-only shadow, mature evidence register.  
**Phase 1 — Correctness spine:** identity, reservation/no-borrow, double witness, cut actual/bundle math, typed production credit, hangtag/carton/OCR conservation, CEO exceptions.  
**Phase 2 — End-to-end optimization:** costing margin checkpoints, DPP tiers, planning dependencies, workforce/machine decisions, surplus gate and CAD adapters.  
**Phase 3 — Continuous improvement:** human-tweak learning, tagged intervention outcomes, tag-pool forecast, economics of variability, experimental recommendations.  
Every rollout capability retains v4's per-aggregate single-writer ownership and explicit rollback.

### V5-to-source coverage register

| Requirement group | Source evidence | New v5 sections | Inherited anchors |
|---|---|---|---|
| Costing/commercial | Part 1 Costing working draft, 30 Apr 2026 | B1–B4 | V4-02; v3 §2, §11 |
| Planning and labor | Part 2 Planning working draft, 30 Apr 2026 | C1–C5 | V4-06; v3 §11–12 |
| Sourcing/material controls | Part 3 Sourcing & Stores working draft, 30 Apr 2026 | A1–A3, D1–D2 | V4-03–04; v3 §6–10 |
| Cutting and sub-bundle identity | Part 4 Cutting working draft, 30 Apr 2026 | D3–D7, E3 | V4-03–05; v3 §7, §10 |
| Quality/finishing/packing/OCR/UX | Part 5 Plant Floor Quality PRD | F1–F5 | v3 §5, §12–14 |
| Second-analysis suggestions | Uploaded additional-perspective text, 26 Oct 2026 | A–I as qualified | v4 evidence/architecture |
| Explicit final policy decisions | User decisions, 26 Oct 2026 | 0.2, G1/G5 | V4-01–V4-11 |

**Source integrity notice:** The Part 1–4 working drafts have synthetic/pending founder anecdotes. This specification preserves founder-locked text where indicated, and treats added operational algorithms as proposals until approved. The new perspective contains examples and interpretations, not independent proof of plant performance. V4 and v3 are reproduced in full below so the original technical and UI detail remains accessible without needing separate files.

---

# APPENDIX A — COMPLETE RETAINED V4.0 SPECIFICATION (INCLUDING ITS FULL V3 BASELINE)

**Reading rule:** For any contradiction with this appendix, V5 sections 0–J govern. Otherwise this appendix's original wording, tables, service interfaces, screen registers and engineering targets remain the inherited proposal. The original title/version is preserved for audit; it is not a competing specification.

# AGIPAREL FACTORY OS — PRD / ARD v4.0

**Brownfield, Evidence-Backed, Closed-Loop Factory Execution & Decision System**  
**Owner:** Sunil Jain, CyberinQ LLC  
**Date:** 25 October 2026  
**Status:** Proposed engineering baseline for business, plant, RFID, IE, software and safety review  
**Relationship to v3:** This v4 addendum takes precedence where it explicitly overrides v3. The complete v3 design baseline is retained later in this document as an inherited reference, not deleted. Targets and proposed service counts remain design hypotheses, not proof of shipped functionality.

## Executive decision

Agiparel shall enter a **running garment factory without demanding replacement of its ERP, Gerber/CAD marker tooling, floor paperwork, operator practice or existing scanners**. It shall first listen and reconcile; next explain and predict; then recommend experimentally; finally control chosen transactions only after observed superiority, authorization and safe rollback. It must understand each new buyer document and immediately compare the proposed job with *real, time-dated* materials, widths, qualities, skills, machines, line loading, financial constraints and supplier choices. The system must ask concise, targeted questions about uncertainties that actually change the plan.

**Product promise:** Agiparel knows what was approved; what the legacy system says; what was physically observed; what was committed; what work remains; where queue time, rework and variability arise; and which practical, authorized intervention is likely to improve accepted output, on-time delivery and contribution margin.

**Nonclaims:** RFID presence is not proof of work; NFC reads do not automatically measure labor effort; a tag does not contain the complete digital bill of materials; a high-quality optimizer cannot override the Gerber marker engine without its approved interchange format; A/B results require adjustment for style mix, learning, line differences and calendar effects.

## V4-01. Brownfield adoption: coexistence is a product feature

### V4-01.1 Progressive authority

| Adoption state | Agiparel writes | Existing legacy owner writes | Entry gate | Exit / rollback |
| --- | --- | --- | --- | --- |
| **M0 Shadow** | Append-only evidence, mappings, nonauthoritative projections | All production transactions | Read-only integrations, user consent, baseline data quality | Disable observer with no legacy effects |
| **M1 Explain** | Reconciliation findings, confidence, candidate schedules | All production transactions | Measured agreement and explainable exceptions | Stop recommendations; retain audited evidence |
| **M2 Advise / A-B** | Versioned suggestions, treatment assignments, approvals and results | All transactions except explicitly delegated pilots | Plant-approved study, baseline, guardrails, reversibility | Switch cohort to control; record outcomes |
| **M3 Delegated action** | Selected transaction aggregates only (e.g. one roll reservation or carton) | All undelegated aggregates | Per-aggregate owner transfer, double-write prevention, training, safety / security review | Signed hand-back and ledger reconciliation |
| **M4 System of record** | Full declared operational owner scopes | Historical, financial or untransferred domains | Audit-complete migration and production readiness | Tested controlled downgrade / export |

Transitions are **per capability, plant, line, customer, order and aggregate**; one factory may simultaneously be M0 for finance, M2 for cutting suggestions and M3 for a chosen packing line. A dual-write connector never creates two authoritative ledgers. Write authority is leased/declared for an explicitly scoped aggregate and effective interval; conflicting writers fail closed.

### V4-01.2 Legacy integration and provenance

- Connect by CSV/XLS, scheduled exports, SFTP, REST, read-only database replication, EDI, files and scanner/gateway adapters; use narrow supported mappings rather than intrusive database triggers.
- Normalize external IDs into `{source_system, object_type, external_id, canonical_id, effective_from, effective_to, mapping_revision}`. Preserve the unmodified source document/record, timestamp, checksum, importer and interpretation version.
- Model **claimed**, **observed**, **accepted**, **reconciled** as separate data states. Example: ERP claims 280 cut pieces; scale/lay scan suggests 276; Agiparel reports discrepancy but cannot silently change the ERP.
- Reconcile counters by exact identity, unit and reporting period; never merge yards into meters or SKU-level UPC into unique item identity.
- Import rejects/duplicates/conflicts queue for named humans; retry is idempotent; provide a documented export and full removal path so brownfield customers are never held hostage.

### V4-01.3 A/B experimentation

- Establish comparator per **same buyer/style complexity, fabric family, widths, machine mix, line loading, shift, operator skill and date**. Prefer randomized paired lays/roll allocations or staggered within-plant rollout where practical. Where not possible, explicitly report observational inference and confidence limits.
- Pre-register `hypothesis`, treatment, unit of assignment, planned sample, primary metric, guardrails and stop/rollback rule before displaying results. Never change definitions after seeing outcomes without recording a new version.
- Priority metrics: accepted first-pass pieces per paid hour; fabric usable yield and cost per accepted piece; WIP queue p90; on-time milestone achievement; rework/scrap; operator burden; downtime; implementation cost and incremental contribution margin.
- Stop treatment immediately for quality, safety, buyer compliance or labor-rule deterioration; no optimization software may issue a machine safety command. Missing observability is a measurement problem, not evidence of zero downtime.
- Clearly report `control`, `treatment`, `sample`, `period`, `effect size`, `uncertainty`, `confounders`, `source coverage`, and `decision`. Allow negative findings to be published. Track realization after the change, not only modeled promise.

## V4-02. Document-to-plan interactive intelligence

### V4-02.1 Ingestion and extraction

When a manager uploads a PO, tech pack, buyer design PDF, artwork, spec sheet, OB, fabric test, sample sheet, carton rule or amended order, the intake service shall: (1) virus/sandbox inspect; (2) hash, retain and classify originals; (3) extract text, tables, drawings and images with page/region citations; (4) detect document family, language, revision and buyer/style/PO identifiers; (5) propose structured facts and uncertainties; (6) create typed relationships; (7) diff against approved master and other documents; and (8) identify required decisions. Extraction output is **proposed** until governing owners approve it.

**Typed knowledge graph nodes:** Buyer, Contract, PO, PO Line, Style, Style Revision, Size, Color, SKU, Fabric Spec, Yarn/Thread Count (with interpretation/unit), Fabric Roll, Lot, Shade, Width Observation, Usable Width, Defect Map, BOM, Trim, Construction Step, Machine Type, Machine, Worker Skill, Route Node, Operation Bulletin Revision, SAM, Cut Plan, Marker Candidate, Lay, Ply, Bundle, SubBundle, Component, Garment, Rework, Buyer Label, Bag, Carton, Manifest, Shipment, Cost Model, Supplier and Evidence. **Edges:** requires, permitted_by, sourced_from, consumes, yields, fits_machine, available_at, reserved_for, precedes, assembled_into, packed_into, supersedes, conflicts_with, signed_by and observed_by. Every edge must declare its provenance, time scope and confidence; some are hypotheses, not facts.

### V4-02.2 Feasibility snapshot on upload

Immediately construct a versioned `PlanningSnapshot` joining document facts with material inventory (roll **usable width** not just nominal width; length, defects, shade/lot, GSM, composition, count/construction where known), reserved/available stock, purchase orders/ETAs, approved CAD marker recipes, cutting tables, CNC capabilities, ply restrictions, sewing operation/machine requirements, certified skills and absence forecast, active WIP, capacity calendars, maintenance state, QA/packing rules, due dates, operating cash/working-capital and capex policy. Each field includes `as_of`, originating system, confidence and owner.

Output three visible lists: **ready facts**, **blocking unknowns** and **nonblocking assumptions**; compute multiple feasible plans only for supported data, including a no-feasible-plan explanation. Forecast with intervals where capacity/lead times are uncertain. Never present a plausible invented number as a measured one.

### V4-02.3 Questions that change a decision

The AI shall ask one short, decision-relevant question at a time when possible, organized into an approval checklist for managers: *Is this the latest approved tech pack? Can this shade be mixed? Is partial carton shipment permitted? Are the 150 cm rolls reserved for another PO? May line 4 work an added shift?* Each question carries: observed conflict, documents/stock facts, options, impact on feasibility/cost/delivery and the role authorized to answer. Known answers are reused subject to date/revision; contradictory answers return to review. Free-form conversation never bypasses signed publication gates.

Proposed APIs: `POST /v1/intake/jobs`, `GET /v1/intake/jobs/{id}/facts`, `GET /v1/planning/feasibility/{job}`, `POST /v1/planning/questions/{id}/answer`, `POST /v1/planning/scenarios`, `POST /v1/planning/scenarios/{id}/approve`, `POST /v1/routes/{id}/publish`. All mutations are idempotent and audited; question answers cannot silently modify buyer-approved attributes.

### V4-02.4 Recommended human handoff

**Upload → facts reviewed → relationship/identity conflicts shown → live availability snapshot → exact unknowns asked → alternatives displayed → approvals → baseline plan → measured actuals → replan.** The manager should see the top five constraints and three decision options before any deep technical graph. A full audit view reveals every source and rejected interpretation.

## V4-03. Fabric and cutting: optimize allocation above Gerber, not the geometry inside it

Agiparel shall be the **global fabric allocation and cutting campaign optimizer**, while existing Gerber/CAD/CNC remains the authority for approved marker geometry, machine instructions and actual cutter control unless a documented integration passes validation. The fabric planner considers heterogeneous rolls and variable bundles, not one idealized width/lot.

### V4-03.1 Variable and constraint model

For roll `r`, keep `roll_id`, `supplier lot`, `material composition`, `construction/weave`, `yarn count / thread count` (typed separately), `GSM`, `usable_width_mm` (measured by length segment if varying), `nominal_width_mm`, `measured_remaining_length_mm`, `shade_band`, `color`, `shrinkage`, `orientation/nap`, `bow/skew`, `defect polygons/coordinates`, `material cost`, `reserved_qty`, `location`, `status`, `owner PO`, `reuse eligibility`, `calibration/uncertainty` and compatible cutter/spreader. Mill claims must be distinguished from measured facts.

Decision variables: assignment of rolls/segments to PO line and lay, selected width-compatible marker family, size-ratio and lay grouping, lay length and ply count (possibly constrained by fabric thickness, cutter limit and quality), table/window and start time, remnant retention and alternate order allocation. Different cut lays may contain different mixes; **do not assume uniform bundle/sub-bundle counts**.

Hard constraints: approved spec/shade/width, roll length and reservation, marker fit/approved file revision, directional grain/nap, defect exclusion, cutter table/knife height, maximum ply/stack thickness, style/size quantity coverage, roll-lot restrictions, available operators/machines and quality approval. Soft objectives: minimize usable fabric cost and waste, setup/changeovers, time-to-first-acceptable-kit, late shipment risk, unfinished remainders and carbon/energy where measured; maximize contribution margin and cut-table throughput without creating a sewing bottleneck.

### V4-03.2 Objective and solve strategy

Generate **candidate marker yields** by querying the CAD system with exact marker/fabric width/size ratio and revision; never invent geometry efficiency. Optimize upstream allocation with MILP/CP-SAT or heuristic + bounded exact improvement; record solver version, time limit, feasibility gap, objective weights and scenario IDs. Compare cost and makespan across at least three options: minimum fabric waste, earliest production readiness, and highest expected on-time contribution margin. Publish only an approved plan, with operator-readable lay instructions and return disposition for remnants.

**Critical math:** `available_length = measured_remaining_length - reserved_length - unusable_length`; `net usable yield = approved cut-piece area / physically consumed usable fabric area`, with consistent units and clearly declared denominator. `planned pcs ≠ actual cut pcs`; a completed lay is recorded from actual accepted outcomes, not marker estimate.

### V4-03.3 Cut actual, leftovers and shared rolls

One roll may feed multiple POs across time with atomic reservation slices/length intervals and recorded location transfers; one PO may use many compatible rolls of distinct widths. Every lay commits roll segments consumed, actual plies, cut counts by part/size, rejected sections and remnant identity in one owning transaction. Return remnants with new measured length/width and traceable source. If a cutter breaks or a roll is quarantined, recompute *remaining* uncut allocations while preserving completed lay records. No cutter geometry file is altered without an approved CAD/CNC contract.

## V4-04. UHF and NFC carrier hierarchy and physical truth

### V4-04.1 Identity hierarchy — required architecture

| Physical object | Preferred carrier / read mode | Authoritative object | Typical transition |
| --- | --- | --- | --- |
| Fabric roll / remnant | **UHF RFID** portal/handheld; optional printed fallback | `roll_id` + inventory ledger | receive → inspect → reserve → cut/return → retire |
| Cut **bundle** / grouped kits | **UHF RFID**, distant zone/handheld reads | `bundle_id` + genealogy / custody | create → stage → consume/merge → close |
| Cut **sub-bundle** / grouped components | **UHF RFID** for zone location and finding | `subbundle_id` + child composition | split → transport → assembly → close |
| Individual cut part | **HF/NFC** deliberate near-field tap where economically / ergonomically justified | `part_instance_id` + work/quality history | cut → assign → operate → join/reject/repair → retire |
| Finished garment | Buyer-specified unique identity / barcode / RFID | `garment_id`, buyer serial if required | finish → release → bag → dispatch |
| Bag/carton | Buyer RFID or unique carton barcode per policy | `bag_id` / `carton_id` | pack → audit → seal → manifest |

**Important technical qualifier:** UHF on a bundle indicates **presence/proximity/location with uncertainty**, not verified contents or sewing completion. NFC on a single part requires a deliberate tap and appropriate anti-collision design; it does not prove the correct work occurred, nor should each part be tagged by default where per-piece cost/handling exceeds proven benefit. An agreed pilot shall quantify tag-on-part feasibility, tag survival in steam/wash/press, throughput burden and false-read rates. For high-volume items, permitted alternatives include NFC on a reusable part fixture/tray plus unique part instance in system records, machine vision, or a tagged kit; these must not be misrepresented as individual-part physical IDs.

### V4-04.2 What is written to tags

The **tag holds a stable compact identifier and optional small, signed version/reference fields**, never the entire BOM, detailed fabric attributes, labor history or secret personal information. Typical UHF EPC: scheme-specific ID or encoded unique object key (e.g. SGTIN-96 for eligible permanent serialized items); optional user memory only if tag/reader and encoding scheme support it. NFC: signed or authenticated object token where practical, with UID anti-clone and source trust limits explicit. **Rich metadata resides in Agiparel's versioned database**, resolved on read even when disconnected using a scoped signed local cache. Rewriting immutable factory facts into tiny tags is unsafe, slow and often not supported by memory bank or reader.

Tag operations: `ProvisionTag → VerifyReadBack → BindObject → ObserveAtZone → CorrectBindingWithAudit → TransferCustody → Unbind → Inspect/Quarantine → EraseOrKillIfSupported → VerifyRelease → ReturnToPool / Destroy`. Reusable tag must not carry a buyer-required permanent serial into a second object's identity. Tag asset, logical object and binding history are different records. Reject simultaneous active bindings, unknown writers, invalid EPC format, duplicates and stale binding revision.

### V4-04.3 Finding and precision

Choose portal/overhead reader power, antenna pattern, multiplex plan, read windows and zone collision rules through empirical commissioning. Zone-level location is a probability/classification, not meter-precise indoor GPS; show `last_seen_at`, `zone`, `reader`, `confidence`, and `stale` status. Use short deliberate NFC at workstations for authoritative actor/part interaction, and role/equipment/QC evidence for credit. Provide a **Find this bundle** function that lists last known zone and confidence and requests a handheld scan where ambiguous.

## V4-05. Variable quantities, dynamic splits/merges and conservation

Bundles and sub-bundles do not have fixed piece counts. Represent `carrier` (container of part instances or measured interchangeable quantity), `carrier_membership` with effective intervals, part class/size/shade/lot/quality state, and immutable parent-child transformation events. An operation may consume 1 part from one sub-bundle and 3 from another, with distinct temporal identities. Physical quantities are integers for pieces; rolls are tracked in standardized length/area units with measurement tolerance. A batch operation records exact input and output quantities, scrap, hold and rework; rejected partial quantities are first-class outcomes.

`Input = accepted output + retained WIP + scrap + documented loss + approved adjustment` **per material/part type and transaction**. Count transformations must not use a global generic formula across dissimilar material units. A partial sub-bundle may be divided into many descendants; completed ancestors are nonconsumable; use transactional row/aggregate locks, idempotency keys and causal events to stop double spends. Track object-level genealogy for individually identified parts, and quantity-level cohort genealogy for undifferentiated pieces, with clear confidence downgrade when identity is not proven.

## V4-06. Time, variability, bottlenecks and resilient replan

Each accepted process transition shall carry `occurred_at`, `received_at`, monotonic elapsed source, clock uncertainty, `work_claim`, `operation_start/end`, `queue_entry/exit`, `pause`, `break`, `machine downtime`, `handoff`, `rework`, `quality hold`, `operator/station`, `source evidence` and revision IDs when observable. Treat intervals as **measured**, **inferred**, or **unknown**; never invent a precise sewing duration from two UHF sightings. Report queue / active / setup / blocked / wait-for-material / transport / rework separately, with p50/p90/p95 and sample counts. Critical path and WIP age are computed from the approved DAG plus current dependency/resource state, not just a visual Gantt.

Plant resilience planner shall detect no-show staff, skills gap, breakdown, missing roll, machine maintenance, power/network loss, failed QC, and changed due date. Replan remaining work with finite resources, preservation of committed physical history, approved changeovers and labor rules. It may propose substitute machine, certified relief worker, alternate eligible roll, reroute to an approved parallel node or overtime subject to approval. Always display **Plan A / Plan B / do nothing**, anticipated impact, uncertainty, required approvals, fallback and why a plan is infeasible.

Gantt visualization is a **versioned project plan**, with dependencies, actual progress, shift calendar, material ETA and critical-path changes; it is not a replacement for station event ledger. Surface a daily one-page plan for supervisors and an interactive exception list for line leaders.

## V4-07. Floor language and accessibility

All worker-facing primary flows shall aim for **grade 5 or simpler reading level in the site's working language**, with shop-floor-tested words and illustrated prompts. Supervisors, IE and finance may use deeper technical language on their own workspaces. Do not equate reading grade with user competence. Use small focused actions, familiar factory words, offline status, audio/haptic indication if appropriate, and role-based translation; every translation requires in-factory acceptance.

| Avoid on floor | Preferred UI phrase | Error/help microcopy |
| --- | --- | --- |
| Carrier lineage violation | Wrong bundle | “This bundle is for another size. Ask your line lead.” |
| Operation prerequisite unmet | Step not ready | “Do the last step first.” |
| Idempotent duplicate | Already counted | “Done. No need to tap again.” |
| Degraded sync / pending write | Saved here / Still sending | “Saved on this machine. Wait for green.” |
| Material lot incompatibility | Wrong cloth lot | “This cloth does not match this order.” |
| Aggregate conflict | Another worker changed it | “Scan again. The bundle list changed.” |
| Quarantined QC outcome | Hold for check | “Stop. Send to the checker.” |
| Approval required | Ask the boss / line lead | “Need OK before packing.” |
| Unknown zone | Not seen lately | “Last seen in Cutting. Please scan to find it.” |

Layout: show one decision per page, icon + <=12-word primary instruction where possible, consistent green/amber/red with text labels for color-blind use, large touch targets and gloves-compatible scans. If a worker has to type routinely, the workflow has not met the target. Worker-facing terms must be empirically tested with actual staff, not only readability scores.

## V4-08. Updated platform boundaries, data and APIs

Retain v3 **29 logical bounded contexts as the future target registry**, but revise the **29 independent backend deployments** requirement to an **optional deployment topology**. Pilot can use an edge-centric modular monolith with strongly isolated domain schemas and contracts; extraction to services is driven by scale, isolation and proven change cadence. The strict invariants are: one writer for each aggregate, documented API/event contracts, no hidden shared state, per-tenant access controls, offline plant authority, outbox/inbox dedupe, and independently recoverable device drivers.

**New or revised logical components** (may reside inside v3 service owners): LegacyBridge + SourceMapper (S28), EvidenceReconciler (S24), ExperimentManager (S26/S27), DocumentGraph + ClarificationQueue (S03/S04/S05), FeasibilityEngine (S08/S09), RollAllocationOptimizer + MarkerGateway (S13/S08), TagAssetRegistry and LifecycleController (S02/S11), CarrierGenealogy (S13/S14), FlowTimeEngine (S24/S25), RecoveryPlanner (S09/S26) and PlainLanguageUI (BFF/web).

### V4-08.1 Additional canonical entities

`adoption_scope`, `authority_lease`, `legacy_mapping`, `source_claim`, `evidence_observation`, `accepted_fact`, `reconciliation_case`, `experiment_design`, `experiment_cohort`, `experiment_result`, `document_assertion`, `knowledge_edge`, `clarification_question`, `decision_answer`, `planning_snapshot`, `fabric_segment`, `roll_reservation_slice`, `marker_candidate`, `cut_campaign`, `lay_actual`, `tag_asset`, `tag_binding`, `zone_sighting`, `subbundle`, `part_instance`, `carrier_membership`, `transformation_event`, `flow_interval`, `disruption`, `scenario`, `scenario_approval`, `intervention_outcome`.

Every planning scenario pins immutable references to document revisions, inventory snapshots, capacity calendar, machine capabilities, rates, assumptions and solver implementation. Every recommendation has an expiry and a required revalidate-on-commit step.

### V4-08.2 Critical API commands and invariants

| Command / query | Required validation |
| --- | --- |
| `POST /v1/legacy/mappings` | Duplicate cross-system identity rejected; tenant scope; versioned approval |
| `POST /v1/adoption/scopes/{id}/transition` | Authority gate, eligible scope and signed handoff |
| `POST /v1/experiments` | Comparator, guardrail, owner, treatment and stop plan |
| `POST /v1/intake/jobs` | Original retained; extraction cannot self-approve |
| `POST /v1/planning/questions/{id}/answer` | Role, revision, provenance and conflict audit |
| `POST /v1/cutting/campaigns/optimize` | Real marker candidates, roll measurements and reservations |
| `POST /v1/lays/{id}/commit-actual` | No overconsumption; exact accepted components; atomic changes |
| `POST /v1/tags/provision` | Namespace, encoding, readback and auth |
| `POST /v1/tags/{id}/bind` | One active binding / eligibility / revision |
| `POST /v1/tags/{id}/release` | Binding closed, object history committed, pool eligibility verified |
| `GET /v1/carriers/{id}/whereabouts` | Freshness, reader evidence, confidence; never assert exact location without evidence |
| `POST /v1/carriers/{id}/split` | Conservation, no duplicate parent use, quality and type correctness |
| `POST /v1/carriers/merge` | Route prerequisites, size/shade compatibility, exact parent consumption |
| `GET /v1/flow/{scope}/intervals` | Include measured/inferred/unknown, denominator and clock quality |
| `POST /v1/recovery/scenarios` | Capacity, resource, skill, safety and cost constraints |
| `POST /v1/interventions/{id}/approve` | Separation of duties, version match, owner and rollback |

## V4-09. Operational acceptance and pilot gates

| ID | Pilot acceptance condition | Applies to |
| --- | --- | --- |
| B01 | M0 installation leaves legacy writes untouched, fully reversible within a shift | Brownfield |
| B02 | At least two distinct legacy input formats map to canonical IDs with conflict/duplicate queue | Coexistence |
| B03 | Same line/PO can operate a mixed M0/M2/M3 capability scope with no dual writes | Authority |
| B04 | Treatment and control assignment, guardrail and negative result persist without cherry-picking | A/B |
| D01 | PO+tech pack intake produces provenance-linked candidate facts and identifies contradictions | AI intake |
| D02 | Feasibility uses timestamped on-hand/reserved rolls, machines, shifts and cost/capex inputs | Planning |
| D03 | Manager resolves blocking gaps interactively; answers are approved and versioned | AI questions |
| C01 | Planner compares heterogeneous roll widths, remnants, defects, plies and style constraints | Cutting |
| C02 | Optimizer uses CAD-provided marker candidates without writing unsafe CNC commands | Gerber integration |
| C03 | Two POs can consume separate reserved slices of one compatible roll without double use | Shared materials |
| C04 | Actual lay quantity differs from plan, and ledger retains actual, variance and leftovers | Cutting actual |
| R01 | UHF roll/bundle/sub-bundle tags can be registered, found by zone and retired correctly | RFID |
| R02 | NFC per-part pilot shows tag survival, cost and read ergonomics before universal rollout | RFID feasibility |
| R03 | Metadata remains complete in offline signed cache even if physical tag holds ID only | Tag data |
| R04 | Duplicate active tag binding blocked; old history remains queryable after reuse | Tag lifecycle |
| G01 | Variable counts split/merge through a DAG with no double spend and exact types | Genealogy |
| T01 | Queue and active time reported separately with uncertainty, outliers and p90 | Flow-time |
| T02 | Worker absence/machine breakdown replan preserves completed transactions | Resilience |
| T03 | Gantt shows constrained critical path and approved change history | Planning UX |
| U01 | Workers complete primary station flows in grade-5-level tested local language | UX |
| U02 | Offline/pending/denied/accepted are never conflated in station HMI | Trust |
| F01 | Every accepted physical, quality and shipping transaction is auditable and retry-safe | Core integrity |
| F02 | Published intervention result compares measured outcome against stated forecast | Closed loop |

### V4-09.1 Adoption-specific go/no-go

No M3 rollout without: documented authority lease for involved aggregates; reconciliation baseline; role/device auth; successful station training; safe offline behavior; demonstrated export/rollback; exception owner; physical count audit; and signed plant acceptance. No productivity headline without comparable control and measured operator burden. No tag-on-every-part purchase without confirmed cost, durability, operator throughput and incremental diagnostic value.

## V4-10. Developer implementation contract

Use the paired **Agiparel v4 Development Backlog** as the executable epic/story/task register. Every story should be deliverable as **individual changed files**, include API + migration + UI + integration/evidence behavior as applicable, and identify explicit `IMPLEMENTED / PARTIAL / STUB / ABSENT / UNKNOWN` status from source inspection. Unit tests are exit evidence but not a substitute for a completed workflow. Deliver vertical increments: running factory shadow baseline → interactive order feasibility → cutting campaign → mixed UHF/NFC genealogy → sewing/QC/packing → resilient optimization → A/B verified financial outcomes.

## V4-11. New decisions and open validation questions

1. Does every part warrant NFC, or only specific critical/defect-prone parts? Benchmark handling time, unit economics, survivability and ROI by SKU family; pilot alternatives explicitly.
2. Which physical chip memory/encoding class and reader protocols are installed? Do not assume every UHF tag supports writable user memory, authentication, or kill command.
3. Which Gerber products/versions and supported exports/APIs are installed (marker utilization, ratio, roll width, CAD files, cutter telemetry)? Integrate only approved vendor surfaces.
4. Who owns real roll measurement and defect maps? Define calibration, sample frequency and uncertainty.
5. How is legacy productivity measured independently of Agiparel? Identify A/B control line and protected sampling period.
6. What are buyer-specific allowances for shade mixing, partial cartons, remnant use, salvage and shipment? They must be approved documents, not AI assumptions.
7. Which workforce and material policies may be varied during recovery plans? Observe labor protections, safety and buyer constraints.
8. Which languages are used on floor, and what terms do workers themselves use? Approve vocabulary in real shifts.

---

# APPENDIX A — INHERITED V3 ENGINEERING BASELINE (FULL TEXT)

The following original v3 specification is included for continuity and traceability. Where it conflicts with V4-01 through V4-11, the v4 sections control; in particular, the deployment count is a *target logical decomposition*, and bundle/sub-bundle UHF plus individual-part NFC replaces the earlier assumption of HF/NFC-only bundle tagging. v3 remains informative for its full screen register, schema examples, sample orders, service scope and acceptance test catalogue.

# AGIPAREL FACTORY OS

A buildable, modular system of record and action for garment manufacturing: from buyer order and technical specification through material, cutting, sewing, quality, packing, shipment, financial performance and order closure.

**Design baseline** 29 independently deployable backend services; 4 audience-specific API/BFF applications; 1 small safety-reviewed local station kernel; 4 persistence technologies; 113 distinct role-oriented screen routes. These are proposed engineering counts, not figures claimed in the source PRDs.

**Boundary of evidence** Buyer PO 220988 and the supplied George trousers tech pack supply the commercial/product acceptance fixture. The separate IZOD shirt pack supplies a second style-family test. The legacy OB is a route-import test, not accepted construction authority until its identity/material conflicts are resolved.

Document purpose: allow architecture, product, hardware, backend, frontend, IE, QA and operations teams to implement and independently test every capability. All “shall” statements below are target requirements unless explicitly identified as source facts or proposed SLOs.

## How to read this specification

Read Chapters 1–3 for the operating model and governing documents, Chapters 4–6 for deployable services and the exact UI/data inventory, Chapters 7–10 for the graph, schemas, APIs and reader controls, and Chapters 11–16 for optimization, permissions, reliability and delivery. The screen IDs, service IDs and event names are stable contracts for the engineering backlog.

| Chapters | Primary question answered |

| --- | --- |

| 01–03  Product and document contracts | What business reality do we model, and which document controls each state change? |

| 04  Service topology | What is independently deployed; what happens locally versus in cloud? |

| 05  UX and screen register | Exactly which workers/managers/office teams need which interfaces? |

| 06–08  Storage, graphs, schema | Where is authoritative data stored; how are IDs, FKs, DAGs and genealogy represented? |

| 09–10  API and sensing | Which commands/events connect services and how do RFID readers prove observations? |

| 11–12  Optimization / governance | How do we replan for profit without bypassing buyer, quality or labor controls? |

| 13–14  Reliability / acceptance | How do we know the system is secure, correct and resilient? |

| 15–16  Engineering delivery / source | How do teams implement and verify the factory pilot? |

**Navigation** Word Navigation Pane lists the numbered headings. All screen IDs are unique and searchable; “Snn” identifiers locate independently deployable service owners. Original PRD IDs are preserved where defined.

## 1. Executive design: the system we are building

Agiparel is a factory operating system, not another collection of dashboards. It converts approved commercial and engineering documents into executable constraints; observes the changing physical factory through RFID and other evidence; commits material and product state atomically; and continuously recomputes realistic production, quality, delivery and contribution-margin outcomes. Operators get the next safe action, supervisors get the next helpful intervention, and executives get reconciled decisions with economics.

### 1.1 The single engineering story

1. Import and interpret the business contract: buyer PO, style, approved tech pack, engineering bulletin, packaging and shipping obligations. Every governed field is versioned with a source and approving authority.

1. Compile the approved product recipe: BOM and measurement spec plus a typed, versioned operation DAG with conditional branches, parallel subassemblies, joins and rework policies.

1. Create a feasible production commitment: materials, fabric yield, people/skills, machine slots, learning curves, QA coverage, cartons and outbound cutoffs constrain the plan.

1. Bind reality to identities: roll, component bundle, garment, reusable tag, buyer RFID, polybag, carton and shipment are separate objects joined through time-bounded relationships.

1. Execute locally: stations validate actor, tag, operation, material compatibility and previously completed work with signed cached rules; a local plant transaction commits accepted events even without cloud access.

1. Propagate once: durable events update locally owned transactional aggregates; versioned, freshness-labeled projections feed role-specific dashboards and analytics.

1. Replan responsibly: constraint solver compares alternate interventions; authorized people choose changes; the system measures whether projected benefit occurred.

1. Close every order: reconcile cut, work-in-progress, defects, losses, finish, bags, cartons and shipment; separately reconcile money through finance systems before final attestation.

### 1.2 Architecture decisions (binding for implementation)

- Independent deployment is real: each of the 29 backend services owns its schema and API contract; deploying/scaling one shall not require rebuilding another. The local station kernel is the exception because it must remain minimal, deterministic and operable offline.

- Operational authority is local at the plant for physical movements, route execution, inspection, packing and loading; cloud owns commercial/versioned planning, cross-plant analytics and financial coordination. Ownership of each aggregate is singular.

- Exactly one transaction writer per material ledger object, carrier lineage aggregate and carton; idempotent commands and an event outbox prevent replay from creating pieces or double credit.

- A tag read asserts proximity/identity, not completion, quality, or unspoofability. Rules combine deliberate scanning, time, role, machine telemetry and independent QC evidence where available. Evidence confidence is visible.

- Never use the AI copilot, a cloud timeout or a commercial exception to switch power to machinery. Interlocks must be designed and certified separately with machine/electrical safety specialists.

- No SaaS availability dependency is allowed in the minute-by-minute local plant; cloud read models may be stale, and staleness is explicit.

## 2. Factory business language and governing documents

The factory is governed by a controlled document graph. A PDF is not itself an instruction until its fields, applicability, revision, approvals and precedence have been resolved. An order item references a specific approved technical configuration, approved production route, sourcing allocation, packing plan, QA plan and shipping obligation. Changes become new versions effective at stated boundaries.

| Control family | Examples (document types) | Accountable owner | System effect |

| --- | --- | --- | --- |

| Commercial | RFQ / enquiry; quotation/cost sheet; sales confirmation; buyer PO; PO change order; contract and Incoterms | Sales, merchandising, finance | Order qty/price, accepted delivery window, chargebacks, buyer terms, currency |

| Product engineering | Buyer tech pack; approved sample and PP/size-set; BOM; graded POM; construction/label/care drawings; wash standard | PLM/IE, buyer approver | Approved fabric and trims, measurement tolerance, size branches, buyer labels |

| Execution recipe | Operation Bulletin (OB); operation master; machine master; route diagram/DAG; SAM/SMV sheet; line balance; digital work instructions | Industrial engineering | Directed precedence, joins/splits, tools, qualifications, versioned standards |

| Materials & inbound | Supplier PO; proforma; ASN; GRN; fabric inspection 4-point report; shade/lot report; inventory receipt; location transfer; issue note | Procurement, stores, incoming QA | Actual material quantity/quality, reservation and custody |

| Planning & cutting | T&A milestone plan; cut order; CAD marker; lay plan/lay slip; spreading record; cutting QC; recut authorization; bundle ticket | Planning, cutting master | Plies, marker, roll usage, actual pieces and bundle genealogy |

| Production & quality | Route traveler; line hourly plan; shift roster; attendance; maintenance job; Andon; inspection sheet; defect/NCR; rework disposition; AQL audit | Operations, quality, maintenance | Good output, provenance, defect status, machine/labor time |

| Finishing & packing | Buyer hangtag / UPC spec; pack plan and assortment; polybag RFID requirement; carton marking memo; carton serialization; pack audit report | Merchandising, finishing, pack manager | Required 2-piece bag, cartons, SKU validation, buyer labeling |

| Shipping | Shipping memo/instruction; packing list; shipping booking; dispatch release; load plan; shipping manifest; seal report; B/L or AWB; origin/customs certificate | Logistics, finance, buyer/3PL | Authorized packed quantity, carton-to-vehicle proof, custody and handoff |

| Finance & closure | Supplier bill; debit/credit memo; cost/variance policy; AR invoice; claims; stage balance sheet; signed order closure report (OCR) | Finance, merchandising, plant manager | Cost-to-complete, reconciliation, invoice basis and closure |

### 2.1 Document platform contract

- Each document carries document_id, document_type, issuer, counterparty, external_reference, scope (buyer/style/PO/plant), revision, effective_from/to, file checksum, source pages, extracted fields and confidence, approval status, signed approvers, attachment URI and supersedes_document_id.

- Incoming PDF/XLS/EDI extraction generates “proposed facts,” never silently changes approved master data. A field diff compares new and currently effective values. Human resolution is required for materially conflicting quantity, fabric, size, operation, label, packing or shipment data.

- Precedence is field-specific and authorized, not a simplistic universal hierarchy: buyer-approved PO/amendment controls ordered SKU quantities and order-specific carton instructions; approved buyer tech spec controls construction; approved IE OB controls how to execute those requirements; approved shipping memo controls booking/dispatch particulars. If sources conflict, hold dependent release and request the proper owner’s documented decision.

- Publishing a new document revision triggers dependency impact: which open orders, cut lots, in-flight work, buyer labels or cartons are affected? A new route version shall not silently change operations already in progress.

- A shipment-document packet is a first-class deliverable, with manifest, pack list, inspection/QA certificates if required, commercial invoice, buyer labels, and proofs attached to the same order/dispatch graph.

### 2.2 Source-backed reference example: Walmart/George style 220988

| Field | Verified benchmark from supplied sources | Implementation consequence |

| --- | --- | --- |

| Orders | REP1 / PO 4500458015: 40,000 units, $252,000 buyer-referenced amount; REP2 / PO 4500458017: 89,000, $560,700. Seven ordered waist sizes; color Black Soot. | Two separate purchase orders under one approved style, never one overwritten sales line. |

| Product | Pleated front cotton twill trousers, 100% cotton 3/1 twill, 265 gsm; the George tech pack includes graded POM and component sketches. | PO-style-to-spec revision binding and measurement/checkpoint tables. |

| Pack | 2 garments/polybag, one buyer RFID sticker per bag; size 30/32/40/42 = 24 pieces/carton, size 34/36/38 = 48; one size/color per carton. | Generate 64,500 two-piece polybags for 129,000 units; enforce carton slot arithmetic. |

| Counts | REP1 printed 1,096 cases; REP2 2,568. Size 42 leaves 16 pieces (REP1) and 8 (REP2) outside complete 24-piece cartons. | Do not infer permission to ship partial cartons. Buyer waiver/order correction must be explicitly recorded. |

| Operation controls | The separately supplied OB appears to refer to straight-fit polyester pants; supplied buyer PO/tech pack refer to cotton-twill pleated pants. | Route compiler must block “approved for 220988” until IE and merchandising resolve identity and fabric/fit mismatch. |

| Second family | The IZOD 45PW207 shirt pack includes wash, softener, pressing and hanger-based finishing. | Use solely as a separate branch-rich regression fixture; do not apply its packaging to 220988. |

### 2.3 Example pack arithmetic — a nonnegotiable acceptance test

| Size | Capacity | REP1 pieces | REP1 full + remainder | REP2 pieces | REP2 full + remainder |

| --- | --- | --- | --- | --- | --- |

| 30 | 24 | 1,080 | 45 + 0 | 2,160 | 90 + 0 |

| 32 | 24 | 2,520 | 105 + 0 | 5,040 | 210 + 0 |

| 34 | 48 | 6,864 | 143 + 0 | 13,680 | 285 + 0 |

| 36 | 48 | 11,520 | 240 + 0 | 23,040 | 480 + 0 |

| 38 | 48 | 9,024 | 188 + 0 | 18,000 | 375 + 0 |

| 40 | 24 | 5,040 | 210 + 0 | 10,080 | 420 + 0 |

| 42 | 24 | 3,952 | 164 + 16 | 17,000 | 708 + 8 |

| TOTAL |  | 40,000 | 1,095 + 16 pcs | 89,000 | 2,568 + 8 pcs |

An exactly filled REP1 plan uses 1,095 complete cartons plus 16 remaining units; a separately authorized partial carton would explain the printed 1,096 case total. REP2 uses 2,568 complete cartons plus 8 remaining units; its printed case count does not authorize a partial carton. Both cases require buyer disposition rather than automatic rounding or silent shipment of unallocated garments.

## 3. Complete factory lifecycle and module boundaries

| Stage | Owner services | Input / governed instruction | Committed outcome | Primary gate |

| --- | --- | --- | --- | --- |

| 0. Order intake | S03–S07 | RFQ, PO, approved buyer files | Approved order + commercial obligation | Hold mismatched revision/price/date |

| 1. Engineering release | S04–S05 | Tech pack, BOM, POM, OB | Frozen style route DAG, approved SAM, pack/QA obligations | No route release with material/fit conflict |

| 2. Material requirements | S08–S10 | Schedule + BOM + actual stock | Net indent / supplier PO / ETA | No double reservation or undocumented borrowing |

| 3. Receive / inspect / reserve | S11–S12 | Roll ASN/GRN, metrology and shade | Lot-aware roll balance and location | Quarantine false measurement/shade |

| 4. Cut & commission | S13 | Approved marker, lay, roll release | Numbered panel lots and bound reusable carrier tags | Quantities from physical cut, not planned lays |

| 5. Assemble / sew | S14–S17 | Approved route, stations, labor, QC | Good/rework/hold by carrier lineage | Reject incompatible size/shade/sequence |

| 6. Finish and identify | S18 | Verified garment + hangtag/label | Finished accepted piece with buyer identity | Reject UPC/hangtag mismatch |

| 7. Bag, pack and audit | S19, S17 | Buyer approved pack plan, AQL | Exact polybag/carton and lot-audit state | No seal with missing/bad scan or hold |

| 8. Stage/load/ship | S20 | Authorized release, load plan | Carton -> manifest -> seal -> dispatch | No unmanifested load |

| 9. Reconcile and account | S21–S22 | Stage ledger, losses, purchase costs | Signed OCR + cost-to-complete / invoice handoff | No false “balanced” without residual explanation |

| 10. Learn and optimize | S24–S29 | Time-series, schedules, skill/machine data | Versioned proposed actions and outcomes | No unapproved automatic physical reallocation |

## 4. Deployable architecture: what runs where

A service is an independently versioned, containerized deployable that owns a bounded transaction model and public API. A product module is a purchasable/installable capability composed of one or more services and screens. A role workspace is a presentation assembly and may legitimately consume many service APIs. These must not be confused.

### 4.1 Deployment topology

```
BUYER / ERP / PLM / HR / 3PL
       |   signed import / EDI / versioned APIs
       v
[CLOUD CONTROL PLANE — tenant aware]
  Sales/PLM/spec/OB | costing | procurement | APS/MRP
  Finance/closure   | optimization | analytics | multi-plant
  PostgreSQL domain DBs + ClickHouse + encrypted object store
  Cloud NATS streams + integration workers + BFF/API gateway
       ^  authenticated, replayable sync; NEVER floor-critical
       |
[PLANT SERVICES — factory intranet, on-site]
  stores/inspection | cutting | sewing | QC | finishing | pack
  maintenance      | workforce cache | dispatch | issue response
  Local PostgreSQL + durable NATS + signed policy/document cache
       ^ private LAN, store-and-forward station events
       |
[STATION EDGE KERNEL — small embedded service / industrial gateway]
  RFID/barcode/scale/encoder/motor sensor adapters
  offline SQLite outbox + tag debounce + cached rule validation
  LED/audio/large button HMI   (no dependency on cloud round trip)
```

### 4.2 Exact service inventory: 29 independent backend deployments

The pilot may install only the necessary product modules. The following 29 are target bounded-context deployables; 4 separately scalable BFF applications serve floor, supervisor, office and executive UIs. Stream brokers, databases and telemetry exporters are infrastructure, not counted as product microservices. The edge kernel is one additional compact runtime per site/station tier, not a distributed service mesh.

| Service / bounded context | Execution locality | Authoritative data / emitted result | Independent scaling unit |

| --- | --- | --- | --- |

| S01  Identity, tenant and access | Cloud + edge cache | users/roles/permissions; AccessRevoked | Read-heavy; cache by tenant; offline scoped grants |

| S02  Device registry and trust | Cloud + plant | device, reader, policy; DeviceEnrolled | by plant/device; firmware rollout independent |

| S03  Document intake / approvals | Cloud | document, revision, extraction, decision; DocumentApproved | async OCR/parser queue per document |

| S04  Product, SKU, BOM / spec | Cloud + signed local read | style, SKU, BOM, POM; StyleRevisionApproved | read replicas and tenant partitioning |

| S05  OB and route compiler | Cloud + signed local read | route_revision, node, edge, gate; RoutePublished | compute build jobs independent of floor |

| S06  Sales orders / amendments | Cloud | buyer_order, line, terms; OrderAccepted | business transaction replicas by tenant |

| S07  Quotation and standard costing | Cloud | quote, rate_card, costing_version; QuoteApproved | cost model jobs by quote/style |

| S08  Requirements / MRP | Cloud + plant feed | net material demand, pegging; MaterialNeedChanged | partition by plan version/PO |

| S09  Capacity plan / T&A scheduler | Cloud + plant read | capacity, calendar, schedule; PlanPublished | async solver jobs; no floor critical path |

| S10  Procurement / supplier ETA | Cloud | supplier_po, ASN, supplier_rule; InboundDelayed | per supplier/order; external adaptors |

| Service / bounded context | Execution locality | Authoritative data / emitted result | Independent scaling unit |

| --- | --- | --- | --- |

| S11  Inventory and material ledger | Plant owner + cloud projection | stock_movement, reservation, location; MaterialIssued | partition by warehouse/site/roll |

| S12  Fabric inspection / metrology | Plant | inspection, 2D defect, shade, calibration; RollAccepted | ingest by inspection lane |

| S13  Cutting and bundle commissioning | Plant | lay, marker, cut_piece, component_bundle; BundleCreated | partition by cutting table/lay |

| S14  Sewing execution / route state | Plant | work_claim, station_cycle, carrier_transition; OpCredited | partition by carrier aggregate or station |

| S15  Workforce / skill / time | Cloud + plant cache | attendance, skill, allocation; WorkerAllocated | bounded plant and shift partitions |

| S16  Maintenance and machine assets | Plant + cloud projection | asset, job, fault; MachineRestored | by plant/machine; avoid sensor raw stream |

| S17  Quality / NCR / AQL | Plant | inspection, finding, disposition, audit; QualityReleased | by unit/lot/audit; policy server-side |

| S18  Finishing / buyer identity | Plant | finish_cycle, buyer_identifier; GarmentReleased | partition by garment/finish station |

| S19  Packing and cartonization | Plant | bag, carton, pack_scan, seal; CartonSealed | single writer per carton; many pack lanes |

| S20  Logistics / dispatch | Plant + cloud sync | manifest, dock, load_scan; ManifestDispatched | by dispatch/load ID |

| Service / bounded context | Execution locality | Authoritative data / emitted result | Independent scaling unit |

| --- | --- | --- | --- |

| S21  Order closure and attestations | Cloud using reconciled ledgers | closure, variance, signoff; OrderClosed | async by order; six-stage evidence |

| S22  Actual cost / finance handoff | Cloud | cost_posting, finance_mapping, invoice_ref; CostPosted | by PO/period; ERP connector boundary |

| S23  Issues, notifications, responses | Plant + cloud projection | issue, owner, watcher, escalation; IssueResolved | partition by plant; fanout scales by channel |

| S24  Realtime projections / query BFF | Plant + cloud | read-only materialized views; ProjectionRefreshed | replicas by dashboard audience |

| S25  Metric definitions / benchmarking | Cloud analytics | metric_registry, cohort, metric_result; MetricsComputed | compute by plant/period/style in ClickHouse |

| S26  Optimizer and digital twin | Cloud; local lightweight guidance | scenario, constraints, recommendation; InterventionProposed | isolated async solver pools; manual approval |

| S27  Agie evidence copilot (optional) | Cloud | query audit, tool calls, grounded answer; InsightPrepared | quota/rate limit; read-only by default |

| S28  Enterprise integrations / EDI | Cloud + plant agents | external IDs, mapping, sync job; ExternalSynced | one connector worker per endpoint/feed |

| S29  Multi-plant orchestration (optional) | Cloud | capacity_offer, transfer_request; AllocationApproved | federated tenant/plant partitions |

### 4.3 Separation of independent UI and API applications

| Application | Audience | What it aggregates | Scale lever |

| --- | --- | --- | --- |

| floor-bff | Handheld HMI / tablets | Short command payloads, station state, offline feedback | By station session / line |

| supervisor-bff | Line, stores, cutting, finishing, maintenance | Line queues, action dispatch, issues, risk read models | By plant audience |

| office-bff | IE, costing, planning, sales, finance and procurement | Document editor, order grid, planning simulation and approval workflows | By tenant/read workload |

| executive-bff | Plant manager, multi-line quality, executive | Cross-line/plant snapshots and evidence-backed drilldowns | By analytical query/freshness tier |

### 4.4 Local execution and control authority

- Station kernel checks signed route revision, badge authorization, permitted operation, tag binding, quantity, prior prerequisites, and configurable evidence policy. For multi-carrier merges and global quantity transactions, the locally deployed route service is authoritative; no offline station may independently assume a merge committed.

- Durable bus: NATS JetStream on the plant LAN and cloud, with per-aggregate subject partitioning and transactional outboxes in owning PostgreSQL services. At-least-once delivery plus deduplication/idempotent consumers; never promise exactly-once across the network.

- Commands that alter stock, bundle genealogy, inspection disposition or cartons are accepted only after an ACID local transaction; the UI distinguishes pending, committed and synchronized. No open-loop “scan means credited.”

- Signed cached rules enable plant operation without WAN; previously unseen customer amendments or expired authorization grants require explicit safe failure or exception workflow, not acceptance from stale cloud data.

- Independent module deployment uses semantic-versioned APIs/events, feature entitlements, documented adapters for absent peers and a consumer-driven contract test suite. Optional modules never force core services to import their code or tables.

### 4.5 Installable module profiles and disabled-peer behavior

| Install profile | Services required | Independent value when other modules absent |

| --- | --- | --- |

| Starter + controlled documents | S01–S06, S28, BFF office | Approved buyer PO/style/OB/versioned documents; no pretend physical WIP |

| Stores and cutting | S01–S05, S11–S13, S28, floor/supervisor BFF | Receipt -> inspect -> issue -> lay -> bundles; can export cut record to incumbent MES |

| Sewing and line QC | S01–S05, S14–S17, S23–S24, floor/supervisor BFF | Operate from approved imported OB + cut bundle adapter, without requiring cloud optimization |

| Finish, pack and dock | S01–S04, S17–S20, S23–S24, floor/supervisor BFF | Accept approved inbound garments from third-party MES, pack/audit/load with own integrity |

| Full factory end-to-end | S01–S24 + S28 and 4 BFFs | Conserved material-to-shipment traceability, live dashboards and closure |

| Enterprise intelligence add-on | S25–S27; optionally S29 | Analytics, optimization, explainable guidance and interplant planning; read-only by default |

Each profile must ship with explicit data-import adapters and module-availability queries. A customer buying only Stores shall not have to deploy payroll, AI, packing or a cloud scheduling engine; a full-site module connects through the same documented APIs. Independently scaled replica count does not change ownership of any material/bundle/carton aggregate.

## 5. User experiences and exact screen inventory

A “screen” means one independently navigable, testable route with a primary action and access policy—not every card, chart, filter or modal. Station actions are not called dashboards. The operational PRD already specified PM-01–05, QS-01–05, QM-01–05, SEW-01–05, FH-00–05, PK-00–05, OC-01–02 and other named screens. We preserve those identifiers; the registry below completes the broader business and platform experience. The total is the new proposed product scope, not a claim that source documents already contained this many.

| UI category | Distinct routes | Design priority | Primary device |

| --- | --- | --- | --- |

| Shop-floor worker | 23 | Tier A: pilot | Phone/tablet |

| Line supervisor | 13 | Tier A: pilot | Tablet / desktop |

| Department supervisor | 21 | Tier A: pilot | Tablet / desktop |

| Factory manager | 18 | Tier A: pilot | Desktop |

| Commercial office | 21 | Tier A: pilot | Desktop |

| Governance / platform | 17 | Tier A: pilot + later optional | Desktop |

| Interaction class | Count | Definition |

| --- | --- | --- |

| Station | 23 | deliberate scan/tap workflow |

| Dashboard | 41 | live decision surface for an operational owner |

| Workbench | 38 | creation, configuration, reconciliation and approval |

| Executive | 5 | multi-domain management decision |

| Administration | 6 | security, devices and service operations |

### 5.1 Role-by-role count; role = default owner, not exclusive permission

| Organizational tier | Persona / default owner | Routes | Examples / governing capability |

| --- | --- | --- | --- |

| Shop-floor worker | Sewing operator | 3 | SEW-01, SEW-06, SEW-07 |

| Shop-floor worker | Material handler / feeder | 2 | MAT-01, MAT-02 |

| Shop-floor worker | Fabric receiving inspector | 2 | STR-02, STR-07 |

| Shop-floor worker | Stores issue clerk | 2 | STR-04, STR-08 |

| Shop-floor worker | Cutting team | 3 | CUT-02, CUT-03, CUT-04 |

| Shop-floor worker | In-line checker | 1 | SEW-02 |

| Shop-floor worker | Mechanic | 1 | SEW-04 |

| Shop-floor worker | Finishing team | 4 | FH-01, FH-02, FH-03, FH-04 |

| Shop-floor worker | Packing team | 3 | PK-02, PK-03, PK-04 |

| Shop-floor worker | Warehouse ship/stage operator | 2 | LOG-01, LOG-02 |

| Line supervisor | Line supervisor | 5 | SEW-03, LIN-01, LIN-02, LIN-03, LIN-04 |

| Line supervisor | Line quality supervisor | 5 | QS-01, QS-02, QS-03, QS-04, QS-05 |

| Line supervisor | Chief mechanic | 3 | SEW-05, MNT-01, MNT-02 |

| Department supervisor | Finishing supervisor | 3 | FH-00, FH-05, FNS-01 |

| Department supervisor | Packing & shipping supervisor | 3 | PK-00, PK-05, LOG-03 |

| Department supervisor | Cutting supervisor | 4 | CUT-01, CUT-05, CUT-06, CUT-07 |

| Department supervisor | Stores manager | 5 | STR-03, STR-05, STR-06, STO-01, STO-02 |

| Department supervisor | Multi-line quality manager | 6 | QM-01, QM-02, QM-03, QM-04, QM-05, QLT-01 |

| Factory manager | Plant manager | 7 | PM-01, PM-02, PM-03, PM-04, PM-05, PM-06, PM-07 |

| Factory manager | IE / production manager | 5 | OB-01, OB-02, IE-01, IE-02, IE-03 |

| Factory manager | Production planner | 6 | PLN-01, PLN-02, PLN-03, PLN-04, PLN-05, PLN-06 |

| Commercial office | Sales / account manager | 5 | CST-02, SAL-01, SAL-02, SAL-03, SAL-04 |

| Commercial office | Merchandiser | 5 | PK-01, MER-01, MER-02, MER-03, MER-04 |

| Commercial office | Costing / finance analyst | 7 | CST-01, FIN-01, FIN-02, FIN-03, FIN-04, FIN-05, FIN-06 |

| Commercial office | Procurement head | 4 | STR-01, SRC-01, SRC-02, SRC-03 |

| Governance / platform | Leadership / CFO / COO | 5 | EXEC-01, EXEC-02, EXE-03, EXE-04, EXE-05 |

| Governance / platform | Order closure auditor | 3 | OC-01, OC-02, OC-03 |

| Governance / platform | Systems / IT administrator | 6 | ADM-01, ADM-02, ADM-03, ADM-04, ADM-05, ADM-06 |

| Governance / platform | Optimization / data scientist | 3 | OPT-01, OPT-02, OPT-03 |

### 5.2 Complete routable screen register

This is the canonical UX backlog and permission inventory. Every ID gets a route URL, acceptance test, role and API dependency in the engineering repository. Multiple roles may share one ID; do not duplicate its implementation or count.

#### Shop-floor worker

| ID | Default role | Screen / route | Primary action and required data |

| --- | --- | --- | --- |

| SEW-01 | Sewing operator | Operation station HMI | IN / OUT / HELP; route, WIP, visual work instruction |

| SEW-06 | Sewing operator | My next bundle | Prioritized available work; deliberate claim |

| SEW-07 | Sewing operator | Exception or rework receipt | Accept rework and inspect route prerequisites |

| MAT-01 | Material handler / feeder | Pickup & deliver | Scan origin/destination; separate travel and wait |

| MAT-02 | Material handler / feeder | Feeder dispatch queue | Deliver ready-to-move kits; acknowledge blockers |

| STR-02 | Fabric receiving inspector | Gate intake and fabric inspection | GRN, roll measured length/usable width/defects |

| STR-07 | Fabric receiving inspector | Shade and roll grading | Record shade band, inspect result, photo evidence |

| STR-04 | Stores issue clerk | Double-witness material issue | Verify location, roll and authenticated receiver |

| STR-08 | Stores issue clerk | Putaway / return scan | Locate, return end bits, quarantine incorrect receipts |

| CUT-02 | Cutting team | Spreading and cutting execution | Actual plies, marker, measured consumption |

| CUT-03 | Cutting team | Post-cut numbering | Assign IDs based on physical quantity |

| CUT-04 | Cutting team | Bundle commissioning | Bind physical reusable tag to typed component bundle |

| SEW-02 | In-line checker | Inline QC | PASS/REJECT and defective unit lineage |

| SEW-04 | Mechanic | Mechanic ticket terminal | Accept repair, parts, evidence, resolved/test |

| FH-01 | Finishing team | Hangtag helper | Match buyer identity with product instance |

| FH-02 | Finishing team | Ironing station | Start/complete press; inspection feedback |

| FH-03 | Finishing team | Final checker | PASS or type-specific redo after inspection |

| FH-04 | Finishing team | Alteration station | Repair, complete and route back to QC |

| PK-02 | Packing team | Carton packing station | Two pieces/bag; count and verify all carton slots |

| PK-03 | Packing team | Final AQL audit | Sample, inspect, record lot verdict |

| PK-04 | Packing team | Dock loader | Verify cartons against approved manifest |

| LOG-01 | Warehouse ship/stage operator | Putaway / staging scan | Move sealed carton to verifiable staging bin |

| LOG-02 | Warehouse ship/stage operator | Repack exception station | Reopen authorized carton; audit and reseal |

#### Line supervisor

| ID | Default role | Screen / route | Primary action and required data |

| --- | --- | --- | --- |

| SEW-03 | Line supervisor | Line execution console | Hourly targets, good units, WIP and actions |

| LIN-01 | Line supervisor | Operation bottleneck map | Queues, cycle distributions, and constrained stations |

| LIN-02 | Line supervisor | Labor/float assignment | Assign skilled floating operators with approval |

| LIN-03 | Line supervisor | Style switch readiness | Pre-production approval and route activation |

| LIN-04 | Line supervisor | Shift handover | Unresolved issues, WIP, machine and quality state |

| QS-01 | Line quality supervisor | Line quality dashboard | DHU and first pass per inspection denominator |

| QS-02 | Line quality supervisor | Live defect feed | Review defect events with source/repair owner |

| QS-03 | Line quality supervisor | Defect heatmap | Dispatch support by operation/station |

| QS-04 | Line quality supervisor | Repair queue | Track repair aging, priorities, verification |

| QS-05 | Line quality supervisor | Audits & style setup | Audit calendar, risk callouts, sampling policy |

| SEW-05 | Chief mechanic | Chief mechanic dispatch | Availability and technician assignment |

| MNT-01 | Chief mechanic | Preventive maintenance plan | Schedule, defer with reason and record downtime |

| MNT-02 | Chief mechanic | Assets, parts and diagnostics | Spare stock, fault history, mean repair interval |

#### Department supervisor

| ID | Default role | Screen / route | Primary action and required data |

| --- | --- | --- | --- |

| FH-00 | Finishing supervisor | Finishing flow map | Throughput of trimming/press/check/rework |

| FH-05 | Finishing supervisor | Finishing control board | Final pass, redo types, queue aging |

| FNS-01 | Finishing supervisor | Finishing staff and station plan | Balance ironing/checkers/alteration capacity |

| PK-00 | Packing & shipping supervisor | Pre-ship flow | Queue by ready/packed/audit/stage/loaded |

| PK-05 | Packing & shipping supervisor | Pack & ship supervisor | Packing/audit/repack/ETD control |

| LOG-03 | Packing & shipping supervisor | Dock, truck and loading calendar | Approve dock/time allocations and deviations |

| CUT-01 | Cutting supervisor | Cut plan optimizer | Roll allocation, nesting and markers |

| CUT-05 | Cutting supervisor | Pre-sewing balance gate | Match issued/consumed/cut/released by size |

| CUT-06 | Cutting supervisor | Cutting multi-table console | Fabric yield and work queue |

| CUT-07 | Cutting supervisor | Defect-aware re-cut | Approve recut after defect and limit waste |

| STR-03 | Stores manager | Live stores and roll location | Stock available/reserved/held by bin |

| STR-05 | Stores manager | Trims and packaging stores | Issues, minimum cover and shortages |

| STR-06 | Stores manager | Surplus/deadstock aging | Sale/write-off/reallocate approvals |

| STO-01 | Stores manager | Inbound GRN / ASN exceptions | Supplier shortfall, missing inspection |

| STO-02 | Stores manager | Material allocation to orders | Reserve roll by lot/shade and expected yield |

| QM-01 | Multi-line quality manager | Plant quality overview | Compare DHU and first pass by line |

| QM-02 | Multi-line quality manager | Defect Pareto | Find systemic defect concentration |

| QM-03 | Multi-line quality manager | Style-wise risk and quality | Open change controls, pre-production risks |

| QM-04 | Multi-line quality manager | Quality trends | Shift/day/line controlled comparisons |

| QM-05 | Multi-line quality manager | Audit calendar & compliance | Buyer visit and AQL coverage |

| QLT-01 | Multi-line quality manager | Quarantine & NCR decisions | Release/rework/scrap governed with evidence |

#### Factory manager

| ID | Default role | Screen / route | Primary action and required data |

| --- | --- | --- | --- |

| PM-01 | Plant manager | Plant overview | All active lines, output, WIP, DHU, incidents |

| PM-02 | Plant manager | Live issues | Owner, watchers, SLA and support action |

| PM-03 | Plant manager | Line drill-in | Resource and station conditions |

| PM-04 | Plant manager | Workforce & attendance | Absence, capability gap, float availability |

| PM-05 | Plant manager | Day performance & next-day plan | Projection and approval queue |

| PM-06 | Plant manager | Factory disruption cockpit | Candidate mitigations with tradeoff costs |

| PM-07 | Plant manager | Delivery and profitability radar | Which PO slips and where cash/WIP sit |

| OB-01 | IE / production manager | OB authoring / costing | Select ops, SAM, machine, prerequisites |

| OB-02 | IE / production manager | OB execution allocation | Publish frozen route/workstations |

| IE-01 | IE / production manager | Operation master | Govern machines, capabilities, methods |

| IE-02 | IE / production manager | SAM/SMV calibration | Compare IE standard vs cleaned observed distribution |

| IE-03 | IE / production manager | Line balance & model | Edit parallel capacity and simulate WIP |

| PLN-01 | Production planner | 8-stream T&A | Fabric, trims, cut, sew, finish, pack, machine, skills |

| PLN-02 | Production planner | Dynamic capacity & loading | Resource-constrained lines, calendars and plants |

| PLN-03 | Production planner | Daily production plan | Publish reason-coded per-line ramp |

| PLN-04 | Production planner | Skill demand horizon | Train/hire/assign future skill coverage |

| PLN-05 | Production planner | Critical-order replanning | Compare consequences and approve change |

| PLN-06 | Production planner | Materials + WIP readiness | Calculate planned start viability |

#### Commercial office

| ID | Default role | Screen / route | Primary action and required data |

| --- | --- | --- | --- |

| CST-02 | Sales / account manager | Order book / margin pipeline | Quoted, confirmed, at-risk and revenue basis |

| SAL-01 | Sales / account manager | Customer and contract master | Buyer-specific deadlines, Incoterms and rules |

| SAL-02 | Sales / account manager | Quote / RFQ pipeline | Offer price/capacity/lead-time with assumptions |

| SAL-03 | Sales / account manager | Sales order & PO amendment | Import/approve terms, quantities and delivery dates |

| SAL-04 | Sales / account manager | Buyer communication / change approval | Track acknowledgements and documentary exceptions |

| PK-01 | Merchandiser | Buyer pack-plan setup | Publish solid/ratio carton and polybag rule |

| MER-01 | Merchandiser | Order technical readiness | Buyer PO, spec, labelling, PP approvals checklist |

| MER-02 | Merchandiser | Trim & fabric milestone tracker | Resolve late approvals and supplier actions |

| MER-03 | Merchandiser | Style / document variance desk | Resolve conflicting tech pack vs PO vs OB |

| MER-04 | Merchandiser | Buyer shipment and document packet | Assemble approved documents and acknowledgements |

| CST-01 | Costing / finance analyst | Enquiry costing studio | Unit BOM, direct labor SAM, wastage, target margin |

| FIN-01 | Costing / finance analyst | Order actual-versus-standard P&L | Cost-to-complete and per-unit contribution |

| FIN-02 | Costing / finance analyst | Consumption, loss & scrap valuation | Price tagged material movements |

| FIN-03 | Costing / finance analyst | Supplier invoice / 3-way match | PO, GRN, invoice exceptions |

| FIN-04 | Costing / finance analyst | AR / shipping invoice handoff | Link dispatch and invoice/document readiness |

| FIN-05 | Costing / finance analyst | Working capital & WIP aging | Cash tied in rolls, part kits, finished cartons |

| FIN-06 | Costing / finance analyst | Finance evidence & adjustments | Approved period close and corrections |

| STR-01 | Procurement head | Indent and surplus reuse gate | Net need from BOM less allocatable stock |

| SRC-01 | Procurement head | Supplier PO / approval | Issue, amend, cancel supplier order |

| SRC-02 | Procurement head | Vendor ETA and risk | ASN, delivery promise, shortage, quality flags |

| SRC-03 | Procurement head | Supplier scorecard | Late supply, usable yield and defect rate |

#### Governance / platform

| ID | Default role | Screen / route | Primary action and required data |

| --- | --- | --- | --- |

| EXEC-01 | Leadership / CFO / COO | Consumption variance approval | Decide exception with source evidence |

| EXEC-02 | Leadership / CFO / COO | Management council cockpit | Order rescue with explicit tradeoffs |

| EXE-03 | Leadership / CFO / COO | Executive performance radar | Profitability, cash, on-time, quality, WIP |

| EXE-04 | Leadership / CFO / COO | Multi-factory benchmark | Compare mix-adjusted performance / risk |

| EXE-05 | Leadership / CFO / COO | Scenario approval center | Sign or reject recommendation and rollback |

| OC-01 | Order closure auditor | Reconciliation working desk | Explain each stage quantity exception and sign |

| OC-02 | Order closure auditor | Immutable OCR certificate | Publish signed output and addenda |

| OC-03 | Order closure auditor | Full traceability / recall | Trace roll-to-carton or carton-to-roll |

| ADM-01 | Systems / IT administrator | Tenant/plant/line master | Configure physical factory and shifts |

| ADM-02 | Systems / IT administrator | Devices, readers and tags | Health, commissioning and key rotation |

| ADM-03 | Systems / IT administrator | Roles and separation of duties | User scope, access and restricted actions |

| ADM-04 | Systems / IT administrator | Integration / API status | Inbound maps, replay and connector errors |

| ADM-05 | Systems / IT administrator | Service health / replay | Stream lag, offline gaps, recover failed events |

| ADM-06 | Systems / IT administrator | Document/rule revision audit | Diff versions, publish effective dates |

| OPT-01 | Optimization / data scientist | Plant digital twin | Observe graph state and predicted bottlenecks |

| OPT-02 | Optimization / data scientist | Scenario comparator | Cost/SLA/confidence before approval |

| OPT-03 | Optimization / data scientist | Intervention experiment evaluation | Observed result vs control; undo or refine |

### 5.3 Dashboard interaction and refresh contracts

| User tier | Refresh / visibility | Primary decision | Failure display |

| --- | --- | --- | --- |

| Station/operator | Immediate local response; event receipt and commit distinct | Can this specific work progress now? | Clear WAIT / RETRY / supervisor help; no silent tap |

| Line/department lead | Live read models with current freshness timestamp; target <5 s within plant | Which help action restores flow/quality? | Last-updated and degraded data banner |

| Plant/factory manager | Cross-line updates target <5 s locally | Which resource, order or quality risk needs intervention? | Explicit missing-line counts or stale indicators |

| Sales/finance/engineering | Version-locked transaction screens; recompute on published change | Approve an immutable commitment or exception | Show which source contract remains pending |

| Executive/multi-plant | Eventual, timestamped rollups; hour/shift metrics often enough | Which scenario improves good output and margin? | Never show stale factory results as instantaneous |

### 5.4 Design language and no-blame UX

- No typing for common operator actions. Deliberate HF IN and OUT pads or scanner plus one large decision; audio/icon prompts adapted to the factory language. Don’t assume arbitrary device screen sizes, and test physical touch-target dimensions with real gloves/hands.

- One primary action, a clear reason for a rejected command, a known responder for every active issue, and visible status/pending state. Per-worker data is restricted for coaching and compliance; shared boards emphasize operation, station and line.

- Global design tokens: target vs accepted output, DHU vs FPY, current WIP, blocked WIP, lateness, device health and event freshness are defined exactly once in S25 and exposed through read models.

- Screens show only modules installed and authorized. Missing modules are handled by supported import/export and “external system” badges, not broken links or empty dashboard promises.

### 5.5 Screen-to-service dependency map

| Screen groups | Primary write owners | Read models / integration sources |

| --- | --- | --- |

| SEW, MAT, QS, LIN | S14 sewing, S17 quality, S23 issues | S05 route, S11 inventory, S15 people, S16 machines, S24 live read |

| STR, STO, CUT | S11 inventory, S12 inspection, S13 cutting | S08 MRP, S09 schedule, S10 procurement, S24 live read |

| FH, FNS, PK, LOG | S18 finishing, S19 packing, S20 dispatch | S17 QC, S04 SKU, S06 PO, S23 issue, S24 live read |

| PM, QM, IE, MNT | S23 issue, S16 maintenance, S05 OB, S09 planning | S14 station, S17 QC, S25 KPI, S24 plant read |

| SAL, MER, CST, SRC | S06 orders, S03 docs, S07 costing, S10 sourcing | S04 product, S08 MRP, S09 capacity, S22 finance |

| FIN, OC | S22 finance, S21 closure | S11 physical movements, S06 orders, S19 cartons, S20 manifests |

| EXEC, EXE, OPT | S26 scenarios, S29 federation, S21 approvals | S22 economics, S25 metrics, S24 freshness-labeled plant reads |

| ADM | S01 access, S02 devices, S03 documents, S28 connectors | S24 observability projections and signed audit |

For every route the frontend repository shall contain screen_id, capability flag, permitted role/scope, BFF query schema, primary command owner, disabled/offline states, and acceptance test IDs. UI orchestration calls BFFs; only domain services decide committed stock, output, QC and shipment state.

## 6. Data system: four persistence technologies, explicit ownership

Recommendation: four durable storage technologies, deployed in five main persistence locations. Do not create 29 unrelated database engines just because there are 29 microservices. Use one database cluster per isolation/availability domain, with separately owned logical databases/schemas and credentials. “Owner” means the only service authorized to write that bounded aggregate.

| Store technology | Pilot deployment location / count | Workload and ownership | Important restriction |

| --- | --- | --- | --- |

| PostgreSQL | 1 plant cluster + 1 cloud cluster | OLTP: orders, docs, stock ledger, route graph, executions, quality, cartons, finance integration; own logical schema/database per service | No peer services writing or cross-joining another owner’s schema; use APIs/events |

| SQLite WAL | 1 local store per reader gateway/edge station runtime as needed | Temporary durable command/outbox buffer, clock offsets and signed rule cache | Edge cache is never a second canonical inventory ledger |

| ClickHouse | 1 cloud analytics cluster initially | RFID observations, append-only historical events, cycle distributions, plant rollups, benchmarking | Analytics-derived aggregates cannot independently authorize material or shipment |

| S3-compatible encrypted object storage | 1 cloud bucket group plus optional on-prem mirror | PO PDF, tech pack, OB original, images, calibrated roll defect maps, signed manifests, OCR reports | Hashes and metadata referenced from Postgres; content version and access audited |

**Infrastructure, not fifth database** NATS JetStream carries durable commands/events at plant and cloud; it is a broker/log, not the transactional business source of truth. Redis is optional only if dashboard BFF caching requires it; it must never carry exclusive stock or carton state. A separate graph database is not required.

### 6.1 Physical partitions and numbers that engineering can size

- At a reference pilot of ≥14 concurrent sewing lines, partition execution stream consumers at least by plant, line and carrier/aggregate hash; ensure all transitions of the same lineage or carton serialize logically. Partition count and retention are capacity-test inputs, not arbitrary vendor promises.

- Plant PostgreSQL: 1 transactional source-of-authority cluster; start with separate schemas and service database accounts. Introduce separate physical clusters only for measured hot services (e.g. execution/packing) or compliance isolation.

- Cloud PostgreSQL: 1 control-plane cluster containing distinct logical service schemas and replicated read projections; tenant partitioning uses plant/tenant keys and enforced RLS on shared tenant-hosted schemas. Sensitive finance may use separate logical DB/keys.

- Cloud ClickHouse: raw observations separated from accepted domain events and verified KPI aggregates. Partition by tenant, plant and event date; define rollups and retention before onboarding multiple factories.

- Immutable object storage: tenant/purpose-specific prefixes/buckets, object-lock retention according to approved policy; store pointers and SHA-256 hashes in Postgres.

- At least two NATS nodes at a reliability pilot are desirable, but a production HA quorum requires a supported three-node failure-domain plan. Choose topology after site power, LAN and operational support assessment.

### 6.2 Data domains and table ownership

| Owning services | Core relational tables | Primary stable IDs | Integrity rule / relationship |

| --- | --- | --- | --- |

| S01–S03 | tenants, plants, lines, users, roles, grants; devices, credentials, policies; documents, document_revisions, field_assertions, approvals | tenant_id, plant_id; document_revision_id | doc -> governed field -> approving user; device enrollment keys |

| S04–S05 | style, style_revision, sku, material_master, bom_component, pom_dimension, measurement_spec, operation, route_revision, route_node, route_edge, route_gate | style_revision_id, sku_id, route_revision_id, node_id | each route_node version belongs to immutable route_revision; graph FK edge references node same revision |

| S06–S07 | customer, buyer_account, customer_po, customer_po_revision, order_line, requested_delivery, quote, quote_line, standard_cost_version | po_id, order_line_id, quote_version_id | one PO has many lines; PO revision retains external line IDs and supersession |

| S08–S10 | material_requirement, pegging, line_loading, planned_operation, capacity_slot, ta_milestone, supplier, supplier_po, supplier_po_line, asn | plan_revision_id, requirement_id, supplier_po_id | requirements pegged to PO-line + BOM revision; supplier order distinct from buyer PO |

| S11–S12 | inventory_location, stock_lot, fabric_roll, roll_measurement, defect_mark, shade_band, reservation, stock_movement, tag_asset, tag_binding | roll_id, stock_movement_id, tag_binding_id | movement ledger append-only; physical UID reusable across nonoverlapping binding intervals |

| S13–S14 | marker_revision, lay_plan, lay_actual, cut_lot, cut_component, bundle, carrier, carrier_parent, work_order, station_cycle, op_credit, operation_claim | lay_id, bundle_id, carrier_id, credit_id | physical qty derived from cut actual and authoritative splits/merges |

| S15–S16 | worker, skill, certification, attendance, assignment, machine, machine_capability, down_event, maintenance_job, spare_issue | worker_id, assignment_id, machine_id | worker role validity interval and machine capability validated at operation claim |

| S17–S18 | inspection, inspection_finding, defect_code, ncr, quality_disposition, rework_ticket, finishing_cycle, garment_instance, buyer_identifier_binding | inspection_id, unit_id, rework_ticket_id | quality state tracked at correct unit/quantity granularity |

| S19–S20 | polybag, polybag_piece, carton, carton_piece, carton_pack_plan, carton_seal, pack_audit, staging_location, manifest, manifest_carton, load_scan | bag_id, carton_id, manifest_id | buyer labels separate from internal carrier tags; carton membership unique until authorized unpack |

| S21–S23 | stage_balance, variance, closure_signoff, ocr_revision, cost_posting, invoice_reference, issue, issue_response, escalation | closure_id, cost_posting_id, issue_id | accounting corrections as new postings/addenda, not mutation of history |

| S24–S29 | read_projection, metric_definition, metric_snapshot, scenario, intervention, scenario_outcome, external_id_map, federation_request | projection_key, scenario_id, mapping_id | derived caches not used as canonical transaction writes |

### 6.3 Relational keys: required technical conventions

- Every authoritative row has immutable UUIDv7/ULID-style ID (actual format standardized in schema), tenant_id, owning plant_id where applicable, created_at, updated_at, revision or aggregate_version, and source provenance; public business numbers (PO number, EPC, UPC, machine code) are attributes/aliases, never global primary keys.

- FKs are enforced inside one service schema. Cross-service references are immutable external entity IDs with a reference projection validated by contract/event; do not create cross-service FK cycles or distributed SQL transactions.

- Natural unique constraints include (tenant_id, buyer_id, buyer_po_no, amendment_revision), (tenant_id, sku_style_revision, color, size), (plant_id, tag_uid, nonoverlap binding interval), (carton_id, accepted unit_id), and idempotency_key per actor/aggregate/command namespace.

- Every count or movement is integer pieces/bags/cartons or a decimal with explicit SI units; never mix meters and centimeters in material-defect coordinates. Shade and lot are required on material-bearing joins.

- Use append-only stock_movements and production_qty_movements with reason/disposition; balance snapshots are projections. Reserve and issue are not the same transaction. Scrap cannot be converted back to good without a separately approved recovery event.

## 7. The graph model: BOM + operation DAG + physical genealogy

Three graphs, three distinct jobs. The BOM graph says what components a product needs. The operation DAG says in what feasible partial order those components can be made and assembled. The immutable genealogy graph says which actual physical lots/components produced an actual finished piece/carton. Do not confuse their edges or substitute a decorative factory layout for any of these.

### 7.1 Design-time typed operation DAG

```
StyleRevision --has--> BOMRevision --requires--> Material/Part/Quantity
            |                     (size, color, shade, pack variant)
            +--has--> RouteRevision
                         +-- RouteNodes: operation | inspect | split | join |
                         |  transfer | buffer | bypass | consume-bulk | terminal
                         +-- RouteEdges: predecessor, successor, condition, qty-ratio
                         +-- Gates: AND/OR, size/shade/lot, approved machine/skill
                         +-- EvidencePolicy: scan, dwell, stitch, check, signoff
Route compile: validate acyclicity, reachable terminal, BOM feasibility,
unique business meaning, material compatibility, conditional coverage,
and specified rework exits before Publish.
```

- Operation node defines stable operation_code, route_revision_id, required part_type, output part_type, machine_class, SAM/SMV approved, operator qualification, incoming/outgoing quantity ratios, quality gate, station evidence policy, explicit waste categories and work-instruction URI.

- Edges define dependency type: mandatory AND-join, alternative OR, optional condition, bulk feed, serialization constraint or synchronization barrier. Continuous waistband/loop strips may be bulk-consumed with measured quantity; do not invent individual tags for every tiny component.

- Examples: 220988 trousers require pocket/front, back/yoke, waistband, fly/zip and hem operation paths, with actual nodes authorized by the correct OB. For waist ≤42, tech-pack belt-loop detail shows 7 loops; sizes ≥44 (not ordered) illustrate a conditional 9-loop route.

- Rework is a separate stateful work-order/ticket graph linked to the original approved operation; repeating a repair operation does not create a cycle in the original published routing DAG or duplicate quantities.

- Graph compilation shall detect missing start/terminal, unjoined required components, incompatible size/shade joins, impossible material supply, negative yield, ambiguous routing branches and operation/machine mismatch.

### 7.2 Runtime genealogy and conservation

```
MaterialLot/Roll ──consumed by──> Lay/CutLot ──creates──> ComponentBundle
                                 │                     │
                            defect map           SPLIT / REWORK
                                                       │
  Piece/Bundle A ──┐                                  v
  Piece/Bundle B ──┼──JOIN(compatible cut/size/shade)──> AssemblyCarrier
  BulkStrip Pool ──┘                                          │
                                        garment_instance <- unit attribution
                                               │
                                        buyer_id/hangtag bind
                                               │
                                        Polybag (exact 2 units)
                                               │
                                     Carton (12 or 24 polybags)
                                               │
                                      Manifest -> Truck/Ship
```

At every owning inventory/process boundary: OpeningQuantity + AcceptedReceipts = AcceptedOutbound + ClosingWIP + ExplicitScrap + ExplicitLoss + ExplicitReturns ± ApprovedAdjustments. Separate piece-count conservation from material-length conservation and shipment order-fulfilment; they have different units and denominators. “Cut = Shipped + Lost + Leftover” is only a valid final closure projection when WIP, shipped status, rework, authorized extras and dispositions are folded into exact declared terms.

### 7.3 Reusable RFID vs buyer identities

| Identity type | Physical medium | Binding and lifecycle | Primary rule |

| --- | --- | --- | --- |

| Roll RFID | UHF EPC Gen2 | Physical tag on inspected fabric roll; receive -> locate -> issue -> return/decommission | UID alone never equals stock quantity; roll_id and custody ledger authoritative |

| Reusable component bundle | HF/NFC deliberate tap | Physical tag_asset reused; tag_binding points to one current bundle | Tag UID can be reused only after prior binding closed and lineage committed |

| Moving rack/garment | UHF optionally, keyed by unit or hanger | Zone tracking / presence, not proof of a completed operation | Portal observation separate from validated custody/production transition |

| Buyer garment identifier | Buyer-specific printed barcode/RFID hangtag | Unique serial or lot-derived tag if buyer issued; must bind to an accepted garment | UPC is SKU-level and generally not unique garment identity |

| Buyer replenishment bag | RFID sticker at 2-piece polybag | bag_id has exactly 2 distinct accepted garment instances | Do not confuse factory bundle tag with buyer polybag sticker |

| Shipping carton | Unique carton label/barcode; RFID only if specified | carton_id contains verified bags; seal and pack plan version | Reject seals until pack slots, audit/release and exception policies satisfied |

## 8. Canonical transaction schema: representative SQL DDL

This is the reference physical contract to drive first migrations; the tables illustrate patterns and are not a substitute for the full per-service migration set. The SQL assumes PostgreSQL 16+ and UUIDs created by the application. Check constraints, tenant isolation, immutable-policy triggers and partial-index details shall be expanded in each service repo.

```
CREATE TABLE customer_po (
  po_id uuid PRIMARY KEY,
  tenant_id uuid NOT NULL,
  buyer_id uuid NOT NULL,
  buyer_po_no text NOT NULL,
  current_revision integer NOT NULL,
  status text NOT NULL CHECK (status IN ('DRAFT','APPROVED','CLOSED')),
  UNIQUE (tenant_id, buyer_id, buyer_po_no)
);
CREATE TABLE po_line (
  po_line_id uuid PRIMARY KEY,
  po_id uuid NOT NULL REFERENCES customer_po(po_id),
  line_no integer NOT NULL,
  sku_id uuid NOT NULL,                  -- S04-owned stable reference
  ordered_qty integer NOT NULL CHECK (ordered_qty > 0),
  pack_plan_revision_id uuid,
  UNIQUE (po_id,line_no)
);
CREATE TABLE route_node (
  route_revision_id uuid NOT NULL,
  node_id uuid NOT NULL,
  operation_code text NOT NULL,
  kind text NOT NULL,
  sam_seconds numeric(12,3),
  input_part_type text, output_part_type text,
  condition_expr jsonb NOT NULL DEFAULT '{}'::jsonb,
  PRIMARY KEY(route_revision_id,node_id)
);
CREATE TABLE route_edge (
  route_revision_id uuid NOT NULL,
  predecessor_id uuid NOT NULL,
  successor_id uuid NOT NULL,
  gate_type text NOT NULL,
  PRIMARY KEY(route_revision_id,predecessor_id,successor_id),
  FOREIGN KEY(route_revision_id,predecessor_id)
    REFERENCES route_node(route_revision_id,node_id),
  FOREIGN KEY(route_revision_id,successor_id)
    REFERENCES route_node(route_revision_id,node_id),
  CHECK(predecessor_id <> successor_id)
);
```

```
CREATE TABLE tag_asset (
  tag_asset_id uuid PRIMARY KEY, plant_id uuid NOT NULL,
  uid text NOT NULL, medium text NOT NULL,
  UNIQUE (plant_id, medium, uid)
);
CREATE TABLE tag_binding (
  binding_id uuid PRIMARY KEY,
  tag_asset_id uuid NOT NULL REFERENCES tag_asset(tag_asset_id),
  logical_carrier_id uuid NOT NULL,
  bound_at timestamptz NOT NULL,
  unbound_at timestamptz,
  CHECK (unbound_at IS NULL OR unbound_at > bound_at)
);
-- Add PostgreSQL exclusion constraint / active-binding partial unique index
-- to ensure no overlapping active bindings for a tag asset.
CREATE UNIQUE INDEX one_open_binding_per_tag
  ON tag_binding(tag_asset_id) WHERE unbound_at IS NULL;
CREATE TABLE inventory_movement (
  movement_id uuid PRIMARY KEY, plant_id uuid NOT NULL,
  lot_id uuid NOT NULL, event_id uuid NOT NULL UNIQUE,
  quantity numeric(18,4) NOT NULL CHECK(quantity > 0),
  uom text NOT NULL, movement_type text NOT NULL,
  from_location_id uuid, to_location_id uuid,
  reason_code text, occurred_at timestamptz NOT NULL
);
CREATE TABLE accepted_operation_credit (
  credit_id uuid PRIMARY KEY, plant_id uuid NOT NULL,
  carrier_id uuid NOT NULL, route_revision_id uuid NOT NULL,
  node_id uuid NOT NULL, operator_id uuid NOT NULL,
  quantity integer NOT NULL CHECK(quantity > 0),
  evidence_policy text NOT NULL, occurred_at timestamptz NOT NULL,
  source_command_id uuid NOT NULL UNIQUE,
  UNIQUE(carrier_id,route_revision_id,node_id,source_command_id)
);
```

```
CREATE TABLE polybag (
  polybag_id uuid PRIMARY KEY,
  plant_id uuid NOT NULL, po_line_id uuid NOT NULL,
  buyer_rfid_id text UNIQUE, status text NOT NULL,
  pack_plan_revision_id uuid NOT NULL
);
CREATE TABLE polybag_piece (
  polybag_id uuid NOT NULL REFERENCES polybag(polybag_id),
  garment_id uuid NOT NULL UNIQUE,
  PRIMARY KEY(polybag_id,garment_id)
);
-- Service transaction enforces required count = 2 at CLOSE, not each INSERT.
CREATE TABLE carton (
  carton_id uuid PRIMARY KEY, plant_id uuid NOT NULL,
  carton_serial text NOT NULL, po_id uuid NOT NULL,
  packing_plan_revision_id uuid NOT NULL,
  status text NOT NULL, aggregate_version bigint NOT NULL DEFAULT 0,
  UNIQUE(plant_id,carton_serial)
);
CREATE TABLE carton_polybag (
  carton_id uuid NOT NULL REFERENCES carton(carton_id),
  polybag_id uuid NOT NULL UNIQUE REFERENCES polybag(polybag_id),
  PRIMARY KEY(carton_id,polybag_id)
);
CREATE TABLE outbox (
  event_id uuid PRIMARY KEY, aggregate_type text NOT NULL,
  aggregate_id uuid NOT NULL, aggregate_version bigint NOT NULL,
  event_type text NOT NULL, schema_version integer NOT NULL,
  payload jsonb NOT NULL, occurred_at timestamptz NOT NULL,
  sent_at timestamptz
);
CREATE INDEX outbox_pending ON outbox(occurred_at) WHERE sent_at IS NULL;
```

### 8.1 Physical indexes, partitions and tenancy rules

- Hot indexes: (plant_id, station_id, occurred_at DESC); (plant_id, carrier_id, aggregate_version); (plant_id, po_line_id, state); (plant_id, carton_id, status); (tenant_id, buyer_id, due_date); partition large accepted-events and audit histories by plant and month.

- Identity and document access: tenant-scoped row-level security, object-store prefix ACL, per-service secrets, auditable emergency access; plant services may only see signed, approved subsets of commercial information.

- Bundle lineage: use a carrier_parent join table with merge_event_id and consumed quantities; prevent double parent consumption with single-authority serializable/locked transaction. For high-level ancestry/descendants use recursive CTEs or maintained closure projections, not a separate OLTP graph database.

- Inspection records have inspection_scope (piece/bundle/lot), pieces_checked, defects_found, sampled_units, and unique source observation; a single defect can create a disposition without decrementing physical piece counts until scrap is posted.

- Outbox: accepted domain command + resulting authoritative ledger delta + event publication record must commit in one database transaction. Consumers store inbox dedupe identity for replay and race protection.

## 9. Service-to-service API and event contracts

Commands are accepted by the owning service; queries return a typed view plus as_of timestamp, owner and provenance. Cross-domain workflows are orchestrated or choreographed with explicit sagas and compensations. A web app may not update a foreign service database directly.

### 9.1 Core API surface (minimum v1)

| Owner | Representative command/query endpoints | Key integration events |

| --- | --- | --- |

| S03 Docs | POST /v1/documents/import; POST /v1/documents/{id}/approve | DocumentImported; DocumentApproved |

| S04 Products | GET /v1/styles/{id}/revisions; POST /v1/boms/{id}/approve | StyleApproved; BomPublished |

| S05 Route | POST /v1/routes/{id}/validate; POST /v1/routes/{id}/publish | RouteValidated; RoutePublished |

| S06 Orders | POST /v1/customer-orders; POST /v1/customer-orders/{id}/amend | OrderAccepted; OrderAmended |

| S07 Quote | POST /v1/quotes; POST /v1/quotes/{id}/approve | QuoteApproved |

| S08 MRP | POST /v1/material-requirements/recompute; GET /v1/shortages | MaterialRequirementChanged |

| S09 Scheduling | POST /v1/plans/{id}/simulate; POST /v1/plans/{id}/publish | PlanPublished; PlanSuperseded |

| S10 Procurement | POST /v1/supplier-pos; POST /v1/receipts/asn | SupplierPoPlaced; AsnReceived |

| S11 Inventory | POST /v1/rolls/{id}/reserve; POST /v1/materials/issue; GET /v1/stock | MaterialReserved; MaterialIssued |

| S12 Inspection | POST /v1/roll-inspections; POST /v1/roll-inspections/{id}/release | RollInspected; RollAccepted |

| S13 Cutting | POST /v1/lays/{id}/complete; POST /v1/bundles/commission | LayCompleted; BundleCommissioned |

| S14 Sewing | POST /v1/carriers/{id}/claim; POST /v1/carriers/{id}/complete; POST /v1/merges | OperationCredited; CarrierMerged |

| S15 Workforce | POST /v1/assignments; GET /v1/skill-availability | WorkerAssigned; SkillGapRaised |

| S16 Maintenance | POST /v1/andons; POST /v1/work-orders/{id}/resolve | MachineDown; MachineRestored |

| S17 Quality | POST /v1/inspections; POST /v1/dispositions/{id}/approve; POST /v1/audits | DefectFound; NCRClosed; AuditDecided |

| S18 Finishing | POST /v1/garments/{id}/bind-buyer-id; POST /v1/finishing/{id}/pass | BuyerIdentityBound; GarmentReleased |

| S19 Packing | POST /v1/polybags; POST /v1/cartons/{id}/add-bag; POST /v1/cartons/{id}/seal | PolybagPacked; CartonSealed |

| S20 Shipping | POST /v1/manifests; POST /v1/manifests/{id}/load; POST /v1/manifests/{id}/dispatch | CartonLoaded; ShipmentDispatched |

| S21 Closure | POST /v1/orders/{id}/reconcile; POST /v1/closures/{id}/sign; POST /v1/closures/{id}/issue | OrderBalanced; OrderClosed |

| S22 Finance | GET /v1/orders/{id}/actual-cost; POST /v1/cost-postings | CostPosted |

| S23 Issues | POST /v1/issues; POST /v1/issues/{id}/assign; POST /v1/issues/{id}/resolve | IssueOpened; HelpAssigned; IssueResolved |

| S25 Metrics | GET /v1/metrics?plant=&period=; GET /v1/benchmark/cohorts | MetricSnapshotPublished |

| S26 Optimize | POST /v1/scenarios; POST /v1/interventions/{id}/approve | ScenarioEvaluated; InterventionApproved |

| S28 Integration | POST /v1/external-mappings; GET /v1/integrations/{id}/health | ExternalIdentityMapped |

### 9.2 Standard command/event envelope and semantics

```
{
  "event_id":"<uuid>", "event_type":"OperationCredited",
  "schema_version":1, "tenant_id":"<uuid>", "plant_id":"<uuid>",
  "aggregate_type":"carrier", "aggregate_id":"<uuid>",
  "aggregate_version":87, "correlation_id":"<uuid>",
  "causation_id":"<uuid>", "idempotency_key":"station:command:<uuid>",
  "occurred_at":"2026-10-24T10:00:00.123Z",
  "received_at":"2026-10-24T10:00:00.150Z",
  "actor_id":"<uuid>", "station_id":"<uuid>",
  "source_device_id":"<uuid>", "policy_revision":"evidence-sewing-v3",
  "data":{
    "carrier_id":"<uuid>", "route_revision_id":"<uuid>",
    "route_node_id":"<uuid>", "qty_credited":20,
    "evidence_types":["nfc_in","nfc_out","role_check"],
    "confidence":"VERIFIED_BY_POLICY"
  }
}
```

- Services must reject unknown breaking event versions, publish before/after changes with causal IDs, and preserve ordering within aggregate; event consumers tolerate reorder between unrelated aggregates.

- Every write API requires an Idempotency-Key and expected aggregate revision for contentious updates. HTTP 409 means version/ownership conflict; 422 means violated business invariant; 503 with retry semantics is not reported as accepted.

- Read models expose `as_of`, `scope`, `latency_ms`, `source_service`, `data_quality` and a degraded flag. A dashboard KPI without an explicit unit and denominator does not satisfy the schema.

- For multi-step work (reserve -> cut -> release -> pack -> dispatch), state transitions are modeled with a saga ID; a failed step creates compensating release where safe or an owned exception task, never an undetected half-commit.

### 9.3 Example transaction: two-piece bag and exact buyer carton

1. Packing station requests/reads an approved carton template referencing a specific PO line and pack_plan_revision. The packer scans the carton serial; S19 claims the aggregate revision.

1. The accepted garment instance list is queried from S18/S17 replicated authorized availability projection. Scanner submits unique garment_id + SKU/size/lot + read evidence. S19 transaction checks not already bagged/packed, not on quality hold, permitted SKU.

1. When exactly two distinct accepted units are assigned, S19 creates one buyer polybag with required RFID sticker, records two membership rows, and marks the polybag sealed or pending label verification.

1. Carton accepts only bags of correct solid-size SKU, exactly 12 bags for a 24-piece carton or 24 bags for a 48-piece carton, unless a separately signed partial-carton amendment is effective. Optional weighing is an evidence check, not a replacement for individual identities.

1. Seal transaction checks bag count, label and carton barcode, required audit state, and active amendment. Emits CartonSealed atomically. S20 may load only released cartons referenced by a manifest.

1. The size-42 residual in REP1/REP2 creates an explicit PackagingExceptionAwaitingBuyer decision. An unauthorized 16- or 8-unit partial carton must never be silently sealed and counted as full.

## 10. RFID, physical readers, and station-control requirements

| Physical location | Identity / sensor | Scan intent | Engineering checks |

| --- | --- | --- | --- |

| Fabric receiving/storage | UHF RFID portals and handhelds; rotary encoder/vision at inspection | Record receipt, roll finding and measured defects | Calibration, read-zone collision, observed vs approved length |

| Material issue gate | Deliberate UHF roll selection + two authenticated witnesses | Transfer roll from stores to cutting | Actor separation, exact roll/lot, local quantity ledger |

| Spreading/bundling | HF/NFC durable bundle tags; cut/lay witness | Commission separate physical component bundles | Source cut lot, qty/size/shade/part_type, duplicate binding prevention |

| Each sewing workstation | Physically distinct HF/NFC IN and OUT touch zones | Deliberate claim and completion | Reader debounce, cycle plausibility, optional stitch/activity sensor; no false UHF flyby credit |

| Assembly/merge station | Multi-tag deliberate taps or controlled batch list | Check complete prerequisites and compatible parent components | Atomic join, missing component exception, tag retirement/reuse |

| Finishing/hangtag | Barcode/RFID buyer label plus internal carrier | Bind buyer identifier to accepted garment | Correct buyer PO/size, avoid serial reuse |

| Packing | Buyer bag RFID, carton barcode, scale if approved | Bag creation, carton slot fill, seal | Two distinct accepted pieces/bag; exact packing rule |

| Dock | Carton scan + load location/container/vehicle | Verify specific carton custody transfer | Manifest authorization, scan dedupe, seal integrity |

- Hardware evidence confidence level E0 (manual/import), E1 (deliberate scan), E2 (+ role/route + timestamps), E3 (+ corroborating machine/stitch/metrology), E4 (+ QC/independent witness) is an implementation proposal; consumers must not falsely label an E1 observation “verified finished garment.”

- Clock: capture device monotonic elapsed time, wall-clock reading, NTP offset/uncertainty, gateway reception time. Report quality when drift exceeds threshold; do not retroactively reorder causal state using imperfect wall clocks.

- If no network is reachable from an individual station, queue only safe/idempotent local intents. For shared ownership-changing actions (tag merge, issue, seal, dispatch), require the local owning plant service and do not independently commit on disconnected devices.

- Minimum dwell (e.g., original 1.5 seconds for final inspection) is a plausibility guard only. Machine current/stitch signals support anomaly detection but do not prove the exact quantity sewn. Safety-critical interlocks cannot be implemented as an arbitrary cloud Modbus command.

- RFID procurement must be validated at real metal-heavy sewing tables, 14+ active lines, trolley routes, water/steam and human occlusion. Tag replacement/reuse economics should be measured as cost per accepted garment, not tag acquisition price alone.

## 11. Planning, optimization, and profitability are operational functions

Optimization is not a nightly report and not unrestricted automation. It operates on a feasible, versioned digital twin built from physical evidence, approved routes and calendars. Its output is a reproducible proposal plus economics; it becomes action only after authority and safe local dispatch checks.

### 11.1 Three planning horizons

| Horizon | Time window | Inputs | Published decision |

| --- | --- | --- | --- |

| Strategic commitment | Weeks to season | RFQ/PO, buyer margin, capacity offer, vendor terms and freight | Which order/style/plant can be committed profitably? |

| Tactical production | Days to weeks | BOM, material ETAs, marker yield, T&A, skills, approved OB, machine schedule | What will run on which line/table/shift; what buffers are justified? |

| Operational intervention | Minutes to shift | RFID WIP/queue ages, breakdown, current defect rates, missing worker, station time | Should we shift staff, repair, re-sequence work, change feeder path or recover material? |

### 11.2 Planning solver inputs and hard constraints

- Solve on exact PO line/color/size quantities, approved BOM materials, actual stock and roll length/defect maps, machine capability, certified skills, shift/labor rules, buyer packing, audit policy, shipping cutoffs, pre-production approvals, and the product operation DAG.

- Forecast factory throughput as accepted good output, not gross scanner taps. Maintain separate IE SAM, observed active-cycle distribution, queue distribution, downtime and setup/learning penalties. Do not re-authorize SAM merely because a noisy scan interval is shorter.

- Hard constraints: inventory cannot be consumed twice; no unapproved shade mixing; no skipped mandatory route node; no incorrect buyer UPC/package; no infeasible machine/skill assignment; no violating safety/manual interlock or buyer commitment without approval.

- Objective is expected cash contribution after lateness, overtime, scrap, rework, material loss, holding/WIP, transfers, expedite and maintenance. Each price/cost rate is a versioned finance assumption. The $6.30 PO field is the stated purchase-price field, not a proved factory selling price or margin.

- Transient absorption: use controlled buffers at material-ready, cut-kit, pre-bottleneck, finishing-accepted and pack-ready points. Optimize queue variability and expected lead time (Little’s Law as a diagnostic) rather than maximizing station utilization at the cost of huge WIP.

- Every suggestion reports baseline, 2–3 alternatives, constraints used, effect on other orders, sensitivity, expected cost/benefit range and data freshness. Any intervention is logged against its predicted and measured outcome.

### 11.3 Required disruption playbooks and optimization actions

| Transient | Detection | Proposed action | Failure prevention / result |

| --- | --- | --- | --- |

| Machine outage | Machine fault/telemetry + blocked station queue | Qualified mechanic; approved compatible machine; adjust feed | Never bypass safety; compute recovered accepted output |

| Worker absent or slow ramp | Attendance/skill coverage + observed line startup | Redeploy eligible float, cross-train or re-sequence | Not a blame algorithm; preserve qualification and labor rules |

| Cut fabric flaw / bad shade | Roll inspection defect map or cutting observation | Change marker, choose compatible reserved roll, recut panels | No wrong-shade merge; value saved in measured meters |

| Defect burst | Controlled DHU/FPY shift above min sample | Quarantine source, inspect upstream, rebalance checker coverage | Do not propagate suspect WIP downstream |

| Material transport queue | Station start/out and material gate dwell distribution | Change feeder loop, pickup size or drop sequence | Avoid local optimization that blocks other stations |

| Rush delivery / delayed supply | ASN/PO milestone change, forecast shortage | Present alternate line/shift/plant and overtime scenarios | Approval includes displaced-order economics |

| Packing residual carton | Exact PO size/carton arithmetic | Request buyer exception or adjust contractual quantity | Shipment held until authorized pack/shortage policy |

| Cloud down / bad clocks | Sync queue age; signed policy version; clock uncertainty | Operate local validated path, stop stale high-risk decisions | No duplicate posting on reconnect; surface confidence |

### 11.4 Economics and manager-facing measures

| Measure | Precise denominator / semantics | Used by |

| --- | --- | --- |

| First pass yield | Accepted units passing first inspection ÷ distinct units first inspected | Quality, planner, COO |

| DHU | 100 × recorded defects ÷ inspected units; disclose inspection stage and whether repeated passes counted | Line quality / multi-line QA |

| Earned labor efficiency | Approved earned SAM on accepted credited work ÷ eligible productive worker-minutes; event and rest exclusions policy-controlled | IE / plant manager |

| Throughput and bottleneck | Good units/hour, station production vs starvation vs blockage; line operation critical path | Line manager / optimizer |

| Material yield | Accepted cut component-equivalent output vs measured good material used; separate meters, area and pieces | Stores/cutting/finance |

| Cost per accepted unit | Material + direct labor + rework + variable overhead allocations on accepted output | CFO / costing |

| Order margin to complete | Contract revenue/price basis − actual spend − modeled remaining cost − approved risk reserve | Sales/CFO/executive |

| WIP cash and age | Current owned quantity/value grouped by location/state and age percentile | Plant manager/finance |

| OTIF | Accepted quantity delivered in full and on time against explicit commercial milestone | Sales/shipping/executive |

| Plan vs reality | Frozen baseline by plan version vs accepted events; reasons for approved amendments | Scheduling/plant manager |

## 12. Operational permissions, ownership, and approvals

Authentication is separated from policy. A role has tenant/plant/line/station scope, time-bound skills, action grants and delegations. A role assignment does not by itself waive a material, safety or buyer rule. High-risk changes demand separation of duties.

| Object/action | Can initiate | Required authorizer / gate | Recorded evidence |

| --- | --- | --- | --- |

| Style/tech pack/label version | Merchandiser / IE | Buyer approval where required; IE + merchandising release | Document revision, source fields, differences |

| OB route publication | IE author | Separate IE approver + quality/pre-production readiness | Route checksum, machine/SAM, change scope |

| Fabric issue | Stores clerk | Stores policy; authenticated receiving cutting staff | Roll, location, timestamps, witnesses |

| Cut/recut adjustment | Cutting supervisor | Material/cut plan policy, higher approval above threshold | Marker/lot, actual qty, reason |

| Carrier split/merge | Station authorized actor | Plant execution graph validates exact graph and qty | Parent IDs, child IDs, quantities, state revision |

| QC scrap/release | QC inspector | Qualified quality disposition authority | Inspection, severity, signoff |

| Carton partial exception | Packing/merchandiser | Buyer contractual exception + merchandiser/ship release | Affected sizes/cartons, case count, approval file |

| Load / dispatch | Loader / dispatch lead | Approved manifest, QC release, ship authorization | Carton scans, vehicle, seal, timestamp |

| Plan reallocation / overtime | Planner / line lead | Plant manager or defined cost authority | Before/after feasibility, displaced commitments |

| OCR final closure | Department owners | Cut, sewing, finish, packing/shipping, PM and QM attestations | Signed stage balance, addenda only |

| Finance variance / writeoff | Finance analyst | CFO/delegated threshold, GL policy | Currency, valuation, period, reconciliation |

### 12.1 Common issue object and response model

The no-blame coordination model from the plant-floor PRD is a core transactional service: Issue(id, tenant, plant/line/station scope, type, severity, started_at, owner, watchers, status, SLA, evidence IDs, action log). Anyone permitted may join/help; the owning responder closes with resolution evidence; critical disposition may require independent verification. Same issue ID appears in the operator HMI, quality, maintenance, line and plant dashboards. No private copies of “alerts” are maintained as independent truths.

## 13. Security, integration, and failure behavior

- Identity: OIDC enterprise SSO for office/management, badge/PIN with local signed session for station users, scoped service identities for APIs and mTLS on plant/cloud service links. Operator identities are protected; public displays use station tokens, not names.

- Authorization: central policy-as-code with signed/offline snapshots; per-plant roles and action scope. Controller/service key rotation and device attestation are audited. Local engine denies unsupported new obligations if WAN is down.

- External integrations: buyer EDI/CSV/PDF and ERP PLM APIs; accounting ERP for AP/AR and GL, attendance/time clocks, IoT gateway drivers, AQL/buyer specs, 3PL shipment response. All external references mapped with {source_system, entity_type, external_id, internal_uuid, effective_range}.

- API data contracts: versioned OpenAPI 3.x for synchronous commands/queries and AsyncAPI/JSON Schema or Protobuf for events; schema registry and consumer-driven compatibility checks run in CI. Breaking contract releases must have overlap/deprecation windows.

- Privacy and industrial data security: least privilege by tenant/site, field-level controls on workforce/pay, no public individual performance league table, cost-sensitive contract fields restricted, append-only audit and log signing/key rotation.

- Offline operations: demonstrate 8-hour WAN isolation with plant services healthy; station reconnect de-duplicates replay. If plant PostgreSQL/broker also fails, device buffering is only a pending intent, not an accepted movement. Recovery/redo flow must be documented.

- Data completeness: every event includes occurred_at, received_at, event UUID, causal identity and confidence. Clock drift, missing sensor, untrusted document extraction and unsigned exceptions cause explicit quality states.

- Supply-chain document security: guard against malicious PDF/Excel macros, untrusted links and embedded commands; extraction happens in isolated workers and cannot execute source instructions.

### 13.1 Failure-mode matrix

| Failure | Safety and state rule | User-facing response | Recovery |

| --- | --- | --- | --- |

| WAN outage | Plant authoritative services continue with approved rules | Plant green/amber; cloud shows stale time | Durable outbox replay + reconcile stream offsets |

| Reader duplicated scan | Idempotency + station debounce | Already scanned; no added credit | No manual cleanup required |

| Reader cannot decode | No identity-dependent progression | Rescan or approved physical label fallback | Provenance-labelled offline exception workflow |

| Wrong material or shade | No merge/credit | Visible reason + send help | Correct kit; signed waiver only if permitted |

| Partial split concurrent with merge | Optimistic aggregate version and deterministic lock ordering | Conflict; refresh live state | Retry with current version or supervisor review |

| Audit FAIL / NCR | Hold affected lot/cartons and downstream dispatch | Audit reject, owner and deadline | Rework/redo inspection; approved release |

| Packing service down | Cannot seal/manifest unverified carton | Seal pending, no printer or despatch release | HA restart; replay safe pending scans |

| Cloud optimization unavailable | Never blocks floor local execution | “Plan guidance unavailable”; baseline preserved | Rerun versioned simulation when ready |

| Bad or superseded OB | Reject route publish and new claims | Show source mismatch (fit/material/size) | IE + merchandising corrective revision |

| Incorrect financial conversion | No silent currency/tax inference | Margin unavailable with reason | Controller-reviewed rate/basis update |

## 14. Nonfunctional engineering targets and test criteria

These numbers are proposed engineering acceptance targets, not validated facts from the uploaded documents. Hardware, factory power and LAN conditions must be measured before contractually guaranteeing them.

| SLO / NFR | Target for pilot | Measurement / mandatory boundary |

| --- | --- | --- |

| Station feedback latency | p95 ≤150 ms and p99 ≤300 ms for local preflight/feedback when plant LAN healthy | Tap -> clear ACCEPT/DENY/PENDING HMI, tagged by test device |

| Plant transaction commit | p95 ≤500 ms for single-aggregate accepted station movement | Command -> owning PostgreSQL commit, distinct from display echo |

| Merge/conservation decision | p95 ≤750 ms; no duplicate parent consumption at 14+ lines | 100k concurrency/negative-control route events |

| Line dashboard staleness | 95% updates visible within 5 s after commit | State projection lag and “as of” always visible |

| Finance/group analytics | Freshness visible; target ≤5 min for critical rollups, ≤1 h for cohorts | Never used as motor/stock authority |

| Cloud isolation | 8 h WAN-off plant operation for approved routes with no accepted-event loss | Replay all buffered effects and check hash/counts |

| Business command reliability | Zero duplicate accepted stock, credited op, finished unit, bag, carton or manifest under retries | Duplicate/reorder/restart injection; identity tests |

| Data recovery | Plant RPO target 0 for committed records with supported HA; RTO target <1 h subject to site design | Power/persistence fault-injection and recovery rehearsal |

| Availability | ≥99.5% plant critical services measured monthly pilot; office analytics separately | Exclude measured planned maintenance; follow signed policy |

| Auditability | Every state-changing command has actor/device, rule rev, causal ID and recorded disposition | Trace any SKU/carton to originating lot and approved documents |

| Security | No cross-tenant read/write; all actions subject to scope and field policy | RBAC/RLS and negative API tests |

### 14.1 High-value acceptance tests: product and physical invariants

| Test | Acceptance behavior |

| --- | --- |

| A01 | Import PO REP1/REP2 separately; seven SKU quantities reproduce 129,000 combined units and stated order fields. |

| A02 | Import PO page-specific dates into distinct typed fields; flag conflicting operational milestone without reclassifying it. |

| A03 | Do not approve polyester/straight-fit OB for 220988 pleated cotton twill without signed reconciliation. |

| A04 | Resolve size-conditioned 7-loop pattern for ordered 30–42; do not introduce ≥44 route to current order. |

| A05 | Validate separate IZOD shirt route as a different style and packaging family. |

| A06 | Delete/archive document original cannot erase approved extracted assertions and source hash. |

| A07 | Publish RouteRevision; stations on previous revision finish in-flight work under explicit cutover policy. |

| A08 | Reject missing DAG prerequisite or cycle on route publish. |

| A09 | Require matching size, shade band and compatible lot/part type for merge; leave parents unchanged on rejection. |

| A10 | Split bundle: good 28 + rejected 2 from 30; sum preserved and no double credit. |

| A11 | Retire a reusable tag binding only after lineage transition persists; reuse tag without object alias collision. |

| A12 | Reject same UID active on two carriers; alert on suspect UHF duplicate presence. |

| Test | Acceptance behavior |

| --- | --- |

| A13 | Deduplicate 100 repeated taps for one single valid station command. |

| A14 | Quarantine physically implausible cycle if evidence policy fails; never substitute for proven fraud or output. |

| A15 | Record station stop/start and breaks; do not treat overnight gap as productive cycle time. |

| A16 | Receive roll by UHF and log measured length/width/defect map from actual metrology. |

| A17 | Prohibit consuming the same reserved roll twice across two cutting plans. |

| A18 | Cut actual plies different from planned: generate physical piece counts, not planned piece counts. |

| A19 | Issue/return using two authenticated witnesses; retain signed custody movement. |

| A20 | Quarantine failed inline QC portion while unaffected accepted pieces advance. |

| A21 | Alteration repaired unit returns to a new independent final inspection and cannot multiply good count. |

| A22 | One buyer garment identifier cannot be active for two garments; UPC alone cannot identify a single piece. |

| A23 | Each sealed polybag is exactly two accepted and distinct garment instances. |

| A24 | 24-piece carton seals with exactly 12 approved bags; 48-piece carton with exactly 24. |

| Test | Acceptance behavior |

| --- | --- |

| A25 | Wrong size/color, wrong PO and duplicate packing scans return typed rejection without increment. |

| A26 | REP1 size 42 residual 16 pieces requires buyer approval for a partial carton despite 1,096 printed case count. |

| A27 | REP2 size 42 residual 8 pieces cannot be auto-shipped in 2,568 declared full cases. |

| A28 | Audit sampled units are unique; lot FAIL blocks unapproved cartons from dispatch. |

| A29 | Manifest accepts exactly its released carton set; duplicate load is idempotent; wrong truck/carton denied. |

| A30 | OCR rejects any unexplained discrepancy across cut, WIP, loss, ship and approved overcut/leftover. |

| A31 | Closure signoffs from six owners refer to frozen aggregate versions; correction creates immutable addendum. |

| A32 | Long-duration WAN outage replays acknowledged events without duplicated committed quantities. |

| A33 | Plant DB outage shows pending buffered commands; cannot claim final state was committed. |

| A34 | Race two concurrent bundle merges and ensure same parent cannot be spent twice. |

| A35 | Race two concurrent carton scans and ensure membership count and serial uniqueness hold. |

| A36 | Delete/disable optimization service and confirm stations, packing and shipping still operate. |

| Test | Acceptance behavior |

| --- | --- |

| A37 | Deploy quality service upgrade independent of cutting code and UI; contract tests pass. |

| A38 | Simulate absent skill or incompatible machine; scheduling reports infeasible explanation. |

| A39 | Simulation recommendation never directly mutates route, staffing or machinery; approval recorded. |

| A40 | Executive numeric question returns unit, SQL/metric ID, source time and confidence; no invented number. |

| A41 | Unauthorized worker cannot approve a cut overage, carton waiver or finance writeoff. |

| A42 | Buyer document packet contains effective pack rule, manifest, buyer-specific paperwork and signed exceptions. |

| A43 | Each of the registered UI routes is permission-checked and renders unavailable optional module gracefully. |

| A44 | Metric DHU/FPY/efficiency report exact denominator, sample and accepted event/reinspection semantics. |

| A45 | NTP clock drift reported and cannot create impossible negative duration silently. |

| A46 | When quality rejects upstream defect cluster, optimizer generates quarantined/recovery scenario without committing it. |

| A47 | Finance P&L never interprets source $6.30 PO amount as verified contribution margin. |

| A48 | Reconciliation after replay returns same counts and hashes as original ledger across restart. |

## 15. Engineering delivery plan: how to actually build it

A 29-service/100+-screen factory OS should be delivered by complete operational value streams, not by implementing every service skeleton and blank dashboard at once. Each phase yields a plant-testable subset with real service contracts, a user-facing workflow, security, observability and verified evidence. Do not equate number of containers with useful functionality.

| Increment | Deploy services / UI first | Exit test / operational value |

| --- | --- | --- |

| Foundation — 0–8 weeks (illustrative) | S01–S06 + S28; Plant identity/roles, docs/spec/OB/order; initial S24, office-bff | Import real PO/techpack; compile/approve route; full document provenance and conflict review |

| Traceable materials — next 8–12 weeks | S10–S13 + S02/S11–12; stores/cut line; floor-bff | Receive/measure/tag roll; allocate, spread, cut, commission and reconcile real bundles |

| Sew and quality — next 10–14 weeks | S14–S17, S23; station kernel; SEW/QS/QM/PM essential routes | 14-line station throughput and quality; split/merge/rework/offline/issue tests |

| Finish-to-cash — next 10–14 weeks | S18–S22; finishing, pack, dock, OCR, finance handoff | Buyer bag/carton/manifest invariants; order stage closure and cost feed |

| Closed-loop optimization — after reliable baselines | S07–S09, S25–S27, S29 where warranted; planning and executive UI | Constrained simulations, intervention approval, measured economic delta; optional multi-plant |

**Scheduling caveat** Durations above are planning hypotheses for a dedicated cross-functional team with access to plant SMEs and hardware; actual commitments depend on verified integrations, device procurement, accepted source OB, safety tests and acceptance-lab throughput. Pilot priority can reorder modules while preserving contract-first independence.

### 15.1 Repository and build contract

```
agiparel/
  contracts/                openapi/, events/, domain-types/, compat-tests/
  services/
    s01-identity/...s29-federation/  each has migrations + OpenAPI + outbox + tests
  bff/                      floor/, supervisor/, office/, executive/
  edge-kernel/              reader drivers, cache, sync, rule evaluation, HMI
  web/                      role routes keyed by screen ID and entitlement
  migrations/               per-service migration manifests (no shared SQL writes)
  packages/                 design tokens, error codes, units, trace context
  infrastructure/           plant/ and cloud/ IaC, secrets, observability
  fixtures/                 walmart-220988, izod-45pw207, negative-cases
  e2e/                      stage-by-stage conservation and offline contract tests
  ops/                      plant incident, rollout, rollback and disaster recovery
```

- Every service repo: domain owner and invariants, API/event schema, migrations, health/metrics endpoints, threat model, offline behavior, test fixtures, idempotency/retry expectations, minimum role grants, and canary rollback playbook.

- Service readiness: autonomous deploy to staging, back-compat contract suite, 24-hour replay/load soak, failure injection, ownership/cost metrics, and user acceptance on real sample order at plant pilot.

- UX readiness: each screen in §5 has unique route, permission, source domain, primary action, empty/loading/conflict/offline states, measurable happy and negative-path acceptance tests and mobile ergonomics where specified.

- Business rules maintained as signed, versioned product policies in DB; frontend never implements unique packaging/route/quality logic as the sole gate.

- No more than one writer is authorized for the same aggregate, including cloud-sync changes. Ownership transfer itself must be a designed command, not two services racing to reconcile records.

### 15.2 Engineering team boundaries and operational responsibilities

| Team / owner | Deliverable | Key dependency |

| --- | --- | --- |

| Platform / reliability | Plant Kubernetes or lighter isolated container runtime; local PG/NATS, security, CI/CD, edge sync | Safety-approved hardware and reliable intranet |

| Product/domain data | Documents, PO/style/BOM/OB, data keys, contracts and normalization | Merchandising, IE and buyer data owners |

| Shop-floor controls | Driver adapters, station kernel, RFID/lineage, sewing and maintenance | IE route revision; physical reader pilot |

| Materials and cut | Inspection/metrology, inventory ledger, cut/lay/BOM reconciliation | Fabric measurement/marker source validation |

| Quality, finishing, pack | Inspection/redo, buyer ID, bag/carton, audit, loader | Buyer pack policy and QC signoff |

| Planning, economics, intelligence | APS/MRP, ClickHouse metrics, digital twin, ROI evaluation | Accepted flow evidence and finance rate cards |

| Role UX | Station, supervisor, management and office workspaces + design system | Stable BFFs, service-issued command decisions |

| Integration / testing | EDI/ERP/finance, golden PO regression tests, fault injection, cyber audit | Real documents, known external IDs, plant UAT |

### 15.3 Implementation readiness gates

- G1 — Business: operations leaders approve the normalized order/stage terminology, screen register, module packaging and source-specific document precedence.

- G2 — Technical: IE validates an actual production-applicable 220988 OB revision; engineering establishes BOM/route graph correctness and material/size branches; buyer resolves partial-carton authority.

- G3 — Hardware: HF/NFC reader placement, UID behavior, latency, bulk UHF portals, encoder accuracy and scanner/scale compatibility are characterized on live sewing and packing stations.

- G4 — Data: ERP/PLM/finance mapping and idempotent import boundaries are approved; who owns buyer PO price basis and each delivery milestone is documented.

- G5 — Reliability: local offline/replay/HA tests complete; physical safety review clears any machine interface; signed operator access and privacy controls verified.

- G6 — Pilot: output/quality/stage counts match independent physical tally and golden order tests; planned vs actual improvement is reported with uncertainty, not marketing uplift.

## 16. Source register and scope of authority

Engineering source references below identify the actual supplied artifacts and why they matter. They are not a history of prior document versions. The factory-specific PO/techpack are binding only to the appropriate order and buyer revision; the two PRD/architecture documents supply product requirements and architectural patterns. Explicit source conflicts remain controlled holds, not silent “corrections.”

| Source document | Facts / requirements taken from it | Use in engineering |

| --- | --- | --- |

| Pleated Twill Pant_220988_PO Sheet.pdf (4 pp.) | REP1 / REP2 quantities, seven ordered sizes, pack 2 per RFID polybag, 24/48 solid-size cartons, case counts, value/date/ship fields | Golden order fixture, packing and shipment acceptance |

| Pleated Twill Pant_220988 Tech Pack.pdf (27 pp.) | Approved George trousers product BOM, graded measurements, 7-vs-9-loop construction diagrams and care/label details | Style revision, POM, variant-rich product/QA model |

| Pleated Twill Pant 220988 (OB).xls | User-provided OB example; prior ARD records fit/material naming mismatch with 220988 PO and tech pack | Quarantined OB import/route validation; requires source-owner approval before execution |

| 45PW207 — IZOD spring 2020 shirt tech pack (24 pp.) | Distinct shirt/wash/press/hanger packaging requirements | Separate product-family conditional-flow test |

| PRD_PlantFloor_PlantManager_Quality.docx (57 pp.) | Role UX, original ~70 screen vision, explicit PM/QS/QM/FH/PK/OC/OB views, no-blame issues and action workflows | Source-compatible UI IDs and workflow requirements |

| End-to-End Architecture RFID Garment Manufacturing ... .docx (20 pp.) | Intentional HF IN/OUT, UHF bulk reading, split/merge tags, machine evidence, offline events, flow-time components | Identity/edge/evidence and genealogy tests |

| Agiparel PRD-ARD Rev 1.1.docx (40 pp.) | S-M-O principles, material/cut/sew/finishing/pack/closure flows, additional STR/CUT/PLN/CST/EXEC screens and operations taxonomy | Extended business capability baseline; engineering risk hypotheses treated as proposals |

**Claims discipline** The target 29-service topology, storage design, SQL DDL, detailed UX count, refresh/availability SLOs, timescale and phase plan are proposed engineering decisions here. They need project design signoff and physical testing; the files are not evidence that these implementations already exist or perform to these levels.