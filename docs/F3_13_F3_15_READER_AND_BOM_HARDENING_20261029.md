# Agiparel F3.13 + F3.15 — LLRP truth and BOM-based planning

Author: Sunil Jain, CyberinQ LLC · 2026-10-29

## Scope and release boundary

This is a **source hardening bundle** of F3.13's LLRP reader core and F3.15's planning workbook evidence path. It is **not** completion of device health/alert delivery, not a finished CAD nesting optimizer, and not certification of physical scanner accuracy. F3.14 (trims stock issuance and signed witness transaction) remains a separate migration; do not apply the unreviewed third-party migration numbered `0042` because F3.10 already occupies 0042.

No new database table, migration, production feature flag, or production credit is introduced. Existing F3.1–F3.12 APIs remain unchanged. This bundle changes these existing functions only:

- `pkg/rfid/llrp_reader.go`: bounded frame reader and parser for LLRP 1.x `RO_ACCESS_REPORT` (message 61), `TagReportData` (TLV 240), `EPC-96` (TV 13), `EPCData` (TLV 241), optional AntennaID (TV 1), PeakRSSI (TV 6), FirstSeenTimestampUTC (TV 2). Supports multiple tags per report, validates TV/TLV lengths and EPC padding, acknowledges reader KEEPALIVE (message 62 → 72, same message ID), reconnects TCP with bounded delay, maintains local thread-safe connection/read/rejection counters. Unknown TV types and malformed frames fail closed. Missing optional RF/time values remain absent/zero with explicit measured booleans; never impersonate real observations using adapter receive time.
- `pkg/onboarding/workbook.go`: optional `STYLE_BOM,style,material,FABRIC|TRIM,quantity_per_garment,UOM,source_reference` CSV rows, with finite positive quantities. This is *declared workbook input*, not cryptographically approved master data. Older workbooks remain parseable.
- `pkg/planning/analyzer.go`: per-style/per-material fabric meters from explicit BOM, never inferred from style-part count or universal meters-per-garment constants. UOM must be meters for fabric; each SKU reconciles independently against workbook-reported eligible rolls excluding reservations belonging to other POs. A BOM omitting the style master shell SKU fails to UNKNOWN. Missing/invalid BOM or ambiguous stock is UNKNOWN (YELLOW). Declared deficits are RED **reported deficits only**, not authorized purchase orders. Even when reported stock covers declared BOM, the result is YELLOW, pending signed/approved marker and live roll reconciliation.

**Source cautions:** Real LLRP frames do not themselves grant signed WIP ingestion. Connection health is local to the reader instance. Configure trusted reader networks and authorized RO reports separately. A valid EPC is not a proof of tag-to-physical-item binding. The F3.3 signed gateway and F3.11 outbox/F3.12 acceptance controls remain responsible for source attestation and replay decisions. Do not convert provisional planner cards to approved purchase/issue commands.

## Tests

Local: Go 1.23 isolated STD library/workbook/planning packages (race and vet); all existing 38 `.cjs` regression scripts plus new source contract. Install on iMac using Go 1.24: `go test -count=1 ./...`, `go test -race -count=1 ./...`, `go vet ./...`, `go build ./...`, and `for script in scripts/test_*.cjs; do node "$script"; done`.

**Hardware acceptance pending:** packet captures from at least two configured reader models, real EPC-96 and variable EPC, absent optional parameter behavior, unreadable/noisy readers, binary fragmentation, power/network interruption, reconnect storms, KEEPALIVE and reboot, scanner clock skew, cross-read antenna verification, tag reuse, independent scanned-item witness, reader firmware/version and key controls. Compare downstream signed scans against exact source event bytes; do not synthesize production evidence.

**Planning acceptance pending:** approved source BOM/marker revisions and actual per-size marker yields; width/shade/lot-aware live reservations and permitted waste allowances; buyer-order matching; cross-plant isolation and reviewer approval. Until then the workbook result must remain non-authoritative.

## Next safely parallel increments

- **F3.14**: new numbered migration 0043 with transactional trims quantity conservation and an explicitly verified two-witness application boundary. Do not treat free-text issuer/recipient IDs or stored hashes as proof verification.
- **F3.13B**: connect parser health to read-only commissioning telemetry and calibrated per-reader heartbeat/backlog policy, with a fail-closed feature gate.
- **F3.15B**: consume verified intake BOM and approved per-size marker yields from authoritative stores/cutting data; prove estimate-to-actual reconciliation before GREEN cutting release.
