# AGIPAREL — MASTER EPICS / STORIES / TASKS v5.3

**Factory-first normative overlay, 26 October 2026.** Existing v5.2 epic/backlog content is preserved below. These `FFO-*` stories have precedence on onboarding sequence and customer UX. Mark maturity precisely: B1 is coded but requires Mac/DB acceptance; B2/C remain planned.

## Epic FFO: Empathetic factory-first discovery before buyer operations

| ID | Priority | Story and acceptance | Status |
|---|---|---|---|
| FFO-01 | P0 | The first navigation CTA leads to a guided factory-first workspace; users can explain partial factory knowledge without worksheets or onboarding jargon | B1 code |
| FFO-02 | P0 | One centralized heterogeneous original-file intake handles factory and buyer documents together; suggestion is never silent approval | Gate 06.5-A + B1 hints |
| FFO-03 | P0 | Typechecked append-only factory observations with `PROVISIONAL/CONFIRMED`, source reference, version and non-destructive correction | B1 code |
| FFO-04 | P0 | A one-question-at-a-time guided wizard assists factory profile, machines/capabilities, workforce/skills, raw materials/stock, rates, calendar and quality | B1 deterministic question priority; AI dialog planned |
| FFO-05 | P0 | Per-action readiness distinguishes exploration, draft planning, cost baseline, material inventory and operational release, without a global onboarding blocker | B1 evidence checks; full authoritative gates unchanged |
| FFO-06 | P0 | Mixed, missing, incompatible or unknown data show explicit suggested next steps; user can skip and return, never silently replace with zero | B1 code, refine B2 |
| FFO-07 | P0 | Normalize source-backed individual Machine/Capability, WorkerRole/Skill, Shift, Material/Stock, CostRate entities with effective dates and reviewed provenance | Planned B2 |
| FFO-08 | P0 | Source document candidate classification spans factory vs buyer, with human correction, conflict detection and file-format adapters including legacy XLS, OCR and cloud sources | B1 basic hints; planned B2 |
| FFO-09 | P0 | Buyer PO, technical pack, designs, packing, calendars and shipping rules linked only after buyer semantic review; many POs per style | Planned B2/C |
| FFO-10 | P0 | Reconcile factory machines, skills, stock, rates, calendars with an approved buyer requirement; report gaps/assumptions and a best next question | Planned C |
| FFO-11 | P0 | AI-to-SQL uses tenant-scoped, bounded, typed semantic queries with citation and missing/conflict evidence; no LLM operational writes | Planned C and 12 |
| FFO-12 | P0 | Regression suite covers stale writes, tenant leakage, rates/UOM truth, buyer-order intake before factory completion, and missing critical facts without blocking exploration | B1 partial tests; expand B2/C |

**B1 installation gate:** migration `0010_factory_discovery_v65b.sql`, Go HTTP methods, embedded `/factory-setup`, operator navigation, public homepage CTA and initial Go regression tests. Do not call B1 production-ready or complete factory-master-data onboarding. Employee PII and payroll remain out of scope for weakly authenticated development.

---

## Retained v5.2 backlog (no content removed)

# AGIPAREL FACTORY OS — MASTER EPICS / STORIES / TASKS v5.2

**Date:** 26 October 2026
**Status:** Proposed backlog reconciliation / rollout plan.
**Paired PRD/ARD:** `Agiparel_Master_PRD_ARD_v5.2.md` (V5.2-1–10 control).
**Backlog identity:** E01–E25 and the original **109 source stories** are retained below without renumbering. The new `DGI-*` work packages are explicit acceptance/delivery refinements mapped to existing epics; **do not count them as newly approved functional stories until groomed**.

## V5.2 backlog delta — priority P0 before Batch 07

### Fixed DEV environment conventions

Use only `AGIPAREL_OPERATOR_USER=dev`, `AGIPAREL_OPERATOR_PASSWORD=1212`, `AGIPAREL_OPERATOR_TENANT=dev` in disposable development instructions and fixtures. Tenant ID is not another login; no client-supplied tenant scope. Do not substitute `DEV-TENANT` or `dev-operator`. This is **development-only**; production authorization remains separately gated.

### Work packages and acceptance

| ID / epic crosswalk | Work to deliver (ordered) | Definition of done / user-visible evidence |
|---|---|---|
| **DGI-01** E02/E17 | Pin one request tenant across Order Intake, Material Planning, Planning, AGIE and future doc service | Every module sees the same `dev` tenant dataset; wrong-tenant uploads/queries denied; `TENANT_DEFAULT` removed from authorized paths; mixed legacy rows handled explicitly |
| **DGI-02** E02/E03/E17 | Shared `document-intake` service + Go gateway routes, PostgreSQL durable job state and content-addressed tenant-local original storage | Upload 14 source files, return individual receipts; jobs survive service restart; bounded retries and reupload idempotency; original SHA-256 retrievable |
| **DGI-03** E02/E03 | Universal MIME/signature-driven dispatch and error/quarantine behavior | PDF, XLSX, XLS, DOCX, CSV, PNG/JPEG first; DOC/ODS/RTF/TXT/JSON/XML/TSV/EDI/PPTX via declared adapter; unsupported and encrypted documents accurately labeled, never silently accepted |
| **DGI-04** E03 | Deterministic source extraction for pages, sheets, cells, layout, tables, formulas and images | Stored source coordinates; mixed PDF with selectable+scanned pages; sheet/cell source probes; no formula execution; no mixed-type value loss |
| **DGI-05** E03 | OCR/layout+vision fallback and typed assertions with model/rule lineage | Confidence per *fact*, regions/cells rendered in reviewer UI; extraction quality measured against gold samples; unresolved facts kept on HOLD |
| **DGI-06** E03/E17 | Review/conflict/version engine; switch off legacy Planning auto-merge | Human explicit approve/reject/correct with before/after and audit; no upload directly modifies authorized planning/stores; competing PO/tech-pack versions visible |
| **DGI-07** E02/E03 | Canonical normalized buyer/PO/PO-revision/PO-line/style-revision/SKU/BOM/OB/pack/measurement schema | 4 Walmart POs remain distinct; 2-piece bags not confused with garments/cartons; tenant-safe FKs; conversions exact; migration preserves Batch 05/06 source artifacts |
| **DGI-08** E04/E16 | Customer Document Workspace inside existing website | One upload -> progress -> extracted fields with click-to-source -> conflicts -> scoped review -> automatic staged-order selector; no mandatory CSV conversion |
| **DGI-09** E02/E03 | Authenticated Google Docs/Sheets source adapters | Connect/import authorized source and persist immutable export with revision ID; no bare URL server fetch; update/reimport produces explicit new revision |
| **DGI-10** E04/E17 | Approved semantic read tools and query metrics, reject unrestricted AI SQL for authoritative answers | Typed template/validated AST results with tenant, revision, units, source IDs and absent-data flags; questions cannot access other tenant or silently mix revisions |
| **DGI-11** E03/E04 | Real-customer benchmark and AI failure taxonomy | 14-file fixture, >=40 source-answered questions, confusion tests for SO/PO/style, gross/net/stock, cartons/bags, operation SAM, ambiguity, no-data; report per-question provenance and failures |
| **DGI-12** E18/E17 | Dedicated deployment, test/observability and rollback | docker compose changes only Agiparel services; service health, per-adapter failure, restart/replay, DB migrations and tenant isolation verified; zero effect on other apps |

### Gate 06.5 delivery splits

**06.5-A — Foundational usable upload (DGI-01, 02, 03 first adapters, 06 bypass, 08 initial page, 12)**

1. Add `document-intake` service in `compose/docker-compose.dev.yml` with no changes to unrelated Docker stacks; Go gateway proxies authenticated tenant scope. Choose a bounded converter/OCR runtime with constrained resources and no macro execution; reuse existing extraction utilities where safe.
2. Define `source_artifact`, `source_version`, `intake_job`, `document_rendition`, `document_segment`, `document_assertion`, `document_review` migrations (names tentative) with tenant-safe keys, original SHA and idempotency.
3. Implement multipart multi-file upload, MIME/signature validation, immutable storage, durable state and job polling; support PDF text, XLSX, DOCX, CSV initially; browser shows partial vs complete and download/view original.
4. Fail-close the old Planning route from automatic `ingest_docs.go` to `onboarding.ApplyToDatabase` merges; wire `AGIPAREL_CAPTURE_FIRST=true` in Compose **until** the legacy endpoint calls the new service.
5. Demonstrate the same staged document receipt after service restart and repeat upload; preserve existing Batch 05/06 screens.

**06.5-B — Customer semantic accuracy (DGI-03 remaining, 04, 05, 06, 07, 08, 09)**

1. OCR/scanned PDF and image adapter with page-region citations; legacy XLS and DOC converter in sandbox; understand image/table-heavy tech packs without asserting full accuracy.
2. Typed assertions and per-assertion source references, conditional/section context and UOM; distinguish no-data from zero and formulas from cached values.
3. Model and version PO/PO-line/style/variant/BOM/packing/OB/measurement; one file may contain multiple POs. Add explicit compare, correct and approve UI; no autonomous promotion.
4. Google Docs/Sheets import only through connected, authorized export APIs; document snapshot metadata, revision and reimport protocol.
5. Demonstrate all 14 real-file imports at least to supported/partial-with-reason, correct 4 Walmart POs, operation bulletins, style/BOM and planning stock arithmetic where evidenced.

**06.5-C — Reuse and trusted answers (DGI-07, 08, 10, 11, 12)**

1. Approved canonical revisions become `Order Intake` selectable records and `Material Planning` inputs without requesting new CSVs. Preserve Batch 05/06 compatibility until migrated; prove source provenance in both.
2. Build semantic metrics/query API: ordered garments, style variant quantity, gross/net material need, stock authority, polybag count vs garments/cartons, OB SAM. Enforce tenant, version, status, units and source IDs before LLM answer synthesis.
3. Run >=40 golden questions with explicit expected values, citations, ambiguous answers and no-answer cases. Record model, parser, prompt, rule and schema revisions. Report **unknown/failed**, not just pass count.
4. Prove reruns and errors don't issue stock, alter authoritative PO/BOM, cross tenant boundary or misrepresent simulated plans as observed physical truth.

### Roadmap after gate (no feature deletion)

| Batch | Business deliverable | V5.2 dependency |
|---|---|---|
| 07 | Genuine inspected stores ledger, passports, eligibility, reservations, independent two-witness issuance and founder exceptions | Approved versioned PO/BOM and sources + order-linked commitments |
| 08 | Marker, lay, classic physical cut bundle, RFID genealogy and conservation | Approved cut plan + size/color variants + source changes |
| 09 | OB import, operation bulletin version, SAM, sewing RFID credit and offline replay | Extracted/approved real XLSX/XLS OB and operation mappings |
| 10 | Inspection/defect/repair/finishing, hangtag identities | Approved measurements, tolerances, labels and style revisions |
| 11 | Garment/bag/carton/manifest packing and closure | Buyer-specific approved packing specification; separate 2-piece RFID bags |
| 12 | Factory-wide observed lead/queue times, bottlenecks, conservation and trustworthy AI answers + pilot | Trusted event/ledger timeline, semantic metrics, real-file gold benchmark |

### Common definition of done for every DGI work package

- Database migration, RLS isolation, reversible failure/rollback method, version and provenance checks.
- API documentation (OpenAPI or equivalent), typed contract, defensive idempotency and bounded error handling.
- Customer-visible UI with accessible no-data / partial / needs-review / conflicting / approved states.
- Evidence-based automated tests and smoke testing on user's Go 1.24 iMac; static repo evidence alone is marked **IMPLEMENTED–UNVERIFIED**.
- No side effects on unrelated Docker containers, databases or volumes; never reset actual customer records to make tests pass.
- Demonstrate a user value at each deployment; do not ship only backend infrastructure for weeks.

---

# RETAINED v5.1 BACKLOG — IN FULL

All v5.1 stories and tasks are retained verbatim for cross-version traceability. V5.2 DGI refinements above control when more specific.

# AGIPAREL FACTORY OS — MASTER ENGINEERING EPICS, STORIES, TASKS v5.1

**Author:** Sunil Jain, CyberinQ LLC
**Date:** 26 October 2026
**Basis:** paired `Agiparel_Master_PRD_ARD_v5.1.md`; original 72-story V4 backlog, first V5 37-story expansion and second V5 26-story alternative; static code audit of `agiparel(6).zip` (commit `7f00d11`, 2026-10-25) as reported in supplied first V5 backlog.
**Status:** source-merger delivery baseline, NOT a claim of build completion; no fresh deployment/hardware test conducted for this merge.

## 0. Backlog truth, precedence, and numbering

- **Canonical epic IDs E01–E25** preserve all 18 V4 epics and seven first-V5 epics. The alternate E00–E07 are merged into these canonical tickets or technical-debt TD tickets, not opened as duplicate work. Every alternative story has an explicit crosswalk.
- **Source stories:** 72 original V4 + 37 expanded V5 = **109 unique functional stories**. Alternate 26 story variants are incorporated as additional acceptance details and tasks; none should be mistaken for 26 new independently schedulable features.
- **Status:** all story statuses remain SOURCE REVIEW REQUIRED / PARTIAL until exact call graph, durable state and integration proof exist. The earlier audit assesses epics from one dated archive; it is not a current deployed status report.
- **Priority:** P0 safety, evidence, privacy, governance or physical correctness gate; P1 full operating cycle/economic loop; P2 differentiated research. Numeric performance/timing/threshold defaults from either flavor are PROPOSED unless measured and signed.
- **Implementation owner:** actual source file path and commit SHA must be filled after a new `git` audit; logical Sxx labels refer to bounded contexts, not necessarily deployable services. Do not turn 28 empty directories into 28 empty microservices merely to satisfy a drawing.
- **Definition of CODE COMPLETE:** durable business transaction with proper authority, DDL migration/reversal, documented API/event and idempotency, attached HMI, hostile tenant/negative/race/offline/restart tests, telemetry, rollback and reference fixture. **PILOT VALIDATED** additionally requires real plant/hardware/user observations and signed acceptance; **PRODUCTION ACCEPTED** requires plant release gates.

## 1. Code audit — evidence-scoped baseline


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
| E07 — Tag hardware and lifecycle | **PARTIAL** | `services/edge-daemon/pkg/rfid/sgtin.go`, `llrp_reader.go`; reader/tag lifecycle hardware commissioning unverified |
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


### Reconciliation of two audit interpretations

The second audit independently highlights possible duplicated `pkg/` and edge package trees, `TENANT_DEFAULT`/`Line 14`/`KK524867`/`MK6574J9` fallbacks, synthetic 2D marker nesting, incomplete ROS/credit/QA flows and missing cross-system integration. These are **investigation targets**, not newly executed tests in this merge. Its “15–20% complete” estimate is **not adopted**: coverage percentages without denominator and verified integration are misleading. Similarly, its statement of “zero integration tests” conflicts with the first audit's observation of some integration test files; the correct claim is **no verified end-to-end plant acceptance**. The Go test command was BLOCKED by missing Go 1.24 download/network, not failed.

## 2. Delivery architecture and story-wide tasks

Every story below preserves the original source's tasks. In addition, implement all these mandatory engineering gates, which are **not** a substitute for deliverable-specific tasks:

1. **Contract & data:** authoritative aggregate, tenant/plant boundary, lifecycle and policy revision; reversible migration and units/identity semantics.
2. **Transaction & event:** conditional compare-and-swap/serializable custody as needed, idempotent command, atomic outbox event with documented replay/fencing.
3. **UI & device:** exact persona, reader/station/permission, one primary action, pending/quarantine/denied recovery, native-script path and offline affordance.
4. **Tests:** positive, wrong buyer, wrong material/shade, duplicate, concurrent, stale revision, restore/replay, read outage, clock skew, cross-tenant exploit.
5. **Independent acceptance:** reference factory order/roll/garment fixture, scanned physical tally, source logs, operator time, verifier/approver, named release gate and rollback evidence.

Production account creation, wage calculation, native CAD geometry modification, automated personnel punishment and unaudited safety machine commands are not implied by these stories.

## 3. Original V4 implementation epics E01–E18 — *full individual stories retained*


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

**Merged production-verification elaboration (required):** Observe-only import must not change legacy production aggregates; concurrent writer and lease-expiry tests.

- **E01-S01-T04 — Stateful integration:** identify the authoritative aggregate and durable migration, exact API handler/source path and outbox mapping; record nonauthoritative legacy observations separately.
- **E01-S01-T05 — Negative and replay fixture:** reproduce the stated original acceptance condition and its failure counterpart; prove tenant isolation, idempotent duplicates, concurrency and restart preservation at a named database transaction boundary.
- **E01-S01-T06 — Worker/pilot proof:** capture the relevant operator or manager role action, device/source evidence, pending/blocked UI, measured accepted output and reviewer signature. Mark IMPLEMENTED-UNVERIFIED until proved.


#### E01-S02 — Implement shadow observation mode

**Story:** As a relevant factory user, I need to capture reads/imports without creating operational writes, so that work is explainable, safe and actionable.
**Acceptance:** Legacy outputs unchanged after full pilot shift.
**Dependencies:** Prior schema/source contracts in S28 / S01 / core; existing installed legacy and device capabilities must be discovered.
**Delivery tasks:**

- **E01-S02-T01 Domain/API:** Define source-of-truth, typed inputs/outputs, idempotency, permissions, state transitions and failure contracts.
- **E01-S02-T02 Implementation/UI:** Implement owning logic and integration adapter, expose a role-appropriate action and clear conflict/pending/approved states.
- **E01-S02-T03 Evidence/pilot:** Run positive, negative, replay and business-invariant checks using a named pilot fixture; capture acceptance evidence and rollback procedure.

**Merged production-verification elaboration (required):** Observe-only import must not change legacy production aggregates; concurrent writer and lease-expiry tests.

- **E01-S02-T04 — Stateful integration:** identify the authoritative aggregate and durable migration, exact API handler/source path and outbox mapping; record nonauthoritative legacy observations separately.
- **E01-S02-T05 — Negative and replay fixture:** reproduce the stated original acceptance condition and its failure counterpart; prove tenant isolation, idempotent duplicates, concurrency and restart preservation at a named database transaction boundary.
- **E01-S02-T06 — Worker/pilot proof:** capture the relevant operator or manager role action, device/source evidence, pending/blocked UI, measured accepted output and reviewer signature. Mark IMPLEMENTED-UNVERIFIED until proved.


#### E01-S03 — Introduce scoped ownership transitions

**Story:** As a relevant factory user, I need to define per-object scope, authority leases, handoff and rollback, so that work is explainable, safe and actionable.
**Acceptance:** At most one authoritative writer at every transition.
**Dependencies:** Prior schema/source contracts in S28 / S01 / core; existing installed legacy and device capabilities must be discovered.
**Delivery tasks:**

- **E01-S03-T01 Domain/API:** Define source-of-truth, typed inputs/outputs, idempotency, permissions, state transitions and failure contracts.
- **E01-S03-T02 Implementation/UI:** Implement owning logic and integration adapter, expose a role-appropriate action and clear conflict/pending/approved states.
- **E01-S03-T03 Evidence/pilot:** Run positive, negative, replay and business-invariant checks using a named pilot fixture; capture acceptance evidence and rollback procedure.

**Merged production-verification elaboration (required):** Observe-only import must not change legacy production aggregates; concurrent writer and lease-expiry tests.

- **E01-S03-T04 — Stateful integration:** identify the authoritative aggregate and durable migration, exact API handler/source path and outbox mapping; record nonauthoritative legacy observations separately.
- **E01-S03-T05 — Negative and replay fixture:** reproduce the stated original acceptance condition and its failure counterpart; prove tenant isolation, idempotent duplicates, concurrency and restart preservation at a named database transaction boundary.
- **E01-S03-T06 — Worker/pilot proof:** capture the relevant operator or manager role action, device/source evidence, pending/blocked UI, measured accepted output and reviewer signature. Mark IMPLEMENTED-UNVERIFIED until proved.


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

**Merged production-verification elaboration (required):** Observe-only import must not change legacy production aggregates; concurrent writer and lease-expiry tests.

- **E01-S04-T04 — Stateful integration:** identify the authoritative aggregate and durable migration, exact API handler/source path and outbox mapping; record nonauthoritative legacy observations separately.
- **E01-S04-T05 — Negative and replay fixture:** reproduce the stated original acceptance condition and its failure counterpart; prove tenant isolation, idempotent duplicates, concurrency and restart preservation at a named database transaction boundary.
- **E01-S04-T06 — Worker/pilot proof:** capture the relevant operator or manager role action, device/source evidence, pending/blocked UI, measured accepted output and reviewer signature. Mark IMPLEMENTED-UNVERIFIED until proved.


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

**Merged production-verification elaboration (required):** Replay the same CSV/XLSX/EDI record with changed upstream IDs; prove typed units, original checksum and deterministic canonical mapping.

- **E02-S01-T04 — Stateful integration:** identify the authoritative aggregate and durable migration, exact API handler/source path and outbox mapping; record nonauthoritative legacy observations separately.
- **E02-S01-T05 — Negative and replay fixture:** reproduce the stated original acceptance condition and its failure counterpart; prove tenant isolation, idempotent duplicates, concurrency and restart preservation at a named database transaction boundary.
- **E02-S01-T06 — Worker/pilot proof:** capture the relevant operator or manager role action, device/source evidence, pending/blocked UI, measured accepted output and reviewer signature. Mark IMPLEMENTED-UNVERIFIED until proved.


#### E02-S02 — Create external ID resolver

**Story:** As a relevant factory user, I need to canonical UUID aliases and effective-dated mappings, so that work is explainable, safe and actionable.
**Acceptance:** No silent conflation of PO/style/roll IDs.
**Dependencies:** Prior schema/source contracts in S28 / S03; existing installed legacy and device capabilities must be discovered.
**Delivery tasks:**

- **E02-S02-T01 Domain/API:** Define source-of-truth, typed inputs/outputs, idempotency, permissions, state transitions and failure contracts.
- **E02-S02-T02 Implementation/UI:** Implement owning logic and integration adapter, expose a role-appropriate action and clear conflict/pending/approved states.
- **E02-S02-T03 Evidence/pilot:** Run positive, negative, replay and business-invariant checks using a named pilot fixture; capture acceptance evidence and rollback procedure.

**Merged production-verification elaboration (required):** Replay the same CSV/XLSX/EDI record with changed upstream IDs; prove typed units, original checksum and deterministic canonical mapping.

- **E02-S02-T04 — Stateful integration:** identify the authoritative aggregate and durable migration, exact API handler/source path and outbox mapping; record nonauthoritative legacy observations separately.
- **E02-S02-T05 — Negative and replay fixture:** reproduce the stated original acceptance condition and its failure counterpart; prove tenant isolation, idempotent duplicates, concurrency and restart preservation at a named database transaction boundary.
- **E02-S02-T06 — Worker/pilot proof:** capture the relevant operator or manager role action, device/source evidence, pending/blocked UI, measured accepted output and reviewer signature. Mark IMPLEMENTED-UNVERIFIED until proved.


#### E02-S03 — Build material and machine master import

**Story:** As a relevant factory user, I need to normalize widths units machine types schedules and skill references, so that work is explainable, safe and actionable.
**Acceptance:** Critical units validated and reversible.
**Dependencies:** Prior schema/source contracts in S28 / S03; existing installed legacy and device capabilities must be discovered.
**Delivery tasks:**

- **E02-S03-T01 Domain/API:** Define source-of-truth, typed inputs/outputs, idempotency, permissions, state transitions and failure contracts.
- **E02-S03-T02 Implementation/UI:** Implement owning logic and integration adapter, expose a role-appropriate action and clear conflict/pending/approved states.
- **E02-S03-T03 Evidence/pilot:** Run positive, negative, replay and business-invariant checks using a named pilot fixture; capture acceptance evidence and rollback procedure.

**Merged production-verification elaboration (required):** Replay the same CSV/XLSX/EDI record with changed upstream IDs; prove typed units, original checksum and deterministic canonical mapping.

- **E02-S03-T04 — Stateful integration:** identify the authoritative aggregate and durable migration, exact API handler/source path and outbox mapping; record nonauthoritative legacy observations separately.
- **E02-S03-T05 — Negative and replay fixture:** reproduce the stated original acceptance condition and its failure counterpart; prove tenant isolation, idempotent duplicates, concurrency and restart preservation at a named database transaction boundary.
- **E02-S03-T06 — Worker/pilot proof:** capture the relevant operator or manager role action, device/source evidence, pending/blocked UI, measured accepted output and reviewer signature. Mark IMPLEMENTED-UNVERIFIED until proved.


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

**Merged production-verification elaboration (required):** Replay the same CSV/XLSX/EDI record with changed upstream IDs; prove typed units, original checksum and deterministic canonical mapping.

- **E02-S04-T04 — Stateful integration:** identify the authoritative aggregate and durable migration, exact API handler/source path and outbox mapping; record nonauthoritative legacy observations separately.
- **E02-S04-T05 — Negative and replay fixture:** reproduce the stated original acceptance condition and its failure counterpart; prove tenant isolation, idempotent duplicates, concurrency and restart preservation at a named database transaction boundary.
- **E02-S04-T06 — Worker/pilot proof:** capture the relevant operator or manager role action, device/source evidence, pending/blocked UI, measured accepted output and reviewer signature. Mark IMPLEMENTED-UNVERIFIED until proved.


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

**Merged production-verification elaboration (required):** Contradictory techpack revision and mismatched BOM must remain proposed, with region citations and signed human publication.

- **E03-S01-T04 — Stateful integration:** identify the authoritative aggregate and durable migration, exact API handler/source path and outbox mapping; record nonauthoritative legacy observations separately.
- **E03-S01-T05 — Negative and replay fixture:** reproduce the stated original acceptance condition and its failure counterpart; prove tenant isolation, idempotent duplicates, concurrency and restart preservation at a named database transaction boundary.
- **E03-S01-T06 — Worker/pilot proof:** capture the relevant operator or manager role action, device/source evidence, pending/blocked UI, measured accepted output and reviewer signature. Mark IMPLEMENTED-UNVERIFIED until proved.


#### E03-S02 — Build typed document relationships

**Story:** As a relevant factory user, I need to connect buyer PO line style fabric BOM route machine pack and terms, so that work is explainable, safe and actionable.
**Acceptance:** Graph detects inconsistent identifiers and revisions.
**Dependencies:** Prior schema/source contracts in S03 / S04 / S05; existing installed legacy and device capabilities must be discovered.
**Delivery tasks:**

- **E03-S02-T01 Domain/API:** Define source-of-truth, typed inputs/outputs, idempotency, permissions, state transitions and failure contracts.
- **E03-S02-T02 Implementation/UI:** Implement owning logic and integration adapter, expose a role-appropriate action and clear conflict/pending/approved states.
- **E03-S02-T03 Evidence/pilot:** Run positive, negative, replay and business-invariant checks using a named pilot fixture; capture acceptance evidence and rollback procedure.

**Merged production-verification elaboration (required):** Contradictory techpack revision and mismatched BOM must remain proposed, with region citations and signed human publication.

- **E03-S02-T04 — Stateful integration:** identify the authoritative aggregate and durable migration, exact API handler/source path and outbox mapping; record nonauthoritative legacy observations separately.
- **E03-S02-T05 — Negative and replay fixture:** reproduce the stated original acceptance condition and its failure counterpart; prove tenant isolation, idempotent duplicates, concurrency and restart preservation at a named database transaction boundary.
- **E03-S02-T06 — Worker/pilot proof:** capture the relevant operator or manager role action, device/source evidence, pending/blocked UI, measured accepted output and reviewer signature. Mark IMPLEMENTED-UNVERIFIED until proved.


#### E03-S03 — Implement conflict and approval workflow

**Story:** As a relevant factory user, I need to field-scoped precedence with owner signatures and effective intervals, so that work is explainable, safe and actionable.
**Acceptance:** Wrong-fit or material OB cannot auto-publish.
**Dependencies:** Prior schema/source contracts in S03 / S04 / S05; existing installed legacy and device capabilities must be discovered.
**Delivery tasks:**

- **E03-S03-T01 Domain/API:** Define source-of-truth, typed inputs/outputs, idempotency, permissions, state transitions and failure contracts.
- **E03-S03-T02 Implementation/UI:** Implement owning logic and integration adapter, expose a role-appropriate action and clear conflict/pending/approved states.
- **E03-S03-T03 Evidence/pilot:** Run positive, negative, replay and business-invariant checks using a named pilot fixture; capture acceptance evidence and rollback procedure.

**Merged production-verification elaboration (required):** Contradictory techpack revision and mismatched BOM must remain proposed, with region citations and signed human publication.

- **E03-S03-T04 — Stateful integration:** identify the authoritative aggregate and durable migration, exact API handler/source path and outbox mapping; record nonauthoritative legacy observations separately.
- **E03-S03-T05 — Negative and replay fixture:** reproduce the stated original acceptance condition and its failure counterpart; prove tenant isolation, idempotent duplicates, concurrency and restart preservation at a named database transaction boundary.
- **E03-S03-T06 — Worker/pilot proof:** capture the relevant operator or manager role action, device/source evidence, pending/blocked UI, measured accepted output and reviewer signature. Mark IMPLEMENTED-UNVERIFIED until proved.


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

