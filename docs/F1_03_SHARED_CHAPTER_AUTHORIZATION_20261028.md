# F1.3 — Shared Chapter Permission Authorization

Author: Sunil Jain, 2026-10-28
Baseline: Git `5aa23b8` (F1.2 plus the Chapter 09 contract-test correction).

## What this increment actually enforces

- One canonical action-to-chapter registry (migration 0036, `workforce_chapter_actions_v17`), for Chapters 01–10.
- One provisioner-owned **authorization source**: `workforce_identity_grants_v15` (F1.1). The application receives SELECT only; no roles or plant grants are inferred from OIDC tokens, HTTP parameters, or cookies.
- Chapter 06 worker-attested sewing completion, Chapter 07 QC inspection/repair and catalog, Chapter 08 staging and readiness, and Chapter 09 reporting now require an exact **enabled** tenant + plant + issuer + subject + permission grant. The database check runs **inside the transaction** holding the existing tenant RLS scope. Revoked workers cannot even replay old commands.
- Existing station assignments, independent reviewer roles, WebAuthn/FIDO2 proofs, signed RFID gateway reports and independent Chapter 09 reader assignments remain mandatory. This change adds another gate, not a replacement.
- The `GET /api/v1/workforce/chapter-access?tenant_id=...&plant_id=...` endpoint returns only the authenticated subject's *provisioned chapter action grants*, tagged with production API enablement and limitations. It supports signed JWT bearer or F1.2 browser session, never unsigned headers or client-supplied roles. It is **not** a production operations API.
- Chapters 01–05 and 10 are cataloged for future RBAC adoption but **remain inaccessible in production** because their legacy handlers do not yet meet the security/evidence criteria. No inherited development Basic Auth routes are opened.

## Mandatory rollout order

1. Apply migration 0036 with privileged migration credentials **before deploying** the new server, or production operations fail closed.
2. Provision exact `workforce_identity_grants_v15` entries using a separately authorized DBA/provisioner connection, after authenticating organization identity and obtaining appropriate approval. Use the *issuer encoded in the transaction-bound assertion* for Chapters 06–08 and the OIDC issuer for Chapter 09. Do not assume these issuers are equal.
3. For each actor, bind `tenant_id`, `plant_id`, `idp_issuer`, `idp_subject`, the exact policy action, `approved_by` and `enabled=true`. No wildcard permission, automatic owner role, or cross-plant grant exists.
4. Confirm the separately governed station/role assignment tables are provisioned; an F1.3 permission alone is deliberately insufficient to start sewing, review QC, stage WIP or read management reports.
5. In staging, verify grant omission/disable, cross-tenant, cross-plant, subject swap, invalid issuer, duplicate replay after revocation, positive authorized evidence and production-unavailable legacy chapter routes.
6. Verify tenant isolation directly as the `agiparel_app` DB role with `SET LOCAL app.current_tenant_id` in transaction, including concurrent requests. DO NOT declare production-ready from code-level tests.

## Example provisioning (for controlled staging only)

```sql
-- Illustrative; run ONLY as a privileged provisioner, using actual approved values.
-- INSERT values are deliberately omitted: never invent tenant, plant or subject IDs.
SELECT chapter_id, permission, authority
FROM workforce_chapter_actions_v17
ORDER BY chapter_id,permission;

-- Read-only audit; must use authorized privileged provisioning connection:
SELECT tenant_id,plant_id,idp_issuer,idp_subject,permission,enabled,approved_by
FROM workforce_identity_grants_v15
WHERE tenant_id = :'tenant_id' AND plant_id = :'plant_id';
```

## Release gates / known boundaries

- **Pilot only:** Browser SSO is currently a Chapter 09 read-only feature. No browser session can authorize sewing, QC repair or packing writes.
- Chapter 01–05 and 10 legacy business operations still need rewritten tenant-scoped APIs and application permissions before this catalog can govern them.
- Production operators must use a configured OIDC IdP and provisioned grants; the new chapter-access endpoint remains disabled in dev and fails closed on invalid JWKS configuration.
- Database integration, cross-tenant/RLS tests, authentic devices, real IdP flows, scale tests, full Go 1.24 build and deployment acceptance remain required outside this container.
