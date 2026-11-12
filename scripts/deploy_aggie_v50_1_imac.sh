#!/usr/bin/env bash
# Author: Sunil Jain, 2026-11-12.
# CI-gated, non-destructive AGIPAREL v50.1 update on the EXISTING iMac stack.
set -Eeuo pipefail
umask 077
cd "$(dirname "$0")/.."
[[ "${AGIPAREL_CONFIRM_AGGIE_V501_IMAC:-}" == YES ]] || {
  echo 'STOP: set AGIPAREL_CONFIRM_AGGIE_V501_IMAC=YES to deploy the existing iMac app.' >&2; exit 2;
}
[[ "$(git branch --show-current)" == main ]] || { echo 'STOP: expected main branch' >&2; exit 1; }
git pull --ff-only origin main
REV="$(git rev-parse HEAD)"
# The code must include this hotfix, even if main advances.
git grep -q 'case aggieintent.MachineStatus:' -- services/edge-daemon/cmd/server/operational_queries.go ||
  { echo 'STOP: v50.1 machine hotfix not present' >&2; exit 1; }
# Fail closed unless GitHub CI passed at precisely the code being deployed.
python3 - "$REV" <<'PY'
# Author: Sunil Jain, 2026-11-12.
# Verify required GitHub checks for the exact checked-out revision before deployment.
import json,sys
from urllib.request import Request,urlopen
sha=sys.argv[1]
url='https://api.github.com/repos/cyberinQ/agiparel/commits/'+sha+'/check-runs'
req=Request(url,headers={'Accept':'application/vnd.github+json','User-Agent':'agiparel-v501-imac'})
try:
    with urlopen(req,timeout=15) as response: data=json.load(response)
except Exception as e:
    raise SystemExit('STOP: cannot confirm GitHub CI; no deployment performed: '+str(e))
checks={item.get('name'):item for item in data.get('check_runs',[])}
required=('verify','postgres-schema')
for name in required:
    run=checks.get(name)
    if not run or run.get('status')!='completed' or run.get('conclusion')!='success':
        raise SystemExit('STOP: GitHub '+name+' check not green at '+sha[:8]+'; no deployment performed')
print('PASS: GitHub verify and postgres-schema CI green at '+sha[:8])
PY
echo '=== Existing iMac safety checks, database snapshot, and application-only restart ==='
bash scripts/deploy_cut_sop2_v48_imac.sh --check
AGIPAREL_CONFIRM_IMAC_DEPLOY=YES bash scripts/deploy_cut_sop2_v48_imac.sh --deploy
echo '=== Live Aggie v50.1 machine/roll isolation smoke ==='
bash scripts/smoke_aggie_v50_1.sh
echo "PASS: iMac v50.1 app deployed and Aggie smoke verified at $(git rev-parse --short HEAD)"