**Merged production-verification elaboration (required):** Contradictory techpack revision and mismatched BOM must remain proposed, with region citations and signed human publication.

- **E03-S04-T04 — Stateful integration:** identify the authoritative aggregate and durable migration, exact API handler/source path and outbox mapping; record nonauthoritative legacy observations separately.
- **E03-S04-T05 — Negative and replay fixture:** reproduce the stated original acceptance condition and its failure counterpart; prove tenant isolation, idempotent duplicates, concurrency and restart preservation at a named database transaction boundary.
- **E03-S04-T06 — Worker/pilot proof:** capture the relevant operator or manager role action, device/source evidence, pending/blocked UI, measured accepted output and reviewer signature. Mark IMPLEMENTED-UNVERIFIED until proved.


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

**Merged production-verification elaboration (required):** Expired inventory/skill snapshot must produce blocking uncertainty, not plausible invented feasibility.

- **E04-S01-T04 — Stateful integration:** identify the authoritative aggregate and durable migration, exact API handler/source path and outbox mapping; record nonauthoritative legacy observations separately.
- **E04-S01-T05 — Negative and replay fixture:** reproduce the stated original acceptance condition and its failure counterpart; prove tenant isolation, idempotent duplicates, concurrency and restart preservation at a named database transaction boundary.
- **E04-S01-T06 — Worker/pilot proof:** capture the relevant operator or manager role action, device/source evidence, pending/blocked UI, measured accepted output and reviewer signature. Mark IMPLEMENTED-UNVERIFIED until proved.


#### E04-S02 — Compute baseline feasible or impossible status

**Story:** As a relevant factory user, I need to validate hard constraints before showing dates or profit, so that work is explainable, safe and actionable.
**Acceptance:** No-plan result includes blocking explanation.
**Dependencies:** Prior schema/source contracts in S03 / S08 / S09 / AGIE; existing installed legacy and device capabilities must be discovered.
**Delivery tasks:**

- **E04-S02-T01 Domain/API:** Define source-of-truth, typed inputs/outputs, idempotency, permissions, state transitions and failure contracts.
- **E04-S02-T02 Implementation/UI:** Implement owning logic and integration adapter, expose a role-appropriate action and clear conflict/pending/approved states.
- **E04-S02-T03 Evidence/pilot:** Run positive, negative, replay and business-invariant checks using a named pilot fixture; capture acceptance evidence and rollback procedure.

**Merged production-verification elaboration (required):** Expired inventory/skill snapshot must produce blocking uncertainty, not plausible invented feasibility.

- **E04-S02-T04 — Stateful integration:** identify the authoritative aggregate and durable migration, exact API handler/source path and outbox mapping; record nonauthoritative legacy observations separately.
- **E04-S02-T05 — Negative and replay fixture:** reproduce the stated original acceptance condition and its failure counterpart; prove tenant isolation, idempotent duplicates, concurrency and restart preservation at a named database transaction boundary.
- **E04-S02-T06 — Worker/pilot proof:** capture the relevant operator or manager role action, device/source evidence, pending/blocked UI, measured accepted output and reviewer signature. Mark IMPLEMENTED-UNVERIFIED until proved.


#### E04-S03 — Generate ranked clarification questions

**Story:** As a relevant factory user, I need to ask only unknowns that change feasible candidate plan, so that work is explainable, safe and actionable.
**Acceptance:** Question includes tradeoffs options and role.
**Dependencies:** Prior schema/source contracts in S03 / S08 / S09 / AGIE; existing installed legacy and device capabilities must be discovered.
**Delivery tasks:**

- **E04-S03-T01 Domain/API:** Define source-of-truth, typed inputs/outputs, idempotency, permissions, state transitions and failure contracts.
- **E04-S03-T02 Implementation/UI:** Implement owning logic and integration adapter, expose a role-appropriate action and clear conflict/pending/approved states.
- **E04-S03-T03 Evidence/pilot:** Run positive, negative, replay and business-invariant checks using a named pilot fixture; capture acceptance evidence and rollback procedure.

**Merged production-verification elaboration (required):** Expired inventory/skill snapshot must produce blocking uncertainty, not plausible invented feasibility.

- **E04-S03-T04 — Stateful integration:** identify the authoritative aggregate and durable migration, exact API handler/source path and outbox mapping; record nonauthoritative legacy observations separately.
- **E04-S03-T05 — Negative and replay fixture:** reproduce the stated original acceptance condition and its failure counterpart; prove tenant isolation, idempotent duplicates, concurrency and restart preservation at a named database transaction boundary.
- **E04-S03-T06 — Worker/pilot proof:** capture the relevant operator or manager role action, device/source evidence, pending/blocked UI, measured accepted output and reviewer signature. Mark IMPLEMENTED-UNVERIFIED until proved.


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

**Merged production-verification elaboration (required):** Expired inventory/skill snapshot must produce blocking uncertainty, not plausible invented feasibility.

- **E04-S04-T04 — Stateful integration:** identify the authoritative aggregate and durable migration, exact API handler/source path and outbox mapping; record nonauthoritative legacy observations separately.
- **E04-S04-T05 — Negative and replay fixture:** reproduce the stated original acceptance condition and its failure counterpart; prove tenant isolation, idempotent duplicates, concurrency and restart preservation at a named database transaction boundary.
- **E04-S04-T06 — Worker/pilot proof:** capture the relevant operator or manager role action, device/source evidence, pending/blocked UI, measured accepted output and reviewer signature. Mark IMPLEMENTED-UNVERIFIED until proved.


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

**Merged production-verification elaboration (required):** Attempt concurrent reservation/return and quality hold on the same roll segment; prove non-negative available, source ancestry and unit balance.

- **E05-S01-T04 — Stateful integration:** identify the authoritative aggregate and durable migration, exact API handler/source path and outbox mapping; record nonauthoritative legacy observations separately.
- **E05-S01-T05 — Negative and replay fixture:** reproduce the stated original acceptance condition and its failure counterpart; prove tenant isolation, idempotent duplicates, concurrency and restart preservation at a named database transaction boundary.
- **E05-S01-T06 — Worker/pilot proof:** capture the relevant operator or manager role action, device/source evidence, pending/blocked UI, measured accepted output and reviewer signature. Mark IMPLEMENTED-UNVERIFIED until proved.


#### E05-S02 — Capture GRN inspection and calibration

**Story:** As a relevant factory user, I need to onboard UHF rolls and width length shade defects with calibrated source, so that work is explainable, safe and actionable.
**Acceptance:** Quarantine bad measurements.
**Dependencies:** Prior schema/source contracts in S11 / S12; existing installed legacy and device capabilities must be discovered.
**Delivery tasks:**

- **E05-S02-T01 Domain/API:** Define source-of-truth, typed inputs/outputs, idempotency, permissions, state transitions and failure contracts.
- **E05-S02-T02 Implementation/UI:** Implement owning logic and integration adapter, expose a role-appropriate action and clear conflict/pending/approved states.
- **E05-S02-T03 Evidence/pilot:** Run positive, negative, replay and business-invariant checks using a named pilot fixture; capture acceptance evidence and rollback procedure.

**Merged production-verification elaboration (required):** Attempt concurrent reservation/return and quality hold on the same roll segment; prove non-negative available, source ancestry and unit balance.

- **E05-S02-T04 — Stateful integration:** identify the authoritative aggregate and durable migration, exact API handler/source path and outbox mapping; record nonauthoritative legacy observations separately.
- **E05-S02-T05 — Negative and replay fixture:** reproduce the stated original acceptance condition and its failure counterpart; prove tenant isolation, idempotent duplicates, concurrency and restart preservation at a named database transaction boundary.
- **E05-S02-T06 — Worker/pilot proof:** capture the relevant operator or manager role action, device/source evidence, pending/blocked UI, measured accepted output and reviewer signature. Mark IMPLEMENTED-UNVERIFIED until proved.


#### E05-S03 — Implement slice reservation and split returns

**Story:** As a relevant factory user, I need to reserve partial lengths across compatible orders with atomic locking, so that work is explainable, safe and actionable.
**Acceptance:** Cannot spend one centimeter twice.
**Dependencies:** Prior schema/source contracts in S11 / S12; existing installed legacy and device capabilities must be discovered.
**Delivery tasks:**

- **E05-S03-T01 Domain/API:** Define source-of-truth, typed inputs/outputs, idempotency, permissions, state transitions and failure contracts.
- **E05-S03-T02 Implementation/UI:** Implement owning logic and integration adapter, expose a role-appropriate action and clear conflict/pending/approved states.
- **E05-S03-T03 Evidence/pilot:** Run positive, negative, replay and business-invariant checks using a named pilot fixture; capture acceptance evidence and rollback procedure.

**Merged production-verification elaboration (required):** Attempt concurrent reservation/return and quality hold on the same roll segment; prove non-negative available, source ancestry and unit balance.

- **E05-S03-T04 — Stateful integration:** identify the authoritative aggregate and durable migration, exact API handler/source path and outbox mapping; record nonauthoritative legacy observations separately.
- **E05-S03-T05 — Negative and replay fixture:** reproduce the stated original acceptance condition and its failure counterpart; prove tenant isolation, idempotent duplicates, concurrency and restart preservation at a named database transaction boundary.
- **E05-S03-T06 — Worker/pilot proof:** capture the relevant operator or manager role action, device/source evidence, pending/blocked UI, measured accepted output and reviewer signature. Mark IMPLEMENTED-UNVERIFIED until proved.


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

**Merged production-verification elaboration (required):** Attempt concurrent reservation/return and quality hold on the same roll segment; prove non-negative available, source ancestry and unit balance.

- **E05-S04-T04 — Stateful integration:** identify the authoritative aggregate and durable migration, exact API handler/source path and outbox mapping; record nonauthoritative legacy observations separately.
- **E05-S04-T05 — Negative and replay fixture:** reproduce the stated original acceptance condition and its failure counterpart; prove tenant isolation, idempotent duplicates, concurrency and restart preservation at a named database transaction boundary.
- **E05-S04-T06 — Worker/pilot proof:** capture the relevant operator or manager role action, device/source evidence, pending/blocked UI, measured accepted output and reviewer signature. Mark IMPLEMENTED-UNVERIFIED until proved.


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

**Merged production-verification elaboration (required):** Marker different geometry but identical width must fail interchange; completed lays immutable when scheduling remainder.

- **E06-S01-T04 — Stateful integration:** identify the authoritative aggregate and durable migration, exact API handler/source path and outbox mapping; record nonauthoritative legacy observations separately.
- **E06-S01-T05 — Negative and replay fixture:** reproduce the stated original acceptance condition and its failure counterpart; prove tenant isolation, idempotent duplicates, concurrency and restart preservation at a named database transaction boundary.
- **E06-S01-T06 — Worker/pilot proof:** capture the relevant operator or manager role action, device/source evidence, pending/blocked UI, measured accepted output and reviewer signature. Mark IMPLEMENTED-UNVERIFIED until proved.


#### E06-S02 — Build heterogeneous roll allocation optimizer

**Story:** As a relevant factory user, I need to choose segments widths plies marker families and PO ratios under hard constraints, so that work is explainable, safe and actionable.
**Acceptance:** At least three justified plans and infeasible reasons.
**Dependencies:** Prior schema/source contracts in S08 / S13; existing installed legacy and device capabilities must be discovered.
**Delivery tasks:**

- **E06-S02-T01 Domain/API:** Define source-of-truth, typed inputs/outputs, idempotency, permissions, state transitions and failure contracts.
- **E06-S02-T02 Implementation/UI:** Implement owning logic and integration adapter, expose a role-appropriate action and clear conflict/pending/approved states.
- **E06-S02-T03 Evidence/pilot:** Run positive, negative, replay and business-invariant checks using a named pilot fixture; capture acceptance evidence and rollback procedure.

**Merged production-verification elaboration (required):** Marker different geometry but identical width must fail interchange; completed lays immutable when scheduling remainder.

- **E06-S02-T04 — Stateful integration:** identify the authoritative aggregate and durable migration, exact API handler/source path and outbox mapping; record nonauthoritative legacy observations separately.
- **E06-S02-T05 — Negative and replay fixture:** reproduce the stated original acceptance condition and its failure counterpart; prove tenant isolation, idempotent duplicates, concurrency and restart preservation at a named database transaction boundary.
- **E06-S02-T06 — Worker/pilot proof:** capture the relevant operator or manager role action, device/source evidence, pending/blocked UI, measured accepted output and reviewer signature. Mark IMPLEMENTED-UNVERIFIED until proved.


#### E06-S03 — Schedule lays and cut-table availability

**Story:** As a relevant factory user, I need to respect knife stack height maintenance people shift and changeovers, so that work is explainable, safe and actionable.
**Acceptance:** Gantt and list show critical constraints.
**Dependencies:** Prior schema/source contracts in S08 / S13; existing installed legacy and device capabilities must be discovered.
**Delivery tasks:**

- **E06-S03-T01 Domain/API:** Define source-of-truth, typed inputs/outputs, idempotency, permissions, state transitions and failure contracts.
- **E06-S03-T02 Implementation/UI:** Implement owning logic and integration adapter, expose a role-appropriate action and clear conflict/pending/approved states.
- **E06-S03-T03 Evidence/pilot:** Run positive, negative, replay and business-invariant checks using a named pilot fixture; capture acceptance evidence and rollback procedure.

**Merged production-verification elaboration (required):** Marker different geometry but identical width must fail interchange; completed lays immutable when scheduling remainder.

- **E06-S03-T04 — Stateful integration:** identify the authoritative aggregate and durable migration, exact API handler/source path and outbox mapping; record nonauthoritative legacy observations separately.
- **E06-S03-T05 — Negative and replay fixture:** reproduce the stated original acceptance condition and its failure counterpart; prove tenant isolation, idempotent duplicates, concurrency and restart preservation at a named database transaction boundary.
- **E06-S03-T06 — Worker/pilot proof:** capture the relevant operator or manager role action, device/source evidence, pending/blocked UI, measured accepted output and reviewer signature. Mark IMPLEMENTED-UNVERIFIED until proved.


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

**Merged production-verification elaboration (required):** Marker different geometry but identical width must fail interchange; completed lays immutable when scheduling remainder.

- **E06-S04-T04 — Stateful integration:** identify the authoritative aggregate and durable migration, exact API handler/source path and outbox mapping; record nonauthoritative legacy observations separately.
- **E06-S04-T05 — Negative and replay fixture:** reproduce the stated original acceptance condition and its failure counterpart; prove tenant isolation, idempotent duplicates, concurrency and restart preservation at a named database transaction boundary.
- **E06-S04-T06 — Worker/pilot proof:** capture the relevant operator or manager role action, device/source evidence, pending/blocked UI, measured accepted output and reviewer signature. Mark IMPLEMENTED-UNVERIFIED until proved.


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

**Merged production-verification elaboration (required):** Duplicate EPC, stale sighting and unreadable tag cannot bind twice or fabricate exact location; reuse preserves prior association.

- **E07-S01-T04 — Stateful integration:** identify the authoritative aggregate and durable migration, exact API handler/source path and outbox mapping; record nonauthoritative legacy observations separately.
- **E07-S01-T05 — Negative and replay fixture:** reproduce the stated original acceptance condition and its failure counterpart; prove tenant isolation, idempotent duplicates, concurrency and restart preservation at a named database transaction boundary.
- **E07-S01-T06 — Worker/pilot proof:** capture the relevant operator or manager role action, device/source evidence, pending/blocked UI, measured accepted output and reviewer signature. Mark IMPLEMENTED-UNVERIFIED until proved.


#### E07-S02 — Implement tag provision and verify

**Story:** As a relevant factory user, I need to provision inventory verify and quarantine damaged duplicate unknown UIDs, so that work is explainable, safe and actionable.
**Acceptance:** No active double binding.
**Dependencies:** Prior schema/source contracts in S02 / S11 / RFID; existing installed legacy and device capabilities must be discovered.
**Delivery tasks:**

- **E07-S02-T01 Domain/API:** Define source-of-truth, typed inputs/outputs, idempotency, permissions, state transitions and failure contracts.
- **E07-S02-T02 Implementation/UI:** Implement owning logic and integration adapter, expose a role-appropriate action and clear conflict/pending/approved states.
- **E07-S02-T03 Evidence/pilot:** Run positive, negative, replay and business-invariant checks using a named pilot fixture; capture acceptance evidence and rollback procedure.

**Merged production-verification elaboration (required):** Duplicate EPC, stale sighting and unreadable tag cannot bind twice or fabricate exact location; reuse preserves prior association.

- **E07-S02-T04 — Stateful integration:** identify the authoritative aggregate and durable migration, exact API handler/source path and outbox mapping; record nonauthoritative legacy observations separately.
- **E07-S02-T05 — Negative and replay fixture:** reproduce the stated original acceptance condition and its failure counterpart; prove tenant isolation, idempotent duplicates, concurrency and restart preservation at a named database transaction boundary.
- **E07-S02-T06 — Worker/pilot proof:** capture the relevant operator or manager role action, device/source evidence, pending/blocked UI, measured accepted output and reviewer signature. Mark IMPLEMENTED-UNVERIFIED until proved.


#### E07-S03 — Implement binding retirement and reuse

**Story:** As a relevant factory user, I need to unbind only after committed genealogy; inspect erase verify pool status, so that work is explainable, safe and actionable.
**Acceptance:** Reused UID never changes prior object history.
**Dependencies:** Prior schema/source contracts in S02 / S11 / RFID; existing installed legacy and device capabilities must be discovered.
**Delivery tasks:**

- **E07-S03-T01 Domain/API:** Define source-of-truth, typed inputs/outputs, idempotency, permissions, state transitions and failure contracts.
- **E07-S03-T02 Implementation/UI:** Implement owning logic and integration adapter, expose a role-appropriate action and clear conflict/pending/approved states.
- **E07-S03-T03 Evidence/pilot:** Run positive, negative, replay and business-invariant checks using a named pilot fixture; capture acceptance evidence and rollback procedure.

**Merged production-verification elaboration (required):** Duplicate EPC, stale sighting and unreadable tag cannot bind twice or fabricate exact location; reuse preserves prior association.

- **E07-S03-T04 — Stateful integration:** identify the authoritative aggregate and durable migration, exact API handler/source path and outbox mapping; record nonauthoritative legacy observations separately.
- **E07-S03-T05 — Negative and replay fixture:** reproduce the stated original acceptance condition and its failure counterpart; prove tenant isolation, idempotent duplicates, concurrency and restart preservation at a named database transaction boundary.
- **E07-S03-T06 — Worker/pilot proof:** capture the relevant operator or manager role action, device/source evidence, pending/blocked UI, measured accepted output and reviewer signature. Mark IMPLEMENTED-UNVERIFIED until proved.


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

**Merged production-verification elaboration (required):** Duplicate EPC, stale sighting and unreadable tag cannot bind twice or fabricate exact location; reuse preserves prior association.

- **E07-S04-T04 — Stateful integration:** identify the authoritative aggregate and durable migration, exact API handler/source path and outbox mapping; record nonauthoritative legacy observations separately.
- **E07-S04-T05 — Negative and replay fixture:** reproduce the stated original acceptance condition and its failure counterpart; prove tenant isolation, idempotent duplicates, concurrency and restart preservation at a named database transaction boundary.
- **E07-S04-T06 — Worker/pilot proof:** capture the relevant operator or manager role action, device/source evidence, pending/blocked UI, measured accepted output and reviewer signature. Mark IMPLEMENTED-UNVERIFIED until proved.


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

**Merged production-verification elaboration (required):** Missing part, duplicate consumption and cross-shade join fail; variable component counts conserve per-part quantities.

- **E08-S01-T04 — Stateful integration:** identify the authoritative aggregate and durable migration, exact API handler/source path and outbox mapping; record nonauthoritative legacy observations separately.
- **E08-S01-T05 — Negative and replay fixture:** reproduce the stated original acceptance condition and its failure counterpart; prove tenant isolation, idempotent duplicates, concurrency and restart preservation at a named database transaction boundary.
- **E08-S01-T06 — Worker/pilot proof:** capture the relevant operator or manager role action, device/source evidence, pending/blocked UI, measured accepted output and reviewer signature. Mark IMPLEMENTED-UNVERIFIED until proved.


#### E08-S02 — Introduce NFC part-level pilot

**Story:** As a relevant factory user, I need to tap part or tagged fixture with role and operation context, so that work is explainable, safe and actionable.
**Acceptance:** Pilot reports per-piece burden tag failure and false reads.
**Dependencies:** Prior schema/source contracts in S13 / S14 / attachment; existing installed legacy and device capabilities must be discovered.
**Delivery tasks:**

- **E08-S02-T01 Domain/API:** Define source-of-truth, typed inputs/outputs, idempotency, permissions, state transitions and failure contracts.
- **E08-S02-T02 Implementation/UI:** Implement owning logic and integration adapter, expose a role-appropriate action and clear conflict/pending/approved states.
- **E08-S02-T03 Evidence/pilot:** Run positive, negative, replay and business-invariant checks using a named pilot fixture; capture acceptance evidence and rollback procedure.

**Merged production-verification elaboration (required):** Missing part, duplicate consumption and cross-shade join fail; variable component counts conserve per-part quantities.

- **E08-S02-T04 — Stateful integration:** identify the authoritative aggregate and durable migration, exact API handler/source path and outbox mapping; record nonauthoritative legacy observations separately.
- **E08-S02-T05 — Negative and replay fixture:** reproduce the stated original acceptance condition and its failure counterpart; prove tenant isolation, idempotent duplicates, concurrency and restart preservation at a named database transaction boundary.
- **E08-S02-T06 — Worker/pilot proof:** capture the relevant operator or manager role action, device/source evidence, pending/blocked UI, measured accepted output and reviewer signature. Mark IMPLEMENTED-UNVERIFIED until proved.


#### E08-S03 — Build atomic split merge transformation ledger

**Story:** As a relevant factory user, I need to lock parent inputs and commit typed outputs scrap holds and event, so that work is explainable, safe and actionable.
**Acceptance:** Exact count per part type on split/merge.
**Dependencies:** Prior schema/source contracts in S13 / S14 / attachment; existing installed legacy and device capabilities must be discovered.
**Delivery tasks:**

- **E08-S03-T01 Domain/API:** Define source-of-truth, typed inputs/outputs, idempotency, permissions, state transitions and failure contracts.
- **E08-S03-T02 Implementation/UI:** Implement owning logic and integration adapter, expose a role-appropriate action and clear conflict/pending/approved states.
- **E08-S03-T03 Evidence/pilot:** Run positive, negative, replay and business-invariant checks using a named pilot fixture; capture acceptance evidence and rollback procedure.

**Merged production-verification elaboration (required):** Missing part, duplicate consumption and cross-shade join fail; variable component counts conserve per-part quantities.

- **E08-S03-T04 — Stateful integration:** identify the authoritative aggregate and durable migration, exact API handler/source path and outbox mapping; record nonauthoritative legacy observations separately.
- **E08-S03-T05 — Negative and replay fixture:** reproduce the stated original acceptance condition and its failure counterpart; prove tenant isolation, idempotent duplicates, concurrency and restart preservation at a named database transaction boundary.
- **E08-S03-T06 — Worker/pilot proof:** capture the relevant operator or manager role action, device/source evidence, pending/blocked UI, measured accepted output and reviewer signature. Mark IMPLEMENTED-UNVERIFIED until proved.


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

**Merged production-verification elaboration (required):** Missing part, duplicate consumption and cross-shade join fail; variable component counts conserve per-part quantities.

- **E08-S04-T04 — Stateful integration:** identify the authoritative aggregate and durable migration, exact API handler/source path and outbox mapping; record nonauthoritative legacy observations separately.
- **E08-S04-T05 — Negative and replay fixture:** reproduce the stated original acceptance condition and its failure counterpart; prove tenant isolation, idempotent duplicates, concurrency and restart preservation at a named database transaction boundary.
- **E08-S04-T06 — Worker/pilot proof:** capture the relevant operator or manager role action, device/source evidence, pending/blocked UI, measured accepted output and reviewer signature. Mark IMPLEMENTED-UNVERIFIED until proved.


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

**Merged production-verification elaboration (required):** Offline station duplicate/reordered requests must credit once; passive UHF read yields zero earned production credit.

- **E09-S01-T04 — Stateful integration:** identify the authoritative aggregate and durable migration, exact API handler/source path and outbox mapping; record nonauthoritative legacy observations separately.
- **E09-S01-T05 — Negative and replay fixture:** reproduce the stated original acceptance condition and its failure counterpart; prove tenant isolation, idempotent duplicates, concurrency and restart preservation at a named database transaction boundary.
- **E09-S01-T06 — Worker/pilot proof:** capture the relevant operator or manager role action, device/source evidence, pending/blocked UI, measured accepted output and reviewer signature. Mark IMPLEMENTED-UNVERIFIED until proved.


#### E09-S02 — Capture pause break changeover and machine signals

**Story:** As a relevant factory user, I need to categorize measured vs inferred time by worker machine and carrier, so that work is explainable, safe and actionable.
**Acceptance:** Overnight hours not counted as cycle time.
**Dependencies:** Prior schema/source contracts in S14 / Edge; existing installed legacy and device capabilities must be discovered.
**Delivery tasks:**

- **E09-S02-T01 Domain/API:** Define source-of-truth, typed inputs/outputs, idempotency, permissions, state transitions and failure contracts.
- **E09-S02-T02 Implementation/UI:** Implement owning logic and integration adapter, expose a role-appropriate action and clear conflict/pending/approved states.
- **E09-S02-T03 Evidence/pilot:** Run positive, negative, replay and business-invariant checks using a named pilot fixture; capture acceptance evidence and rollback procedure.

**Merged production-verification elaboration (required):** Offline station duplicate/reordered requests must credit once; passive UHF read yields zero earned production credit.

- **E09-S02-T04 — Stateful integration:** identify the authoritative aggregate and durable migration, exact API handler/source path and outbox mapping; record nonauthoritative legacy observations separately.
- **E09-S02-T05 — Negative and replay fixture:** reproduce the stated original acceptance condition and its failure counterpart; prove tenant isolation, idempotent duplicates, concurrency and restart preservation at a named database transaction boundary.
- **E09-S02-T06 — Worker/pilot proof:** capture the relevant operator or manager role action, device/source evidence, pending/blocked UI, measured accepted output and reviewer signature. Mark IMPLEMENTED-UNVERIFIED until proved.


#### E09-S03 — Implement station anomaly and wait flows

**Story:** As a relevant factory user, I need to wrong shade size missing operation duplicate claim and damaged tag, so that work is explainable, safe and actionable.
**Acceptance:** Simple fail closed operator prompt.
**Dependencies:** Prior schema/source contracts in S14 / Edge; existing installed legacy and device capabilities must be discovered.
**Delivery tasks:**

- **E09-S03-T01 Domain/API:** Define source-of-truth, typed inputs/outputs, idempotency, permissions, state transitions and failure contracts.
- **E09-S03-T02 Implementation/UI:** Implement owning logic and integration adapter, expose a role-appropriate action and clear conflict/pending/approved states.
- **E09-S03-T03 Evidence/pilot:** Run positive, negative, replay and business-invariant checks using a named pilot fixture; capture acceptance evidence and rollback procedure.

**Merged production-verification elaboration (required):** Offline station duplicate/reordered requests must credit once; passive UHF read yields zero earned production credit.

- **E09-S03-T04 — Stateful integration:** identify the authoritative aggregate and durable migration, exact API handler/source path and outbox mapping; record nonauthoritative legacy observations separately.
- **E09-S03-T05 — Negative and replay fixture:** reproduce the stated original acceptance condition and its failure counterpart; prove tenant isolation, idempotent duplicates, concurrency and restart preservation at a named database transaction boundary.
- **E09-S03-T06 — Worker/pilot proof:** capture the relevant operator or manager role action, device/source evidence, pending/blocked UI, measured accepted output and reviewer signature. Mark IMPLEMENTED-UNVERIFIED until proved.


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

**Merged production-verification elaboration (required):** Offline station duplicate/reordered requests must credit once; passive UHF read yields zero earned production credit.

- **E09-S04-T04 — Stateful integration:** identify the authoritative aggregate and durable migration, exact API handler/source path and outbox mapping; record nonauthoritative legacy observations separately.
- **E09-S04-T05 — Negative and replay fixture:** reproduce the stated original acceptance condition and its failure counterpart; prove tenant isolation, idempotent duplicates, concurrency and restart preservation at a named database transaction boundary.
- **E09-S04-T06 — Worker/pilot proof:** capture the relevant operator or manager role action, device/source evidence, pending/blocked UI, measured accepted output and reviewer signature. Mark IMPLEMENTED-UNVERIFIED until proved.


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

**Merged production-verification elaboration (required):** Failed inspection holds only affected units; repaired unit must be independently accepted once.

