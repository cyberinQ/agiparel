# AGIPAREL FACTORY OS — V5 DEVELOPMENT EPICS, STORIES, TASKS & CODE-COMPLETENESS AUDIT
**Author:** Sunil Jain, CyberinQ LLC  |  **Date:** 26 October 2026  |  **Basis:** supplied `agiparel(6).zip`, V4 backlog, V5 PRD/ARD
**Audit scope:** static repository inspection and attempted test execution; no live plant, RFID hardware, database or deployed runtime was available. Therefore code-complete means end-to-end verified behavior and cannot be inferred from file existence.

## 1. Executive engineering verdict
**NOT CODE COMPLETE for V4, and therefore not V5 code complete.** The repository contains substantive Go domain engines, an HTTP edge application, early planning/document workflows, integration tests and embedded operator/manager HTML. It does not evidence the full specified 29-service architecture, complete durable workflows, all role screens or verified factory deployment. Do not describe it as an empty prototype, either: tested domain components and usable vertical endpoints exist.

**Audit distinctions:** `PRESENT-IN-SOURCE` = tangible logic or page, not acceptance; `PARTIAL` = some supporting logic, but contract/production integration incomplete; `NOT-FOUND-IN-INSPECTED-SOURCE` = no corresponding implementation verified, not proof of absolute absence; `BLOCKED-VERIFICATION` = tooling/live hardware missing.

### Grounded repository inventory
- 99 inspected Go/SQL/HTML/YAML/Markdown files (excluding git metadata); 81 Go files; 72 V4 stories across 18 epics.
- Named `services/s01`–`s29` directories: 29 exist, 28 contain no regular files, and `s01-identity` contains SQL migration files only. Functional implementation is concentrated in `services/edge-daemon`, `services/core-api`, and `pkg`.
- Four embedded UI HTML files (`cutting_view.html`, `sew01.html`, `onboarding.html`, `plant_view.html`); this is not evidence of the complete v3/v4 screen inventory.
- Runtime edge route group includes onboarding, planning, cutting, stores, sewing, packing, OCR, topology and AGIE; many services named in the design have no corresponding independently running service implementation.
- Database foundation includes migration SQL and identity middleware, but several critical `EdgeServer` fields are in-process state (`activePackings`, `activeOCRSess`, `StoresEngine`, `stagedCfg`).
- The upload job registry is process-local and explicitly returns “jobs expire on restart.”
- Release workflow builds the edge binary on version tags and does not require unit/integration tests.
- `go test ./...` was attempted; **could not run** because Go 1.24 toolchain download requires unavailable network access in audit environment. This is NOT a test failure and NOT a pass.
- Git snapshot head: `7f00d11` dated 2026-10-25 (`v4 making progress.`). This is the inspected snapshot, not a claim about any newer main branch.

## 2. V4 epic assessment — source evidence, not speculative percentages
| Epic | Source assessment | Specific evidence / missing proof |
|---|---|---|
| E01 — Brownfield adoption and system authority | **PARTIAL** | `pkg/brownfield/authority.go` pure rule; lacks durable fencing/lease state |
| E02 — Legacy adapters, master data and ingestion | **PARTIAL** | onboarding workbook, document capture, database foundation; adapters/ID lineage incomplete |
| E03 — Document intelligence and versioned knowledge graph | **PARTIAL** | `pkg/docscribe`, `pkg/documentcapture`; source-backed cross-document approval graph unverified |
| E04 — Interactive feasibility copilot | **PARTIAL** | planning analyzer, readiness questions, upload jobs and endpoints; durable full snapshot/approval incomplete |
| E05 — Material roll passport and stock truth | **PARTIAL** | stores issuance/roll feasibility; no persistent production-grade roll segment ledger established |
| E06 — High-level cutting campaign optimization | **PARTIAL** | cutting planner, roll feasibility, geometry optimizer; CAD vendor bridge and live cut-table integration unverified |
| E07 — Tag hardware and lifecycle | **PARTIAL** | `pkg/rfid/sgtin.go`, `llrp_reader.go`; reader/tag lifecycle hardware commissioning unverified |
| E08 — Sub-bundle and individual-part traceability | **PARTIAL** | attachment engines and tests present; full physical bundle/garment genealogy not established |
| E09 — Sewing station execution | **PARTIAL** | dwell logic, sewing endpoints, `sew01.html`; typed production credit/full offline workflow unverified |
| E10 — Quality, finishing and buyer identity | **NOT-FOUND-IN-INSPECTED-SOURCE** | No corresponding code in empty s17/s18; some adjacent shared evidence/UI but comprehensive finishing absent |
| E11 — Buyer packs and dispatch | **PARTIAL** | carton packer and pack session routes; full buyer audit/repack/manifest release not established |
| E12 — Order closure and financial truth | **PARTIAL** | OCR reconciliation, signoff, PDF code; complete finance ERP handoff unverified |
| E13 — Flow-time and variability intelligence | **PARTIAL** | dwell/correlator and plant dashboard, but defensible cross-stage queue-time analytics not established |
| E14 — Disruption handling and finite-capacity replan | **PARTIAL** | planning analyzer and heuristics; full finite-capacity replan governance unverified |
| E15 — Experimentation and demonstrated ROI | **NOT-FOUND-IN-INSPECTED-SOURCE** | No A/B assignment, guardrail monitoring, causal inference results workflow found |
| E16 — Operator and manager experience | **PARTIAL** | Four HTML HMIs; missing verified role-specific end-to-end screen coverage |
| E17 — Plant security, event truth and reliability | **PARTIAL** | WAL, HMAC ledger, SQL migrations, identity code; atomic persisted business state and auth tests incomplete |
| E18 — Deployment telemetry and pilot onboarding | **PARTIAL** | compose/deploy templates and some tests; signed plant integration/release gates absent |

**Critical conclusion:** A passing domain unit test is not evidence that its API, database, on-floor devices, authorization, UI, retry and downstream reconciliation work together. Treat all unproven stories as requiring acceptance evidence.

## 3. V4 stories retained as active implementation backlog
All original IDs E01-S01 … E18-S04 are retained and unchanged in meaning. Source assessment at epic level above is a triage starting point, never permission to close an individual story. Each needs `source path → execution/API trace → state persistence → tenant/role validation → negative/replay test → UI proof → pilot signoff`.

