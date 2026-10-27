# Engineering release: Cutting reader gateway and controlled sewing release (08C)

## Customer experience

The customer-facing **Cutting & Bundle Genealogy** workbench now has a **Reader events & sewing handoff** evidence section. No internal epic/batch/gate numbering appears in the customer-facing HTML. The same cleanup covers Stores, document inbox, buyer review, factory setup, material planning, order intake, public portal, and onboarding pages. Internal release IDs remain in filenames/docs/tests only.

The view shows expected classic sub-bundles, independently human-attested EPCs, enrolled gateway reports, exact label+EPC matches, and whether Cutting has been signed off for Sewing. The shared `dev` login is *read only* for this release status. No browser form can mint reader events or provide fake release signatures.

## Existing prerequisites (must remain intact)

1. Buyer source-linked order and cutting lay, marker positions and pattern parts (Cutting drafts, schema 0019).
2. Independently witnessed material issue associated with the order, cut, material SKU and shade.
3. Immutable operator spread with exact-millimetre material issue allocation (schema 0020).
4. Human-signed bundle/EPC witness for **every** classic sub-sub-bundle (`cut_bundle_scan_witnesses_v8`), enrolled using a strongly authenticated external IdP. A manually typed EPC is not an antenna read.

## New device evidence

**Admin-enrolled Ed25519 public keys** authenticate reader-gateway reporting identity. The gateway's private signing key is generated and held OUTSIDE Agiparel and must never be sent to the server. Only a database administrator/security officer may enroll, enable, disable or rotate a device key; the `agiparel_app` role has `SELECT` access to registry entries but **no INSERT or UPDATE** on that registry.

`POST /api/v1/cutting/reader-events` requires one JSON envelope containing `{ "event": {...}, "signature_hex": "..." }`. The Ed25519 signature is over **UTF-8 bytes of Go `json.Marshal(event)`** (compact JSON, struct field order below, no signature). Use exact types and names. The server rejects unknown fields, stale/future observations, malformed EPCs, foreign tenants/plants/stations, disabled/unregistered gateways, invalid Ed25519 signatures, duplicate/reassigned EPCs, and observations for bundles lacking the same human-witnessed EPC.

Example schema (values fictional; this example is NOT a signed event):

```json
{
  "event": {
    "version": "CUT_READER_V1",
    "tenant_id": "dev",
    "plant_id": "PLANT_DEFAULT",
    "gateway_id": "rfid-gw-01",
    "station_id": "cutting-exit-1",
    "event_id": "event_012345678901",
    "observed_at": "2026-10-27T13:15:30Z",
    "draft_id": "00000000-0000-4000-8000-000000000001",
    "spread_id": "00000000-0000-4000-8000-000000000002",
    "label_code": "CB08-0123456789ABCDEF01234567",
    "epc_hex": "3034257BF7194E4000001A85",
    "antenna_port": 1
  },
  "signature_hex": "<128 lowercase or uppercase hex chars from Ed25519 signing of canonical event>"
}
```

The event timestamp must be within the last **2 minutes**, and at most **30 seconds** ahead of the server, permitting bounded clock skew; the gateway should use NTP/PTP. Exact `(tenant_id, gateway_id, event_id)` replay with the same signed event is idempotent (200); replay with modified payload is rejected. Only one event can count toward each `(tenant, spread, label)` or `(tenant, spread, EPC)`.

Gateway enrollment example for a security administrator, using a **32-byte hexadecimal public key** already exported from the reader's secure signing module:

```sql
BEGIN;
INSERT INTO cut_reader_gateways_v8
  (tenant_id,gateway_id,plant_id,station_id,public_key,enabled)
VALUES
  ('dev','rfid-gw-01','PLANT_DEFAULT','cutting-exit-1',decode('<64_HEX_CHAR_ED25519_PUBLIC_KEY>','hex'),false);
COMMIT;
```

Leave `enabled=false` until key custody, gateway commissioning, RF site survey, antenna placement and station scope are verified by an administrator. To activate, a database administrator may change `enabled` to `true`; the factory application cannot do this. **Do not generate or claim real RF events with synthetic fixtures**. These schema examples are for integration implementers only.

The existing operator Basic gate also covers the gateway endpoint; a real hardware rollout must provision a device-specific transport/mTLS identity at the reverse proxy instead of using the shared development password. Ed25519 is a separate, mandatory message-integrity mechanism even behind such transport protection.

**Evidence meaning:** gateway signature proves the enrolled gateway reported the event, NOT independent optical/radio proof that the tag was present. Release approvers must explicitly inspect site reader controls and evidence. An independently enrolled real reader source must supply the events; the release is not operational on a clean local DB.

## Strongly signed release

`GET /api/v1/cutting/releases/challenge?spread_id=<UUID>` requires a bearer assertion for `cut.release.prepare` (role `cutting_supervisor`, WebAuthn/FIDO2). It checks the expected bundle count, human witnesses, enrolled gateway events, exact EPC matches, source-plant agreement and millimetre allocations. The challenge is a deterministic SHA-256 digest of the spread identity, original lay snapshot, issued roll IDs/amounts, consumption and every matched label/EPC/human witness/reader event. If anything is missing, the request returns `ERR_CUT_RELEASE_NOT_READY`.

`POST /api/v1/cutting/releases` requires a trusted bearer assertion for `cut.release.submit` (role `cutting_supervisor`) and JSON:

```json
{
  "spread_id": "<real spread UUID>",
  "supervisor_attestation": "<externally signed EdDSA JWT>",
  "quality_attestation": "<separately signed EdDSA JWT>"
}
```

Both signer assertions must bind the exact same digest under action `cut.release.witness`; the supervisor JWT must assert `cutting_supervisor`, the quality JWT `quality_inspector`, both strong FIDO2/WebAuthn, same tenant/plant, **different subjects and JTI IDs**, and supervisor subject identical to the submitting actor. An immutable, unique release row is written only after the evidence is rechecked in a serializable transaction; SQL additionally blocks release if gateway/human evidence, plant or issuance coverage is incomplete. There is **no** `dev` override, test issuance path, or implicit credit to Sewing.

`GET /api/v1/cutting/release-status?draft_id=<UUID>` provides only the read-only aggregate for factory operators, never a signing action or a falsely elevated `ready_for_signed_release` based on the Basic operator login. A released record means a signed cutting-to-sewing handoff exists; actual Sewing line scans and production credit belong to Batch 09.

## Database and operational behavior

- Migration `0021` creates `cut_reader_gateways_v8`, `cut_reader_events_v8` and `cut_sewing_releases_v8`, with tenant RLS and append-only evidence.
- No changes to Stores stock, reservations, signed issues, original PDF documents, or previous Cutting drafts.
- `agiparel_app` cannot register gateway keys.
- Gateway event and signed release tables allow SELECT/INSERT only for `agiparel_app` and reject UPDATE/DELETE even for DBA-sessions via immutability triggers.
- Revoking a gateway before a release prevents that device's prior events counting toward a future release (per the Go readiness logic and SQL release guard).
- The release is one per signed spread, enforced by unique database identity.
- No customer-visible release numbers/batch IDs, but internal generated bundle **identity** prefix `CB08-` is preserved because changing issued physical label IDs would break genealogy. This is a *label namespace*, not a website roadmap tag.

## Limits

No manufacturer-specific LLRP/Impinj/Zebra SDK driver, physical antenna certification, external IdP provisioning, secure gateway-key management UI or sewing transaction integration is claimed. Device wiring must be completed for the selected reader model. The `dev` account cannot perform strong signature steps. MacBook full Go 1.24 and live PostgreSQL validation must be completed by the operator.
