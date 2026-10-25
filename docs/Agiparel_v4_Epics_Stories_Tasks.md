# AGIPAREL v4 — DEVELOPMENT EPICS, STORIES AND TASKS

**Owner:** Sunil Jain, CyberinQ LLC  
**Date:** 25 October 2026  
**Execution model:** Individual file-level changes; vertical business transactions; brownfield observation before authority transfer.  

## Backlog rules

- **P0** enables safe MVP/pilot; **P1** closes commercial and optimization loops; **P2** differentiates later. Priority is not a promise of calendar duration.
- Each story status starts **SOURCE AUDIT NEEDED**. Test coverage never establishes feature absence. Inspect live source/API/data/UI before marking IMPLEMENTED, PARTIAL, STUB or ABSENT.
- Estimate with T-shirt sizes after code audit; put exact source paths in delivery tickets, rather than guessing filenames.
- Every story must deliver compatible migration, API, event/decision audit, operator-facing change and failure/retry handling if relevant.
- A story is complete only when a plant worker or manager can perform its intended action end-to-end and obtain an independently checkable outcome.
- Uncontrolled hardware interactions and unapproved AI recommendations are not allowed.

## Release increments

| Increment | Epics / critical slice | Exit condition |
| --- | --- | --- |
| R0 — Observe without disruption | E01 E02 E16 E17 E18 | Observe running factory; no legacy behavior modified; reconciliation visible |
| R1 — From buyer document to answerable plan | E03 E04 + E05 inventory snapshot | Manager uploads approved PO/tech pack, answers blockers and compares feasible options |
| R2 — Save fabric and cut smarter | E05 E06 + marker integration | Compare heterogeneous roll and ply plans; actual lay conserved |
| R3 — Find every kit and complete garment | E07 E08 E09 E10 | UHF/NFC planned scheme, reversible genealogy, credible cycle evidence, QC gates |
| R4 — Ship what was really made | E11 E12 | Bag/carton/manifest/OCR coherent; source and cost traceable |
| R5 — Improve and prove it | E13 E14 E15 | Resilient replan and A/B verified outcomes with uncertainty |

## Stories and implementation tasks

### E01 — Brownfield adoption and system authority
**Priority:** P0 · **Logical owners:** S28 / S01 / core · **Acceptance references:** B01,B03

#### E01-S01 — Map legacy systems and data feeds
**Story:** As a relevant factory user, I need to identify active ERP, scans, spreadsheets, output owners and historical IDs, so that work is explainable, safe and actionable.
**Acceptance:** Two formats mapped; source evidence retained.
**Dependencies:** Prior schema/source contracts in S28 / S01 / core; existing installed legacy and device capabilities must be discovered.
**Delivery tasks:**

- **E01-S01-T01 Domain/API:** Define source-of-truth, typed inputs/outputs, idempotency, permissions, state transitions and failure contracts.
- **E01-S01-T02 Implementation/UI:** Implement owning logic and integration adapter, expose a role-appropriate action and clear conflict/pending/approved states.
- **E01-S01-T03 Evidence/pilot:** Run positive, negative, replay and business-invariant checks using a named pilot fixture; capture acceptance evidence and rollback procedure.

#### E01-S02 — Implement shadow observation mode
**Story:** As a relevant factory user, I need to capture reads/imports without creating operational writes, so that work is explainable, safe and actionable.
**Acceptance:** Legacy outputs unchanged after full pilot shift.
**Dependencies:** Prior schema/source contracts in S28 / S01 / core; existing installed legacy and device capabilities must be discovered.
**Delivery tasks:**

- **E01-S02-T01 Domain/API:** Define source-of-truth, typed inputs/outputs, idempotency, permissions, state transitions and failure contracts.
- **E01-S02-T02 Implementation/UI:** Implement owning logic and integration adapter, expose a role-appropriate action and clear conflict/pending/approved states.
- **E01-S02-T03 Evidence/pilot:** Run positive, negative, replay and business-invariant checks using a named pilot fixture; capture acceptance evidence and rollback procedure.

#### E01-S03 — Introduce scoped ownership transitions
**Story:** As a relevant factory user, I need to define per-object scope, authority leases, handoff and rollback, so that work is explainable, safe and actionable.
**Acceptance:** At most one authoritative writer at every transition.
**Dependencies:** Prior schema/source contracts in S28 / S01 / core; existing installed legacy and device capabilities must be discovered.
**Delivery tasks:**

- **E01-S03-T01 Domain/API:** Define source-of-truth, typed inputs/outputs, idempotency, permissions, state transitions and failure contracts.
- **E01-S03-T02 Implementation/UI:** Implement owning logic and integration adapter, expose a role-appropriate action and clear conflict/pending/approved states.
- **E01-S03-T03 Evidence/pilot:** Run positive, negative, replay and business-invariant checks using a named pilot fixture; capture acceptance evidence and rollback procedure.

#### E01-S04 — Build reconciliation exception desk
**Story:** As a relevant factory user, I need to compare legacy claims and observed/accepted facts with human resolution, so that work is explainable, safe and actionable.
**Acceptance:** Every difference has owner, reason and source.
**Dependencies:** Prior schema/source contracts in S28 / S01 / core; existing installed legacy and device capabilities must be discovered.
**Delivery tasks:**

- **E01-S04-T01 Domain/API:** Define source-of-truth, typed inputs/outputs, idempotency, permissions, state transitions and failure contracts.
- **E01-S04-T02 Implementation/UI:** Implement owning logic and integration adapter, expose a role-appropriate action and clear conflict/pending/approved states.
- **E01-S04-T03 Evidence/pilot:** Run positive, negative, replay and business-invariant checks using a named pilot fixture; capture acceptance evidence and rollback procedure.

### E02 — Legacy adapters, master data and ingestion
**Priority:** P0 · **Logical owners:** S28 / S03 · **Acceptance references:** B02

#### E02-S01 — Build safe import adapter framework
**Story:** As a relevant factory user, I need to cSV XLSX EDI REST file jobs with mapping version and failure queue, so that work is explainable, safe and actionable.
**Acceptance:** Retries yield same canonical records.
**Dependencies:** Prior schema/source contracts in S28 / S03; existing installed legacy and device capabilities must be discovered.
**Delivery tasks:**

- **E02-S01-T01 Domain/API:** Define source-of-truth, typed inputs/outputs, idempotency, permissions, state transitions and failure contracts.
- **E02-S01-T02 Implementation/UI:** Implement owning logic and integration adapter, expose a role-appropriate action and clear conflict/pending/approved states.
- **E02-S01-T03 Evidence/pilot:** Run positive, negative, replay and business-invariant checks using a named pilot fixture; capture acceptance evidence and rollback procedure.

#### E02-S02 — Create external ID resolver
**Story:** As a relevant factory user, I need to canonical UUID aliases and effective-dated mappings, so that work is explainable, safe and actionable.
**Acceptance:** No silent conflation of PO/style/roll IDs.
**Dependencies:** Prior schema/source contracts in S28 / S03; existing installed legacy and device capabilities must be discovered.
**Delivery tasks:**

- **E02-S02-T01 Domain/API:** Define source-of-truth, typed inputs/outputs, idempotency, permissions, state transitions and failure contracts.
- **E02-S02-T02 Implementation/UI:** Implement owning logic and integration adapter, expose a role-appropriate action and clear conflict/pending/approved states.
- **E02-S02-T03 Evidence/pilot:** Run positive, negative, replay and business-invariant checks using a named pilot fixture; capture acceptance evidence and rollback procedure.

#### E02-S03 — Build material and machine master import
**Story:** As a relevant factory user, I need to normalize widths units machine types schedules and skill references, so that work is explainable, safe and actionable.
**Acceptance:** Critical units validated and reversible.
**Dependencies:** Prior schema/source contracts in S28 / S03; existing installed legacy and device capabilities must be discovered.
**Delivery tasks:**

- **E02-S03-T01 Domain/API:** Define source-of-truth, typed inputs/outputs, idempotency, permissions, state transitions and failure contracts.
- **E02-S03-T02 Implementation/UI:** Implement owning logic and integration adapter, expose a role-appropriate action and clear conflict/pending/approved states.
- **E02-S03-T03 Evidence/pilot:** Run positive, negative, replay and business-invariant checks using a named pilot fixture; capture acceptance evidence and rollback procedure.

