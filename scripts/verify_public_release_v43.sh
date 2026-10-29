#!/usr/bin/env bash
# Author: Sunil Jain, CyberinQ LLC, 2026-10-29.
# Fail closed unless real public HTTPS hostname serves the new embedded release.
set -Eeuo pipefail
URL="${AGIPAREL_PUBLIC_URL:-https://agiparel.empowerai.us}"
case "$URL" in https://*) ;; *) echo 'ERROR: Public demo verification requires HTTPS.' >&2; exit 1;; esac
headers="$(mktemp)"; body="$(mktemp)"; trap 'rm -f "$headers" "$body"' EXIT
curl --fail --silent --show-error --location --connect-timeout 10 --max-time 30 --dump-header "$headers" --output "$body" "$URL/"
if ! tr -d '\r' < "$headers" | grep -Eiq '^x-agiparel-release:[[:space:]]*v43[[:space:]]*$'; then
  echo "ERROR: $URL is NOT serving v43. Check DNS, TLS, reverse proxy, tunnel and origin. No Git push should occur." >&2
  exit 1
fi
if ! grep -Fq 'id="why-agiparel"' "$body"; then
  echo 'ERROR: Public site does not contain the new collapsible three-question section.' >&2;exit 1
fi
if ! grep -Fq '3. What is different about Agiparel?' "$body"; then
  echo 'ERROR: Public Q&A section incomplete.' >&2;exit 1
fi
printf 'PASS: %s serves Agiparel v43 public Factory Home.\n' "$URL"
code="$(curl --silent --show-error --location --max-time 20 --output /dev/null --write-out '%{http_code}' "$URL/engineer-ops")"
case "$code" in
  401|403|404|503) printf 'PASS: Engineer Ops anonymous GET is blocked (%s).\n' "$code";;
  *) echo "ERROR: Engineer Ops should not be anonymously accessible (HTTP $code)." >&2; exit 1;;
esac
