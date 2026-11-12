# AGIPAREL v49 — Worker guides and Aggie capability audit

Author: Sunil Jain, November 12, 2026. Source baseline: `cyberinQ/agiparel`, commit `f900a2e`.

## What v49 adds

- A single, embedded, verified source of **all ten worker chapters**: `services/edge-daemon/internal/workerhelp/guide.json`. It includes the exact button names, six steps, what to prepare, what counts as done, and when to stop.
- A collapsible **Show simple worker instructions** block on every primary chapter (01–10), the spreadsheet order intake, the cutting control tower, and the production-report variant.
- A shared **Ask Aggie** widget on chapters that lacked it. Existing chapters keep their original chat widget and session continuity.
- Authenticated `GET /api/v1/agie/manual?chapter=1..10` for one guide, or `GET /api/v1/agie/manual` for all ten. It is intentionally **not a public route**. It contains instructions, not private factory data.
- The chat's procedural questions are answered directly from the embedded guide, **even if the database or Ollama is offline**. For less obvious questions the model gains the `read_worker_manual` tool. It cannot use a manual answer as proof of live production.
- The browser supplies the current chapter path with the question. The backend accepts only user/assistant messages, rejecting forged system/tool messages.

## Aggie — what is actually connected today

| Capability | Code evidence | Boundary |
|---|---|---|
| Review entire worker guide | `workerhelp`, `read_worker_manual` | Yes, complete ten-chapter source. No live-data claim. |
| Answer specific chapter how-to | `workerhelp.Match`, `workerhelp.Answer` | Deterministic for recognized questions; current chapter used only as a hint. |
| Answer selected direct factory questions | `operational_queries.go` | Factory name, worker/machine list, capacity caveats, attendance, in-progress POs. Tenant-scoped SQL; requires connected DB and authorized path. |
| Inspect SQL schema | `interviewer/tools.go`, `sqlguard/inspector.go` | Existing `inspect_db_catalog` tool; public-schema metadata, no writes. |
| Query factory records | `run_safe_sql`, `CatalogInspector.ExecuteSafeSQL` | One read-only `SELECT`, limited to 100 rows and 5 seconds, scoped by current tenant; access remains limited by DB role/RLS. |
| Broad natural-language answers | Ollama `AGIE_MODEL` or configured cloud fallback | Conditional on model availability, query design, permissions and verified evidence. It does **not** guarantee every question can be answered. |
| Update stock, approve a PO, execute cutting, release QC, ship cartons | Not part of Aggie's tool list | **No.** These actions require separate signed/approved workflows and must not be simulated by chatbot text. |

## Critical limitations — do not claim unrestricted access

1. The current development chat is protected by a **shared operator Basic Auth gate**, not per-worker identity. In a production environment, the generic `/api/v1/agie/chat` is closed by default until an approved identity-and-authorization path exists.
2. `intakeTenant()` pins the active factory tenant for chat; it is **not per-worker data entitlements**. SQL catalog inspection and model-written SELECT still require a reviewed read policy, data-class restrictions, row-level isolation on every source and negative tests before opening to real workers.
3. The model may be down, may issue an invalid query, or may not find a suitable source. The server returns an explicit unknown/unavailable state. A successful query proves only that **that particular query** worked; no system can guarantee precise answers to **any** question without the needed data and authority.
4. The optional cloud-model path may transmit model requests and tool results outside the factory. Verify data-processing and privacy rules before enabling `CLOUD_S27_ENDPOINT` + `S27_API_KEY` on sensitive workloads.
5. Existing live metrics have known limits; simulated cuts, sewing, QC and packing are not confirmed production counts. Manual answers never imply physical issuance or authorization.

## Validation

- `node scripts/test_worker_manual_v49.cjs` verifies primary and advanced chapter wiring, protected route classification and single-source manual completeness.
- `go test -race ./services/edge-daemon/internal/workerhelp ./services/edge-daemon/internal/agie/interviewer` verifies isolated guide and model-tool behavior in an environment with the repository's Go toolchain.
- `bash scripts/smoke_aggie_worker_v49.sh` on the iMac checks authenticated manual retrieval, ten-chapter availability, the `GET /api/v1/agie/capabilities` catalog + read-only query probe, deterministic manual chat, and one direct live database question. A pass is **not** certification of the model, every database table or all authorizations. The capability endpoint deliberately reports `model_tool_calling: not_tested`.
- Rebuild on the MacBook and require all GitHub Actions checks before the iMac deployment. Do not create production write permissions for Aggie.
