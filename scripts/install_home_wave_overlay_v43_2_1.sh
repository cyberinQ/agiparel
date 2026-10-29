#!/usr/bin/env bash
# Author: Sunil Jain, CyberinQ LLC, 2026-10-29.
# Layout-only update: move existing fabric waves behind "Why Agiparel".
# No migration, identity change, content change or database reset.
set -Eeuo pipefail
export GIT_PAGER=cat PAGER=cat
: "${1:?Usage: bash installer.sh NEW_PATCH.zip EXPECTED_SHA256}"
: "${2:?Expected SHA-256 required}"
NEW_ZIP="$1"
EXPECTED="$2"
OLD_ZIP="$HOME/Downloads/Agiparel_v43_2_REVISED_FABRIC_WAVES_ENGINEER_OPS_PATCH.zip"
OLD_SHA="6eb1b7b30e85fd2c7d65cf57701e8976fa4ff8a5a089eb1bfbca3b4a9c788436"
ROOT="$(git rev-parse --show-toplevel)"
cd "$ROOT"
[[ "$(git branch --show-current)" == main ]] || { echo 'ERROR: main branch required.' >&2; exit 1; }
for tool in docker python3 node git unzip shasum curl go; do command -v "$tool" >/dev/null || { echo "ERROR: missing $tool" >&2; exit 1; }; done
[[ -r "$NEW_ZIP" ]] && [[ -r "$OLD_ZIP" ]] || { echo "ERROR: both the new overlay ZIP and the previously installed v43.2 patch ZIP are required in Downloads" >&2; exit 1; }
[[ "$(shasum -a 256 "$NEW_ZIP" | awk '{print $1}')" == "$EXPECTED" ]] || { echo 'ERROR: new patch SHA-256 mismatch' >&2; exit 1; }
[[ "$(shasum -a 256 "$OLD_ZIP" | awk '{print $1}')" == "$OLD_SHA" ]] || { echo 'ERROR: previous v43.2 ZIP SHA-256 mismatch' >&2; exit 1; }
unzip -tq "$NEW_ZIP" >/dev/null
unzip -tq "$OLD_ZIP" >/dev/null

printf '\n=== 1 VERIFY EXACT INSTALLED v43.2 SOURCE ===\n'
python3 - "$OLD_ZIP" "$NEW_ZIP" <<'PYVERIFY'
from pathlib import Path
import subprocess,sys,zipfile
old=zipfile.ZipFile(sys.argv[1]); new=zipfile.ZipFile(sys.argv[2]); allowed=set(old.namelist())|set(new.namelist())
if any(not p.startswith(('services/edge-daemon/internal/web/static/portal.', 'scripts/')) for p in new.namelist()):
    raise SystemExit('ERROR: new patch contains files beyond the home page and test/installer')
for row in subprocess.check_output(['git','status','--porcelain','--untracked-files=all'],text=True).splitlines():
    p=row[3:]
    if p not in allowed:
        raise SystemExit('ERROR: unrelated working-tree change; refusing to make a release: '+p)
for name in old.namelist():
    p=Path(name)
    if not p.is_file(): raise SystemExit('ERROR: previous v43.2 source missing: '+name)
    content=p.read_bytes(); expected=old.read(name)
    if name=='services/edge-daemon/internal/web/static/portal.js':
        valid=(expected,expected.rstrip(b'\r\n')+b'\n')
    elif name=='scripts/install_release_v43_2_revised.sh':
        valid=(expected,)
        if b'v43.2 EOF whitespace hotfix' in content and content.startswith(expected[:70]): continue
    elif name in new.namelist() and name!='scripts/install_home_wave_overlay_v43_2_1.sh':
        valid=(expected,new.read(name))
    else: valid=(expected,)
    if content not in valid:
        raise SystemExit('ERROR: existing v43.2 file differs from verified source: '+name)
for name in new.namelist():
    if name not in old.namelist() and Path(name).exists() and Path(name).read_bytes()!=new.read(name):
        raise SystemExit('ERROR: new patch file conflicts with an existing local file: '+name)
print('PASS: existing files are verified against the earlier patch; no unrelated changes')
PYVERIFY

printf '\n=== 2 APPLY HOME-PAGE-ONLY LAYOUT CHANGE ===\n'
python3 - "$NEW_ZIP" <<'PYAPPLY'
from pathlib import Path
import sys,zipfile
with zipfile.ZipFile(sys.argv[1]) as z:
    for p in z.namelist():
        target=Path(p)
        target.parent.mkdir(parents=True,exist_ok=True)
        target.write_bytes(z.read(p))