| Story | Original outcome to deliver | Audit disposition |
|---|---|---|
| E01-S01 | Map legacy systems and data feeds | **SOURCE REVIEW REQUIRED** (PARTIAL epic) |
| E01-S02 | Implement shadow observation mode | **SOURCE REVIEW REQUIRED** (PARTIAL epic) |
| E01-S03 | Introduce scoped ownership transitions | **SOURCE REVIEW REQUIRED** (PARTIAL epic) |
| E01-S04 | Build reconciliation exception desk | **SOURCE REVIEW REQUIRED** (PARTIAL epic) |
| E02-S01 | Build safe import adapter framework | **SOURCE REVIEW REQUIRED** (PARTIAL epic) |
| E02-S02 | Create external ID resolver | **SOURCE REVIEW REQUIRED** (PARTIAL epic) |
| E02-S03 | Build material and machine master import | **SOURCE REVIEW REQUIRED** (PARTIAL epic) |
| E02-S04 | Instrument connector health and backfill | **SOURCE REVIEW REQUIRED** (PARTIAL epic) |
| E03-S01 | Extract buyer files into proposed facts | **SOURCE REVIEW REQUIRED** (PARTIAL epic) |
| E03-S02 | Build typed document relationships | **SOURCE REVIEW REQUIRED** (PARTIAL epic) |
| E03-S03 | Implement conflict and approval workflow | **SOURCE REVIEW REQUIRED** (PARTIAL epic) |
| E03-S04 | Create revision impact analyzer | **SOURCE REVIEW REQUIRED** (PARTIAL epic) |
| E04-S01 | Build PlanningSnapshot aggregator | **SOURCE REVIEW REQUIRED** (PARTIAL epic) |
| E04-S02 | Compute baseline feasible or impossible status | **SOURCE REVIEW REQUIRED** (PARTIAL epic) |
| E04-S03 | Generate ranked clarification questions | **SOURCE REVIEW REQUIRED** (PARTIAL epic) |
| E04-S04 | Publish controlled plan from conversation | **SOURCE REVIEW REQUIRED** (PARTIAL epic) |
| E05-S01 | Define roll and fabric segment schema | **SOURCE REVIEW REQUIRED** (PARTIAL epic) |
| E05-S02 | Capture GRN inspection and calibration | **SOURCE REVIEW REQUIRED** (PARTIAL epic) |
| E05-S03 | Implement slice reservation and split returns | **SOURCE REVIEW REQUIRED** (PARTIAL epic) |
| E05-S04 | Implement material moves and remnants | **SOURCE REVIEW REQUIRED** (PARTIAL epic) |
| E06-S01 | Connect CAD/Gerber marker candidate interface | **SOURCE REVIEW REQUIRED** (PARTIAL epic) |
| E06-S02 | Build heterogeneous roll allocation optimizer | **SOURCE REVIEW REQUIRED** (PARTIAL epic) |
| E06-S03 | Schedule lays and cut-table availability | **SOURCE REVIEW REQUIRED** (PARTIAL epic) |
| E06-S04 | Commit actual cut outcomes | **SOURCE REVIEW REQUIRED** (PARTIAL epic) |
| E07-S01 | Build UHF EPC/NFC encoding registry | **SOURCE REVIEW REQUIRED** (PARTIAL epic) |
| E07-S02 | Implement tag provision and verify | **SOURCE REVIEW REQUIRED** (PARTIAL epic) |
| E07-S03 | Implement binding retirement and reuse | **SOURCE REVIEW REQUIRED** (PARTIAL epic) |
| E07-S04 | Commission read zones and location confidence | **SOURCE REVIEW REQUIRED** (PARTIAL epic) |
| E08-S01 | Model variable bundle/sub-bundle memberships | **SOURCE REVIEW REQUIRED** (PARTIAL epic) |
| E08-S02 | Introduce NFC part-level pilot | **SOURCE REVIEW REQUIRED** (PARTIAL epic) |
| E08-S03 | Build atomic split merge transformation ledger | **SOURCE REVIEW REQUIRED** (PARTIAL epic) |
| E08-S04 | Build reverse and forward genealogy explorer | **SOURCE REVIEW REQUIRED** (PARTIAL epic) |
| E09-S01 | Build claim/complete station state machine | **SOURCE REVIEW REQUIRED** (PARTIAL epic) |
| E09-S02 | Capture pause break changeover and machine signals | **SOURCE REVIEW REQUIRED** (PARTIAL epic) |
| E09-S03 | Implement station anomaly and wait flows | **SOURCE REVIEW REQUIRED** (PARTIAL epic) |
| E09-S04 | Build offline command journal and reconciliation | **SOURCE REVIEW REQUIRED** (PARTIAL epic) |
| E10-S01 | Build inspection/NCR with partial batch holds | **SOURCE REVIEW REQUIRED** (NOT-FOUND-IN-INSPECTED-SOURCE epic) |
| E10-S02 | Implement repair loop with independent check | **SOURCE REVIEW REQUIRED** (NOT-FOUND-IN-INSPECTED-SOURCE epic) |
| E10-S03 | Support finishing route variants | **SOURCE REVIEW REQUIRED** (NOT-FOUND-IN-INSPECTED-SOURCE epic) |
| E10-S04 | Bind buyer garment identifiers | **SOURCE REVIEW REQUIRED** (NOT-FOUND-IN-INSPECTED-SOURCE epic) |
| E11-S01 | Compile buyer-specific pack rule | **SOURCE REVIEW REQUIRED** (PARTIAL epic) |
| E11-S02 | Build UHF bag and barcode carton evidence | **SOURCE REVIEW REQUIRED** (PARTIAL epic) |
| E11-S03 | Create pack audit and carton seal gates | **SOURCE REVIEW REQUIRED** (PARTIAL epic) |
| E11-S04 | Implement manifest load and release | **SOURCE REVIEW REQUIRED** (PARTIAL epic) |
| E12-S01 | Implement stage-by-stage count conservation | **SOURCE REVIEW REQUIRED** (PARTIAL epic) |
| E12-S02 | Generate immutable order closure certificate | **SOURCE REVIEW REQUIRED** (PARTIAL epic) |
| E12-S03 | Compute material and conversion actual costs | **SOURCE REVIEW REQUIRED** (PARTIAL epic) |
| E12-S04 | Connect ERP financial handoff | **SOURCE REVIEW REQUIRED** (PARTIAL epic) |
| E13-S01 | Build event interval reconstruction | **SOURCE REVIEW REQUIRED** (PARTIAL epic) |
| E13-S02 | Separate queue touch wait rework and handoffs | **SOURCE REVIEW REQUIRED** (PARTIAL epic) |
| E13-S03 | Publish distributions and fairness guards | **SOURCE REVIEW REQUIRED** (PARTIAL epic) |
| E13-S04 | Build root-cause bottleneck view | **SOURCE REVIEW REQUIRED** (PARTIAL epic) |
| E14-S01 | Integrate absence and breakdown events | **SOURCE REVIEW REQUIRED** (PARTIAL epic) |
| E14-S02 | Compute critical path with finite machines and shifts | **SOURCE REVIEW REQUIRED** (PARTIAL epic) |
| E14-S03 | Generate controlled alternatives | **SOURCE REVIEW REQUIRED** (PARTIAL epic) |
| E14-S04 | Implement approval and rebaseline workflow | **SOURCE REVIEW REQUIRED** (PARTIAL epic) |
| E15-S01 | Create experiment registry and guardrails | **SOURCE REVIEW REQUIRED** (NOT-FOUND-IN-INSPECTED-SOURCE epic) |
| E15-S02 | Assign matched A/B exposure | **SOURCE REVIEW REQUIRED** (NOT-FOUND-IN-INSPECTED-SOURCE epic) |
| E15-S03 | Calculate treatment effect and uncertainty | **SOURCE REVIEW REQUIRED** (NOT-FOUND-IN-INSPECTED-SOURCE epic) |
| E15-S04 | Close the decision loop | **SOURCE REVIEW REQUIRED** (NOT-FOUND-IN-INSPECTED-SOURCE epic) |
| E16-S01 | Create grade-five station language pack | **SOURCE REVIEW REQUIRED** (PARTIAL epic) |
| E16-S02 | Build minimal tap-first station screens | **SOURCE REVIEW REQUIRED** (PARTIAL epic) |
| E16-S03 | Connect role worklists and action ownership | **SOURCE REVIEW REQUIRED** (PARTIAL epic) |
| E16-S04 | Build supervisor Gantt and factory action dashboard | **SOURCE REVIEW REQUIRED** (PARTIAL epic) |
| E17-S01 | Enforce per-aggregate transaction ownership | **SOURCE REVIEW REQUIRED** (PARTIAL epic) |
| E17-S02 | Implement durable outbox/inbox and replay | **SOURCE REVIEW REQUIRED** (PARTIAL epic) |
| E17-S03 | Qualify device clocks and local signed rules | **SOURCE REVIEW REQUIRED** (PARTIAL epic) |
| E17-S04 | Build recovery audit and release gates | **SOURCE REVIEW REQUIRED** (PARTIAL epic) |
| E18-S01 | Build plant install and nonintrusive rollback | **SOURCE REVIEW REQUIRED** (PARTIAL epic) |
| E18-S02 | Instrument service device connector and data health | **SOURCE REVIEW REQUIRED** (PARTIAL epic) |
| E18-S03 | Prepare golden factory fixtures | **SOURCE REVIEW REQUIRED** (PARTIAL epic) |
| E18-S04 | Run plant increment signoffs | **SOURCE REVIEW REQUIRED** (PARTIAL epic) |

## 4. New V5 epics and actionable stories
Every new story requires an idempotent API/command, state and schema migration, event/decision proof, exact negative tests, floor-appropriate UI, measured output, named approver and replay/rollback behavior. Story actions below are deliverable-specific, not reused placeholder tasks.
### E19 — Founder Governance & Material Control
#### E19-S01 — CEO-only exception signing  [V5 A2]
**Story:** As the accountable factory role, I can ceo-only exception signing so downstream production, financial and audit decisions remain correct.
**Business contract:** Bind authenticated CEO identity, written actual story, exact event hash, immutable permission constraint and nonce; refuse delegates and replay.
- **E19-S01-T01 — Domain/authority:** Define exact input identities, source-of-truth state, permission/CEO constraints, rejection codes and ownership.
- **E19-S01-T02 — Persistence/API:** Implement transactional schema, idempotency and domain actions for: Bind authenticated CEO identity, written actual story, exact event hash, immutable permission constraint and nonce; refuse delegates and replay.
- **E19-S01-T03 — Floor/manager UX:** Build role-specific happy path, blocked/unknown states, recovery and human-readable evidence presentation.
- **E19-S01-T04 — Invariants & adversarial tests:** Test concurrent requests, duplicate/replayed/offline input, wrong buyer/tenant, invalid revision/quantity and tamper resistance.
- **E19-S01-T05 — Pilot/acceptance:** Capture signed accepted and rejected fixtures, source snapshot, actual event trace, restart recovery and rollback result.
**Done when:** Actual authorized operation survives restart and replay, rejects violating evidence, propagates correct downstream state, and satisfies a recorded pilot acceptance test.

#### E19-S02 — Top Management Council shortage replan  [V5 A2]
**Story:** As the accountable factory role, I can top management council shortage replan so downstream production, financial and audit decisions remain correct.
**Business contract:** Create shortage docket, required council membership, meeting decision, impact snapshot, signed approved revision and downstream lock.
- **E19-S02-T01 — Domain/authority:** Define exact input identities, source-of-truth state, permission/CEO constraints, rejection codes and ownership.
- **E19-S02-T02 — Persistence/API:** Implement transactional schema, idempotency and domain actions for: Create shortage docket, required council membership, meeting decision, impact snapshot, signed approved revision and downstream lock.
- **E19-S02-T03 — Floor/manager UX:** Build role-specific happy path, blocked/unknown states, recovery and human-readable evidence presentation.
- **E19-S02-T04 — Invariants & adversarial tests:** Test concurrent requests, duplicate/replayed/offline input, wrong buyer/tenant, invalid revision/quantity and tamper resistance.
- **E19-S02-T05 — Pilot/acceptance:** Capture signed accepted and rejected fixtures, source snapshot, actual event trace, restart recovery and rollback result.
**Done when:** Actual authorized operation survives restart and replay, rejects violating evidence, propagates correct downstream state, and satisfies a recorded pilot acceptance test.

#### E19-S03 — Non-borrowing + atomic reservations  [V5 A1]
**Story:** As the accountable factory role, I can non-borrowing + atomic reservations so downstream production, financial and audit decisions remain correct.
**Business contract:** Enforce buyer ownership, order-specific reservations and no cross-PO lending at database transaction boundary.
- **E19-S03-T01 — Domain/authority:** Define exact input identities, source-of-truth state, permission/CEO constraints, rejection codes and ownership.
- **E19-S03-T02 — Persistence/API:** Implement transactional schema, idempotency and domain actions for: Enforce buyer ownership, order-specific reservations and no cross-PO lending at database transaction boundary.
- **E19-S03-T03 — Floor/manager UX:** Build role-specific happy path, blocked/unknown states, recovery and human-readable evidence presentation.
- **E19-S03-T04 — Invariants & adversarial tests:** Test concurrent requests, duplicate/replayed/offline input, wrong buyer/tenant, invalid revision/quantity and tamper resistance.
- **E19-S03-T05 — Pilot/acceptance:** Capture signed accepted and rejected fixtures, source snapshot, actual event trace, restart recovery and rollback result.
**Done when:** Actual authorized operation survives restart and replay, rejects violating evidence, propagates correct downstream state, and satisfies a recorded pilot acceptance test.

#### E19-S04 — Dual-witness physical custody  [V5 A3]
**Story:** As the accountable factory role, I can dual-witness physical custody so downstream production, financial and audit decisions remain correct.
**Business contract:** Verify issuer and recipient independently at transfer, physical identity, quantity and audit receipt; account for offline handoff.
- **E19-S04-T01 — Domain/authority:** Define exact input identities, source-of-truth state, permission/CEO constraints, rejection codes and ownership.
- **E19-S04-T02 — Persistence/API:** Implement transactional schema, idempotency and domain actions for: Verify issuer and recipient independently at transfer, physical identity, quantity and audit receipt; account for offline handoff.
- **E19-S04-T03 — Floor/manager UX:** Build role-specific happy path, blocked/unknown states, recovery and human-readable evidence presentation.
- **E19-S04-T04 — Invariants & adversarial tests:** Test concurrent requests, duplicate/replayed/offline input, wrong buyer/tenant, invalid revision/quantity and tamper resistance.
- **E19-S04-T05 — Pilot/acceptance:** Capture signed accepted and rejected fixtures, source snapshot, actual event trace, restart recovery and rollback result.
**Done when:** Actual authorized operation survives restart and replay, rejects violating evidence, propagates correct downstream state, and satisfies a recorded pilot acceptance test.

