# AGIPAREL v50.1 — Aggie Machine Answer Accuracy, iMac-Only Release

Author: Sunil Jain, 2026-11-12.

## Fixed
- "How many machines?" is answered using `machine_roster`, not a prior fabric-roll count.
- "Which machines are open?" reports stored status labels and expressly does **not** assert physical availability, safety or real-time uptime.
- Combined roll/machine/worker questions are independently classified from the latest user question; never reuses an earlier chat turn's entity.
- Read-only PostgreSQL transactions, application tenant context and parameterized tenant predicates are retained.
- Race/unit and actual handler regression tests cover rolls → machines → status and combined queries.

## iMac-only installation and deployment
The source is published to `cyberinQ/agiparel` main; no ZIP, MacBook or manual patch installation is needed.

On the existing iMac, run:

```bash
# Author: Sunil Jain, 2026-11-12.
# CI-gated backup, rebuild, restart and live accuracy checks on the existing iMac.
cd ~/agiparel
git pull --ff-only origin main
AGIPAREL_CONFIRM_AGGIE_V501_IMAC=YES bash scripts/deploy_aggie_v50_1_imac.sh
```

The wrapper refuses deployment before the `verify` and `postgres-schema` GitHub checks pass for the exact local commit. The existing guarded deployment script snapshots PostgreSQL before restarting only AGIPAREL and document intake. Aerolake orphan containers are **not** removed. No data reset, new schema migration, or volume pruning.

The smoke test gets Basic operator credentials from the running local container without printing them. Run only against loopback.

## Limitations
The machine roster is an administrative record, not a live equipment monitor. The word ACTIVE does not mean running, available for work or certified safe. Absence of attendance records does not mean zero workers in the plant. Development shared Basic credentials are not suitable production per-worker identity. Live model-tool-calling outside the deterministic routes is still not fully verified.

Never expose the existing development operator routes through a public reverse proxy without separate workforce identity and authorization.
