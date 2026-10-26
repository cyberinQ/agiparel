# Agiparel Factory OS — Gate 06.5-B3

**Source-supported document routing and versioned factory-onboarding conflict review**

## Purpose and scope

This increment builds on Gate 06.5-B2. One centralized Document Inbox accepts customer and factory files. The system helps the factory understand *what* a document might contain, *where* the relevant evidence was found, and *whether* two sources agree. No source interpretation is treated as automatically verified.

### Added capabilities

1. **Possible document purpose:** deterministic two-signal labels for machine registers, worker skills, stock observations, rates, buyer orders, tech packs, operation bulletins, and packing rules. Grounded in extracted segment ordinal. Filename alone never assigns a category. Categories are suggestions, not a guarantee that all contents were parsed.
2. **Factory draft grouping:** B2's `ACCEPT_PROVISIONAL` registers are grouped by `(tenant, entity type, logical entity key)`. Canonicalized JSON attributes distinguish true disagreements from formatting-only differences.
3. **Human resolution:** select an uploaded source, mark the onboarding draft as reviewed or needing clarification, explain conflicting facts. Conflicts require at least 12 characters of explanation. Subsequent changes produce a new revision, not an update to an old decision.
4. **Source links:** deep links lead to exact source-segment locations in the Document Inbox.
5. **Progressive onboarding:** unfinished or conflicting records do not prevent document upload or exploratory factory setup; they do not authorize inventory issuance, payroll, costing, or production.

## Database: migration 0012

New `factory_draft_decisions_v65b3` stores `(tenant_id, entity_type, entity_key, revision)` as an append-only revision key. Each reviewed decision points to an already accepted B2 source candidate by FK. PostgreSQL RLS is forced using `app.current_tenant_id`. App role has only `SELECT` and `INSERT`; no `UPDATE` or `DELETE`.

This table is **not** a new operational factory master. It is a source-linked onboarding review history. Controlled promotion to authoritative machine, workforce, material and costing models remains a subsequent gate and must enforce domain-specific business prerequisites.

## API contracts

- `GET /api/v1/document-triage?document_id=DOC-...` — `possible_categories[]`, `source_status`, `review_required:true`. Handles queued/unsupported files without fabrication.
- `GET /api/v1/factory-onboarding/draft-masters` — grouped provisional evidence, canonical conflict counts, latest decision revision, truncation indicator, source references.
- `POST /api/v1/factory-onboarding/draft-masters/review` — `entity_type`, `entity_key`, `candidate_id`, `decision` (`VERIFY_DRAFT` or `NEEDS_CLARIFICATION`), `expected_revision`, `note`. Requires B2 `ACCEPT_PROVISIONAL` and uses per-tenant advisory lock plus optimistic revision check. Responds `409` for changed revisions, unresolved short conflict explanations, or unaccepted evidence.

## Sample operator journey

1. Upload `examples/factory-onboarding/06_conflicting_machine_register_A.csv` and `07_conflicting_machine_register_B.csv` through **Documents**.
2. Wait for extraction; Document Inbox suggests **Possible Machine / equipment register**, with source links.
3. In **Factory Setup**, scan both uploaded documents and select **Keep as provisional** for the machine rows.
4. Open **Resolve conflicting factory information**. The two `MC-101` observations disagree about machine type. Select the trusted original, explain which evidence was checked, then select **Mark reviewed draft**.
5. Confirm latest decision revision is shown, and both original source variants are still visible. Nothing has updated the operational machine register.

## Limitations (intentional)

- Classification is deterministic and evidence-bounded, not comprehensive LLM document interpretation. Complex scanned PDFs, legacy XLS and cloud-native links still need their corresponding converters/connectors.
- Source parsing is not yet a complete semantic extraction model for arbitrary tech packs, worker records or multi-PO packets.
- Machine capabilities, ownership, skills, inspection, available stock, and effective cost rates still require domain-specific verification.
- Since the current development credential is intentionally weak (`dev`/`1212`), keep the environment disposable and segregated from real employee personal details, payroll and operational authority.
- If the proposed-register list exceeds 500 rows, the UI warns that its conflict display is incomplete and disables review actions. The backend validates decisions against the full relevant group before committing.

## Next: Gate 06.5-B4 / C

Move reviewed onboarding drafts into *versioned, domain-specific, operationally gated master records* after machine identity, skills assessment, stock inspection, rate effective-date and ownership checks. Then extend buyer document classification into actual multi-PO/BOM/tech-pack extraction, field-by-field review and approved order handoff. Avoid parallel isolated upload channels and avoid a premature general-purpose LLM SQL generator.
