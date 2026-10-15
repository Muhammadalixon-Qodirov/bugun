# shellcheck shell=bash
# Shared settings for deploy.sh, backup.sh and restore.sh (sourced, not run).
#
# Layout (all paths overridable through the environment):
#   DEPLOY_DIR      the git checkout            default: this repository's root
#   BUGUN_ENV_FILE  server secrets and settings default: $DEPLOY_DIR/../.env
#   BACKUP_DIR      dumps and media archives    default: $DEPLOY_DIR/../backups
# Settings not given in the environment are read from BUGUN_ENV_FILE.
#
# The scripts need no root: a user in the "docker" group that owns
# DEPLOY_DIR, BACKUP_DIR and BUGUN_ENV_FILE is enough.

DEPLOY_DIR=${DEPLOY_DIR:-$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)}
BUGUN_ENV_FILE=${BUGUN_ENV_FILE:-$(dirname "$DEPLOY_DIR")/.env}
COMPOSE_PROJECT=${COMPOSE_PROJECT:-bugun}
export DEPLOY_DIR BUGUN_ENV_FILE

if [ ! -r "$BUGUN_ENV_FILE" ]; then
  echo "env file not readable: $BUGUN_ENV_FILE (set BUGUN_ENV_FILE)" >&2
  exit 1
fi

# env_get NAME DEFAULT: the environment wins, then the env file, then DEFAULT.
# The file is parsed, never sourced, so values with shell characters are safe.
env_get() {
  local name=$1 default=${2-} value
  if [ -n "${!name-}" ]; then
    printf '%s' "${!name}"
    return
  fi
  # awk always exits 0: a failing pipeline here would fire callers' ERR traps.
  value=$(awk -v n="$name" 'index($0, n "=") == 1 { v = substr($0, length(n) + 2) } END { print v }' "$BUGUN_ENV_FILE")
  value=${value%$'\r'}
  value=${value#\"}; value=${value%\"}
  printf '%s' "${value:-$default}"
}

# shellcheck disable=SC2034 # used by the scripts that source this file
BACKUP_DIR=$(env_get BACKUP_DIR "$(dirname "$DEPLOY_DIR")/backups")

compose() {
  docker compose -p "$COMPOSE_PROJECT" -f "$DEPLOY_DIR/deploy/docker-compose.yml" \
    --env-file "$BUGUN_ENV_FILE" "$@"
}

log() { printf '%s %s\n' "$(date -u +%H:%M:%S)" "$*"; }