- **E10-S01-T04 — Stateful integration:** identify the authoritative aggregate and durable migration, exact API handler/source path and outbox mapping; record nonauthoritative legacy observations separately.
- **E10-S01-T05 — Negative and replay fixture:** reproduce the stated original acceptance condition and its failure counterpart; prove tenant isolation, idempotent duplicates, concurrency and restart preservation at a named database transaction boundary.
- **E10-S01-T06 — Worker/pilot proof:** capture the relevant operator or manager role action, device/source evidence, pending/blocked UI, measured accepted output and reviewer signature. Mark IMPLEMENTED-UNVERIFIED until proved.


#### E10-S02 — Implement repair loop with independent check

**Story:** As a relevant factory user, I need to return altered unit through required final inspection, so that work is explainable, safe and actionable.
**Acceptance:** No double counting good units.
**Dependencies:** Prior schema/source contracts in S17 / S18; existing installed legacy and device capabilities must be discovered.
**Delivery tasks:**

- **E10-S02-T01 Domain/API:** Define source-of-truth, typed inputs/outputs, idempotency, permissions, state transitions and failure contracts.
- **E10-S02-T02 Implementation/UI:** Implement owning logic and integration adapter, expose a role-appropriate action and clear conflict/pending/approved states.
- **E10-S02-T03 Evidence/pilot:** Run positive, negative, replay and business-invariant checks using a named pilot fixture; capture acceptance evidence and rollback procedure.

**Merged production-verification elaboration (required):** Failed inspection holds only affected units; repaired unit must be independently accepted once.

- **E10-S02-T04 — Stateful integration:** identify the authoritative aggregate and durable migration, exact API handler/source path and outbox mapping; record nonauthoritative legacy observations separately.
- **E10-S02-T05 — Negative and replay fixture:** reproduce the stated original acceptance condition and its failure counterpart; prove tenant isolation, idempotent duplicates, concurrency and restart preservation at a named database transaction boundary.
- **E10-S02-T06 — Worker/pilot proof:** capture the relevant operator or manager role action, device/source evidence, pending/blocked UI, measured accepted output and reviewer signature. Mark IMPLEMENTED-UNVERIFIED until proved.


#### E10-S03 — Support finishing route variants

**Story:** As a relevant factory user, I need to wash press hanger trim skip per approved style-specific DAG, so that work is explainable, safe and actionable.
**Acceptance:** Wrong product-family steps rejected.
**Dependencies:** Prior schema/source contracts in S17 / S18; existing installed legacy and device capabilities must be discovered.
**Delivery tasks:**

- **E10-S03-T01 Domain/API:** Define source-of-truth, typed inputs/outputs, idempotency, permissions, state transitions and failure contracts.
- **E10-S03-T02 Implementation/UI:** Implement owning logic and integration adapter, expose a role-appropriate action and clear conflict/pending/approved states.
- **E10-S03-T03 Evidence/pilot:** Run positive, negative, replay and business-invariant checks using a named pilot fixture; capture acceptance evidence and rollback procedure.

**Merged production-verification elaboration (required):** Failed inspection holds only affected units; repaired unit must be independently accepted once.

- **E10-S03-T04 — Stateful integration:** identify the authoritative aggregate and durable migration, exact API handler/source path and outbox mapping; record nonauthoritative legacy observations separately.
- **E10-S03-T05 — Negative and replay fixture:** reproduce the stated original acceptance condition and its failure counterpart; prove tenant isolation, idempotent duplicates, concurrency and restart preservation at a named database transaction boundary.
- **E10-S03-T06 — Worker/pilot proof:** capture the relevant operator or manager role action, device/source evidence, pending/blocked UI, measured accepted output and reviewer signature. Mark IMPLEMENTED-UNVERIFIED until proved.


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

**Merged production-verification elaboration (required):** Failed inspection holds only affected units; repaired unit must be independently accepted once.

- **E10-S04-T04 — Stateful integration:** identify the authoritative aggregate and durable migration, exact API handler/source path and outbox mapping; record nonauthoritative legacy observations separately.
- **E10-S04-T05 — Negative and replay fixture:** reproduce the stated original acceptance condition and its failure counterpart; prove tenant isolation, idempotent duplicates, concurrency and restart preservation at a named database transaction boundary.
- **E10-S04-T06 — Worker/pilot proof:** capture the relevant operator or manager role action, device/source evidence, pending/blocked UI, measured accepted output and reviewer signature. Mark IMPLEMENTED-UNVERIFIED until proved.


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

**Merged production-verification elaboration (required):** Incorrect SKU/serial and unauthorized partial carton prevent sealing; duplicate carton load cannot increment count.

- **E11-S01-T04 — Stateful integration:** identify the authoritative aggregate and durable migration, exact API handler/source path and outbox mapping; record nonauthoritative legacy observations separately.
- **E11-S01-T05 — Negative and replay fixture:** reproduce the stated original acceptance condition and its failure counterpart; prove tenant isolation, idempotent duplicates, concurrency and restart preservation at a named database transaction boundary.
- **E11-S01-T06 — Worker/pilot proof:** capture the relevant operator or manager role action, device/source evidence, pending/blocked UI, measured accepted output and reviewer signature. Mark IMPLEMENTED-UNVERIFIED until proved.


#### E11-S02 — Build UHF bag and barcode carton evidence

**Story:** As a relevant factory user, I need to enforce uniqueness and exact garment membership, so that work is explainable, safe and actionable.
**Acceptance:** Duplicate scans do not increment count.
**Dependencies:** Prior schema/source contracts in S19 / S20; existing installed legacy and device capabilities must be discovered.
**Delivery tasks:**

- **E11-S02-T01 Domain/API:** Define source-of-truth, typed inputs/outputs, idempotency, permissions, state transitions and failure contracts.
- **E11-S02-T02 Implementation/UI:** Implement owning logic and integration adapter, expose a role-appropriate action and clear conflict/pending/approved states.
- **E11-S02-T03 Evidence/pilot:** Run positive, negative, replay and business-invariant checks using a named pilot fixture; capture acceptance evidence and rollback procedure.

**Merged production-verification elaboration (required):** Incorrect SKU/serial and unauthorized partial carton prevent sealing; duplicate carton load cannot increment count.

- **E11-S02-T04 — Stateful integration:** identify the authoritative aggregate and durable migration, exact API handler/source path and outbox mapping; record nonauthoritative legacy observations separately.
- **E11-S02-T05 — Negative and replay fixture:** reproduce the stated original acceptance condition and its failure counterpart; prove tenant isolation, idempotent duplicates, concurrency and restart preservation at a named database transaction boundary.
- **E11-S02-T06 — Worker/pilot proof:** capture the relevant operator or manager role action, device/source evidence, pending/blocked UI, measured accepted output and reviewer signature. Mark IMPLEMENTED-UNVERIFIED until proved.


#### E11-S03 — Create pack audit and carton seal gates

**Story:** As a relevant factory user, I need to qC hold and wrong SKU or size block sealing, so that work is explainable, safe and actionable.
**Acceptance:** Approved exception cites buyer authorization.
**Dependencies:** Prior schema/source contracts in S19 / S20; existing installed legacy and device capabilities must be discovered.
**Delivery tasks:**

- **E11-S03-T01 Domain/API:** Define source-of-truth, typed inputs/outputs, idempotency, permissions, state transitions and failure contracts.
- **E11-S03-T02 Implementation/UI:** Implement owning logic and integration adapter, expose a role-appropriate action and clear conflict/pending/approved states.
- **E11-S03-T03 Evidence/pilot:** Run positive, negative, replay and business-invariant checks using a named pilot fixture; capture acceptance evidence and rollback procedure.

**Merged production-verification elaboration (required):** Incorrect SKU/serial and unauthorized partial carton prevent sealing; duplicate carton load cannot increment count.

- **E11-S03-T04 — Stateful integration:** identify the authoritative aggregate and durable migration, exact API handler/source path and outbox mapping; record nonauthoritative legacy observations separately.
- **E11-S03-T05 — Negative and replay fixture:** reproduce the stated original acceptance condition and its failure counterpart; prove tenant isolation, idempotent duplicates, concurrency and restart preservation at a named database transaction boundary.
- **E11-S03-T06 — Worker/pilot proof:** capture the relevant operator or manager role action, device/source evidence, pending/blocked UI, measured accepted output and reviewer signature. Mark IMPLEMENTED-UNVERIFIED until proved.


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

**Merged production-verification elaboration (required):** Incorrect SKU/serial and unauthorized partial carton prevent sealing; duplicate carton load cannot increment count.

- **E11-S04-T04 — Stateful integration:** identify the authoritative aggregate and durable migration, exact API handler/source path and outbox mapping; record nonauthoritative legacy observations separately.
- **E11-S04-T05 — Negative and replay fixture:** reproduce the stated original acceptance condition and its failure counterpart; prove tenant isolation, idempotent duplicates, concurrency and restart preservation at a named database transaction boundary.
- **E11-S04-T06 — Worker/pilot proof:** capture the relevant operator or manager role action, device/source evidence, pending/blocked UI, measured accepted output and reviewer signature. Mark IMPLEMENTED-UNVERIFIED until proved.


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

**Merged production-verification elaboration (required):** Late warehouse correction cannot mutate signed OCR; signed amendments preserve the original snapshot.

- **E12-S01-T04 — Stateful integration:** identify the authoritative aggregate and durable migration, exact API handler/source path and outbox mapping; record nonauthoritative legacy observations separately.
- **E12-S01-T05 — Negative and replay fixture:** reproduce the stated original acceptance condition and its failure counterpart; prove tenant isolation, idempotent duplicates, concurrency and restart preservation at a named database transaction boundary.
- **E12-S01-T06 — Worker/pilot proof:** capture the relevant operator or manager role action, device/source evidence, pending/blocked UI, measured accepted output and reviewer signature. Mark IMPLEMENTED-UNVERIFIED until proved.


#### E12-S02 — Generate immutable order closure certificate

**Story:** As a relevant factory user, I need to freeze evidence revisions and role-based signoffs, so that work is explainable, safe and actionable.
**Acceptance:** Corrections are signed addenda.
**Dependencies:** Prior schema/source contracts in S21 / S22; existing installed legacy and device capabilities must be discovered.
**Delivery tasks:**

- **E12-S02-T01 Domain/API:** Define source-of-truth, typed inputs/outputs, idempotency, permissions, state transitions and failure contracts.
- **E12-S02-T02 Implementation/UI:** Implement owning logic and integration adapter, expose a role-appropriate action and clear conflict/pending/approved states.
- **E12-S02-T03 Evidence/pilot:** Run positive, negative, replay and business-invariant checks using a named pilot fixture; capture acceptance evidence and rollback procedure.

**Merged production-verification elaboration (required):** Late warehouse correction cannot mutate signed OCR; signed amendments preserve the original snapshot.

- **E12-S02-T04 — Stateful integration:** identify the authoritative aggregate and durable migration, exact API handler/source path and outbox mapping; record nonauthoritative legacy observations separately.
- **E12-S02-T05 — Negative and replay fixture:** reproduce the stated original acceptance condition and its failure counterpart; prove tenant isolation, idempotent duplicates, concurrency and restart preservation at a named database transaction boundary.
- **E12-S02-T06 — Worker/pilot proof:** capture the relevant operator or manager role action, device/source evidence, pending/blocked UI, measured accepted output and reviewer signature. Mark IMPLEMENTED-UNVERIFIED until proved.


#### E12-S03 — Compute material and conversion actual costs

**Story:** As a relevant factory user, I need to use price basis scrap labor machine freight and rates provenance, so that work is explainable, safe and actionable.
**Acceptance:** PO unit amount never assumed to be margin.
**Dependencies:** Prior schema/source contracts in S21 / S22; existing installed legacy and device capabilities must be discovered.
**Delivery tasks:**

- **E12-S03-T01 Domain/API:** Define source-of-truth, typed inputs/outputs, idempotency, permissions, state transitions and failure contracts.
- **E12-S03-T02 Implementation/UI:** Implement owning logic and integration adapter, expose a role-appropriate action and clear conflict/pending/approved states.
- **E12-S03-T03 Evidence/pilot:** Run positive, negative, replay and business-invariant checks using a named pilot fixture; capture acceptance evidence and rollback procedure.

**Merged production-verification elaboration (required):** Late warehouse correction cannot mutate signed OCR; signed amendments preserve the original snapshot.

- **E12-S03-T04 — Stateful integration:** identify the authoritative aggregate and durable migration, exact API handler/source path and outbox mapping; record nonauthoritative legacy observations separately.
- **E12-S03-T05 — Negative and replay fixture:** reproduce the stated original acceptance condition and its failure counterpart; prove tenant isolation, idempotent duplicates, concurrency and restart preservation at a named database transaction boundary.
- **E12-S03-T06 — Worker/pilot proof:** capture the relevant operator or manager role action, device/source evidence, pending/blocked UI, measured accepted output and reviewer signature. Mark IMPLEMENTED-UNVERIFIED until proved.


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

**Merged production-verification elaboration (required):** Late warehouse correction cannot mutate signed OCR; signed amendments preserve the original snapshot.

- **E12-S04-T04 — Stateful integration:** identify the authoritative aggregate and durable migration, exact API handler/source path and outbox mapping; record nonauthoritative legacy observations separately.
- **E12-S04-T05 — Negative and replay fixture:** reproduce the stated original acceptance condition and its failure counterpart; prove tenant isolation, idempotent duplicates, concurrency and restart preservation at a named database transaction boundary.
- **E12-S04-T06 — Worker/pilot proof:** capture the relevant operator or manager role action, device/source evidence, pending/blocked UI, measured accepted output and reviewer signature. Mark IMPLEMENTED-UNVERIFIED until proved.


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

**Merged production-verification elaboration (required):** Unsynchronized clocks, overnight gaps and missing RFID reads produce unknown duration instead of false cycle time.

- **E13-S01-T04 — Stateful integration:** identify the authoritative aggregate and durable migration, exact API handler/source path and outbox mapping; record nonauthoritative legacy observations separately.
- **E13-S01-T05 — Negative and replay fixture:** reproduce the stated original acceptance condition and its failure counterpart; prove tenant isolation, idempotent duplicates, concurrency and restart preservation at a named database transaction boundary.
- **E13-S01-T06 — Worker/pilot proof:** capture the relevant operator or manager role action, device/source evidence, pending/blocked UI, measured accepted output and reviewer signature. Mark IMPLEMENTED-UNVERIFIED until proved.


#### E13-S02 — Separate queue touch wait rework and handoffs

**Story:** As a relevant factory user, I need to segment active blocked idle transport repair by evidence, so that work is explainable, safe and actionable.
**Acceptance:** No overnight false cycle.
**Dependencies:** Prior schema/source contracts in S24 / S25; existing installed legacy and device capabilities must be discovered.
**Delivery tasks:**

- **E13-S02-T01 Domain/API:** Define source-of-truth, typed inputs/outputs, idempotency, permissions, state transitions and failure contracts.
- **E13-S02-T02 Implementation/UI:** Implement owning logic and integration adapter, expose a role-appropriate action and clear conflict/pending/approved states.
- **E13-S02-T03 Evidence/pilot:** Run positive, negative, replay and business-invariant checks using a named pilot fixture; capture acceptance evidence and rollback procedure.

**Merged production-verification elaboration (required):** Unsynchronized clocks, overnight gaps and missing RFID reads produce unknown duration instead of false cycle time.

- **E13-S02-T04 — Stateful integration:** identify the authoritative aggregate and durable migration, exact API handler/source path and outbox mapping; record nonauthoritative legacy observations separately.
- **E13-S02-T05 — Negative and replay fixture:** reproduce the stated original acceptance condition and its failure counterpart; prove tenant isolation, idempotent duplicates, concurrency and restart preservation at a named database transaction boundary.
- **E13-S02-T06 — Worker/pilot proof:** capture the relevant operator or manager role action, device/source evidence, pending/blocked UI, measured accepted output and reviewer signature. Mark IMPLEMENTED-UNVERIFIED until proved.


#### E13-S03 — Publish distributions and fairness guards

**Story:** As a relevant factory user, I need to p50/p90/p95 sample counts mix and quality-adjusted KPIs, so that work is explainable, safe and actionable.
**Acceptance:** Metrics specify denominator and sample.
**Dependencies:** Prior schema/source contracts in S24 / S25; existing installed legacy and device capabilities must be discovered.
**Delivery tasks:**

- **E13-S03-T01 Domain/API:** Define source-of-truth, typed inputs/outputs, idempotency, permissions, state transitions and failure contracts.
- **E13-S03-T02 Implementation/UI:** Implement owning logic and integration adapter, expose a role-appropriate action and clear conflict/pending/approved states.
- **E13-S03-T03 Evidence/pilot:** Run positive, negative, replay and business-invariant checks using a named pilot fixture; capture acceptance evidence and rollback procedure.

**Merged production-verification elaboration (required):** Unsynchronized clocks, overnight gaps and missing RFID reads produce unknown duration instead of false cycle time.

- **E13-S03-T04 — Stateful integration:** identify the authoritative aggregate and durable migration, exact API handler/source path and outbox mapping; record nonauthoritative legacy observations separately.
- **E13-S03-T05 — Negative and replay fixture:** reproduce the stated original acceptance condition and its failure counterpart; prove tenant isolation, idempotent duplicates, concurrency and restart preservation at a named database transaction boundary.
- **E13-S03-T06 — Worker/pilot proof:** capture the relevant operator or manager role action, device/source evidence, pending/blocked UI, measured accepted output and reviewer signature. Mark IMPLEMENTED-UNVERIFIED until proved.


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

**Merged production-verification elaboration (required):** Unsynchronized clocks, overnight gaps and missing RFID reads produce unknown duration instead of false cycle time.

- **E13-S04-T04 — Stateful integration:** identify the authoritative aggregate and durable migration, exact API handler/source path and outbox mapping; record nonauthoritative legacy observations separately.
- **E13-S04-T05 — Negative and replay fixture:** reproduce the stated original acceptance condition and its failure counterpart; prove tenant isolation, idempotent duplicates, concurrency and restart preservation at a named database transaction boundary.
- **E13-S04-T06 — Worker/pilot proof:** capture the relevant operator or manager role action, device/source evidence, pending/blocked UI, measured accepted output and reviewer signature. Mark IMPLEMENTED-UNVERIFIED until proved.


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

**Merged production-verification elaboration (required):** Absence triggers replan within finite machine availability; authorized shift/skill/maintenance constraints never silently weakened.

- **E14-S01-T04 — Stateful integration:** identify the authoritative aggregate and durable migration, exact API handler/source path and outbox mapping; record nonauthoritative legacy observations separately.
- **E14-S01-T05 — Negative and replay fixture:** reproduce the stated original acceptance condition and its failure counterpart; prove tenant isolation, idempotent duplicates, concurrency and restart preservation at a named database transaction boundary.
- **E14-S01-T06 — Worker/pilot proof:** capture the relevant operator or manager role action, device/source evidence, pending/blocked UI, measured accepted output and reviewer signature. Mark IMPLEMENTED-UNVERIFIED until proved.


#### E14-S02 — Compute critical path with finite machines and shifts

**Story:** As a relevant factory user, I need to schedule active orders using approved DAG and time calendars, so that work is explainable, safe and actionable.
**Acceptance:** Gantt shows dependencies and slack.
**Dependencies:** Prior schema/source contracts in S09 / S26; existing installed legacy and device capabilities must be discovered.
**Delivery tasks:**

- **E14-S02-T01 Domain/API:** Define source-of-truth, typed inputs/outputs, idempotency, permissions, state transitions and failure contracts.
- **E14-S02-T02 Implementation/UI:** Implement owning logic and integration adapter, expose a role-appropriate action and clear conflict/pending/approved states.
- **E14-S02-T03 Evidence/pilot:** Run positive, negative, replay and business-invariant checks using a named pilot fixture; capture acceptance evidence and rollback procedure.

**Merged production-verification elaboration (required):** Absence triggers replan within finite machine availability; authorized shift/skill/maintenance constraints never silently weakened.

- **E14-S02-T04 — Stateful integration:** identify the authoritative aggregate and durable migration, exact API handler/source path and outbox mapping; record nonauthoritative legacy observations separately.
- **E14-S02-T05 — Negative and replay fixture:** reproduce the stated original acceptance condition and its failure counterpart; prove tenant isolation, idempotent duplicates, concurrency and restart preservation at a named database transaction boundary.
- **E14-S02-T06 — Worker/pilot proof:** capture the relevant operator or manager role action, device/source evidence, pending/blocked UI, measured accepted output and reviewer signature. Mark IMPLEMENTED-UNVERIFIED until proved.


#### E14-S03 — Generate controlled alternatives

**Story:** As a relevant factory user, I need to compare shift changes certified worker substitutions and alternate fabric, so that work is explainable, safe and actionable.
**Acceptance:** Hard constraints never weakened silently.
**Dependencies:** Prior schema/source contracts in S09 / S26; existing installed legacy and device capabilities must be discovered.
**Delivery tasks:**

- **E14-S03-T01 Domain/API:** Define source-of-truth, typed inputs/outputs, idempotency, permissions, state transitions and failure contracts.
- **E14-S03-T02 Implementation/UI:** Implement owning logic and integration adapter, expose a role-appropriate action and clear conflict/pending/approved states.
- **E14-S03-T03 Evidence/pilot:** Run positive, negative, replay and business-invariant checks using a named pilot fixture; capture acceptance evidence and rollback procedure.

**Merged production-verification elaboration (required):** Absence triggers replan within finite machine availability; authorized shift/skill/maintenance constraints never silently weakened.

- **E14-S03-T04 — Stateful integration:** identify the authoritative aggregate and durable migration, exact API handler/source path and outbox mapping; record nonauthoritative legacy observations separately.
- **E14-S03-T05 — Negative and replay fixture:** reproduce the stated original acceptance condition and its failure counterpart; prove tenant isolation, idempotent duplicates, concurrency and restart preservation at a named database transaction boundary.
- **E14-S03-T06 — Worker/pilot proof:** capture the relevant operator or manager role action, device/source evidence, pending/blocked UI, measured accepted output and reviewer signature. Mark IMPLEMENTED-UNVERIFIED until proved.


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

**Merged production-verification elaboration (required):** Absence triggers replan within finite machine availability; authorized shift/skill/maintenance constraints never silently weakened.

- **E14-S04-T04 — Stateful integration:** identify the authoritative aggregate and durable migration, exact API handler/source path and outbox mapping; record nonauthoritative legacy observations separately.
- **E14-S04-T05 — Negative and replay fixture:** reproduce the stated original acceptance condition and its failure counterpart; prove tenant isolation, idempotent duplicates, concurrency and restart preservation at a named database transaction boundary.
- **E14-S04-T06 — Worker/pilot proof:** capture the relevant operator or manager role action, device/source evidence, pending/blocked UI, measured accepted output and reviewer signature. Mark IMPLEMENTED-UNVERIFIED until proved.


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

**Merged production-verification elaboration (required):** Pre-registered treatment/control cannot be changed after seeing data; negative effects and operator burden disclosed.

- **E15-S01-T04 — Stateful integration:** identify the authoritative aggregate and durable migration, exact API handler/source path and outbox mapping; record nonauthoritative legacy observations separately.
- **E15-S01-T05 — Negative and replay fixture:** reproduce the stated original acceptance condition and its failure counterpart; prove tenant isolation, idempotent duplicates, concurrency and restart preservation at a named database transaction boundary.
- **E15-S01-T06 — Worker/pilot proof:** capture the relevant operator or manager role action, device/source evidence, pending/blocked UI, measured accepted output and reviewer signature. Mark IMPLEMENTED-UNVERIFIED until proved.


#### E15-S02 — Assign matched A/B exposure

**Story:** As a relevant factory user, I need to randomize or match comparable cut lays/lines and calendar context, so that work is explainable, safe and actionable.
**Acceptance:** Baseline confounders disclosed.
**Dependencies:** Prior schema/source contracts in S26 / S27; existing installed legacy and device capabilities must be discovered.
**Delivery tasks:**

- **E15-S02-T01 Domain/API:** Define source-of-truth, typed inputs/outputs, idempotency, permissions, state transitions and failure contracts.
- **E15-S02-T02 Implementation/UI:** Implement owning logic and integration adapter, expose a role-appropriate action and clear conflict/pending/approved states.
- **E15-S02-T03 Evidence/pilot:** Run positive, negative, replay and business-invariant checks using a named pilot fixture; capture acceptance evidence and rollback procedure.

**Merged production-verification elaboration (required):** Pre-registered treatment/control cannot be changed after seeing data; negative effects and operator burden disclosed.

- **E15-S02-T04 — Stateful integration:** identify the authoritative aggregate and durable migration, exact API handler/source path and outbox mapping; record nonauthoritative legacy observations separately.
- **E15-S02-T05 — Negative and replay fixture:** reproduce the stated original acceptance condition and its failure counterpart; prove tenant isolation, idempotent duplicates, concurrency and restart preservation at a named database transaction boundary.
- **E15-S02-T06 — Worker/pilot proof:** capture the relevant operator or manager role action, device/source evidence, pending/blocked UI, measured accepted output and reviewer signature. Mark IMPLEMENTED-UNVERIFIED until proved.


#### E15-S03 — Calculate treatment effect and uncertainty

**Story:** As a relevant factory user, I need to compare fabric yield accepted units lead time quality and cost, so that work is explainable, safe and actionable.
**Acceptance:** Report negative and inconclusive results.
**Dependencies:** Prior schema/source contracts in S26 / S27; existing installed legacy and device capabilities must be discovered.
**Delivery tasks:**

- **E15-S03-T01 Domain/API:** Define source-of-truth, typed inputs/outputs, idempotency, permissions, state transitions and failure contracts.
- **E15-S03-T02 Implementation/UI:** Implement owning logic and integration adapter, expose a role-appropriate action and clear conflict/pending/approved states.
- **E15-S03-T03 Evidence/pilot:** Run positive, negative, replay and business-invariant checks using a named pilot fixture; capture acceptance evidence and rollback procedure.

**Merged production-verification elaboration (required):** Pre-registered treatment/control cannot be changed after seeing data; negative effects and operator burden disclosed.

- **E15-S03-T04 — Stateful integration:** identify the authoritative aggregate and durable migration, exact API handler/source path and outbox mapping; record nonauthoritative legacy observations separately.
- **E15-S03-T05 — Negative and replay fixture:** reproduce the stated original acceptance condition and its failure counterpart; prove tenant isolation, idempotent duplicates, concurrency and restart preservation at a named database transaction boundary.
- **E15-S03-T06 — Worker/pilot proof:** capture the relevant operator or manager role action, device/source evidence, pending/blocked UI, measured accepted output and reviewer signature. Mark IMPLEMENTED-UNVERIFIED until proved.


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

**Merged production-verification elaboration (required):** Pre-registered treatment/control cannot be changed after seeing data; negative effects and operator burden disclosed.

- **E15-S04-T04 — Stateful integration:** identify the authoritative aggregate and durable migration, exact API handler/source path and outbox mapping; record nonauthoritative legacy observations separately.
- **E15-S04-T05 — Negative and replay fixture:** reproduce the stated original acceptance condition and its failure counterpart; prove tenant isolation, idempotent duplicates, concurrency and restart preservation at a named database transaction boundary.
- **E15-S04-T06 — Worker/pilot proof:** capture the relevant operator or manager role action, device/source evidence, pending/blocked UI, measured accepted output and reviewer signature. Mark IMPLEMENTED-UNVERIFIED until proved.


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

**Merged production-verification elaboration (required):** Worker on low-end device must navigate primary action/error in chosen language, without keyboard for routine station event.

- **E16-S01-T04 — Stateful integration:** identify the authoritative aggregate and durable migration, exact API handler/source path and outbox mapping; record nonauthoritative legacy observations separately.
- **E16-S01-T05 — Negative and replay fixture:** reproduce the stated original acceptance condition and its failure counterpart; prove tenant isolation, idempotent duplicates, concurrency and restart preservation at a named database transaction boundary.
- **E16-S01-T06 — Worker/pilot proof:** capture the relevant operator or manager role action, device/source evidence, pending/blocked UI, measured accepted output and reviewer signature. Mark IMPLEMENTED-UNVERIFIED until proved.


#### E16-S02 — Build minimal tap-first station screens

**Story:** As a relevant factory user, I need to one action at a time icons large targets and clear pending status, so that work is explainable, safe and actionable.
**Acceptance:** No mandatory routine keyboard typing.
**Dependencies:** Prior schema/source contracts in BFF / Web; existing installed legacy and device capabilities must be discovered.
**Delivery tasks:**

- **E16-S02-T01 Domain/API:** Define source-of-truth, typed inputs/outputs, idempotency, permissions, state transitions and failure contracts.
- **E16-S02-T02 Implementation/UI:** Implement owning logic and integration adapter, expose a role-appropriate action and clear conflict/pending/approved states.
- **E16-S02-T03 Evidence/pilot:** Run positive, negative, replay and business-invariant checks using a named pilot fixture; capture acceptance evidence and rollback procedure.

