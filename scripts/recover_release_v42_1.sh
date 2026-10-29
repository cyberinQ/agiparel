#!/usr/bin/env bash
# Author: Sunil Jain, 2026-10-29.
# Resume the failed v42 schema comparison only after verifying exact partial files.
set -euo pipefail
ZIP="${1:?Pass v42.1 ZIP}"
SHA="${2:?Pass v42.1 SHA256}"
OLDZIP="${3:?Pass original v42 ZIP}"
[[ "$(git rev-parse --show-toplevel)" == "$(pwd -P)" ]] || { echo 'ERROR: run from repo root'; exit 2; }
[[ "$(git rev-parse --short HEAD)" == '96e6add' ]] || { echo 'ERROR: unexpected Git baseline'; exit 2; }
[[ "$(git branch --show-current)" == main ]] || { echo 'ERROR: must be on main'; exit 2; }
[[ -z "$(git diff --name-only)" && -z "$(git diff --cached --name-only)" ]] || {
  echo 'ERROR: tracked working files have changed; preserve and inspect them'; exit 2;
}
printf '%s  %s\n' "$SHA" "$ZIP" | shasum -a 256 -c -
unzip -tq "$ZIP"
# Only two known untracked files were created before the v42 error.
EXPECTED1='scripts/fixtures/v41_legacy_migrations.tar.gz'
EXPECTED2='scripts/generate_baseline_v42.sh'
while IFS= read -r -d '' path; do
  [[ "$path" == "$EXPECTED1" || "$path" == "$EXPECTED2" ]] || {
    echo "ERROR: unexpected untracked file ($path); do not delete user changes"; exit 2;
  }
done < <(git ls-files --others --exclude-standard -z)
[[ -f "$EXPECTED1" && -f "$EXPECTED2" ]] || {
  echo 'ERROR: failed v42 partial files not both present; refusing automatic cleanup'; exit 2;
}
cmp -s "$EXPECTED1" <(unzip -p "$OLDZIP" "$EXPECTED1") || {
  echo 'ERROR: previous fixture differs from original v42 ZIP'; exit 2;
}
cmp -s "$EXPECTED2" <(unzip -p "$OLDZIP" "$EXPECTED2") || {
  echo 'ERROR: previous generator differs from original v42 ZIP'; exit 2;
}
echo '=== VERIFIED PREVIOUS PARTIAL FILES: removing only installer-created copies ==='
rm -- "$EXPECTED1" "$EXPECTED2"
[[ -z "$(git status --porcelain)" ]] || { echo 'ERROR: non-clean repository'; exit 2; }
SCRIPT="$(mktemp)"
trap 'rm -f "$SCRIPT"' EXIT
unzip -p "$ZIP" scripts/install_release_v42.sh > "$SCRIPT"
bash -n "$SCRIPT"
AGIPAREL_V42_RESET_DEV=YES bash "$SCRIPT" "$ZIP" "$SHA"
