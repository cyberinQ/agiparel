#!/usr/bin/env bash
# Author: Sunil Jain, 2026-11-12.
# Validate ten worker chapter guides and protected Aggie support before a fast-forward GitHub publish.
set -Eeuo pipefail
cd "$(dirname "$0")/.."
MODE="${1:---check}"
[[ "$MODE" == --check || "$MODE" == --push ]] || { echo 'Usage: [--check|--push]' >&2; exit 2; }
[[ "$(git branch --show-current)" == main ]] || { echo 'STOP: main branch required' >&2; exit 1; }
command -v node >/dev/null && command -v go >/dev/null || { echo 'Node and Go are required' >&2; exit 1; }
git fetch origin main
[[ "$(git rev-parse HEAD)" == "$(git rev-parse refs/remotes/origin/main)" ]] || { echo 'STOP: local main does not match GitHub; reconcile without force' >&2; exit 1; }
FILES=(
  docs/AGGIE_WORKER_GUIDE_AUDIT_V49.md
  scripts/ship_worker_guides_v49_macbook.sh
  scripts/smoke_aggie_worker_v49.sh
  scripts/test_worker_manual_v49.cjs
  services/edge-daemon/cmd/server/aggie_worker_manual_v49.go
  services/edge-daemon/cmd/server/aggie_worker_manual_v49_test.go
  services/edge-daemon/cmd/server/main.go
  services/edge-daemon/internal/agie/interviewer/handler.go
  services/edge-daemon/internal/agie/interviewer/manual_tool_v49_test.go
  services/edge-daemon/internal/agie/interviewer/tools.go
  services/edge-daemon/internal/web/static/buyer_review.html
  services/edge-daemon/internal/web/static/cutting_sop2.html
  services/edge-daemon/internal/web/static/cutting_view.html
  services/edge-daemon/internal/web/static/documents.html
  services/edge-daemon/internal/web/static/factory_intelligence.html
  services/edge-daemon/internal/web/static/factory_intelligence_production.html
  services/edge-daemon/internal/web/static/factory_setup.html
  services/edge-daemon/internal/web/static/material_planning.html
  services/edge-daemon/internal/web/static/order_intake.html
  services/edge-daemon/internal/web/static/packing_execution.html
  services/edge-daemon/internal/web/static/portal.js
  services/edge-daemon/internal/web/static/quality_execution.html
  services/edge-daemon/internal/web/static/sewing_execution.html
  services/edge-daemon/internal/web/static/stores.html
  services/edge-daemon/internal/web/static/worker_help_v49.css
  services/edge-daemon/internal/web/static/worker_help_v49.js
  services/edge-daemon/internal/workerhelp/guide.go
  services/edge-daemon/internal/workerhelp/guide.json
  services/edge-daemon/internal/workerhelp/guide_test.go
)
for path in "${FILES[@]}"; do [[ -f "$path" ]] || { echo "STOP: release file missing: $path" >&2; exit 1; }; done
while IFS= read -r file; do
  found=0
  for allowed in "${FILES[@]}"; do [[ "$file" == "$allowed" ]] && { found=1; break; }; done
  [[ "$found" == 1 ]] || { echo "STOP: unrelated staged file: $file" >&2; exit 1; }
done < <(git diff --cached --name-only)
while IFS= read -r file; do
  found=0
  for allowed in "${FILES[@]}"; do [[ "$file" == "$allowed" ]] && { found=1; break; }; done
  [[ "$found" == 1 ]] || { echo "STOP: unrelated modified file: $file" >&2; exit 1; }
done < <(git diff --name-only)

echo '=== Test all chapters, Aggie and the application ==='
for script in scripts/test_*.cjs; do node "$script"; done
GOFLAGS=-mod=readonly go test -count=1 ./...
GOFLAGS=-mod=readonly go test -race -count=1 ./services/edge-daemon/internal/workerhelp ./services/edge-daemon/internal/agie/interviewer ./services/edge-daemon/cmd/server
GOFLAGS=-mod=readonly go vet ./...
GOFLAGS=-mod=readonly go build ./...
[[ -z "$(gofmt -l services/edge-daemon/internal/workerhelp/*.go services/edge-daemon/internal/agie/interviewer/*.go services/edge-daemon/cmd/server/aggie_worker_manual_v49*.go services/edge-daemon/cmd/server/main.go)" ]] || { echo 'STOP: Go files need gofmt' >&2; exit 1; }
git diff --check
if [[ "$MODE" == --check ]]; then
  echo 'PASS: manual and Aggie release source is validated; no Git commit or push performed.'
  exit 0
fi

git add -- "${FILES[@]}"
git diff --cached --check
while IFS= read -r file; do
  found=0
  for allowed in "${FILES[@]}"; do [[ "$file" == "$allowed" ]] && { found=1; break; }; done
  [[ "$found" == 1 ]] || { echo "STOP: unrelated staged file: $file" >&2; exit 1; }
done < <(git diff --cached --name-only)
if git diff --cached --quiet; then echo 'No changes to publish'; exit 0; fi
git --no-pager diff --cached --stat
git commit -m 'feat(worker-help): ten chapter guides and grounded Aggie manual answers v49'
git push origin main
echo "PUSHED: $(git rev-parse --short HEAD)"
echo 'Wait for GitHub CI verify and postgres-schema success before iMac deployment.'