**Merged production-verification elaboration (required):** Worker on low-end device must navigate primary action/error in chosen language, without keyboard for routine station event.

- **E16-S02-T04 — Stateful integration:** identify the authoritative aggregate and durable migration, exact API handler/source path and outbox mapping; record nonauthoritative legacy observations separately.
- **E16-S02-T05 — Negative and replay fixture:** reproduce the stated original acceptance condition and its failure counterpart; prove tenant isolation, idempotent duplicates, concurrency and restart preservation at a named database transaction boundary.
- **E16-S02-T06 — Worker/pilot proof:** capture the relevant operator or manager role action, device/source evidence, pending/blocked UI, measured accepted output and reviewer signature. Mark IMPLEMENTED-UNVERIFIED until proved.


#### E16-S03 — Connect role worklists and action ownership

**Story:** As a relevant factory user, I need to line lead inspector and manager see permitted work and exceptions, so that work is explainable, safe and actionable.
**Acceptance:** No orphan issue after escalation.
**Dependencies:** Prior schema/source contracts in BFF / Web; existing installed legacy and device capabilities must be discovered.
**Delivery tasks:**

- **E16-S03-T01 Domain/API:** Define source-of-truth, typed inputs/outputs, idempotency, permissions, state transitions and failure contracts.
- **E16-S03-T02 Implementation/UI:** Implement owning logic and integration adapter, expose a role-appropriate action and clear conflict/pending/approved states.
- **E16-S03-T03 Evidence/pilot:** Run positive, negative, replay and business-invariant checks using a named pilot fixture; capture acceptance evidence and rollback procedure.

**Merged production-verification elaboration (required):** Worker on low-end device must navigate primary action/error in chosen language, without keyboard for routine station event.

- **E16-S03-T04 — Stateful integration:** identify the authoritative aggregate and durable migration, exact API handler/source path and outbox mapping; record nonauthoritative legacy observations separately.
- **E16-S03-T05 — Negative and replay fixture:** reproduce the stated original acceptance condition and its failure counterpart; prove tenant isolation, idempotent duplicates, concurrency and restart preservation at a named database transaction boundary.
- **E16-S03-T06 — Worker/pilot proof:** capture the relevant operator or manager role action, device/source evidence, pending/blocked UI, measured accepted output and reviewer signature. Mark IMPLEMENTED-UNVERIFIED until proved.


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

**Merged production-verification elaboration (required):** Worker on low-end device must navigate primary action/error in chosen language, without keyboard for routine station event.

- **E16-S04-T04 — Stateful integration:** identify the authoritative aggregate and durable migration, exact API handler/source path and outbox mapping; record nonauthoritative legacy observations separately.
- **E16-S04-T05 — Negative and replay fixture:** reproduce the stated original acceptance condition and its failure counterpart; prove tenant isolation, idempotent duplicates, concurrency and restart preservation at a named database transaction boundary.
- **E16-S04-T06 — Worker/pilot proof:** capture the relevant operator or manager role action, device/source evidence, pending/blocked UI, measured accepted output and reviewer signature. Mark IMPLEMENTED-UNVERIFIED until proved.


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

**Merged production-verification elaboration (required):** Kill DB/broker/edge process at each commit boundary; tenant-safe one accepted physical transition after replay.

- **E17-S01-T04 — Stateful integration:** identify the authoritative aggregate and durable migration, exact API handler/source path and outbox mapping; record nonauthoritative legacy observations separately.
- **E17-S01-T05 — Negative and replay fixture:** reproduce the stated original acceptance condition and its failure counterpart; prove tenant isolation, idempotent duplicates, concurrency and restart preservation at a named database transaction boundary.
- **E17-S01-T06 — Worker/pilot proof:** capture the relevant operator or manager role action, device/source evidence, pending/blocked UI, measured accepted output and reviewer signature. Mark IMPLEMENTED-UNVERIFIED until proved.


#### E17-S02 — Implement durable outbox/inbox and replay

**Story:** As a relevant factory user, I need to atomic business event + outbox inbox consumer dedupe, so that work is explainable, safe and actionable.
**Acceptance:** No double accepted physical quantity.
**Dependencies:** Prior schema/source contracts in S01 / S02 / core; existing installed legacy and device capabilities must be discovered.
**Delivery tasks:**

- **E17-S02-T01 Domain/API:** Define source-of-truth, typed inputs/outputs, idempotency, permissions, state transitions and failure contracts.
- **E17-S02-T02 Implementation/UI:** Implement owning logic and integration adapter, expose a role-appropriate action and clear conflict/pending/approved states.
- **E17-S02-T03 Evidence/pilot:** Run positive, negative, replay and business-invariant checks using a named pilot fixture; capture acceptance evidence and rollback procedure.

**Merged production-verification elaboration (required):** Kill DB/broker/edge process at each commit boundary; tenant-safe one accepted physical transition after replay.

- **E17-S02-T04 — Stateful integration:** identify the authoritative aggregate and durable migration, exact API handler/source path and outbox mapping; record nonauthoritative legacy observations separately.
- **E17-S02-T05 — Negative and replay fixture:** reproduce the stated original acceptance condition and its failure counterpart; prove tenant isolation, idempotent duplicates, concurrency and restart preservation at a named database transaction boundary.
- **E17-S02-T06 — Worker/pilot proof:** capture the relevant operator or manager role action, device/source evidence, pending/blocked UI, measured accepted output and reviewer signature. Mark IMPLEMENTED-UNVERIFIED until proved.


#### E17-S03 — Qualify device clocks and local signed rules

**Story:** As a relevant factory user, I need to clock uncertainty key rotation and offline permission cache, so that work is explainable, safe and actionable.
**Acceptance:** Drift never yields false duration.
**Dependencies:** Prior schema/source contracts in S01 / S02 / core; existing installed legacy and device capabilities must be discovered.
**Delivery tasks:**

- **E17-S03-T01 Domain/API:** Define source-of-truth, typed inputs/outputs, idempotency, permissions, state transitions and failure contracts.
- **E17-S03-T02 Implementation/UI:** Implement owning logic and integration adapter, expose a role-appropriate action and clear conflict/pending/approved states.
- **E17-S03-T03 Evidence/pilot:** Run positive, negative, replay and business-invariant checks using a named pilot fixture; capture acceptance evidence and rollback procedure.

**Merged production-verification elaboration (required):** Kill DB/broker/edge process at each commit boundary; tenant-safe one accepted physical transition after replay.

- **E17-S03-T04 — Stateful integration:** identify the authoritative aggregate and durable migration, exact API handler/source path and outbox mapping; record nonauthoritative legacy observations separately.
- **E17-S03-T05 — Negative and replay fixture:** reproduce the stated original acceptance condition and its failure counterpart; prove tenant isolation, idempotent duplicates, concurrency and restart preservation at a named database transaction boundary.
- **E17-S03-T06 — Worker/pilot proof:** capture the relevant operator or manager role action, device/source evidence, pending/blocked UI, measured accepted output and reviewer signature. Mark IMPLEMENTED-UNVERIFIED until proved.


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

**Merged production-verification elaboration (required):** Kill DB/broker/edge process at each commit boundary; tenant-safe one accepted physical transition after replay.

- **E17-S04-T04 — Stateful integration:** identify the authoritative aggregate and durable migration, exact API handler/source path and outbox mapping; record nonauthoritative legacy observations separately.
- **E17-S04-T05 — Negative and replay fixture:** reproduce the stated original acceptance condition and its failure counterpart; prove tenant isolation, idempotent duplicates, concurrency and restart preservation at a named database transaction boundary.
- **E17-S04-T06 — Worker/pilot proof:** capture the relevant operator or manager role action, device/source evidence, pending/blocked UI, measured accepted output and reviewer signature. Mark IMPLEMENTED-UNVERIFIED until proved.


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

**Merged production-verification elaboration (required):** Deploy/uninstall shadow adapter with independent reconciliation; device/network crash drill captured before signoff.

- **E18-S01-T04 — Stateful integration:** identify the authoritative aggregate and durable migration, exact API handler/source path and outbox mapping; record nonauthoritative legacy observations separately.
- **E18-S01-T05 — Negative and replay fixture:** reproduce the stated original acceptance condition and its failure counterpart; prove tenant isolation, idempotent duplicates, concurrency and restart preservation at a named database transaction boundary.
- **E18-S01-T06 — Worker/pilot proof:** capture the relevant operator or manager role action, device/source evidence, pending/blocked UI, measured accepted output and reviewer signature. Mark IMPLEMENTED-UNVERIFIED until proved.


#### E18-S02 — Instrument service device connector and data health

**Story:** As a relevant factory user, I need to logs traces event age read rejection tag failure and queue lag, so that work is explainable, safe and actionable.
**Acceptance:** Each failed edge has owner and remediation.
**Dependencies:** Prior schema/source contracts in Platform / Ops; existing installed legacy and device capabilities must be discovered.
**Delivery tasks:**

- **E18-S02-T01 Domain/API:** Define source-of-truth, typed inputs/outputs, idempotency, permissions, state transitions and failure contracts.
- **E18-S02-T02 Implementation/UI:** Implement owning logic and integration adapter, expose a role-appropriate action and clear conflict/pending/approved states.
- **E18-S02-T03 Evidence/pilot:** Run positive, negative, replay and business-invariant checks using a named pilot fixture; capture acceptance evidence and rollback procedure.

**Merged production-verification elaboration (required):** Deploy/uninstall shadow adapter with independent reconciliation; device/network crash drill captured before signoff.

- **E18-S02-T04 — Stateful integration:** identify the authoritative aggregate and durable migration, exact API handler/source path and outbox mapping; record nonauthoritative legacy observations separately.
- **E18-S02-T05 — Negative and replay fixture:** reproduce the stated original acceptance condition and its failure counterpart; prove tenant isolation, idempotent duplicates, concurrency and restart preservation at a named database transaction boundary.
- **E18-S02-T06 — Worker/pilot proof:** capture the relevant operator or manager role action, device/source evidence, pending/blocked UI, measured accepted output and reviewer signature. Mark IMPLEMENTED-UNVERIFIED until proved.


#### E18-S03 — Prepare golden factory fixtures

**Story:** As a relevant factory user, I need to use real PO 220988 and IZOD variants without assuming conflicting OB valid, so that work is explainable, safe and actionable.
**Acceptance:** Acceptance held on source mismatch.
**Dependencies:** Prior schema/source contracts in Platform / Ops; existing installed legacy and device capabilities must be discovered.
**Delivery tasks:**

- **E18-S03-T01 Domain/API:** Define source-of-truth, typed inputs/outputs, idempotency, permissions, state transitions and failure contracts.
- **E18-S03-T02 Implementation/UI:** Implement owning logic and integration adapter, expose a role-appropriate action and clear conflict/pending/approved states.
- **E18-S03-T03 Evidence/pilot:** Run positive, negative, replay and business-invariant checks using a named pilot fixture; capture acceptance evidence and rollback procedure.

**Merged production-verification elaboration (required):** Deploy/uninstall shadow adapter with independent reconciliation; device/network crash drill captured before signoff.

- **E18-S03-T04 — Stateful integration:** identify the authoritative aggregate and durable migration, exact API handler/source path and outbox mapping; record nonauthoritative legacy observations separately.
- **E18-S03-T05 — Negative and replay fixture:** reproduce the stated original acceptance condition and its failure counterpart; prove tenant isolation, idempotent duplicates, concurrency and restart preservation at a named database transaction boundary.
- **E18-S03-T06 — Worker/pilot proof:** capture the relevant operator or manager role action, device/source evidence, pending/blocked UI, measured accepted output and reviewer signature. Mark IMPLEMENTED-UNVERIFIED until proved.


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

**Merged production-verification elaboration (required):** Deploy/uninstall shadow adapter with independent reconciliation; device/network crash drill captured before signoff.

- **E18-S04-T04 — Stateful integration:** identify the authoritative aggregate and durable migration, exact API handler/source path and outbox mapping; record nonauthoritative legacy observations separately.
- **E18-S04-T05 — Negative and replay fixture:** reproduce the stated original acceptance condition and its failure counterpart; prove tenant isolation, idempotent duplicates, concurrency and restart preservation at a named database transaction boundary.
- **E18-S04-T06 — Worker/pilot proof:** capture the relevant operator or manager role action, device/source evidence, pending/blocked UI, measured accepted output and reviewer signature. Mark IMPLEMENTED-UNVERIFIED until proved.


## 4. Consolidated V5 operational epics E19–E25 — *fully detailed stories and alternate-source acceptance*

The second V5's “Epic 01–07” repeats and deepens specific requirements here. Its original wording (acceptance numbers and exact proposed implementation tasks) is retained **inside its mapped canonical story** with explicit master-contract qualifications. `ALT-xx.x` denotes provenance, not another ticket to bill.


### E19 — Founder Governance & Material Control


#### E19-S01 — CEO-only exception signing  [V5 A2]
**Canonical user story:** As the **Factory CEO**, I need to personally sign a scoped material exception with a written Actual Story and independent authentication, so that no unauthorized shortage or excess issue becomes a released physical movement.


**Original V5 templated intent (retained for provenance):** As the accountable factory role, I can ceo-only exception signing so downstream production, financial and audit decisions remain correct.
**Business contract:** Bind authenticated CEO identity, written actual story, exact event hash, immutable permission constraint and nonce; refuse delegates and replay.
- **E19-S01-T01 — Domain/authority:** Define exact input identities, source-of-truth state, permission/CEO constraints, rejection codes and ownership.
- **E19-S01-T02 — Persistence/API:** Implement transactional schema, idempotency and domain actions for: Bind authenticated CEO identity, written actual story, exact event hash, immutable permission constraint and nonce; refuse delegates and replay.
- **E19-S01-T03 — Floor/manager UX:** Build role-specific happy path, blocked/unknown states, recovery and human-readable evidence presentation.
- **E19-S01-T04 — Invariants & adversarial tests:** Test concurrent requests, duplicate/replayed/offline input, wrong buyer/tenant, invalid revision/quantity and tamper resistance.
- **E19-S01-T05 — Pilot/acceptance:** Capture signed accepted and rejected fixtures, source snapshot, actual event trace, restart recovery and rollback result.
**Done when:** Actual authorized operation survives restart and replay, rejects violating evidence, propagates correct downstream state, and satisfies a recorded pilot acceptance test.

##### Integrated alternate story ALT-01.2 — Zero-Borrowing Hard Lock & CEO Narrative Override

**Provenance:** second V5 engineering backlog, Story 01.2. The following original acceptance/tasks are preserved as additional implementation detail, subject to the Master PRD/ARD.


- **As the** Factory CEO,

- **I want** physical material issuances that exceed order reservation to be blocked at stores until I sign off with an "Actual Story" narrative,

- **So that** informal inter-order borrowing is made structurally impossible.

- **Acceptance Criteria**:

  1.  Stores issuance engine hard-blocks if requested length exceeds order-reserved roll yardage.

  2.  System generates a ConsumptionVarianceRequest requiring CEO FIDO2/hardware-key authentication.

  3.  Re-cut or consumption variance requests without a written narrative root cause are rejected.

- **Tasks**:

  - TASK-012-1: Implement CheckSacrosanctReservation guard in services/edge-daemon/pkg/stores/issuance.go.

  - TASK-012-2: Build POST /v1/governance/overrides/consumption-variance endpoint with cryptographic signature validation.

  - TASK-012-3: Implement EXEC-01 CEO override dashboard with narrative entry and impact ledger.


**Master correction / qualification:** Material borrowing is forbidden. CEO consent governs specific defined exceptions; no cross-PO transfer without approved new plan and free reservation.


**Explicit closure tests for E19-S01:** Use the business contract above to construct one successful signed fixture, one policy/identity rejection fixture and one network/restart replay fixture; prove that the actual measured state equals the ledger, not a UI counter.

- **E19-S01-T06 — Schema/transaction review:** reviewer checks exact active IDs, tenant compound keys, expected invariants, migration/rollback, and canonical event ownership.
- **E19-S01-T07 — End-to-end physical failure fixture:** demonstrate forbidden alternative and stale/duplicate/cross-PO event; preserve rejected evidence with zero unauthorized state mutation.
- **E19-S01-T08 — Promotion evidence:** attach UI/device proof, real/fixture source documents, persisted event trail, clock/latency observations and signed acceptance against master V5-T / MGT gates.


#### E19-S02 — Top Management Council shortage replan  [V5 A2]
**Canonical user story:** As the **Planning Council chair**, I need to coordinate a versioned cross-department decision on supplier shortage or rejected material, so that affected orders replan openly without inventing usable stock.


**Original V5 templated intent (retained for provenance):** As the accountable factory role, I can top management council shortage replan so downstream production, financial and audit decisions remain correct.
**Business contract:** Create shortage docket, required council membership, meeting decision, impact snapshot, signed approved revision and downstream lock.
- **E19-S02-T01 — Domain/authority:** Define exact input identities, source-of-truth state, permission/CEO constraints, rejection codes and ownership.
- **E19-S02-T02 — Persistence/API:** Implement transactional schema, idempotency and domain actions for: Create shortage docket, required council membership, meeting decision, impact snapshot, signed approved revision and downstream lock.
- **E19-S02-T03 — Floor/manager UX:** Build role-specific happy path, blocked/unknown states, recovery and human-readable evidence presentation.
- **E19-S02-T04 — Invariants & adversarial tests:** Test concurrent requests, duplicate/replayed/offline input, wrong buyer/tenant, invalid revision/quantity and tamper resistance.
- **E19-S02-T05 — Pilot/acceptance:** Capture signed accepted and rejected fixtures, source snapshot, actual event trace, restart recovery and rollback result.
**Done when:** Actual authorized operation survives restart and replay, rejects violating evidence, propagates correct downstream state, and satisfies a recorded pilot acceptance test.

**Added deterministic invariant:** Council quorum includes CEO, Sales/Merchandising, Sourcing, Production, IE and Finance; shortage and rejected fabric remain unavailable while decision pending.


**Explicit closure tests for E19-S02:** Council quorum includes CEO, Sales/Merchandising, Sourcing, Production, IE and Finance; shortage and rejected fabric remain unavailable while decision pending.

- **E19-S02-T06 — Schema/transaction review:** reviewer checks exact active IDs, tenant compound keys, expected invariants, migration/rollback, and canonical event ownership.
- **E19-S02-T07 — End-to-end physical failure fixture:** demonstrate forbidden alternative and stale/duplicate/cross-PO event; preserve rejected evidence with zero unauthorized state mutation.
- **E19-S02-T08 — Promotion evidence:** attach UI/device proof, real/fixture source documents, persisted event trail, clock/latency observations and signed acceptance against master V5-T / MGT gates.


#### E19-S03 — Non-borrowing + atomic reservations  [V5 A1]
**Canonical user story:** As the **Stores Head**, I need to enforce atomic order reservations and prevent cross-order material borrowing, so that each PO uses only authorized eligible physical inventory.


**Original V5 templated intent (retained for provenance):** As the accountable factory role, I can non-borrowing + atomic reservations so downstream production, financial and audit decisions remain correct.
**Business contract:** Enforce buyer ownership, order-specific reservations and no cross-PO lending at database transaction boundary.
- **E19-S03-T01 — Domain/authority:** Define exact input identities, source-of-truth state, permission/CEO constraints, rejection codes and ownership.
- **E19-S03-T02 — Persistence/API:** Implement transactional schema, idempotency and domain actions for: Enforce buyer ownership, order-specific reservations and no cross-PO lending at database transaction boundary.
- **E19-S03-T03 — Floor/manager UX:** Build role-specific happy path, blocked/unknown states, recovery and human-readable evidence presentation.
- **E19-S03-T04 — Invariants & adversarial tests:** Test concurrent requests, duplicate/replayed/offline input, wrong buyer/tenant, invalid revision/quantity and tamper resistance.
- **E19-S03-T05 — Pilot/acceptance:** Capture signed accepted and rejected fixtures, source snapshot, actual event trace, restart recovery and rollback result.
**Done when:** Actual authorized operation survives restart and replay, rejects violating evidence, propagates correct downstream state, and satisfies a recorded pilot acceptance test.

**Added deterministic invariant:** Atomic cross-PO borrowing prohibition applies to UI, direct API, legacy imports and concurrent optimizing workers.


**Explicit closure tests for E19-S03:** Atomic cross-PO borrowing prohibition applies to UI, direct API, legacy imports and concurrent optimizing workers.

- **E19-S03-T06 — Schema/transaction review:** reviewer checks exact active IDs, tenant compound keys, expected invariants, migration/rollback, and canonical event ownership.
- **E19-S03-T07 — End-to-end physical failure fixture:** demonstrate forbidden alternative and stale/duplicate/cross-PO event; preserve rejected evidence with zero unauthorized state mutation.
- **E19-S03-T08 — Promotion evidence:** attach UI/device proof, real/fixture source documents, persisted event trail, clock/latency observations and signed acceptance against master V5-T / MGT gates.


#### E19-S04 — Dual-witness physical custody  [V5 A3]
**Canonical user story:** As the **Store issuer and recipient**, I need to independently witness exact physical material issuance and receipt, so that custody is verifiable even after a device failure.


**Original V5 templated intent (retained for provenance):** As the accountable factory role, I can dual-witness physical custody so downstream production, financial and audit decisions remain correct.
**Business contract:** Verify issuer and recipient independently at transfer, physical identity, quantity and audit receipt; account for offline handoff.
- **E19-S04-T01 — Domain/authority:** Define exact input identities, source-of-truth state, permission/CEO constraints, rejection codes and ownership.
- **E19-S04-T02 — Persistence/API:** Implement transactional schema, idempotency and domain actions for: Verify issuer and recipient independently at transfer, physical identity, quantity and audit receipt; account for offline handoff.
- **E19-S04-T03 — Floor/manager UX:** Build role-specific happy path, blocked/unknown states, recovery and human-readable evidence presentation.
- **E19-S04-T04 — Invariants & adversarial tests:** Test concurrent requests, duplicate/replayed/offline input, wrong buyer/tenant, invalid revision/quantity and tamper resistance.
- **E19-S04-T05 — Pilot/acceptance:** Capture signed accepted and rejected fixtures, source snapshot, actual event trace, restart recovery and rollback result.
**Done when:** Actual authorized operation survives restart and replay, rejects violating evidence, propagates correct downstream state, and satisfies a recorded pilot acceptance test.

##### Integrated alternate story ALT-01.3 — Double-Witness Material Custody Handshake

**Provenance:** second V5 engineering backlog, Story 01.3. The following original acceptance/tasks are preserved as additional implementation detail, subject to the Master PRD/ARD.


- **As a** Stores Clerk,

- **I want** every material roll issuance out of the warehouse to require distinct digital signatures from both the issuer and the cutting receiver,

- **So that** verbal handoffs are eliminated and custody is cryptographically proven.

- **Acceptance Criteria**:

  1.  Reject issuances where issuer_id == recipient_id or either digital ID is empty/unauthenticated.

  2.  Transaction generates an immutable HMAC/Ed25519 CustodyTransferEvent containing roll ID, cut number, issued length, and timestamps.

  3.  Cutting tables verify the double-witness cryptographic signature offline before commencing spreading.

- **Tasks**:

  - TASK-013-1: Enhance store_issuances schema with cryptographic witness proofs.

  - TASK-013-2: Implement dual-badge scan validation workflow in STR-04 tablet terminal.

  - TASK-013-3: Add offline signature verification test in services/edge-daemon/pkg/stores/issuance_test.go.


**Explicit closure tests for E19-S04:** Use the business contract above to construct one successful signed fixture, one policy/identity rejection fixture and one network/restart replay fixture; prove that the actual measured state equals the ledger, not a UI counter.

- **E19-S04-T06 — Schema/transaction review:** reviewer checks exact active IDs, tenant compound keys, expected invariants, migration/rollback, and canonical event ownership.
- **E19-S04-T07 — End-to-end physical failure fixture:** demonstrate forbidden alternative and stale/duplicate/cross-PO event; preserve rejected evidence with zero unauthorized state mutation.
- **E19-S04-T08 — Promotion evidence:** attach UI/device proof, real/fixture source documents, persisted event trail, clock/latency observations and signed acceptance against master V5-T / MGT gates.


#### E19-S05 — Surplus-first indent hard gate  [V5 D1]
**Canonical user story:** As the **Sourcing Head**, I need to net eligible unreserved surplus before approving a new supplier indent, so that we do not purchase stock already available and permitted for use.


**Original V5 templated intent (retained for provenance):** As the accountable factory role, I can surplus-first indent hard gate so downstream production, financial and audit decisions remain correct.
**Business contract:** Apply six-dimensional eligibility, net requirements and approved exceptions before issuing PO.
- **E19-S05-T01 — Domain/authority:** Define exact input identities, source-of-truth state, permission/CEO constraints, rejection codes and ownership.
- **E19-S05-T02 — Persistence/API:** Implement transactional schema, idempotency and domain actions for: Apply six-dimensional eligibility, net requirements and approved exceptions before issuing PO.
- **E19-S05-T03 — Floor/manager UX:** Build role-specific happy path, blocked/unknown states, recovery and human-readable evidence presentation.
- **E19-S05-T04 — Invariants & adversarial tests:** Test concurrent requests, duplicate/replayed/offline input, wrong buyer/tenant, invalid revision/quantity and tamper resistance.
- **E19-S05-T05 — Pilot/acceptance:** Capture signed accepted and rejected fixtures, source snapshot, actual event trace, restart recovery and rollback result.
**Done when:** Actual authorized operation survives restart and replay, rejects violating evidence, propagates correct downstream state, and satisfies a recorded pilot acceptance test.

### E20 — Commercial Costing & Promise Ledger

##### Integrated alternate story ALT-01.1 — Pre-PO Surplus Compatibility Engine

**Provenance:** second V5 engineering backlog, Story 01.1. The following original acceptance/tasks are preserved as additional implementation detail, subject to the Master PRD/ARD.


- **As a** Sourcing Head,

- **I want** the system to match BOM requirements against unreserved stock across 6 criteria before issuing supplier purchase indents,

- **So that** the factory never procures fresh fabric when usable surplus already sits in the warehouse.

- **Acceptance Criteria**:

  1.  Match criteria evaluates: exact fabric spec/GSM \$\pm 3\\\$, shade band Delta-E, usable width $\geq$ marker width, inspection status ACCEPTED_PASS, stock age within limits, and buyer escrow compatibility.

  2.  Purchase indent quantity is clamped strictly to $\max_{}(0,BOMRequired - EligibleSurplus)$.

  3.  Merchandiser overrides require a mandatory structured reason code and increment the per-user override frequency metric.

- **Tasks**:

  - TASK-011-1: Create database migration for surplus_compatibility_rules and indent_overrides.

  - TASK-011-2: Implement EvaluatePrePOGate(bomLine, inventoryPool) in services/edge-daemon/pkg/stores/indent.go.

  - TASK-011-3: Wire gate validation into STR-01 desktop UI.


**Master correction / qualification:** GSM ±3% and shade ΔE are buyer-configured proposals; no merchandising override bypasses an absolute buyer/ownership lock.


**Explicit closure tests for E19-S05:** Use the business contract above to construct one successful signed fixture, one policy/identity rejection fixture and one network/restart replay fixture; prove that the actual measured state equals the ledger, not a UI counter.

- **E19-S05-T06 — Schema/transaction review:** reviewer checks exact active IDs, tenant compound keys, expected invariants, migration/rollback, and canonical event ownership.
- **E19-S05-T07 — End-to-end physical failure fixture:** demonstrate forbidden alternative and stale/duplicate/cross-PO event; preserve rejected evidence with zero unauthorized state mutation.
- **E19-S05-T08 — Promotion evidence:** attach UI/device proof, real/fixture source documents, persisted event trail, clock/latency observations and signed acceptance against master V5-T / MGT gates.


### E20 — Commercial Costing & Promise Ledger


#### E20-S01 — Commercial buyer and buying-house hierarchy  [V5 B1]
**Canonical user story:** As the **Merchandising Head**, I need to maintain buyer, buying-house and agent terms and contacts by effective date, so that each quote applies the correct contractual obligations.


**Original V5 templated intent (retained for provenance):** As the accountable factory role, I can commercial buyer and buying-house hierarchy so downstream production, financial and audit decisions remain correct.
**Business contract:** Implement buyer agents, contact terms, compliance, claimed history, quote ownership and negotiation rounds.
- **E20-S01-T01 — Domain/authority:** Define exact input identities, source-of-truth state, permission/CEO constraints, rejection codes and ownership.
- **E20-S01-T02 — Persistence/API:** Implement transactional schema, idempotency and domain actions for: Implement buyer agents, contact terms, compliance, claimed history, quote ownership and negotiation rounds.
- **E20-S01-T03 — Floor/manager UX:** Build role-specific happy path, blocked/unknown states, recovery and human-readable evidence presentation.
- **E20-S01-T04 — Invariants & adversarial tests:** Test concurrent requests, duplicate/replayed/offline input, wrong buyer/tenant, invalid revision/quantity and tamper resistance.
- **E20-S01-T05 — Pilot/acceptance:** Capture signed accepted and rejected fixtures, source snapshot, actual event trace, restart recovery and rollback result.
**Done when:** Actual authorized operation survives restart and replay, rejects violating evidence, propagates correct downstream state, and satisfies a recorded pilot acceptance test.

