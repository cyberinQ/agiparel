#!/usr/bin/env bash
# Author: Sunil Jain, CyberinQ LLC, 2026-10-29.
# v43.2.2: public nav/About only. No migration, DB reset, admin or auth policy changes.
set -Eeuo pipefail
export GIT_PAGER=cat PAGER=cat
: "${1:?Usage: installer.sh PATCH.zip EXPECTED_SHA256}"
: "${2:?Expected ZIP sha256 required}"
ZIP="$1"; EXPECTED="$2"
ROOT="$(git rev-parse --show-toplevel)"; cd "$ROOT"
[[ "$(git branch --show-current)" == "main" ]] || { echo 'ERROR: run on main branch' >&2; exit 1; }
for t in git go python3 node docker curl unzip shasum; do command -v "$t" >/dev/null || { echo "ERROR: missing tool: $t" >&2; exit 1; }; done
[[ -r "$ZIP" ]] || { echo 'ERROR: patch file missing' >&2; exit 1; }
ACTUAL="$(shasum -a 256 "$ZIP" | awk '{print $1}')"
[[ "$ACTUAL" == "$EXPECTED" ]] || { echo "ERROR: patch checksum mismatch: $ACTUAL" >&2; exit 1; }
unzip -tq "$ZIP" >/dev/null
if ! git diff --cached --quiet; then
  echo 'ERROR: unrelated staged changes exist. Please review them before installing a new release.' >&2
  git --no-pager diff --cached --shortstat
  exit 1
fi
printf '\n=== 1 VERIFY AND APPLY PUBLIC-ONLY PATCH ===\n'
python3 - "$ZIP" <<'PYAPPLY'
from pathlib import Path
import hashlib,sys,zipfile
prior = {'services/edge-daemon/internal/web/static/portal.html': '08c0328ccf6dc9031a9ebb5ad686647abfd43a25aa1283cf53fcfea11635a710', 'services/edge-daemon/internal/web/static/portal.css': '669562a5f9ea7ba66994a4ddeb31be06044530f2657251450a36b4649a458000', 'services/edge-daemon/internal/web/assets.go': '1bc8fe88045c5e5171f7cf3d27266de9647a9a8997b5915e73173104e8cf6b25', 'services/edge-daemon/cmd/server/portal.go': 'ed47074f2eea244e2c148d08a3afbbc29a920e6c081d81986d71c37edfbd93a3', 'services/edge-daemon/cmd/server/main.go': 'c676f19351702ed21bfb8c27868da44ff3586a9de4bda4826373a262e26296c0', 'scripts/test_factory_v43_2_home_ops.cjs': '75e410cbaefd74efbaf10d0f73a6a91e10f086caa60dcbbcbb55ee26e7503e5a'}
current = {'services/edge-daemon/internal/web/static/portal.html': 'c77feffaa696b70a1c1518719de5bb5aff469de33ee15f5e8575a5c714e812ef', 'services/edge-daemon/internal/web/static/portal.css': '9b1a383b1a3c20d7b8079765da900e44c7da8c00822e6f7ef54558c71f3bc9fc', 'services/edge-daemon/internal/web/assets.go': '7c5e66be570a80495d38d3d05aeef32478976835bddeac3da9fe9724da314799', 'services/edge-daemon/cmd/server/portal.go': '309281c2d28e6f5df721009b3ae5191e35cca3b386e76fe62233379679764950', 'services/edge-daemon/cmd/server/main.go': 'dc851f85a4f2a1e051193d1711d7ba7d16b8e837ff823ceb099c377a67c1c03b', 'scripts/test_factory_v43_2_home_ops.cjs': 'd86bebc039afbb225f9aa79c86170a7d3773325c1ccc94b4e521a711a716db58', 'services/edge-daemon/internal/web/static/about.html': 'cd6da2d6a9c8c089a959de763b91942e3da3252106436fd257b20fb3a2e7e296', 'services/edge-daemon/cmd/server/public_account_v43_2_2.go': 'f8950671fb107bb416dda4e357ce81d3376d3a9333d5fb7423b489d3bcf8b84f', 'services/edge-daemon/cmd/server/public_account_v43_2_2_test.go': 'bfde4b7f3126b8e8a98d064f24d71b82968d8014ee04c22c428b54e01e8faece', 'scripts/test_factory_v43_2_2_public_nav.cjs': '719ba2022334ebe27ccdedf7c2b3e8847efba7ee20d35babe96427f2eab4cb58', 'docs/V43_2_2_PUBLIC_NAV_AND_ABOUT.md': 'f93000173d53f5e8fdfb17586a323c22b731db39d3ab6e9dded458897691133d'}
paths=set(current)|{'scripts/install_release_v43_2_2.sh'}
with zipfile.ZipFile(sys.argv[1]) as z:
    members=set(z.namelist())
    if members!=paths:
        raise SystemExit('ERROR: unexpected files in ZIP: '+repr(sorted(members^paths)))
    for name in sorted(paths):
        dest=Path(name)
        if name not in current:
            # Self-contained installer integrity is pinned by the ZIP SHA-256.
            if dest.is_file() and dest.read_bytes()!=z.read(name):
                raise SystemExit('ERROR: installed release script differs from verified ZIP: '+name)
            continue
        observed=hashlib.sha256(dest.read_bytes()).hexdigest() if dest.is_file() else None
        allowed={prior.get(name),current[name]}
        if observed not in allowed:
            raise SystemExit('ERROR: local file differs from expected v43.2.1 baseline: '+name)
        incoming=z.read(name)
        if hashlib.sha256(incoming).hexdigest()!=current[name]:
            raise SystemExit('ERROR: ZIP file content differs from expected: '+name)
    for name in sorted(paths):
        dest=Path(name)
        dest.parent.mkdir(parents=True,exist_ok=True)
        dest.write_bytes(z.read(name))
