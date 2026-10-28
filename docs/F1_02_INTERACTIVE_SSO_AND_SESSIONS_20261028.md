# Agiparel F1.2 — Interactive workforce SSO and secure browser sessions

Author: Sunil Jain, 2026-10-28

**Scope / baseline:** Git `576bd33` (F1.1). Read-only Chapter 09 production evidence viewer. The existing layout/color palette is unchanged. This increment does **not** authorize sewing, QC, packing, inventory, shipment, payroll, or any other operational writes.

## Delivered controls

1. Confidential-client OIDC Authorization Code flow with S256 PKCE; explicit, provisioned HTTPS authorization/token/callback URLs; no dynamic identity-provider-controlled redirect or `jku` lookup.
2. Cryptographically random, browser-bound, five-minute one-time state and nonce; state is consumed before the token exchange; replay/cross-browser reuse denied.
3. Token endpoint exchange via server-side client authentication; cryptographic ID-token signature, issuer, client audience/`azp`, expiry, nonce and MFA/strong-auth check. ID tokens cannot be used as F1.1 API Bearer access tokens. No access/refresh token is saved in the browser or database.
4. Opaque `__Host-` Secure + HttpOnly + SameSite=Lax cookie; only its SHA-256 digest is held in PostgreSQL. Absolute lifetime 8h, idle lifetime 20m. Sessions operate across replicas sharing PostgreSQL; no in-process session store.
5. `/api/v1/workforce/session` confirms active session and issues an ephemeral CSRF token. `POST /auth/logout` requires same HTTPS origin and CSRF token; revokes the database session. The IdP's global SSO login is **not** revoked by application logout.
6. The Chapter 09 read-only API accepts a valid browser session when enterprise OIDC is enabled, and still performs the **two independent, current database checks**: `workforce_identity_grants_v15` and `intelligence_prod_readers_v13`. Client-selected tenant/plant identifiers confer no authority. If an Authorization header is supplied, the existing Bearer verifier remains authoritative instead of silently falling back to cookies.
7. Runtime remains disabled in development. A separate Compose override opts into SSO in a provisioned staging/production environment. Existing challenge-bound production mutation APIs do not accept these session cookies.
8. Corrected routing: the production `/intelligence` page is available with the `mod_sewing` module alone, not dependent on enabling rehearsal packing.

## Database migration 0035

Additive tables `workforce_oidc_requests_v16`, `workforce_browser_sessions_v16` with expiry indexes. **No** identities, permissions or roles are automatically provisioned. Both tables have forced RLS with no general application table access. Only narrow `SECURITY DEFINER` functions, owned by a trusted migration/provisioning role, are granted to `agiparel_app` (begin/consume login, create/read/revoke sessions, rotate CSRF). Validate function ownership, public-schema permissions and effective RLS in staging before activation. Backup before applying. Apply once to each target database.

Sessions are append-created and revoked, not user-auditable production evidence. Arrange privileged scheduled pruning for expired login attempts and expired/revoked sessions. Provision database encryption, operational audit logs, restricted secret-file permissions, TLS termination and anti-automation/rate limits at the ingress gateway.

## Identity provider and deployment preconditions

- OIDC confidential Web client registered with **exact** redirect `https://<your-edge-host>/auth/callback` and Code + PKCE S256 enabled. MFA must be expressed in the signed ID token `amr` (`mfa`, `webauthn`, `fido2` or `hwk`). These strict requirements must be configured in the IdP, not relaxed in code.
- `AGIPAREL_WORKFORCE_OIDC_ISSUER`: issuer (unchanged F1.1); `AGIPAREL_WORKFORCE_OIDC_AUDIENCE`: production API audience (unchanged F1.1); pinned approved JWKS snapshot.
- New `AGIPAREL_WORKFORCE_OIDC_CLIENT_ID`: OIDC Web client audience; `AGIPAREL_WORKFORCE_OIDC_CLIENT_SECRET_HOST_FILE`: file containing confidential-client secret (not in Git or environment string); explicit `AGIPAREL_WORKFORCE_OIDC_AUTHORIZATION_ENDPOINT`, `AGIPAREL_WORKFORCE_OIDC_TOKEN_ENDPOINT`, `AGIPAREL_WORKFORCE_OIDC_CALLBACK_URL`.
- Opt-in environment: `AGIPAREL_WORKFORCE_BROWSER_ENABLED=true`; existing `AGIPAREL_WORKFORCE_OIDC_ENABLED=true`; `ENV=staging|production`; provisioned PostgreSQL application role; TLS reverse proxy. Only grant report rights through independent provisioning controls.
- Use Compose layers **in this order**: `docker-compose.production.yml`, `docker-compose.workforce-oidc.yml`, `docker-compose.workforce-browser.yml`. Inspect the effective config for port isolation and secret mounts, and deploy only to an approved staging cluster first.

## Required staging release gates

- Run migration on a staging PostgreSQL clone and verify restricted table privileges/functions, one-time state, replay, idle/absolute expiration, logout revocation, concurrent callbacks, session sharing across two replicas and RLS negative tenant tests.
- Confirm IdP callback URL, signature/JWKS rotation, nonce mismatch, MFA omission, invalid audience and unauthorized plant all fail closed. Test in a real browser through a TLS proxy, including logout/reload and expired sessions.
- Test a selected worker with an approved F1.1 grant **and** Chapter 09 reader assignment. Remove either grant while a session is active and confirm report access immediately fails.
- Verify production sewing/QC/packing endpoints reject session-cookie-only mutations, and that no simulated records can become production KPIs.
- Verify rate limits, OIDC logout policy, log redaction, access monitoring, backup restoration and migration rollback plan.

## Remaining production blockers

This is a *staging/pilot-only* integration until the above gates and real IdP/browser tests pass. Workforce lifecycle automation/SCIM, IdP-wide revocation/back-channel logout, shared authorization middleware across Chapters 01–10, secure production document access, hardware-backed action approvals, high-availability failover, abuse/rate-limit controls, security review and comprehensive performance testing remain subsequent F1/F10 work. Do not authorize any physical production mutation with these browser sessions.