**Added deterministic invariant:** Buyer → agent/buying house → office/contact; contract terms effective-dated, source-linked and not merged by name alone.


**Explicit closure tests for E20-S01:** Buyer → agent/buying house → office/contact; contract terms effective-dated, source-linked and not merged by name alone.

- **E20-S01-T06 — Schema/transaction review:** reviewer checks exact active IDs, tenant compound keys, expected invariants, migration/rollback, and canonical event ownership.
- **E20-S01-T07 — End-to-end physical failure fixture:** demonstrate forbidden alternative and stale/duplicate/cross-PO event; preserve rejected evidence with zero unauthorized state mutation.
- **E20-S01-T08 — Promotion evidence:** attach UI/device proof, real/fixture source documents, persisted event trail, clock/latency observations and signed acceptance against master V5-T / MGT gates.


#### E20-S02 — Four costing policies and granular CMT  [V5 B1]
**Canonical user story:** As the **Costing Engineer**, I need to select one of the founder-approved costing policies and show Cutting/Making/Finishing economics separately, so that quotes are reproducible and margin-assumption errors are visible.


**Original V5 templated intent (retained for provenance):** As the accountable factory role, I can four costing policies and granular cmt so downstream production, financial and audit decisions remain correct.
**Business contract:** Implement founder-approved costing policy alternatives; independently compute cutting/making/finishing and overheads.
- **E20-S02-T01 — Domain/authority:** Define exact input identities, source-of-truth state, permission/CEO constraints, rejection codes and ownership.
- **E20-S02-T02 — Persistence/API:** Implement transactional schema, idempotency and domain actions for: Implement founder-approved costing policy alternatives; independently compute cutting/making/finishing and overheads.
- **E20-S02-T03 — Floor/manager UX:** Build role-specific happy path, blocked/unknown states, recovery and human-readable evidence presentation.
- **E20-S02-T04 — Invariants & adversarial tests:** Test concurrent requests, duplicate/replayed/offline input, wrong buyer/tenant, invalid revision/quantity and tamper resistance.
- **E20-S02-T05 — Pilot/acceptance:** Capture signed accepted and rejected fixtures, source snapshot, actual event trace, restart recovery and rollback result.
**Done when:** Actual authorized operation survives restart and replay, rejects violating evidence, propagates correct downstream state, and satisfies a recorded pilot acceptance test.

**Added deterministic invariant:** Four source-defined costing policies, distinct Cutting/Making/Finishing CMT; prohibit invented formula until founder review.


**Explicit closure tests for E20-S02:** Four source-defined costing policies, distinct Cutting/Making/Finishing CMT; prohibit invented formula until founder review.

- **E20-S02-T06 — Schema/transaction review:** reviewer checks exact active IDs, tenant compound keys, expected invariants, migration/rollback, and canonical event ownership.
- **E20-S02-T07 — End-to-end physical failure fixture:** demonstrate forbidden alternative and stale/duplicate/cross-PO event; preserve rejected evidence with zero unauthorized state mutation.
- **E20-S02-T08 — Promotion evidence:** attach UI/device proof, real/fixture source documents, persisted event trail, clock/latency observations and signed acceptance against master V5-T / MGT gates.


#### E20-S03 — Versioned quote assumption ledger  [V5 B2]
**Canonical user story:** As the **Merchandiser**, I need to freeze the sources and assumptions behind an approved quotation, so that later engineering changes do not rewrite the original commercial promise.


**Original V5 templated intent (retained for provenance):** As the accountable factory role, I can versioned quote assumption ledger so downstream production, financial and audit decisions remain correct.
**Business contract:** Freeze OB SAM, fabric yield, supplier ETA, exchange rates, capex, margin and confidence on quote signature.
- **E20-S03-T01 — Domain/authority:** Define exact input identities, source-of-truth state, permission/CEO constraints, rejection codes and ownership.
- **E20-S03-T02 — Persistence/API:** Implement transactional schema, idempotency and domain actions for: Freeze OB SAM, fabric yield, supplier ETA, exchange rates, capex, margin and confidence on quote signature.
- **E20-S03-T03 — Floor/manager UX:** Build role-specific happy path, blocked/unknown states, recovery and human-readable evidence presentation.
- **E20-S03-T04 — Invariants & adversarial tests:** Test concurrent requests, duplicate/replayed/offline input, wrong buyer/tenant, invalid revision/quantity and tamper resistance.
- **E20-S03-T05 — Pilot/acceptance:** Capture signed accepted and rejected fixtures, source snapshot, actual event trace, restart recovery and rollback result.
**Done when:** Actual authorized operation survives restart and replay, rejects violating evidence, propagates correct downstream state, and satisfies a recorded pilot acceptance test.

**Added deterministic invariant:** Frozen quote source hashes and assumption revisions remain reproducible after SAM/FX/fabric changes.


**Explicit closure tests for E20-S03:** Frozen quote source hashes and assumption revisions remain reproducible after SAM/FX/fabric changes.

- **E20-S03-T06 — Schema/transaction review:** reviewer checks exact active IDs, tenant compound keys, expected invariants, migration/rollback, and canonical event ownership.
- **E20-S03-T07 — End-to-end physical failure fixture:** demonstrate forbidden alternative and stale/duplicate/cross-PO event; preserve rejected evidence with zero unauthorized state mutation.
- **E20-S03-T08 — Promotion evidence:** attach UI/device proof, real/fixture source documents, persisted event trail, clock/latency observations and signed acceptance against master V5-T / MGT gates.


#### E20-S04 — SAM change commercial-impact automation  [V5 B2]
**Canonical user story:** As the **Costing Head**, I need to automatically assess commercial impact when approved Factory SAM or OB changes, so that affected quotes are re-costed and reviewed before margin erosion is hidden.


**Original V5 templated intent (retained for provenance):** As the accountable factory role, I can sam change commercial-impact automation so downstream production, financial and audit decisions remain correct.
**Business contract:** Re-cost affected quoted/booked orders on approved OB revision and route consequential changes to merchandiser.
- **E20-S04-T01 — Domain/authority:** Define exact input identities, source-of-truth state, permission/CEO constraints, rejection codes and ownership.
- **E20-S04-T02 — Persistence/API:** Implement transactional schema, idempotency and domain actions for: Re-cost affected quoted/booked orders on approved OB revision and route consequential changes to merchandiser.
- **E20-S04-T03 — Floor/manager UX:** Build role-specific happy path, blocked/unknown states, recovery and human-readable evidence presentation.
- **E20-S04-T04 — Invariants & adversarial tests:** Test concurrent requests, duplicate/replayed/offline input, wrong buyer/tenant, invalid revision/quantity and tamper resistance.
- **E20-S04-T05 — Pilot/acceptance:** Capture signed accepted and rejected fixtures, source snapshot, actual event trace, restart recovery and rollback result.
**Done when:** Actual authorized operation survives restart and replay, rejects violating evidence, propagates correct downstream state, and satisfies a recorded pilot acceptance test.

**Added deterministic invariant:** New OB SAM revision creates candidate recost and merchandiser decision; never overwrite booked version.


**Explicit closure tests for E20-S04:** New OB SAM revision creates candidate recost and merchandiser decision; never overwrite booked version.

- **E20-S04-T06 — Schema/transaction review:** reviewer checks exact active IDs, tenant compound keys, expected invariants, migration/rollback, and canonical event ownership.
- **E20-S04-T07 — End-to-end physical failure fixture:** demonstrate forbidden alternative and stale/duplicate/cross-PO event; preserve rejected evidence with zero unauthorized state mutation.
- **E20-S04-T08 — Promotion evidence:** attach UI/device proof, real/fixture source documents, persisted event trail, clock/latency observations and signed acceptance against master V5-T / MGT gates.


#### E20-S05 — Margin evolution and variance attribution  [V5 B3]
**Canonical user story:** As the **CFO**, I need to reconcile quoted, booked, preproduction, projected and realized contribution, so that price and execution variance have signed, nonoverlapping explanations.


**Original V5 templated intent (retained for provenance):** As the accountable factory role, I can margin evolution and variance attribution so downstream production, financial and audit decisions remain correct.
**Business contract:** Calculate quoted/booked/projected/final margins and distinguish estimate variance from execution variance.
- **E20-S05-T01 — Domain/authority:** Define exact input identities, source-of-truth state, permission/CEO constraints, rejection codes and ownership.
- **E20-S05-T02 — Persistence/API:** Implement transactional schema, idempotency and domain actions for: Calculate quoted/booked/projected/final margins and distinguish estimate variance from execution variance.
- **E20-S05-T03 — Floor/manager UX:** Build role-specific happy path, blocked/unknown states, recovery and human-readable evidence presentation.
- **E20-S05-T04 — Invariants & adversarial tests:** Test concurrent requests, duplicate/replayed/offline input, wrong buyer/tenant, invalid revision/quantity and tamper resistance.
- **E20-S05-T05 — Pilot/acceptance:** Capture signed accepted and rejected fixtures, source snapshot, actual event trace, restart recovery and rollback result.
**Done when:** Actual authorized operation survives restart and replay, rejects violating evidence, propagates correct downstream state, and satisfies a recorded pilot acceptance test.

### E21 — Factory Planning & Capacity Doctrine

##### Integrated alternate story ALT-02.2 — Closed-Loop Margin Waterfall & Variance Attribution

**Provenance:** second V5 engineering backlog, Story 02.2. The following original acceptance/tasks are preserved as additional implementation detail, subject to the Master PRD/ARD.


- **As the** Chief Financial Officer,

- **I want** margin variance between Quote and Actual Closure decomposed into non-overlapping operational buckets,

- **So that** profitability leakages are attributed to their root physical causes.

- **Acceptance Criteria**:

  1.  Track 5 margin milestones: Quoted $\rightarrow$ Booked $\rightarrow$ Pre-Production $\rightarrow$ Dynamic Projected $\rightarrow$ Realized.

  2.  Final variance decomposed into: SAM drift, efficiency drift, material yield, machine downtime, skill mismatch, and scrap.

  3.  Persist attribution records in cost_variance_attribution table upon Order Closure.

- **Tasks**:

  - TASK-022-1: Create database migration for cost_variance_attribution.

  - TASK-022-2: Implement margin waterfall calculation engine in services/core-api.

  - TASK-022-3: Render waterfall breakdown chart on FIN-01 and EXEC-03 screens.


**Explicit closure tests for E20-S05:** Use the business contract above to construct one successful signed fixture, one policy/identity rejection fixture and one network/restart replay fixture; prove that the actual measured state equals the ledger, not a UI counter.

- **E20-S05-T06 — Schema/transaction review:** reviewer checks exact active IDs, tenant compound keys, expected invariants, migration/rollback, and canonical event ownership.
- **E20-S05-T07 — End-to-end physical failure fixture:** demonstrate forbidden alternative and stale/duplicate/cross-PO event; preserve rejected evidence with zero unauthorized state mutation.
- **E20-S05-T08 — Promotion evidence:** attach UI/device proof, real/fixture source documents, persisted event trail, clock/latency observations and signed acceptance against master V5-T / MGT gates.


### E21 — Factory Planning & Capacity Doctrine


#### E21-S01 — Three-layer immutable DPP  [V5 C1]
**Canonical user story:** As the **Factory Head**, I need to publish a reason-coded daily commitment separate from IE target and measured achievement, so that planning optimism can be distinguished from operating delivery.


**Original V5 templated intent (retained for provenance):** As the accountable factory role, I can three-layer immutable dpp so downstream production, financial and audit decisions remain correct.
**Business contract:** Freeze IE baseline, factory-head reason-coded commitment and measured RFID actuals independently per shift.
- **E21-S01-T01 — Domain/authority:** Define exact input identities, source-of-truth state, permission/CEO constraints, rejection codes and ownership.
- **E21-S01-T02 — Persistence/API:** Implement transactional schema, idempotency and domain actions for: Freeze IE baseline, factory-head reason-coded commitment and measured RFID actuals independently per shift.
- **E21-S01-T03 — Floor/manager UX:** Build role-specific happy path, blocked/unknown states, recovery and human-readable evidence presentation.
- **E21-S01-T04 — Invariants & adversarial tests:** Test concurrent requests, duplicate/replayed/offline input, wrong buyer/tenant, invalid revision/quantity and tamper resistance.
- **E21-S01-T05 — Pilot/acceptance:** Capture signed accepted and rejected fixtures, source snapshot, actual event trace, restart recovery and rollback result.
**Done when:** Actual authorized operation survives restart and replay, rejects violating evidence, propagates correct downstream state, and satisfies a recorded pilot acceptance test.

##### Integrated alternate story ALT-03.2 — Three-Layer Daily Production Plan (DPP) Variance Loop

**Provenance:** second V5 engineering backlog, Story 03.2. The following original acceptance/tasks are preserved as additional implementation detail, subject to the Master PRD/ARD.


- **As a** Factory Head,

- **I want** to record daily line commitments across three distinct layers (IE Default, Adjusted, and Actual),

- **So that** chronic planning optimism is separated from floor execution failures.

- **Acceptance Criteria**:

  1.  Layer 1 (IE Default) is computed from 100% calibrated SAM.

  2.  Layer 2 (Adjusted) requires Factory Head sign-off by 08:30 with mandatory reason codes (absenteeism, line balancing, machine maintenance).

  3.  Layer 3 (Actual) is populated solely from validated workstation telemetry.

  4.  Realism Gap ($L1 - L2$) and Execution Gap ($L2 - L3$) are surfaced in the daily huddle screen (PM-05).

- **Tasks**:

  - TASK-032-1: Implement schema and endpoints for daily_production_plan.

  - TASK-032-2: Build PLN-03 daily adjustment and reason-code capture screen.

  - TASK-032-3: Connect PM-05 situation room to live 3-layer variance streams.


**Master correction / qualification:** 08:30 and >10% for five shifts are plant-configured proposed thresholds; actual means accepted output.


**Explicit closure tests for E21-S01:** Use the business contract above to construct one successful signed fixture, one policy/identity rejection fixture and one network/restart replay fixture; prove that the actual measured state equals the ledger, not a UI counter.

- **E21-S01-T06 — Schema/transaction review:** reviewer checks exact active IDs, tenant compound keys, expected invariants, migration/rollback, and canonical event ownership.
- **E21-S01-T07 — End-to-end physical failure fixture:** demonstrate forbidden alternative and stale/duplicate/cross-PO event; preserve rejected evidence with zero unauthorized state mutation.
- **E21-S01-T08 — Promotion evidence:** attach UI/device proof, real/fixture source documents, persisted event trail, clock/latency observations and signed acceptance against master V5-T / MGT gates.


#### E21-S02 — Eight-stream schedule with sewing anchor  [V5 C2]
**Canonical user story:** As the **Production Planner**, I need to coordinate eight owned Time-and-Action streams around sewing readiness and delivery, so that critical delays propagate to the right accountable owners.


**Original V5 templated intent (retained for provenance):** As the accountable factory role, I can eight-stream schedule with sewing anchor so downstream production, financial and audit decisions remain correct.
**Business contract:** Implement source-defined milestones and dependencies, separate packing trims, critical path and slip propagation.
- **E21-S02-T01 — Domain/authority:** Define exact input identities, source-of-truth state, permission/CEO constraints, rejection codes and ownership.
- **E21-S02-T02 — Persistence/API:** Implement transactional schema, idempotency and domain actions for: Implement source-defined milestones and dependencies, separate packing trims, critical path and slip propagation.
- **E21-S02-T03 — Floor/manager UX:** Build role-specific happy path, blocked/unknown states, recovery and human-readable evidence presentation.
- **E21-S02-T04 — Invariants & adversarial tests:** Test concurrent requests, duplicate/replayed/offline input, wrong buyer/tenant, invalid revision/quantity and tamper resistance.
- **E21-S02-T05 — Pilot/acceptance:** Capture signed accepted and rejected fixtures, source snapshot, actual event trace, restart recovery and rollback result.
**Done when:** Actual authorized operation survives restart and replay, rejects violating evidence, propagates correct downstream state, and satisfies a recorded pilot acceptance test.

##### Integrated alternate story ALT-03.1 — Synchronized 8-Stream T&A Scheduler

**Provenance:** second V5 engineering backlog, Story 03.1. The following original acceptance/tasks are preserved as additional implementation detail, subject to the Master PRD/ARD.


- **As a** Production Planner,

- **I want** order scheduling governed across 8 parallel, interdependent T&A tracks,

- **So that** missing packaging trims, machine deficits, or skill shortages are caught weeks before sewing start.

- **Acceptance Criteria**:

  1.  Maintain 8 explicit streams: Fabric, Sewing Trims, Cutting, Sewing (bottleneck), Finishing, Packing Trims, Machine Plan, and Skill Plan.

  2.  Enforce strict dependency: Packing Trims T&A is isolated from Sewing Trims to prevent carton delays from halting assembly.

  3.  Slippage in any upstream stream automatically shifts the critical path and recalculates float days.

- **Tasks**:

  - TASK-031-1: Create relational schema for ta_stream_milestones with dependency graph edges.

  - TASK-031-2: Implement forward/backward cascade solver in pkg/planning/.

  - TASK-031-3: Build PLN-01 8-stream interactive Gantt view.


**Explicit closure tests for E21-S02:** Use the business contract above to construct one successful signed fixture, one policy/identity rejection fixture and one network/restart replay fixture; prove that the actual measured state equals the ledger, not a UI counter.

- **E21-S02-T06 — Schema/transaction review:** reviewer checks exact active IDs, tenant compound keys, expected invariants, migration/rollback, and canonical event ownership.
- **E21-S02-T07 — End-to-end physical failure fixture:** demonstrate forbidden alternative and stale/duplicate/cross-PO event; preserve rejected evidence with zero unauthorized state mutation.
- **E21-S02-T08 — Promotion evidence:** attach UI/device proof, real/fixture source documents, persisted event trail, clock/latency observations and signed acceptance against master V5-T / MGT gates.


#### E21-S03 — Factory-wide operator reassignment  [V5 C3]
**Canonical user story:** As the **Industrial Engineering Head**, I need to recommend globally beneficial operator transfers with training, donor-line and worker constraints, so that plant output improves without excessive line churn or compliance risk.


**Original V5 templated intent (retained for provenance):** As the accountable factory role, I can factory-wide operator reassignment so downstream production, financial and audit decisions remain correct.
**Business contract:** Optimize net plant output, qualified skills, home line soft preference, movement burden and labor compliance.
- **E21-S03-T01 — Domain/authority:** Define exact input identities, source-of-truth state, permission/CEO constraints, rejection codes and ownership.
- **E21-S03-T02 — Persistence/API:** Implement transactional schema, idempotency and domain actions for: Optimize net plant output, qualified skills, home line soft preference, movement burden and labor compliance.
- **E21-S03-T03 — Floor/manager UX:** Build role-specific happy path, blocked/unknown states, recovery and human-readable evidence presentation.
- **E21-S03-T04 — Invariants & adversarial tests:** Test concurrent requests, duplicate/replayed/offline input, wrong buyer/tenant, invalid revision/quantity and tamper resistance.
- **E21-S03-T05 — Pilot/acceptance:** Capture signed accepted and rejected fixtures, source snapshot, actual event trace, restart recovery and rollback result.
**Done when:** Actual authorized operation survives restart and replay, rejects violating evidence, propagates correct downstream state, and satisfies a recorded pilot acceptance test.

##### Integrated alternate story ALT-03.3 — Factory-Pool Labor Mobility & Compliance Audit Trail

**Provenance:** second V5 engineering backlog, Story 03.3. The following original acceptance/tasks are preserved as additional implementation detail, subject to the Master PRD/ARD.


- **As a** Compliance Auditor,

- **I want** an immutable log of every operator's daily line and station reassignments,

- **So that** buyer social compliance standards (SMETA, BSCI) are satisfied without line-churn productivity losses.

- **Acceptance Criteria**:

  1.  Operators maintain a "soft home line" default; cross-line transfers require IE skill verification.

  2.  Moving an operator from a bottleneck station triggers a supervisor confirmation warning.

  3.  Generate an immutable DailyWorkforceAssignmentRecord logging operator ID, line, station, active hours, and earned wages.

- **Tasks**:

  - TASK-033-1: Add DailyWorkforceAssignmentRecord entity and RLS policies.

  - TASK-033-2: Implement operator reallocation verification check in PM-04.

  - TASK-033-3: Build exportable buyer compliance muster report.


**Master correction / qualification:** Earned wages are read only from authorized payroll systems; wage calculation is not implicitly in scope.


**Explicit closure tests for E21-S03:** Use the business contract above to construct one successful signed fixture, one policy/identity rejection fixture and one network/restart replay fixture; prove that the actual measured state equals the ledger, not a UI counter.

- **E21-S03-T06 — Schema/transaction review:** reviewer checks exact active IDs, tenant compound keys, expected invariants, migration/rollback, and canonical event ownership.
- **E21-S03-T07 — End-to-end physical failure fixture:** demonstrate forbidden alternative and stale/duplicate/cross-PO event; preserve rejected evidence with zero unauthorized state mutation.
- **E21-S03-T08 — Promotion evidence:** attach UI/device proof, real/fixture source documents, persisted event trail, clock/latency observations and signed acceptance against master V5-T / MGT gates.


#### E21-S04 — Machine procurement and qualification pipeline  [V5 C4]
**Canonical user story:** As the **Maintenance and IE Heads**, I need to evaluate machine shortage against purchase, rental and reassignment options, so that approved capacity is ready before the production constraint becomes critical.


**Original V5 templated intent (retained for provenance):** As the accountable factory role, I can machine procurement and qualification pipeline so downstream production, financial and audit decisions remain correct.
**Business contract:** Model shortage detection, capex alternatives, approval, ordered/received/commissioned state and readiness.
- **E21-S04-T01 — Domain/authority:** Define exact input identities, source-of-truth state, permission/CEO constraints, rejection codes and ownership.
- **E21-S04-T02 — Persistence/API:** Implement transactional schema, idempotency and domain actions for: Model shortage detection, capex alternatives, approval, ordered/received/commissioned state and readiness.
- **E21-S04-T03 — Floor/manager UX:** Build role-specific happy path, blocked/unknown states, recovery and human-readable evidence presentation.
- **E21-S04-T04 — Invariants & adversarial tests:** Test concurrent requests, duplicate/replayed/offline input, wrong buyer/tenant, invalid revision/quantity and tamper resistance.
- **E21-S04-T05 — Pilot/acceptance:** Capture signed accepted and rejected fixtures, source snapshot, actual event trace, restart recovery and rollback result.
**Done when:** Actual authorized operation survives restart and replay, rejects violating evidence, propagates correct downstream state, and satisfies a recorded pilot acceptance test.

**Added deterministic invariant:** Machine procurement/rental/capex options have installation/commissioning lead times and non-conflicting capacity.


**Explicit closure tests for E21-S04:** Machine procurement/rental/capex options have installation/commissioning lead times and non-conflicting capacity.

- **E21-S04-T06 — Schema/transaction review:** reviewer checks exact active IDs, tenant compound keys, expected invariants, migration/rollback, and canonical event ownership.
- **E21-S04-T07 — End-to-end physical failure fixture:** demonstrate forbidden alternative and stale/duplicate/cross-PO event; preserve rejected evidence with zero unauthorized state mutation.
- **E21-S04-T08 — Promotion evidence:** attach UI/device proof, real/fixture source documents, persisted event trail, clock/latency observations and signed acceptance against master V5-T / MGT gates.


#### E21-S05 — Skills training and outsourcing governance  [V5 C4]
**Canonical user story:** As the **HR and Production Heads**, I need to forecast required skills and govern subcontract operations and returns, so that skill shortages and poor capacity estimates are addressed proactively.


**Original V5 templated intent (retained for provenance):** As the accountable factory role, I can skills training and outsourcing governance so downstream production, financial and audit decisions remain correct.
**Business contract:** Forecast skill deficits and onboarding; track subcontract justification, actual quality and recurring estimate drift.
- **E21-S05-T01 — Domain/authority:** Define exact input identities, source-of-truth state, permission/CEO constraints, rejection codes and ownership.
- **E21-S05-T02 — Persistence/API:** Implement transactional schema, idempotency and domain actions for: Forecast skill deficits and onboarding; track subcontract justification, actual quality and recurring estimate drift.
- **E21-S05-T03 — Floor/manager UX:** Build role-specific happy path, blocked/unknown states, recovery and human-readable evidence presentation.
- **E21-S05-T04 — Invariants & adversarial tests:** Test concurrent requests, duplicate/replayed/offline input, wrong buyer/tenant, invalid revision/quantity and tamper resistance.
- **E21-S05-T05 — Pilot/acceptance:** Capture signed accepted and rejected fixtures, source snapshot, actual event trace, restart recovery and rollback result.
**Done when:** Actual authorized operation survives restart and replay, rejects violating evidence, propagates correct downstream state, and satisfies a recorded pilot acceptance test.

### E22 — Cutting Exactness & Operator Contracts

**Added deterministic invariant:** Skill training and qualified subcontract capacity have forward deadlines, traceable material custody and return QA.


**Explicit closure tests for E21-S05:** Skill training and qualified subcontract capacity have forward deadlines, traceable material custody and return QA.

- **E21-S05-T06 — Schema/transaction review:** reviewer checks exact active IDs, tenant compound keys, expected invariants, migration/rollback, and canonical event ownership.
- **E21-S05-T07 — End-to-end physical failure fixture:** demonstrate forbidden alternative and stale/duplicate/cross-PO event; preserve rejected evidence with zero unauthorized state mutation.
- **E21-S05-T08 — Promotion evidence:** attach UI/device proof, real/fixture source documents, persisted event trail, clock/latency observations and signed acceptance against master V5-T / MGT gates.


### E22 — Cutting Exactness & Operator Contracts


#### E22-S01 — Classic cut-bundle profile generator  [V5 D2]
**Canonical user story:** As the **Cutting Numbering Operator**, I need to derive exact consecutive-ply Classic Profile bundles from accepted physical cuts, so that sewing never receives phantom serials or disruptive orphan bundles.


**Original V5 templated intent (retained for provenance):** As the accountable factory role, I can classic cut-bundle profile generator so downstream production, financial and audit decisions remain correct.
**Business contract:** Calculate cut/marker/part/ply groups, deterministic serials, consecutive ply validity and remainder redistribution.
- **E22-S01-T01 — Domain/authority:** Define exact input identities, source-of-truth state, permission/CEO constraints, rejection codes and ownership.
- **E22-S01-T02 — Persistence/API:** Implement transactional schema, idempotency and domain actions for: Calculate cut/marker/part/ply groups, deterministic serials, consecutive ply validity and remainder redistribution.
- **E22-S01-T03 — Floor/manager UX:** Build role-specific happy path, blocked/unknown states, recovery and human-readable evidence presentation.
- **E22-S01-T04 — Invariants & adversarial tests:** Test concurrent requests, duplicate/replayed/offline input, wrong buyer/tenant, invalid revision/quantity and tamper resistance.
- **E22-S01-T05 — Pilot/acceptance:** Capture signed accepted and rejected fixtures, source snapshot, actual event trace, restart recovery and rollback result.
**Done when:** Actual authorized operation survives restart and replay, rejects violating evidence, propagates correct downstream state, and satisfies a recorded pilot acceptance test.

##### Integrated alternate story ALT-04.2 — Anti-Orphan Bundle Balancing Arithmetic

**Provenance:** second V5 engineering backlog, Story 04.2. The following original acceptance/tasks are preserved as additional implementation detail, subject to the Master PRD/ARD.


- **As a** Cutting Room Operator,

- **I want** cut plies divided into balanced bundles without tiny orphan remainders,

- **So that** downstream sewing lines maintain steady WIP flow.

- **Acceptance Criteria**:

  1.  Given $P$ plies, target size $S_{t}$, and minimum size $S_{\min_{}}$, when remainder $R < S_{\min_{}}$, redistribute $R$ across final bundles such that all bundles $\geq S_{\min_{}}$.

  2.  Worked fixture: 101 plies, target 25, minimum 13 produces exactly $25,25,25,13,13$.

  3.  Redistribution preserves consecutive ply continuity and shade/roll boundaries.

- **Tasks**:

  - TASK-042-1: Implement CalculateBalancedBundles in services/edge-daemon/pkg/cutting/planner.go.

  - TASK-042-2: Write parameterized unit tests covering odd remainders, single-ply cuts, and extreme ratios.


**Master correction / qualification:** The cited grouping applies only when contiguous ply, shade and part boundaries permit it; optimize legal groups.


##### Integrated alternate story ALT-04.3 — Dual-Layer Sequential Numbering & Internal Provenance

**Provenance:** second V5 engineering backlog, Story 04.3. The following original acceptance/tasks are preserved as additional implementation detail, subject to the Master PRD/ARD.


- **As a** Sewing Floor Operator,

- **I want** bundle cards to display a simple sequential integer from 1 to $N$ per PO,

