#!/usr/bin/env bash
# Author: Sunil Jain, CyberinQ LLC, 2026-10-29.
# Corrected v43.2: decorative fabric waves only on home; Engineer Ops metrics/catalogs.
# No database reset, no v44 authentik rollout.
set -Eeuo pipefail
: "${1:?Provide patch ZIP path}"
: "${2:?Provide SHA-256 checksum}"
ZIP="$1"; EXPECTED="$2"
cd "$(git rev-parse --show-toplevel)"
export GIT_PAGER=cat PAGER=cat
[ "$(git branch --show-current)" = main ] || { echo 'ERROR: main branch required' >&2; exit 1; }
for tool in docker python3 node go git unzip curl shasum; do
  command -v "$tool" >/dev/null || { echo "ERROR: missing $tool" >&2; exit 1; }
done
[ "$(shasum -a 256 "$ZIP" | awk '{print $1}')" = "$EXPECTED" ] || { echo 'ERROR: ZIP SHA mismatch' >&2; exit 1; }
unzip -tq "$ZIP" >/dev/null
# Refuse unrelated working tree edits, but permit restart of this exact patch.
PATCH_PATHS="$(unzip -Z1 "$ZIP")"
while IFS= read -r line; do
  [ -n "$line" ] || continue
  path="${line:3}"
  if ! printf '%s\n' "$PATCH_PATHS" | grep -Fx -- "$path" >/dev/null; then
    echo "ERROR: unrelated local change: $path" >&2; exit 1
  fi
done < <(git status --porcelain --untracked-files=all)
python3 - "$ZIP" <<'PYBASE'
from pathlib import Path
from hashlib import sha256
import sys,zipfile
z=zipfile.ZipFile(sys.argv[1])
checks=[('.github/workflows/ci.yml', '50da796e3d8c997927cbc5f43ae6ef9e73e7acbc7fd9d952270d34b7dcea0944'), ('.github/workflows/release.yml', '4545874c5facd056458c34e23a0ecac6f08665819e717bccd11e4202d95bc099'), ('compose/docker-compose.dev.yml', '91f810d6efd1cfe7d8bfb3e6ba64f90397579aefa8aed9427b4a41b258f34b3d'), ('scripts/test_v42_baseline_contract.cjs', '912f14b73d2ca734a1f486aec51b703e0a84d683fdf0a09dcbfbc637aec1cd8b'), ('services/edge-daemon/cmd/server/engineer_ops_v43.go', 'cf6267c59f2c759074e5fc79ebdc7855fbb3a82e0b7711ba2a7688821c7bd09a'), ('services/edge-daemon/internal/web/static/engineer_ops.html', '717b621996dcf2c8bca3e6d088d7bbe62f5a1da64e574fcde47792647ebcc534'), ('services/edge-daemon/internal/web/static/engineer_ops.css', '9db8bd2b15ace47a9f86e0e34f454cd9420c93f6bda8cc80c8ff6e34ce304424'), ('services/edge-daemon/internal/web/static/engineer_ops.js', '57362c0f62cfd02c01f9f1a329aff5b4afcdef1eb807804943cf755bf33ac938'), ('services/edge-daemon/internal/web/static/portal.html', 'c1574d229e6a96a4e91060cebda34d6323a80c6d5b146e13c55b4aa3688697ff'), ('services/edge-daemon/internal/web/static/portal.css', 'b592d84099e1f4af9ed7dd098cf6096e4a0517ea6a8a7006de3bad92eb14fe03'), ('services/edge-daemon/internal/web/static/portal.js', '333c3df2070ed6b54db3f2c4efcb4019c282c8f11d87d36c2e7293d8638bea31')]
for path, expected in checks:
 p=Path(path)
 if not p.is_file(): raise SystemExit('ERROR: missing expected v43.1 source: '+path)
 actual=sha256(p.read_bytes()).hexdigest()
 deployed=sha256(z.read(path)).hexdigest()
 if actual not in (expected,deployed):
  raise SystemExit('ERROR: file diverged from v43.1 and patch: '+path)