#### E02-S04 — Instrument connector health and backfill
**Story:** As a relevant factory user, I need to freshness latency errors and source completeness displayed, so that work is explainable, safe and actionable.
**Acceptance:** Disconnected source marked stale with recovery job.
**Dependencies:** Prior schema/source contracts in S28 / S03; existing installed legacy and device capabilities must be discovered.
**Delivery tasks:**

- **E02-S04-T01 Domain/API:** Define source-of-truth, typed inputs/outputs, idempotency, permissions, state transitions and failure contracts.
- **E02-S04-T02 Implementation/UI:** Implement owning logic and integration adapter, expose a role-appropriate action and clear conflict/pending/approved states.
- **E02-S04-T03 Evidence/pilot:** Run positive, negative, replay and business-invariant checks using a named pilot fixture; capture acceptance evidence and rollback procedure.

### E03 — Document intelligence and versioned knowledge graph
**Priority:** P0 · **Logical owners:** S03 / S04 / S05 · **Acceptance references:** D01

#### E03-S01 — Extract buyer files into proposed facts
**Story:** As a relevant factory user, I need to classify PDF XLSX images tables drawings and cite page regions, so that work is explainable, safe and actionable.
**Acceptance:** Every critical asserted field has source pointer.
**Dependencies:** Prior schema/source contracts in S03 / S04 / S05; existing installed legacy and device capabilities must be discovered.
**Delivery tasks:**

- **E03-S01-T01 Domain/API:** Define source-of-truth, typed inputs/outputs, idempotency, permissions, state transitions and failure contracts.
- **E03-S01-T02 Implementation/UI:** Implement owning logic and integration adapter, expose a role-appropriate action and clear conflict/pending/approved states.
- **E03-S01-T03 Evidence/pilot:** Run positive, negative, replay and business-invariant checks using a named pilot fixture; capture acceptance evidence and rollback procedure.

#### E03-S02 — Build typed document relationships
**Story:** As a relevant factory user, I need to connect buyer PO line style fabric BOM route machine pack and terms, so that work is explainable, safe and actionable.
**Acceptance:** Graph detects inconsistent identifiers and revisions.
**Dependencies:** Prior schema/source contracts in S03 / S04 / S05; existing installed legacy and device capabilities must be discovered.
**Delivery tasks:**

- **E03-S02-T01 Domain/API:** Define source-of-truth, typed inputs/outputs, idempotency, permissions, state transitions and failure contracts.
- **E03-S02-T02 Implementation/UI:** Implement owning logic and integration adapter, expose a role-appropriate action and clear conflict/pending/approved states.
- **E03-S02-T03 Evidence/pilot:** Run positive, negative, replay and business-invariant checks using a named pilot fixture; capture acceptance evidence and rollback procedure.

#### E03-S03 — Implement conflict and approval workflow
**Story:** As a relevant factory user, I need to field-scoped precedence with owner signatures and effective intervals, so that work is explainable, safe and actionable.
**Acceptance:** Wrong-fit or material OB cannot auto-publish.
**Dependencies:** Prior schema/source contracts in S03 / S04 / S05; existing installed legacy and device capabilities must be discovered.
**Delivery tasks:**

- **E03-S03-T01 Domain/API:** Define source-of-truth, typed inputs/outputs, idempotency, permissions, state transitions and failure contracts.
- **E03-S03-T02 Implementation/UI:** Implement owning logic and integration adapter, expose a role-appropriate action and clear conflict/pending/approved states.
- **E03-S03-T03 Evidence/pilot:** Run positive, negative, replay and business-invariant checks using a named pilot fixture; capture acceptance evidence and rollback procedure.

#### E03-S04 — Create revision impact analyzer
**Story:** As a relevant factory user, I need to identify stock plans active lays kits and labels touched by amendment, so that work is explainable, safe and actionable.
**Acceptance:** Changes produce hold/impact report without mutation.
**Dependencies:** Prior schema/source contracts in S03 / S04 / S05; existing installed legacy and device capabilities must be discovered.
**Delivery tasks:**

- **E03-S04-T01 Domain/API:** Define source-of-truth, typed inputs/outputs, idempotency, permissions, state transitions and failure contracts.
- **E03-S04-T02 Implementation/UI:** Implement owning logic and integration adapter, expose a role-appropriate action and clear conflict/pending/approved states.
- **E03-S04-T03 Evidence/pilot:** Run positive, negative, replay and business-invariant checks using a named pilot fixture; capture acceptance evidence and rollback procedure.

### E04 — Interactive feasibility copilot
**Priority:** P0 · **Logical owners:** S03 / S08 / S09 / AGIE · **Acceptance references:** D02,D03

#### E04-S01 — Build PlanningSnapshot aggregator
**Story:** As a relevant factory user, I need to join source evidence to on-hand reserved stock capacity skills machines due dates and finance, so that work is explainable, safe and actionable.
**Acceptance:** Snapshot pins freshness and exact owner.
**Dependencies:** Prior schema/source contracts in S03 / S08 / S09 / AGIE; existing installed legacy and device capabilities must be discovered.
**Delivery tasks:**

- **E04-S01-T01 Domain/API:** Define source-of-truth, typed inputs/outputs, idempotency, permissions, state transitions and failure contracts.
- **E04-S01-T02 Implementation/UI:** Implement owning logic and integration adapter, expose a role-appropriate action and clear conflict/pending/approved states.
- **E04-S01-T03 Evidence/pilot:** Run positive, negative, replay and business-invariant checks using a named pilot fixture; capture acceptance evidence and rollback procedure.

#### E04-S02 — Compute baseline feasible or impossible status
**Story:** As a relevant factory user, I need to validate hard constraints before showing dates or profit, so that work is explainable, safe and actionable.
**Acceptance:** No-plan result includes blocking explanation.
**Dependencies:** Prior schema/source contracts in S03 / S08 / S09 / AGIE; existing installed legacy and device capabilities must be discovered.
**Delivery tasks:**

- **E04-S02-T01 Domain/API:** Define source-of-truth, typed inputs/outputs, idempotency, permissions, state transitions and failure contracts.
- **E04-S02-T02 Implementation/UI:** Implement owning logic and integration adapter, expose a role-appropriate action and clear conflict/pending/approved states.
- **E04-S02-T03 Evidence/pilot:** Run positive, negative, replay and business-invariant checks using a named pilot fixture; capture acceptance evidence and rollback procedure.

#### E04-S03 — Generate ranked clarification questions
**Story:** As a relevant factory user, I need to ask only unknowns that change feasible candidate plan, so that work is explainable, safe and actionable.
**Acceptance:** Question includes tradeoffs options and role.
**Dependencies:** Prior schema/source contracts in S03 / S08 / S09 / AGIE; existing installed legacy and device capabilities must be discovered.
**Delivery tasks:**

- **E04-S03-T01 Domain/API:** Define source-of-truth, typed inputs/outputs, idempotency, permissions, state transitions and failure contracts.
- **E04-S03-T02 Implementation/UI:** Implement owning logic and integration adapter, expose a role-appropriate action and clear conflict/pending/approved states.
- **E04-S03-T03 Evidence/pilot:** Run positive, negative, replay and business-invariant checks using a named pilot fixture; capture acceptance evidence and rollback procedure.

#### E04-S04 — Publish controlled plan from conversation
**Story:** As a relevant factory user, I need to approve answers and compare multiple scenario versions, so that work is explainable, safe and actionable.
**Acceptance:** Chat cannot silently amend commercial facts.
**Dependencies:** Prior schema/source contracts in S03 / S08 / S09 / AGIE; existing installed legacy and device capabilities must be discovered.
**Delivery tasks:**

- **E04-S04-T01 Domain/API:** Define source-of-truth, typed inputs/outputs, idempotency, permissions, state transitions and failure contracts.
- **E04-S04-T02 Implementation/UI:** Implement owning logic and integration adapter, expose a role-appropriate action and clear conflict/pending/approved states.
- **E04-S04-T03 Evidence/pilot:** Run positive, negative, replay and business-invariant checks using a named pilot fixture; capture acceptance evidence and rollback procedure.

### E05 — Material roll passport and stock truth
**Priority:** P0 · **Logical owners:** S11 / S12 · **Acceptance references:** C03

#### E05-S01 — Define roll and fabric segment schema
**Story:** As a relevant factory user, I need to track width-by-length defects shade GSM composition weave yarn count and cost, so that work is explainable, safe and actionable.
**Acceptance:** Measured vs mill-claimed fields distinct.
**Dependencies:** Prior schema/source contracts in S11 / S12; existing installed legacy and device capabilities must be discovered.
**Delivery tasks:**

