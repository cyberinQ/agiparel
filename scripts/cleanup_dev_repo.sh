#!/usr/bin/env bash
# Author: Sunil Jain, 2026-10-29.
# Safe reviewed cleanup inventory; v42 installer owns all actual removals.
set -euo pipefail
[[ "${1:-}" == '--dry-run' && "$#" == 1 ]] || {
  echo 'Usage: bash scripts/cleanup_dev_repo.sh --dry-run';
  echo 'The v42 release installer performs reviewed removals after database equivalence.';
  exit 2;
}
[[ -f scripts/v42_delete_manifest.txt ]] || { echo 'ERROR: no reviewed cleanup manifest'; exit 1; }
printf 'Reviewed v42 cleanup targets (no files removed):\n'
cat scripts/v42_delete_manifest.txt
