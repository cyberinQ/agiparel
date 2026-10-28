# Agiparel — Production Readiness Master Plan

Consolidated assessment · Chapters 01–10 · October 28, 2026

Executive assessment: Agiparel has progressed from a working prototype to a credible production architecture. However, the remaining gaps are concentrated in a few shared foundations rather than ten independent chapters.

My recommendation is to consolidate the outstanding work into 10 reusable engineering workstreams, prioritize the blockers, and then activate each chapter against common production acceptance criteria.

## 1. Ten common mitigation workstreams

These should become reusable platform capabilities—not chapter-specific patches.

| ID  | Shared mitigation                                                                                                              | Priority | Chapters benefiting |
| --- | ------------------------------------------------------------------------------------------------------------------------------ | -------- | ------------------- |
| F1  | Enterprise identity, SSO, RBAC, separation of duties, tenant/plant permissions and audit                                       | P0       | 01–10               |
| F2  | Canonical, versioned master data: factories, buyers, styles, BOMs, routes, skills, schedules, units and commercial contracts   | P0       | 01–09               |
| F3  | End-to-end individual garment genealogy: cut parts → bundles → sewing → QC → packing → shipment                                | P0       | 05–09               |
| F4  | Authoritative inventory and custody ledger: receipts, inspections, reservations, consumption, transfers, scrap and returns     | P0       | 02–08               |
| F5  | Real factory execution: enrolled RFID devices, signed events, offline synchronization, station controls and worker terminals   | P0       | 04–08               |
| F6  | Governed quality, finishing, cartonization and dispatch, with configurable buyer-specific rules                                | P0       | 02, 05–09           |
| F7  | Costing, finite-capacity planning, resource scheduling and contribution-margin engine                                          | P0/P1    | 01–09               |
| F8  | Secure document intelligence: document authorization, version comparisons, malware isolation, search and retention             | P0/P1    | 01–04, 10           |
| F9  | Exception management and trustworthy analytics: operational queues, SLAs, KPI definitions, history and profitability reporting | P1       | 01–10               |
| F10 | Production engineering: automated testing, observability, database performance, backup/recovery, CI/CD and deployment safety   | P0       | 01–10               |

P0 = required before the affected workflow can be production-authoritative. P1 = important for scaled operational excellence, with specific items becoming P0 when required by a buyer or factory.

The critical insight is that F1 + F2 + F3 + F4 + F5 form the operational truth layer. Once established, QC, packing, management dashboards and financial optimization can share the same verified facts instead of independently reconstructing factory reality.

## 2. Chapter-by-chapter remaining gaps and mitigations

The table consolidates the gaps from our earlier reviews, including those not yet mitigated in Chapters 01–06.

