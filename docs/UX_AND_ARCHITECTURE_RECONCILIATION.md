# Agiparel — Customer experience and architecture reconciliation

**Baseline:** cleaned source at `97e4cc3`, master plan v5.3, migrations 0001–0025.  
**Scope of this delivery:** presentation, route guidance, words, one shared light theme, public-home truthfulness, and UI regression checks. **No database migration, business rule, role, audit rule, simulated-scan contract, or authorization gate was changed.**

## Customer experience decisions

- **Factory first.** Start with machines, people, skills, stock and what is known. Upload files at any time; orders and planning are next. Missing noncritical facts do not prevent learning about the factory.
- **One upload inbox.** `/documents` for PDFs, Word, Excel and CSV; `/order-intake` is an advanced paired-CSV path, not a competing ordinary upload step.
- **One interface family.** High-contrast paper/ink background, common top bar, common inputs, keyboard focus, touch-sized controls, current department name, Previous/Next area, and collapsed specialist tools. Page accents distinguish departments; green/amber/red still mean Pass/Hold/Reject regardless of department.
- **One short job per screen.** Step 1 / Step 2 / Step 3 wording; standard garment terms: buyer PO, cloth, lay, ply, bundle, sewing job, quality check, buyer tag, carton, dispatch list. Required codes, units and specialist controls remain intact.
- **No manufactured success stories.** The **public Home no longer embeds made-up orders, timestamps, RFID reads, quantities or throughput**. It is a navigation-only introduction, not a dashboard with real factory data.
- **Honest simulation.** A button tap remains `UI_SIMULATED`; it is not a physical reader event, a worker's signed identity, a production credit, or real dispatch approval. The interface can stay simple without treating simulation as authoritative proof.
- **Existing specialist workspaces stay reachable** from More tools: paired CSV order intake, line planning, floor board, old setup help and old station demo. Old `/onboarding` no longer pretends to do production planning.

## Customer routes

| Primary page | Route | Purpose | Accent |
|---|---|---|---|
| Factory setup | `/factory-setup` | Add and check what is known | Sky |
| Upload files | `/documents` | One place for all buyer/factory files | Indigo |
| Buyer order | `/buyer-review` | Check PO, colors, sizes, BOM | Blue |
| Plan material | `/material-planning` | Check possible stock and buy list | Violet |
| Fabric store | `/stores` | Inspect rolls, reconcile, request approval, pick | Amber |
| Cutting | `/cutting` | Plan lays, spreads and bundle status | Orange |
| Sewing | `/sewing` | Simulated station scan and operation cycle | Teal |
| Quality | `/quality` | Per-piece check, repair, finishing, buyer tag | Green |
| Packing | `/packing` | Cartons, manifest and simulated dispatch | Rose |
| Factory report | `/intelligence` | Count, delay and exception reports | Slate |

The order of a **navigation link** is guidance, not authorization. Server-side custody, scanner, signed identity and release checks remain mandatory.

## Verified technical gaps to address in a separate engineering release

**P0 — Trust boundaries and single authority**

1. **Overlapping legacy and new tables.** `0001_baseline_zero.sql` defines operational tables such as `store_issuances`, `bundle_registry`, `hangtag_binds` and `carton_pack_manifests`; migrations 0005–0025 add versioned material, cutting, sewing, quality and packing evidence. Audit actual read/write owners, define one authoritative state per aggregate, and build tenant-safe *read projections* without treating provisional stage events as approved stock. Do not drop either schema or create unscoped joins.
2. **Identity split.** `services/s01-identity/migrations/0001_s01_s02_foundation.sql` creates `s01_identity.tenants` and `s02_device.registered_devices`; `services/core-api/migrations/0001_baseline_zero.sql` uses `public.tenants`, while `0021_cutting_reader_events_and_release_v8.sql` uses `public.cut_reader_gateways_v8`. Define a canonical tenant/device/worker identity mapping and a trust/audit contract. A cross-schema foreign key by itself will not reconcile different storage or trust domains.
3. **UI simulation versus physical proof.** `/sewing`, `/quality`, `/packing` record button events; `/cutting` still requires signed reader and custody evidence. The future hardware adapter must authenticate reader/device and apply RF anti-replay, tag uniqueness, station association and authority policies. **Do not turn on real inventory/production credit by changing `UI_SIMULATED` to `HARDWARE` in the browser.** Test rejection/hold cases end-to-end.
4. **Public access and authentication.** The dev Basic login is inappropriate for real employee or buyer data. Enforce strong human identity and permissions before pilot production; avoid exposing employee records and audit authority through test credentials.

**P1 — Calculation and physical process correctness**

5. **Hardcoded cloth estimate.** `services/edge-daemon/pkg/planning/analyzer.go` computes `neededMeters := float64(targetCut) * 1.25`. The actual production path must use reviewed BOM, approved consumption/units and approved marker details, not this fixed number. Fence legacy estimates as examples or remove when callers migrate.
6. **Illustrative marker optimizer.** `services/edge-daemon/pkg/cutting/geometry_optimizer.go` uses `standardTrouserParts` in `Optimize2DMarkerAndVision`; it cannot be treated as a real buyer CAD/Gerber marker. Use a source-verified marker revision for factory decisions. If retaining a demo, label it illustrative in its API and screens.
7. **Duplicate domain packages.** Identical top-level/root copies include `cutting/planner.go`, `rfid/sgtin.go`, `stores/indent.go` and `stores/issuance.go`. `sewing/dwell.go` and `attachment/attachment_graph.go` differ between root and `services/edge-daemon/pkg`. Map imports and tests, choose canonical owners, merge differences, then remove duplicates safely. Never delete root `pkg/` without updating Dockerfile `COPY pkg/ pkg/` and service imports.
8. **Document formats.** `services/document-intake/extract/extract.go` returns partial status for scanned/image PDFs and unsupported for `.xls`. This is a real intake gap relative to v5.3's OCR/legacy-file vision. Keep originals and clear partial status; implement tested conversion/OCR rather than claiming every file can be read automatically.

**P1/P2 — Workflow maturity and manager value**

9. **Buyer-specific packing controls still incomplete.** v5.3 specifies bag identities, approved pack ratios, buyer carton rules, controlled repack, verified serial membership, carrier proofs and dispatch signoffs. The current packing flow is a software demonstration only. Confirm whether buyer label is per-garment unique (UPC often isn't).
10. **Acceptance versus observed throughput.** Stage counts and duration reports have recorded-time limitations and simulated inputs. Shift-aware queue time, verified yield/scrap and accepted output should be separate typed measures before managers rely on DPP, costing or order closure.
11. **Local language and low-end device testing.** Plain English is a start, not literacy support. Pilot with Hindi/Bangla or relevant site languages, clear icons, native input, large targets, readable status and poor-connectivity behavior; do not auto-translate safety-critical terms without review.
12. **25-epic completeness is not proven by 25 migrations.** Master plan v5.3 contains more than the demo of cutting→sewing→quality→packing. Track each epic against code path, production authority, tests and factory validation. A working simulator is **not** full operational acceptance.

## Acceptance for this UI release

- Every page references shared navigation and stylesheet, except public Home which uses its own matching light layout.
- Existing controls/DOM IDs and JS API calls remain unchanged; no server permission or database layer changes.
- No roadmap batch/gate numbers appear in customer HTML.
- The public Home contains only a truthful journey and no fabricated order data.
- Existing `_ui_test.go` safeguards are kept; new customer consistency tests extend coverage to all 14 restricted pages and public Home.
- Full Go 1.24 tests, browser smoke testing, keyboard/touch checks and real operator observation are still required on the development iMac before acceptance.
