#!/usr/bin/env bash
# Author: Sunil Jain, 2026-11-12.
# Verify authenticated Aggie machine and roll answers against the running iMac database.
set -Eeuo pipefail
cd "$(dirname "$0")/.."
python3 - <<'PY'
# Author: Sunil Jain, 2026-11-12.
# Read the running container's operator credentials without displaying or logging secrets.
import base64
import json
import subprocess
from pathlib import Path
from urllib.request import Request, urlopen
from urllib.error import HTTPError, URLError

try:
    raw = subprocess.check_output(
        ['docker','inspect','--format','{{json .Config.Env}}','agiparel-edge-daemon'],
        text=True, stderr=subprocess.DEVNULL)
except (OSError, subprocess.CalledProcessError) as exc:
    raise SystemExit('STOP: agiparel-edge-daemon is not running') from exc
vars = dict(item.split('=',1) for item in json.loads(raw) if '=' in item)
username = vars.get('AGIPAREL_OPERATOR_USER','')
password = vars.get('AGIPAREL_OPERATOR_PASSWORD','')
if not username or not password:
    raise SystemExit('STOP: operator credentials are not configured')
if username == 'dev' or password == '1212':
    print('SECURITY WARNING: weak development credentials detected; operator routes must not be public.')
port = '18530'
if Path('.env').exists():
    for line in Path('.env').read_text().splitlines():
        if line.startswith('EDGE_DAEMON_PORT='):
            port=line.split('=',1)[1].strip().strip('"\'')
if not port.isdecimal() or not 1 <= int(port) <= 65535:
    raise SystemExit('STOP: invalid localhost port')
url='http://127.0.0.1:'+port+'/api/v1/agie/chat'
token=base64.b64encode((username+':'+password).encode()).decode()

def ask(history):
    body=json.dumps({'messages':history}).encode()
    req=Request(url,data=body,method='POST',headers={
        'Authorization':'Basic '+token,'Content-Type':'application/json'})
    try:
        with urlopen(req,timeout=20) as res:
            if res.status != 200: raise ValueError('unexpected HTTP status')
            response=json.load(res)
    except (HTTPError, URLError, TimeoutError, ValueError) as exc:
        raise SystemExit('FAIL: Aggie localhost request failed: '+str(exc)) from exc
    if response.get('source') != 'live_postgresql':
        raise SystemExit('FAIL: no tenant-scoped database provenance for Aggie answer')
    return response.get('reply','')

history=[{'role':'user','content':'How many rolls in inventory?'}]
rolls=ask(history)
if 'Fabric stock book:' not in rolls: raise SystemExit('FAIL: missing fabric ledger answer')
print('PASS: rolls answered from stock book')
history.extend([{'role':'assistant','content':rolls},{'role':'user','content':'How many machines?'}])
machines=ask(history)
if 'Machine register:' not in machines or 'roll' in machines.lower():
    raise SystemExit('FAIL: machine answer contaminated by earlier roll answer')
print('PASS: machine count separated from previous turn')
history.extend([{'role':'assistant','content':machines},{'role':'user','content':'Which machines are open?'}])
statuses=ask(history)
if 'roll' in statuses.lower() or not (
    'not verified live availability' in statuses or
    'cannot determine which machines' in statuses
):
    raise SystemExit('FAIL: machine availability represented incorrectly')
print('PASS: machine roster status is not claimed as live availability')
combined=ask([{'role':'user','content':'How many machines and rolls do we have?'}])
if 'Machine register:' not in combined or 'Fabric stock book:' not in combined:
    raise SystemExit('FAIL: combined counts did not preserve entities')
print('PASS: machine and fabric queries independently grounded')
print('NOT VERIFIED: live machine availability or model-generated answers')
PY