| Chapter                    | Remaining production gaps                                                                                                                                                                                                                                                                                                 | Recommended mitigation                                                                                                                                                                    |
| -------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 01 — Factory Setup         | No fully governed multi-plant master; limited machine/skill/shift capability; unreliable capacity estimates; ambiguous labor/overhead rate bases; discovery facts not promoted to approved masters; limited opportunity-cost visibility                                                                                   | F1, F2, F7, F8. Versioned plant registry, machines, worker qualifications, shift calendars, effective-dated rates and bottleneck capacity. Require formal master-data publication         |
| 02 — Buyer Orders          | No authoritative PO amendment lifecycle; incomplete commercial terms and source reconciliation; unverified delivery feasibility; buyer-specific packing/label rules; two intake paths with differing review semantics                                                                                                     | F2, F6, F7, F8. Governed PO/tech-pack revisions, approved line-level commitments, contract rule engine, contribution and capacity feasibility checks                                      |
| 03 — Material Planning     | No authoritative lot reservations; incomplete approved BOM consumption and landed cost; competing orders may use the same stock; no governed allocation/shortage policy; supplier MOQ and lead-time gaps                                                                                                                  | F2, F4, F7, F9. Transactional reservation ledger, versioned BOM/UOM/quotes, scarcity-aware allocation and exception-led procurement                                                       |
| 04 — Fabric Stores         | Potential cross-plant picking; no complete physical roll custody, usable-length/defect history or verified remnant balances; missing supplier-claim recovery; limited plant-scale roll search; no actionable exception queue                                                                                              | F4, F5, F9. Verified roll passports, reservation/movement accounting, signed transfers, filtered pagination, supplier claims and discrepancy ownership                                    |
| 05 — Cutting               | Cutting-table/plant authority incomplete; manually entered marker values; no verified CAD-marker compatibility; insufficient accepted-piece, defect, recut and scrap genealogy; incomplete cutting optimization and exception monitoring                                                                                  | F2, F3, F4, F5, F7. Certified marker evidence, physical consumption and accepted-cut counts, unit/part lineage, verified cutting-table custody and campaign optimization                  |
| 06 — Sewing                | Production API exists, but real enrolled hardware, workforce login and operator console remain uncommissioned; incomplete approved route/skill/machine administration; no authoritative garment completion; hard-coded physical model limits remain upstream; weak real downtime/rework economics and exception workflows | F1, F2, F3, F5, F7, F9. Real device integrations, authenticated execution terminals, configurable routing, unit-level completion, typed holds and SAM-based performance                   |
| 07 — QC & Finishing        | Only group-level quality attestations; no garment-level disposition; no real inspector UI; no physical quarantine/scrap custody; no configurable buyer AQL/measurement plans; insufficient defect and rework economics                                                                                                    | F1, F3, F5, F6, F9. Individual-unit inspections, approved sampling plans, independent disposition, quarantine and finishing release; defect Pareto/DHU/FPY based on verified denominators |
| 08 — Packing & Dispatch    | Production staging is HOLD-only; no verified individual garment scans, finished-stock release, physical carton seal/reopen, buyer carton assortment, shipping authorization or carrier handoff                                                                                                                            | F3, F4, F5, F6. Unique unit-to-carton assignment, buyer packing rule validation, authenticated carton sealing, manifest reconciliation, independent gate-out and carrier evidence         |
| 09 — Intelligence          | Verified garment output, labor efficiency, first-pass yield, OTIF and margin remain unavailable; manual pilot authentication; no trusted shift/SAM/cost sources; incomplete historical KPI rollups and large-tenant performance qualification                                                                             | F1, F3, F7, F9, F10. Governed KPI semantic layer with lineage, consistent time windows, formula versioning, production dashboards and materialized histories                              |
| 10 — Document Intelligence | No fully integrated workforce SSO/document permissions; secure extraction, malware quarantine, access logging and retention incomplete; search limited compared with enterprise needs; no automated amendment comparison and document exception queue                                                                     | F1, F8, F9, F10. Secure evidence vault, sandboxed processing, classification-based access, indexed search, version-diff intelligence and immutable access audit                           |

This table includes remaining architectural gaps—not merely the defects corrected during the initial chapter reviews.

The most important dependency: Chapters 07–09 must not claim production garment quantities, finished inventory, shipments, yield or profitability until Chapters 05–06 establish verifiable unit-level genealogy and Chapters 07–08 complete the associated acceptance and custody controls.

## 3. Additional gaps we may have missed

These are additional production risks to investigate. They are not all confirmed defects in the present code.

| Priority | Additional risk                                                                                                                                            | Recommended mitigation                                                                                                                              |
| -------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------- |
| P0       | Cross-chapter quantity conservation. A garment could conceivably be counted multiple times across cutting, sewing, repair and packing                      | Define enforceable material/unit conservation rules, including splits, recuts, replacements, scrap and returns. Test every state transition         |
| P0       | Reusable RFID tag versus garment identity. A reusable physical carrier must not become a permanent garment identity                                        | Separate physical carrier ID, tag-assignment lifecycle, bundle ID and garment serial. Preserve historical bindings when tags are retired and reused |
| P0       | Device trust lifecycle. Signed reader messages alone do not prove the tag was physically where claimed                                                     | Device enrollment, key rotation/revocation, secure firmware, station calibration, location validation and suspicious-read detection                 |
| P0       | Offline consistency and clock correctness. Disconnected factories may replay delayed, duplicated or out-of-order events                                    | Durable outbox/inbox, unique event IDs, trusted observed/received timestamps, conflict resolution, replay quarantine and recovery testing           |
| P0       | Enterprise integrations. Orders, procurement, accounting and shipping may exist in other authoritative systems                                             | Versioned adapters for ERP, PLM, WMS, HR/identity, buyer EDI/ASN and carrier systems; defined systems of record and reconciliation                  |
| P0       | Security and sensitive data. Payroll, workforce identities, supplier prices and buyer contracts require distinct controls                                  | Encryption, secrets management, least privilege, document classification, retention, access monitoring and privacy-policy enforcement               |
| P0       | Disaster recovery and deployment reliability. A factory must recover from infrastructure failure without corrupting inventory or production history        | Defined RPO/RTO, proven backup restoration, HA strategy, health checks, rollback-compatible migrations, immutable event recovery                    |
| P0       | Database isolation under actual load. Passing unit tests cannot establish cross-tenant security or factory-scale performance                               | Real PostgreSQL/RLS tests, concurrent reservation tests, transaction race tests, query-plan benchmarks and realistic workload simulations           |
| P0       | Versioned configuration and measurement semantics. Different factories use different physical limits, units, currencies, calendars and product definitions | Effective-dated configurations, decimal/UOM conversion rules, timezone-aware calendars, currency/FX provenance and schema compatibility tests       |
| P1       | Operational fallback and adoption. Workers need safe procedures when scanners, network links or terminals fail                                             | Controlled manual exception mode, supervisor authorization, training, multilingual UX, accessibility and later evidence reconciliation              |
| P1       | Compliance and buyer auditability. Country-of-origin, labor, workplace, chemical, labeling and export obligations vary                                     | Configurable compliance requirements tied to buyer, product, facility and destination, with retained evidence and approval checkpoints              |

