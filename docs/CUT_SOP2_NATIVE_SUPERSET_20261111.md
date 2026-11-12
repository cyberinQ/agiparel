# AGIPAREL Cut-SOP-2 — Native Superset: Architecture, Executable Controls and Acceptance Gates

**Author: Sunil Jain · CyberinQ LLC · 2026-11-11**  
**Source baseline:** uploaded `agiparel(20261111-153943).zip` and `Cut-SOP-2.md`, whose own title is **Cut-SOP-1 (Rajesh, 2026-11-10)**.

## Executive design choice

**One AGIPAREL cutting system, not a competing application.** Existing PO/BOM intake, `store_rolls`, `cut_lay_drafts_v8`, `cut_spread_reports_v8`, signed `cut_reader_events_v8`, `cut_sewing_releases_v8`, and production genealogy remain the single sources of physical transaction truth. The SOP-2 engine supplies *advice* and versioned extra-quantity decisions; it never synthesizes signed scans, inventory issues, machine evidence or approvals.

### Flow

```
Buyer PO + BOM / Stores roll passports        Trusted immutable source records
         |                     |
         v                     v
Defect/width/shade/relaxation policy      Approved CAD marker and cutter envelope (pending)
         |                     |
         +----------+----------+
                    v
         Extras band → discrete target optimizer → frozen per-SKU version
                    |                              |
Frozen sewing plan / style lead → capacity scheduling → lay-draft quantity limits
                    |                              |
              Assigned rolls / reserve / endbits (advisory until admitted)
                    |
      Existing signed fabric issue → measured spread → physical EPC witness
                    |                         |
      Per-ply part genealogy / NFC marriages / same-roll recuts (further wiring)
                    |                         |
           Existing independent signed release + QC / sewing WIP
                    |
       Exact-size balances / discrepancy alerts / versioned impact approvals
```

### Built in this patch

| SOP requirement | Code/evidence | Deployment authority |
|---|---|---|
| Dynamic extras, frozen per-size targets, PO aggregate bands | `sop2/targets.go`; `cut_sop2_target_versions_v46` and SKU rows; frozen target used by existing draft quantity validator | First version persistently frozen in **development** after explicit flag; subsequent changes blocked |
| Constrained fewest-cuts / least-fabric / time search | `sop2.SolveTargets`, bounded discrete optimizer, explicit `optimality_proven` and `search_truncated` | Advisory; physical markers/rolls not automatically certified |
| Exact balance and overcut rejection | `sop2.Remaining`; source-linked control-tower draft vs spread vs independently released counts | Signed releases are *not* QC-accepted pieces |
| Accepted roll geometry and defect policy | `sop2.InspectRoll`, append-only inspection evidence and versioned acceptance policy | Records inspection; **does not mark store stock PASS** or issue materials |
| Shade/width/plant segregation, reserve/endbit pool matching | `sop2.AllocateRolls`; multi-roll bounded global combinatorial search, unverified status disclosed | Advisory; one roll to one lay conservative model |
| Same-roll recut and fabric reconciliation | `sop2.ValidateRecut`, `sop2.ReconcileFabric` | Pure controls; physical reserve transactions still need integration |
| Sewing-driven cut proposal with lead days and line-stock priority | `sop2.SuggestDailyCut` | Advisory, as authoritative frozen sewing plans / capacity master are not yet present |
| One garment/ply, complete sub-bundle, family marriage and NFC binding | `sop2.VerifyFamily` with explicit connected marriage graph | Simulation only; signed EPC release unchanged |
| Stakeholder-complete version change logic | `sop2.EvaluateChange`; append-only change-request/approval schema | **No production approval endpoint** until authenticated identity and approval policy are integrated |
| Operator control tower | `/cutting/sop2`, source-backed `/targets`, `/control-tower`, solver and other guarded endpoints | Restricted to existing developer operator environment |

### Why v8 signing remains untouched

The current physical release path requires matching human EPC witnesses, authenticated gateways, signed issued fabric and two independent authorized release signatures. This patch never inserts into those physical or signed evidence tables. A demo user cannot mint a real scan merely by submitting JSON. Physical release status and quantity summaries are always labeled separately from drafts and operator reports.

## High-priority completion roadmap (not claimed complete)

