#!/usr/bin/env bash
# Migrate existing bot workspace containers to the new bind-mount /data layout.
#
# Background: starting from this commit, the workspace container's /data is
# bind-mounted from <data_root>/workspaces/<bot_id>/data on the host, so data
# survives container deletion / image rebuilds.
#
# This script:
#   1. Restores each bot's previously backed-up /data (from a tar.gz produced
#      by scripts/backup-bot-data.sh) into <data_root>/workspaces/<bot_id>/data.
#   2. Deletes the existing docker container so the next server start recreates
#      it with the new bind-mount layout.
#
# Usage:
#   scripts/migrate-bot-data-to-bind-mount.sh \
#       --backup-dir backups/bot-data-YYYYMMDDTHHMMSSZ \
#       [--data-root ~/.config/Memoh/data] \
#       [--container-prefix workspace-] \
#       [--dry-run]
#
# Required: server should be stopped before running so containers are not in
# use, e.g. `systemctl --user stop memoh-server.service`.

set -euo pipefail

BACKUP_DIR=""
DATA_ROOT="${HOME}/.config/Memoh/data"
CONTAINER_PREFIX="workspace-"
DRY_RUN=0

while [[ $# -gt 0 ]]; do
  case "$1" in
    --backup-dir) BACKUP_DIR="$2"; shift 2 ;;
    --data-root) DATA_ROOT="$2"; shift 2 ;;
    --container-prefix) CONTAINER_PREFIX="$2"; shift 2 ;;
    --dry-run) DRY_RUN=1; shift ;;
    -h|--help) sed -n '2,25p' "$0"; exit 0 ;;
    *) echo "unknown flag: $1" >&2; exit 2 ;;
  esac
done

if [[ -z "$BACKUP_DIR" ]]; then
  echo "ERROR: --backup-dir is required" >&2
  exit 2
fi
if [[ ! -d "$BACKUP_DIR" ]]; then
  echo "ERROR: backup dir not found: $BACKUP_DIR" >&2
  exit 2
fi
if [[ ! -f "$BACKUP_DIR/manifest.tsv" ]]; then
  echo "ERROR: manifest.tsv missing under $BACKUP_DIR" >&2
  exit 2
fi

run() {
  if [[ "$DRY_RUN" -eq 1 ]]; then
    echo "+ $*"
  else
    eval "$@"
  fi
}

# Refuse to run when server is alive and has open handles on the containers.
if pgrep -x memoh-server >/dev/null 2>&1; then
  echo "ERROR: memoh-server is still running. Stop it first:" >&2
  echo "       systemctl --user stop memoh-server.service" >&2
  exit 1
fi

mapfile -t LINES < <(tail -n +2 "$BACKUP_DIR/manifest.tsv")
if [[ ${#LINES[@]} -eq 0 ]]; then
  echo "no entries in manifest, nothing to do"
  exit 0
fi

ok=0
fail=0
for line in "${LINES[@]}"; do
  bot_id="$(printf '%s' "$line" | cut -f1)"
  archive="$(printf '%s' "$line" | cut -f5)"
  archive_path="$BACKUP_DIR/$archive"

  if [[ -z "$bot_id" || ! -f "$archive_path" ]]; then
    echo "SKIP: bot=$bot_id archive=$archive (missing)" >&2
    fail=$((fail+1))
    continue
  fi

  target_parent="$DATA_ROOT/workspaces/$bot_id"
  target_data="$target_parent/data"
  echo "==> $bot_id"
  echo "    archive: $archive_path"
  echo "    target : $target_data"

  if [[ -d "$target_data" ]] && [[ -n "$(ls -A "$target_data" 2>/dev/null || true)" ]]; then
    echo "    skip extract: $target_data already exists and is non-empty"
  else
    run "mkdir -p '$target_parent'"
    # Tarball top-level entry is 'data/' (from docker cp CID:/data -),
    # so extracting at $target_parent yields $target_parent/data/...
    run "tar -xzf '$archive_path' -C '$target_parent'"
  fi

  ctr_name="${CONTAINER_PREFIX}${bot_id}"
  if docker inspect "$ctr_name" >/dev/null 2>&1; then
    echo "    removing legacy container $ctr_name"
    run "docker rm -f '$ctr_name' >/dev/null"
  else
    echo "    no existing container named $ctr_name (already removed)"
  fi

  ok=$((ok+1))
done

echo
echo "done: $ok migrated, $fail failed"
echo "next: systemctl --user start memoh-server.service"
echo "      (containers will be recreated on first bot interaction with bind-mount /data)"