One particularly important discovered constraint: earlier cutting and sewing models retain physical limits such as 210 plies, 80 marker positions, 8 parts per garment and 16 sewing-route parts. These are domain-model restrictions, not ordinary API pagination limits. They need a backward-compatible, configuration-driven redesign before claims of universal factory scalability.

## 4. Recommended execution roadmap

Stage 1 — Common production foundation

F1 + F10 · P0

Implement real workforce/device identity, tenant/plant authorization, production environments, migration governance, security testing, observability and backup recovery.

Exit gate: Independently authenticated actors, proven tenant isolation, reproducible deployments and verified recovery.

Stage 2 — Business and material source of truth

F2 + F4 + F8 + essential F7 · P0

Govern factories, styles, BOMs, PO revisions, supplier quotes, roll inventory, custody, reservations and costing inputs.

Exit gate: No double-reserved inventory, no superseded buyer instruction used for approval, no undocumented cross-plant stock movement.

Stage 3 — Real cutting and sewing genealogy

F3 + F5 · P0

Establish unit-level garment identity, enrolled physical readers, approved routes, real worker transactions, recuts, scrap and verified WIP transfers.

Exit gate: Trace a garment forward from material/parts to sewing completion and backward to its originating evidence, without duplicate output credit.

Stage 4 — QC, packing and dispatch authority

F6 · P0

Implement genuine inspections, quarantine, rework, finishing, buyer-compliant cartons, sealing, manifests and authenticated carrier handoff.

Exit gate: No failed or unreleased garment can enter a finalized carton or shipping manifest; every dispatched unit has custody proof.

Stage 5 — Operational excellence and margin optimization

Full F7 + F9 · P1

Build verified throughput, yield, line balance, lead-time risk, buyer profitability, working-capital efficiency, exception queues and cross-factory optimization.

Exit gate: Every KPI reconciles to its authoritative events, with formula, denominator, effective period, cost basis and evidence provenance.

F8 document security and the essential parts of F7 costing begin earlier, because buyer commitment, purchasing and material allocation cannot safely wait for the analytics phase.

## 5. Release acceptance scorecard

A chapter moves from pilot-only to production-authoritative only when its required gates pass.

Production qualification tracker

0 / 7 verified

Identity and tenant isolation

Real IdP, RBAC, segregation of duties, RLS and cross-plant denial tests

Physical evidence integrity

Enrolled hardware or approved independent attestations; no simulation-derived credits

Quantity and financial reconciliation

No duplicate allocation, counted output, shipping or cost credit

Failure and concurrency resilience

Offline replay, duplicate events, concurrent writes, partial failure and recovery

Factory-agnostic configuration

Small, medium and large plant models with varied products, routes and capacities

Security and recovery

Threat tests, document controls, backup restoration and deploy/rollback verification

Real operator acceptance

End-to-end workflows validated with actual factory roles and devices

Planning checklist only. Tick gates after actual verification; no production acceptance has been inferred.

## 6. Where I recommend we start

Start with F1 — Enterprise Identity & Factory Authorization, paired with the core data contracts for F2/F3.

The first engineering increment should deliver a shared tenant/plant security model, genuine OIDC-based login and authorization, production-grade operator sessions, provisioner-controlled roles, database isolation tests, and adapters for the existing Chapter 06–09 production APIs. Preserve the existing UI and do not activate simulation-to-production promotion.

The next increment establishes individual garment identity and an authoritative event/quantity ledger. That removes the biggest dependency currently blocking legitimate QC, finished inventory, dispatch and management KPIs.

For every increment, we will continue your established delivery recipe:

Changed-files ZIP → Inflate/sync → Migration → Test → Build → Deploy → Git, with explicit production acceptance results.

Bottom line: The priority is not to add more screens. It is to make every factory transaction provably authentic, traceable, reconcilable, scalable and economically meaningful. Once those foundations are in place, Agiparel can become a genuinely multi-factory operating system rather than a collection of individually functional modules.