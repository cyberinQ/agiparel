<!-- Author: Sunil Jain, CyberinQ LLC, 2026-10-29 -->
# v43: Public Factory Home and Engineer Ops

No schema migration. Release baseline: `7a8df62` (v42), canonical SQL remains `0001_v42_baseline.sql`. No material, quality, shipment, carrier, stock or financial authorization is changed.

## Public Factory Home

`/` and `/demo` show one collapsed-by-default native HTML `<details>` section at the bottom of the ten-chapter home page. All language is marketing-oriented but factual. No live tenant data is embedded. The public page includes an `X-Agiparel-Release: v43` marker for actual remote verification.

## Engineer Ops

`/engineer-ops` shows current release, service status, enabled modules, database health, database role and an RLS-aware **read-only schema inventory**. Its JSON endpoint is `/api/v1/engineer-ops/overview`. The dedicated administrator password must be at least 24 characters and is independent of DEV operator Basic credentials. The entire page, CSS, JS and API are blocked when the admin feature is disabled, and cannot be used anonymously. All endpoints are GET/HEAD only, no arbitrary SQL, secret values, shell commands, data mutations, or cross-tenant record access.

Engineer-level host/database/code privileges continue through separately managed SSH, PostgreSQL admin and GitHub identities. This is the first engineer-ops UI, **not a root console or an unrestricted cross-tenant superuser**. Those capabilities should never be granted simply by visiting a web page. A later admin policy, MFA, audit log and scoped, explicitly approved mutation API are necessary for management actions.

To enable locally, add to ignored `.env` (installer generates and retains a random secret):

```
AGIPAREL_ENGINEER_OPS_ENABLED=true
AGIPAREL_ENGINEER_OPS_USER=engineerops
AGIPAREL_ENGINEER_OPS_PASSWORD=<generate-with-openssl-rand-hex-32>
AGIPAREL_ENGINEER_OPS_TRUST_PROXY_TLS=false
```

Do not put passwords in committed files. To reach `/engineer-ops` on the public domain behind a **trusted** HTTPS-terminating proxy, ensure the backend port is private, the proxy removes client-supplied `X-Forwarded-Proto` and replaces it with `https`, then set `AGIPAREL_ENGINEER_OPS_TRUST_PROXY_TLS=true` and restart Docker. A public HTTP connection never permits admin access. Do not give any account blanket database SUPERUSER or OS root via the web application.

## Publish on agiparel.empowerai.us

The developer laptop and the public DNS hostname may not point at the same machine. Deployment is verified only when **HTTPS `https://agiparel.empowerai.us/` actually returns `X-Agiparel-Release: v43` and the three-question section**. The installer stops without committing if this cannot be verified; a healthy `http://127.0.0.1:18530/healthz` does not prove publication.

Perform the build on the host that the existing domain reverse proxy/tunnel uses as its origin, or deploy a pinned image to that host. Use the site's existing Cloudflare Tunnel/nginx/Caddy routing rather than adding a second proxy. If the site currently points to another host, update the external route first. A Caddy reverse-proxy **example**, not an automatic DNS change, is `compose/Caddyfile.agiparel.example`. This repository contains no access to the live DNS registrar or hosting account.

1. Keep the existing domain TLS certificate/termination and route `agiparel.empowerai.us` to the deployed origin on `127.0.0.1:18530` (same host). The dev Compose binds the origin to loopback by default. If your existing reverse proxy is on another host, configure `AGIPAREL_EDGE_BIND_HOST` to a private host IP and firewall it to the trusted proxy only (never expose the admin origin directly). Do not expose development PostgreSQL/Redis on the public network.
2. Run the combined installer from the verified v42 `main` checkout on that host; no database reset occurs. It tests, builds, checks local health and verifies the public URL.
3. If external routing is temporarily stale, complete the routing and run `bash scripts/finalize_public_release_v43.sh` from that already-patched checkout. It checks public v43 before GitHub commit/push. Run `bash scripts/verify_public_release_v43.sh` for later acceptance.
4. Anonymous `/engineer-ops` must report 401 (configured), 403 (missing trusted TLS), 404 (disabled) or 503 (unconfigured), never 200. The site public home and sample examples require no credentials. Actual factory APIs and privileged operations continue requiring proper identity.

**Customer demos:** publicly show the landing page and explain the actual running system with guided, authenticated demo data. The restricted chapter and factory workflows are not automatically made anonymous by v43; never populate them with customer secrets for a public walkthrough.