print('PASS: exact v43.2.1 base validated; public-only navigation patch applied')
PYAPPLY
bash -n scripts/install_release_v43_2_2.sh
printf '\n=== 2 JAVASCRIPT + GO + RACE TESTS ===\n'
count=0
for file in scripts/test_*.cjs; do node "$file" >/dev/null; count=$((count+1)); done
node --check services/edge-daemon/internal/web/static/portal.js
printf 'PASS: %d JavaScript scripts\n' "$count"
GOFLAGS=-mod=readonly go test ./...
GOFLAGS=-mod=readonly go test -race ./...
GOFLAGS=-mod=readonly go vet ./...
if [[ -n "$(gofmt -l services/edge-daemon/cmd/server/portal.go services/edge-daemon/cmd/server/main.go services/edge-daemon/cmd/server/public_account_v43_2_2.go services/edge-daemon/cmd/server/public_account_v43_2_2_test.go services/edge-daemon/internal/web/assets.go)" ]]; then
    echo 'ERROR: Go formatting check failed' >&2; exit 1
fi
git diff --check -- services/edge-daemon/internal/web/static/portal.html services/edge-daemon/internal/web/static/portal.css services/edge-daemon/internal/web/assets.go services/edge-daemon/cmd/server/portal.go services/edge-daemon/cmd/server/main.go scripts/test_factory_v43_2_home_ops.cjs
printf '\n=== 3 REBUILD AND DEPLOY EDGE CONTAINER (NO DB CHANGE) ===\n'
COMPOSE=(docker compose --env-file .env -f compose/docker-compose.dev.yml)
"${COMPOSE[@]}" up -d --build plant-edge-os
curl --retry 12 --retry-delay 2 --retry-connrefused -fsS http://127.0.0.1:18530/healthz
echo
printf '\n=== 4 VERIFY LOCALLY SERVED HOME / ABOUT / LOGIN / REGISTER ===\n'
python3 - <<'PYLOCAL'
import urllib.request,urllib.error
root='http://127.0.0.1:18530'
def get(path):
    with urllib.request.urlopen(root+path,timeout=15) as resp:
        return resp.status,resp.read().decode('utf-8','replace'),resp.headers
for path,check in [('/', 'class="site-links"'),('/about','Where we are today.'),('/login','AGIPAREL ACCOUNT'),('/register','AGIPAREL ACCOUNT')]:
    code,html,headers=get(path)
    assert code==200 and check in html and 'WWW-Authenticate' not in headers,(path,code)
    assert headers.get('X-Frame-Options')=='DENY',path
    print('PASS:',path)
home=get('/')[1]
assert 'Every garment<br><em>tells a story.</em>' in home
assert 'id="agiparel-wave-canvas"' in home
assert all('href="'+p+'"' in home for p in ('/about','/login','/register'))
assert 'class="fabric-wave-art"' in home[home.index('<section class="factory-why"'):home.index('</section>',home.index('<section class="factory-why"'))]
css=get('/demo-assets/portal.css')[1]
assert '.site-links' in css and '.fabric-wave-art' in css
print('PASS: existing home copy, fabric waves, account navigation and CSS')
PYLOCAL
printf '\n=== 5 REPORT PUBLIC HTTPS VISIBILITY (NEVER MISCLASSIFY AN UPSTREAM 403) ===\n'
python3 - <<'PYPUBLIC'
import urllib.request,urllib.error
req=urllib.request.Request('https://agiparel.empowerai.us/',headers={'User-Agent':'Mozilla/5.0','Cache-Control':'no-cache'})
try:
    with urllib.request.urlopen(req,timeout=15) as response:
        content=response.read().decode('utf-8','replace')
        if 'href="/about"' in content and 'href="/register"' in content:
            print('PASS: public HTTPS home serves the updated navigation')
        else:
            print('WARNING: public HTTPS route does not yet serve updated navigation. Check tunnel/cache/origin.')
except urllib.error.HTTPError as exc:
    print(f'WARNING: public gateway HTTP {exc.code}; localhost verified. Check ingress rules separately.')
except Exception as exc:
    print(f'WARNING: public domain unverified ({type(exc).__name__}); localhost verified.')
PYPUBLIC
printf '\n=== 6 COMMIT PATCH FILES ONLY AND PUSH ===\n'
git add -- services/edge-daemon/internal/web/static/portal.html services/edge-daemon/internal/web/static/portal.css services/edge-daemon/internal/web/assets.go services/edge-daemon/cmd/server/portal.go services/edge-daemon/cmd/server/main.go scripts/test_factory_v43_2_home_ops.cjs services/edge-daemon/internal/web/static/about.html services/edge-daemon/cmd/server/public_account_v43_2_2.go services/edge-daemon/cmd/server/public_account_v43_2_2_test.go scripts/test_factory_v43_2_2_public_nav.cjs docs/V43_2_2_PUBLIC_NAV_AND_ABOUT.md scripts/install_release_v43_2_2.sh
git diff --cached --check
git --no-pager diff --cached --shortstat
if ! git diff --cached --quiet; then
  git -c core.pager=cat commit -m 'feat(factory): add public account navigation and About page v43.2.2'
fi
git push origin main
[[ "$(git rev-parse HEAD)" == "$(git rev-parse origin/main)" ]] || { echo 'ERROR: remote tracking ref does not match local commit' >&2; exit 1; }
git status --short
git log -1 --oneline
echo 'PASS: local v43.2.2 deployment and GitHub push. Public HTTPS status reported separately.'