- **While the system** preserves full cut, marker position, and ply range lineage internally.

- **Acceptance Criteria**:

  1.  Human-readable bundle number is an integer \$1 \dots N\$ across the entire customer PO.

  2.  Server retains canonical tuple: cut_no, pant_position, part_type, ssb_group, ply_range, source_rolls, garment_serial_range.

  3.  Garment replacement panels cut from end-bits inherit the exact original garment serial number.

- **Tasks**:

  - TASK-043-1: Update bundle_registry database schema to enforce dual-layer identifiers.

  - TASK-043-2: Update bundle card generation logic in services/edge-daemon/pkg/cutting/planner.go.

  - TASK-043-3: Update CUT-03 post-cut numbering screen.


**Explicit closure tests for E22-S01:** Use the business contract above to construct one successful signed fixture, one policy/identity rejection fixture and one network/restart replay fixture; prove that the actual measured state equals the ledger, not a UI counter.

- **E22-S01-T06 — Schema/transaction review:** reviewer checks exact active IDs, tenant compound keys, expected invariants, migration/rollback, and canonical event ownership.
- **E22-S01-T07 — End-to-end physical failure fixture:** demonstrate forbidden alternative and stale/duplicate/cross-PO event; preserve rejected evidence with zero unauthorized state mutation.
- **E22-S01-T08 — Promotion evidence:** attach UI/device proof, real/fixture source documents, persisted event trail, clock/latency observations and signed acceptance against master V5-T / MGT gates.


#### E22-S02 — Cut Master frozen plan and permitted tweaks  [V5 D3]
**Canonical user story:** As the **Cutting Master**, I need to accept or make only permitted tweaks to a CAD-backed plan, so that the approved marker and reserved fabric remain controlled.


**Original V5 templated intent (retained for provenance):** As the accountable factory role, I can cut master frozen plan and permitted tweaks so downstream production, financial and audit decisions remain correct.
**Business contract:** Authorize only sequence, in-shade roll, ply, table; enforce marker/reservation locks and Planning-replan unlock.
- **E22-S02-T01 — Domain/authority:** Define exact input identities, source-of-truth state, permission/CEO constraints, rejection codes and ownership.
- **E22-S02-T02 — Persistence/API:** Implement transactional schema, idempotency and domain actions for: Authorize only sequence, in-shade roll, ply, table; enforce marker/reservation locks and Planning-replan unlock.
- **E22-S02-T03 — Floor/manager UX:** Build role-specific happy path, blocked/unknown states, recovery and human-readable evidence presentation.
- **E22-S02-T04 — Invariants & adversarial tests:** Test concurrent requests, duplicate/replayed/offline input, wrong buyer/tenant, invalid revision/quantity and tamper resistance.
- **E22-S02-T05 — Pilot/acceptance:** Capture signed accepted and rejected fixtures, source snapshot, actual event trace, restart recovery and rollback result.
**Done when:** Actual authorized operation survives restart and replay, rejects violating evidence, propagates correct downstream state, and satisfies a recorded pilot acceptance test.

**Added deterministic invariant:** Cut Master may change sequence/eligible roll/plies/table only; marker and reservation locked until signed plan version.


**Explicit closure tests for E22-S02:** Cut Master may change sequence/eligible roll/plies/table only; marker and reservation locked until signed plan version.

- **E22-S02-T06 — Schema/transaction review:** reviewer checks exact active IDs, tenant compound keys, expected invariants, migration/rollback, and canonical event ownership.
- **E22-S02-T07 — End-to-end physical failure fixture:** demonstrate forbidden alternative and stale/duplicate/cross-PO event; preserve rejected evidence with zero unauthorized state mutation.
- **E22-S02-T08 — Promotion evidence:** attach UI/device proof, real/fixture source documents, persisted event trail, clock/latency observations and signed acceptance against master V5-T / MGT gates.


#### E22-S03 — Cursor-based RFID commissioning  [V5 D4]
**Canonical user story:** As the **Bundling Helper**, I need to bind reusable RFID tags to printed physical bundle cards with a single scanning cursor, so that each real bundle has exactly one correctly verified active tag.


**Original V5 templated intent (retained for provenance):** As the accountable factory role, I can cursor-based rfid commissioning so downstream production, financial and audit decisions remain correct.
**Business contract:** Print cards, scan one unused tag, bind to next expected bundle atomically, readback and cursor advance.
- **E22-S03-T01 — Domain/authority:** Define exact input identities, source-of-truth state, permission/CEO constraints, rejection codes and ownership.
- **E22-S03-T02 — Persistence/API:** Implement transactional schema, idempotency and domain actions for: Print cards, scan one unused tag, bind to next expected bundle atomically, readback and cursor advance.
- **E22-S03-T03 — Floor/manager UX:** Build role-specific happy path, blocked/unknown states, recovery and human-readable evidence presentation.
- **E22-S03-T04 — Invariants & adversarial tests:** Test concurrent requests, duplicate/replayed/offline input, wrong buyer/tenant, invalid revision/quantity and tamper resistance.
- **E22-S03-T05 — Pilot/acceptance:** Capture signed accepted and rejected fixtures, source snapshot, actual event trace, restart recovery and rollback result.
**Done when:** Actual authorized operation survives restart and replay, rejects violating evidence, propagates correct downstream state, and satisfies a recorded pilot acceptance test.

##### Integrated alternate story ALT-04.4 — Cursor-Driven RFID Helper Commissioning Workflow

**Provenance:** second V5 engineering backlog, Story 04.4. The following original acceptance/tasks are preserved as additional implementation detail, subject to the Master PRD/ARD.


- **As a** Bundling Helper,

- **I want** a single-scan RFID binding interface that automatically advances the active cursor,

- **So that** I can tag 720 bundles per cut in under 15 minutes without touching the screen.

- **Acceptance Criteria**:

  1.  Triggering cut completion opens CUT-04 with cursor autofocus on Sub-sub Bundle \#1.

  2.  Single RFID tag presentation binds EPC to cursor bundle, validates CRC readability, and auto-advances to bundle \#2 in $< 300ms$.

  3.  Unreadable or damaged tags trigger an error tone, quarantine the tag, and prevent cursor advance.

- **Tasks**:

  - TASK-044-1: Build cursor-driven auto-advance WebSocket endpoint on edge gateway.

  - TASK-044-2: Implement fast-path keyboard/scanner wedge handler in CUT-04 UI.

  - TASK-044-3: Implement 30-second unmap/undo recovery action for mistaken attachments.


**Master correction / qualification:** 720 bundles/15 min and 300ms cursor are candidate performance fixtures; qualify actual scanner and printer.


**Explicit closure tests for E22-S03:** Use the business contract above to construct one successful signed fixture, one policy/identity rejection fixture and one network/restart replay fixture; prove that the actual measured state equals the ledger, not a UI counter.

- **E22-S03-T06 — Schema/transaction review:** reviewer checks exact active IDs, tenant compound keys, expected invariants, migration/rollback, and canonical event ownership.
- **E22-S03-T07 — End-to-end physical failure fixture:** demonstrate forbidden alternative and stale/duplicate/cross-PO event; preserve rejected evidence with zero unauthorized state mutation.
- **E22-S03-T08 — Promotion evidence:** attach UI/device proof, real/fixture source documents, persisted event trail, clock/latency observations and signed acceptance against master V5-T / MGT gates.


#### E22-S04 — Attachment-map compatibility enforcement  [V5 E3]
**Canonical user story:** As the **Sewing Operator**, I need to receive immediate rejection when parts from incompatible garments or shade lots are paired, so that incorrect assemblies earn zero production credit.


**Original V5 templated intent (retained for provenance):** As the accountable factory role, I can attachment-map compatibility enforcement so downstream production, financial and audit decisions remain correct.
**Business contract:** Compile approved OB part type and marker relationships; reject cross-garment/shade attachment credit.
- **E22-S04-T01 — Domain/authority:** Define exact input identities, source-of-truth state, permission/CEO constraints, rejection codes and ownership.
- **E22-S04-T02 — Persistence/API:** Implement transactional schema, idempotency and domain actions for: Compile approved OB part type and marker relationships; reject cross-garment/shade attachment credit.
- **E22-S04-T03 — Floor/manager UX:** Build role-specific happy path, blocked/unknown states, recovery and human-readable evidence presentation.
- **E22-S04-T04 — Invariants & adversarial tests:** Test concurrent requests, duplicate/replayed/offline input, wrong buyer/tenant, invalid revision/quantity and tamper resistance.
- **E22-S04-T05 — Pilot/acceptance:** Capture signed accepted and rejected fixtures, source snapshot, actual event trace, restart recovery and rollback result.
**Done when:** Actual authorized operation survives restart and replay, rejects violating evidence, propagates correct downstream state, and satisfies a recorded pilot acceptance test.

**Added deterministic invariant:** Wrong garment marker position/shade/size and missing predecessor yield zero credited operation.


**Explicit closure tests for E22-S04:** Wrong garment marker position/shade/size and missing predecessor yield zero credited operation.

- **E22-S04-T06 — Schema/transaction review:** reviewer checks exact active IDs, tenant compound keys, expected invariants, migration/rollback, and canonical event ownership.
- **E22-S04-T07 — End-to-end physical failure fixture:** demonstrate forbidden alternative and stale/duplicate/cross-PO event; preserve rejected evidence with zero unauthorized state mutation.
- **E22-S04-T08 — Promotion evidence:** attach UI/device proof, real/fixture source documents, persisted event trail, clock/latency observations and signed acceptance against master V5-T / MGT gates.


#### E22-S05 — Defective panel replacement identity  [V5 D5]
**Canonical user story:** As the **Cutting Quality Checker**, I need to replace a rejected panel from an approved remnant without changing garment identity, so that physical genealogy and WIP accounting remain conserved.


**Original V5 templated intent (retained for provenance):** As the accountable factory role, I can defective panel replacement identity so downstream production, financial and audit decisions remain correct.
**Business contract:** Create remnant-based replacement with original garment serial and links to rejected component.
- **E22-S05-T01 — Domain/authority:** Define exact input identities, source-of-truth state, permission/CEO constraints, rejection codes and ownership.
- **E22-S05-T02 — Persistence/API:** Implement transactional schema, idempotency and domain actions for: Create remnant-based replacement with original garment serial and links to rejected component.
- **E22-S05-T03 — Floor/manager UX:** Build role-specific happy path, blocked/unknown states, recovery and human-readable evidence presentation.
- **E22-S05-T04 — Invariants & adversarial tests:** Test concurrent requests, duplicate/replayed/offline input, wrong buyer/tenant, invalid revision/quantity and tamper resistance.
- **E22-S05-T05 — Pilot/acceptance:** Capture signed accepted and rejected fixtures, source snapshot, actual event trace, restart recovery and rollback result.
**Done when:** Actual authorized operation survives restart and replay, rejects violating evidence, propagates correct downstream state, and satisfies a recorded pilot acceptance test.

**Added deterministic invariant:** New physical replacement panel links old rejected part and same logical garment serial without duplicate WIP.


**Explicit closure tests for E22-S05:** New physical replacement panel links old rejected part and same logical garment serial without duplicate WIP.

- **E22-S05-T06 — Schema/transaction review:** reviewer checks exact active IDs, tenant compound keys, expected invariants, migration/rollback, and canonical event ownership.
- **E22-S05-T07 — End-to-end physical failure fixture:** demonstrate forbidden alternative and stale/duplicate/cross-PO event; preserve rejected evidence with zero unauthorized state mutation.
- **E22-S05-T08 — Promotion evidence:** attach UI/device proof, real/fixture source documents, persisted event trail, clock/latency observations and signed acceptance against master V5-T / MGT gates.


#### E22-S06 — Cut consumption reconciliation release gate  [V5 D6]
**Canonical user story:** As the **Cutting Supervisor**, I need to reconcile actual issued/consumed/returned/scrapped material before bundle release, so that unexplained fabric leakage cannot enter sewing as accepted stock.


**Original V5 templated intent (retained for provenance):** As the accountable factory role, I can cut consumption reconciliation release gate so downstream production, financial and audit decisions remain correct.
**Business contract:** Balance reservations consumption returned remnants scrap, block out-of-tolerance release pending CEO.
- **E22-S06-T01 — Domain/authority:** Define exact input identities, source-of-truth state, permission/CEO constraints, rejection codes and ownership.
- **E22-S06-T02 — Persistence/API:** Implement transactional schema, idempotency and domain actions for: Balance reservations consumption returned remnants scrap, block out-of-tolerance release pending CEO.
- **E22-S06-T03 — Floor/manager UX:** Build role-specific happy path, blocked/unknown states, recovery and human-readable evidence presentation.
- **E22-S06-T04 — Invariants & adversarial tests:** Test concurrent requests, duplicate/replayed/offline input, wrong buyer/tenant, invalid revision/quantity and tamper resistance.
- **E22-S06-T05 — Pilot/acceptance:** Capture signed accepted and rejected fixtures, source snapshot, actual event trace, restart recovery and rollback result.
**Done when:** Actual authorized operation survives restart and replay, rejects violating evidence, propagates correct downstream state, and satisfies a recorded pilot acceptance test.

##### Integrated alternate story ALT-04.5 — Cut Reconciliation Gate Before Bundle Release

**Provenance:** second V5 engineering backlog, Story 04.5. The following original acceptance/tasks are preserved as additional implementation detail, subject to the Master PRD/ARD.


- **As the** Cutting Master,

- **I want** the system to compute fabric balance before releasing cut bundles to sewing,

- **So that** fabric leakages greater than \$\pm 2\\\$ require CEO review.

- **Acceptance Criteria**:

  1.  Compute $NetVariance = Reserved - Consumed - End - bitReturned - Scrap$.

  2.  If \$\|\text{Net Variance}\| \> 2\\\$, bundle release is locked and routes to CEO variance queue.

  3.  Sewing HMI rejects scans of bundles from unreconciled or locked cuts.

- **Tasks**:

  - TASK-045-1: Implement cut reconciliation verification gate in services/edge-daemon/pkg/cutting/.

  - TASK-045-2: Build bundle release lock mechanism in services/core-api.

  - TASK-045-3: Connect CUT-05 pre-sewing balance gate screen.


**Master correction / qualification:** Reconcile ISSUED fabric to disposition; RESERVED is a separate commitment. ±2% is illustrative.


**Explicit closure tests for E22-S06:** Use the business contract above to construct one successful signed fixture, one policy/identity rejection fixture and one network/restart replay fixture; prove that the actual measured state equals the ledger, not a UI counter.

- **E22-S06-T06 — Schema/transaction review:** reviewer checks exact active IDs, tenant compound keys, expected invariants, migration/rollback, and canonical event ownership.
- **E22-S06-T07 — End-to-end physical failure fixture:** demonstrate forbidden alternative and stale/duplicate/cross-PO event; preserve rejected evidence with zero unauthorized state mutation.
- **E22-S06-T08 — Promotion evidence:** attach UI/device proof, real/fixture source documents, persisted event trail, clock/latency observations and signed acceptance against master V5-T / MGT gates.


#### E22-S07 — Cut Master tweak learning measurements  [V5 D3]
**Canonical user story:** As the **Industrial Engineering Lead**, I need to learn from Cutting Master tweaks and measured outcomes, so that the optimizer improves without incentivizing silent acceptance of poor plans.


**Original V5 templated intent (retained for provenance):** As the accountable factory role, I can cut master tweak learning measurements so downstream production, financial and audit decisions remain correct.
**Business contract:** Store recommended plan, precise deltas, override reasons, actual efficiency and non-punitive acceptance analytics.
- **E22-S07-T01 — Domain/authority:** Define exact input identities, source-of-truth state, permission/CEO constraints, rejection codes and ownership.
- **E22-S07-T02 — Persistence/API:** Implement transactional schema, idempotency and domain actions for: Store recommended plan, precise deltas, override reasons, actual efficiency and non-punitive acceptance analytics.
- **E22-S07-T03 — Floor/manager UX:** Build role-specific happy path, blocked/unknown states, recovery and human-readable evidence presentation.
- **E22-S07-T04 — Invariants & adversarial tests:** Test concurrent requests, duplicate/replayed/offline input, wrong buyer/tenant, invalid revision/quantity and tamper resistance.
- **E22-S07-T05 — Pilot/acceptance:** Capture signed accepted and rejected fixtures, source snapshot, actual event trace, restart recovery and rollback result.
**Done when:** Actual authorized operation survives restart and replay, rejects violating evidence, propagates correct downstream state, and satisfies a recorded pilot acceptance test.

### E23 — Operation Bulletin & Earned Credit

**Added deterministic invariant:** Evaluate suggested plan vs human tweak on actual waste/quality/cycle; override rate is not punitive.


**Explicit closure tests for E22-S07:** Evaluate suggested plan vs human tweak on actual waste/quality/cycle; override rate is not punitive.

- **E22-S07-T06 — Schema/transaction review:** reviewer checks exact active IDs, tenant compound keys, expected invariants, migration/rollback, and canonical event ownership.
- **E22-S07-T07 — End-to-end physical failure fixture:** demonstrate forbidden alternative and stale/duplicate/cross-PO event; preserve rejected evidence with zero unauthorized state mutation.
- **E22-S07-T08 — Promotion evidence:** attach UI/device proof, real/fixture source documents, persisted event trail, clock/latency observations and signed acceptance against master V5-T / MGT gates.


### E23 — Operation Bulletin & Earned Credit


#### E23-S01 — Progressive OB document contract  [V5 E1]
**Canonical user story:** As the **Industrial Engineering Head**, I need to author a single progressively enriched OB with stage-specific approvals, so that costing, planning and execution share one versioned route.


**Original V5 templated intent (retained for provenance):** As the accountable factory role, I can progressive ob document contract so downstream production, financial and audit decisions remain correct.
**Business contract:** Model costing/planning/shift-time enrichment, owners, signatures, operation revisions and exact machine assignment.
- **E23-S01-T01 — Domain/authority:** Define exact input identities, source-of-truth state, permission/CEO constraints, rejection codes and ownership.
- **E23-S01-T02 — Persistence/API:** Implement transactional schema, idempotency and domain actions for: Model costing/planning/shift-time enrichment, owners, signatures, operation revisions and exact machine assignment.
- **E23-S01-T03 — Floor/manager UX:** Build role-specific happy path, blocked/unknown states, recovery and human-readable evidence presentation.
- **E23-S01-T04 — Invariants & adversarial tests:** Test concurrent requests, duplicate/replayed/offline input, wrong buyer/tenant, invalid revision/quantity and tamper resistance.
- **E23-S01-T05 — Pilot/acceptance:** Capture signed accepted and rejected fixtures, source snapshot, actual event trace, restart recovery and rollback result.
**Done when:** Actual authorized operation survives restart and replay, rejects violating evidence, propagates correct downstream state, and satisfies a recorded pilot acceptance test.

##### Integrated alternate story ALT-05.1 — Three-Stage Progressively Enriched Operation Bulletin

**Provenance:** second V5 engineering backlog, Story 05.1. The following original acceptance/tasks are preserved as additional implementation detail, subject to the Master PRD/ARD.


- **As an** IE and Factory Management Team,

- **I want** a single OB entity that enriches across Costing, Planning, and Execution stages,

- **So that** operations, machines, SAMs, and operator assignments are locked at their appropriate operational phases.

- **Acceptance Criteria**:

  1.  **Stage 1 (Costing)**: Owned by IE; locks standard Op ID, machine type, Factory SAM with confidence tier, scan flag, bundle flag, and skill. Requires 2-person IE sign-off.

  2.  **Stage 2 (Planning)**: Owned by IE + Production; locks line allocation, target PPH, and workstation count.

  3.  **Stage 3 (Execution)**: Owned by Factory Head; locks operator ID, machine unit ID, and station ID at shift start (08:30).

  4.  Changes post-freeze require explicit reason codes and append to the audit log.

- **Tasks**:

  - TASK-051-1: Create schema migrations for operation_bulletin_header and entries.

  - TASK-051-2: Implement stage-lock state machine in services/core-api.

  - TASK-051-3: Build OB-01 (Costing authoring) and OB-02 (Execution freeze) UI screens.


**Explicit closure tests for E23-S01:** Use the business contract above to construct one successful signed fixture, one policy/identity rejection fixture and one network/restart replay fixture; prove that the actual measured state equals the ledger, not a UI counter.

- **E23-S01-T06 — Schema/transaction review:** reviewer checks exact active IDs, tenant compound keys, expected invariants, migration/rollback, and canonical event ownership.
- **E23-S01-T07 — End-to-end physical failure fixture:** demonstrate forbidden alternative and stale/duplicate/cross-PO event; preserve rejected evidence with zero unauthorized state mutation.
- **E23-S01-T08 — Promotion evidence:** attach UI/device proof, real/fixture source documents, persisted event trail, clock/latency observations and signed acceptance against master V5-T / MGT gates.


#### E23-S02 — Factory SAM evidence estimates  [V5 E2]
**Canonical user story:** As the **Industrial Engineer**, I need to estimate Factory SAM from context-specific measured cycles and transparent uncertainty, so that quotes and line targets do not claim unwarranted precision.


**Original V5 templated intent (retained for provenance):** As the accountable factory role, I can factory sam evidence estimates so downstream production, financial and audit decisions remain correct.
**Business contract:** Separate IE estimates from RFID-observed times, use robust sample uncertainty and context-specific validity.
- **E23-S02-T01 — Domain/authority:** Define exact input identities, source-of-truth state, permission/CEO constraints, rejection codes and ownership.
- **E23-S02-T02 — Persistence/API:** Implement transactional schema, idempotency and domain actions for: Separate IE estimates from RFID-observed times, use robust sample uncertainty and context-specific validity.
- **E23-S02-T03 — Floor/manager UX:** Build role-specific happy path, blocked/unknown states, recovery and human-readable evidence presentation.
- **E23-S02-T04 — Invariants & adversarial tests:** Test concurrent requests, duplicate/replayed/offline input, wrong buyer/tenant, invalid revision/quantity and tamper resistance.
- **E23-S02-T05 — Pilot/acceptance:** Capture signed accepted and rejected fixtures, source snapshot, actual event trace, restart recovery and rollback result.
**Done when:** Actual authorized operation survives restart and replay, rejects violating evidence, propagates correct downstream state, and satisfies a recorded pilot acceptance test.

##### Integrated alternate story ALT-02.1 — Statistical Factory SAM Confidence Engine

**Provenance:** second V5 engineering backlog, Story 02.1. The following original acceptance/tasks are preserved as additional implementation detail, subject to the Master PRD/ARD.


- **As an** Industrial Engineer,

- **I want** the system to categorize standard allowed minutes (SAM) into statistical confidence tiers based on actual floor telemetry,

- **So that** commercial quotes do not rely on theoretical 100% standard efficiency tables.

- **Acceptance Criteria**:

  1.  Confidence tiers computed dynamically:

      - RFID·H: $\geq 500$ runs, coefficient of variation $c_{v} \leq 0.12$.

      - IE·M: $100to499$ runs, $c_{v} \leq 0.20$.

      - RFID·L: $< 100$ runs or high variance.

      - IE·PROV: Manual seed without telemetry.

  2.  Outlier cycles (\$\< 40\\\$ or \$\> 300\\\$ SAM) are censored from confidence calculation.

  3.  Quotations utilizing \$\> 20\\\$ RFID·L or IE·PROV operations display a risk warning banner in CST-01.

- **Tasks**:

  - TASK-021-1: Implement statistical variance calculator over ClickHouse/PostgreSQL cycle time distributions.

  - TASK-021-2: Add SAM confidence pill generator to OB-01 and CST-01 views.

  - TASK-021-3: Write unit tests validating threshold graduation and outlier filtering.


**Master correction / qualification:** Counts and CV thresholds are candidate policies; censoring outliers requires review, not blind exclusion.


**Explicit closure tests for E23-S02:** Use the business contract above to construct one successful signed fixture, one policy/identity rejection fixture and one network/restart replay fixture; prove that the actual measured state equals the ledger, not a UI counter.

- **E23-S02-T06 — Schema/transaction review:** reviewer checks exact active IDs, tenant compound keys, expected invariants, migration/rollback, and canonical event ownership.
- **E23-S02-T07 — End-to-end physical failure fixture:** demonstrate forbidden alternative and stale/duplicate/cross-PO event; preserve rejected evidence with zero unauthorized state mutation.
- **E23-S02-T08 — Promotion evidence:** attach UI/device proof, real/fixture source documents, persisted event trail, clock/latency observations and signed acceptance against master V5-T / MGT gates.


#### E23-S03 — Operation-type production credit ledger  [V5 E3]
**Canonical user story:** As the **Sewing Station Controller**, I need to issue operation credit only for valid typed part-to-garment transformations, so that passive scans, duplicates and wrong attachments never become earned output.


**Original V5 templated intent (retained for provenance):** As the accountable factory role, I can operation-type production credit ledger so downstream production, financial and audit decisions remain correct.
**Business contract:** Enforce S2S/B2B/B2S input output cardinalities, preconditions, dedupe and signed production credit.
- **E23-S03-T01 — Domain/authority:** Define exact input identities, source-of-truth state, permission/CEO constraints, rejection codes and ownership.
- **E23-S03-T02 — Persistence/API:** Implement transactional schema, idempotency and domain actions for: Enforce S2S/B2B/B2S input output cardinalities, preconditions, dedupe and signed production credit.
- **E23-S03-T03 — Floor/manager UX:** Build role-specific happy path, blocked/unknown states, recovery and human-readable evidence presentation.
- **E23-S03-T04 — Invariants & adversarial tests:** Test concurrent requests, duplicate/replayed/offline input, wrong buyer/tenant, invalid revision/quantity and tamper resistance.
- **E23-S03-T05 — Pilot/acceptance:** Capture signed accepted and rejected fixtures, source snapshot, actual event trace, restart recovery and rollback result.
**Done when:** Actual authorized operation survives restart and replay, rejects violating evidence, propagates correct downstream state, and satisfies a recorded pilot acceptance test.

##### Integrated alternate story ALT-05.2 — Typed OperationExecutionContract & Anti-Mistake Credit Engine

**Provenance:** second V5 engineering backlog, Story 05.2. The following original acceptance/tasks are preserved as additional implementation detail, subject to the Master PRD/ARD.


- **As a** Production Lead,

- **I want** sewing operations governed by typed cardinality contracts (Single-to-Single, Bundle-to-Bundle, Bundle-to-Single),

- **So that** production credit is awarded only upon valid, non-duplicated physical transformations.

- **Acceptance Criteria**:

  1.  Bundle-to-Single operation consumes multiple parent component bundles and issues serialized garments without double-crediting parents.

  2.  Credit requires simultaneous validation: authorized worker skill, active OB revision, machine match, DAG prerequisites, and attachment compatibility.

  3.  Workstation scan of invalid sub-bundle attachment denies credit and triggers local audio/visual error.

- **Tasks**:

  - TASK-052-1: Implement OperationExecutionContract validator in services/edge-daemon/pkg/sewing/credit.go.

  - TASK-052-2: Connect attachment.ValidatePair to the transaction credit outbox.

  - TASK-052-3: Update SEW-01 operator tablet to render execution mode and attachment feedback.


**Master correction / qualification:** Dwell thresholds and RFID presence are anomaly signals, not independently sufficient proof of work or earned pay.


**Explicit closure tests for E23-S03:** Use the business contract above to construct one successful signed fixture, one policy/identity rejection fixture and one network/restart replay fixture; prove that the actual measured state equals the ledger, not a UI counter.

- **E23-S03-T06 — Schema/transaction review:** reviewer checks exact active IDs, tenant compound keys, expected invariants, migration/rollback, and canonical event ownership.
- **E23-S03-T07 — End-to-end physical failure fixture:** demonstrate forbidden alternative and stale/duplicate/cross-PO event; preserve rejected evidence with zero unauthorized state mutation.
- **E23-S03-T08 — Promotion evidence:** attach UI/device proof, real/fixture source documents, persisted event trail, clock/latency observations and signed acceptance against master V5-T / MGT gates.


#### E23-S04 — No-blame issue intervention outcomes  [V5 E4]
**Canonical user story:** As the **Line Quality Supervisor**, I need to assign, coordinate and verify corrective action on shop-floor issues, so that resolution means measured recovery rather than merely closing an alert.


**Original V5 templated intent (retained for provenance):** As the accountable factory role, I can no-blame issue intervention outcomes so downstream production, financial and audit decisions remain correct.
**Business contract:** Track joiners, action, verification and recurrence for defect, breakdown and downtime interventions.
- **E23-S04-T01 — Domain/authority:** Define exact input identities, source-of-truth state, permission/CEO constraints, rejection codes and ownership.
- **E23-S04-T02 — Persistence/API:** Implement transactional schema, idempotency and domain actions for: Track joiners, action, verification and recurrence for defect, breakdown and downtime interventions.
- **E23-S04-T03 — Floor/manager UX:** Build role-specific happy path, blocked/unknown states, recovery and human-readable evidence presentation.
- **E23-S04-T04 — Invariants & adversarial tests:** Test concurrent requests, duplicate/replayed/offline input, wrong buyer/tenant, invalid revision/quantity and tamper resistance.
- **E23-S04-T05 — Pilot/acceptance:** Capture signed accepted and rejected fixtures, source snapshot, actual event trace, restart recovery and rollback result.
**Done when:** Actual authorized operation survives restart and replay, rejects violating evidence, propagates correct downstream state, and satisfies a recorded pilot acceptance test.

