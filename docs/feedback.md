### Executive Summary & Repository Pruning Commands

The primary reason this repository lists 900+ items is that the local zip archive includes the uncompressed `.git/objects/` tree (hundreds of individual commit, tree, and blob loose objects) and macOS filesystem metadata artifacts (`__MACOSX/` resource forks and `.DS_Store` files).

Execute the following commands in the root directory to eliminate all redundant files, OS metadata, local cover profiles, and git object bloat:

```bash
# 1. Strip macOS resource forks, attribute caches, and desktop indices
rm -rf __MACOSX
find . -name ".DS_Store" -type f -delete
find . -name "._*" -type f -delete

# 2. Prune local coverage dumps, test binaries, and local audit snapshots
rm -f coverage.out
rm -f machine_audit_report.json machine_audit_db_report.json
find . -type f -name "*.test" -delete
find . -type f -name "*.out" -delete

# 3. Clean Go build and module cache
go clean -cache -testcache -modcache

# 4. Repack git loose objects into a single optimized packfile
# (eliminates hundreds of loose hash directories under .git/objects/)
git prune
git gc --aggressive --prune=now

```

---

### Technical Audit Matrix

| Domain & File Location | Primary Function | Logic & Correctness | Availability | Scalability | Idempotency | Immutability | Atomicity |
| --- | --- | --- | --- | --- | --- | --- | --- |
| **Document Intake Worker**<br>

<br>`services/document-intake/cmd/worker/main.go` | Background queue processor for OCR/document segmentation.

 | **Pass**: Claims via `SKIP LOCKED`. Stale processing lease fallback (`updated_at < now() - 5 min`) prevents worker death locks.

 | **Pass**: No HTTP listener exposed; crash-isolated from edge API. Database backoff on idle/error.

 | **Pass**: Strict connection pooling (`MaxOpenConns: 3`); chunked 6,000-char segment slicing prevents memory bloat.

 | **Pass**: `attempts` counter bounded at 3; segments insert with `ON CONFLICT DO NOTHING`.

 | **Pass**: Original bytes are read-only (`SELECT`); extracts only to append-only tables.

 | **Pass**: `finish()` bundles segment creation and job status update in a single `db.BeginTx` transaction.

 |
| **Stores Issuance Engine**<br>

<br>`pkg/stores/issuance.go` | Dual-witness digital signing and fabric roll stock issuance.

 | **Pass**: Millimeter rounding (`math.Round(x * 1000)`) eliminates IEEE-754 floating-point drift. Requires distinct witness IDs.

 | **Pass**: In-memory operation; fast cryptographic evaluation with HMAC-SHA256.

 | **Weak**: Single in-memory struct with `sync.Mutex`; does not scale across multi-instance deployments without DB backing.

 | **Weak**: Generates `IssuanceID` using millisecond wall-clock (`ISS-%d`). Replays with identical inputs can create distinct records if invoked milliseconds apart.

 | **Pass**: HMAC claim binds `(RollID, OrderID, CutNo, Length, Witnesses, Timestamp)`.

 | **Pass**: Mutex locks state mutation; status updates to `DEPLETED` or `ISSUED` alongside length reduction.

 |
| **Material Variance Authority**<br>

<br>`pkg/stores/material_variance.go` | CEO-approved over-consumption variance workflow.

 | **Pass**: Rigorous parameter validation; verifies active enrolled CEO identity and enforces plant isolation.

 | **Pass**: Fails closed on database disconnections or unconfigured authority.

 | **Pass**: Explicit `FOR UPDATE` row-level locks on reservations and rolls serialize concurrent variance requests.

 | **Pass**: Explicit duplicate detection on `(tenant_id, request_key)`; returns existing record without recalculating challenge.

 | **Pass**: Database triggers (`0006`) prevent updates once state leaves `PENDING_CEO`. Challenge hash is immutable.

 | **Pass**: DB transaction binds roll validation, reservation increment, variance status update, and outbox event.

 |
| **Classic Bundle Generator**<br>

<br>`pkg/cutting/classic_bundle.go` | Pure functional calculation of sequential bundle cards and serials.

 | **Pass**: `BalanceConsecutivePlies` guarantees no orphan bundles below minimum threshold.

 | **Pass**: Zero external I/O; deterministic algorithm.

 | **Pass**: Allocates arrays cleanly; strict integer arithmetic prevents allocation bloat.

 | **Pass**: Deterministic output given the same `ClassicBundleRequest` parameters.

 | **Pass**: Pure domain calculator; does not mutate input slices. Output represents proposed records only.

 | **Pass**: State-free evaluation; caller holds responsibility for transactional counter allocation.

 |
