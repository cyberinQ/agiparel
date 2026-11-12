#!/usr/bin/env bash
# Author: Sunil Jain, 2026-11-12.
# Validate a reviewed v50 patch and optionally publish its explicit files to GitHub.
set -Eeuo pipefail
cd "$(dirname "$0")/.."
MODE="${1:---check}"
[[ "$MODE" == --check || "$MODE" == --push ]] || { echo 'Usage: --check or --push' >&2; exit 2; }
[[ "$(git branch --show-current)" == main ]] || { echo 'STOP: expected main branch' >&2; exit 1; }
command -v node >/dev/null && command -v go >/dev/null && command -v python3 >/dev/null || {
  echo 'STOP: Node 22, Go 1.26 and Python3 required' >&2; exit 1;
}
FILES=(
  .gitignore README.md compose/docker-compose.dev.yml
  compose/docker-compose.factory-v50.yml
  docs/FACTORY_EDGE_V50_RELEASE_CANDIDATE.md
  scripts/factory_pilot_v50.sh scripts/package_factory_v50.py
  scripts/ship_factory_v50_macbook.sh scripts/test_factory_v50_contract.cjs
  .github/workflows/factory-pilot-v50.yml
  services/aerolake-bridge/main.go services/aerolake-bridge/retention_v50_test.go
  services/edge-daemon/cmd/server/main.go
  services/edge-daemon/cmd/server/operational_queries.go
  services/edge-daemon/cmd/server/operational_counts_v50_test.go
  services/edge-daemon/pkg/aggieintent/classifier.go
  services/edge-daemon/pkg/aggieintent/classifier_test.go
)
git fetch origin main
[[ "$(git rev-parse HEAD)" == "$(git rev-parse origin/main)" ]] || {
  echo 'STOP: local main differs from origin/main. Reconcile before publishing.' >&2; exit 1;
}
# Untracked local setup artifacts are ignored, but any unexpected source edit stops.
while IFS= read -r entry; do
  [[ -n "$entry" ]] || continue
  path="${entry:3}"
  matched=no
  for allowed in "${FILES[@]}"; do [[ "$path" != "$allowed" ]] || matched=yes; done
  [[ "$matched" == yes ]] || { echo "STOP: unrelated change $path" >&2; exit 1; }
done < <(git status --porcelain --untracked-files=no)
export GOFLAGS=-mod=readonly
echo '=== Go formatting, regression tests, build and v50 release checks ==='
if [[ -n "$(gofmt -l services/edge-daemon/cmd/server/main.go services/edge-daemon/cmd/server/operational_queries.go services/edge-daemon/cmd/server/operational_counts_v50_test.go services/edge-daemon/pkg/aggieintent/*.go services/aerolake-bridge/main.go services/aerolake-bridge/retention_v50_test.go)" ]]; then
  echo 'STOP: Go files not formatted' >&2; exit 1
fi
for script in scripts/test_*.cjs; do node "$script" >/dev/null; done
go test -count=1 ./...
go test -race -count=1 ./services/edge-daemon/pkg/aggieintent ./services/aerolake-bridge ./services/edge-daemon/cmd/server
go vet ./...
go build ./...
python3 scripts/package_factory_v50.py dist/agiparel-factory-v50-pilot-source.zip
printf 'PASS: source tested; %s\n' "$(git rev-parse --short HEAD)"
if [[ "$MODE" == --check ]]; then echo 'No Git changes made by this command.'; exit 0; fi
git add -- "${FILES[@]}"
[[ -n "$(git diff --cached --name-only)" ]] || { echo 'Already committed; no source changes.'; exit 0; }
git commit -m 'feat(factory): v50 Aggie exact counts and isolated fail-closed factory pilot'
git push origin main
echo "PASS: pushed v50 source to origin/main at $(git rev-parse --short HEAD)"
echo 'NEXT: Wait for GitHub Actions verify + postgres-schema; do not deploy to iMac before green CI.'