### E24 — Finishing, Packing & OCR

**Added deterministic invariant:** Issue cannot be resolved solely by click; verify recovered metric, recurrence and owner/action.


**Explicit closure tests for E23-S04:** Issue cannot be resolved solely by click; verify recovered metric, recurrence and owner/action.

- **E23-S04-T06 — Schema/transaction review:** reviewer checks exact active IDs, tenant compound keys, expected invariants, migration/rollback, and canonical event ownership.
- **E23-S04-T07 — End-to-end physical failure fixture:** demonstrate forbidden alternative and stale/duplicate/cross-PO event; preserve rejected evidence with zero unauthorized state mutation.
- **E23-S04-T08 — Promotion evidence:** attach UI/device proof, real/fixture source documents, persisted event trail, clock/latency observations and signed acceptance against master V5-T / MGT gates.


### E24 — Finishing, Packing & OCR


#### E24-S01 — Hangtag identity transition with tag return  [V5 F1]
**Canonical user story:** As the **Hangtag Helper**, I need to atomically bind an internal garment to a buyer-serialized hangtag before retiring the internal RFID, so that all downstream identity remains exact and reusable tags return safely.


**Original V5 templated intent (retained for provenance):** As the accountable factory role, I can hangtag identity transition with tag return so downstream production, financial and audit decisions remain correct.
**Business contract:** Atomically bind internal garment to unique buyer serial, retire carrier, quarantine reader error.
- **E24-S01-T01 — Domain/authority:** Define exact input identities, source-of-truth state, permission/CEO constraints, rejection codes and ownership.
- **E24-S01-T02 — Persistence/API:** Implement transactional schema, idempotency and domain actions for: Atomically bind internal garment to unique buyer serial, retire carrier, quarantine reader error.
- **E24-S01-T03 — Floor/manager UX:** Build role-specific happy path, blocked/unknown states, recovery and human-readable evidence presentation.
- **E24-S01-T04 — Invariants & adversarial tests:** Test concurrent requests, duplicate/replayed/offline input, wrong buyer/tenant, invalid revision/quantity and tamper resistance.
- **E24-S01-T05 — Pilot/acceptance:** Capture signed accepted and rejected fixtures, source snapshot, actual event trace, restart recovery and rollback result.
**Done when:** Actual authorized operation survives restart and replay, rejects violating evidence, propagates correct downstream state, and satisfies a recorded pilot acceptance test.

##### Integrated alternate story ALT-06.1 — Atomic Finishing Hangtag Transition (Hangtag Swap)

**Provenance:** second V5 engineering backlog, Story 06.1. The following original acceptance/tasks are preserved as additional implementation detail, subject to the Master PRD/ARD.


- **As a** Hangtag Helper,

- **I want** to scan the internal RFID tag and buyer hangtag barcode simultaneously,

- **So that** garment genealogy permanently binds to customer identity and the RFID tag returns to the pool.

- **Acceptance Criteria**:

  1.  Scan of internal RFID tag + buyer hangtag barcode generates an atomic HangtagBind record.

  2.  Internal RFID tag is disassociated, set to STRIPPED_UNBOUND, and unlocked for re-pooling.

  3.  Downstream operations (ironing, final check, packing) identify garments exclusively via buyer hangtag UPC/barcode.

- **Tasks**:

  - TASK-061-1: Create hangtag_piece_binding table with unique constraint on buyer barcode.

  - TASK-061-2: Implement atomic transition service in services/core-api.

  - TASK-061-3: Build FH-01 Hangtag Helper mobile scanner interface.


**Master correction / qualification:** Buyer UPC denotes SKU unless truly serialized; tag goes to inspection before reuse.


**Explicit closure tests for E24-S01:** Use the business contract above to construct one successful signed fixture, one policy/identity rejection fixture and one network/restart replay fixture; prove that the actual measured state equals the ledger, not a UI counter.

- **E24-S01-T06 — Schema/transaction review:** reviewer checks exact active IDs, tenant compound keys, expected invariants, migration/rollback, and canonical event ownership.
- **E24-S01-T07 — End-to-end physical failure fixture:** demonstrate forbidden alternative and stale/duplicate/cross-PO event; preserve rejected evidence with zero unauthorized state mutation.
- **E24-S01-T08 — Promotion evidence:** attach UI/device proof, real/fixture source documents, persisted event trail, clock/latency observations and signed acceptance against master V5-T / MGT gates.


#### E24-S02 — Type-driven finishing redo and rescue rate  [V5 F2]
**Canonical user story:** As the **Final Quality Checker**, I need to route finishing and sewing defects to the proper repair station and reinspect, so that repair rescue and first-pass yield have correct denominators.


**Original V5 templated intent (retained for provenance):** As the accountable factory role, I can type-driven finishing redo and rescue rate so downstream production, financial and audit decisions remain correct.
**Business contract:** Route hangtag/ironing/sewing redo, independent reinspection, denominator-correct rescue rate.
- **E24-S02-T01 — Domain/authority:** Define exact input identities, source-of-truth state, permission/CEO constraints, rejection codes and ownership.
- **E24-S02-T02 — Persistence/API:** Implement transactional schema, idempotency and domain actions for: Route hangtag/ironing/sewing redo, independent reinspection, denominator-correct rescue rate.
- **E24-S02-T03 — Floor/manager UX:** Build role-specific happy path, blocked/unknown states, recovery and human-readable evidence presentation.
- **E24-S02-T04 — Invariants & adversarial tests:** Test concurrent requests, duplicate/replayed/offline input, wrong buyer/tenant, invalid revision/quantity and tamper resistance.
- **E24-S02-T05 — Pilot/acceptance:** Capture signed accepted and rejected fixtures, source snapshot, actual event trace, restart recovery and rollback result.
**Done when:** Actual authorized operation survives restart and replay, rejects violating evidence, propagates correct downstream state, and satisfies a recorded pilot acceptance test.

##### Integrated alternate story ALT-06.2 — Defect-Specific 3-Way Redo Routing & Rescue Rate

**Provenance:** second V5 engineering backlog, Story 06.2. The following original acceptance/tasks are preserved as additional implementation detail, subject to the Master PRD/ARD.


- **As a** Final Quality Checker,

- **I want** garment redo rejections routed directly to responsible workstations by failure type,

- **So that** defects are repaired at the source and rescue rates are measured.

- **Acceptance Criteria**:

  1.  Route redo by type: Hangtag flaw $\rightarrow$ FH-01 Hangtag Helper; Ironing defect $\rightarrow$ FH-02 Ironer; Sewing defect $\rightarrow$ FH-04 Alteration Operator.

  2.  Alteration screen (FH-04) presents original sewing line, operation, and operator context.

  3.  Track Rescue Rate metric: $\frac{PassedRepairedPieces}{TotalAlterations} \times 100$.

- **Tasks**:

  - TASK-062-1: Implement redo dispatch state machine in pkg/quality/.

  - TASK-062-2: Build FH-03 Final Checker PASS/REDO tap interface.

  - TASK-062-3: Build FH-04 Alteration Operator repair terminal with original operator context.


**Explicit closure tests for E24-S02:** Use the business contract above to construct one successful signed fixture, one policy/identity rejection fixture and one network/restart replay fixture; prove that the actual measured state equals the ledger, not a UI counter.

- **E24-S02-T06 — Schema/transaction review:** reviewer checks exact active IDs, tenant compound keys, expected invariants, migration/rollback, and canonical event ownership.
- **E24-S02-T07 — End-to-end physical failure fixture:** demonstrate forbidden alternative and stale/duplicate/cross-PO event; preserve rejected evidence with zero unauthorized state mutation.
- **E24-S02-T08 — Promotion evidence:** attach UI/device proof, real/fixture source documents, persisted event trail, clock/latency observations and signed acceptance against master V5-T / MGT gates.


#### E24-S03 — Misscan negative-state contract  [V5 F3]
**Canonical user story:** As the **Carton Packer**, I need to reject a wrong garment scan locally without changing carton contents, so that packing errors cannot quietly corrupt buyer carton membership.


**Original V5 templated intent (retained for provenance):** As the accountable factory role, I can misscan negative-state contract so downstream production, financial and audit decisions remain correct.
**Business contract:** Invalid carton scan emits warning/audit while preserving member set, seal eligibility and current counts.
- **E24-S03-T01 — Domain/authority:** Define exact input identities, source-of-truth state, permission/CEO constraints, rejection codes and ownership.
- **E24-S03-T02 — Persistence/API:** Implement transactional schema, idempotency and domain actions for: Invalid carton scan emits warning/audit while preserving member set, seal eligibility and current counts.
- **E24-S03-T03 — Floor/manager UX:** Build role-specific happy path, blocked/unknown states, recovery and human-readable evidence presentation.
- **E24-S03-T04 — Invariants & adversarial tests:** Test concurrent requests, duplicate/replayed/offline input, wrong buyer/tenant, invalid revision/quantity and tamper resistance.
- **E24-S03-T05 — Pilot/acceptance:** Capture signed accepted and rejected fixtures, source snapshot, actual event trace, restart recovery and rollback result.
**Done when:** Actual authorized operation survives restart and replay, rejects violating evidence, propagates correct downstream state, and satisfies a recorded pilot acceptance test.

##### Integrated alternate story ALT-06.3 — Packing Misscan Contract & Audited Repack State Machine

**Provenance:** second V5 engineering backlog, Story 06.3. The following original acceptance/tasks are preserved as additional implementation detail, subject to the Master PRD/ARD.


- **As a** Carton Packer,

- **I want** incorrect garment scans rejected instantly without corrupting the carton slot state,

- **And when** an audited box is unsealed, require complete item re-verification.

- **Acceptance Criteria**:

  1.  Local edge terminal flags invalid garment UPC in $\leq 200ms$ with an amber warning banner, leaving carton contents untouched.

  2.  Misscans increment the operator coaching counter; exceeding 5 misscans/hr alerts supervisor (PK-05).

  3.  Unsealing a carton for AQL inspection transitions box to REPACK_MODE; re-sealing requires re-scanning every piece to verify original hangtag serials.

- **Tasks**:

  - TASK-063-1: Implement split-horizon local validation in services/edge-daemon/pkg/packing/carton_packer.go.

  - TASK-063-2: Add REPACK_MODE lifecycle transition to packing engine.

  - TASK-063-3: Update PK-02 tablet UI with sub-200ms audio/visual feedback.


**Master correction / qualification:** 200ms local UI and 5 misscans/hour coaching are proposed nonpunitive, device-qualified settings; repack is a separate canonical story E24-S04.


**Explicit closure tests for E24-S03:** Use the business contract above to construct one successful signed fixture, one policy/identity rejection fixture and one network/restart replay fixture; prove that the actual measured state equals the ledger, not a UI counter.

- **E24-S03-T06 — Schema/transaction review:** reviewer checks exact active IDs, tenant compound keys, expected invariants, migration/rollback, and canonical event ownership.
- **E24-S03-T07 — End-to-end physical failure fixture:** demonstrate forbidden alternative and stale/duplicate/cross-PO event; preserve rejected evidence with zero unauthorized state mutation.
- **E24-S03-T08 — Promotion evidence:** attach UI/device proof, real/fixture source documents, persisted event trail, clock/latency observations and signed acceptance against master V5-T / MGT gates.


#### E24-S04 — Audited unseal and repack mode  [V5 F3]
**Canonical user story:** As the **AQL Repack Operator**, I need to reconcile every original carton member and authorized substitution before resealing, so that inspected cartons remain traceable and dispatch-safe.


**Original V5 templated intent (retained for provenance):** As the accountable factory role, I can audited unseal and repack mode so downstream production, financial and audit decisions remain correct.
**Business contract:** Preserve original membership baseline; require exact recheck or authorized replacement before resealing.
- **E24-S04-T01 — Domain/authority:** Define exact input identities, source-of-truth state, permission/CEO constraints, rejection codes and ownership.
- **E24-S04-T02 — Persistence/API:** Implement transactional schema, idempotency and domain actions for: Preserve original membership baseline; require exact recheck or authorized replacement before resealing.
- **E24-S04-T03 — Floor/manager UX:** Build role-specific happy path, blocked/unknown states, recovery and human-readable evidence presentation.
- **E24-S04-T04 — Invariants & adversarial tests:** Test concurrent requests, duplicate/replayed/offline input, wrong buyer/tenant, invalid revision/quantity and tamper resistance.
- **E24-S04-T05 — Pilot/acceptance:** Capture signed accepted and rejected fixtures, source snapshot, actual event trace, restart recovery and rollback result.
**Done when:** Actual authorized operation survives restart and replay, rejects violating evidence, propagates correct downstream state, and satisfies a recorded pilot acceptance test.

**Added deterministic invariant:** Snapshot carton members at unseal; exact unique serial accounting or approved replacement before reseal.


**Explicit closure tests for E24-S04:** Snapshot carton members at unseal; exact unique serial accounting or approved replacement before reseal.

- **E24-S04-T06 — Schema/transaction review:** reviewer checks exact active IDs, tenant compound keys, expected invariants, migration/rollback, and canonical event ownership.
- **E24-S04-T07 — End-to-end physical failure fixture:** demonstrate forbidden alternative and stale/duplicate/cross-PO event; preserve rejected evidence with zero unauthorized state mutation.
- **E24-S04-T08 — Promotion evidence:** attach UI/device proof, real/fixture source documents, persisted event trail, clock/latency observations and signed acceptance against master V5-T / MGT gates.


#### E24-S05 — Manifest decremental dock confirmation  [V5 F4]
**Canonical user story:** As the **Dock Loader**, I need to load only unique accepted cartons from the approved manifest and count remaining correctly, so that the truck cannot be released with missing or foreign cartons.


**Original V5 templated intent (retained for provenance):** As the accountable factory role, I can manifest decremental dock confirmation so downstream production, financial and audit decisions remain correct.
**Business contract:** Enforce expected carton set, unique arrival scans, mismatch resolution, handoff authorization.
- **E24-S05-T01 — Domain/authority:** Define exact input identities, source-of-truth state, permission/CEO constraints, rejection codes and ownership.
- **E24-S05-T02 — Persistence/API:** Implement transactional schema, idempotency and domain actions for: Enforce expected carton set, unique arrival scans, mismatch resolution, handoff authorization.
- **E24-S05-T03 — Floor/manager UX:** Build role-specific happy path, blocked/unknown states, recovery and human-readable evidence presentation.
- **E24-S05-T04 — Invariants & adversarial tests:** Test concurrent requests, duplicate/replayed/offline input, wrong buyer/tenant, invalid revision/quantity and tamper resistance.
- **E24-S05-T05 — Pilot/acceptance:** Capture signed accepted and rejected fixtures, source snapshot, actual event trace, restart recovery and rollback result.
**Done when:** Actual authorized operation survives restart and replay, rejects violating evidence, propagates correct downstream state, and satisfies a recorded pilot acceptance test.

##### Integrated alternate story ALT-06.4 — Decremental Manifest Dock Loading

**Provenance:** second V5 engineering backlog, Story 06.4. The following original acceptance/tasks are preserved as additional implementation detail, subject to the Master PRD/ARD.


- **As a** Shipping Dock Loader,

- **I want** loading progress tracked via a live decremental manifest counter,

- **So that** loading an unauthorized carton is physically rejected.

- **Acceptance Criteria**:

  1.  Loader scans ShipManifest; counter initializes with target carton count (e.g. 800 Remaining).

  2.  Valid carton scans decrement the counter by 1.

  3.  Scanning duplicate cartons or boxes from a different PO manifest triggers a red lockout alert.

  4.  Sealing the manifest is disabled until remaining carton count reaches zero.

- **Tasks**:

  - TASK-064-1: Build ShipManifest loading aggregate in services/core-api.

  - TASK-064-2: Implement PK-04 decremental loader handheld screen.


**Explicit closure tests for E24-S05:** Use the business contract above to construct one successful signed fixture, one policy/identity rejection fixture and one network/restart replay fixture; prove that the actual measured state equals the ledger, not a UI counter.

- **E24-S05-T06 — Schema/transaction review:** reviewer checks exact active IDs, tenant compound keys, expected invariants, migration/rollback, and canonical event ownership.
- **E24-S05-T07 — End-to-end physical failure fixture:** demonstrate forbidden alternative and stale/duplicate/cross-PO event; preserve rejected evidence with zero unauthorized state mutation.
- **E24-S05-T08 — Promotion evidence:** attach UI/device proof, real/fixture source documents, persisted event trail, clock/latency observations and signed acceptance against master V5-T / MGT gates.


#### E24-S06 — Immutable six-party OCR proof  [V5 F5]
**Canonical user story:** As the **Order Closure Owner**, I need to obtain six attestations over one frozen reconciled evidence snapshot, so that the factory cannot close an unexplained or mutable order.


**Original V5 templated intent (retained for provenance):** As the accountable factory role, I can immutable six-party ocr proof so downstream production, financial and audit decisions remain correct.
**Business contract:** Freeze stage balances, evidence hash, correction addenda and binding six signoffs before certificate.
- **E24-S06-T01 — Domain/authority:** Define exact input identities, source-of-truth state, permission/CEO constraints, rejection codes and ownership.
- **E24-S06-T02 — Persistence/API:** Implement transactional schema, idempotency and domain actions for: Freeze stage balances, evidence hash, correction addenda and binding six signoffs before certificate.
- **E24-S06-T03 — Floor/manager UX:** Build role-specific happy path, blocked/unknown states, recovery and human-readable evidence presentation.
- **E24-S06-T04 — Invariants & adversarial tests:** Test concurrent requests, duplicate/replayed/offline input, wrong buyer/tenant, invalid revision/quantity and tamper resistance.
- **E24-S06-T05 — Pilot/acceptance:** Capture signed accepted and rejected fixtures, source snapshot, actual event trace, restart recovery and rollback result.
**Done when:** Actual authorized operation survives restart and replay, rejects violating evidence, propagates correct downstream state, and satisfies a recorded pilot acceptance test.

### E25 — Platform Adoption & Productization

##### Integrated alternate story ALT-06.5 — Cryptographic Order Closure Report (OCRCertificate)

**Provenance:** second V5 engineering backlog, Story 06.5. The following original acceptance/tasks are preserved as additional implementation detail, subject to the Master PRD/ARD.


- **As the** Order Closure Auditor,

- **I want** the OCR generated as a cryptographic proof across frozen stage balances,

- **So that** no order can close without satisfying conservation and obtaining 6-department attestations.

- **Acceptance Criteria**:

  1.  Strictly enforce conservation equation: $Cut = Shipped + Lost + Leftover$.

  2.  Non-zero variance requires mandatory, written department comments.

  3.  Freeze data snapshot and generate OCRCertificate signed by 6 department heads (Cutting, Sewing, Finishing, Packing, Quality, Plant Manager).

  4.  Post-close modifications can only be issued as append-only numbered addenda.

- **Tasks**:

  - TASK-065-1: Implement OCRCertificateGenerator producing tamper-evident SHA-256 hashes and digital signatures.

  - TASK-065-2: Build OC-01 multi-department sign-off desk and OC-02 printable certificate view.


**Master correction / qualification:** Cut=Ship+Lost+Leftover is valid only after typed stage/unit conservation; six signatures are bound to frozen hash.


**Explicit closure tests for E24-S06:** Use the business contract above to construct one successful signed fixture, one policy/identity rejection fixture and one network/restart replay fixture; prove that the actual measured state equals the ledger, not a UI counter.

- **E24-S06-T06 — Schema/transaction review:** reviewer checks exact active IDs, tenant compound keys, expected invariants, migration/rollback, and canonical event ownership.
- **E24-S06-T07 — End-to-end physical failure fixture:** demonstrate forbidden alternative and stale/duplicate/cross-PO event; preserve rejected evidence with zero unauthorized state mutation.
- **E24-S06-T08 — Promotion evidence:** attach UI/device proof, real/fixture source documents, persisted event trail, clock/latency observations and signed acceptance against master V5-T / MGT gates.


### E25 — Platform Adoption & Productization


#### E25-S01 — RFID qualification gate for production  [V5 G1]
**Canonical user story:** As the **Plant Integration Engineer**, I need to qualify mandatory RFID coverage before delegating production authority, so that brownfield onboarding stays nonintrusive without offering RFID-free production credit.


**Original V5 templated intent (retained for provenance):** As the accountable factory role, I can rfid qualification gate for production so downstream production, financial and audit decisions remain correct.
**Business contract:** Station zone mapping, tag encoding profile, validation evidence and M0/M1 legacy shadow exceptions.
- **E25-S01-T01 — Domain/authority:** Define exact input identities, source-of-truth state, permission/CEO constraints, rejection codes and ownership.
- **E25-S01-T02 — Persistence/API:** Implement transactional schema, idempotency and domain actions for: Station zone mapping, tag encoding profile, validation evidence and M0/M1 legacy shadow exceptions.
- **E25-S01-T03 — Floor/manager UX:** Build role-specific happy path, blocked/unknown states, recovery and human-readable evidence presentation.
- **E25-S01-T04 — Invariants & adversarial tests:** Test concurrent requests, duplicate/replayed/offline input, wrong buyer/tenant, invalid revision/quantity and tamper resistance.
- **E25-S01-T05 — Pilot/acceptance:** Capture signed accepted and rejected fixtures, source snapshot, actual event trace, restart recovery and rollback result.
**Done when:** Actual authorized operation survives restart and replay, rejects violating evidence, propagates correct downstream state, and satisfies a recorded pilot acceptance test.

##### Integrated alternate story ALT-07.3 — Brownfield Authority Gate Enforcement

**Provenance:** second V5 engineering backlog, Story 07.3. The following original acceptance/tasks are preserved as additional implementation detail, subject to the Master PRD/ARD.


- **As an** Integration Engineer,

- **I want** the system to enforce progressive authority leases (M0 Shadow to M4 System of Record),

- **So that** Agiparel coexists safely with legacy ERPs without duplicate transaction authority.

- **Acceptance Criteria**:

  1.  Enforce CanCommit checks on all domain write transactions: reject writes if scope is in OBSERVE or ADVISE mode.

  2.  Leased write authority requires signed manager approval and explicitly bounded object IDs.

  3.  Conflicting legacy writes fail closed and route to human reconciliation queues.

- **Tasks**:

  - TASK-073-1: Intercept all state-modifying HTTP/NATS commands with brownfield.CanCommit validation.

  - TASK-073-2: Implement POST /v1/adoption/scopes/{id}/transition authority leasing workflow.

  - TASK-073-3: Add integration tests verifying default-deny write behavior across M0–M4 states.


**Master correction / qualification:** Brownfield M0/M1 may observe, but M2/M3 authoritative RFID-dependent actions require commissioned coverage.


**Explicit closure tests for E25-S01:** Use the business contract above to construct one successful signed fixture, one policy/identity rejection fixture and one network/restart replay fixture; prove that the actual measured state equals the ledger, not a UI counter.

- **E25-S01-T06 — Schema/transaction review:** reviewer checks exact active IDs, tenant compound keys, expected invariants, migration/rollback, and canonical event ownership.
- **E25-S01-T07 — End-to-end physical failure fixture:** demonstrate forbidden alternative and stale/duplicate/cross-PO event; preserve rejected evidence with zero unauthorized state mutation.
- **E25-S01-T08 — Promotion evidence:** attach UI/device proof, real/fixture source documents, persisted event trail, clock/latency observations and signed acceptance against master V5-T / MGT gates.


#### E25-S02 — Tag rotation and loss economics  [V5 G2]
**Canonical user story:** As the **Plant IT and Stores Heads**, I need to manage the circulating RFID asset pool, health, loss and replenishment, so that tag shortages and unsafe reuse do not stop the factory.


**Original V5 templated intent (retained for provenance):** As the accountable factory role, I can tag rotation and loss economics so downstream production, financial and audit decisions remain correct.
**Business contract:** Tag inventory life cycles, circulation delay, reader failures, pool sizing and attrition alerts.
- **E25-S02-T01 — Domain/authority:** Define exact input identities, source-of-truth state, permission/CEO constraints, rejection codes and ownership.
- **E25-S02-T02 — Persistence/API:** Implement transactional schema, idempotency and domain actions for: Tag inventory life cycles, circulation delay, reader failures, pool sizing and attrition alerts.
- **E25-S02-T03 — Floor/manager UX:** Build role-specific happy path, blocked/unknown states, recovery and human-readable evidence presentation.
- **E25-S02-T04 — Invariants & adversarial tests:** Test concurrent requests, duplicate/replayed/offline input, wrong buyer/tenant, invalid revision/quantity and tamper resistance.
- **E25-S02-T05 — Pilot/acceptance:** Capture signed accepted and rejected fixtures, source snapshot, actual event trace, restart recovery and rollback result.
**Done when:** Actual authorized operation survives restart and replay, rejects violating evidence, propagates correct downstream state, and satisfies a recorded pilot acceptance test.

##### Integrated alternate story ALT-07.1 — Circulating RFID Carrier Pool Lifecycle & Attrition Monitor

**Provenance:** second V5 engineering backlog, Story 07.1. The following original acceptance/tasks are preserved as additional implementation detail, subject to the Master PRD/ARD.


- **As a** Plant IT Engineer,

- **I want** to track reusable UHF bundle tags through an 8-stage circulation lifecycle,

- **So that** tag loss is detected early and carrier inventory is properly sized.

- **Acceptance Criteria**:

  1.  Size carrier pool dynamically: $N_{pool} = \lambda \cdot T_{return} + N_{safety}$.

  2.  Implement 8 lifecycle states: AVAILABLE $\rightarrow$ MAPPED $\rightarrow$ IN_TRANSIT $\rightarrow$ IN_SEWING $\rightarrow$ ASSEMBLY_PASS $\rightarrow$ STRIPPED $\rightarrow$ CLEAN_REENCODE $\rightarrow$ RETURNED.

  3.  Automatic loss alert fires if cycle attrition exceeds \$2.0\\\$ over a rolling 7-day period.

- **Tasks**:

  - TASK-071-1: Create schema and state machine for rfid_carrier_asset.

  - TASK-071-2: Build pool inventory calculator and attrition monitor service.

  - TASK-071-3: Build tag commissioning and health verification screen in ADM-02.


**Master correction / qualification:** Loss >2%, rotation volumes and turnaround days are capacity hypotheses until live pilot data.


**Explicit closure tests for E25-S02:** Use the business contract above to construct one successful signed fixture, one policy/identity rejection fixture and one network/restart replay fixture; prove that the actual measured state equals the ledger, not a UI counter.

- **E25-S02-T06 — Schema/transaction review:** reviewer checks exact active IDs, tenant compound keys, expected invariants, migration/rollback, and canonical event ownership.
- **E25-S02-T07 — End-to-end physical failure fixture:** demonstrate forbidden alternative and stale/duplicate/cross-PO event; preserve rejected evidence with zero unauthorized state mutation.
- **E25-S02-T08 — Promotion evidence:** attach UI/device proof, real/fixture source documents, persisted event trail, clock/latency observations and signed acceptance against master V5-T / MGT gates.


#### E25-S03 — Multi-vendor CAD interchange  [V5 G3]
**Canonical user story:** As the **CAD Integration Engineer**, I need to validate marker metadata across multiple licensed CAD adapters, so that Agiparel optimizes material allocation without inventing cutter geometry.


**Original V5 templated intent (retained for provenance):** As the accountable factory role, I can multi-vendor cad interchange so downstream production, financial and audit decisions remain correct.
**Business contract:** Versioned marker candidate adapter interface with DXF and tested Gerber/Optitex/Lectra profiles.
- **E25-S03-T01 — Domain/authority:** Define exact input identities, source-of-truth state, permission/CEO constraints, rejection codes and ownership.
- **E25-S03-T02 — Persistence/API:** Implement transactional schema, idempotency and domain actions for: Versioned marker candidate adapter interface with DXF and tested Gerber/Optitex/Lectra profiles.
- **E25-S03-T03 — Floor/manager UX:** Build role-specific happy path, blocked/unknown states, recovery and human-readable evidence presentation.
- **E25-S03-T04 — Invariants & adversarial tests:** Test concurrent requests, duplicate/replayed/offline input, wrong buyer/tenant, invalid revision/quantity and tamper resistance.
- **E25-S03-T05 — Pilot/acceptance:** Capture signed accepted and rejected fixtures, source snapshot, actual event trace, restart recovery and rollback result.
**Done when:** Actual authorized operation survives restart and replay, rejects violating evidence, propagates correct downstream state, and satisfies a recorded pilot acceptance test.

##### Integrated alternate story ALT-04.1 — Multi-CAD Tiered Ingestion Adapters

**Provenance:** second V5 engineering backlog, Story 04.1. The following original acceptance/tasks are preserved as additional implementation detail, subject to the Master PRD/ARD.