#### E19-S05 — Surplus-first indent hard gate  [V5 D1]
**Story:** As the accountable factory role, I can surplus-first indent hard gate so downstream production, financial and audit decisions remain correct.
**Business contract:** Apply six-dimensional eligibility, net requirements and approved exceptions before issuing PO.
- **E19-S05-T01 — Domain/authority:** Define exact input identities, source-of-truth state, permission/CEO constraints, rejection codes and ownership.
- **E19-S05-T02 — Persistence/API:** Implement transactional schema, idempotency and domain actions for: Apply six-dimensional eligibility, net requirements and approved exceptions before issuing PO.
- **E19-S05-T03 — Floor/manager UX:** Build role-specific happy path, blocked/unknown states, recovery and human-readable evidence presentation.
- **E19-S05-T04 — Invariants & adversarial tests:** Test concurrent requests, duplicate/replayed/offline input, wrong buyer/tenant, invalid revision/quantity and tamper resistance.
- **E19-S05-T05 — Pilot/acceptance:** Capture signed accepted and rejected fixtures, source snapshot, actual event trace, restart recovery and rollback result.
**Done when:** Actual authorized operation survives restart and replay, rejects violating evidence, propagates correct downstream state, and satisfies a recorded pilot acceptance test.

### E20 — Commercial Costing & Promise Ledger
#### E20-S01 — Commercial buyer and buying-house hierarchy  [V5 B1]
**Story:** As the accountable factory role, I can commercial buyer and buying-house hierarchy so downstream production, financial and audit decisions remain correct.
**Business contract:** Implement buyer agents, contact terms, compliance, claimed history, quote ownership and negotiation rounds.
- **E20-S01-T01 — Domain/authority:** Define exact input identities, source-of-truth state, permission/CEO constraints, rejection codes and ownership.
- **E20-S01-T02 — Persistence/API:** Implement transactional schema, idempotency and domain actions for: Implement buyer agents, contact terms, compliance, claimed history, quote ownership and negotiation rounds.
- **E20-S01-T03 — Floor/manager UX:** Build role-specific happy path, blocked/unknown states, recovery and human-readable evidence presentation.
- **E20-S01-T04 — Invariants & adversarial tests:** Test concurrent requests, duplicate/replayed/offline input, wrong buyer/tenant, invalid revision/quantity and tamper resistance.
- **E20-S01-T05 — Pilot/acceptance:** Capture signed accepted and rejected fixtures, source snapshot, actual event trace, restart recovery and rollback result.
**Done when:** Actual authorized operation survives restart and replay, rejects violating evidence, propagates correct downstream state, and satisfies a recorded pilot acceptance test.

#### E20-S02 — Four costing policies and granular CMT  [V5 B1]
**Story:** As the accountable factory role, I can four costing policies and granular cmt so downstream production, financial and audit decisions remain correct.
**Business contract:** Implement founder-approved costing policy alternatives; independently compute cutting/making/finishing and overheads.
- **E20-S02-T01 — Domain/authority:** Define exact input identities, source-of-truth state, permission/CEO constraints, rejection codes and ownership.
- **E20-S02-T02 — Persistence/API:** Implement transactional schema, idempotency and domain actions for: Implement founder-approved costing policy alternatives; independently compute cutting/making/finishing and overheads.
- **E20-S02-T03 — Floor/manager UX:** Build role-specific happy path, blocked/unknown states, recovery and human-readable evidence presentation.
- **E20-S02-T04 — Invariants & adversarial tests:** Test concurrent requests, duplicate/replayed/offline input, wrong buyer/tenant, invalid revision/quantity and tamper resistance.
- **E20-S02-T05 — Pilot/acceptance:** Capture signed accepted and rejected fixtures, source snapshot, actual event trace, restart recovery and rollback result.
**Done when:** Actual authorized operation survives restart and replay, rejects violating evidence, propagates correct downstream state, and satisfies a recorded pilot acceptance test.

#### E20-S03 — Versioned quote assumption ledger  [V5 B2]
**Story:** As the accountable factory role, I can versioned quote assumption ledger so downstream production, financial and audit decisions remain correct.
**Business contract:** Freeze OB SAM, fabric yield, supplier ETA, exchange rates, capex, margin and confidence on quote signature.
- **E20-S03-T01 — Domain/authority:** Define exact input identities, source-of-truth state, permission/CEO constraints, rejection codes and ownership.
- **E20-S03-T02 — Persistence/API:** Implement transactional schema, idempotency and domain actions for: Freeze OB SAM, fabric yield, supplier ETA, exchange rates, capex, margin and confidence on quote signature.
- **E20-S03-T03 — Floor/manager UX:** Build role-specific happy path, blocked/unknown states, recovery and human-readable evidence presentation.
- **E20-S03-T04 — Invariants & adversarial tests:** Test concurrent requests, duplicate/replayed/offline input, wrong buyer/tenant, invalid revision/quantity and tamper resistance.
- **E20-S03-T05 — Pilot/acceptance:** Capture signed accepted and rejected fixtures, source snapshot, actual event trace, restart recovery and rollback result.
**Done when:** Actual authorized operation survives restart and replay, rejects violating evidence, propagates correct downstream state, and satisfies a recorded pilot acceptance test.

#### E20-S04 — SAM change commercial-impact automation  [V5 B2]
**Story:** As the accountable factory role, I can sam change commercial-impact automation so downstream production, financial and audit decisions remain correct.
**Business contract:** Re-cost affected quoted/booked orders on approved OB revision and route consequential changes to merchandiser.
- **E20-S04-T01 — Domain/authority:** Define exact input identities, source-of-truth state, permission/CEO constraints, rejection codes and ownership.
- **E20-S04-T02 — Persistence/API:** Implement transactional schema, idempotency and domain actions for: Re-cost affected quoted/booked orders on approved OB revision and route consequential changes to merchandiser.
- **E20-S04-T03 — Floor/manager UX:** Build role-specific happy path, blocked/unknown states, recovery and human-readable evidence presentation.
- **E20-S04-T04 — Invariants & adversarial tests:** Test concurrent requests, duplicate/replayed/offline input, wrong buyer/tenant, invalid revision/quantity and tamper resistance.
- **E20-S04-T05 — Pilot/acceptance:** Capture signed accepted and rejected fixtures, source snapshot, actual event trace, restart recovery and rollback result.
**Done when:** Actual authorized operation survives restart and replay, rejects violating evidence, propagates correct downstream state, and satisfies a recorded pilot acceptance test.

#### E20-S05 — Margin evolution and variance attribution  [V5 B3]
**Story:** As the accountable factory role, I can margin evolution and variance attribution so downstream production, financial and audit decisions remain correct.
**Business contract:** Calculate quoted/booked/projected/final margins and distinguish estimate variance from execution variance.
- **E20-S05-T01 — Domain/authority:** Define exact input identities, source-of-truth state, permission/CEO constraints, rejection codes and ownership.
- **E20-S05-T02 — Persistence/API:** Implement transactional schema, idempotency and domain actions for: Calculate quoted/booked/projected/final margins and distinguish estimate variance from execution variance.
- **E20-S05-T03 — Floor/manager UX:** Build role-specific happy path, blocked/unknown states, recovery and human-readable evidence presentation.
- **E20-S05-T04 — Invariants & adversarial tests:** Test concurrent requests, duplicate/replayed/offline input, wrong buyer/tenant, invalid revision/quantity and tamper resistance.
- **E20-S05-T05 — Pilot/acceptance:** Capture signed accepted and rejected fixtures, source snapshot, actual event trace, restart recovery and rollback result.
**Done when:** Actual authorized operation survives restart and replay, rejects violating evidence, propagates correct downstream state, and satisfies a recorded pilot acceptance test.

### E21 — Factory Planning & Capacity Doctrine
#### E21-S01 — Three-layer immutable DPP  [V5 C1]
**Story:** As the accountable factory role, I can three-layer immutable dpp so downstream production, financial and audit decisions remain correct.
**Business contract:** Freeze IE baseline, factory-head reason-coded commitment and measured RFID actuals independently per shift.
- **E21-S01-T01 — Domain/authority:** Define exact input identities, source-of-truth state, permission/CEO constraints, rejection codes and ownership.
- **E21-S01-T02 — Persistence/API:** Implement transactional schema, idempotency and domain actions for: Freeze IE baseline, factory-head reason-coded commitment and measured RFID actuals independently per shift.
- **E21-S01-T03 — Floor/manager UX:** Build role-specific happy path, blocked/unknown states, recovery and human-readable evidence presentation.
- **E21-S01-T04 — Invariants & adversarial tests:** Test concurrent requests, duplicate/replayed/offline input, wrong buyer/tenant, invalid revision/quantity and tamper resistance.
- **E21-S01-T05 — Pilot/acceptance:** Capture signed accepted and rejected fixtures, source snapshot, actual event trace, restart recovery and rollback result.
**Done when:** Actual authorized operation survives restart and replay, rejects violating evidence, propagates correct downstream state, and satisfies a recorded pilot acceptance test.

#### E21-S02 — Eight-stream schedule with sewing anchor  [V5 C2]
**Story:** As the accountable factory role, I can eight-stream schedule with sewing anchor so downstream production, financial and audit decisions remain correct.
**Business contract:** Implement source-defined milestones and dependencies, separate packing trims, critical path and slip propagation.
- **E21-S02-T01 — Domain/authority:** Define exact input identities, source-of-truth state, permission/CEO constraints, rejection codes and ownership.
- **E21-S02-T02 — Persistence/API:** Implement transactional schema, idempotency and domain actions for: Implement source-defined milestones and dependencies, separate packing trims, critical path and slip propagation.
- **E21-S02-T03 — Floor/manager UX:** Build role-specific happy path, blocked/unknown states, recovery and human-readable evidence presentation.
- **E21-S02-T04 — Invariants & adversarial tests:** Test concurrent requests, duplicate/replayed/offline input, wrong buyer/tenant, invalid revision/quantity and tamper resistance.
- **E21-S02-T05 — Pilot/acceptance:** Capture signed accepted and rejected fixtures, source snapshot, actual event trace, restart recovery and rollback result.
**Done when:** Actual authorized operation survives restart and replay, rejects violating evidence, propagates correct downstream state, and satisfies a recorded pilot acceptance test.

