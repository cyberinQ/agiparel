# v43.2.2 — Public Navigation and About

Author: Sunil Jain, CyberinQ LLC · 2026-10-29

- Factory Home: top-right **About**, **Sign in**, **Create account**, with a simple account icon. No changes to existing public prose, chapters, Aggie or fabric-wave artwork.
- `/about`: public, concise introduction to the approach, with explicit pilot/physical-commissioning qualification.
- `/login`: public branded sign-in entry. When existing OIDC browser configuration is actually enabled, its button links to `/auth/login`; otherwise it displays a truthful setup message.
- `/register`: public branded registration entry. Only redirects to a trusted HTTPS authentik enrollment flow on the exact configured issuer host when OIDC is enabled; otherwise registration remains unavailable, without collecting passwords.
- No additional account database or plaintext password handling is introduced. The existing temporary operator gate is *not* removed; v44 will replace it once the identity provider is provisioned.
- `/engineer-ops`, factory authorization, database, migrations and all existing content are unchanged.

Deploy: use `scripts/install_release_v43_2_2.sh` from the PATCH ZIP. It verifies the exact prior v43.2.1 files, runs tests, deploys the edge container, checks all four public pages and optionally reports whether the public HTTPS hostname serves the changes. An upstream 403 is a warning, not a fabricated application failure.