- **E05-S01-T01 Domain/API:** Define source-of-truth, typed inputs/outputs, idempotency, permissions, state transitions and failure contracts.
- **E05-S01-T02 Implementation/UI:** Implement owning logic and integration adapter, expose a role-appropriate action and clear conflict/pending/approved states.
- **E05-S01-T03 Evidence/pilot:** Run positive, negative, replay and business-invariant checks using a named pilot fixture; capture acceptance evidence and rollback procedure.

#### E05-S02 — Capture GRN inspection and calibration
**Story:** As a relevant factory user, I need to onboard UHF rolls and width length shade defects with calibrated source, so that work is explainable, safe and actionable.
**Acceptance:** Quarantine bad measurements.
**Dependencies:** Prior schema/source contracts in S11 / S12; existing installed legacy and device capabilities must be discovered.
**Delivery tasks:**

- **E05-S02-T01 Domain/API:** Define source-of-truth, typed inputs/outputs, idempotency, permissions, state transitions and failure contracts.
- **E05-S02-T02 Implementation/UI:** Implement owning logic and integration adapter, expose a role-appropriate action and clear conflict/pending/approved states.
- **E05-S02-T03 Evidence/pilot:** Run positive, negative, replay and business-invariant checks using a named pilot fixture; capture acceptance evidence and rollback procedure.

#### E05-S03 — Implement slice reservation and split returns
**Story:** As a relevant factory user, I need to reserve partial lengths across compatible orders with atomic locking, so that work is explainable, safe and actionable.
**Acceptance:** Cannot spend one centimeter twice.
**Dependencies:** Prior schema/source contracts in S11 / S12; existing installed legacy and device capabilities must be discovered.
**Delivery tasks:**

- **E05-S03-T01 Domain/API:** Define source-of-truth, typed inputs/outputs, idempotency, permissions, state transitions and failure contracts.
- **E05-S03-T02 Implementation/UI:** Implement owning logic and integration adapter, expose a role-appropriate action and clear conflict/pending/approved states.
- **E05-S03-T03 Evidence/pilot:** Run positive, negative, replay and business-invariant checks using a named pilot fixture; capture acceptance evidence and rollback procedure.

#### E05-S04 — Implement material moves and remnants
**Story:** As a relevant factory user, I need to trace transfers partial consumption and reusable remnants, so that work is explainable, safe and actionable.
**Acceptance:** Sum of movements reconciles per unit.
**Dependencies:** Prior schema/source contracts in S11 / S12; existing installed legacy and device capabilities must be discovered.
**Delivery tasks:**

- **E05-S04-T01 Domain/API:** Define source-of-truth, typed inputs/outputs, idempotency, permissions, state transitions and failure contracts.
- **E05-S04-T02 Implementation/UI:** Implement owning logic and integration adapter, expose a role-appropriate action and clear conflict/pending/approved states.
- **E05-S04-T03 Evidence/pilot:** Run positive, negative, replay and business-invariant checks using a named pilot fixture; capture acceptance evidence and rollback procedure.

### E06 — High-level cutting campaign optimization
**Priority:** P0 · **Logical owners:** S08 / S13 · **Acceptance references:** C01,C02,C04

#### E06-S01 — Connect CAD/Gerber marker candidate interface
**Story:** As a relevant factory user, I need to read approved marker yield width size ratios revision and tool metadata, so that work is explainable, safe and actionable.
**Acceptance:** No unsupported control commands sent to cutter.
**Dependencies:** Prior schema/source contracts in S08 / S13; existing installed legacy and device capabilities must be discovered.
**Delivery tasks:**

- **E06-S01-T01 Domain/API:** Define source-of-truth, typed inputs/outputs, idempotency, permissions, state transitions and failure contracts.
- **E06-S01-T02 Implementation/UI:** Implement owning logic and integration adapter, expose a role-appropriate action and clear conflict/pending/approved states.
- **E06-S01-T03 Evidence/pilot:** Run positive, negative, replay and business-invariant checks using a named pilot fixture; capture acceptance evidence and rollback procedure.

#### E06-S02 — Build heterogeneous roll allocation optimizer
**Story:** As a relevant factory user, I need to choose segments widths plies marker families and PO ratios under hard constraints, so that work is explainable, safe and actionable.
**Acceptance:** At least three justified plans and infeasible reasons.
**Dependencies:** Prior schema/source contracts in S08 / S13; existing installed legacy and device capabilities must be discovered.
**Delivery tasks:**

- **E06-S02-T01 Domain/API:** Define source-of-truth, typed inputs/outputs, idempotency, permissions, state transitions and failure contracts.
- **E06-S02-T02 Implementation/UI:** Implement owning logic and integration adapter, expose a role-appropriate action and clear conflict/pending/approved states.
- **E06-S02-T03 Evidence/pilot:** Run positive, negative, replay and business-invariant checks using a named pilot fixture; capture acceptance evidence and rollback procedure.

#### E06-S03 — Schedule lays and cut-table availability
**Story:** As a relevant factory user, I need to respect knife stack height maintenance people shift and changeovers, so that work is explainable, safe and actionable.
**Acceptance:** Gantt and list show critical constraints.
**Dependencies:** Prior schema/source contracts in S08 / S13; existing installed legacy and device capabilities must be discovered.
**Delivery tasks:**

- **E06-S03-T01 Domain/API:** Define source-of-truth, typed inputs/outputs, idempotency, permissions, state transitions and failure contracts.
- **E06-S03-T02 Implementation/UI:** Implement owning logic and integration adapter, expose a role-appropriate action and clear conflict/pending/approved states.
- **E06-S03-T03 Evidence/pilot:** Run positive, negative, replay and business-invariant checks using a named pilot fixture; capture acceptance evidence and rollback procedure.

#### E06-S04 — Commit actual cut outcomes
**Story:** As a relevant factory user, I need to record actual lay roll use cut quantity scrap and residual segments, so that work is explainable, safe and actionable.
**Acceptance:** Physical actuals do not equal planned by default.
**Dependencies:** Prior schema/source contracts in S08 / S13; existing installed legacy and device capabilities must be discovered.
**Delivery tasks:**

- **E06-S04-T01 Domain/API:** Define source-of-truth, typed inputs/outputs, idempotency, permissions, state transitions and failure contracts.
- **E06-S04-T02 Implementation/UI:** Implement owning logic and integration adapter, expose a role-appropriate action and clear conflict/pending/approved states.
- **E06-S04-T03 Evidence/pilot:** Run positive, negative, replay and business-invariant checks using a named pilot fixture; capture acceptance evidence and rollback procedure.

### E07 — Tag hardware and lifecycle
**Priority:** P0 · **Logical owners:** S02 / S11 / RFID · **Acceptance references:** R01,R03,R04

#### E07-S01 — Build UHF EPC/NFC encoding registry
**Story:** As a relevant factory user, I need to scheme version write permissions readback and fallback labels, so that work is explainable, safe and actionable.
**Acceptance:** Tag memory profile validated before write.
**Dependencies:** Prior schema/source contracts in S02 / S11 / RFID; existing installed legacy and device capabilities must be discovered.
**Delivery tasks:**

- **E07-S01-T01 Domain/API:** Define source-of-truth, typed inputs/outputs, idempotency, permissions, state transitions and failure contracts.
- **E07-S01-T02 Implementation/UI:** Implement owning logic and integration adapter, expose a role-appropriate action and clear conflict/pending/approved states.
- **E07-S01-T03 Evidence/pilot:** Run positive, negative, replay and business-invariant checks using a named pilot fixture; capture acceptance evidence and rollback procedure.

#### E07-S02 — Implement tag provision and verify
**Story:** As a relevant factory user, I need to provision inventory verify and quarantine damaged duplicate unknown UIDs, so that work is explainable, safe and actionable.
**Acceptance:** No active double binding.
**Dependencies:** Prior schema/source contracts in S02 / S11 / RFID; existing installed legacy and device capabilities must be discovered.
**Delivery tasks:**

- **E07-S02-T01 Domain/API:** Define source-of-truth, typed inputs/outputs, idempotency, permissions, state transitions and failure contracts.
- **E07-S02-T02 Implementation/UI:** Implement owning logic and integration adapter, expose a role-appropriate action and clear conflict/pending/approved states.
- **E07-S02-T03 Evidence/pilot:** Run positive, negative, replay and business-invariant checks using a named pilot fixture; capture acceptance evidence and rollback procedure.