1. **P0: Commissioned factory master data and trusted optimization inputs.** Load approved marker DXF/ASTM/PLT files and SHA-256, exact part polygons, cutter clearance, table length, width-loss/defect avoidance, machine knife-height and per-fabric ply caps. Establish authenticated decision roles for factory fabric/lead-time policies. The existing heuristic optimizer is not CAD nesting; a marker must be physically machine-verified.
2. **P0: Frozen sewing demand source.** Add authoritative sewing line-loading, lead-time and live cut-stock master data (these do not exist as frozen planner records in the provided schema). Connect to scheduling with version change propagation and deadlines. Preserve pull-based line stock; never treat the simulation's `frozen:true` flag as evidence.
3. **P0: Physical roll allocation and serial material conservation.** Add plant-scoped, signed reservation of roll intervals, defect/width/shrinkage matching, plies-per-roll genealogy, reserve remnant identities, reissue/return custody, endbit cuts, and mutation-safe conservation. The in-memory global search is only a decision proposal.
4. **P0: Actual-plie adaptive replan.** Existing spread endpoint rejects plies differing from planned draft. Build a supervised variance event that revises the draft *before* physical reporting, regenerates family identities, checks all remaining per-SKU targets and keeps old versions for audit. Never allow partial-physical bundle manifests to be released.
5. **P0: NFC marriage and complete sewing handoff.** Enroll real hardware NFC UID and EPC mapping with independently authenticated staff, append-only tag bind/unbind lifecycle, required family edge set and per-ply same-roll join checks. In the authoritative release transaction, require complete sub-bundle reader witnesses, matching marriage graph and correct line. Never treat browser scans as authority.
6. **P0: Authenticated multi-role change approvals.** The required roles depend on impact: planning, cutting, fabric stores, sewing lines, merchandising, sometimes process/QC. Provide affected-person diff, cryptographic proof IDs, one distinct identity per applicable role, and atomic frozen-version promotion. Do not unfreeze on the strength of dev actor text.
7. **P1: Production-grade global solver.** Combine integer programming / column generation (markers, sizes, cuts, plies, rolls, time, remnant inventory and setup costs). Deterministic budget and solver certificate; if no proven optimum, show feasible incumbent and provable gap. Add daily incremental re-optimization, scenario comparisons and human approval.
8. **P1: Exception and economics intelligence.** An auditable material-use ledger, endbit value, factory capacity and bottleneck alerts, delivery-risk simulation, scrap/profit opportunity, and real-time issue aging by role, with baselines/observed evidence—never claim fabricated savings.

## Eight factory policy decisions still open in Rajesh's SOP

The supplied SOP explicitly leaves these unresolved. Configure rather than guess: fabric defect-point acceptance standard, fabric-type relaxation hours, cutting-capacity unit and measurement, maximum plies per fabric/cutter, per-line minimum cut stock, short-lay/endbit rules, NFC tag reuse/unlinking procedure, and stakeholders per frozen-plan change type.

### Acceptance criteria (production gates)

- **Exact targets:** each ordered SKU + approved extra band yields one frozen integer; sum within PO extras band; no confirmed negative balance; final reconciliation exactly zero (not inferred from drafts).
- **Marker truth:** approved CAD hash, widths and cutter capacity match every actual lay; defect and shade group match tested fabric; relaxation is time-stamped.
- **Physical material:** every used roll is signed-reserved to the right plant/lay; cut length + used/returned reserve + endbits + defect cutouts exactly equals signed issued quantity, in consistent units; no duplicate custody.
- **One ply / garment:** every part for one garment/ply has one source roll/lot and stable serial; recuts preserve original serial and source roll, never inflate accepted quantities.
- **Complete handoff:** missing, duplicate, foreign, stale or unbound NFC/EPC witnesses block issue; family marriage is closed and auditable; line cut stock updates only once upon signed receipt.
- **Change control:** every frozen input has a version, diff, impact and independently authenticated approval from *every* affected role; unilateral edits are impossible.
- **Quality:** first-pass cut yield, defect-recut count, final accepted garment count and PO shipment tally are separate, source-backed numbers.
- **Trust:** tenant isolation, idempotency, no implicit cross-plant sharing, no unsigned privilege escalation, and negative tests for every gate.

## Local build and test

1. Apply the patch with the supplied manifest-checking installer, not an unguarded `unzip -o` into a changed working tree.
2. Run the Go 1.26 application build and all repository tests on the user development host. For standalone policy code, Go 1.23 is sufficient to run the contained no-dependency tests; this is not a substitute for compiling the full server.
3. Before installing PostgreSQL migration `0005_cut_sop2_control_plane_v46.sql`, back up the existing database. Migration is additive; no reset and no historical v8 mutation. Validate tenant policies and app-role read/write access in staging.
4. Run Node source/route tests and browser JS syntax check; start Docker Compose staging and verify `/cutting/sop2` is behind existing operator authentication.
5. To enable initial **development-only** target writes, set `AGIPAREL_CUT_SOP2_WRITES_ENABLED=true` and restart `plant-edge-os`. Default is `false`; no production writers are enabled.
6. Commission defect policies per SKU in `cut_sop2_inspection_policies_v46` and audit the process. A factory policy record and operator name are not production signatures.

## External communication principle

Share the capability matrix and working AGIPAREL demo with Rajesh as a common implementation target. Invite him to refine the constraints and owner responsibilities. The source does not establish that he is building a competing implementation; maximize interoperability and single source of truth rather than treating suspected duplication as a fact.
