# v43.1 — HTTPS proxy and Aggie attendance latency repair

Author: Sunil Jain, CyberinQ LLC, 2026-10-29.

## Engineer Ops behind HTTPS

The v43 administrator page exists at `/engineer-ops` but denies ordinary HTTP
at the origin. For an HTTPS-terminating reverse proxy (Caddy, nginx or a tunnel),
run `bash scripts/enable_engineer_ops_proxy_v43_1.sh` **on the Docker origin**.
The script refuses to enable trust unless the app's host binding is loopback.
The upstream proxy must explicitly **overwrite** incoming `X-Forwarded-Proto`
with `https` on requests received over public HTTPS; it must not pass through
an untrusted header from a web client. Browser GET without credentials must
return 401 (Basic Auth challenge), not 403. Local development can use
`http://127.0.0.1:18530/engineer-ops` with independently provisioned credentials.

Do not publish the raw port on all interfaces or bypass the administrator gate.
The page remains read-only and does not provide shell or arbitrary SQL access.

## Aggie

A question such as `who is present and who is absent today` previously missed
its fast tenant-scoped PostgreSQL route because it did not say `worker`,
`employee` or `staff`. v43.1 recognizes this question and returns source-backed
counts plus a bounded named roster from the same tenant's attendance register
(joined to the workers roster). It explicitly distinguishes *ABSENT* from
*not checked in / unrecorded*. Without attendance records it answers that the
facts are unavailable; no person is falsely counted as absent. This uses the
existing Asia/Kolkata shift-day convention and creates no new records.

## Deployment

Apply the source patch without recreating PostgreSQL. Test on the Mac before
shipping to public HTTPS. For the existing v43 release: install the patch,
run the full regression suite and build, then run the proxy configuration
script and verify `https://agiparel.empowerai.us/engineer-ops` returns an
unauthenticated 401. All deployment errors stop before Git push. The public
home remains freely readable.
