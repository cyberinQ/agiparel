#!/usr/bin/env bash
# Agiparel repository cleanup. Run AFTER inflating the cleanup overlay.
# --apply deletes tracked obsolete docs, all DB dumps and staging data,
# stale root files, and macOS metadata. Never touches .git or Docker volumes.
set -euo pipefail

usage() {
  echo 'Usage: bash scripts/cleanup_dev_repo.sh --apply'
  echo 'Deletes stale repository artifacts. Does not reset PostgreSQL.'
}
[[ "${1:-}" == '--apply' && "$#" == 1 ]] || { usage; exit 2; }
ROOT="$(git rev-parse --show-toplevel 2>/dev/null || true)"
[[ -n "$ROOT" && "$(pwd -P)" == "$(cd "$ROOT" && pwd -P)" ]] || {
  echo 'ERROR: run from the root of the Agiparel Git checkout'; exit 1;
}
[[ -f go.mod && -f services/core-api/migrations/0025_factory_intelligence_snapshots_v12.sql ]] || {
  echo 'ERROR: expected Agiparel source/migration 0025 not found; aborting'; exit 1;
}
[[ -f scripts/ingest_docs_test.go ]] || { echo 'ERROR: tests missing'; exit 1; }

# Test fixtures are not disposable app data. Restore these if the overlay was not
# unpacked yet (the overlay already includes them).
mkdir -p tests/fixtures/intake
if [[ -d data/staging/docs ]]; then
  for f in data/staging/docs/*; do
    [[ -f "$f" ]] || continue
    cp -n "$f" tests/fixtures/intake/ 2>/dev/null || true
  done
fi
[[ -f 'tests/fixtures/intake/agiparel SAMPLE DATA .xlsx' ]] || {
  echo 'ERROR: expected seven regression fixtures absent. Nothing deleted.'; exit 1;
}

# Retain ONLY current normative docs, a concise README, and cleanup guide.
# All historical batch notes and superseded PRDs remain in Git history.
for f in docs/*; do
  [[ -f "$f" ]] || continue
  case "$(basename "$f")" in
    Agiparel_Master_PRD_ARD_v5.3.md|Agiparel_Master_Epics_Stories_Tasks_v5.3.md|Agiparel_Implementation_Plan_v5.3.md|REPOSITORY_CLEANUP.md)
      ;;
    *) git rm -f --ignore-unmatch -- "$f" >/dev/null; rm -f -- "$f";;
  esac
done

# Previous migrations/code remain untouched. Only disposable state is removed.
git rm -r -f --ignore-unmatch -- backups data/staging >/dev/null || true
rm -rf -- backups data/staging

git rm -f --ignore-unmatch -- README.txt agiparel_rfid_simulation.ipynb >/dev/null || true
rm -f -- README.txt agiparel_rfid_simulation.ipynb

# Do not descend into Git's object database.
find . -name .git -prune -o -type f \( -name '.DS_Store' -o -name '._*' \) -exec rm -f -- {} +
find . -name .git -prune -o -type d -name '__MACOSX' -prune -exec rm -rf -- {} +

echo 'CLEANUP DONE. Staged deletions are visible in git status.'
echo 'No database reset was performed by this script.'
git status --short
echo
printf 'Current working tree size (including Git history): '; du -sh . | cut -f1
