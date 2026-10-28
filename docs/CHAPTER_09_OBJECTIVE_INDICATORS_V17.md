<!-- Author: Sunil Jain, 2026-10-28. Objective, nonpunitive factory metric system. -->
# Chapter 09: evidence-first indicators for Chapters 01–08

## Product contract

The home of Chapter 09 remains **Choose work → See progress → Check what needs attention**, with collapsible specialist detail. The additional **Explore your factory** section contains eight chapter-colored groups, each with three clickable measures. Every measure opens a large accessible detail view showing its definition, a current value or **Not measured**, evidence provenance, explicit limitations, a defensible in-report snapshot plot if available, and a link to the source chapter.

**Blame-proof operating rules**

- Measures characterize **systems, conditions, throughput, evidence quality and handoffs**; they must not produce individual worker rankings, blame labels, or disciplinary scores.
- A report distinguishes `OBSERVED_EVENT`, `ESTIMATE / SIMULATION`, and `PROVISIONAL_UNCONFIRMED`. Current Chapter 09 uses only `REHEARSAL_ONLY_NOT_PRODUCTION_CLOSURE` and must label it practice/simulation.
- A zero is valid only when the measurement source can establish a zero. No records or an absent field means **Not measured**, never zero.
- Always expose a formal metric definition, numerator and denominator if a ratio, unit, cohort, source table, sample count, exclusions, time zone/calendar policy, and last measured time. When those properties cannot be established, show no authoritative metric.
- No inferred causality from an elapsed time or count mismatch; unknown is unknown. Account for clock drift, missed scans, overnight gaps, rework loops and late corrections before publishing cycle or queue trends.
- Machine downtime, material stock delays, training, maintenance, approvals, change orders and system errors are contextual evidence, not blame attribution.
- No user-uploaded spreadsheet or AI response becomes an operational fact until mapped, validated, reviewed and approved in its source chapter.

## First-release coverage

24 well-defined measures, three per chapter; linked in `factory_indicator_catalog.js`. Source-backed **practice** values are possible from the existing selected-lay API only for Cutting, Sewing, Quality and Packing. Chapters 01–04 and unimplemented KPIs deliberately say **Not measured**. All charts of these values are explicitly **single-report comparisons**, not trends. The existing snapshots API exposes capture hashes, not historical numeric series: historical plots are not yet available.

## Next backend increment: production-grade E13

1. Version a metric catalog in code/schema (metric ID, formula, numerator/denominator, source, scope, sampling, units, exclusions, authority).
2. Build tenant-isolated, indexed, immutable per-period metric facts with evidence references, clocks and shift-calendar constraints. Prefer a canonical read model to repeated ad-hoc scans over event tables.
3. Provide an authenticated, bounded `GET /api/v1/intelligence/metrics/{metric_id}?period=...&scope=...` and `GET .../series` returning `{value, unit, authority, sample_count, exclusions, evidence_links, as_of, formula_version}`. Return explicit insufficient-evidence metadata instead of fabricated numbers.
4. Render dated trend charts only from comparable metric series with formula-revision tracking; never interpolate unknowns as zeros. Filters: order, style, line, factory, shift, date. Preserve selected filters across navigation.
5. Aggie may resolve metric intents and narrate governed calculations, proposing checks. The LLM must not define ad-hoc denominators, bypass source review, grant approvals, or assert cause without evidence.

### Acceptance criteria

A manager can open any measure, see the source-backed basis or **Not measured**, open the producing chapter, and return to the indicator panel without losing form work or Aggie conversation. All plots display units, sample limits and authority. A practice value is never mislabeled real; human identities are not ranked or scored; missing records do not become zeros; numerical historical trends remain hidden until real comparable series exist.
