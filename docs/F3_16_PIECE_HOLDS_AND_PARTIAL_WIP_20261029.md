# F3.16A — One-piece holds and partial-bundle WIP
Author: Sunil Jain, CyberinQ LLC, 2026-10-29.

## What is functional
- POST `/api/v1/wip/pieces/challenge`, POST `/api/v1/wip/pieces/events`: strong, current signed operator `HOLD` and distinct supervisor `REVIEW` actions on **one permanent garment ID** (one marker position + ply). Each operation requires provisioned action grants and assigned station; event IDs and proof IDs are immutable and replay guarded.
- Each held garment's **original and recut cut parts** are stopped at the database WIP movement boundary. Other garments in that same physical bundle are **not** blocked by this new hold. Existing F3.10 exception holds continue independently.
- Review requires a fresh, signed, station-scoped, current carrier-bound WIP scan of a constituent part; it never fabricates a custody transfer, reverses a cut disposition, certifies repair or grants operation credit.
- GET `/api/v1/wip/pieces/groups?tenant_id=...&plant_id=...&lay_draft_id=...&marker_position=...&ply_group=...`: scoped, authenticated **identity-reservation** counts and active holds. Missing IDs are shown as missing, not fabricated. Reserved IDs are not physically verified garment output.
- SQL holds block QUALITY_RELEASE for the affected garment; the existing whole-ply-group sewing completion remains blocked while *any* constituent is held. This is a deliberate anti-phantom-credit rule.
- Migration 0045 is forward-only and follows 0044. New flag `AGIPAREL_PIECE_HOLDS_ENABLED=true` only in configured staging/production with existing WIP, genealogy, cut reconciliation, IdP, grants and PG.

## What is NOT done
- No split of a `sew_prod_completions_v10` aggregate into 24 individualized sewing completion credits. That legacy table still uses the entire ply group. F3.16B must introduce a per-unit signed completion ledger with observation consumption and per-unit predecessor DAG checks before 24 can count as produced.
- No RF precision certification, physical repair certification, payroll credit, stock mutation, automatic recut or QC release.

## Example of real group
Original bundle 25 plies: 25 reserved garment IDs; one signed HOLD => `reserved_garment_identities=25`, `held_garment_identities=1`, `unheld_reserved_identities=24`. These are **identity counts only**, not verified physical output or operation credit. Existing part-level signed F3.3 custody handoffs for the 24 remain permissible if all their independent route, part acceptance, reader and workforce constraints pass.

## Operational acceptance (staging / witnessed)
1. Provision `sewing.piece.hold` and `sewing.piece.review` to different enrolled people assigned to the relevant station.
2. Configure existing F3.3 cut-part acceptance and reader enrollment, and enable the feature flag only in staging.
3. Open a HOLD on one genuine `gen_garments_v18.id`; verify a movement for one of its parts fails with `ERR_F316_PIECE_HOLD_MOVEMENT` while another garment's independently signed movement can proceed.
4. Verify affected garment QC release and whole-bundle completion are blocked. Verify no completed pieces are auto-credited.
5. Present a new gateway-signed observation for a constituent part *after* the HOLD, then have an independent supervisor sign the exact REVIEW challenge; verify it clears the administrative hold, not physical repair or QC decision.
6. Check tenant RLS, proof replay, stale/wrong-station scans, scanner misbinding, and concurrent movement/review races.

Do not run real production-credit simulations on production data. Rehearsal should use a disposable cloned database and no live factory tags.