#### E07-S03 — Implement binding retirement and reuse
**Story:** As a relevant factory user, I need to unbind only after committed genealogy; inspect erase verify pool status, so that work is explainable, safe and actionable.
**Acceptance:** Reused UID never changes prior object history.
**Dependencies:** Prior schema/source contracts in S02 / S11 / RFID; existing installed legacy and device capabilities must be discovered.
**Delivery tasks:**

- **E07-S03-T01 Domain/API:** Define source-of-truth, typed inputs/outputs, idempotency, permissions, state transitions and failure contracts.
- **E07-S03-T02 Implementation/UI:** Implement owning logic and integration adapter, expose a role-appropriate action and clear conflict/pending/approved states.
- **E07-S03-T03 Evidence/pilot:** Run positive, negative, replay and business-invariant checks using a named pilot fixture; capture acceptance evidence and rollback procedure.

#### E07-S04 — Commission read zones and location confidence
**Story:** As a relevant factory user, I need to tune portals handheld UHF sighting confidence and staleness, so that work is explainable, safe and actionable.
**Acceptance:** Find feature never invents exact position.
**Dependencies:** Prior schema/source contracts in S02 / S11 / RFID; existing installed legacy and device capabilities must be discovered.
**Delivery tasks:**

- **E07-S04-T01 Domain/API:** Define source-of-truth, typed inputs/outputs, idempotency, permissions, state transitions and failure contracts.
- **E07-S04-T02 Implementation/UI:** Implement owning logic and integration adapter, expose a role-appropriate action and clear conflict/pending/approved states.
- **E07-S04-T03 Evidence/pilot:** Run positive, negative, replay and business-invariant checks using a named pilot fixture; capture acceptance evidence and rollback procedure.

### E08 — Sub-bundle and individual-part traceability
**Priority:** P0 · **Logical owners:** S13 / S14 / attachment · **Acceptance references:** R02,G01

#### E08-S01 — Model variable bundle/sub-bundle memberships
**Story:** As a relevant factory user, I need to permit arbitrary part types sizes and counts with compatible routes, so that work is explainable, safe and actionable.
**Acceptance:** No equal count assumption.
**Dependencies:** Prior schema/source contracts in S13 / S14 / attachment; existing installed legacy and device capabilities must be discovered.
**Delivery tasks:**

- **E08-S01-T01 Domain/API:** Define source-of-truth, typed inputs/outputs, idempotency, permissions, state transitions and failure contracts.
- **E08-S01-T02 Implementation/UI:** Implement owning logic and integration adapter, expose a role-appropriate action and clear conflict/pending/approved states.
- **E08-S01-T03 Evidence/pilot:** Run positive, negative, replay and business-invariant checks using a named pilot fixture; capture acceptance evidence and rollback procedure.

#### E08-S02 — Introduce NFC part-level pilot
**Story:** As a relevant factory user, I need to tap part or tagged fixture with role and operation context, so that work is explainable, safe and actionable.
**Acceptance:** Pilot reports per-piece burden tag failure and false reads.
**Dependencies:** Prior schema/source contracts in S13 / S14 / attachment; existing installed legacy and device capabilities must be discovered.
**Delivery tasks:**

- **E08-S02-T01 Domain/API:** Define source-of-truth, typed inputs/outputs, idempotency, permissions, state transitions and failure contracts.
- **E08-S02-T02 Implementation/UI:** Implement owning logic and integration adapter, expose a role-appropriate action and clear conflict/pending/approved states.
- **E08-S02-T03 Evidence/pilot:** Run positive, negative, replay and business-invariant checks using a named pilot fixture; capture acceptance evidence and rollback procedure.

#### E08-S03 — Build atomic split merge transformation ledger
**Story:** As a relevant factory user, I need to lock parent inputs and commit typed outputs scrap holds and event, so that work is explainable, safe and actionable.
**Acceptance:** Exact count per part type on split/merge.
**Dependencies:** Prior schema/source contracts in S13 / S14 / attachment; existing installed legacy and device capabilities must be discovered.
**Delivery tasks:**

- **E08-S03-T01 Domain/API:** Define source-of-truth, typed inputs/outputs, idempotency, permissions, state transitions and failure contracts.
- **E08-S03-T02 Implementation/UI:** Implement owning logic and integration adapter, expose a role-appropriate action and clear conflict/pending/approved states.
- **E08-S03-T03 Evidence/pilot:** Run positive, negative, replay and business-invariant checks using a named pilot fixture; capture acceptance evidence and rollback procedure.

#### E08-S04 — Build reverse and forward genealogy explorer
**Story:** As a relevant factory user, I need to roll→part→garment→carton and carton→roll with confidence, so that work is explainable, safe and actionable.
**Acceptance:** Source and uncertainty shown for every ancestry link.
**Dependencies:** Prior schema/source contracts in S13 / S14 / attachment; existing installed legacy and device capabilities must be discovered.
**Delivery tasks:**

- **E08-S04-T01 Domain/API:** Define source-of-truth, typed inputs/outputs, idempotency, permissions, state transitions and failure contracts.
- **E08-S04-T02 Implementation/UI:** Implement owning logic and integration adapter, expose a role-appropriate action and clear conflict/pending/approved states.
- **E08-S04-T03 Evidence/pilot:** Run positive, negative, replay and business-invariant checks using a named pilot fixture; capture acceptance evidence and rollback procedure.

### E09 — Sewing station execution
**Priority:** P0 · **Logical owners:** S14 / Edge · **Acceptance references:** G01,F01

#### E09-S01 — Build claim/complete station state machine
**Story:** As a relevant factory user, I need to nFC deliberate touch with role revision and prerequisite gate, so that work is explainable, safe and actionable.
**Acceptance:** No credit from UHF pass alone.
**Dependencies:** Prior schema/source contracts in S14 / Edge; existing installed legacy and device capabilities must be discovered.
**Delivery tasks:**

- **E09-S01-T01 Domain/API:** Define source-of-truth, typed inputs/outputs, idempotency, permissions, state transitions and failure contracts.
- **E09-S01-T02 Implementation/UI:** Implement owning logic and integration adapter, expose a role-appropriate action and clear conflict/pending/approved states.
- **E09-S01-T03 Evidence/pilot:** Run positive, negative, replay and business-invariant checks using a named pilot fixture; capture acceptance evidence and rollback procedure.

#### E09-S02 — Capture pause break changeover and machine signals
**Story:** As a relevant factory user, I need to categorize measured vs inferred time by worker machine and carrier, so that work is explainable, safe and actionable.
**Acceptance:** Overnight hours not counted as cycle time.
**Dependencies:** Prior schema/source contracts in S14 / Edge; existing installed legacy and device capabilities must be discovered.
**Delivery tasks:**

- **E09-S02-T01 Domain/API:** Define source-of-truth, typed inputs/outputs, idempotency, permissions, state transitions and failure contracts.
- **E09-S02-T02 Implementation/UI:** Implement owning logic and integration adapter, expose a role-appropriate action and clear conflict/pending/approved states.
- **E09-S02-T03 Evidence/pilot:** Run positive, negative, replay and business-invariant checks using a named pilot fixture; capture acceptance evidence and rollback procedure.

#### E09-S03 — Implement station anomaly and wait flows
**Story:** As a relevant factory user, I need to wrong shade size missing operation duplicate claim and damaged tag, so that work is explainable, safe and actionable.
**Acceptance:** Simple fail closed operator prompt.
**Dependencies:** Prior schema/source contracts in S14 / Edge; existing installed legacy and device capabilities must be discovered.
**Delivery tasks:**

- **E09-S03-T01 Domain/API:** Define source-of-truth, typed inputs/outputs, idempotency, permissions, state transitions and failure contracts.
- **E09-S03-T02 Implementation/UI:** Implement owning logic and integration adapter, expose a role-appropriate action and clear conflict/pending/approved states.
- **E09-S03-T03 Evidence/pilot:** Run positive, negative, replay and business-invariant checks using a named pilot fixture; capture acceptance evidence and rollback procedure.

