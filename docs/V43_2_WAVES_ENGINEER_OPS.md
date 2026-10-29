# v43.2 — Decorative fabric waves + Engineer Ops observability

Author: Sunil Jain, CyberinQ LLC, 2026-10-29

## Public Factory Home
- Starting source is v43.1, **not** the rejected v43.2 wave-release.
- Only adds a decorative fabric-thread canvas near the bottom of existing Factory Home.
- Every pre-existing word, headline, tagline, chapter card, optional Q&A, and Aggie message is byte-for-byte preserved.
- Uses Agiparel teal/blue on a light textile-colored field, with reduced-motion static rendering and background-tab/offscreen animation pausing.

## Engineer Ops
- Database catalog is collapsible, initially closed; retains existing read-only catalog data.
- API catalog is collapsible, initially closed, source-derived list of registered API paths in this Go release, searchable by path or service area. The inventory is not an OpenAPI spec and *not* a claim that every route is currently enabled.
- Platform analytics: registered customer tenant records from `s01_identity.tenants`, identity user records from `s01_identity.users`, and request count (not factory efficiency) since current edge-server process startup. No invented metrics. Counts show Unavailable when DB/function missing.
- SQL count-only `SECURITY DEFINER` function, no raw identity-record privileges given to app role; only an Engineer Ops authenticated HTTP overview can display the aggregates.
- Existing privileged Engineer Ops HTTPS/proxy gate unchanged. No new SQL console or platform write API.

## Deployment
- Migration `0002_engineer_ops_aggregates_v43_2.sql` is additive, non-destructive; applied by installer to existing local dev DB and included in fresh Docker initialization and CI.
- Run against v43.1 or an identical source tree. The installer validates exact affected-file hashes before modifying files.
- Installer backs up dev DB before migration, runs JS+Go/race/vet plus isolated DB assertion, rebuilds app, verifies local and public URL, and pushes only after success. No database reset, no v44 authentik dependencies.
- The total utilization count resets when the edge-server process restarts; sustained, unique-user or machine utilization metrics are future work.