print('PASS: affected files match v43.1 or exact v43.2 patch bytes')
PYBASE
printf '\n=== 1 APPLY PATCH (NO BASELINE REBUILD) ===\n'
unzip -oq "$ZIP" -d .
chmod +x scripts/install_release_v43_2_revised.sh
printf '\n=== 2 BACKUP DB BEFORE ADDITIVE MIGRATION ===\n'
COMPOSE=(docker compose --env-file .env -f compose/docker-compose.dev.yml)
"${COMPOSE[@]}" up -d postgres
mkdir -p "$HOME/agiparel-db-backups"
STAMP="$(date +%Y%m%d_%H%M%S)"
BACKUP="$HOME/agiparel-db-backups/pre_v43_2_${STAMP}_$$.dump"
"${COMPOSE[@]}" exec -T postgres sh -c 'exec pg_dump -U "$POSTGRES_USER" -d "$POSTGRES_DB" -Fc' > "$BACKUP"
[ -s "$BACKUP" ] || { echo 'ERROR: empty backup' >&2; exit 1; }
echo "Backup: $BACKUP"
printf '\n=== 3 APPLY COUNT-ONLY POSTGRESQL MIGRATION ===\n'
"${COMPOSE[@]}" exec -T postgres sh -c 'exec psql -X -U "$POSTGRES_USER" -d "$POSTGRES_DB" -v ON_ERROR_STOP=1' < services/core-api/migrations/0002_engineer_ops_aggregates_v43_2.sql
"${COMPOSE[@]}" exec -T postgres sh -c 'exec psql -X -U "$POSTGRES_USER" -d "$POSTGRES_DB" -v ON_ERROR_STOP=1' < scripts/test_factory_v43_2_ops_metrics_pg.sql
printf '\n=== 4 REGRESSIONS / GO RACE / VET ===\n'
for f in scripts/test_*.cjs; do node "$f" >/dev/null; done
node scripts/test_factory_v43_2_home_ops.cjs
gofmt -l services/edge-daemon/cmd/server/engineer_ops_v43.go services/edge-daemon/cmd/server/engineer_ops_api_catalog_v43_2.go | grep -q . && { echo 'ERROR: gofmt needed' >&2; exit 1; }
GOFLAGS=-mod=readonly go test -count=1 ./...
GOFLAGS=-mod=readonly go test -race -count=1 ./...
GOFLAGS=-mod=readonly go vet ./...
# v43.2 EOF whitespace hotfix: normalize the only affected asset before diff check.
python3 - <<'PYWHITESPACE'
from pathlib import Path
p=Path("services/edge-daemon/internal/web/static/portal.js")
p.write_bytes(p.read_bytes().rstrip(b"\r\n")+b"\n")
PYWHITESPACE
git diff --check
printf '\n=== 5 BUILD AND DEPLOY THE EXISTING EDGE SERVER ===\n'
"${COMPOSE[@]}" up -d --build plant-edge-os
curl -fsS --retry 12 --retry-delay 2 --retry-connrefused http://127.0.0.1:18530/healthz
echo
printf '\n=== 6 VERIFY LOCAL AND PUBLIC HOME ===\n'
LOCAL="$(curl -fsS http://127.0.0.1:18530/)"
[[ "$LOCAL" == *'id="agiparel-wave-canvas"'* && "$LOCAL" == *'Every garment<br><em>tells a story.</em>'* ]] || { echo 'ERROR: local Home does not show exact old copy plus wave canvas' >&2; exit 1; }
PUBLIC="$(curl -fsSL --connect-timeout 10 --max-time 35 'https://agiparel.empowerai.us/?release=v43_2_corrected')"
[[ "$PUBLIC" == *'id="agiparel-wave-canvas"'* && "$PUBLIC" == *'Every garment<br><em>tells a story.</em>'* ]] || { echo 'ERROR: public site not serving corrected v43.2; check reverse proxy and cache' >&2; exit 2; }
JS="$(curl -fsSL --connect-timeout 10 --max-time 35 'https://agiparel.empowerai.us/demo-assets/portal.js?release=v43_2_corrected')"
[[ "$JS" == *'fabric-thread decorative waves only'* ]] || { echo 'ERROR: public portal JS still cached or routed to another origin' >&2; exit 2; }
# Confirm admin route exists, but do not disclose its credentials or bypass its gate.
STATUS="$(curl -sS -o /dev/null -w '%{http_code}' --connect-timeout 10 --max-time 20 'https://agiparel.empowerai.us/engineer-ops')"
[[ "$STATUS" == 401 || "$STATUS" == 200 ]] || { echo "ERROR: Engineer Ops public access status $STATUS (expected 401 without credentials)" >&2; exit 2; }
printf '\n=== 7 COMMIT AND PUSH ===\n'
git add -- .github/workflows/ci.yml \
  .github/workflows/release.yml \
  compose/docker-compose.dev.yml \
  docs/V43_2_WAVES_ENGINEER_OPS.md \
  scripts/install_release_v43_2_revised.sh \
  scripts/test_factory_v43_2_ops_metrics_pg.sql \
  scripts/test_factory_v43_2_home_ops.cjs \
  scripts/test_v42_baseline_contract.cjs \
  services/core-api/migrations/0002_engineer_ops_aggregates_v43_2.sql \
  services/edge-daemon/cmd/server/engineer_ops_v43.go \
  services/edge-daemon/cmd/server/engineer_ops_v43_2_test.go \
  services/edge-daemon/cmd/server/engineer_ops_api_catalog_v43_2.go \
  services/edge-daemon/internal/web/static/engineer_ops.html \
  services/edge-daemon/internal/web/static/engineer_ops.css \
  services/edge-daemon/internal/web/static/engineer_ops.js \
  services/edge-daemon/internal/web/static/portal.html \
  services/edge-daemon/internal/web/static/portal.css \
  services/edge-daemon/internal/web/static/portal.js
git diff --cached --check
git --no-pager diff --cached --shortstat
if ! git diff --cached --quiet; then git -c core.pager=cat commit -m 'feat(factory): subtle fabric waves and Engineer Ops analytics v43.2'; fi
git push origin main
[[ "$(git rev-parse HEAD)" = "$(git rev-parse origin/main)" ]] || { echo 'ERROR: Git remote hash mismatch' >&2; exit 1; }
git status --short
git log -1 --oneline
printf 'PASS: corrected v43.2 deployed and pushed; no factory data reset.\n'
