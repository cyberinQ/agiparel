# Agiparel Gate 06.5-B4 — Versioned Factory Reference Catalog

**Architecture:** Factory-first, progressive onboarding with original-document evidence. This gate is an incremental step between B3 reviewed drafts and future operational master authority. It does **not** promote evidence into production permissions, certified capabilities, the inventory ledger, payroll, committed costing, or purchasing. The public website remains a clearly labeled illustration; only the authenticated Factory Setup workbench handles user records.

## Customer workflow

1. Upload original machine lists, skill registers and rate sheets to `/documents`. The same central inbox serves buyer and factory documents.
2. In `/factory-setup`, use **Find possible factory records**. B2 proposes structured, source-anchored candidates and a user chooses **Keep as provisional**. Unrecognized evidence can be handled manually through factory discovery; unknown values are not fabricated.
3. In **Resolve conflicting factory information**, select the right source and create a `VERIFY_DRAFT` revision, explaining conflicts in ordinary language. B3 preserves the alternatives and notes.
4. In **Factory reference catalog** (this release), inspect the reviewed source, confirm the evidence-to-reference boundary and record a verification explanation. For a rate, explicitly choose per hour, piece, metre, yard, kilogram, day or unit. Click **Publish factory reference**.
5. The catalogue displays the source, current action, version and explanation. An obsolete reference can be **Withdrawn**, appending a new version. Previous versions remain inspectable.

A factory can begin with partial information. Publishing a reference is optional and does not block document upload, guided discovery, exploratory planning or adding buyer documents.

## Data contracts and authority boundaries

- `factory_reference_versions_v65b4` (migration `0013`) is tenant-isolated with RLS and an append-only, role-restricted event history. Primary key `(tenant_id, entity_type, entity_key, revision)`.
- Eligible types: `MACHINE`, `SKILL`, `RATE`. **MATERIAL is deliberately excluded.** An opening stock sheet does not prove inspected usable stock or ownership; physical stock belongs to Batch 07 ledger authority.
- Each version references the **specific B3 draft revision** and B2 candidate source, which is tied through document segments to original source bytes. Rate basis must be explicitly provided by the operator.
- A publish is refused if the selected draft is not the latest `VERIFY_DRAFT`, if the candidate differs, if a more recently accepted source appeared since review, if source attributes are invalid, if the reference version is stale, or if the explicit acknowledgement is absent.
- Withdrawal appends `WITHDRAW` and cannot overwrite prior values. Both publish and withdraw require a reason of at least 12 characters.
- All writes use the tenant-scoped restricted application role and transaction-level advisory locks/optimistic version checks. The document-intake worker cannot publish reference records.
- The current catalogue is derived by selecting the latest revision per type/key; history uses the same table. Records expose **reference** status only, never operational authorization.

## Known limitations and next steps

- Strictly extracted B2 CSV/XLSX columns currently produce the most dependable structured candidates. Scanned PDFs, legacy XLS, drawings and narrative tech packs still require robust OCR/extraction and human semantic review.
- We do not infer a machine's certified capabilities from its name; neither workforce availability nor proficiency authorization is proven by a skill-sheet row.
- Rate entries are source-specific quoted references; the final costing engine still needs verified effective dates, currencies, conversion policy, labor/overhead scope and approvals.
- This gate does not rewrite Batch 05 buyer POs or material scenarios; customer files do not automatically become approved business entities. Factory-to-buyer-order integration remains Gate 06.5-C.
- Users with no source documents can continue recording B1 provisional/confirmed discovery facts; connecting independently verified manual facts into the same versioned reference workflow is future work.
- HTTP Basic with `dev`/`1212` is still development-only and not appropriate for sensitive customer, employee or payroll information on a public deployment.
- Database integration test is opt-in and skipped by ordinary `go test ./...` unless all required variables are provided.

## Acceptance test examples

- After reviewing `MC-101` as a Lockstitch machine from an uploaded CSV, publish revision 1. The catalog lists a Lockstitch reference, with a link back to the exact source segment.
- If a competing machine register is accepted after the draft review, publishing must fail with a prompt to re-review newer evidence.
- Attempting to publish `MATERIAL` must fail: no stock balances are altered.
- A `RATE` without a verified basis is rejected. A rate of `=SUM(A1:A9)`, a negative amount, or nondecimal source value is rejected rather than silently calculated.
- Withdrawing an existing reference creates revision 2; requesting revision 1 in the UI shows the original explanation and evidence.
- Stale repeated submit fails with a conflict; another tenant cannot read records through the restricted app role.
