<!-- Author: Sunil Jain, 2026-10-28 — Chapter 01 factory-operations review and delivered change contract. -->
# Chapter 01: Factory Setup — Operational Review

**Review date:** 2026-10-28. **Scope:** Chapter 01 `/factory-setup` and its factory-discovery, document-candidate, draft, approval and reference pathways. **Exclusions:** Chapters 02–10, production release, material issuance and cross-factory dispatch.

## Operating judgment

Factory-first onboarding is directionally right: estimates are accepted without manufacturing false certainty, original documents are preserved, factual corrections are versioned, buyer inputs can arrive before factory setup, and authoritative production/material activity remains separately gated. The largest risk is *not* lack of more fields; it is that the UI obscures the existing evidence-readiness outputs and lets users confuse a self-confirmed estimate or a reference record with a feasible order or a price that makes money.

### Implemented in this increment

| Finding | Factory consequence | Fix |
|---|---|---|
| Capacity, cost and stores readiness calculated in `factorySuggestions` but concealed in an `sr-only` element inside a normally closed advanced dialog | Manager cannot see missing decisions or why a quote/production estimate would be premature | Display three succinct readiness cards with missing facts and explicit **not approved** boundary |
| Backend prompts and frontend hard-coded priorities can diverge; completed priority prompts loop back to factory name | Repeated questions waste supervisor time; verified information looks unfinished | Single server-provided ordered `next_questions`, including remaining fields; correct all-complete state |
| No skip/resume on the one-question wizard | Pressures operators to invent data or abandon onboarding | Non-writing “Skip for now” and “Review skipped questions”, per page only; no false confirmations |
| “Review & Submit” is marked as a dialog-opening button but actually navigates to Documents | Misleading affordance and accessibility semantics | True link to `/documents#review-submit`, retaining pending-count display |
| Chapter 01 upload invitation explicitly names payroll, although the development login is not suited for PII | Encourages exposure of employee personal data | Replace with a clear warning: **do not upload payroll, identity documents or named staff records** in this dev deployment |
| Discovery feed is limited to 40 recently captured files; editing a fact linked to an older source can silently clear its provenance | Critical audit and source lineage can disappear | Preserve older source option when editing; show “source attached (outside recent file list)” instead of “no document” |

**Schema/migrations:** None. The JSON response adds `next_questions` without removing existing fields. No new operational write endpoint, identity, production gate, inventory balance, or cost authorization has been added.

## Issues retained for later Chapter 01 backlog (not fabricated as complete)

| Priority | Gap | Concrete acceptance target |
|---|---|---|
| **P0** | One `factory.name`, `headcount`, and `sewing_count` are inadequate for a group of small/medium/large plants. | Tenant-owned **plant/site ID**; plant-specific machine, people, shift, cost and capability records; authorized cross-plant summary without mixing inventories, orders or permissions. Requires schema/auth design and a migration. |
| **P0** | Working-machine count and total production headcount do not establish available operators, uptime, bottleneck SAM/SMV, changeovers, or usable capacity. | Source-backed machine/station status + qualified operators by shift + downtime + operation-route SAM. Report **capacity unknown** until complete; verified capacity based on the bottleneck, not total sewing machines. |
| **P0** | `costs.labor_rate` and `costs.overhead_rate` say “per hour” without a governed numerator/basis; the separate reference catalog already distinguishes rate bases. | Capture rate basis, currency, effective date, costing allocation method, and revision. Prevent margin or quote calculation from these ambiguous discovery values. Do not silently reinterpret old records. |
| **P0** | Dev Basic credentials and `dev` tenant are not production-grade access controls, especially for worker records. | Per-role authenticated approval, plant-tenant scoping, PII retention/consent, upload restrictions and audit, adversarial tenant-access tests **before** real payroll or named employees. |
| **P1** | Limited current view of older document evidence. The fix preserves lineage, but the dropdown is still a newest-40 view. | Authenticated, paginated source search by ID/filename/version and click-through document evidence; distinguish stale source from missing data. |
| **P1** | Manager cannot spot unusually costly idle capacity or quote small fill-in jobs across facilities. | Read-only, evidence-grade opportunity screen: available staffed hours by *qualified operation*, material availability by verified lots, order contribution with freight/rework/lead-time risk; expose assumptions, source versions and confidence. No automatic order acceptance. |
| **P1** | No operational conversion from these 17 discovery observations into governed factory master records. | Human-reviewed publication to normalized, effective-dated machine/skill/calendar/rate masters, with withdrawn/changed versions and no source-to-ledger bypass. |

### Economics and data truth

Never infer garments/day from `machine_count × shift_hours`: **machine-hours are not garment throughput**. A credible estimate needs worker-skill match, station-specific utilization, operation SAM, line balance, rework and material readiness. Nor can a factory profitably accept an opportunistic order just because a line looks idle: calculate *incremental contribution* (incremental revenue minus all avoidable costs, logistics, quality risk and opportunity cost) against its due date and capacity constraint. Until these inputs exist, show *unknown*, not zero or an invented green profitability badge.

### Chapter 01 operator acceptance

1. Start with only factory name; see provisional versus confirmed facts clearly. Uploads remain available regardless.
2. Read readiness of draft capacity/costing/material **without opening advanced review**; see why facts are missing and that no operational approval was granted.
3. Skip an uncertain question without writing `UNKNOWN`, `0`, an invented rate or a confirmation. Revisit skipped items in this session.
4. Finish all verified fields; wizard shows completion rather than repeating factory name.
5. Edit a fact linked to a document older than the 40 recent results; its linked `source_document_id` remains preserved on update.
6. Review & Submit opens the existing Documents review flow. All original append-only revision, stale-write, tenant and material authorization gates stay in force.
7. Do not introduce payroll/named employee files on the development login.

### Verification limitations

- Browser guidance and existing static UI contracts: Node and standalone Go web tests executed in this sandbox.
- Full server Go tests/build and PostgreSQL integration: **must run on an environment with Go 1.24 and configured PostgreSQL**; sandbox contains Go 1.23.2 and cannot retrieve the toolchain. Do not claim those tests passed here.
- Docker rebuild, live application smoke test, and Git commit: run on your iMac per installation instructions; no live deployment was performed here.