#### E21-S03 — Factory-wide operator reassignment  [V5 C3]
**Story:** As the accountable factory role, I can factory-wide operator reassignment so downstream production, financial and audit decisions remain correct.
**Business contract:** Optimize net plant output, qualified skills, home line soft preference, movement burden and labor compliance.
- **E21-S03-T01 — Domain/authority:** Define exact input identities, source-of-truth state, permission/CEO constraints, rejection codes and ownership.
- **E21-S03-T02 — Persistence/API:** Implement transactional schema, idempotency and domain actions for: Optimize net plant output, qualified skills, home line soft preference, movement burden and labor compliance.
- **E21-S03-T03 — Floor/manager UX:** Build role-specific happy path, blocked/unknown states, recovery and human-readable evidence presentation.
- **E21-S03-T04 — Invariants & adversarial tests:** Test concurrent requests, duplicate/replayed/offline input, wrong buyer/tenant, invalid revision/quantity and tamper resistance.
- **E21-S03-T05 — Pilot/acceptance:** Capture signed accepted and rejected fixtures, source snapshot, actual event trace, restart recovery and rollback result.
**Done when:** Actual authorized operation survives restart and replay, rejects violating evidence, propagates correct downstream state, and satisfies a recorded pilot acceptance test.

#### E21-S04 — Machine procurement and qualification pipeline  [V5 C4]
**Story:** As the accountable factory role, I can machine procurement and qualification pipeline so downstream production, financial and audit decisions remain correct.
**Business contract:** Model shortage detection, capex alternatives, approval, ordered/received/commissioned state and readiness.
- **E21-S04-T01 — Domain/authority:** Define exact input identities, source-of-truth state, permission/CEO constraints, rejection codes and ownership.
- **E21-S04-T02 — Persistence/API:** Implement transactional schema, idempotency and domain actions for: Model shortage detection, capex alternatives, approval, ordered/received/commissioned state and readiness.
- **E21-S04-T03 — Floor/manager UX:** Build role-specific happy path, blocked/unknown states, recovery and human-readable evidence presentation.
- **E21-S04-T04 — Invariants & adversarial tests:** Test concurrent requests, duplicate/replayed/offline input, wrong buyer/tenant, invalid revision/quantity and tamper resistance.
- **E21-S04-T05 — Pilot/acceptance:** Capture signed accepted and rejected fixtures, source snapshot, actual event trace, restart recovery and rollback result.
**Done when:** Actual authorized operation survives restart and replay, rejects violating evidence, propagates correct downstream state, and satisfies a recorded pilot acceptance test.

#### E21-S05 — Skills training and outsourcing governance  [V5 C4]
**Story:** As the accountable factory role, I can skills training and outsourcing governance so downstream production, financial and audit decisions remain correct.
**Business contract:** Forecast skill deficits and onboarding; track subcontract justification, actual quality and recurring estimate drift.
- **E21-S05-T01 — Domain/authority:** Define exact input identities, source-of-truth state, permission/CEO constraints, rejection codes and ownership.
- **E21-S05-T02 — Persistence/API:** Implement transactional schema, idempotency and domain actions for: Forecast skill deficits and onboarding; track subcontract justification, actual quality and recurring estimate drift.
- **E21-S05-T03 — Floor/manager UX:** Build role-specific happy path, blocked/unknown states, recovery and human-readable evidence presentation.
- **E21-S05-T04 — Invariants & adversarial tests:** Test concurrent requests, duplicate/replayed/offline input, wrong buyer/tenant, invalid revision/quantity and tamper resistance.
- **E21-S05-T05 — Pilot/acceptance:** Capture signed accepted and rejected fixtures, source snapshot, actual event trace, restart recovery and rollback result.
**Done when:** Actual authorized operation survives restart and replay, rejects violating evidence, propagates correct downstream state, and satisfies a recorded pilot acceptance test.

### E22 — Cutting Exactness & Operator Contracts
#### E22-S01 — Classic cut-bundle profile generator  [V5 D2]
**Story:** As the accountable factory role, I can classic cut-bundle profile generator so downstream production, financial and audit decisions remain correct.
**Business contract:** Calculate cut/marker/part/ply groups, deterministic serials, consecutive ply validity and remainder redistribution.
- **E22-S01-T01 — Domain/authority:** Define exact input identities, source-of-truth state, permission/CEO constraints, rejection codes and ownership.
- **E22-S01-T02 — Persistence/API:** Implement transactional schema, idempotency and domain actions for: Calculate cut/marker/part/ply groups, deterministic serials, consecutive ply validity and remainder redistribution.
- **E22-S01-T03 — Floor/manager UX:** Build role-specific happy path, blocked/unknown states, recovery and human-readable evidence presentation.
- **E22-S01-T04 — Invariants & adversarial tests:** Test concurrent requests, duplicate/replayed/offline input, wrong buyer/tenant, invalid revision/quantity and tamper resistance.
- **E22-S01-T05 — Pilot/acceptance:** Capture signed accepted and rejected fixtures, source snapshot, actual event trace, restart recovery and rollback result.
**Done when:** Actual authorized operation survives restart and replay, rejects violating evidence, propagates correct downstream state, and satisfies a recorded pilot acceptance test.

#### E22-S02 — Cut Master frozen plan and permitted tweaks  [V5 D3]
**Story:** As the accountable factory role, I can cut master frozen plan and permitted tweaks so downstream production, financial and audit decisions remain correct.
**Business contract:** Authorize only sequence, in-shade roll, ply, table; enforce marker/reservation locks and Planning-replan unlock.
- **E22-S02-T01 — Domain/authority:** Define exact input identities, source-of-truth state, permission/CEO constraints, rejection codes and ownership.
- **E22-S02-T02 — Persistence/API:** Implement transactional schema, idempotency and domain actions for: Authorize only sequence, in-shade roll, ply, table; enforce marker/reservation locks and Planning-replan unlock.
- **E22-S02-T03 — Floor/manager UX:** Build role-specific happy path, blocked/unknown states, recovery and human-readable evidence presentation.
- **E22-S02-T04 — Invariants & adversarial tests:** Test concurrent requests, duplicate/replayed/offline input, wrong buyer/tenant, invalid revision/quantity and tamper resistance.
- **E22-S02-T05 — Pilot/acceptance:** Capture signed accepted and rejected fixtures, source snapshot, actual event trace, restart recovery and rollback result.
**Done when:** Actual authorized operation survives restart and replay, rejects violating evidence, propagates correct downstream state, and satisfies a recorded pilot acceptance test.

#### E22-S03 — Cursor-based RFID commissioning  [V5 D4]
**Story:** As the accountable factory role, I can cursor-based rfid commissioning so downstream production, financial and audit decisions remain correct.
**Business contract:** Print cards, scan one unused tag, bind to next expected bundle atomically, readback and cursor advance.
- **E22-S03-T01 — Domain/authority:** Define exact input identities, source-of-truth state, permission/CEO constraints, rejection codes and ownership.
- **E22-S03-T02 — Persistence/API:** Implement transactional schema, idempotency and domain actions for: Print cards, scan one unused tag, bind to next expected bundle atomically, readback and cursor advance.
- **E22-S03-T03 — Floor/manager UX:** Build role-specific happy path, blocked/unknown states, recovery and human-readable evidence presentation.
- **E22-S03-T04 — Invariants & adversarial tests:** Test concurrent requests, duplicate/replayed/offline input, wrong buyer/tenant, invalid revision/quantity and tamper resistance.
- **E22-S03-T05 — Pilot/acceptance:** Capture signed accepted and rejected fixtures, source snapshot, actual event trace, restart recovery and rollback result.
**Done when:** Actual authorized operation survives restart and replay, rejects violating evidence, propagates correct downstream state, and satisfies a recorded pilot acceptance test.

#### E22-S04 — Attachment-map compatibility enforcement  [V5 E3]
**Story:** As the accountable factory role, I can attachment-map compatibility enforcement so downstream production, financial and audit decisions remain correct.
**Business contract:** Compile approved OB part type and marker relationships; reject cross-garment/shade attachment credit.
- **E22-S04-T01 — Domain/authority:** Define exact input identities, source-of-truth state, permission/CEO constraints, rejection codes and ownership.
- **E22-S04-T02 — Persistence/API:** Implement transactional schema, idempotency and domain actions for: Compile approved OB part type and marker relationships; reject cross-garment/shade attachment credit.
- **E22-S04-T03 — Floor/manager UX:** Build role-specific happy path, blocked/unknown states, recovery and human-readable evidence presentation.
- **E22-S04-T04 — Invariants & adversarial tests:** Test concurrent requests, duplicate/replayed/offline input, wrong buyer/tenant, invalid revision/quantity and tamper resistance.
- **E22-S04-T05 — Pilot/acceptance:** Capture signed accepted and rejected fixtures, source snapshot, actual event trace, restart recovery and rollback result.
**Done when:** Actual authorized operation survives restart and replay, rejects violating evidence, propagates correct downstream state, and satisfies a recorded pilot acceptance test.

#### E22-S05 — Defective panel replacement identity  [V5 D5]
**Story:** As the accountable factory role, I can defective panel replacement identity so downstream production, financial and audit decisions remain correct.
**Business contract:** Create remnant-based replacement with original garment serial and links to rejected component.
- **E22-S05-T01 — Domain/authority:** Define exact input identities, source-of-truth state, permission/CEO constraints, rejection codes and ownership.
- **E22-S05-T02 — Persistence/API:** Implement transactional schema, idempotency and domain actions for: Create remnant-based replacement with original garment serial and links to rejected component.
- **E22-S05-T03 — Floor/manager UX:** Build role-specific happy path, blocked/unknown states, recovery and human-readable evidence presentation.
- **E22-S05-T04 — Invariants & adversarial tests:** Test concurrent requests, duplicate/replayed/offline input, wrong buyer/tenant, invalid revision/quantity and tamper resistance.
- **E22-S05-T05 — Pilot/acceptance:** Capture signed accepted and rejected fixtures, source snapshot, actual event trace, restart recovery and rollback result.
**Done when:** Actual authorized operation survives restart and replay, rejects violating evidence, propagates correct downstream state, and satisfies a recorded pilot acceptance test.

