# AGIPAREL Factory Edge v50 — integrated local pilot (release candidate)

**Author:** Sunil Jain, 2026-11-12. **Base:** `cyberinQ/agiparel` main `2db494fee5bfedab269086b2218b09a249b680f7`.

## What is delivered

1. **Aggie exact counts**: Handles combined questions such as “How many workers are in the factory today? How many rolls in inventory?” without relying on Ollama. Counts are from scoped PostgreSQL, not simulated UI events. Attendance is *recorded presence*, not verified current physical presence. Roll counts come from `store_rolls`, not unapproved roll observations.
2. **Aerolake retention safety**: The bridge defaults to a kill switch and never runs telemetry deletion—not even when the legacy kill-switch variable is set `false`. This sacrifices automatic cleanup until a real authenticated durable Parquet checkpoint verifier, schema mapping, CDC/Flight producer, and recovery tests are implemented. A listening gRPC socket **is not archive proof**.
3. **Full pilot topology**: Seven isolated Compose services: AGIPAREL edge, document intake, PostgreSQL 16, Redis, Ollama, Aerolake engine, and Aerolake bridge. Only the pilot HTTP endpoint is exposed to the host, on loopback. Pilot volumes are separate from existing `agiparel.empowerai.us` volumes and containers.
4. **Installer/preflight**: Generates unique passwords, checks restrictive secret-file permissions, requires digest-pinned Aerolake and Ollama images, refuses production mode, and requires explicit consent before starting. Local Ollama model is pulled and checked. No public exposure, migration of existing databases, or data-destructive reset.
5. **Reproducible source bundle**: Build script creates a self-contained pilot source bundle and SHA-256 manifest for an eventual GitHub prerelease. It contains no secrets or third-party container binaries.

## Running a brand-new pilot

Prerequisites: Mac or Linux with Docker Engine / Docker Desktop and Compose V2, Python 3, OpenSSL, curl, sufficient RAM/storage for the chosen model, network access to **authorized** image registries. The entire source must be checked out (the bundle contains only installer files).

```bash
# Author: Sunil Jain, 2026-11-12 — Start a separate loopback-only factory pilot.
cd ~/agiparel
bash scripts/factory_pilot_v50.sh --init
# Edit .env.factory-v50 locally: set the correct AGIPAREL_FACTORY_TIMEZONE,
# AGIPAREL_OLLAMA_IMAGE and AGIPAREL_AEROLAKE_IMAGE to approved image@sha256 digests.
bash scripts/factory_pilot_v50.sh --check
AGIPAREL_CONFIRM_V50_PILOT=YES bash scripts/factory_pilot_v50.sh --up
bash scripts/factory_pilot_v50.sh --verify
```

The test factory is then **only** at `http://127.0.0.1:18539/`, protected by randomly generated development credentials in `.env.factory-v50`. Open Chapter 01 — Factory Setup and Chapter 10 — Upload Files to enter **sample non-sensitive** factory information. Do not enter payroll, private employee records, or real production transactions. Stop the pilot without deleting its data:

```bash
# Author: Sunil Jain, 2026-11-12 — Preserve isolated pilot volumes.
bash scripts/factory_pilot_v50.sh --down
```

This is **not** a production installer. The public demo domain is **not** changed by any of these commands.

## Expected Aggie behavior on the existing iMac after deploying the code

Ask: **How many workers are in the factory today? How many rolls in inventory?**

Expected: two separate replies, each grounded in the current tenant's read-only records. If no attendance for the current factory date is recorded, Aggie explicitly says it cannot establish today's presence; it does not equate absence of records to zero attendance. It distinguishes inspected/available, reserved and held fabric rolls. These are not proof of physical inventory or reservation for a PO.

Set `AGIPAREL_FACTORY_TIMEZONE` to a valid IANA time zone for the factory (for example `Asia/Kolkata`) in your existing iMac `.env` before deploying. If omitted, historical code assumes India time and v50 calls this out in its reply. This is a development shared-tenant chat; real workforce authorization is not installed by v50.

## Hard acceptance gates before selling or letting customers self-install

| Gate | v50 status | Production acceptance |
|---|---|---|
| Exact worker/roll questions | Code and isolated tests | Live PostgreSQL role/RLS, realistic tenant test and actual data reconciliation |
| Aerolake process packaging | Defined, but **image absent** | Approved registry artifact, image digest, architecture/device compatibility, licensed distribution |
| Aerolake data replication and checkpoint | **Not yet implemented** | Verified CDC + Arrow Flight, persisted signed checkpoint, loss/replay drills, measured archive lag |
| Telemetry pruning | **Intentionally blocked** | Enable only after independently verified durable archive and replay safety |
| Ollama packaging | Compose contract, **image absent** | Approved pinned image, compatible model redistribution / license, resource sizing and runtime diagnostics |
| User/tenant registration | Existing login supports optional OIDC, no self-service | External IdP integration, customer enrollment, strong per-worker authorization and data separation |
| Factory onboarding | Existing Chapters 01 and 10 | Secure first-admin provisioning, import validation, readiness wizard, device commissioning |
| Hardware / signed production | Still gated | Onsite physical reader, witness and signed custody acceptance tests |
| Customer upgrade/rollback | Source patch + DB guard only | Signed OCI images, rollback automation, migration policy, backup restore drill and support SLA |

**Distribution status:** GitHub repository contains source and Compose contracts. The release workflow can distribute a source-only pilot bundle; **no complete licensed, signed customer installer, preloaded AI models, or Aerolake engine image is available here**. Never advertise the ZIP as an offline one-click installer.

## Security details

- Pilot dev Basic Auth is not production workforce identity; bind only to localhost, do not attach a tunnel to port 18539.
- No `docker compose down -v`, `system prune --volumes`, old volume mount or existing `.env` overwrites.
- Keep `CLOUD_S27_ENDPOINT` disabled unless separately approved for sending sensitive factory data externally.
- All factory status queries use the configured dev tenant, database row-level security, read-only transactions and bound parameters. This still is **not** per-worker authorization.
- The legacy `compose/docker-compose.mesh.yml` and `compose/docker-compose.aerolake.yml` are inconsistent and may contain hard-coded development credentials. They are **not** used by the v50 pilot.
