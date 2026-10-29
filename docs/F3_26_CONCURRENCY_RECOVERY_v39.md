# F3.26 v39 — PostgreSQL concurrency and recovery negative-path validation

Author: Sunil Jain, CyberinQ LLC — 2026-10-29

Baseline `7309deb`. **No database migration**: invariants already implemented by prior immutable event schemas and unique constraints. This release verifies rather than recreates them.

## Verified design invariants

* The existing `pack_one_carton_per_garment_v22` partial unique index is the final concurrency arbiter, even if two different carton transactions race to add the same garment.
* A backend forcibly terminated after INSERT and before COMMIT cannot leave committed phantom evidence.
* The uniqueness scope is per tenant; the same opaque garment UUID used for two distinct tenants is not a uniqueness violation.
* Application role cannot UPDATE/DELETE immutable dock evidence nor INSERT/UPDATE/DELETE provisioner-only gateway, buyer, carrier and signer trust roots.
* Authenticated dock status/report SQL is bounded by an 8-second PostgreSQL statement timeout and a 1.5-second lock wait limit, within the existing 10-second HTTP context. Database lock starvation is handled by an error, not fabricated readiness.
* Production Compose exposes the v35–v38 feature switches as explicit, **default false** settings; none imply shipment/stock credit.

## Validation limits

The concurrency harness requires a *disposable restored database* and an explicit `AGIPAREL_TEST_DB_DISPOSABLE=YES` guard. The test is intentionally executed with a database superuser in `session_replication_role=replica` to inject synthetic rows that cannot pass real signature and physical evidence guards. Index uniqueness, WAL rollback, and tenant-key scoping are tested on actual tables; **device cryptography, operational transaction sequencing, distributed multi-factory contention, latency/SLA, network partitions, shipment posting, inventory and financial conservation are not certified.**

The release installer first backs up PostgreSQL, restores into a new scratch database, runs existing schema and tenant-isolation scripts plus the v39 negative-path drill, then Go/race/vet, JavaScript contracts, build and local health check before committing/pushing the exact patch paths. Do not use the test harness against any production or shared database. No migration `0058` is introduced.

## Future acceptance

A meaningful concurrency benchmark still needs real signed enrollment, at least two independent authorized client sessions, actual physical reader/gateway retries and network disconnection, lock contention metrics, operation idempotency, and cross-ledger invariants at realistic factory throughput. A passed v39 test is **not** shipment authorization.