#### E09-S04 — Build offline command journal and reconciliation
**Story:** As a relevant factory user, I need to pending vs committed events and replay duplicates handled safely, so that work is explainable, safe and actionable.
**Acceptance:** Same operation credited once after reconnect.
**Dependencies:** Prior schema/source contracts in S14 / Edge; existing installed legacy and device capabilities must be discovered.
**Delivery tasks:**

- **E09-S04-T01 Domain/API:** Define source-of-truth, typed inputs/outputs, idempotency, permissions, state transitions and failure contracts.
- **E09-S04-T02 Implementation/UI:** Implement owning logic and integration adapter, expose a role-appropriate action and clear conflict/pending/approved states.
- **E09-S04-T03 Evidence/pilot:** Run positive, negative, replay and business-invariant checks using a named pilot fixture; capture acceptance evidence and rollback procedure.

### E10 — Quality, finishing and buyer identity
**Priority:** P0 · **Logical owners:** S17 / S18 · **Acceptance references:** F01

#### E10-S01 — Build inspection/NCR with partial batch holds
**Story:** As a relevant factory user, I need to grade defect by affected piece or count and evidence, so that work is explainable, safe and actionable.
**Acceptance:** Only affected portions held.
**Dependencies:** Prior schema/source contracts in S17 / S18; existing installed legacy and device capabilities must be discovered.
**Delivery tasks:**

- **E10-S01-T01 Domain/API:** Define source-of-truth, typed inputs/outputs, idempotency, permissions, state transitions and failure contracts.
- **E10-S01-T02 Implementation/UI:** Implement owning logic and integration adapter, expose a role-appropriate action and clear conflict/pending/approved states.
- **E10-S01-T03 Evidence/pilot:** Run positive, negative, replay and business-invariant checks using a named pilot fixture; capture acceptance evidence and rollback procedure.

#### E10-S02 — Implement repair loop with independent check
**Story:** As a relevant factory user, I need to return altered unit through required final inspection, so that work is explainable, safe and actionable.
**Acceptance:** No double counting good units.
**Dependencies:** Prior schema/source contracts in S17 / S18; existing installed legacy and device capabilities must be discovered.
**Delivery tasks:**

- **E10-S02-T01 Domain/API:** Define source-of-truth, typed inputs/outputs, idempotency, permissions, state transitions and failure contracts.
- **E10-S02-T02 Implementation/UI:** Implement owning logic and integration adapter, expose a role-appropriate action and clear conflict/pending/approved states.
- **E10-S02-T03 Evidence/pilot:** Run positive, negative, replay and business-invariant checks using a named pilot fixture; capture acceptance evidence and rollback procedure.

#### E10-S03 — Support finishing route variants
**Story:** As a relevant factory user, I need to wash press hanger trim skip per approved style-specific DAG, so that work is explainable, safe and actionable.
**Acceptance:** Wrong product-family steps rejected.
**Dependencies:** Prior schema/source contracts in S17 / S18; existing installed legacy and device capabilities must be discovered.
**Delivery tasks:**

- **E10-S03-T01 Domain/API:** Define source-of-truth, typed inputs/outputs, idempotency, permissions, state transitions and failure contracts.
- **E10-S03-T02 Implementation/UI:** Implement owning logic and integration adapter, expose a role-appropriate action and clear conflict/pending/approved states.
- **E10-S03-T03 Evidence/pilot:** Run positive, negative, replay and business-invariant checks using a named pilot fixture; capture acceptance evidence and rollback procedure.

#### E10-S04 — Bind buyer garment identifiers
**Story:** As a relevant factory user, I need to validate style size buyer serial and acceptable finish state, so that work is explainable, safe and actionable.
**Acceptance:** No serial assigned to two active garments.
**Dependencies:** Prior schema/source contracts in S17 / S18; existing installed legacy and device capabilities must be discovered.
**Delivery tasks:**

- **E10-S04-T01 Domain/API:** Define source-of-truth, typed inputs/outputs, idempotency, permissions, state transitions and failure contracts.
- **E10-S04-T02 Implementation/UI:** Implement owning logic and integration adapter, expose a role-appropriate action and clear conflict/pending/approved states.
- **E10-S04-T03 Evidence/pilot:** Run positive, negative, replay and business-invariant checks using a named pilot fixture; capture acceptance evidence and rollback procedure.

### E11 — Buyer packs and dispatch
**Priority:** P0 · **Logical owners:** S19 / S20 · **Acceptance references:** F01

#### E11-S01 — Compile buyer-specific pack rule
**Story:** As a relevant factory user, I need to define exact 2-piece bags carton capacity and buyer exceptions, so that work is explainable, safe and actionable.
**Acceptance:** Invalid partials blocked by default.
**Dependencies:** Prior schema/source contracts in S19 / S20; existing installed legacy and device capabilities must be discovered.
**Delivery tasks:**

- **E11-S01-T01 Domain/API:** Define source-of-truth, typed inputs/outputs, idempotency, permissions, state transitions and failure contracts.
- **E11-S01-T02 Implementation/UI:** Implement owning logic and integration adapter, expose a role-appropriate action and clear conflict/pending/approved states.
- **E11-S01-T03 Evidence/pilot:** Run positive, negative, replay and business-invariant checks using a named pilot fixture; capture acceptance evidence and rollback procedure.

#### E11-S02 — Build UHF bag and barcode carton evidence
**Story:** As a relevant factory user, I need to enforce uniqueness and exact garment membership, so that work is explainable, safe and actionable.
**Acceptance:** Duplicate scans do not increment count.
**Dependencies:** Prior schema/source contracts in S19 / S20; existing installed legacy and device capabilities must be discovered.
**Delivery tasks:**

- **E11-S02-T01 Domain/API:** Define source-of-truth, typed inputs/outputs, idempotency, permissions, state transitions and failure contracts.
- **E11-S02-T02 Implementation/UI:** Implement owning logic and integration adapter, expose a role-appropriate action and clear conflict/pending/approved states.
- **E11-S02-T03 Evidence/pilot:** Run positive, negative, replay and business-invariant checks using a named pilot fixture; capture acceptance evidence and rollback procedure.

#### E11-S03 — Create pack audit and carton seal gates
**Story:** As a relevant factory user, I need to qC hold and wrong SKU or size block sealing, so that work is explainable, safe and actionable.
**Acceptance:** Approved exception cites buyer authorization.
**Dependencies:** Prior schema/source contracts in S19 / S20; existing installed legacy and device capabilities must be discovered.
**Delivery tasks:**

- **E11-S03-T01 Domain/API:** Define source-of-truth, typed inputs/outputs, idempotency, permissions, state transitions and failure contracts.
- **E11-S03-T02 Implementation/UI:** Implement owning logic and integration adapter, expose a role-appropriate action and clear conflict/pending/approved states.
- **E11-S03-T03 Evidence/pilot:** Run positive, negative, replay and business-invariant checks using a named pilot fixture; capture acceptance evidence and rollback procedure.

#### E11-S04 — Implement manifest load and release
**Story:** As a relevant factory user, I need to match scanned carton dock truck manifest and handoff, so that work is explainable, safe and actionable.
**Acceptance:** Cannot dispatch unaudited carton.
**Dependencies:** Prior schema/source contracts in S19 / S20; existing installed legacy and device capabilities must be discovered.
**Delivery tasks:**

- **E11-S04-T01 Domain/API:** Define source-of-truth, typed inputs/outputs, idempotency, permissions, state transitions and failure contracts.
- **E11-S04-T02 Implementation/UI:** Implement owning logic and integration adapter, expose a role-appropriate action and clear conflict/pending/approved states.
- **E11-S04-T03 Evidence/pilot:** Run positive, negative, replay and business-invariant checks using a named pilot fixture; capture acceptance evidence and rollback procedure.

### E12 — Order closure and financial truth
**Priority:** P1 · **Logical owners:** S21 / S22 · **Acceptance references:** F01,F02

#### E12-S01 — Implement stage-by-stage count conservation
**Story:** As a relevant factory user, I need to reconcile cut good WIP repair scrap pack and ship by type, so that work is explainable, safe and actionable.
**Acceptance:** Residual explained rather than rounded.
**Dependencies:** Prior schema/source contracts in S21 / S22; existing installed legacy and device capabilities must be discovered.
**Delivery tasks:**

- **E12-S01-T01 Domain/API:** Define source-of-truth, typed inputs/outputs, idempotency, permissions, state transitions and failure contracts.
- **E12-S01-T02 Implementation/UI:** Implement owning logic and integration adapter, expose a role-appropriate action and clear conflict/pending/approved states.
- **E12-S01-T03 Evidence/pilot:** Run positive, negative, replay and business-invariant checks using a named pilot fixture; capture acceptance evidence and rollback procedure.