#### E22-S06 — Cut consumption reconciliation release gate  [V5 D6]
**Story:** As the accountable factory role, I can cut consumption reconciliation release gate so downstream production, financial and audit decisions remain correct.
**Business contract:** Balance reservations consumption returned remnants scrap, block out-of-tolerance release pending CEO.
- **E22-S06-T01 — Domain/authority:** Define exact input identities, source-of-truth state, permission/CEO constraints, rejection codes and ownership.
- **E22-S06-T02 — Persistence/API:** Implement transactional schema, idempotency and domain actions for: Balance reservations consumption returned remnants scrap, block out-of-tolerance release pending CEO.
- **E22-S06-T03 — Floor/manager UX:** Build role-specific happy path, blocked/unknown states, recovery and human-readable evidence presentation.
- **E22-S06-T04 — Invariants & adversarial tests:** Test concurrent requests, duplicate/replayed/offline input, wrong buyer/tenant, invalid revision/quantity and tamper resistance.
- **E22-S06-T05 — Pilot/acceptance:** Capture signed accepted and rejected fixtures, source snapshot, actual event trace, restart recovery and rollback result.
**Done when:** Actual authorized operation survives restart and replay, rejects violating evidence, propagates correct downstream state, and satisfies a recorded pilot acceptance test.

#### E22-S07 — Cut Master tweak learning measurements  [V5 D3]
**Story:** As the accountable factory role, I can cut master tweak learning measurements so downstream production, financial and audit decisions remain correct.
**Business contract:** Store recommended plan, precise deltas, override reasons, actual efficiency and non-punitive acceptance analytics.
- **E22-S07-T01 — Domain/authority:** Define exact input identities, source-of-truth state, permission/CEO constraints, rejection codes and ownership.
- **E22-S07-T02 — Persistence/API:** Implement transactional schema, idempotency and domain actions for: Store recommended plan, precise deltas, override reasons, actual efficiency and non-punitive acceptance analytics.
- **E22-S07-T03 — Floor/manager UX:** Build role-specific happy path, blocked/unknown states, recovery and human-readable evidence presentation.
- **E22-S07-T04 — Invariants & adversarial tests:** Test concurrent requests, duplicate/replayed/offline input, wrong buyer/tenant, invalid revision/quantity and tamper resistance.
- **E22-S07-T05 — Pilot/acceptance:** Capture signed accepted and rejected fixtures, source snapshot, actual event trace, restart recovery and rollback result.
**Done when:** Actual authorized operation survives restart and replay, rejects violating evidence, propagates correct downstream state, and satisfies a recorded pilot acceptance test.

### E23 — Operation Bulletin & Earned Credit
#### E23-S01 — Progressive OB document contract  [V5 E1]
**Story:** As the accountable factory role, I can progressive ob document contract so downstream production, financial and audit decisions remain correct.
**Business contract:** Model costing/planning/shift-time enrichment, owners, signatures, operation revisions and exact machine assignment.
- **E23-S01-T01 — Domain/authority:** Define exact input identities, source-of-truth state, permission/CEO constraints, rejection codes and ownership.
- **E23-S01-T02 — Persistence/API:** Implement transactional schema, idempotency and domain actions for: Model costing/planning/shift-time enrichment, owners, signatures, operation revisions and exact machine assignment.
- **E23-S01-T03 — Floor/manager UX:** Build role-specific happy path, blocked/unknown states, recovery and human-readable evidence presentation.
- **E23-S01-T04 — Invariants & adversarial tests:** Test concurrent requests, duplicate/replayed/offline input, wrong buyer/tenant, invalid revision/quantity and tamper resistance.
- **E23-S01-T05 — Pilot/acceptance:** Capture signed accepted and rejected fixtures, source snapshot, actual event trace, restart recovery and rollback result.
**Done when:** Actual authorized operation survives restart and replay, rejects violating evidence, propagates correct downstream state, and satisfies a recorded pilot acceptance test.

#### E23-S02 — Factory SAM evidence estimates  [V5 E2]
**Story:** As the accountable factory role, I can factory sam evidence estimates so downstream production, financial and audit decisions remain correct.
**Business contract:** Separate IE estimates from RFID-observed times, use robust sample uncertainty and context-specific validity.
- **E23-S02-T01 — Domain/authority:** Define exact input identities, source-of-truth state, permission/CEO constraints, rejection codes and ownership.
- **E23-S02-T02 — Persistence/API:** Implement transactional schema, idempotency and domain actions for: Separate IE estimates from RFID-observed times, use robust sample uncertainty and context-specific validity.
- **E23-S02-T03 — Floor/manager UX:** Build role-specific happy path, blocked/unknown states, recovery and human-readable evidence presentation.
- **E23-S02-T04 — Invariants & adversarial tests:** Test concurrent requests, duplicate/replayed/offline input, wrong buyer/tenant, invalid revision/quantity and tamper resistance.
- **E23-S02-T05 — Pilot/acceptance:** Capture signed accepted and rejected fixtures, source snapshot, actual event trace, restart recovery and rollback result.
**Done when:** Actual authorized operation survives restart and replay, rejects violating evidence, propagates correct downstream state, and satisfies a recorded pilot acceptance test.

#### E23-S03 — Operation-type production credit ledger  [V5 E3]
**Story:** As the accountable factory role, I can operation-type production credit ledger so downstream production, financial and audit decisions remain correct.
**Business contract:** Enforce S2S/B2B/B2S input output cardinalities, preconditions, dedupe and signed production credit.
- **E23-S03-T01 — Domain/authority:** Define exact input identities, source-of-truth state, permission/CEO constraints, rejection codes and ownership.
- **E23-S03-T02 — Persistence/API:** Implement transactional schema, idempotency and domain actions for: Enforce S2S/B2B/B2S input output cardinalities, preconditions, dedupe and signed production credit.
- **E23-S03-T03 — Floor/manager UX:** Build role-specific happy path, blocked/unknown states, recovery and human-readable evidence presentation.
- **E23-S03-T04 — Invariants & adversarial tests:** Test concurrent requests, duplicate/replayed/offline input, wrong buyer/tenant, invalid revision/quantity and tamper resistance.
- **E23-S03-T05 — Pilot/acceptance:** Capture signed accepted and rejected fixtures, source snapshot, actual event trace, restart recovery and rollback result.
**Done when:** Actual authorized operation survives restart and replay, rejects violating evidence, propagates correct downstream state, and satisfies a recorded pilot acceptance test.

#### E23-S04 — No-blame issue intervention outcomes  [V5 E4]
**Story:** As the accountable factory role, I can no-blame issue intervention outcomes so downstream production, financial and audit decisions remain correct.
**Business contract:** Track joiners, action, verification and recurrence for defect, breakdown and downtime interventions.
- **E23-S04-T01 — Domain/authority:** Define exact input identities, source-of-truth state, permission/CEO constraints, rejection codes and ownership.
- **E23-S04-T02 — Persistence/API:** Implement transactional schema, idempotency and domain actions for: Track joiners, action, verification and recurrence for defect, breakdown and downtime interventions.
- **E23-S04-T03 — Floor/manager UX:** Build role-specific happy path, blocked/unknown states, recovery and human-readable evidence presentation.
- **E23-S04-T04 — Invariants & adversarial tests:** Test concurrent requests, duplicate/replayed/offline input, wrong buyer/tenant, invalid revision/quantity and tamper resistance.
- **E23-S04-T05 — Pilot/acceptance:** Capture signed accepted and rejected fixtures, source snapshot, actual event trace, restart recovery and rollback result.
**Done when:** Actual authorized operation survives restart and replay, rejects violating evidence, propagates correct downstream state, and satisfies a recorded pilot acceptance test.

### E24 — Finishing, Packing & OCR
#### E24-S01 — Hangtag identity transition with tag return  [V5 F1]
**Story:** As the accountable factory role, I can hangtag identity transition with tag return so downstream production, financial and audit decisions remain correct.
**Business contract:** Atomically bind internal garment to unique buyer serial, retire carrier, quarantine reader error.
- **E24-S01-T01 — Domain/authority:** Define exact input identities, source-of-truth state, permission/CEO constraints, rejection codes and ownership.
- **E24-S01-T02 — Persistence/API:** Implement transactional schema, idempotency and domain actions for: Atomically bind internal garment to unique buyer serial, retire carrier, quarantine reader error.
- **E24-S01-T03 — Floor/manager UX:** Build role-specific happy path, blocked/unknown states, recovery and human-readable evidence presentation.
- **E24-S01-T04 — Invariants & adversarial tests:** Test concurrent requests, duplicate/replayed/offline input, wrong buyer/tenant, invalid revision/quantity and tamper resistance.
- **E24-S01-T05 — Pilot/acceptance:** Capture signed accepted and rejected fixtures, source snapshot, actual event trace, restart recovery and rollback result.
**Done when:** Actual authorized operation survives restart and replay, rejects violating evidence, propagates correct downstream state, and satisfies a recorded pilot acceptance test.

#### E24-S02 — Type-driven finishing redo and rescue rate  [V5 F2]
**Story:** As the accountable factory role, I can type-driven finishing redo and rescue rate so downstream production, financial and audit decisions remain correct.
**Business contract:** Route hangtag/ironing/sewing redo, independent reinspection, denominator-correct rescue rate.
- **E24-S02-T01 — Domain/authority:** Define exact input identities, source-of-truth state, permission/CEO constraints, rejection codes and ownership.
- **E24-S02-T02 — Persistence/API:** Implement transactional schema, idempotency and domain actions for: Route hangtag/ironing/sewing redo, independent reinspection, denominator-correct rescue rate.
- **E24-S02-T03 — Floor/manager UX:** Build role-specific happy path, blocked/unknown states, recovery and human-readable evidence presentation.
- **E24-S02-T04 — Invariants & adversarial tests:** Test concurrent requests, duplicate/replayed/offline input, wrong buyer/tenant, invalid revision/quantity and tamper resistance.
- **E24-S02-T05 — Pilot/acceptance:** Capture signed accepted and rejected fixtures, source snapshot, actual event trace, restart recovery and rollback result.
**Done when:** Actual authorized operation survives restart and replay, rejects violating evidence, propagates correct downstream state, and satisfies a recorded pilot acceptance test.

