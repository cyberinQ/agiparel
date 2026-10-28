# F1.1 — Workforce identity & read-only plant reporting

Author: Sunil Jain, 2026-10-28

Baseline: user-supplied Agiparel archive with HEAD `e03223fea89638518fb8c6560ead252361ef9ba7` (including Chapters 01–10, migrations through 0033).

## Implemented

- `pkg/workforceauth` verifies **externally issued JWT access tokens** using a provisioned, read-only JWKS snapshot: RSA RS256 or Ed25519 EdDSA. Exact HTTPS issuer and API audience, pinned `kid`/algorithm, expiry/freshness, MFA/WebAuthn/FIDO2 claim checks; no untrusted `jku` or proxy identity headers. Authenticated subject is the only identity taken from the signed token.
- Migration `0034_workforce_identity_grants_v15.sql` introduces independently provisioned tenant/plant/permission grants under PostgreSQL forced RLS. Application role has **SELECT only**. No automatic user grants, no role trust from client claims.
- Chapter 09 read-only production report optionally accepts an enterprise IdP token, plus explicit tenant and plant **request selectors**, only after checking the independent workforce grant and the pre-existing `intelligence_prod_readers_v13` assignment inside one RLS-scoped repeatable-read transaction.
- When `AGIPAREL_WORKFORCE_OIDC_ENABLED=true`, a missing/invalid IdP configuration disables the report; **pilot assertion fallback is disabled**. Existing short-lived transaction-bound attestation remains required for material, sewing, QC and packing mutations.
- Chapter 09's existing optional access panel offers an OIDC token/scope mode. All page layouts, styles and colors remain unchanged. No browser storage is used.
- `compose/docker-compose.workforce-oidc.yml` is an **opt-in** production override, mounting an approved JWKS file read-only. Development remains unchanged.

## Operational provisioning (authorized provisioning role, NOT the agiparel_app role)

1. Configure IdP for a JWT **access token** with the dedicated Agiparel API audience, a stable `iss`/`sub`, short expiry (no longer than 1 hour), `amr` evidence of MFA, and supported JWKS signing keys. Verify this mapping against the actual IdP before activation.
2. Periodically export verified issuer JWKS to a protected file outside the repository. Never use `jku`/`x5u` from a token. Rotate via a new approved JWKS file and service restart. Keep overlap keys during rotation.
3. Provision `workforce_identity_grants_v15` row with exact IdP issuer, subject, tenant, plant and `intelligence.read` permission, `enabled=true`, `approved_by` recorded; independently provision an enabled `intelligence_prod_readers_v13` row. Revocation is effective on the next database check.
4. Activate **staging first** with the production Compose file and the OIDC override, after configuring the IdP, database roles, secrets and TLS proxy. Never run production config using `docker-compose.dev.yml` or the sample development credential.
5. Require MFA, wrong tenant/plant denial, disabled grant denial, expired token denial, real PostgreSQL row-security tests, concurrency/load tests, JWKS rotation, and verified production ingress before promoting out of staging.

## Limitations — explicit non-production acceptance

- This increment does **not** implement authorization-code/PKCE interactive SSO, login sessions, browser token exchange, SCIM provisioning, cross-instance session management, or hardware onboarding.
- The Chapter 09 UI still accepts a manually supplied short-lived access token for a controlled pilot; do not expose a token-acquisition flow to public users or store tokens in local storage.
- This is **F1.1 only**: the new IdP auth path authorizes only *read-only* Chapter 09 reports. All mutation APIs retain stronger transaction-bound proof, and all legacy development APIs remain fail-closed in production.
- This is not permission to call any simulated evidence 'production', nor to treat sewing-completion groups as garment counts, finished inventory, payroll or shipment authority.

## Release gates

- On iMac with Go 1.24: `go test -count=1 ./...`, `go test -race -count=1 ./...`, `go vet ./...`, `go build ./...`, and all `scripts/test_*.cjs`.
- Apply migration 0034 once after backup, to development/staging first. Verify table/grants and RLS behavior using distinct roles.
- Build/redeploy development without enabling OIDC; check Chapter 09 pilot path remains unchanged; inspect restart logs.
- Production deployment is a distinct, manual staging change-control item requiring approved IdP, JWKS, DBA provisioner, HTTPS reverse proxy, secret management and rollback plan.

Next F1 increments: interactive SSO with authorization-code + PKCE and secure session management; shared RLS-scoped identity context for Chapters 01–10; independent staff and device provisioning, role approval, audit and revocation; integration tests using PostgreSQL and a test IdP.
