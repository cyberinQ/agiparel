#!/usr/bin/env bash
# Author: Sunil Jain, CyberinQ LLC, 2026-10-29.
# Idempotent finalize step for a delayed DNS/proxy change.
set -Eeuo pipefail
cd "$(git rev-parse --show-toplevel)"
[ "$(git branch --show-current)" = main ] || { echo 'ERROR: main required' >&2; exit 1; }
node scripts/test_factory_v43_public_admin.cjs
bash scripts/verify_public_release_v43.sh
git diff --check
git add -- .github README.md compose docs scripts services
# Exclude secrets even if a previous developer accidentally forced .env tracking.
git diff --cached --name-only | grep -Eq '(^|/)\.env($|\.)' && { echo 'ERROR: secret file staged' >&2; exit 1; } || :
git diff --cached --check
if ! git diff --cached --quiet; then
  git -c core.pager=cat commit -m 'feat(factory): public rationale, protected engineer ops and verified public demo v43'
fi
git push origin main
[ "$(git rev-parse HEAD)" = "$(git rev-parse origin/main)" ] || { echo 'ERROR: push HEAD mismatch' >&2; exit 1; }
git status --short
git log -1 --oneline
printf '\nPASS: public v43 and GitHub main verified.\n'
