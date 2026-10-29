#!/usr/bin/env bash
# Author: Sunil Jain, CyberinQ LLC, 2026-10-29.
# Activate HTTPS proxy TLS awareness only when Docker publishes loopback.
set -Eeuo pipefail
cd "$(git rev-parse --show-toplevel)"
[[ -f .env ]] || { echo 'ERROR: .env missing' >&2; exit 1; }
command -v python3 >/dev/null || exit 1
python3 - <<'PY'
from pathlib import Path
import os
p=Path('.env'); s=p.read_text(); items={}
for line in s.splitlines():
    if line and not line.lstrip().startswith('#') and '=' in line:
        key,value=line.split('=',1)
        items[key.strip()]=value.strip().strip('"').strip("'")
host=items.get('AGIPAREL_EDGE_BIND_HOST','127.0.0.1')
if host not in ('127.0.0.1','localhost','::1'):
    raise SystemExit('ERROR: Refusing trusted-proxy TLS: public origin binding '+repr(host))
if items.get('AGIPAREL_ENGINEER_OPS_ENABLED')!='true':
    raise SystemExit('ERROR: set AGIPAREL_ENGINEER_OPS_ENABLED=true and provision admin credentials first')
if not items.get('AGIPAREL_ENGINEER_OPS_USER') or len(items.get('AGIPAREL_ENGINEER_OPS_PASSWORD',''))<24:
    raise SystemExit('ERROR: Engineer Ops admin credentials not provisioned')
key='AGIPAREL_ENGINEER_OPS_TRUST_PROXY_TLS'
lines=s.splitlines()
if any(line.startswith(key+'=') for line in lines):
    lines=[(key+'=true') if line.startswith(key+'=') else line for line in lines]
else:
    lines.append(key+'=true')
new='\n'.join(lines)+'\n'
if s!=new:
    p.write_text(new)
    os.chmod(p,0o600)
print('PASS: loopback backend binding and administrator provisioning; trusted HTTPS proxy enabled')
PY
printf '\n=== RESTART APP (DATABASE PERSISTED) ===\n'
docker compose --env-file .env -f compose/docker-compose.dev.yml up -d --no-deps --force-recreate plant-edge-os
curl -fsS --retry 10 --retry-delay 2 --retry-connrefused http://127.0.0.1:18530/healthz; echo
printf '\n=== VERIFY PUBLIC HTTPS ADMIN CHALLENGE ===\n'
URL="${AGIPAREL_PUBLIC_URL:-https://agiparel.empowerai.us}"
case "$URL" in https://*) ;; *) echo 'ERROR: Admin URL must use public HTTPS' >&2;exit 1;; esac
code="$(curl --silent --show-error --location --max-time 20 --output /dev/null --write-out '%{http_code}' "$URL/engineer-ops")"
case "$code" in
  401) echo 'PASS: HTTPS Engineer Ops is available and requires independent credentials.';;
  403) echo 'ERROR: Proxy failed to forward X-Forwarded-Proto: https (or routes to a different origin). Fix HTTPS proxy forwarding.' >&2;exit 2;;
  *) echo "ERROR: public Engineer Ops returned HTTP $code instead of 401; check routing and active application." >&2;exit 2;;
esac
