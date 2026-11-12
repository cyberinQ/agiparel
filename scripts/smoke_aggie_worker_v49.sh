#!/usr/bin/env bash
# Author: Sunil Jain, 2026-11-12.
# Check the running, authenticated worker guide, manual chat and direct PostgreSQL answer.
set -Eeuo pipefail
cd "$(dirname "$0")/.."
python3 - "$@" <<'PY'
# Author: Sunil Jain, 2026-11-12.
# Use local Basic credentials without printing passwords or contacting the public site.
import os, sys, json, base64, urllib.request, urllib.error
from pathlib import Path

env = {}
p = Path('.env')
if p.is_file():
    for line in p.read_text().splitlines():
        if line.lstrip().startswith('#') or '=' not in line: continue
        key, value = line.split('=', 1)
        key = key.removeprefix('export ').strip()
        if key not in ('AGIPAREL_OPERATOR_USER', 'AGIPAREL_OPERATOR_PASSWORD', 'EDGE_DAEMON_PORT'): continue
        val = value.strip()
        if len(val) >= 2 and val[0] == val[-1] and val[0] in ('"', "'"):
            val = val[1:-1]
        env[key] = val
user = os.getenv('AGIPAREL_OPERATOR_USER', env.get('AGIPAREL_OPERATOR_USER', ''))
password = os.getenv('AGIPAREL_OPERATOR_PASSWORD', env.get('AGIPAREL_OPERATOR_PASSWORD', ''))
port = os.getenv('EDGE_DAEMON_PORT', env.get('EDGE_DAEMON_PORT', '18530'))
base = os.getenv('AGIPAREL_CHECK_URL', f'http://127.0.0.1:{port}').rstrip('/')
if not user or not password:
    sys.exit('STOP: set AGIPAREL_OPERATOR_USER and AGIPAREL_OPERATOR_PASSWORD. No secrets printed.')
if not base.startswith(('http://127.0.0.1:', 'http://localhost:')):
    sys.exit('STOP: diagnostics are localhost-only.')
headers = {'Authorization':'Basic '+base64.b64encode(f'{user}:{password}'.encode()).decode(), 'Accept':'application/json'}
def request(path, data=None):
    opts = dict(headers=headers)
    if data is not None:
        opts['headers'] = {**headers,'Content-Type':'application/json'}
        opts['data'] = json.dumps(data).encode()
    try:
        with urllib.request.urlopen(urllib.request.Request(base+path,**opts),timeout=20) as r:
            return r.status,json.loads(r.read().decode())
    except urllib.error.HTTPError as e:
        return e.code, {'error':e.read(500).decode(errors='replace')}

status, guide = request('/api/v1/agie/manual?chapter=5')
if status != 200 or guide.get('chapter',{}).get('number') != 5:
    sys.exit(f'FAIL: chapter guide not available (HTTP {status}).')
print('PASS: reviewed chapter 05 instructions available to authenticated operator')
status, allguide = request('/api/v1/agie/manual')
if status != 200 or len(allguide.get('chapters',[])) != 10:
    sys.exit(f'FAIL: full ten-chapter guide missing (HTTP {status}).')
print('PASS: all 10 chapters accessible to Aggie and worker browser')
status, caps = request('/api/v1/agie/capabilities')
if status != 200 or caps.get('manual_chapters') != 10:
    sys.exit(f'FAIL: Aggie capabilities endpoint not available (HTTP {status}).')
if not caps.get('catalog_probe') or not caps.get('read_only_select_probe'):
    print('NOT VERIFIED: catalog and bounded read-only query tools did not both pass runtime probes.')
    sys.exit(1)
print('PASS: catalog inspector and read-only query tool work against this local app connection')
status, answer = request('/api/v1/agie/chat', {'page':'/cutting','messages':[{'role':'user','content':'How do I plan a cut?'}]})
if status != 200 or answer.get('source') != 'reviewed_worker_manual' or 'Save lay draft' not in answer.get('reply',''):
    sys.exit(f'FAIL: grounded manual chat not available (HTTP {status}).')
print('PASS: Aggie answers procedure questions from the manual without guessing')
status, record = request('/api/v1/agie/chat', {'page':'/factory-setup','messages':[{'role':'user','content':'What is our factory name?'}]})
if status != 200 or record.get('source') != 'live_postgresql':
    print(f'NOT VERIFIED: direct tenant-scoped database answer (HTTP {status}).')
    sys.exit(1)
print('PASS: one direct read-only PostgreSQL question answered; this does NOT certify full-table access')
print('NOT TESTED: live model tool-calling, every factory table, worker-level permissions or all possible questions')
PY
