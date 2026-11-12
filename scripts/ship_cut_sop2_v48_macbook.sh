#!/usr/bin/env bash
# Author: Sunil Jain, 2026-11-12.
# Test and publish only the reviewed v46-v48 cutting-source delta from the MacBook to origin/main.
set -Eeuo pipefail
cd "$(dirname "$0")/.."
[[ "$(git rev-parse --show-toplevel)" == "$(pwd -P)" ]] || { echo 'Run in the AGIPAREL checkout' >&2; exit 1; }
[[ "$(git branch --show-current)" == main ]] || { echo 'STOP: main branch required' >&2; exit 1; }
[[ -n "$(git remote get-url origin)" ]] || { echo 'STOP: origin remote is not configured' >&2; exit 1; }
[[ ! -f .env || -z "$(git ls-files -- .env)" ]] || { echo 'STOP: .env is tracked. Remove secrets from git before proceeding' >&2; exit 1; }
command -v go >/dev/null && command -v node >/dev/null || { echo 'Go and Node are required' >&2; exit 1; }

git fetch origin main
[[ "$(git rev-parse HEAD)" == "$(git rev-parse refs/remotes/origin/main)" ]] || {
  echo 'STOP: local main differs from GitHub main. Reconcile history without force-pushing.' >&2; exit 1;
}

# Whitelist constructed from the three independently installed and source-hashed patches,
# plus the reviewed opt-in v48 release change. Never use git add -A for a release.
FILES=(
  .gitignore .github/workflows/ci.yml .github/workflows/release.yml
  compose/docker-compose.dev.yml
  services/edge-daemon/Dockerfile
  services/edge-daemon/cmd/server/main.go
  services/edge-daemon/cmd/server/portal.go
  services/edge-daemon/cmd/server/public_cutter_demo_v48.go
  services/edge-daemon/cmd/server/public_cutter_demo_v48_test.go
  services/edge-daemon/cmd/server/cutting_lay_drafts.go
  services/edge-daemon/cmd/server/cutting_sop2_api.go
  services/edge-daemon/cmd/server/cutting_sop2_bridge.go
  services/edge-daemon/internal/web/assets.go
  services/edge-daemon/internal/web/static/cutting_view.html
  services/edge-daemon/internal/web/static/cutting_sop2.html
  services/edge-daemon/internal/web/static/cutting_sop2.css
  services/edge-daemon/internal/web/static/cutting_sop2.js
  services/edge-daemon/internal/web/static/cutter_demo.html
  services/edge-daemon/internal/web/static/cutter_demo.css
  services/edge-daemon/internal/web/static/cutter_demo.js
  services/edge-daemon/internal/web/static/cutter_demo_model.js
  services/edge-daemon/pkg/cutting/sop2/targets.go
  services/edge-daemon/pkg/cutting/sop2/fabric.go
  services/edge-daemon/pkg/cutting/sop2/execution.go
  services/edge-daemon/pkg/cutting/sop2/sop2_test.go
  services/core-api/migrations/0005_cut_sop2_control_plane_v46.sql
  scripts/agiparel_macbook_localdev.sh
  scripts/test_cut_sop2_contract.cjs
  scripts/test_cut_sop2_v46_schema_pg.sql
  scripts/test_cutter_mode_v47.cjs
  scripts/test_public_cutter_v48.cjs
  scripts/run_cut_sop2_v46_checks.sh
  scripts/ship_cut_sop2_v48_macbook.sh
  scripts/deploy_cut_sop2_v48_imac.sh
  docs/CUT_SOP2_NATIVE_SUPERSET_20261111.md
  docs/CUTTER_MODE_MACBOOK_DEMO_V47.md
  docs/CUT_SOP2_GITHUB_IMAC_RELEASE_V48.md
)
for f in "${FILES[@]}"; do [[ -f "$f" ]] || { echo "STOP: release file missing: $f" >&2; exit 1; }; done
# Permit an earlier --check invocation to have staged only approved paths.
while IFS= read -r staged; do
  found=0
  for file in "${FILES[@]}"; do [[ "$file" == "$staged" ]] && { found=1; break; }; done
  [[ "$found" == 1 ]] || { echo "STOP: unrelated file already staged: $staged" >&2; exit 1; }
done < <(git diff --cached --name-only)

echo '=== MACBOOK: tests, static checks, vet and build ==='
node scripts/test_cutter_mode_v47.cjs
node scripts/test_public_cutter_v48.cjs
bash scripts/run_cut_sop2_v46_checks.sh --test
GOFLAGS=-mod=readonly go test -count=1 ./...
GOFLAGS=-mod=readonly go vet ./...
GOFLAGS=-mod=readonly go build ./...
[[ -z "$(gofmt -l services/edge-daemon/cmd/server/public_cutter_demo_v48*.go services/edge-daemon/cmd/server/portal.go services/edge-daemon/internal/web/assets.go)" ]] || { echo 'STOP: gofmt required' >&2; exit 1; }

git add -- "${FILES[@]}"
git diff --cached --check
# Verify no extra source paths were staged by checking exact expected whitelist.
while IFS= read -r staged; do
  found=0
  for file in "${FILES[@]}"; do [[ "$file" == "$staged" ]] && { found=1; break; }; done
  [[ "$found" == 1 ]] || { echo "STOP: unexpected staged path: $staged" >&2; exit 1; }
done < <(git diff --cached --name-only)

git --no-pager diff --cached --stat
if git diff --cached --quiet; then echo 'No source delta to publish'; exit 0; fi
if [[ "${1:-}" != --push ]]; then
  echo 'CHECK ONLY: run bash scripts/ship_cut_sop2_v48_macbook.sh --push to commit and push reviewed source.'
  exit 0
fi

git commit -m 'feat(cutting): native SOP-2 engine, cutter training and guarded public demo v48'
git push origin main
[[ "$(git rev-parse HEAD)" == "$(git rev-parse refs/remotes/origin/main)" ]] || { echo 'STOP: GitHub push not reflected in remote tracking reference' >&2; exit 1; }
echo "PASS: pushed AGIPAREL v48 source $(git rev-parse --short HEAD) to origin/main"
echo 'Before iMac deployment, verify GitHub Actions CI passes for this exact commit.'