#### E24-S03 — Misscan negative-state contract  [V5 F3]
**Story:** As the accountable factory role, I can misscan negative-state contract so downstream production, financial and audit decisions remain correct.
**Business contract:** Invalid carton scan emits warning/audit while preserving member set, seal eligibility and current counts.
- **E24-S03-T01 — Domain/authority:** Define exact input identities, source-of-truth state, permission/CEO constraints, rejection codes and ownership.
- **E24-S03-T02 — Persistence/API:** Implement transactional schema, idempotency and domain actions for: Invalid carton scan emits warning/audit while preserving member set, seal eligibility and current counts.
- **E24-S03-T03 — Floor/manager UX:** Build role-specific happy path, blocked/unknown states, recovery and human-readable evidence presentation.
- **E24-S03-T04 — Invariants & adversarial tests:** Test concurrent requests, duplicate/replayed/offline input, wrong buyer/tenant, invalid revision/quantity and tamper resistance.
- **E24-S03-T05 — Pilot/acceptance:** Capture signed accepted and rejected fixtures, source snapshot, actual event trace, restart recovery and rollback result.
**Done when:** Actual authorized operation survives restart and replay, rejects violating evidence, propagates correct downstream state, and satisfies a recorded pilot acceptance test.

#### E24-S04 — Audited unseal and repack mode  [V5 F3]
**Story:** As the accountable factory role, I can audited unseal and repack mode so downstream production, financial and audit decisions remain correct.
**Business contract:** Preserve original membership baseline; require exact recheck or authorized replacement before resealing.
- **E24-S04-T01 — Domain/authority:** Define exact input identities, source-of-truth state, permission/CEO constraints, rejection codes and ownership.
- **E24-S04-T02 — Persistence/API:** Implement transactional schema, idempotency and domain actions for: Preserve original membership baseline; require exact recheck or authorized replacement before resealing.
- **E24-S04-T03 — Floor/manager UX:** Build role-specific happy path, blocked/unknown states, recovery and human-readable evidence presentation.
- **E24-S04-T04 — Invariants & adversarial tests:** Test concurrent requests, duplicate/replayed/offline input, wrong buyer/tenant, invalid revision/quantity and tamper resistance.
- **E24-S04-T05 — Pilot/acceptance:** Capture signed accepted and rejected fixtures, source snapshot, actual event trace, restart recovery and rollback result.
**Done when:** Actual authorized operation survives restart and replay, rejects violating evidence, propagates correct downstream state, and satisfies a recorded pilot acceptance test.

#### E24-S05 — Manifest decremental dock confirmation  [V5 F4]
**Story:** As the accountable factory role, I can manifest decremental dock confirmation so downstream production, financial and audit decisions remain correct.
**Business contract:** Enforce expected carton set, unique arrival scans, mismatch resolution, handoff authorization.
- **E24-S05-T01 — Domain/authority:** Define exact input identities, source-of-truth state, permission/CEO constraints, rejection codes and ownership.
- **E24-S05-T02 — Persistence/API:** Implement transactional schema, idempotency and domain actions for: Enforce expected carton set, unique arrival scans, mismatch resolution, handoff authorization.
- **E24-S05-T03 — Floor/manager UX:** Build role-specific happy path, blocked/unknown states, recovery and human-readable evidence presentation.
- **E24-S05-T04 — Invariants & adversarial tests:** Test concurrent requests, duplicate/replayed/offline input, wrong buyer/tenant, invalid revision/quantity and tamper resistance.
- **E24-S05-T05 — Pilot/acceptance:** Capture signed accepted and rejected fixtures, source snapshot, actual event trace, restart recovery and rollback result.
**Done when:** Actual authorized operation survives restart and replay, rejects violating evidence, propagates correct downstream state, and satisfies a recorded pilot acceptance test.

#### E24-S06 — Immutable six-party OCR proof  [V5 F5]
**Story:** As the accountable factory role, I can immutable six-party ocr proof so downstream production, financial and audit decisions remain correct.
**Business contract:** Freeze stage balances, evidence hash, correction addenda and binding six signoffs before certificate.
- **E24-S06-T01 — Domain/authority:** Define exact input identities, source-of-truth state, permission/CEO constraints, rejection codes and ownership.
- **E24-S06-T02 — Persistence/API:** Implement transactional schema, idempotency and domain actions for: Freeze stage balances, evidence hash, correction addenda and binding six signoffs before certificate.
- **E24-S06-T03 — Floor/manager UX:** Build role-specific happy path, blocked/unknown states, recovery and human-readable evidence presentation.
- **E24-S06-T04 — Invariants & adversarial tests:** Test concurrent requests, duplicate/replayed/offline input, wrong buyer/tenant, invalid revision/quantity and tamper resistance.
- **E24-S06-T05 — Pilot/acceptance:** Capture signed accepted and rejected fixtures, source snapshot, actual event trace, restart recovery and rollback result.
**Done when:** Actual authorized operation survives restart and replay, rejects violating evidence, propagates correct downstream state, and satisfies a recorded pilot acceptance test.

### E25 — Platform Adoption & Productization
#### E25-S01 — RFID qualification gate for production  [V5 G1]
**Story:** As the accountable factory role, I can rfid qualification gate for production so downstream production, financial and audit decisions remain correct.
**Business contract:** Station zone mapping, tag encoding profile, validation evidence and M0/M1 legacy shadow exceptions.
- **E25-S01-T01 — Domain/authority:** Define exact input identities, source-of-truth state, permission/CEO constraints, rejection codes and ownership.
- **E25-S01-T02 — Persistence/API:** Implement transactional schema, idempotency and domain actions for: Station zone mapping, tag encoding profile, validation evidence and M0/M1 legacy shadow exceptions.
- **E25-S01-T03 — Floor/manager UX:** Build role-specific happy path, blocked/unknown states, recovery and human-readable evidence presentation.
- **E25-S01-T04 — Invariants & adversarial tests:** Test concurrent requests, duplicate/replayed/offline input, wrong buyer/tenant, invalid revision/quantity and tamper resistance.
- **E25-S01-T05 — Pilot/acceptance:** Capture signed accepted and rejected fixtures, source snapshot, actual event trace, restart recovery and rollback result.
**Done when:** Actual authorized operation survives restart and replay, rejects violating evidence, propagates correct downstream state, and satisfies a recorded pilot acceptance test.

#### E25-S02 — Tag rotation and loss economics  [V5 G2]
**Story:** As the accountable factory role, I can tag rotation and loss economics so downstream production, financial and audit decisions remain correct.
**Business contract:** Tag inventory life cycles, circulation delay, reader failures, pool sizing and attrition alerts.
- **E25-S02-T01 — Domain/authority:** Define exact input identities, source-of-truth state, permission/CEO constraints, rejection codes and ownership.
- **E25-S02-T02 — Persistence/API:** Implement transactional schema, idempotency and domain actions for: Tag inventory life cycles, circulation delay, reader failures, pool sizing and attrition alerts.
- **E25-S02-T03 — Floor/manager UX:** Build role-specific happy path, blocked/unknown states, recovery and human-readable evidence presentation.
- **E25-S02-T04 — Invariants & adversarial tests:** Test concurrent requests, duplicate/replayed/offline input, wrong buyer/tenant, invalid revision/quantity and tamper resistance.
- **E25-S02-T05 — Pilot/acceptance:** Capture signed accepted and rejected fixtures, source snapshot, actual event trace, restart recovery and rollback result.
**Done when:** Actual authorized operation survives restart and replay, rejects violating evidence, propagates correct downstream state, and satisfies a recorded pilot acceptance test.

#### E25-S03 — Multi-vendor CAD interchange  [V5 G3]
**Story:** As the accountable factory role, I can multi-vendor cad interchange so downstream production, financial and audit decisions remain correct.
**Business contract:** Versioned marker candidate adapter interface with DXF and tested Gerber/Optitex/Lectra profiles.
- **E25-S03-T01 — Domain/authority:** Define exact input identities, source-of-truth state, permission/CEO constraints, rejection codes and ownership.
- **E25-S03-T02 — Persistence/API:** Implement transactional schema, idempotency and domain actions for: Versioned marker candidate adapter interface with DXF and tested Gerber/Optitex/Lectra profiles.
- **E25-S03-T03 — Floor/manager UX:** Build role-specific happy path, blocked/unknown states, recovery and human-readable evidence presentation.
- **E25-S03-T04 — Invariants & adversarial tests:** Test concurrent requests, duplicate/replayed/offline input, wrong buyer/tenant, invalid revision/quantity and tamper resistance.
- **E25-S03-T05 — Pilot/acceptance:** Capture signed accepted and rejected fixtures, source snapshot, actual event trace, restart recovery and rollback result.
**Done when:** Actual authorized operation survives restart and replay, rejects violating evidence, propagates correct downstream state, and satisfies a recorded pilot acceptance test.

#### E25-S04 — Native-script device and printing support  [V5 G4]
**Story:** As the accountable factory role, I can native-script device and printing support so downstream production, financial and audit decisions remain correct.
**Business contract:** Translation glossary, RTL, barcode label print fonts, Android floor device testing and operator comprehension.
- **E25-S04-T01 — Domain/authority:** Define exact input identities, source-of-truth state, permission/CEO constraints, rejection codes and ownership.
- **E25-S04-T02 — Persistence/API:** Implement transactional schema, idempotency and domain actions for: Translation glossary, RTL, barcode label print fonts, Android floor device testing and operator comprehension.
- **E25-S04-T03 — Floor/manager UX:** Build role-specific happy path, blocked/unknown states, recovery and human-readable evidence presentation.
- **E25-S04-T04 — Invariants & adversarial tests:** Test concurrent requests, duplicate/replayed/offline input, wrong buyer/tenant, invalid revision/quantity and tamper resistance.
- **E25-S04-T05 — Pilot/acceptance:** Capture signed accepted and rejected fixtures, source snapshot, actual event trace, restart recovery and rollback result.
**Done when:** Actual authorized operation survives restart and replay, rejects violating evidence, propagates correct downstream state, and satisfies a recorded pilot acceptance test.

