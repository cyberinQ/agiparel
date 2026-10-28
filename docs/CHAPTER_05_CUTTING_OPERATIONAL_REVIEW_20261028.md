# Chapter 05 — Cutting, Spreading & Bundle Genealogy · Operational review

Author: Sunil Jain · 2026-10-28

## Operating boundary and evaluation

Reviewed the Chapter 05 lay-draft, measured-spread and release-status browser flows; server draft, quantity, spread and signed-reader/release rules; the Chapter 05 SQL tables/constraints; and the buyer-PO/stores interfaces. **A draft is never an executed cut. A spread report is operator testimony with signed-issue attribution, not scanner-proof cutting. Signed reader events and two independent authenticated approvals are still required for sewing release.** The changes below do not bypass or replace these controls.

### Implemented now (P0/P1)

| Control | Previous operational problem | Correction / acceptance evidence |
|---|---|---|
| Order-level SKU cutting balance | Operator saw ordered quantity but no aggregate plan balance across cuts | Read-only source matrix now shows ordered, latest-revision draft-planned and unplanned units by color/size. Source quantities remain server-controlled. |
| Physical lay color integrity | One lay accepted marker positions for multiple garment colors while specifying one fabric SKU and shade | Reject mixed color codes at the API validation boundary; multiple sizes in one lay remain valid. A different color needs a different cut/lay. |
| Revision preparation | Revising an existing lay meant manually reconstructing marker positions and pattern parts, leading to errors | One explicit **Prepare next revision** action reconstructs data only from a complete, latest saved snapshot; older snapshots cannot overwrite current revisions. Server blocks revision when spread evidence already exists. |
| Cutting capacity honesty | Register displayed a limit of 60 latest plans without saying totals were partial; “latest plan” label summed multiple plans | API provides `has_more`, UI states the register is capped, and labels aggregate *shown draft* quantities rather than physical production. |
| Stale evidence protection | Changing buyer PO or cut number left another lay's bundle, spread, reader and release evidence visible | Selection reset invalidates old requests, disables spread/print/revision controls and clears release badge; late responses cannot reassert old evidence. |
| Spread consumption economics | Minimum physical spread requirement and unattributed signed-issue capacity were not obvious | Live mm/m breakdown compares measured marker × plies, entered used fabric and open signed-issue attribution; client checks quantities and exact allocation sum, while server remains authoritative. No client-generated fabric consumption. |
| Network retry integrity | Every retry regenerated a request key, causing avoidable revision conflicts or ambiguous operator outcomes | Identical draft/spread resubmissions retain the same idempotency key; edits and successful submissions generate new keys. |

No schema changes. Existing layout, typography, Chapter 05 burnt-orange color tokens, steps, and control hierarchy are retained. Two modest text readouts and one revision button are added; no new modals or navigational changes.

## Deferred architectural deficiencies — deliberately not fabricated

1. **P0 — Plant-local material and cutting authority.** Existing cutting lays are order-scoped and have no governed plant/table assignment. Signed issues may correspond to different sites. Introduce explicit plant, cutting table, spread crew and authoritatively reserved roll custody; prevent cross-plant material attribution without a signed transfer. Requires schema/authorization work and backwards-compatible migration.
2. **P0 — Machine-verifiable marker fit and fabric utilization.** Marker code, width, length and color are operator-entered. Do not claim CAD marker efficiency, verified fabric width, nesting/defect avoidance or true yield from these fields. Add approved-marker file/hash binding, width tolerance, usable roll width, defects, cut-table capacity, fabric relaxation, and machine-compatible marker verification.
3. **P0 — True cut yield and quality reconciliation.** Track attempted vs accepted pieces, failed QC, recuts, shortage claims, shade mismatches, production conservation and accountable WIP handoff. Never equate planned garments with first-pass accepted units.
4. **P1 — Cutting campaign economics.** Optimize across orders within approved plant/lot/width/shade constraints for actual remnant value, marker yield, cutter queue, setup cost and delivery penalties. Compare incremental contribution margin and working-capital costs; do not promise numerical savings without actual data.
5. **P1 — Large-factory pagination and drill-down.** Add cursor-based latest-revision plan queries and filtered registers; the present 60-row cap is surfaced instead of inventing a factory-wide total.
6. **P1 — Exception-led control tower.** Queue spreads with unattributed signed fabric, expected-versus-observed ply variance, missing EPC witnesses, failed gateway attestation and dual approval waits; track age, owner and service levels by plant.

## Tests and authorization

- New Chapter 05 Node regression: capped register labels, latest revision preparation, order-context invalidation, stale release requests, immutable retry keys and exact signed-mm checks.
- New Go UI contract test; existing tests unchanged; quantity validator test includes pre-existing overplanning and a mixed-color lay.
- Run on iMac with Go 1.24: `go test -count=1 ./...`, `go vet ./...`, `go build ./...`, `node scripts/test_cutting_ch05_workflow.cjs`, then all `scripts/test_*.cjs` scripts. Docker build/deploy only after those succeed.
- **Migration: none.** No signed stores/cutting/release event, hash, historical snapshot or RLS policy is modified.
