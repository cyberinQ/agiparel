#!/usr/bin/env bash
# Author: Sunil Jain, CyberinQ LLC, 2026-10-29.
# Non-destructive v43 patch: verify -> inflate -> test -> build -> health -> public domain -> git.
set -Eeuo pipefail
: "${1:?Usage: bash installer ZIP EXPECTED_SHA256}"
: "${2:?Supply the exact SHA256 printed with this release}"
ZIP="$1"; SHA="$2"
cd "$(git rev-parse --show-toplevel)"
command -v docker >/dev/null; command -v unzip >/dev/null; command -v node >/dev/null; command -v go >/dev/null
[ "$(git branch --show-current)" = main ] || { echo 'ERROR: main branch required' >&2; exit 1; }
current="$(git rev-parse HEAD)"
case "$current" in 7a8df62*) ;; *) echo "ERROR: expected v42 commit 7a8df62, found $current" >&2; exit 1;; esac
[ -z "$(git status --porcelain)" ] || { echo 'ERROR: uncommitted work detected; backup/stash it before patch.' >&2; exit 1; }
[ "$(shasum -a 256 "$ZIP" | awk '{print $1}')" = "$SHA" ] || { echo 'ERROR: patch checksum mismatch' >&2; exit 1; }
unzip -tq "$ZIP" >/dev/null
printf '\n=== 1 INFLATE v43 PATCH (NO DB RESET / NO MIGRATION) ===\n'
unzip -oq "$ZIP" -d .
chmod +x scripts/install_release_v43.sh scripts/verify_public_release_v43.sh
printf '\n=== 2 CHECK AND PROVISION ENGINEER OPS SECRET LOCALLY ===\n'
[ -f .env ] || { echo 'ERROR: local .env is missing. v42 credentials must be retained.' >&2; exit 1; }
# Preserve any prior secret. Only create an administrator secret once.
if ! grep -q '^AGIPAREL_ENGINEER_OPS_PASSWORD=' .env; then
   printf '\nAGIPAREL_ENGINEER_OPS_PASSWORD=%s\n' "$(openssl rand -hex 32)" >> .env
fi
if ! grep -q '^AGIPAREL_ENGINEER_OPS_ENABLED=' .env; then printf 'AGIPAREL_ENGINEER_OPS_ENABLED=true\n' >> .env; fi
if ! grep -q '^AGIPAREL_ENGINEER_OPS_USER=' .env; then printf 'AGIPAREL_ENGINEER_OPS_USER=engineerops\n' >> .env; fi
if ! grep -q '^AGIPAREL_ENGINEER_OPS_TRUST_PROXY_TLS=' .env; then printf 'AGIPAREL_ENGINEER_OPS_TRUST_PROXY_TLS=false\n' >> .env; fi
chmod 600 .env
printf 'Engineering secret was generated/preserved in .env (not printed, not committed).\n'
printf '\n=== 3 TEST STATIC CONTRACTS + GO / RACE / VET ===\n'
node scripts/test_factory_v43_public_admin.cjs
GOFLAGS=-mod=readonly go test -count=1 ./...
GOFLAGS=-mod=readonly go test -race -count=1 ./...
GOFLAGS=-mod=readonly go vet ./...
for t in scripts/test_*.cjs; do node "$t"; done
printf '\n=== 4 BUILD + RESTART EXISTING DEV DEMO (DB PERSISTED) ===\n'
docker compose --env-file .env -f compose/docker-compose.dev.yml up -d --build
curl -fsS --retry 10 --retry-delay 2 --retry-connrefused http://127.0.0.1:18530/healthz
echo
printf '\n=== 5 VERIFY LIVE PUBLIC DOMAIN ===\n'
if ! bash scripts/verify_public_release_v43.sh; then
   printf 'Public routing not yet updated. Local v43 code is running. Fix the reverse proxy/tunnel; then run bash scripts/finalize_public_release_v43.sh.\n' >&2
   exit 2
fi
printf '\n=== 6 COMMIT + PUSH AFTER PUBLIC VERIFICATION ===\n'
bash scripts/finalize_public_release_v43.sh