#### E25-S05 — Requirement source and maturity matrix  [V5 G5]
**Story:** As the accountable factory role, I can requirement source and maturity matrix so downstream production, financial and audit decisions remain correct.
**Business contract:** Tag requirements F/S/V4/D and VALIDATED/DESIGNED/PROPOSED/FUTURE with signed evidence.
- **E25-S05-T01 — Domain/authority:** Define exact input identities, source-of-truth state, permission/CEO constraints, rejection codes and ownership.
- **E25-S05-T02 — Persistence/API:** Implement transactional schema, idempotency and domain actions for: Tag requirements F/S/V4/D and VALIDATED/DESIGNED/PROPOSED/FUTURE with signed evidence.
- **E25-S05-T03 — Floor/manager UX:** Build role-specific happy path, blocked/unknown states, recovery and human-readable evidence presentation.
- **E25-S05-T04 — Invariants & adversarial tests:** Test concurrent requests, duplicate/replayed/offline input, wrong buyer/tenant, invalid revision/quantity and tamper resistance.
- **E25-S05-T05 — Pilot/acceptance:** Capture signed accepted and rejected fixtures, source snapshot, actual event trace, restart recovery and rollback result.
**Done when:** Actual authorized operation survives restart and replay, rejects violating evidence, propagates correct downstream state, and satisfies a recorded pilot acceptance test.

## 5. Explicit technical debt — remediation tickets
| ID | Priority | Observed debt | Required remediation | Source |
|---|---|---|---|---|
| TD01 | P0 | **Service topology misleading.** 28 of the 29 numbered s01–s29 service folders have zero files; s01 has SQL migration only | Create architecture decision: modular monolith now or implement bounded services; remove claims of 29 deployed microservices; assign actual ownership. | `services/s01-identity; services/s02-device-registry..s29-federation` |
| TD02 | P0 | **Volatile business state.** Server holds stores, packing, OCR, attachments and config in process maps; restart loses non-ledger state | Persist transactional aggregates and rebuild from event log; crash/restart evidence tests. | `services/edge-daemon/cmd/server/main.go` |
| TD03 | P0 | **Demo roll and pilot config in runtime.** ROLL-007 and Noble Garments pilot details are registered as runtime defaults | Use tenant-scoped onboarding fixtures only; production startup must not synthesize stock or orders. | `services/edge-daemon/cmd/server/main.go` |
| TD04 | P0 | **Weak authority lease.** Pure CanCommit checks scopes/writer but itself documents that it is not a distributed lock/lease | Enforce lease epoch, expiry and single-writer uniqueness in transactional database; fail closed offline. | `services/edge-daemon/pkg/brownfield/authority.go` |
| TD05 | P0 | **Witness attestation insufficient.** Two string IDs and HMAC shared secret are insufficient proof that two persons authenticated separately | Bind separate authentication sessions/challenges, approved device and proof timestamps; sign ledger receipt. | `services/edge-daemon/pkg/stores/issuance.go` |
| TD06 | P0 | **OCR mutable signoff contents.** Signoffs are role strings; doc hash covers only five scalar values; no complete immutable source snapshot | Hash canonical report, dispositions, underlying event revision and signer identity; re-sign on alteration. | `services/edge-daemon/internal/ocr/signoff.go` |
| TD07 | P0 | **In-memory ingestion job registry.** Jobs map process-local; GET explicitly reports expiration on restart | Persist queue/status/result and job ownership; recover interrupted jobs on restart. | `services/edge-daemon/cmd/server/planning_upload_jobs.go` |
| TD08 | P0 | **CI build-only release.** GitHub release workflow builds edge binary but does not run test/vet/coverage/migration/tenant isolation checks | Introduce required PR CI and release attestation, test matrix, migration, secret scanning and SBOM. | `.github/workflows/release.yml` |
| TD09 | P0 | **Unverified build in this audit.** Go test could not execute: local toolchain attempted external go1.24 download, network unavailable | Run pinned Go 1.24 offline-ready CI, record go test/race/vet/staticcheck/build results and reproducible image digest. | `go.mod; audit environment` |
| TD10 | P0 | **Tenant authentication and RLS integration.** Migrations and middleware exist but path-by-path authorization could not be verified statically | Inventory every mutating API, prove authenticated tenant claims and DB RLS enforced; hostile cross-tenant tests. | `services/core-api/internal/identity; services/core-api/migrations` |
| TD11 | P1 | **Frontend coverage concentrated.** Four static HTML HMIs found; source lacks complete role-based application for specified screen inventory | Build screen-to-route matrix, mark real/wired/mock, implement missing role journeys and device accessibility tests. | `services/edge-daemon/internal/web/static` |
| TD12 | P1 | **Service contract and integration drift.** V4 backlog uses owner service labels with many unimplemented directories; APIs partly under monolith | Generate OpenAPI/contracts from actual router; validate expected V5 APIs with consumer tests and API versioning. | `services/edge-daemon/cmd/server/main.go` |
| TD13 | P1 | **No verified hardware commissioning tests.** Reader adapter and simulation exist, but no source-supported live reader vendor/pilot evidence | Add field calibration, duplicate/stray read rejection, offline sync, tag recycle cycle and instrumented acceptance. | `services/edge-daemon/pkg/rfid` |
| TD14 | P1 | **WAL/DB consistency unresolved.** Local WAL replays ledger events; multiple business aggregates remain volatile | Design atomic business outbox/inbox and loss/replay invariants; fault-injection tests on crash boundaries. | `services/edge-daemon/internal/wal; services/edge-daemon/cmd/server/main.go` |
| TD15 | P1 | **Document intake trust boundary.** Multipart upload and external extraction command path need hostile file and tenant isolation threat review | Sandbox documents, size/decompression quotas, AV detection, prompt-injection handling, evidence citations and cancellation. | `services/edge-daemon/cmd/server/planning_upload_jobs.go` |
| TD16 | P1 | **No recorded pilot evidence bundle.** Source contains tests and mockups, not signed plant validation for 29 service or all V5 workflows | Version golden PO fixtures, physical scans, signed acceptance, latency, source coverage and rollback evidence. | `tests/integration; docs` |

### TD01 — Service topology misleading
**Evidence:** 28 of the 29 numbered s01–s29 service folders have zero files; s01 has SQL migration only. **Files:** `services/s01-identity; services/s02-device-registry..s29-federation`.
- **TD01-T01:** Reproduce issue and add failing test at identified boundary, with negative and crash/retry cases.
- **TD01-T02:** Create architecture decision: modular monolith now or implement bounded services; remove claims of 29 deployed microservices; assign actual ownership.
- **TD01-T03:** Add integration and security/tenant regressions, document migration/rollback and collect CI/pilot logs.
**Exit:** Independent reviewer can reproduce baseline problem, prove fixed invariant, and observe recovery without data corruption.

### TD02 — Volatile business state
**Evidence:** Server holds stores, packing, OCR, attachments and config in process maps; restart loses non-ledger state. **Files:** `services/edge-daemon/cmd/server/main.go`.
- **TD02-T01:** Reproduce issue and add failing test at identified boundary, with negative and crash/retry cases.
- **TD02-T02:** Persist transactional aggregates and rebuild from event log; crash/restart evidence tests.
- **TD02-T03:** Add integration and security/tenant regressions, document migration/rollback and collect CI/pilot logs.
**Exit:** Independent reviewer can reproduce baseline problem, prove fixed invariant, and observe recovery without data corruption.

### TD03 — Demo roll and pilot config in runtime
**Evidence:** ROLL-007 and Noble Garments pilot details are registered as runtime defaults. **Files:** `services/edge-daemon/cmd/server/main.go`.
- **TD03-T01:** Reproduce issue and add failing test at identified boundary, with negative and crash/retry cases.
- **TD03-T02:** Use tenant-scoped onboarding fixtures only; production startup must not synthesize stock or orders.
- **TD03-T03:** Add integration and security/tenant regressions, document migration/rollback and collect CI/pilot logs.
**Exit:** Independent reviewer can reproduce baseline problem, prove fixed invariant, and observe recovery without data corruption.

### TD04 — Weak authority lease
**Evidence:** Pure CanCommit checks scopes/writer but itself documents that it is not a distributed lock/lease. **Files:** `services/edge-daemon/pkg/brownfield/authority.go`.
- **TD04-T01:** Reproduce issue and add failing test at identified boundary, with negative and crash/retry cases.
- **TD04-T02:** Enforce lease epoch, expiry and single-writer uniqueness in transactional database; fail closed offline.
- **TD04-T03:** Add integration and security/tenant regressions, document migration/rollback and collect CI/pilot logs.
**Exit:** Independent reviewer can reproduce baseline problem, prove fixed invariant, and observe recovery without data corruption.

### TD05 — Witness attestation insufficient
**Evidence:** Two string IDs and HMAC shared secret are insufficient proof that two persons authenticated separately. **Files:** `services/edge-daemon/pkg/stores/issuance.go`.
- **TD05-T01:** Reproduce issue and add failing test at identified boundary, with negative and crash/retry cases.
- **TD05-T02:** Bind separate authentication sessions/challenges, approved device and proof timestamps; sign ledger receipt.
- **TD05-T03:** Add integration and security/tenant regressions, document migration/rollback and collect CI/pilot logs.
**Exit:** Independent reviewer can reproduce baseline problem, prove fixed invariant, and observe recovery without data corruption.

### TD06 — OCR mutable signoff contents
**Evidence:** Signoffs are role strings; doc hash covers only five scalar values; no complete immutable source snapshot. **Files:** `services/edge-daemon/internal/ocr/signoff.go`.
- **TD06-T01:** Reproduce issue and add failing test at identified boundary, with negative and crash/retry cases.
- **TD06-T02:** Hash canonical report, dispositions, underlying event revision and signer identity; re-sign on alteration.
- **TD06-T03:** Add integration and security/tenant regressions, document migration/rollback and collect CI/pilot logs.
**Exit:** Independent reviewer can reproduce baseline problem, prove fixed invariant, and observe recovery without data corruption.

### TD07 — In-memory ingestion job registry
**Evidence:** Jobs map process-local; GET explicitly reports expiration on restart. **Files:** `services/edge-daemon/cmd/server/planning_upload_jobs.go`.
- **TD07-T01:** Reproduce issue and add failing test at identified boundary, with negative and crash/retry cases.
- **TD07-T02:** Persist queue/status/result and job ownership; recover interrupted jobs on restart.
- **TD07-T03:** Add integration and security/tenant regressions, document migration/rollback and collect CI/pilot logs.
**Exit:** Independent reviewer can reproduce baseline problem, prove fixed invariant, and observe recovery without data corruption.