#### E12-S02 — Generate immutable order closure certificate
**Story:** As a relevant factory user, I need to freeze evidence revisions and role-based signoffs, so that work is explainable, safe and actionable.
**Acceptance:** Corrections are signed addenda.
**Dependencies:** Prior schema/source contracts in S21 / S22; existing installed legacy and device capabilities must be discovered.
**Delivery tasks:**

- **E12-S02-T01 Domain/API:** Define source-of-truth, typed inputs/outputs, idempotency, permissions, state transitions and failure contracts.
- **E12-S02-T02 Implementation/UI:** Implement owning logic and integration adapter, expose a role-appropriate action and clear conflict/pending/approved states.
- **E12-S02-T03 Evidence/pilot:** Run positive, negative, replay and business-invariant checks using a named pilot fixture; capture acceptance evidence and rollback procedure.

#### E12-S03 — Compute material and conversion actual costs
**Story:** As a relevant factory user, I need to use price basis scrap labor machine freight and rates provenance, so that work is explainable, safe and actionable.
**Acceptance:** PO unit amount never assumed to be margin.
**Dependencies:** Prior schema/source contracts in S21 / S22; existing installed legacy and device capabilities must be discovered.
**Delivery tasks:**

- **E12-S03-T01 Domain/API:** Define source-of-truth, typed inputs/outputs, idempotency, permissions, state transitions and failure contracts.
- **E12-S03-T02 Implementation/UI:** Implement owning logic and integration adapter, expose a role-appropriate action and clear conflict/pending/approved states.
- **E12-S03-T03 Evidence/pilot:** Run positive, negative, replay and business-invariant checks using a named pilot fixture; capture acceptance evidence and rollback procedure.

#### E12-S04 — Connect ERP financial handoff
**Story:** As a relevant factory user, I need to post audited invoice basis and adjustment with external ack, so that work is explainable, safe and actionable.
**Acceptance:** No double invoice on retries.
**Dependencies:** Prior schema/source contracts in S21 / S22; existing installed legacy and device capabilities must be discovered.
**Delivery tasks:**

- **E12-S04-T01 Domain/API:** Define source-of-truth, typed inputs/outputs, idempotency, permissions, state transitions and failure contracts.
- **E12-S04-T02 Implementation/UI:** Implement owning logic and integration adapter, expose a role-appropriate action and clear conflict/pending/approved states.
- **E12-S04-T03 Evidence/pilot:** Run positive, negative, replay and business-invariant checks using a named pilot fixture; capture acceptance evidence and rollback procedure.

### E13 — Flow-time and variability intelligence
**Priority:** P1 · **Logical owners:** S24 / S25 · **Acceptance references:** T01

#### E13-S01 — Build event interval reconstruction
**Story:** As a relevant factory user, I need to derive stage enter/exit and source freshness with clock error, so that work is explainable, safe and actionable.
**Acceptance:** Unknown time distinguished from zero.
**Dependencies:** Prior schema/source contracts in S24 / S25; existing installed legacy and device capabilities must be discovered.
**Delivery tasks:**

- **E13-S01-T01 Domain/API:** Define source-of-truth, typed inputs/outputs, idempotency, permissions, state transitions and failure contracts.
- **E13-S01-T02 Implementation/UI:** Implement owning logic and integration adapter, expose a role-appropriate action and clear conflict/pending/approved states.
- **E13-S01-T03 Evidence/pilot:** Run positive, negative, replay and business-invariant checks using a named pilot fixture; capture acceptance evidence and rollback procedure.

#### E13-S02 — Separate queue touch wait rework and handoffs
**Story:** As a relevant factory user, I need to segment active blocked idle transport repair by evidence, so that work is explainable, safe and actionable.
**Acceptance:** No overnight false cycle.
**Dependencies:** Prior schema/source contracts in S24 / S25; existing installed legacy and device capabilities must be discovered.
**Delivery tasks:**

- **E13-S02-T01 Domain/API:** Define source-of-truth, typed inputs/outputs, idempotency, permissions, state transitions and failure contracts.
- **E13-S02-T02 Implementation/UI:** Implement owning logic and integration adapter, expose a role-appropriate action and clear conflict/pending/approved states.
- **E13-S02-T03 Evidence/pilot:** Run positive, negative, replay and business-invariant checks using a named pilot fixture; capture acceptance evidence and rollback procedure.

#### E13-S03 — Publish distributions and fairness guards
**Story:** As a relevant factory user, I need to p50/p90/p95 sample counts mix and quality-adjusted KPIs, so that work is explainable, safe and actionable.
**Acceptance:** Metrics specify denominator and sample.
**Dependencies:** Prior schema/source contracts in S24 / S25; existing installed legacy and device capabilities must be discovered.
**Delivery tasks:**

- **E13-S03-T01 Domain/API:** Define source-of-truth, typed inputs/outputs, idempotency, permissions, state transitions and failure contracts.
- **E13-S03-T02 Implementation/UI:** Implement owning logic and integration adapter, expose a role-appropriate action and clear conflict/pending/approved states.
- **E13-S03-T03 Evidence/pilot:** Run positive, negative, replay and business-invariant checks using a named pilot fixture; capture acceptance evidence and rollback procedure.

#### E13-S04 — Build root-cause bottleneck view
**Story:** As a relevant factory user, I need to attribute queue causes to feed starvation skill mix machine and rework, so that work is explainable, safe and actionable.
**Acceptance:** Recommender cites source observations.
**Dependencies:** Prior schema/source contracts in S24 / S25; existing installed legacy and device capabilities must be discovered.
**Delivery tasks:**

- **E13-S04-T01 Domain/API:** Define source-of-truth, typed inputs/outputs, idempotency, permissions, state transitions and failure contracts.
- **E13-S04-T02 Implementation/UI:** Implement owning logic and integration adapter, expose a role-appropriate action and clear conflict/pending/approved states.
- **E13-S04-T03 Evidence/pilot:** Run positive, negative, replay and business-invariant checks using a named pilot fixture; capture acceptance evidence and rollback procedure.

### E14 — Disruption handling and finite-capacity replan
**Priority:** P1 · **Logical owners:** S09 / S26 · **Acceptance references:** T02,T03

#### E14-S01 — Integrate absence and breakdown events
**Story:** As a relevant factory user, I need to import shift absence qualified skills MTTR and unavailable machine state, so that work is explainable, safe and actionable.
**Acceptance:** Change has effective times and evidence.
**Dependencies:** Prior schema/source contracts in S09 / S26; existing installed legacy and device capabilities must be discovered.
**Delivery tasks:**

- **E14-S01-T01 Domain/API:** Define source-of-truth, typed inputs/outputs, idempotency, permissions, state transitions and failure contracts.
- **E14-S01-T02 Implementation/UI:** Implement owning logic and integration adapter, expose a role-appropriate action and clear conflict/pending/approved states.
- **E14-S01-T03 Evidence/pilot:** Run positive, negative, replay and business-invariant checks using a named pilot fixture; capture acceptance evidence and rollback procedure.

#### E14-S02 — Compute critical path with finite machines and shifts
**Story:** As a relevant factory user, I need to schedule active orders using approved DAG and time calendars, so that work is explainable, safe and actionable.
**Acceptance:** Gantt shows dependencies and slack.
**Dependencies:** Prior schema/source contracts in S09 / S26; existing installed legacy and device capabilities must be discovered.
**Delivery tasks:**

- **E14-S02-T01 Domain/API:** Define source-of-truth, typed inputs/outputs, idempotency, permissions, state transitions and failure contracts.
- **E14-S02-T02 Implementation/UI:** Implement owning logic and integration adapter, expose a role-appropriate action and clear conflict/pending/approved states.
- **E14-S02-T03 Evidence/pilot:** Run positive, negative, replay and business-invariant checks using a named pilot fixture; capture acceptance evidence and rollback procedure.

#### E14-S03 — Generate controlled alternatives
**Story:** As a relevant factory user, I need to compare shift changes certified worker substitutions and alternate fabric, so that work is explainable, safe and actionable.
**Acceptance:** Hard constraints never weakened silently.
**Dependencies:** Prior schema/source contracts in S09 / S26; existing installed legacy and device capabilities must be discovered.
**Delivery tasks:**

