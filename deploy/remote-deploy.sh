#!/usr/bin/env bash
# Run from your machine: push main to GitHub, then deploy it on the server.
#   BUGUN_SSH_HOST    SSH host or ~/.ssh/config alias   (default: bugun-server)
#   BUGUN_REMOTE_DIR  checkout on the server, absolute or relative to the
#                     remote user's home                (default: Bugun/app)
# Extra arguments are passed to deploy.sh (e.g. --no-pull).
set -euo pipefail
host=${BUGUN_SSH_HOST:-bugun-server}
dir=${BUGUN_REMOTE_DIR:-Bugun/app}

cd "$(git rev-parse --show-toplevel)"
if [ -n "$(git status --porcelain)" ]; then
  echo "Commit or stash your changes first." >&2
  exit 1
fi
if [ "$(git symbolic-ref --short -q HEAD)" != main ]; then
  echo "Switch to main first." >&2
  exit 1
fi
git push origin main
args=''
[ $# -eq 0 ] || args=$(printf ' %q' "$@")
# shellcheck disable=SC2029 # expanded locally on purpose
ssh "$host" "bash $(printf '%q' "$dir")/deploy/deploy.sh$args"