### TD08 — CI build-only release
**Evidence:** GitHub release workflow builds edge binary but does not run test/vet/coverage/migration/tenant isolation checks. **Files:** `.github/workflows/release.yml`.
- **TD08-T01:** Reproduce issue and add failing test at identified boundary, with negative and crash/retry cases.
- **TD08-T02:** Introduce required PR CI and release attestation, test matrix, migration, secret scanning and SBOM.
- **TD08-T03:** Add integration and security/tenant regressions, document migration/rollback and collect CI/pilot logs.
**Exit:** Independent reviewer can reproduce baseline problem, prove fixed invariant, and observe recovery without data corruption.

### TD09 — Unverified build in this audit
**Evidence:** Go test could not execute: local toolchain attempted external go1.24 download, network unavailable. **Files:** `go.mod; audit environment`.
- **TD09-T01:** Reproduce issue and add failing test at identified boundary, with negative and crash/retry cases.
- **TD09-T02:** Run pinned Go 1.24 offline-ready CI, record go test/race/vet/staticcheck/build results and reproducible image digest.
- **TD09-T03:** Add integration and security/tenant regressions, document migration/rollback and collect CI/pilot logs.
**Exit:** Independent reviewer can reproduce baseline problem, prove fixed invariant, and observe recovery without data corruption.

### TD10 — Tenant authentication and RLS integration
**Evidence:** Migrations and middleware exist but path-by-path authorization could not be verified statically. **Files:** `services/core-api/internal/identity; services/core-api/migrations`.
- **TD10-T01:** Reproduce issue and add failing test at identified boundary, with negative and crash/retry cases.
- **TD10-T02:** Inventory every mutating API, prove authenticated tenant claims and DB RLS enforced; hostile cross-tenant tests.
- **TD10-T03:** Add integration and security/tenant regressions, document migration/rollback and collect CI/pilot logs.
**Exit:** Independent reviewer can reproduce baseline problem, prove fixed invariant, and observe recovery without data corruption.

### TD11 — Frontend coverage concentrated
**Evidence:** Four static HTML HMIs found; source lacks complete role-based application for specified screen inventory. **Files:** `services/edge-daemon/internal/web/static`.
- **TD11-T01:** Reproduce issue and add failing test at identified boundary, with negative and crash/retry cases.
- **TD11-T02:** Build screen-to-route matrix, mark real/wired/mock, implement missing role journeys and device accessibility tests.
- **TD11-T03:** Add integration and security/tenant regressions, document migration/rollback and collect CI/pilot logs.
**Exit:** Independent reviewer can reproduce baseline problem, prove fixed invariant, and observe recovery without data corruption.

### TD12 — Service contract and integration drift
**Evidence:** V4 backlog uses owner service labels with many unimplemented directories; APIs partly under monolith. **Files:** `services/edge-daemon/cmd/server/main.go`.
- **TD12-T01:** Reproduce issue and add failing test at identified boundary, with negative and crash/retry cases.
- **TD12-T02:** Generate OpenAPI/contracts from actual router; validate expected V5 APIs with consumer tests and API versioning.
- **TD12-T03:** Add integration and security/tenant regressions, document migration/rollback and collect CI/pilot logs.
**Exit:** Independent reviewer can reproduce baseline problem, prove fixed invariant, and observe recovery without data corruption.

### TD13 — No verified hardware commissioning tests
**Evidence:** Reader adapter and simulation exist, but no source-supported live reader vendor/pilot evidence. **Files:** `services/edge-daemon/pkg/rfid`.
- **TD13-T01:** Reproduce issue and add failing test at identified boundary, with negative and crash/retry cases.
- **TD13-T02:** Add field calibration, duplicate/stray read rejection, offline sync, tag recycle cycle and instrumented acceptance.
- **TD13-T03:** Add integration and security/tenant regressions, document migration/rollback and collect CI/pilot logs.
**Exit:** Independent reviewer can reproduce baseline problem, prove fixed invariant, and observe recovery without data corruption.

### TD14 — WAL/DB consistency unresolved
**Evidence:** Local WAL replays ledger events; multiple business aggregates remain volatile. **Files:** `services/edge-daemon/internal/wal; services/edge-daemon/cmd/server/main.go`.
- **TD14-T01:** Reproduce issue and add failing test at identified boundary, with negative and crash/retry cases.
- **TD14-T02:** Design atomic business outbox/inbox and loss/replay invariants; fault-injection tests on crash boundaries.
- **TD14-T03:** Add integration and security/tenant regressions, document migration/rollback and collect CI/pilot logs.
**Exit:** Independent reviewer can reproduce baseline problem, prove fixed invariant, and observe recovery without data corruption.

### TD15 — Document intake trust boundary
**Evidence:** Multipart upload and external extraction command path need hostile file and tenant isolation threat review. **Files:** `services/edge-daemon/cmd/server/planning_upload_jobs.go`.
- **TD15-T01:** Reproduce issue and add failing test at identified boundary, with negative and crash/retry cases.
- **TD15-T02:** Sandbox documents, size/decompression quotas, AV detection, prompt-injection handling, evidence citations and cancellation.
- **TD15-T03:** Add integration and security/tenant regressions, document migration/rollback and collect CI/pilot logs.
**Exit:** Independent reviewer can reproduce baseline problem, prove fixed invariant, and observe recovery without data corruption.

### TD16 — No recorded pilot evidence bundle
**Evidence:** Source contains tests and mockups, not signed plant validation for 29 service or all V5 workflows. **Files:** `tests/integration; docs`.
- **TD16-T01:** Reproduce issue and add failing test at identified boundary, with negative and crash/retry cases.
- **TD16-T02:** Version golden PO fixtures, physical scans, signed acceptance, latency, source coverage and rollback evidence.
- **TD16-T03:** Add integration and security/tenant regressions, document migration/rollback and collect CI/pilot logs.
**Exit:** Independent reviewer can reproduce baseline problem, prove fixed invariant, and observe recovery without data corruption.

## 6. Prioritized vertical implementation sequence
- **Gate 0 — Build trust (P0):** TD01/TD02/TD03/TD04/TD08/TD09/TD10. Produce reproducible build, portable deployment, transaction ownership and evidence of existing behavior before feature expansion.
- **Gate 1 — Observe and preserve identities (P0):** E01/E02/E17/E18, E25 RFID qualification. Legacy unchanged in M0/M1, real tags gate M3/M4, tenant/source truth explicit.
- **Gate 2 — Buyer to realistic production commitment (P0):** E03/E04 plus E20 quote, E21 DPP; ensure approved documents, actual inventory, IE/manpower/machine capacity yield explainable constraints.
- **Gate 3 — Govern stock and cutting (P0):** E05/E06/E19/E22. Verify no-borrowing, CEO signing, double witnesses, classic bundle arithmetic, cut reconciliation, tag handoff.
- **Gate 4 — Earned credit and finish-to-ship (P0):** E07–E12/E23/E24. Execute physical scan → admissible credit → independent QC → buyer identity → pack/repack → manifest → closure.
- **Gate 5 — Continuous optimization (P1):** E13–E15, workforce/capex/skills, experiments and margin effects with protected holdout baselines.

## 7. Nonnegotiable verification matrix
| Requirement | Positive evidence | Required negative / resilience proof |
|---|---|---|
| RFID gate | commissioned live read with authorized source | missing/duplicate/stray tag cannot invent earned credit |
| No-borrowing | PO A reserved roll used only on approved cut | PO B urgent allocation rejected under concurrent requests |
| CEO-only approval | CEO signature bound to exception/hash | manager delegation, forged/replayed CEO proof blocked |
| Dual witness | independent issuer/receiver sessions completed | same-session identity and unreceived handoff blocked |
| Three-layer DPP | IE baseline, manager commit and actual independently queryable | late edits cannot rewrite baseline |
| Bundle arithmetic | marker-position × part × consecutive ply mapping reconciles | orphan/out-of-shade/bad-mapping rejected |
| Earned SAM | accepted operation timing with clock provenance | passive UHF presence not treated as operator cycle |
| Packing/repack | original set verified and sealed | misscan leaves carton unchanged; unverified repack blocked |
| OCR | immutable approved reconciled snapshot with six signatures | late stock/event mutation cannot silently change sealed report |
| Outbox replay | one accepted physical event after restart | duplicates and network partitions do not double count |
| Tenant isolation | authorized user sees own buyer/order | cross-tenant ID swap rejected end-to-end |
| Brownfield rollback | shadow observer removed without ERP changes | no implicit operational control when lease absent |

## 8. Code-complete definition and reporting template
A story is **CODE COMPLETE** only after verified production-bound logic, durable data migration, authorized API, integrated UI/device workflow, business invariants, negative/concurrency tests, crash/replay/backup recovery, operator comprehension, observable telemetry, rollout/rollback, and signed fixture evidence. Code-complete is separate from PILOT-VALIDATED and PLANT-PRODUCTION-READY.
**Status vocabulary:** ABSENT / PARTIAL / IMPLEMENTED-UNVERIFIED / CODE-COMPLETE / PILOT-VALIDATED / PRODUCTION-ACCEPTED, plus BLOCKED-VERIFICATION. Each status requires commit SHA, exact files, test command/result, fixture, evidence link, approver and date.

**Recommended dashboard fields:** `ticket_id`, `priority`, `source_requirement`, `tenant_and_plant_scope`, `code_owner`, `repo_commit`, `actual_files`, `status`, `test_pass`, `negative_test_pass`, `live_device_evidence`, `migration_id`, `dependency`, `defect`, `operational_owner`, `signed_acceptance`.

## 9. Review notes and caveats
- The inspected code snapshot may lag a live deployed environment; no production system credentials or deployed service inventory were provided. Evidence here is limited to the attached archive.
- Empty numbered service folders are decisive evidence that those folders do not implement independently deployable services in this snapshot. They do not imply that no corresponding business logic exists elsewhere in the edge monolith or `pkg`.
- A test unable to execute due to missing toolchain/network is recorded as blocked, never reported as failing or passing.
- Do not translate unverified planned scope into % completion or calendar delivery estimates.
- V5 founder locks and Classic Cut-Bundle Profile are mandatory as specified; proposed optimization and SAM confidence heuristics still require founder/IE approval.
