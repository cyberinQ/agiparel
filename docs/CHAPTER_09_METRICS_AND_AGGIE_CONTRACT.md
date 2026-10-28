<!-- Author: Sunil Jain, 2026-10-28. Chapter 09 evidence and metric-governance contract. -->
# Chapter 09 — Manager measures, source-of-truth, and Aggie boundary

Reference: `Agiparel_Master_PRD_ARD_v5.3.md` §11.4 and V5-C5; `Agiparel_Master_Epics_Stories_Tasks_v5.3.md` Epic E13.

## Guiding rule

The employee sees **How is work moving? → What needs attention? → What should I check next?**. Five manager/IE reporting groups remain available as progressive detail. A missing metric is **Not measured**, not zero; estimated or simulated information is never silently labeled actual.

**Current implementation:** `/api/v1/intelligence/sim/report` is a *development-only, cutting-lay-scoped rehearsal*. It reads simulated cut/sew/quality/packing tables, calculates distinct serial counts, conservation-style residual warnings and p50/p90/p95 practice-time distributions, and saves immutable simulated snapshots. Its bounded intervals do **not** measure shift-adjusted queueing, labor-paid time, blockage/starvation, or observed physical throughput. The Chapter 09 UI v16 renders this authority explicitly; it adds no production claims.

## Canonical future metric definitions (NOT yet implemented as production facts)

| Tier | KPI contract | Required validated basis | Release condition |
|---|---|---|---|
| Flow | Station p50/p90/p95 cycle and queue distributions | Time-synchronized event/processing timestamps, unique genealogy, shift calendar, censored/missing-event rules | Report sample size, omissions, interval type, clock uncertainty and versioned operation/SAM |
| Flow | Pace deviation | `(comparable observed active operation sec − approved target SAM sec) / target SAM sec` | Guard SAM > 0, matching operation/style/cohort/unit; example +15% is configurable, not an automatic policy |
| Flow | Starvation / blockage | Explicit absence of upstream supply vs downstream buffer-full evidence | Never classify idle duration by scan gaps alone; root cause remains unknown without causal event chain |
| Flow | Line-balance efficiency | Sum of matched standard operation workload / effective available station time | Define parallel staffing, tasks, station routing and mix; do not divide mismatched SAM and wall time |
| People | Earned labor efficiency | `approved earned SAM on accepted credited work / eligible productive worker minutes` | Approved SAM and accepted credits; controlled break/briefing/outage exclusion policy; not payroll derived from scans |
| People | Stoppage frequency / machine utilization | Typed issue events and measured downtime; calibrated production capacity and available operating time | Separate planned, unplanned, idle, blocked and unobserved; do not rank individuals for punishment |
| Quality | First-pass yield | First-accepted first-inspection units / distinct units first inspected | Maintain immutable first-inspection facts, not current accepted state alone |
| Quality | DHU | `100 × recorded defects / inspected units` | Versioned defect taxonomy; declare inspection station and whether repeat inspections count |
| Quality | Repair turnaround | QA HOLD → repair work → reinspection accepted elapsed | Verified identity and timestamps, shift calendar, exclusions, censored repairs |
| Material | Fabric yield and remnant | Accepted cut component-equivalent output against measured good material issued/used | Preserve meters vs square meters vs piece units; measured width, scrap and returns |
| Material | Cut-to-ship reconciliation | Typed stage balance for cut, sewn, QA hold, packed, sealed, physically shipped and unexplained | Verify common garment equivalent, PO boundary and signed reconciliations; never count draft cut positions as units |
| Order | OTIF | Contractual milestone and accepted delivered full quantity | Validated shipment/receipt authority, buyer allocation and due-date amendments |
| Order | Cost per accepted garment | Actually incurred material, direct labor, rework and variable overhead / accepted units | Approved cost model, currency, allocations, rate versions and denominator |
| Order | Margin to complete | Contract revenue − actual incurred spend − modeled remaining spend − approved risk reserve | Governed costing assumptions, revision, exchange rates and scenario confidence |

## Aggie v16 behavior

The Chapter 09 **Ask Aggie to explain** button pre-populates the existing full-height chat with the current *simulated* evidence, source and top warnings. It does not send an automatic question, run model-generated SQL or authorize a decision. Incomplete previous chat input is preserved instead of overwritten. The deterministic counters and alert labels are the source-of-truth; Aggie is a narrative assistant.

### Production evolution (gated; no claims of completion)

1. Version `metric_id`, formula, denominator, inclusion/exclusion, units, source table, authority, scope (tenant/plant/line/style/order/shift), clock policy, SAM revision, confidence/evidence and last-refresh time in a **metric catalog**.
2. Project validated event streams into bounded, indexable materialized/rollup facts. Use distinct garment/serial genealogies, snapshots and event-time versus ingest-time. Raw scan events remain audit evidence, not the default dashboard query surface.
3. Give Aggie a `resolve_metric_intent` tool returning one catalog metric + typed filters + a source-bound result with evidence. **No model-generated SQL by default.** Query templates should be parameterized and tenant-scoped using read-only credentials and time/row limits.
4. For unlisted investigations, require a reviewed query template plus `sqlguard.CatalogInspector` permissions, allowlisted views, `EXPLAIN` cost cap, a read-only transaction, audit trail and operator confirmation before promotion to a shared metric. A 100-row limit alone does not bound scans or compute cost.
5. Show `OBSERVED_EVENT`, `ESTIMATE / SIMULATION`, or `PROVISIONAL_UNCONFIRMED`, plus numerator/denominator, sample, exclusions, as-of timestamp and drill-down evidence. Never treat a model-authored explanation as causal proof; phrase it as a hypothesis with verification actions.
6. Pilot with out-of-order/duplicate scans, overnight and unsynchronized clocks, missed RFID, changing SAMs, repair loops, hold-to-accepted transitions, mixed units, split orders, and late corrections. Verify immutable signed snapshots survive replay and amendment.

The canonical production views and real manager KPIs are **future backend milestones**. They must not be simulated in frontend code merely to populate attractive KPI cards.
