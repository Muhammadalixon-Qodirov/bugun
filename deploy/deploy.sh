#!/usr/bin/env bash
# Runs on the server: fast-forward the checkout, back up, build an image tagged
# with the git SHA, migrate, switch the API, and roll back if it is unhealthy.
#
#   deploy/deploy.sh             pull origin/$DEPLOY_BRANCH, then deploy it
#   deploy/deploy.sh --no-pull   redeploy the current checkout
#
# Steps:
#   1. refuse if the checkout has local changes; fetch, fast-forward only
#   2. back up database and photos (deploy/backup.sh) if postgres runs
#   3. build bugun-api:<sha> (base images pinned in backend/Dockerfile)
#   4. "migrate up" in the new image, as its own step; on failure the old
#      API keeps running and nothing else changes
#   5. start the new API and wait for GET /healthz = 200 (503 if the
#      database is unreachable)
#   6. healthy: tag it bugun-api:latest, keep the last KEEP_IMAGES tags
#      unhealthy: restart bugun-api:latest (the previous good image), move the
#      checkout back and exit 1. Migrations are not reverted, so they must stay
#      compatible with the previous release (add, do not drop or rename).
#
# Settings (environment or the server .env, see deploy/.env.example):
# DEPLOY_DIR, BUGUN_ENV_FILE, BACKUP_DIR, DEPLOY_BRANCH, HEALTH_TIMEOUT,
# KEEP_IMAGES, API_PORT.
#
# Runs as any user in the "docker" group (root is not needed; note that docker
# group membership is itself root-equivalent on the host). One-time setup:
#   sudo adduser --disabled-password --gecos "" deploy
#   sudo usermod -aG docker deploy
#   sudo install -d -o deploy -g deploy -m 750 /srv/bugun
#   sudo -u deploy git clone <repository> /srv/bugun/app
#   sudo -u deploy install -m 600 /srv/bugun/app/deploy/.env.example /srv/bugun/.env
# An existing checkout can simply be moved (volumes belong to the compose
# project "bugun", not to the path):
#   sudo mv /root/Bugun /srv/bugun && sudo chown -R deploy:deploy /srv/bugun
set -Eeuo pipefail

# Everything runs inside main(), which bash reads completely before running,
# so the fast-forward below cannot change the script under our feet.
main() {
  trap 'echo "deploy FAILED at line $LINENO" >&2' ERR
  # shellcheck source=SCRIPTDIR/lib.sh
  . "$(dirname "${BASH_SOURCE[0]}")/lib.sh"
  local branch health_timeout keep api_port
  branch=$(env_get DEPLOY_BRANCH main)
  health_timeout=$(env_get HEALTH_TIMEOUT 60)
  keep=$(env_get KEEP_IMAGES 5)
  api_port=$(env_get API_PORT 8090)

  cd "$DEPLOY_DIR"
  if [ "${1-}" != --no-pull ]; then
    pull "$branch"
    # Continue with the freshly pulled deploy.sh.
    DEPLOY_PREV_SHA=$prev_sha exec bash "$DEPLOY_DIR/deploy/deploy.sh" --no-pull
  fi
  local prev_sha=${DEPLOY_PREV_SHA:-$(git rev-parse HEAD)}

  local tag
  tag=$(git rev-parse --short=12 HEAD)
  export API_IMAGE_TAG=$tag
  log "deploying $(git log -1 --format='%h %s')"

  if compose ps --status running -q postgres | grep -q .; then
    "$DEPLOY_DIR/deploy/backup.sh"
  fi

  local have_prev=''
  if docker image inspect bugun-api:latest > /dev/null 2>&1; then
    have_prev=1
  fi

  log "building bugun-api:$tag"
  compose build --pull --build-arg "GIT_SHA=$(git rev-parse HEAD)" api
  compose up -d --wait postgres

  log "running migrations"
  if ! compose run --rm migrate up; then
    echo "migrations failed; the running API was left unchanged" >&2
    restore_checkout "$prev_sha"
    exit 1
  fi

  log "starting bugun-api:$tag"
  if compose up -d --remove-orphans api && wait_healthy "$api_port" "$health_timeout"; then
    docker tag "bugun-api:$tag" bugun-api:latest
    prune_images "$tag" "$keep"
    log "deployed $tag"
    exit 0
  fi

  echo "bugun-api:$tag is not healthy; recent logs:" >&2
  compose logs --tail 80 api >&2 || true
  restore_checkout "$prev_sha"
  if [ -z "$have_prev" ]; then
    echo "no previous image (bugun-api:latest) to roll back to" >&2
    exit 1
  fi
  log "rolling back to bugun-api:latest"
  API_IMAGE_TAG=latest compose up -d api
  if wait_healthy "$api_port" "$health_timeout"; then
    echo "rolled back to the previous image; deploy of $tag FAILED" >&2
  else
    echo "rollback is unhealthy too; check 'docker compose ... logs api'" >&2
  fi
  exit 1
}

prev_sha=''
pull() {
  local branch=$1 current
  if [ -n "$(git status --porcelain)" ]; then
    echo "the server checkout has local changes; refusing to deploy:" >&2
    git status --short >&2
    exit 1
  fi
  current=$(git symbolic-ref --short -q HEAD || true)
  if [ "$current" != "$branch" ]; then
    echo "the checkout is on '${current:-detached HEAD}', expected '$branch'" >&2
    exit 1
  fi
  prev_sha=$(git rev-parse HEAD)
  git fetch --quiet origin "$branch"
  if ! git merge --ff-only --quiet "origin/$branch"; then
    echo "origin/$branch cannot be fast-forwarded onto the server checkout; resolve by hand" >&2
    exit 1
  fi
  log "code: $(git rev-parse --short "$prev_sha") -> $(git rev-parse --short HEAD)"
}

# Point the checkout back at the release that is still running.
restore_checkout() {
  if [ "$(git rev-parse HEAD)" != "$1" ]; then
    git reset --quiet --keep "$1"
    log "checkout moved back to $(git rev-parse --short HEAD)"
  fi
}

wait_healthy() {
  local port=$1 timeout=$2 code='' i
  for ((i = 1; i <= timeout; i++)); do
    code=$(curl -s -o /dev/null -m 3 -w '%{http_code}' "http://127.0.0.1:$port/healthz" || true)
    if [ "$code" = 200 ]; then
      log "healthy after ${i}s"
      return 0
    fi
    sleep 1
  done
  echo "/healthz still answers '$code' after ${timeout}s" >&2
  return 1
}

# Keep "latest", the current tag and the newest $2 SHA tags; drop the rest.
prune_images() {
  local current=$1 keep=$2 n=0 t
  while read -r t; do
    case $t in latest | "$current" | '<none>') continue ;; esac
    n=$((n + 1))
    if [ "$n" -ge "$keep" ]; then
      docker image rm "bugun-api:$t" > /dev/null || true
    fi
  done < <(docker image ls bugun-api --format '{{.Tag}}')
  docker image prune -f --filter label=app=bugun > /dev/null
}

main "$@"