- **As a** Cutting Supervisor,

- **I want** Agiparel to ingest marker files directly from Gerber, Lectra, Optitex, and universal DXF,

- **So that** fabric allocation optimizes roll assignments without altering pattern geometries.

- **Acceptance Criteria**:

  1.  Support parsing ASTM/AAMA DXF (.dxf), Gerber AccuMark (.ord, .zip), Lectra (.iba, .vet), and Optitex (.pds, .plx).

  2.  Extract marker metadata: width, length, garment count, ratio, and pattern piece areas.

  3.  Mark ingested markers as immutable records locked to external CAD versioning.

- **Tasks**:

  - TASK-041-1: Implement binary and text parsers for DXF and native CAD files in pkg/cad/.

  - TASK-041-2: Build CAD file upload endpoint POST /v1/cad/markers.

  - TASK-041-3: Wire marker metadata ingestion into CUT-01 cut planner.


**Master correction / qualification:** Native CAD extension support requires vendor files, licenses and reference differential tests; never invent working binary parsers.


**Explicit closure tests for E25-S03:** Use the business contract above to construct one successful signed fixture, one policy/identity rejection fixture and one network/restart replay fixture; prove that the actual measured state equals the ledger, not a UI counter.

- **E25-S03-T06 — Schema/transaction review:** reviewer checks exact active IDs, tenant compound keys, expected invariants, migration/rollback, and canonical event ownership.
- **E25-S03-T07 — End-to-end physical failure fixture:** demonstrate forbidden alternative and stale/duplicate/cross-PO event; preserve rejected evidence with zero unauthorized state mutation.
- **E25-S03-T08 — Promotion evidence:** attach UI/device proof, real/fixture source documents, persisted event trail, clock/latency observations and signed acceptance against master V5-T / MGT gates.


#### E25-S04 — Native-script device and printing support  [V5 G4]
**Canonical user story:** As the **Operator Experience Lead**, I need to deliver reviewed native-script, RTL and printed station workflows, so that frontline workers can operate reliably on actual low-end devices.


**Original V5 templated intent (retained for provenance):** As the accountable factory role, I can native-script device and printing support so downstream production, financial and audit decisions remain correct.
**Business contract:** Translation glossary, RTL, barcode label print fonts, Android floor device testing and operator comprehension.
- **E25-S04-T01 — Domain/authority:** Define exact input identities, source-of-truth state, permission/CEO constraints, rejection codes and ownership.
- **E25-S04-T02 — Persistence/API:** Implement transactional schema, idempotency and domain actions for: Translation glossary, RTL, barcode label print fonts, Android floor device testing and operator comprehension.
- **E25-S04-T03 — Floor/manager UX:** Build role-specific happy path, blocked/unknown states, recovery and human-readable evidence presentation.
- **E25-S04-T04 — Invariants & adversarial tests:** Test concurrent requests, duplicate/replayed/offline input, wrong buyer/tenant, invalid revision/quantity and tamper resistance.
- **E25-S04-T05 — Pilot/acceptance:** Capture signed accepted and rejected fixtures, source snapshot, actual event trace, restart recovery and rollback result.
**Done when:** Actual authorized operation survives restart and replay, rejects violating evidence, propagates correct downstream state, and satisfies a recorded pilot acceptance test.

##### Integrated alternate story ALT-07.2 — Native-Script Floor Localization Engine

**Provenance:** second V5 engineering backlog, Story 07.2. The following original acceptance/tasks are preserved as additional implementation detail, subject to the Master PRD/ARD.


- **As a** Sewing Machine Operator,

- **I want** my workstation terminal rendered in my native script at a Grade-5 reading level,

- **So that** I can complete production actions without language or reading barriers.

- **Acceptance Criteria**:

  1.  Support Day-1 locales: English, Hindi (Devanagari), Bengali, Tamil, Telugu, Kannada, Gujarati, Marathi, Urdu (RTL layout), Vietnamese, Indonesian, and Spanish.

  2.  Layouts support large touch targets ($\geq 48px$) and zero regular keyboard typing for floor operators.

  3.  Translations validated by in-factory operators; fallback to plain language icons when untranslated.

- **Tasks**:

  - TASK-072-1: Implement i18n localization dictionary service with RTL support in edge UI.

  - TASK-072-2: Create native-script translation bundles for operator workflows (SEW-01, CUT-04, FH-01, PK-02).

  - TASK-072-3: Test font rendering and layout integrity on low-end Android floor tablets.


**Explicit closure tests for E25-S04:** Use the business contract above to construct one successful signed fixture, one policy/identity rejection fixture and one network/restart replay fixture; prove that the actual measured state equals the ledger, not a UI counter.

- **E25-S04-T06 — Schema/transaction review:** reviewer checks exact active IDs, tenant compound keys, expected invariants, migration/rollback, and canonical event ownership.
- **E25-S04-T07 — End-to-end physical failure fixture:** demonstrate forbidden alternative and stale/duplicate/cross-PO event; preserve rejected evidence with zero unauthorized state mutation.
- **E25-S04-T08 — Promotion evidence:** attach UI/device proof, real/fixture source documents, persisted event trail, clock/latency observations and signed acceptance against master V5-T / MGT gates.


#### E25-S05 — Requirement source and maturity matrix  [V5 G5]
**Canonical user story:** As the **Plant Release Manager**, I need to track source, maturity, negative tests and signed physical evidence for each feature, so that DESIGNED screens are never advertised as plant-validated behavior.


**Original V5 templated intent (retained for provenance):** As the accountable factory role, I can requirement source and maturity matrix so downstream production, financial and audit decisions remain correct.
**Business contract:** Tag requirements F/S/V4/D and VALIDATED/DESIGNED/PROPOSED/FUTURE with signed evidence.
- **E25-S05-T01 — Domain/authority:** Define exact input identities, source-of-truth state, permission/CEO constraints, rejection codes and ownership.
- **E25-S05-T02 — Persistence/API:** Implement transactional schema, idempotency and domain actions for: Tag requirements F/S/V4/D and VALIDATED/DESIGNED/PROPOSED/FUTURE with signed evidence.
- **E25-S05-T03 — Floor/manager UX:** Build role-specific happy path, blocked/unknown states, recovery and human-readable evidence presentation.
- **E25-S05-T04 — Invariants & adversarial tests:** Test concurrent requests, duplicate/replayed/offline input, wrong buyer/tenant, invalid revision/quantity and tamper resistance.
- **E25-S05-T05 — Pilot/acceptance:** Capture signed accepted and rejected fixtures, source snapshot, actual event trace, restart recovery and rollback result.
**Done when:** Actual authorized operation survives restart and replay, rejects violating evidence, propagates correct downstream state, and satisfies a recorded pilot acceptance test.

**Added deterministic invariant:** Source maturity transitions demand plant + date + signed evidence, not claims in document titles.


**Explicit closure tests for E25-S05:** Source maturity transitions demand plant + date + signed evidence, not claims in document titles.

- **E25-S05-T06 — Schema/transaction review:** reviewer checks exact active IDs, tenant compound keys, expected invariants, migration/rollback, and canonical event ownership.
- **E25-S05-T07 — End-to-end physical failure fixture:** demonstrate forbidden alternative and stale/duplicate/cross-PO event; preserve rejected evidence with zero unauthorized state mutation.
- **E25-S05-T08 — Promotion evidence:** attach UI/device proof, real/fixture source documents, persisted event trail, clock/latency observations and signed acceptance against master V5-T / MGT gates.


## 5. Technical-debt register — original 16 debts + reconciled alternate findings

All 16 first-audit TD entries are preserved verbatim below, including evidence and tasks. Newly isolated TD17–TD22 encode alternate-audit concerns; overlapping facts are linked, not duplicated as independent business features. Source paths are audit leads rather than proof of current HEAD.


### TD01–TD16 — primary V5 audit tasks
| ID | Priority | Observed debt | Required remediation | Source |
|---|---|---|---|---|
| TD01 | P0 | **Service topology misleading.** 28 of the 29 numbered s01–s29 service folders have zero files; s01 has SQL migration only | Create architecture decision: modular monolith now or implement bounded services; remove claims of 29 deployed microservices; assign actual ownership. | `services/s01-identity; services/s02-device-registry..s29-federation` |
| TD02 | P0 | **Volatile business state.** Server holds stores, packing, OCR, attachments and config in process maps; restart loses non-ledger state | Persist transactional aggregates and rebuild from event log; crash/restart evidence tests. | `services/edge-daemon/cmd/server/main.go` |
| TD03 | P0 | **Demo roll and pilot config in runtime.** ROLL-007 and Noble Garments pilot details are registered as runtime defaults | Use tenant-scoped onboarding fixtures only; production startup must not synthesize stock or orders. | `services/edge-daemon/cmd/server/main.go` |
| TD04 | P0 | **Weak authority lease.** Pure CanCommit checks scopes/writer but itself documents that it is not a distributed lock/lease | Enforce lease epoch, expiry and single-writer uniqueness in transactional database; fail closed offline. | `services/edge-daemon/pkg/brownfield/authority.go` |
| TD05 | P0 | **Witness attestation insufficient.** Two string IDs and HMAC shared secret are insufficient proof that two persons authenticated separately | Bind separate authentication sessions/challenges, approved device and proof timestamps; sign ledger receipt. | `services/edge-daemon/pkg/stores/issuance.go` |
| TD06 | P0 | **OCR mutable signoff contents.** Signoffs are role strings; doc hash covers only five scalar values; no complete immutable source snapshot | Hash canonical report, dispositions, underlying event revision and signer identity; re-sign on alteration. | `services/edge-daemon/pkg/ocr/signoff.go` |
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
**Evidence:** Signoffs are role strings; doc hash covers only five scalar values; no complete immutable source snapshot. **Files:** `services/edge-daemon/pkg/ocr/signoff.go`.
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


### TD17 — Shared package/source-tree divergence [P0; SOURCE-AUDIT-NEEDED]


**Evidence from alternate audit:** Possible distinct implementations under `pkg/` and `services/edge-daemon/internal/` or `services/edge-daemon/pkg/`. First audit did not establish code-equivalence or duplication; inventory imports and hashes before deleting.


#### Integrated ALT-00.1 — Package Consolidation & Unified Domain Kernel

- **As a** Platform Engineer,

- **I want** to eliminate duplicate packages across pkg/ and services/edge-daemon/,

- **So that** all edge and cloud runtimes import a single, authoritative set of domain entities and business invariants.

- **Acceptance Criteria**:

  1.  Move shared domain models (stores, cutting, sewing, packing, ocr, rfid, attachment) into pkg/domain/.

  2.  Remove all orphaned duplicate directories under services/edge-daemon/internal/ and services/edge-daemon/pkg/.

  3.  Ensure go build ./... and go test ./... compile cleanly without namespace aliasing or circular dependencies.

- **Tasks**:

  - TASK-001-1: Audit all imports in cmd/server/main.go and internal handlers.

  - TASK-001-2: Consolidate carton_packer.go, dwell.go, issuance.go, attachment_graph.go, and reconciliation.go into pkg/domain/.

  - TASK-001-3: Update Docker build pipelines to reference unified package roots.


**Merge gate:** preserve legitimate distinct modules, remove only proven redundant implementations; update import graph and contracts, or for marker path prove only independently CAD-approved geometry reaches cutter planning; test no synthetic marker can publish.


### TD18 — Unsupported toy marker geometry publishing path [P0; SOURCE-AUDIT-NEEDED]


**Evidence from alternate audit:** A rectangular strip/shelf nesting heuristic may be executable in `geometry_optimizer.go` and may violate CAD-sovereignty if allowed to publish marker geometry. Determine actual execution paths and label simulation-only code explicitly.


#### Integrated ALT-00.3 — Decommission Toy CAD Nesting Optimizer

- **As an** Industrial Engineering Lead,

- **I want** to remove the synthetic 2D strip-packing polygon generator from geometry_optimizer.go,

- **So that** the system strictly consumes verified CAD marker geometry (DXF/Gerber/Lectra) rather than generating unverified cutting patterns.

- **Acceptance Criteria**:

  1.  Deprecate Optimize2DMarkerAndVision in favor of a read-only MarkerGeometryPlan parser.

  2.  Restrict internal cutting algorithms to linear roll allocation, ply calculation, remnant tracking, and defect avoidance.

- **Tasks**:

  - TASK-003-1: Strip synthetic pattern polygon generation out of geometry_optimizer.go.

  - TASK-003-2: Refactor /api/v1/cutting/optimize-geometry to accept pre-computed marker recipes from external CAD.


**Merge gate:** preserve legitimate distinct modules, remove only proven redundant implementations; update import graph and contracts, or for marker path prove only independently CAD-approved geometry reaches cutter planning; test no synthetic marker can publish.


### TD02-A — Alternate-source implementation details for existing TD02 (not a new debt)


#### Integrated ALT-00.2 — State Persistence & PostgreSQL Outbox Migration

- **As an** Edge Systems Architect,

- **I want** all stores reservations, packing sessions, and attachment maps to persist across container restarts,

- **So that** plant-floor edge terminals do not lose live session state during network drops or host reboots.

- **Acceptance Criteria**:

  1.  Replace in-memory sync.Mutex maps in main.go (activePackings, activeOCRSess, stores.inventory) with SQLite-backed local outbox storage syncing to PostgreSQL.

  2.  Packing sessions and double-witness issuance slips survive an immediate edge daemon restart.

  3.  All SQL mutations enforce dynamic app.current_tenant_id session configurations without fallback to "TENANT_DEFAULT".

- **Tasks**:

  - TASK-002-1: Implement persistent SQLite outbox store for edge sessions.

  - TASK-002-2: Refactor StoresEngine and AttachmentEngine to load initial state from database tables on startup.

  - TASK-002-3: Remove all hardcoded "TENANT_DEFAULT" fallbacks across API handlers.


**Correction:** SQLite as a local outbox is an option; authoritative plant state must have a deliberate single-writer architecture. Local write buffering alone does not solve offline conflicting authors, tenant RLS or DB+outbox atomicity.


### TD19 — Hardcoded demonstration values and unsafe tenant fallbacks [P0; SOURCE-AUDIT-NEEDED]

- **Evidence lead:** alternate review cites `TENANT_DEFAULT`, `Line 14`, `KK524867`, `MK6574J9`, 1.25 fabric factors and fixed cut waste clamps in edge handlers/planning. First audit also flagged demo roll/staged config (TD03).
- **TD19-T01:** search active production call graph for constants and determine whether test fixtures, examples or executable fallbacks.
- **TD19-T02:** replace all authoritative defaults with tenant-configured validated typed IDs/units, distinguish scenario assumptions from measured fabric factors.
- **TD19-T03:** add hostile tenant/no-config tests; unknown tenant returns forbidden/needs configuration, never implicit default.
- **Exit:** no effective unreviewed sample tenant, buyer/line/style/order or cost coefficient on any production API path.

### TD20 — Cross-system integration and mathematical fixture coverage [P0]

- **Problem:** isolated domain tests do not prove end-to-end inventory→cut→sew→QC→hangtag→carton→dock→OCR invariants; the first audit identified integration test files but no accepted plant-flow test run.
- **TD20-T01:** build a golden PO with measured roll segments, heterogeneous marker, 97 actual of 100 planned plies, rejected panel replacement and explicit shade mismatch.
- **TD20-T02:** exercise station offline crashes, duplicate and conflicting writer requests; reconcile quantities/costs/stage proofs after restart.
- **TD20-T03:** publish canonical E2E fixture traces and property-based/fuzz regression suite to release pipeline.
- **Exit:** independently reconstructed signed stage balances and no double credits under adversarial replay.

### TD21 — Unverified maturity/provenance contamination [P0]

- **Problem:** alternate V5 explicitly labels multiple sectors VALIDATED and document itself Approved, but supplied materials contain no corresponding live signed proof. This risks improper rollout.
- **TD21-T01:** migrate provenance into requirement/pilot evidence register with separate SPECIFIED, DESIGNED, PROPOSED, IMPLEMENTED-UNVERIFIED, PILOT-VALIDATED.
- **TD21-T02:** strip assumed VALIDATED/production claims from release checks, feature flags and operational dashboards until source-backed.
- **TD21-T03:** require plant identifier, build SHA, tested_on, verifier, device logs, negative result and rollback acceptance before promotion.
- **Exit:** a screen/mockup/table name alone cannot raise maturity.

### TD22 — Identity/cardinality and draft SQL contract mismatches [P0]

- **Problem:** alternate DDL models serials as contiguous ranges, UPC as unique unit identity and stores frozen runtime assignments in static OB entry; these cannot represent mixed bundles, reusable tags and repeated shifts safely.
- **TD22-T01:** design typed garment serial set/physical component lineage and effective-dated OB assignment revisions.
- **TD22-T02:** enforce both tag and bundle active-binding uniqueness, unique actual buyer serial when present, tenant compound foreign keys, checked units and clean compensation.
- **TD22-T03:** migration/replay tests on multi-tenant 101/102-ply mixed marker and buyer UPC repeated across 100 garments.
- **Exit:** no mixed-unit merge, duplicate active tag, misidentified UPC, inaccurate serial range or stale assignment overwrite.


## 6. Canonical crosswalk — every alternate epic/story preserved

| Alternate story | Canonical ticket | Source scope / disposition |
|---|---|---|

| ALT-00.1 — Package Consolidation & Unified Domain Kernel | **TD17** | Source acceptance and tasks retained in mapped canonical block. |

| ALT-00.2 — State Persistence & PostgreSQL Outbox Migration | **TD02** | Source acceptance and tasks retained in mapped canonical block. |

| ALT-00.3 — Decommission Toy CAD Nesting Optimizer | **TD18** | Source acceptance and tasks retained in mapped canonical block. |

| ALT-01.1 — Pre-PO Surplus Compatibility Engine | **E19-S05** | GSM ±3% and shade ΔE are buyer-configured proposals; no merchandising override bypasses an absolute buyer/ownership lock. |

| ALT-01.2 — Zero-Borrowing Hard Lock & CEO Narrative Override | **E19-S01** | Material borrowing is forbidden. CEO consent governs specific defined exceptions; no cross-PO transfer without approved new plan and free reservation. |

| ALT-01.3 — Double-Witness Material Custody Handshake | **E19-S04** | Source acceptance and tasks retained in mapped canonical block. |

| ALT-02.1 — Statistical Factory SAM Confidence Engine | **E23-S02** | Counts and CV thresholds are candidate policies; censoring outliers requires review, not blind exclusion. |

| ALT-02.2 — Closed-Loop Margin Waterfall & Variance Attribution | **E20-S05** | Source acceptance and tasks retained in mapped canonical block. |

| ALT-03.1 — Synchronized 8-Stream T&A Scheduler | **E21-S02** | Source acceptance and tasks retained in mapped canonical block. |

| ALT-03.2 — Three-Layer Daily Production Plan (DPP) Variance Loop | **E21-S01** | 08:30 and >10% for five shifts are plant-configured proposed thresholds; actual means accepted output. |

| ALT-03.3 — Factory-Pool Labor Mobility & Compliance Audit Trail | **E21-S03** | Earned wages are read only from authorized payroll systems; wage calculation is not implicitly in scope. |

| ALT-04.1 — Multi-CAD Tiered Ingestion Adapters | **E25-S03** | Native CAD extension support requires vendor files, licenses and reference differential tests; never invent working binary parsers. |

| ALT-04.2 — Anti-Orphan Bundle Balancing Arithmetic | **E22-S01** | The cited grouping applies only when contiguous ply, shade and part boundaries permit it; optimize legal groups. |

| ALT-04.3 — Dual-Layer Sequential Numbering & Internal Provenance | **E22-S01** | Source acceptance and tasks retained in mapped canonical block. |

| ALT-04.4 — Cursor-Driven RFID Helper Commissioning Workflow | **E22-S03** | 720 bundles/15 min and 300ms cursor are candidate performance fixtures; qualify actual scanner and printer. |

| ALT-04.5 — Cut Reconciliation Gate Before Bundle Release | **E22-S06** | Reconcile ISSUED fabric to disposition; RESERVED is a separate commitment. ±2% is illustrative. |

| ALT-05.1 — Three-Stage Progressively Enriched Operation Bulletin | **E23-S01** | Source acceptance and tasks retained in mapped canonical block. |

| ALT-05.2 — Typed OperationExecutionContract & Anti-Mistake Credit Engine | **E23-S03** | Dwell thresholds and RFID presence are anomaly signals, not independently sufficient proof of work or earned pay. |

| ALT-06.1 — Atomic Finishing Hangtag Transition (Hangtag Swap) | **E24-S01** | Buyer UPC denotes SKU unless truly serialized; tag goes to inspection before reuse. |

| ALT-06.2 — Defect-Specific 3-Way Redo Routing & Rescue Rate | **E24-S02** | Source acceptance and tasks retained in mapped canonical block. |

| ALT-06.3 — Packing Misscan Contract & Audited Repack State Machine | **E24-S03** | 200ms local UI and 5 misscans/hour coaching are proposed nonpunitive, device-qualified settings; repack is a separate canonical story E24-S04. |

| ALT-06.4 — Decremental Manifest Dock Loading | **E24-S05** | Source acceptance and tasks retained in mapped canonical block. |

| ALT-06.5 — Cryptographic Order Closure Report (OCRCertificate) | **E24-S06** | Cut=Ship+Lost+Leftover is valid only after typed stage/unit conservation; six signatures are bound to frozen hash. |

| ALT-07.1 — Circulating RFID Carrier Pool Lifecycle & Attrition Monitor | **E25-S02** | Loss >2%, rotation volumes and turnaround days are capacity hypotheses until live pilot data. |

| ALT-07.2 — Native-Script Floor Localization Engine | **E25-S04** | Source acceptance and tasks retained in mapped canonical block. |

| ALT-07.3 — Brownfield Authority Gate Enforcement | **E25-S01** | Brownfield M0/M1 may observe, but M2/M3 authoritative RFID-dependent actions require commissioned coverage. |


## 7. Dependency graph and release increments — evidence-based, not fixed-week promises

| Gate | Minimum stories / debt | Physically verifiable exit |
|---|---|---|
| G0 Build / preserve truth | TD01–TD10, TD14, TD17–TD22, E17 | pinned build/test, no default tenant, DB/business outbox atomic, replay & tenant exploit clean |
| G1 Brownfield qualification | E01–E04, E16–E18, E25-S01/S05 | legacy M0/M1 unchanged; signed source revisions; RFID gate for pilot scoped transactions |
| G2 Commercial and material commitment | E05, E19, E20, E21-S01/S02 | buyer-approved quote, net indent, double witness, no borrowing, realistic DPP/8 streams |
| G3 Physical cutting truth | E06–E08, E22, E25-S03 | approved CAD marker, measured actual plies, classic valid groups, tag cursor, material balance |
| G4 Assembly earned credit | E09, E23, E21-S03/S05 | signed OB/skill/attachment prerequisites; zero double credit, offline replay; FPY evidence |
| G5 Finish to dispatch closure | E10–E12, E24, E25-S02/S04 | unique buyer binding, QC redo, exact carton/repack, manifest, six-signed OCR |
| G6 Optimization with ROI | E13–E15, E20-S05, E21-S04 | calibrated queues/costs and controlled A/B evidence with uncertainty |

Alternate plan proposed **six two-week sprints** and **four six-week horizons**; retain these as planning scenarios, not factual estimates. One sprint cannot be credibly committed until actual source audit, team capacity and hardware/vendor lead time are known. Incremental pilot entry can use one vertical flow without requiring all bounded contexts to become individual deployments.

## 8. Verification and security/policy matrix


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


**All 28 normative V5-T tests, 15 mapped alternate GATE-V5 scenarios and 12 master MGT release gates from the master PRD apply.** Each story must cite exact gate IDs on its delivery ticket. Numeric candidates (08:30, 2%, 200/300 ms, SAM n/CV, 5 misscans/hr, 2% tag loss and 8h WAN) remain test hypotheses unless approved plant settings.

## 9. Audit evidence contract and unresolved work


## 9. Review notes and caveats
- The inspected code snapshot may lag a live deployed environment; no production system credentials or deployed service inventory were provided. Evidence here is limited to the attached archive.
- Empty numbered service folders are decisive evidence that those folders do not implement independently deployable services in this snapshot. They do not imply that no corresponding business logic exists elsewhere in the edge monolith or `pkg`.
- A test unable to execute due to missing toolchain/network is recorded as blocked, never reported as failing or passing.
- Do not translate unverified planned scope into % completion or calendar delivery estimates.
- V5 founder locks and Classic Cut-Bundle Profile are mandatory as specified; proposed optimization and SAM confidence heuristics still require founder/IE approval.


## 10. Archived alternate-V5 sprint proposal — schedule hypothesis, not an engineering commitment

The second backlog's proposed six two-week sprints are retained here with their original story grouping. They are *not* validated durations; dependency and team-capacity gates take precedence.


## Part 3: Sprint Delivery Sequence

> Sprint S1 (Weeks 1–2): Tech Debt Remediation & Governance Foundation
> ├── Story 00.1: Package Consolidation & Unified Domain Kernel
> ├── Story 00.2: State Persistence & SQLite/Postgres Outbox Migration
> └── Story 01.2: Zero-Borrowing Hard Lock & CEO Narrative Override
>
> Sprint S2 (Weeks 3–4): Sourcing Precision & Commercial Intelligence
> ├── Story 01.1: Pre-PO Surplus Compatibility Engine
> ├── Story 01.3: Double-Witness Material Custody Handshake
> ├── Story 02.1: Statistical Factory SAM Confidence Engine
> └── Story 00.3: Decommission Toy CAD Nesting Optimizer
>
> Sprint S3 (Weeks 5–6): Cutting Room Optimization & Bundling Architecture
> ├── Story 04.1: Multi-CAD Tiered Ingestion Adapters
> ├── Story 04.2: Anti-Orphan Bundle Balancing Arithmetic
> ├── Story 04.3: Dual-Layer Sequential Numbering & Internal Provenance
> └── Story 04.4: Cursor-Driven RFID Helper Commissioning Workflow
>
> Sprint S4 (Weeks 7–8): Production Floor Intelligence & Execution Contracts
> ├── Story 04.5: Cut Reconciliation Gate Before Bundle Release
> ├── Story 05.1: Three-Stage Progressively Enriched Operation Bulletin
> ├── Story 05.2: Typed OperationExecutionContract & Anti-Mistake Credit Engine
> └── Story 07.1: Circulating RFID Carrier Pool Lifecycle & Attrition Monitor
>
> Sprint S5 (Weeks 9–10): Finish-to-Ship Logistics & Quality Integrity
> ├── Story 06.1: Atomic Finishing Hangtag Transition (Hangtag Swap)
> ├── Story 06.2: Defect-Specific 3-Way Redo Routing & Rescue Rate
> ├── Story 06.3: Packing Misscan Contract & Audited Repack State Machine
> └── Story 06.4: Decremental Manifest Dock Loading
>
> Sprint S6 (Weeks 11–12): T&A Planning, Order Closure & Global Localization
> ├── Story 03.1: Synchronized 8-Stream T&A Scheduler
> ├── Story 03.2: Three-Layer Daily Production Plan (DPP) Variance Loop
> ├── Story 03.3: Factory-Pool Labor Mobility & Compliance Audit Trail
> ├── Story 02.2: Closed-Loop Margin Waterfall & Variance Attribution
> ├── Story 06.5: Cryptographic Order Closure Report (OCRCertificate)
> ├── Story 07.2: Native-Script Floor Localization Engine
> └── Story 07.3: Brownfield Authority Gate Enforcement


### Critical unresolved design holds

- Confirm four exact Part-1 costing models and CFO/CEO commercial exception matrix; do not implement invented formula names.
- Confirm Top Management Council quorum including Finance, CEO incapacity plan and legal custody of buyer-owned stock.
- Confirm whether CEO-only margin floor and surplus exceptions extend founder locks; default deny until signed.
- Confirm physical tag types/portals, lot/shade segment handling, cut bundle minima and practical station timings at each plant.
- Confirm SAM tier and censoring policies, DPP daily cutoff, material tolerance, tag attrition denominator and locally measured performance SLOs.
- Confirm the inherited 113 screen route inventory and actual installed UI contracts; the few HTML files in snapshot do not prove all 113 shipped.
- Confirm actual live repo commit/build/test and pilot hardware acceptance; current audit cannot establish full code completion.

### Source accounting

- 72 V4 stories retained with original story description, acceptance, dependencies and 3 original tasks; 3 new operational closure tasks each.
- 37 primary V5 stories retained with 5 original delivery tasks each, explicit integration details and 3 additional closure tasks each.
- 26 alternate stories mapped and their distinct acceptance/tasks preserved inside canonical tickets (00.x in TD tickets); all remain source proposals when numerical claims lack validation.
- All original 16 technical debts preserved; TD17/TD18/TD19/TD20/TD21/TD22 added and TD02 augmented.
- Cross-reference: PRD/ARD V5 A–J + reconciliations M1–M12 + inherited V4/V3.
