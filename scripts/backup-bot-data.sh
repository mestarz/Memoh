#!/usr/bin/env bash
# backup-bot-data.sh — one-shot backup of every Memoh bot workspace container's /data.
#
# What it does:
#   * Finds every Docker container labelled `memoh.bot_id=<id>` (running or stopped).
#   * Streams /data out of each container with `docker cp` (works without
#     starting the container) and re-compresses to .tar.gz on the host.
#   * Writes one archive per bot under the chosen output directory plus a
#     manifest.tsv summarising bot_id, container name, image, status, size, sha256.
#
# Usage:
#   scripts/backup-bot-data.sh [OUTPUT_DIR]
#
# Defaults to ./backups/bot-data-<UTC timestamp>/ next to the repo root.
# Requires: docker, tar, gzip, sha256sum (or shasum on macOS).

set -euo pipefail

OUTPUT_DIR="${1:-}"
if [[ -z "$OUTPUT_DIR" ]]; then
  TS="$(date -u +%Y%m%dT%H%M%SZ)"
  OUTPUT_DIR="$(pwd)/backups/bot-data-${TS}"
fi

mkdir -p "$OUTPUT_DIR"
MANIFEST="$OUTPUT_DIR/manifest.tsv"
LOG="$OUTPUT_DIR/backup.log"

if command -v sha256sum >/dev/null 2>&1; then
  SHA_CMD=(sha256sum)
elif command -v shasum >/dev/null 2>&1; then
  SHA_CMD=(shasum -a 256)
else
  echo "error: need sha256sum or shasum" >&2
  exit 1
fi

if ! command -v docker >/dev/null 2>&1; then
  echo "error: docker not found in PATH" >&2
  exit 1
fi

if ! docker info >/dev/null 2>&1; then
  echo "error: cannot talk to docker daemon (is it running? do you have permission?)" >&2
  exit 1
fi

log() { printf '%s\t%s\n' "$(date -u +%FT%TZ)" "$*" | tee -a "$LOG" >&2; }

printf 'bot_id\tcontainer\timage\tstatus\tarchive\tsize_bytes\tsha256\n' > "$MANIFEST"

mapfile -t CONTAINERS < <(
  docker ps -a \
    --filter 'label=memoh.bot_id' \
    --format '{{.ID}}\t{{.Names}}\t{{.Image}}\t{{.Status}}\t{{.Label "memoh.bot_id"}}'
)

if [[ "${#CONTAINERS[@]}" -eq 0 ]]; then
  log "no containers with label memoh.bot_id found; nothing to back up"
  echo "no bot containers found; backup directory: $OUTPUT_DIR"
  exit 0
fi

log "found ${#CONTAINERS[@]} bot container(s); writing to $OUTPUT_DIR"

OK=0
FAIL=0
for row in "${CONTAINERS[@]}"; do
  IFS=$'\t' read -r CID NAME IMAGE STATUS BOT <<<"$row"
  if [[ -z "$BOT" ]]; then
    log "skip: container $NAME ($CID) has empty memoh.bot_id label"
    continue
  fi

  ARCHIVE="$OUTPUT_DIR/${BOT}.tar.gz"
  log "backup bot=$BOT container=$NAME status=$STATUS -> $(basename "$ARCHIVE")"

  # docker cp - streams a tar of the source path to stdout. We re-pipe through gzip.
  # If /data is missing we still produce a (small) archive holding just the dir entry.
  if ! docker cp -a "$CID:/data" - 2>>"$LOG" | gzip -n > "$ARCHIVE.tmp"; then
    log "  FAILED docker cp for $BOT (see $LOG); leaving partial $ARCHIVE.tmp for inspection"
    FAIL=$((FAIL+1))
    continue
  fi
  mv "$ARCHIVE.tmp" "$ARCHIVE"

  SIZE=$(stat -c '%s' "$ARCHIVE" 2>/dev/null || stat -f '%z' "$ARCHIVE")
  SHA=$("${SHA_CMD[@]}" "$ARCHIVE" | awk '{print $1}')
  printf '%s\t%s\t%s\t%s\t%s\t%s\t%s\n' \
    "$BOT" "$NAME" "$IMAGE" "$STATUS" "$(basename "$ARCHIVE")" "$SIZE" "$SHA" \
    >> "$MANIFEST"
  OK=$((OK+1))
done

log "done: $OK ok, $FAIL failed; output: $OUTPUT_DIR"
echo
echo "Backup complete."
echo "  output:    $OUTPUT_DIR"
echo "  manifest:  $MANIFEST"
echo "  log:       $LOG"
echo "  ok:        $OK"
echo "  failed:    $FAIL"

if [[ "$FAIL" -gt 0 ]]; then
  exit 1
fi
