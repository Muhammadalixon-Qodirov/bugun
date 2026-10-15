#!/usr/bin/env bash
# Restores a backup made by deploy/backup.sh. DESTRUCTIVE: the current database
# and/or photo volume are replaced.
#
#   deploy/restore.sh --db backups/bugun-20260926-023000-db.dump \
#                     --media backups/bugun-20260926-023000-media.tar.gz [--yes]
#
# Either file may be given alone. Older plain-SQL dumps (bugun-*.sql.gz) are
# accepted for --db too. The API is stopped during the restore and started
# again afterwards with the "latest" image (or API_IMAGE_TAG).
# Offsite copies: fetch them first, e.g. rclone copy "$RCLONE_REMOTE/<file>" backups/
set -Eeuo pipefail

usage() { sed -n '2,11p' "$0" | sed 's/^# \{0,1\}//'; exit 2; }

db_file='' media_file='' yes=''
while [ $# -gt 0 ]; do
  case $1 in
    --db) db_file=${2:?--db needs a file}; shift 2 ;;
    --media) media_file=${2:?--media needs a file}; shift 2 ;;
    --yes | -y) yes=1; shift ;;
    -h | --help) usage ;;
    *) echo "unknown argument: $1" >&2; usage ;;
  esac
done
[ -n "$db_file$media_file" ] || usage
for f in "$db_file" "$media_file"; do
  [ -z "$f" ] || [ -r "$f" ] || { echo "cannot read $f" >&2; exit 2; }
done

# shellcheck source=SCRIPTDIR/lib.sh
. "$(dirname "${BASH_SOURCE[0]}")/lib.sh"
MEDIA_VOLUME="${COMPOSE_PROJECT}_media"

echo "This replaces, in compose project '$COMPOSE_PROJECT':"
[ -z "$db_file" ] || echo "  database 'bugun'    <- $db_file"
[ -z "$media_file" ] || echo "  volume '$MEDIA_VOLUME' <- $media_file"
if [ -z "$yes" ]; then
  read -r -p "Type 'restore' to continue: " answer
  [ "$answer" = restore ] || { echo "aborted"; exit 1; }
fi

trap 'echo "restore FAILED at line $LINENO; the API is stopped, fix and rerun or start it with: docker compose ... up -d api" >&2' ERR

compose up -d --wait postgres
log "stopping api"
compose stop api

if [ -n "$db_file" ]; then
  log "recreating database from $db_file"
  compose exec -T postgres psql -U bugun -d postgres -v ON_ERROR_STOP=1 \
    -c 'DROP DATABASE IF EXISTS bugun WITH (FORCE)' -c 'CREATE DATABASE bugun OWNER bugun'
  case $db_file in
    *.sql.gz)
      gunzip -c "$db_file" | compose exec -T postgres psql -U bugun -d bugun -v ON_ERROR_STOP=1 -q ;;
    *)
      compose exec -T postgres pg_restore -U bugun -d bugun --no-owner --no-privileges \
        --exit-on-error --single-transaction < "$db_file" ;;
  esac
fi

if [ -n "$media_file" ]; then
  log "replacing photos from $media_file"
  tar -tzf "$media_file" > /dev/null
  tar_image=$(env_get BACKUP_TAR_IMAGE "$(docker inspect -f '{{.Config.Image}}' "$(compose ps -q postgres)")")
  # The archive holds a top-level "media/" directory; files belong to uid 10001 (app).
  docker run --rm -i --network none -u 0 -v "$MEDIA_VOLUME:/data/media" --entrypoint sh "$tar_image" \
    -c 'find /data/media -mindepth 1 -delete && tar -C /data -xzf - && chown -R 10001:10001 /data/media' \
    < "$media_file"
fi

# A dump older than the current code needs the newer migrations.
log "applying migrations"
compose run --rm migrate up

log "starting api"
compose up -d api
echo "restore done; check: curl -fsS http://127.0.0.1:$(env_get API_PORT 8090)/healthz"
