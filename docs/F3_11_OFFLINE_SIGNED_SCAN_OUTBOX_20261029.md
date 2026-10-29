# F3.11 — Durable offline outbox for signed gateway scans
Author: Sunil Jain, CyberinQ LLC, 2026-10-29

## Real function, narrowly bounded authority

`services/edge-daemon/cmd/scanrelay` is a **standalone executable on the gateway host**. The gateway's enrolled signing process first creates a complete, immutable F3.3 `WIP_GATEWAY_SCAN_V1` JSON envelope, including `event` and `signature_hex`. The relay **never signs** and never changes the event fields or signature. It durably queues the already-signed envelope, then retries delivery to the *existing* `POST /api/v1/wip/production/scans` endpoint through HTTPS. That endpoint is still the only authority to check Ed25519 signature, gateway enrollment and scope, accepted-part genealogy, and current reusable-carrier binding. `scanrelay` cannot issue WIP moves, cure missing parts, remove an F3.10 hold, grant sewing output, or release QC/shipping.

This is not a modified PostgreSQL ingestion path, not a standalone scanner firmware driver, and not guaranteed physical RF presence. An upstream gateway application must integrate `scanrelay enqueue` after producing a genuine signature. Do not generate fake enrollment records or production observations for commissioning.

## Gateway host install

Build from the Agiparel repository on the gateway or a compatible Go 1.24 builder:

```sh
mkdir -p "$HOME/bin"
go build -o "$HOME/bin/agiparel-scanrelay" ./services/edge-daemon/cmd/scanrelay
mkdir -p "$HOME/.local/share/agiparel-scan-outbox"
chmod 700 "$HOME/.local/share/agiparel-scan-outbox"
```

Use a **persistent local filesystem**, not `/tmp`, an NFS volume, a source repository directory, or a container's ephemeral overlay. Ensure sufficient disk space, reliable time sync, owner-only access, and a power-loss-safe storage configuration. A dedicated local service account is recommended. The relay creates private `pending`, `inflight`, `accepted`, `quarantine` directories and private state locks. Retain its accepted receipts and quarantine folder according to the plant's audit and data-retention policy.

## Actual operation

```sh
# From the gateway's trusted signer, not an unsigned scan event:
"$HOME/bin/agiparel-scanrelay" enqueue \
  --dir "$HOME/.local/share/agiparel-scan-outbox" \
  --file /secure/path/signed_gateway_scan_envelope.json

# Repeated after reconnection, from the same local host:
"$HOME/bin/agiparel-scanrelay" flush \
  --dir "$HOME/.local/share/agiparel-scan-outbox" \
  --endpoint https://YOUR-APPROVED-INGRESS.example/api/v1/wip/production/scans \
  --max-age 12m --batch 50

"$HOME/bin/agiparel-scanrelay" status \
  --dir "$HOME/.local/share/agiparel-scan-outbox"
```

The gateway integration must persist the signed JSON envelope **before acknowledging a locally captured event**; if `enqueue` fails, do not claim the event was queued. Schedule `flush` from a supervised local timer or service. A single flush process is serialized across restarts; a different short-lived state lock lets scanner producers continue queueing during a network outage. Disk fsync and atomic rename guard the local state, but do not replace OS/disk-level durability tests.

## Safe retry and exception semantics

- Event identity is `(tenant_id, gateway_id, event_id)`; exact signed duplicates are idempotent, while changed payloads under an existing ID are refused. Files are keyed by a SHA-256 digest of this identity. Buffer is bounded to 5,000 pending/inflight entries; a full queue fails rather than dropping the oldest event.
- On an unacknowledged network failure, 429, 5xx, or malformed 2xx acknowledgement, the original signed event stays pending. A crash with an event in `inflight` is recovered and resubmitted. F3.3's server-side immutable event-ID deduplication decides whether it is new or a replay.
- A successful 200/201 must contain an actual scan UUID, replay boolean, the **exact F3.3 authority**, and `production_credit:false`. Only then is an accepted receipt durably stored and the pending payload retired.
- A 4xx rejection (other than 429), a locally expired observation or invalid clock drift is **quarantined**, with the original signed JSON and a sidecar `.reason`. Quarantine does not repair, edit, extend, or backdate scans. It requires manual investigation and, where WIP trust is compromised, the existing F3.10 signed exception workflow.
- The local default delivery deadline is 12 minutes, conservative relative to the server's 15-minute default. It is configurable between 1 and 15 minutes. When the server uses a *stricter* window, its rejection wins. Gateway and server clocks must be synchronized. **Long outages do not create late production authority**; replaying a stale scan cannot prove physical presence.
- Only HTTPS ingress with normal certificate validation is accepted. The exact WIP scan route is enforced; no URL credentials, query string, redirects, or Bearer authorization are forwarded. Provision device TLS/reverse-proxy connectivity independently. This tool does not configure mutual TLS or manage gateway private keys.

## Validation and qualification

Local testing: `go test -count=1 -race ./services/edge-daemon/cmd/scanrelay` and `node scripts/test_wip_f311_contract.cjs`, followed by the full repository suite. The relay's unit tests use TLS test servers and generated, **non-production** Ed25519 test envelopes; they do not write the operational database.

Physical staging acceptance still must demonstrate genuine gateway signatures and enrollment, positive delivery to a real PostgreSQL 16-backed F3.3 service, network cable unplug/restore, duplicate/changed events, power failure, full disk, incompatible time drift, gateway disablement, cross-plant isolation, scans consumed in WIP moves, and safe F3.10 supervised recovery. Neither a valid receipt nor signed gateway metadata is independent proof of physical scanner performance.

F3.11 requires **no schema migration** and does **not** switch any server production feature gate on. The existing server flags and workforce authorization remain unchanged.
