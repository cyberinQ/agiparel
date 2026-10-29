#!/usr/bin/env bash
# Author: Sunil Jain, CyberinQ LLC, 2026-10-29.
# v43.1: targeted no-migration repair of Aggie and HTTPS Engineer Ops.
set -Eeuo pipefail
: "${1:?Usage: bash installer PATCH_ZIP EXPECTED_SHA256}"
: "${2:?Provide patch SHA256}"
ZIP="$1"; EXPECTED="$2"
cd "$(git rev-parse --show-toplevel)"
[[ "$(git branch --show-current)" == "main" ]] || { echo "ERROR: main branch required" >&2; exit 1; }
[[ -z "$(git diff --cached --name-only)" ]] || { echo "ERROR: staged edits already present; preserve or commit them first" >&2;exit 1; }
[[ "$(shasum -a 256 "$ZIP" | awk '{print $1}')" == "$EXPECTED" ]] || { echo "ERROR: wrong zip checksum" >&2;exit 1; }
unzip -tq "$ZIP" >/dev/null
command -v go >/dev/null; command -v node >/dev/null; command -v docker >/dev/null; command -v python3 >/dev/null
printf "\n=== 1 VERIFY EXISTING v43 FILES ===\n"
python3 - <<'PY_CHECK'
from pathlib import Path
from hashlib import sha256
checks = [
    ('services/edge-daemon/cmd/server/engineer_ops_v43.go', '4a609166f8af754cd9e441f3b8f83188a545b25e6dc74569f55561b2cf122691'),
    ('services/edge-daemon/cmd/server/engineer_ops_v43_test.go', 'b78d4074bc9780a3eb0489683a4af4fb198dfc344fa0764e910696ed5c57ad08'),
    ('services/edge-daemon/cmd/server/operational_queries.go', '43c1866a904ef865ca0e55e639cc3e90c0e5013252c14f3ffe2256504be1bdb0'),
    ('services/edge-daemon/cmd/server/operational_queries_test.go', '1957a47b7e7e5adc4d83cb9b85215155c56affc7759e31e9de81fd614fd88d60'),
    ('services/edge-daemon/internal/web/static/portal.css', '36a05879340f2883c243815c796a4c5b6faa072804f8b1623d741cf7e83ccadb'),
    ('services/edge-daemon/internal/web/static/portal.html', 'a2958fee3c3ebdfd9cb2b7223efbccfe604a63d723a87f5d80f202d1046bf2e9'),
]
for name,expected in checks:
    p=Path(name)
    if not p.is_file() or sha256(p.read_bytes()).hexdigest()!=expected:
        raise SystemExit(f"ERROR: unexpected existing source {name}; no patch applied")
print("PASS: existing source files verified")
PY_CHECK
printf "\n=== 2 INFLATE v43.1 REPAIR (NO DATABASE RESET) ===\n"
unzip -oq "$ZIP" -d .
chmod +x scripts/install_release_v43_1.sh scripts/enable_engineer_ops_proxy_v43_1.sh
printf "\n=== 3 RUN REGRESSION / RACE / VET ===\n"
for f in scripts/test_*.cjs; do node "$f"; done
GOFLAGS=-mod=readonly go test -count=1 ./...
GOFLAGS=-mod=readonly go test -race -count=1 ./...
GOFLAGS=-mod=readonly go vet ./...
printf "\n=== 4 BUILD + DEPLOY WITHOUT DATABASE REBUILD ===\n"
docker compose --env-file .env -f compose/docker-compose.dev.yml up -d --build
curl -fsS --retry 10 --retry-delay 2 --retry-connrefused http://127.0.0.1:18530/healthz
echo
printf "\n=== 5 ENABLE TRUSTED HTTPS PROXY FOR ENGINEER OPS ===\n"
bash scripts/enable_engineer_ops_proxy_v43_1.sh
printf "\n=== 6 VERIFY ACTUAL PUBLIC HOME ===\n"
bash scripts/verify_public_release_v43.sh
PUBLIC_URL="${AGIPAREL_PUBLIC_URL:-https://agiparel.empowerai.us}"
curl -fsSL --max-time 20 "$PUBLIC_URL/" | grep -Fq 'title="Restricted engineering dashboard"' || { echo "ERROR: public home did not pick up v43.1" >&2;exit 2; }
printf "\n=== 7 GIT COMMIT / PUSH ===\n"
git diff --check
git add -- \
  README.md \
  compose/Caddyfile.agiparel.example \
  compose/docker-compose.dev.yml \
  compose/docker-compose.production.yml \
  docs/V43_1_ENGINEER_OPS_AGGIE_REPAIR.md \
  docs/V43_PUBLIC_DEMO_AND_ENGINEER_OPS.md \
  scripts/enable_engineer_ops_proxy_v43_1.sh \
  scripts/finalize_public_release_v43.sh \
  scripts/install_release_v43.sh \
  scripts/install_release_v43_1.sh \
  scripts/test_factory_v43_1_agie_ops.cjs \
  scripts/test_factory_v43_public_admin.cjs \
  scripts/verify_public_release_v43.sh \
  services/edge-daemon/cmd/server/engineer_ops_v43.go \
  services/edge-daemon/cmd/server/engineer_ops_v43_test.go \
  services/edge-daemon/cmd/server/main.go \
  services/edge-daemon/cmd/server/operational_queries.go \
  services/edge-daemon/cmd/server/operational_queries_test.go \
  services/edge-daemon/cmd/server/portal.go \
  services/edge-daemon/internal/web/assets.go \
  services/edge-daemon/internal/web/static/engineer_ops.css \
  services/edge-daemon/internal/web/static/engineer_ops.html \
  services/edge-daemon/internal/web/static/engineer_ops.js \
  services/edge-daemon/internal/web/static/portal.css \
  services/edge-daemon/internal/web/static/portal.html
git diff --cached --check
git --no-pager diff --cached --shortstat
if ! git diff --cached --quiet; then git -c core.pager=cat commit -m "fix(factory): restore engineer-ops HTTPS and fast named attendance v43.1";fi
git push origin main
[[ "$(git rev-parse HEAD)" == "$(git rev-parse origin/main)" ]] || { echo "ERROR: remote head mismatch" >&2; exit 1; }
git status --short
git log -1 --oneline
echo "PASS: v43.1 verified on public HTTPS and pushed."
