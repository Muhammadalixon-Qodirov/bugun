#!/usr/bin/env bash
# Backs up the database (pg_dump, custom format) and the photo volume (tar.gz)
# into BACKUP_DIR, deletes local copies older than BACKUP_RETENTION_DAYS, and
# optionally copies the new files offsite with rclone (RCLONE_REMOTE).
# Exits non-zero on any failure and pings BACKUP_HEARTBEAT_URL (+ "/fail").
#
# Runs daily from deploy/systemd/bugun-backup.timer and before every deploy.
# Cron alternative (crontab -e as the deploy user; 21:30 UTC = 02:30 Tashkent,
# written for a server clock in UTC):
#   30 21 * * * /srv/bugun/app/deploy/backup.sh >> /srv/bugun/backups/backup.log 2>&1
# Restore with deploy/restore.sh.
set -Eeuo pipefail
umask 077

# shellcheck source=SCRIPTDIR/lib.sh
. "$(dirname "${BASH_SOURCE[0]}")/lib.sh"

RETENTION_DAYS=$(env_get BACKUP_RETENTION_DAYS 14)
RCLONE_REMOTE=$(env_get RCLONE_REMOTE)
RCLONE_RETENTION_DAYS=$(env_get RCLONE_RETENTION_DAYS "$RETENTION_DAYS")
HEARTBEAT=$(env_get BACKUP_HEARTBEAT_URL)
MEDIA_VOLUME="${COMPOSE_PROJECT}_media"

case $RETENTION_DAYS in '' | *[!0-9]*) echo "BACKUP_RETENTION_DAYS must be a number" >&2; exit 2 ;; esac

ping_heartbeat() {
  [ -n "$HEARTBEAT" ] || return 0
  curl -fsS -m 10 --retry 3 -o /dev/null "$HEARTBEAT$1" || echo "heartbeat ping failed" >&2
}

stamp=$(date -u +%Y%m%d-%H%M%S)
db_file="$BACKUP_DIR/bugun-$stamp-db.dump"
media_file="$BACKUP_DIR/bugun-$stamp-media.tar.gz"
new_files=("$db_file")

on_error() {
  local rc=$?
  rm -f "$db_file.part" "$media_file.part"
  echo "backup FAILED (exit $rc, line $1)" >&2
  ping_heartbeat /fail
  exit "$rc"
}
trap 'on_error $LINENO' ERR

mkdir -p "$BACKUP_DIR"

# One backup at a time (timer and deploy may overlap).
if command -v flock >/dev/null; then
  exec 9>"$BACKUP_DIR/.lock"
  flock -w 600 9
fi

pg_container=$(compose ps -q postgres)
if [ -z "$pg_container" ]; then
  echo "postgres is not running" >&2
  false
fi
# The media archive is written by a throwaway container of an image already on
# the host (the postgres image has busybox tar).
TAR_IMAGE=$(env_get BACKUP_TAR_IMAGE "$(docker inspect -f '{{.Config.Image}}' "$pg_container")")

# Database: custom format is compressed and restorable table by table.
log "dumping database"
compose exec -T postgres pg_dump -U bugun -d bugun \
  --format=custom --compress=6 --no-owner --no-privileges > "$db_file.part"
# Reading every block back (as SQL, discarded) catches a truncated archive;
# "pg_restore --list" alone does not.
compose exec -T postgres pg_restore -f /dev/null < "$db_file.part"
mv "$db_file.part" "$db_file"

# Photos: the named volume, read-only, from a throwaway container.
if docker volume inspect "$MEDIA_VOLUME" > /dev/null 2>&1; then
  log "archiving media volume $MEDIA_VOLUME"
  docker run --rm --network none -u 0 -v "$MEDIA_VOLUME:/data/media:ro" \
    --entrypoint tar "$TAR_IMAGE" -C /data -czf - media > "$media_file.part"
  tar -tzf "$media_file.part" > /dev/null
  mv "$media_file.part" "$media_file"
  new_files+=("$media_file")
else
  echo "media volume $MEDIA_VOLUME not found, skipping photos" >&2
fi

find "$BACKUP_DIR" -maxdepth 1 -type f \
  \( -name 'bugun-*.dump' -o -name 'bugun-*.tar.gz' -o -name 'bugun-*.sql.gz' \) \
  -mtime +"$RETENTION_DAYS" -delete

if [ -n "$RCLONE_REMOTE" ]; then
  log "copying offsite to $RCLONE_REMOTE"
  for f in "${new_files[@]}"; do
    rclone copy --no-traverse "$f" "$RCLONE_REMOTE"
  done
  # 0 keeps offsite copies forever (e.g. when the bucket has its own lifecycle).
  if [ "$RCLONE_RETENTION_DAYS" -gt 0 ] 2>/dev/null; then
    rclone delete --min-age "${RCLONE_RETENTION_DAYS}d" --include 'bugun-*' "$RCLONE_REMOTE"
  fi
fi

log "backup ok:"
du -h "${new_files[@]}"
ping_heartbeat ""