- **E14-S03-T01 Domain/API:** Define source-of-truth, typed inputs/outputs, idempotency, permissions, state transitions and failure contracts.
- **E14-S03-T02 Implementation/UI:** Implement owning logic and integration adapter, expose a role-appropriate action and clear conflict/pending/approved states.
- **E14-S03-T03 Evidence/pilot:** Run positive, negative, replay and business-invariant checks using a named pilot fixture; capture acceptance evidence and rollback procedure.

#### E14-S04 — Implement approval and rebaseline workflow
**Story:** As a relevant factory user, I need to version schedule and preserve completed history, so that work is explainable, safe and actionable.
**Acceptance:** Manager can undo unstarted interventions.
**Dependencies:** Prior schema/source contracts in S09 / S26; existing installed legacy and device capabilities must be discovered.
**Delivery tasks:**

- **E14-S04-T01 Domain/API:** Define source-of-truth, typed inputs/outputs, idempotency, permissions, state transitions and failure contracts.
- **E14-S04-T02 Implementation/UI:** Implement owning logic and integration adapter, expose a role-appropriate action and clear conflict/pending/approved states.
- **E14-S04-T03 Evidence/pilot:** Run positive, negative, replay and business-invariant checks using a named pilot fixture; capture acceptance evidence and rollback procedure.

### E15 — Experimentation and demonstrated ROI
**Priority:** P1 · **Logical owners:** S26 / S27 · **Acceptance references:** B04,F02

#### E15-S01 — Create experiment registry and guardrails
**Story:** As a relevant factory user, I need to record hypothesis unit assignment comparator and stop rule, so that work is explainable, safe and actionable.
**Acceptance:** No post-hoc result redefinition.
**Dependencies:** Prior schema/source contracts in S26 / S27; existing installed legacy and device capabilities must be discovered.
**Delivery tasks:**

- **E15-S01-T01 Domain/API:** Define source-of-truth, typed inputs/outputs, idempotency, permissions, state transitions and failure contracts.
- **E15-S01-T02 Implementation/UI:** Implement owning logic and integration adapter, expose a role-appropriate action and clear conflict/pending/approved states.
- **E15-S01-T03 Evidence/pilot:** Run positive, negative, replay and business-invariant checks using a named pilot fixture; capture acceptance evidence and rollback procedure.

#### E15-S02 — Assign matched A/B exposure
**Story:** As a relevant factory user, I need to randomize or match comparable cut lays/lines and calendar context, so that work is explainable, safe and actionable.
**Acceptance:** Baseline confounders disclosed.
**Dependencies:** Prior schema/source contracts in S26 / S27; existing installed legacy and device capabilities must be discovered.
**Delivery tasks:**

- **E15-S02-T01 Domain/API:** Define source-of-truth, typed inputs/outputs, idempotency, permissions, state transitions and failure contracts.
- **E15-S02-T02 Implementation/UI:** Implement owning logic and integration adapter, expose a role-appropriate action and clear conflict/pending/approved states.
- **E15-S02-T03 Evidence/pilot:** Run positive, negative, replay and business-invariant checks using a named pilot fixture; capture acceptance evidence and rollback procedure.

#### E15-S03 — Calculate treatment effect and uncertainty
**Story:** As a relevant factory user, I need to compare fabric yield accepted units lead time quality and cost, so that work is explainable, safe and actionable.
**Acceptance:** Report negative and inconclusive results.
**Dependencies:** Prior schema/source contracts in S26 / S27; existing installed legacy and device capabilities must be discovered.
**Delivery tasks:**

- **E15-S03-T01 Domain/API:** Define source-of-truth, typed inputs/outputs, idempotency, permissions, state transitions and failure contracts.
- **E15-S03-T02 Implementation/UI:** Implement owning logic and integration adapter, expose a role-appropriate action and clear conflict/pending/approved states.
- **E15-S03-T03 Evidence/pilot:** Run positive, negative, replay and business-invariant checks using a named pilot fixture; capture acceptance evidence and rollback procedure.

#### E15-S04 — Close the decision loop
**Story:** As a relevant factory user, I need to approve scale pause rollback and measure realized outcome, so that work is explainable, safe and actionable.
**Acceptance:** Forecast-versus-actual variance persisted.
**Dependencies:** Prior schema/source contracts in S26 / S27; existing installed legacy and device capabilities must be discovered.
**Delivery tasks:**

- **E15-S04-T01 Domain/API:** Define source-of-truth, typed inputs/outputs, idempotency, permissions, state transitions and failure contracts.
- **E15-S04-T02 Implementation/UI:** Implement owning logic and integration adapter, expose a role-appropriate action and clear conflict/pending/approved states.
- **E15-S04-T03 Evidence/pilot:** Run positive, negative, replay and business-invariant checks using a named pilot fixture; capture acceptance evidence and rollback procedure.

### E16 — Operator and manager experience
**Priority:** P0 · **Logical owners:** BFF / Web · **Acceptance references:** U01,U02

#### E16-S01 — Create grade-five station language pack
**Story:** As a relevant factory user, I need to translate core errors and instructions with factory worker review, so that work is explainable, safe and actionable.
**Acceptance:** Floor comprehension and action success validated.
**Dependencies:** Prior schema/source contracts in BFF / Web; existing installed legacy and device capabilities must be discovered.
**Delivery tasks:**

- **E16-S01-T01 Domain/API:** Define source-of-truth, typed inputs/outputs, idempotency, permissions, state transitions and failure contracts.
- **E16-S01-T02 Implementation/UI:** Implement owning logic and integration adapter, expose a role-appropriate action and clear conflict/pending/approved states.
- **E16-S01-T03 Evidence/pilot:** Run positive, negative, replay and business-invariant checks using a named pilot fixture; capture acceptance evidence and rollback procedure.

#### E16-S02 — Build minimal tap-first station screens
**Story:** As a relevant factory user, I need to one action at a time icons large targets and clear pending status, so that work is explainable, safe and actionable.
**Acceptance:** No mandatory routine keyboard typing.
**Dependencies:** Prior schema/source contracts in BFF / Web; existing installed legacy and device capabilities must be discovered.
**Delivery tasks:**

- **E16-S02-T01 Domain/API:** Define source-of-truth, typed inputs/outputs, idempotency, permissions, state transitions and failure contracts.
- **E16-S02-T02 Implementation/UI:** Implement owning logic and integration adapter, expose a role-appropriate action and clear conflict/pending/approved states.
- **E16-S02-T03 Evidence/pilot:** Run positive, negative, replay and business-invariant checks using a named pilot fixture; capture acceptance evidence and rollback procedure.

#### E16-S03 — Connect role worklists and action ownership
**Story:** As a relevant factory user, I need to line lead inspector and manager see permitted work and exceptions, so that work is explainable, safe and actionable.
**Acceptance:** No orphan issue after escalation.
**Dependencies:** Prior schema/source contracts in BFF / Web; existing installed legacy and device capabilities must be discovered.
**Delivery tasks:**

- **E16-S03-T01 Domain/API:** Define source-of-truth, typed inputs/outputs, idempotency, permissions, state transitions and failure contracts.
- **E16-S03-T02 Implementation/UI:** Implement owning logic and integration adapter, expose a role-appropriate action and clear conflict/pending/approved states.
- **E16-S03-T03 Evidence/pilot:** Run positive, negative, replay and business-invariant checks using a named pilot fixture; capture acceptance evidence and rollback procedure.

#### E16-S04 — Build supervisor Gantt and factory action dashboard
**Story:** As a relevant factory user, I need to show last refresh plan risks blocked work and owner, so that work is explainable, safe and actionable.
**Acceptance:** No stale KPI disguised as live.
**Dependencies:** Prior schema/source contracts in BFF / Web; existing installed legacy and device capabilities must be discovered.
**Delivery tasks:**

- **E16-S04-T01 Domain/API:** Define source-of-truth, typed inputs/outputs, idempotency, permissions, state transitions and failure contracts.
- **E16-S04-T02 Implementation/UI:** Implement owning logic and integration adapter, expose a role-appropriate action and clear conflict/pending/approved states.
- **E16-S04-T03 Evidence/pilot:** Run positive, negative, replay and business-invariant checks using a named pilot fixture; capture acceptance evidence and rollback procedure.

### E17 — Plant security, event truth and reliability
**Priority:** P0 · **Logical owners:** S01 / S02 / core · **Acceptance references:** F01