PYAPPLY
chmod 755 scripts/install_home_wave_overlay_v43_2_1.sh
bash -n scripts/install_home_wave_overlay_v43_2_1.sh
node scripts/test_factory_v43_2_home_ops.cjs
for f in scripts/test_*.cjs; do node "$f" >/dev/null; done
GOFLAGS=-mod=readonly go test ./... >/dev/null
GOFLAGS=-mod=readonly go test -race ./... >/dev/null
git diff --check
git diff --cached --check
printf 'PASS: all existing JS tests, Go tests and race tests\n'

printf '\n=== 3 REBUILD AND RESTART EDGE ONLY (NO DB WRITES) ===\n'
COMPOSE=(docker compose --env-file .env -f compose/docker-compose.dev.yml)
"${COMPOSE[@]}" up -d --build plant-edge-os
curl --retry 12 --retry-delay 2 --retry-connrefused -fsS http://127.0.0.1:18530/healthz
echo
printf '\n=== 4 VERIFY ACTUAL LOCALLY SERVED HOME ===\n'
python3 - <<'PYLOCAL'
import hashlib,re,urllib.request
base='http://127.0.0.1:18530'
def get(path):
    with urllib.request.urlopen(base+path,timeout=15) as r: return r.read().decode('utf-8')
h=get('/')
section=re.search(r'<section class="factory-why"[^>]*>(.*?)</section>',h,re.S)
assert section and section[0].count('id="agiparel-wave-canvas"')==1
assert section[0].index('class="fabric-wave-art"')<section[0].index('id="why-agiparel"')
assert 'fabric-wave-art' not in h[h.index('</section>',h.index('<section class="factory-why"')):h.index('</main>')]
s=' '.join(re.sub(r'<[^>]+>',' ',re.sub(r'<!--.*?-->','',h,flags=re.S)).split())
assert hashlib.sha256(s.encode()).hexdigest()=='cc7ee683b8e1b54c9d908d6bc041cf11367d50505400eea9ec2e25ccf3cbe123'
css=get('/demo-assets/portal.css')
assert '.fabric-wave-art{position:absolute;inset:0' in css
assert 'background:rgba(255,255,255,.68)' in css
js=get('/demo-assets/portal.js')
assert "canvas.closest('.fabric-wave-art')" in js
print('PASS: serving waves behind Why Agiparel, original text unchanged, separate bottom waves removed')
PYLOCAL

printf '\n=== 5 CHECK PUBLIC DOMAIN (DO NOT CONFUSE A 403 WITH DEPLOYMENT FAILURE) ===\n'
python3 - <<'PYPUBLIC'
import urllib.request,urllib.error,re
url='https://agiparel.empowerai.us/?fabric_overlay=v43_2_1'
try:
    with urllib.request.urlopen(urllib.request.Request(url,headers={'Cache-Control':'no-cache','User-Agent':'Mozilla/5.0'}),timeout=18) as resp:
        html=resp.read().decode('utf-8','replace')
        section=re.search(r'<section class="factory-why"[^>]*>(.*?)</section>',html,re.S)
        if section and 'class="fabric-wave-art"' in section[0] and section[0].index('class="fabric-wave-art"')<section[0].index('id="why-agiparel"'):
            print('PASS: public HTTPS home serves new wave placement (HTML only)')
        else:
            print('WARNING: public HTTPS home still serves old HTML; inspect CDN/proxy/cache. Local deployment PASSED.')
except urllib.error.HTTPError as err:
    print(f'WARNING: public HTTPS endpoint returned {err.code}; gateway access is unresolved. Local deployment PASSED.')
except Exception as err:
    print(f'WARNING: public HTTPS request failed ({type(err).__name__}); local deployment PASSED.')
PYPUBLIC

printf '\n=== 6 COMMIT VERIFIED RELEASE FILES AND PUSH ===\n'
# The earlier v43.2 installer stopped before its commit, so stage its verified
# files together with this placement-only refinement. Do not stage other files.
while IFS= read -r p; do [[ -n "$p" ]] && git add -- "$p"; done < <(unzip -Z1 "$OLD_ZIP")
while IFS= read -r p; do [[ -n "$p" ]] && git add -- "$p"; done < <(unzip -Z1 "$NEW_ZIP")
git diff --cached --check
git --no-pager diff --cached --shortstat
if ! git diff --cached --quiet; then
  git -c core.pager=cat commit -m 'feat(factory): place subtle fabric waves behind Why Agiparel'
fi
git push origin main
[[ "$(git rev-parse HEAD)" == "$(git rev-parse origin/main)" ]] || { echo 'ERROR: origin/main differs from local HEAD' >&2; exit 1; }
git status --short
git log -1 --oneline
echo 'PASS: local deployment and GitHub push complete. Public visibility is reported separately above.'