| **Attachment Engine DAG**<br>

<br>`pkg/attachment/attachment_graph.go` | Graph validation for garment part assembly, cut/pant pairing, and tag retirement.

 | **Pass**: Validates `CutNo`, `PantPosition`, and `PlyStart/PlyEnd` matches prior to part marry.

 | **Pass**: Protected by `sync.RWMutex`. Fails fast on missing tags.

 | **Moderate**: In-memory `map[string]SubBundleNode`. Fine for edge appliances, but must be flushed/synced on restarts.

 | **Pass**: Idempotent rule and tag registration.

 | **Pass**: Validated assembly rules are strictly immutable within an active style ID.

 | **Pass**: Mutex ensures tag retirement (`delete`) and validation happen deterministically without race conditions.

 |
| **Hardware Attestation**<br>

<br>`pkg/attestation/verifier.go` | Cryptographic Ed25519 verification of external IdP JWS claims.

 | **Pass**: Enforces EdDSA algorithm, constant-time verification, freshness windows, and max token lifetimes.

 | **Pass**: Local public key lookups avoid network round-trips to identity providers.

 | **Pass**: High-throughput zero-allocation verification; strictly capped token input sizes (8192 bytes).

 | **Pass**: Verifier is completely stateless and side-effect free.

 | **Pass**: Public keys and verification parameters are read-only at runtime.

 | **Pass**: Evaluation is binary (valid claims or explicit error).

 |
| **Catalog Inspector (SQL Guard)**<br>

<br>`pkg/sqlguard/inspector.go` | Read-only schema inspector and SQL sandboxing.

 | **Pass**: Enforces `SELECT`-only prefix, blocks stacked queries (`;`), and matches forbidden tokens via whole-word boundaries.

 | **Pass**: Uses 5-second context timeouts to prevent runaway query execution.

 | **Pass**: Forces `LIMIT 100` wrapping on arbitrary input queries to prevent OOM errors.

 | **Pass**: Read-only SQL operations guarantee no state changes across repeated queries.

 | **Pass**: Transactions explicitly execute with `&sql.TxOptions{ReadOnly: true}`.

 | **Pass**: Transaction rolls back immediately upon completion.

 |
| **Tenant Context Isolation**<br>

<br>`services/core-api/internal/identity/context.go` | PostgreSQL RLS session configuration injector.

 | **Pass**: Executes `SELECT set_config('app.current_tenant_id', $1, true)` within the target transaction.

 | **Pass**: Propagates context directly into `BeginTx` and all query calls.

 | **Pass**: Uses default connection pool; zero per-query overhead beyond standard session setting.

 | **Pass**: Calling `set_config` repeatedly with the same tenant ID is idempotent within the transaction scope.

 | **Pass**: Transaction rollback is deferred, ensuring untrusted errors never leave an unclosed transaction.

 | **Pass**: Guarantees all commands executed inside `fn(tx)` share the isolated transaction and commit atomically.

 |

---

### Key Architectural Findings & Recommendations

1. **Dual Stores Implementation (In-Memory vs. PostgreSQL Spine):**
* `pkg/stores/issuance.go` uses an in-memory map protected by a mutex with millisecond timestamps (`ISS-%d`).


* `services/core-api/migrations/0005_material_transaction_spine.sql` and `pkg/stores/material_variance.go` implement the durable, production-grade model with row-level locks, SHA-256 challenges, and PostgreSQL triggers enforcing `CHECK (issued_mm + released_mm <= reserved_mm)`.


* *Remediation*: Deprecate the in-memory `StoresEngine` in `pkg/stores/issuance.go` for multi-instance production environments. Direct all custody operations through `MaterialRepository` in `pkg/stores/material_variance.go` backed by migration `0005`.




2. **Carton Packing Session Concurrency:**
* In `pkg/packing/carton_packer.go`, `CartonPackingSession` manages ratio packing and scale verification with a mutex. While internally thread-safe, if an edge node restarts mid-pack, the session state is lost unless persisted to SQLite or PostgreSQL.


* *Remediation*: Ensure the edge daemon persists `CartonPackingSession` state changes directly into the Write-Ahead Log (`internal/wal/wal.go`) or the PostgreSQL `carton_pack_manifests` table.




3. **Workbook Import Retirement:**
* `pkg/onboarding/workbook.go` contains `ApplyToDatabase()`, which explicitly returns `ERR_LEGACY_IMPORT_RETIRED`. This is verified by `workbook_integrity_test.go`.


* *Remediation*: Retain this fail-closed stub. It prevents accidental direct-write imports from legacy CSV workbooks, forcing all inputs through the audited `document_sources_v65` and `factory_draft_decisions_v65b3` pipelines.