#### E17-S01 — Enforce per-aggregate transaction ownership
**Story:** As a relevant factory user, I need to signed authority lease roles RLS and idempotency, so that work is explainable, safe and actionable.
**Acceptance:** Conflicting writers denied.
**Dependencies:** Prior schema/source contracts in S01 / S02 / core; existing installed legacy and device capabilities must be discovered.
**Delivery tasks:**

- **E17-S01-T01 Domain/API:** Define source-of-truth, typed inputs/outputs, idempotency, permissions, state transitions and failure contracts.
- **E17-S01-T02 Implementation/UI:** Implement owning logic and integration adapter, expose a role-appropriate action and clear conflict/pending/approved states.
- **E17-S01-T03 Evidence/pilot:** Run positive, negative, replay and business-invariant checks using a named pilot fixture; capture acceptance evidence and rollback procedure.

#### E17-S02 — Implement durable outbox/inbox and replay
**Story:** As a relevant factory user, I need to atomic business event + outbox inbox consumer dedupe, so that work is explainable, safe and actionable.
**Acceptance:** No double accepted physical quantity.
**Dependencies:** Prior schema/source contracts in S01 / S02 / core; existing installed legacy and device capabilities must be discovered.
**Delivery tasks:**

- **E17-S02-T01 Domain/API:** Define source-of-truth, typed inputs/outputs, idempotency, permissions, state transitions and failure contracts.
- **E17-S02-T02 Implementation/UI:** Implement owning logic and integration adapter, expose a role-appropriate action and clear conflict/pending/approved states.
- **E17-S02-T03 Evidence/pilot:** Run positive, negative, replay and business-invariant checks using a named pilot fixture; capture acceptance evidence and rollback procedure.

#### E17-S03 — Qualify device clocks and local signed rules
**Story:** As a relevant factory user, I need to clock uncertainty key rotation and offline permission cache, so that work is explainable, safe and actionable.
**Acceptance:** Drift never yields false duration.
**Dependencies:** Prior schema/source contracts in S01 / S02 / core; existing installed legacy and device capabilities must be discovered.
**Delivery tasks:**

- **E17-S03-T01 Domain/API:** Define source-of-truth, typed inputs/outputs, idempotency, permissions, state transitions and failure contracts.
- **E17-S03-T02 Implementation/UI:** Implement owning logic and integration adapter, expose a role-appropriate action and clear conflict/pending/approved states.
- **E17-S03-T03 Evidence/pilot:** Run positive, negative, replay and business-invariant checks using a named pilot fixture; capture acceptance evidence and rollback procedure.

#### E17-S04 — Build recovery audit and release gates
**Story:** As a relevant factory user, I need to restore station broker DB from backup and reconcile count hashes, so that work is explainable, safe and actionable.
**Acceptance:** Uncommitted buffered command shown pending.
**Dependencies:** Prior schema/source contracts in S01 / S02 / core; existing installed legacy and device capabilities must be discovered.
**Delivery tasks:**

- **E17-S04-T01 Domain/API:** Define source-of-truth, typed inputs/outputs, idempotency, permissions, state transitions and failure contracts.
- **E17-S04-T02 Implementation/UI:** Implement owning logic and integration adapter, expose a role-appropriate action and clear conflict/pending/approved states.
- **E17-S04-T03 Evidence/pilot:** Run positive, negative, replay and business-invariant checks using a named pilot fixture; capture acceptance evidence and rollback procedure.

### E18 — Deployment telemetry and pilot onboarding
**Priority:** P0 · **Logical owners:** Platform / Ops · **Acceptance references:** B01,B02

#### E18-S01 — Build plant install and nonintrusive rollback
**Story:** As a relevant factory user, I need to docker or chosen runtime with sidecar observer and config snapshot, so that work is explainable, safe and actionable.
**Acceptance:** Remove observer without disrupting existing tools.
**Dependencies:** Prior schema/source contracts in Platform / Ops; existing installed legacy and device capabilities must be discovered.
**Delivery tasks:**

- **E18-S01-T01 Domain/API:** Define source-of-truth, typed inputs/outputs, idempotency, permissions, state transitions and failure contracts.
- **E18-S01-T02 Implementation/UI:** Implement owning logic and integration adapter, expose a role-appropriate action and clear conflict/pending/approved states.
- **E18-S01-T03 Evidence/pilot:** Run positive, negative, replay and business-invariant checks using a named pilot fixture; capture acceptance evidence and rollback procedure.

#### E18-S02 — Instrument service device connector and data health
**Story:** As a relevant factory user, I need to logs traces event age read rejection tag failure and queue lag, so that work is explainable, safe and actionable.
**Acceptance:** Each failed edge has owner and remediation.
**Dependencies:** Prior schema/source contracts in Platform / Ops; existing installed legacy and device capabilities must be discovered.
**Delivery tasks:**

- **E18-S02-T01 Domain/API:** Define source-of-truth, typed inputs/outputs, idempotency, permissions, state transitions and failure contracts.
- **E18-S02-T02 Implementation/UI:** Implement owning logic and integration adapter, expose a role-appropriate action and clear conflict/pending/approved states.
- **E18-S02-T03 Evidence/pilot:** Run positive, negative, replay and business-invariant checks using a named pilot fixture; capture acceptance evidence and rollback procedure.

#### E18-S03 — Prepare golden factory fixtures
**Story:** As a relevant factory user, I need to use real PO 220988 and IZOD variants without assuming conflicting OB valid, so that work is explainable, safe and actionable.
**Acceptance:** Acceptance held on source mismatch.
**Dependencies:** Prior schema/source contracts in Platform / Ops; existing installed legacy and device capabilities must be discovered.
**Delivery tasks:**

- **E18-S03-T01 Domain/API:** Define source-of-truth, typed inputs/outputs, idempotency, permissions, state transitions and failure contracts.
- **E18-S03-T02 Implementation/UI:** Implement owning logic and integration adapter, expose a role-appropriate action and clear conflict/pending/approved states.
- **E18-S03-T03 Evidence/pilot:** Run positive, negative, replay and business-invariant checks using a named pilot fixture; capture acceptance evidence and rollback procedure.

#### E18-S04 — Run plant increment signoffs
**Story:** As a relevant factory user, I need to stage rollout with documented training approvals and business result, so that work is explainable, safe and actionable.
**Acceptance:** Each step has quantitative go/no-go evidence.
**Dependencies:** Prior schema/source contracts in Platform / Ops; existing installed legacy and device capabilities must be discovered.
**Delivery tasks:**

- **E18-S04-T01 Domain/API:** Define source-of-truth, typed inputs/outputs, idempotency, permissions, state transitions and failure contracts.
- **E18-S04-T02 Implementation/UI:** Implement owning logic and integration adapter, expose a role-appropriate action and clear conflict/pending/approved states.
- **E18-S04-T03 Evidence/pilot:** Run positive, negative, replay and business-invariant checks using a named pilot fixture; capture acceptance evidence and rollback procedure.

## Story readiness and completion gate

**Ready:** exact business actor, source record, physical identity, authority state, upstream/downstream impact, acceptance fixture, integration owner and failure semantics. **Done:** integrated code, migration, UI, controlled action, audit, accessible copy, retry/offline path, observed pilot outcome, source-level file diff and signed product owner review.

## Dependency guidance

E01/E02/E17 establish nonintrusive source authority. E03/E04 establish approved document inputs and planning snapshot. E05 precedes true E06 roll allocation. E07 precedes E08 physical carrier lifecycle; E08 enables meaningful E09 station genealogy. E10 controls release into E11; E11 plus material state enables E12. E13 depends on reliable event semantics, while E14/E15 depend on sufficiently measured baselines. E16 is continuous; station language work starts in R0.

## High-impact first sprint slices

1. Inventory actual integrations and assign source authority by aggregate; implement view-only plant source status and discrepancy queue.
2. Implement manager upload → proposed PO/style/BOM facts → conflicts → planning snapshot → short question → candidate plan, using live existing stock and manually declared uncertainty where unavailable.
3. Add roll-usable-width and remaining-length facts to stores model, and first CAD/Gerber candidate import adapter before building a global optimizer.
4. Pilot UHF roll/bundle/sub-bundle reading and NFC individual part on one style/line with measured cost and time burden; do not prematurely mandate all-part tagging.
5. Record plan/actual deltas and one matched A/B pilot with holdback/rollback criteria.