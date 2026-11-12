#!/usr/bin/env bash
# Author: Sunil Jain, 2026-11-12
# Run AGIPAREL v46 on this MacBook only: isolated PostgreSQL 16 and loopback-bound Go edge server.
# No Docker, cloud access, iMac access, production data copy, or destructive reset.
set -Eeuo pipefail
umask 077
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
STATE="${AGIPAREL_MACBOOK_STATE:-$HOME/.agiparel-macbook-dev}"
PGDIR="$STATE/postgres16"
PGLOG="$STATE/postgres.log"
PGSECRETS="$STATE/postgres-secrets.env"
APPSECRETS="$STATE/app-secrets.env"
APPLOG="$STATE/edge.log"
APPBIN="$STATE/bin/plant-edge-os"
PIDFILE="$STATE/edge.pid"
WEB_PORT=18531
DB_PORT=15435
DB_NAME=agiparel_macbook_dev
DB_SUPER=agiparel_admin
APPUSER=agiparel_app

usage() { cat <<'USAGE'
AGIPAREL MacBook native localhost development (Docker-free)
  bash scripts/agiparel_macbook_localdev.sh up           # Install PG16 if needed, bootstrap isolated DB, test, build and run
  bash scripts/agiparel_macbook_localdev.sh status       # Check native PostgreSQL and local web server
  bash scripts/agiparel_macbook_localdev.sh test         # Existing SOP-2 test/build checks and database assertions
  bash scripts/agiparel_macbook_localdev.sh down         # Stop ONLY this kit's application and isolated PostgreSQL
  bash scripts/agiparel_macbook_localdev.sh logs         # Show application log
  bash scripts/agiparel_macbook_localdev.sh credentials  # Show browser-local Basic Auth credentials
  bash scripts/agiparel_macbook_localdev.sh doctor       # Print prerequisites and port status

UI: http://127.0.0.1:18531/cutting/sop2
PostgreSQL: 127.0.0.1:15435, database agiparel_macbook_dev
State: ~/.agiparel-macbook-dev (outside Git repository)
USAGE
}
die() { echo "ERROR: $*" >&2; exit 1; }
ensure_repo() {
  [[ -f "$REPO/go.mod" && -f "$REPO/services/core-api/migrations/0001_v42_baseline.sql" && -f "$REPO/services/core-api/migrations/0005_cut_sop2_control_plane_v46.sql" ]] || die "Expected v46 AGIPAREL repository at $REPO"
  grep -Fq 'os.Getenv("AGIPAREL_BIND_HOST") + ":" + port' "$REPO/services/edge-daemon/cmd/server/main.go" || die "Loopback bind patch not installed; use supplied installer first."
}
pg16_bin() {
  local b=""
  if command -v brew >/dev/null 2>&1; then
    local prefix=""
    prefix="$(brew --prefix postgresql@16 2>/dev/null || true)"
    [[ -z "$prefix" ]] || b="$prefix/bin"
  fi
  if [[ -z "$b" || ! -x "$b/initdb" ]]; then
    for p in /opt/homebrew/opt/postgresql@16/bin /usr/local/opt/postgresql@16/bin; do
      if [[ -x "$p/initdb" ]]; then b="$p"; break; fi
    done
  fi
  if [[ -n "$b" && -x "$b/initdb" ]]; then
    [[ "$("$b/postgres" --version)" == *' 16.'* ]] || die 'PostgreSQL 16 required'
    printf '%s\n' "$b"
    return 0
  fi
  return 1
}
prepare_pg16() {
  local bin=""
  bin="$(pg16_bin || true)"
  if [[ -z "$bin" ]]; then
    command -v brew >/dev/null 2>&1 || die 'Homebrew missing. Install Homebrew then run: brew install postgresql@16'
    echo 'Installing Homebrew PostgreSQL 16 locally (no Docker)...' >&2
    brew install postgresql@16 >&2
    bin="$(pg16_bin || true)"
  fi
  [[ -n "$bin" ]] || die 'PostgreSQL 16 not found after installation'
  printf '%s\n' "$bin"
}
create_secrets() {
  mkdir -p "$STATE/bin" "$STATE/wal"
  chmod 700 "$STATE" "$STATE/bin" "$STATE/wal"
  command -v openssl >/dev/null || die 'openssl is required'
  if [[ ! -e "$PGSECRETS" ]]; then
    ( echo "PG_ADMIN_PASSWORD=$(openssl rand -hex 24)"
      echo "PG_APP_PASSWORD=$(openssl rand -hex 24)"
      echo "PG_DOC_PASSWORD=$(openssl rand -hex 24)" ) > "$PGSECRETS"
    chmod 600 "$PGSECRETS"
  fi
  # shellcheck source=/dev/null
  . "$PGSECRETS"
  [[ "${PG_ADMIN_PASSWORD:-}" =~ ^[a-f0-9]{48}$ && "${PG_APP_PASSWORD:-}" =~ ^[a-f0-9]{48}$ && "${PG_DOC_PASSWORD:-}" =~ ^[a-f0-9]{48}$ ]] || die 'Local PostgreSQL secrets invalid'
  if [[ ! -e "$APPSECRETS" ]]; then
    local operator_password ledger_secret
    operator_password="$(openssl rand -hex 16)"
    ledger_secret="$(openssl rand -hex 32)"
    {
      echo 'ENV=development'
      echo 'AGIPAREL_ENVIRONMENT=development'
      echo 'AGIPAREL_BIND_HOST=127.0.0.1'
      echo "PORT=$WEB_PORT"
      echo 'AGIPAREL_RELEASE=cut-sop2-v46-macbook'
      echo "LEDGER_SECRET=$ledger_secret"
      printf 'WAL_PATH=%q\n' "$STATE/wal/events.wal"
      echo 'AGIPAREL_OPERATOR_USER=macdev'
      echo "AGIPAREL_OPERATOR_PASSWORD=$operator_password"
      echo 'AGIPAREL_OPERATOR_TENANT=dev'
      echo 'AGIPAREL_CUT_SOP2_WRITES_ENABLED=false'
      echo 'AGIPAREL_CAPTURE_FIRST=true'
      echo "DATABASE_URL=postgres://$APPUSER:$PG_APP_PASSWORD@127.0.0.1:$DB_PORT/$DB_NAME?sslmode=disable"
    } > "$APPSECRETS"
    chmod 600 "$APPSECRETS"
  fi
  [[ ! -L "$PGSECRETS" && ! -L "$APPSECRETS" ]] || die 'Refusing to read symlinked credentials file'
}
pg_admin() { PGPASSWORD="$PG_ADMIN_PASSWORD" "$PG_BIN/psql" -X -v ON_ERROR_STOP=1 -h 127.0.0.1 -p "$DB_PORT" -U "$DB_SUPER" -d "${1:-$DB_NAME}" "${@:2}"; }
pg_app() { PGPASSWORD="$PG_APP_PASSWORD" "$PG_BIN/psql" -X -v ON_ERROR_STOP=1 -h 127.0.0.1 -p "$DB_PORT" -U "$APPUSER" -d "$DB_NAME" "$@"; }
start_pg() {
  [[ -x "$PG_BIN/pg_ctl" ]] || die 'pg_ctl missing'
  if [[ ! -f "$PGDIR/PG_VERSION" ]]; then
    [[ ! -e "$PGDIR" ]] || die 'PG directory exists but is not initialized; refusing to overwrite'
    local tmp_pw="$STATE/.initdb.pass"
    printf '%s\n' "$PG_ADMIN_PASSWORD" > "$tmp_pw"
    chmod 600 "$tmp_pw"
    "$PG_BIN/initdb" -D "$PGDIR" --username="$DB_SUPER" --auth=scram-sha-256 --pwfile="$tmp_pw" --encoding=UTF8 --no-instructions >/dev/null
    rm -f "$tmp_pw"
  fi
  [[ "$(cat "$PGDIR/PG_VERSION")" == '16' ]] || die 'Refusing to use non-PG16 cluster'
  if ! "$PG_BIN/pg_ctl" -D "$PGDIR" status >/dev/null 2>&1; then
    # pg_ctl start fails without touching any other PostgreSQL instance if the port is occupied.
    "$PG_BIN/pg_ctl" -D "$PGDIR" -l "$PGLOG" -o "-h 127.0.0.1 -p $DB_PORT" -w start >/dev/null || die "Private PG16 failed to start; see $PGLOG"
  fi
  pg_admin postgres -Atqc 'SELECT 1' | grep -qx 1 || die 'Private PG16 authentication failed'
}
bootstrap_db() {
  local exists=""
  exists="$(pg_admin postgres -Atqc "SELECT 1 FROM pg_database WHERE datname='$DB_NAME'")"
  if [[ "$exists" != 1 ]]; then
    PGPASSWORD="$PG_ADMIN_PASSWORD" "$PG_BIN/createdb" -h 127.0.0.1 -p "$DB_PORT" -U "$DB_SUPER" -O "$DB_SUPER" -T template0 "$DB_NAME"
  fi
  if [[ ! -f "$STATE/schema-ready-v46" ]]; then
    local existing=""
    existing="$(pg_admin "$DB_NAME" -Atqc "SELECT to_regclass('public.store_rolls') IS NOT NULL")"
    [[ "$existing" == f ]] || die 'Partially initialized database: do not auto-reset. Inspect schema and logs.'
    echo 'Initializing disposable MacBook-only v42 + v43 + SOP-2 database...'
    pg_admin "$DB_NAME" -v "app_password=$PG_APP_PASSWORD" -v "doc_password=$PG_DOC_PASSWORD" -f "$REPO/scripts/bootstrap_roles_v42.sql" > "$STATE/bootstrap.log"
    pg_admin "$DB_NAME" -f "$REPO/services/core-api/migrations/0001_v42_baseline.sql" > "$STATE/schema.log"
    pg_admin "$DB_NAME" -f "$REPO/services/core-api/migrations/0002_engineer_ops_aggregates_v43_2.sql" >> "$STATE/schema.log"
    pg_admin "$DB_NAME" -f "$REPO/services/core-api/migrations/0005_cut_sop2_control_plane_v46.sql" >> "$STATE/schema.log"
    pg_admin "$DB_NAME" -f "$REPO/scripts/test_cut_sop2_v46_schema_pg.sql" >> "$STATE/schema.log"
    touch "$STATE/schema-ready-v46"
  fi
  pg_app -Atqc 'SELECT current_user' | grep -qx "$APPUSER" || die 'Application database login failed'
  pg_admin "$DB_NAME" -f "$REPO/scripts/test_cut_sop2_v46_schema_pg.sql" > "$STATE/schema-check.log"
}
app_running() {
  if [[ -f "$PIDFILE" ]]; then
    local pid; pid="$(cat "$PIDFILE")"
    if [[ "$pid" =~ ^[0-9]+$ ]] && kill -0 "$pid" 2>/dev/null; then return 0; fi
  fi
  return 1
}
smoke() {
  # shellcheck source=/dev/null
  . "$APPSECRETS"
  local url="http://127.0.0.1:$WEB_PORT"
  local http_code=000
  for _ in {1..30}; do
    http_code="$(curl -sS --connect-timeout 1 -o /dev/null -w '%{http_code}' -u "$AGIPAREL_OPERATOR_USER:$AGIPAREL_OPERATOR_PASSWORD" "$url/cutting/sop2" 2>/dev/null || true)"
    [[ "$http_code" == 200 ]] && break
    sleep 1
  done
  [[ "$http_code" == 200 ]] || die "SOP-2 endpoint not HTTP 200 (got $http_code); see $APPLOG"
  curl -fsS -u "$AGIPAREL_OPERATOR_USER:$AGIPAREL_OPERATOR_PASSWORD" "$url/healthz" >/dev/null
  echo "PASS: localhost SOP-2 web interface on $url/cutting/sop2"
  echo 'PASS: native PostgreSQL and SOP-2 schema'
  echo "Local browser Basic Auth: $AGIPAREL_OPERATOR_USER (password: run 'bash scripts/agiparel_macbook_localdev.sh credentials')"
}
cmd_up() {
  ensure_repo
  for cmd in go node gofmt curl; do command -v "$cmd" >/dev/null || die "Missing $cmd"; done
  create_secrets
  PG_BIN="$(prepare_pg16)"
  start_pg
  bootstrap_db
  (cd "$REPO" && bash scripts/run_cut_sop2_v46_checks.sh --test)
  if app_running; then
    echo "AGIPAREL local server already running: PID $(cat "$PIDFILE")"
    smoke
    return
  fi
  if command -v lsof >/dev/null 2>&1 && lsof -nP -iTCP:"$WEB_PORT" -sTCP:LISTEN 2>/dev/null | grep -q .; then
    die "Port $WEB_PORT is occupied; refusing to kill an unrelated process"
  fi
  (cd "$REPO" && GOFLAGS=-mod=readonly go build -o "$APPBIN" ./services/edge-daemon/cmd/server)
  # Server inherits ONLY the application .env; not the privileged PostgreSQL superuser secrets.
  (
    set -a
    # shellcheck source=/dev/null
    . "$APPSECRETS"
    set +a
    cd "$REPO"
    nohup "$APPBIN" > "$APPLOG" 2>&1 < /dev/null &
    echo "$!" > "$PIDFILE"
  )
  smoke
}
cmd_down() {
  if app_running; then
    kill "$(cat "$PIDFILE")"
    for _ in {1..20}; do app_running || break; sleep .25; done
    app_running && die "Failed to stop local server PID $(cat "$PIDFILE")"
    echo 'Stopped MacBook-only AGIPAREL edge process.'
  fi
  rm -f "$PIDFILE"
  PG_BIN="$(pg16_bin || true)"
  if [[ -n "$PG_BIN" && -f "$PGDIR/PG_VERSION" ]] && "$PG_BIN/pg_ctl" -D "$PGDIR" status >/dev/null 2>&1; then
    "$PG_BIN/pg_ctl" -D "$PGDIR" -m fast -w stop >/dev/null
    echo 'Stopped isolated MacBook PostgreSQL 16 cluster.'
  fi
  echo 'Data and secrets preserved under ~/.agiparel-macbook-dev.'
}
cmd_test() {
  ensure_repo
  (cd "$REPO" && bash scripts/run_cut_sop2_v46_checks.sh --test)
  if [[ -f "$PGSECRETS" && -f "$PGDIR/PG_VERSION" ]]; then
    # shellcheck source=/dev/null
    . "$PGSECRETS"
    PG_BIN="$(pg16_bin || true)"
    [[ -n "$PG_BIN" ]] || die 'PostgreSQL 16 missing'
    pg_admin "$DB_NAME" -f "$REPO/scripts/test_cut_sop2_v46_schema_pg.sql"
  else echo 'Database absent: unit/build checks only (run up for full integration).'; fi
}
cmd_status() {
  if app_running; then echo "Web: RUNNING PID $(cat "$PIDFILE") http://127.0.0.1:$WEB_PORT/cutting/sop2"; else echo 'Web: STOPPED'; fi
  local bin; bin="$(pg16_bin || true)"
  if [[ -n "$bin" && -f "$PGDIR/PG_VERSION" ]] && "$bin/pg_ctl" -D "$PGDIR" status >/dev/null 2>&1; then
    echo "PostgreSQL: RUNNING 127.0.0.1:$DB_PORT ($DB_NAME)"
  else echo 'PostgreSQL: STOPPED / NOT INSTALLED'; fi
}
cmd_doctor() {
  echo "AGIPAREL repo: $REPO"
  echo "State: $STATE"
  for tool in go node gofmt curl openssl brew; do
    if command -v "$tool" >/dev/null; then echo "$tool: $(command -v "$tool")"; else echo "$tool: NOT FOUND"; fi
  done
  local b; b="$(pg16_bin || true)"
  echo "PostgreSQL 16: ${b:-not installed (up will install via Homebrew)}"
  cmd_status
}
cmd_credentials() {
  [[ -f "$APPSECRETS" ]] || die 'Run up first'
  # shellcheck source=/dev/null
  . "$APPSECRETS"
  printf 'URL: http://127.0.0.1:%s/cutting/sop2\nUsername: %s\nPassword: %s\n' "$WEB_PORT" "$AGIPAREL_OPERATOR_USER" "$AGIPAREL_OPERATOR_PASSWORD"
}
case "${1:-}" in
  up) cmd_up ;;
  down) cmd_down ;;
  status) cmd_status ;;
  test) cmd_test ;;
  logs) [[ -f "$APPLOG" ]] || die 'No application log yet'; tail -n 80 "$APPLOG" ;;
  credentials) cmd_credentials ;;
  doctor) cmd_doctor ;;
  *) usage; [[ $# == 0 || "$1" == -h || "$1" == --help ]] || exit 2 ;;
esac
