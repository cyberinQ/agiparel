#!/usr/bin/env bash
# Author: Sunil Jain, 2026-11-12.
# Create and check a LOCAL-only, isolated Factory Edge pilot; never touch production.
set -Eeuo pipefail
umask 077
cd "$(dirname "$0")/.."
ENVFILE=.env.factory-v50
COMPOSEFILE=compose/docker-compose.factory-v50.yml
PROJECT=agiparel-v50-pilot
MODE="${1:---help}"
die(){ echo "STOP: $*" >&2; exit 1; }
require_env(){ [[ -f "$ENVFILE" ]] || die "run $0 --init first"; }
compose(){ docker compose -p "$PROJECT" --env-file "$ENVFILE" -f "$COMPOSEFILE" "$@"; }
check_pins(){
  local key val
  for key in AGIPAREL_AEROLAKE_IMAGE AGIPAREL_OLLAMA_IMAGE; do
    val="$(sed -n "s/^${key}=//p" "$ENVFILE" | tail -n1)"
    [[ "$val" =~ ^[a-zA-Z0-9._:/-]+@sha256:[a-f0-9]{64}$ ]] || die "$key must be an approved registry image with an exact sha256 digest. No bundled Aerolake engine image exists yet."
  done
}
case "$MODE" in
  --init)
    [[ ! -e "$ENVFILE" ]] || die "$ENVFILE already exists; refusing to overwrite secrets"
    command -v openssl >/dev/null || die 'openssl is required to generate passwords'
    target="$(mktemp ./.v50-env.XXXXXX)"
    trap 'rm -f "$target"' EXIT
    chmod 600 "$target"
    {
      echo '# Author: Sunil Jain, 2026-11-12 — LOCAL PILOT ONLY. Never expose publicly.'
      echo "POSTGRES_PASSWORD=$(openssl rand -hex 24)"
      echo "AGIPAREL_APP_DB_PASSWORD=$(openssl rand -hex 24)"
      echo "AGIPAREL_DOC_DB_PASSWORD=$(openssl rand -hex 24)"
      echo "LEDGER_HMAC_SECRET=$(openssl rand -hex 32)"
      echo 'AGIPAREL_OPERATOR_USER=pilotoperator'
      echo "AGIPAREL_OPERATOR_PASSWORD=$(openssl rand -hex 24)"
      echo 'AGIPAREL_PILOT_PORT=18539'
      echo 'AGIPAREL_FACTORY_TIMEZONE=Asia/Kolkata'
      echo 'AGIE_MODEL=qwen3:8b'
      echo 'AGIPAREL_AEROLAKE_IMAGE=REPLACE_WITH_APPROVED_IMAGE_DIGEST'
      echo 'AGIPAREL_OLLAMA_IMAGE=REPLACE_WITH_APPROVED_IMAGE_DIGEST'
    } > "$target"
    mv "$target" "$ENVFILE"
    trap - EXIT
    echo "PASS: generated $ENVFILE with restricted permissions."
    echo 'NEXT: set the factory time zone, an approved Ollama image digest, and an approved Aerolake image digest.'
    echo 'These image digests must come from your authorized release registry; they are not bundled.'
    ;;
  --check|--up|--verify|--down)
    require_env
    python3 - "$ENVFILE" <<'PYMODE' || die 'pilot credentials file must have chmod 600'
# Author: Sunil Jain, 2026-11-12 — Portable private credential-file check.
import os, stat, sys
if stat.S_IMODE(os.stat(sys.argv[1]).st_mode) != 0o600:
    sys.exit(1)
PYMODE
    command -v docker >/dev/null || die 'Docker Desktop / Engine required'
    command -v python3 >/dev/null || die 'Python 3 is required for safe secret-file checks'
    command -v curl >/dev/null || die 'curl is required'
    if [[ "$MODE" == --check || "$MODE" == --up ]]; then
      check_pins
      [[ -z "${AGIPAREL_ENVIRONMENT:-}" || "${AGIPAREL_ENVIRONMENT}" == development ]] || die 'not intended for production'
      compose config --quiet
      echo 'PASS: isolated project, image pins, config and local-only port validated.'
      if [[ "$MODE" == --check ]]; then exit 0; fi
    fi
    case "$MODE" in
      --up)
        [[ "${AGIPAREL_CONFIRM_V50_PILOT:-}" == YES ]] || die 'set AGIPAREL_CONFIRM_V50_PILOT=YES to start the isolated pilot'
        echo 'Starting seven isolated services; no existing AGIPAREL volumes or containers are reused.'
        compose up -d --build
        model="$(sed -n 's/^AGIE_MODEL=//p' "$ENVFILE" | tail -n1)"
        [[ "$model" =~ ^[a-zA-Z0-9][a-zA-Z0-9_./:-]{1,100}$ ]] || die 'invalid Ollama model name'
        compose exec -T ollama ollama pull "$model"
        "$0" --verify
        ;;
      --verify)
        port="$(sed -n 's/^AGIPAREL_PILOT_PORT=//p' "$ENVFILE" | tail -n1)"
        [[ "$port" =~ ^[0-9]{4,5}$ ]] || die 'invalid local pilot port'
        compose ps
        for service in postgres redis ollama aerolake-agentd aerolake-bridge plant-edge-os document-intake; do
          cid="$(compose ps -q "$service")"
          [[ -n "$cid" ]] || die "$service container not found"
          [[ "$(docker inspect --format '{{.State.Running}}' "$cid")" == true ]] || die "$service container is not running"
        done
        curl --fail --silent --show-error --max-time 10 "http://127.0.0.1:${port}/healthz" > /dev/null || die 'AGIPAREL localhost health unavailable'
        model="$(sed -n 's/^AGIE_MODEL=//p' "$ENVFILE" | tail -n1)"
        compose exec -T ollama ollama show "$model" >/dev/null || die 'required local model not installed'
        echo 'PASS: local edge responds and the model is installed.'
        echo 'UNVERIFIED: Aerolake CDC/Flight archival, hardware signing, production workforce SSO and live operator identity.'
        echo "LOCAL URL: http://127.0.0.1:${port}/"
        ;;
      --down)
        compose down  # Intentionally preserve volumes. Never run down -v.
        echo 'PASS: pilot stopped. Its named data volumes are retained.'
        ;;
    esac
    ;;
  *)
    cat <<'HELP'
AGIPAREL v50 isolated factory pilot (NOT production):
  bash scripts/factory_pilot_v50.sh --init       Create isolated .env.factory-v50
  bash scripts/factory_pilot_v50.sh --check      Validate approved image references
  AGIPAREL_CONFIRM_V50_PILOT=YES bash scripts/factory_pilot_v50.sh --up
  bash scripts/factory_pilot_v50.sh --verify     Confirm localhost and AI model
  bash scripts/factory_pilot_v50.sh --down       Stop without deleting any volumes
HELP
    ;;
esac
