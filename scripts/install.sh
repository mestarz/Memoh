#!/usr/bin/env bash
# Memoh systemd --user installer (single entry point).
#
# 1. (Optional) runs scripts/check.sh first.
# 2. Creates ~/.config/Memoh/{config.toml, data/, data/run, data/workspaces}.
# 3. Renders the three service unit files into ~/.config/systemd/user/,
#    substituting %h/workspace/memoh/Memoh with the actual repo root.
# 4. Reloads systemd and (with --enable) enables + starts everything.
#
# Usage:
#   scripts/install.sh                 # install units (do not enable/start)
#   scripts/install.sh --enable        # install + enable + start
#   scripts/install.sh --remove        # disable & remove units (data preserved)
#   scripts/install.sh --skip-check    # don't run scripts/check.sh first
#
# After --enable:
#   journalctl --user -u memoh-server -f
#   systemctl --user status 'memoh-*.service'
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
SYSTEMD_USER_DIR="$HOME/.config/systemd/user"
CONFIG_DIR="${XDG_CONFIG_HOME:-$HOME/.config}/Memoh"
CONFIG_FILE="$CONFIG_DIR/config.toml"
DATA_ROOT="$CONFIG_DIR/data"
SERVICES=(memoh-infra.service memoh-server.service memoh-web.service)

ACTION="install"
RUN_CHECK=true
for arg in "$@"; do
  case "$arg" in
    --enable)     ACTION="enable" ;;
    --remove)     ACTION="remove" ;;
    --skip-check) RUN_CHECK=false ;;
    -h|--help)    sed -n '2,18p' "$0"; exit 0 ;;
    *) echo "[install] unknown option: $arg" >&2; exit 2 ;;
  esac
done

log() { printf '\033[1;34m[install]\033[0m %s\n' "$*"; }

# 0. (optional) check --------------------------------------------------------
if [ "$ACTION" != "remove" ] && $RUN_CHECK && [ -x "$SCRIPT_DIR/check.sh" ]; then
  log "running scripts/check.sh first (use --skip-check to bypass)"
  if ! "$SCRIPT_DIR/check.sh"; then
    log "ERROR: environment check failed; aborting. Re-run with --skip-check to ignore."
    exit 1
  fi
fi

remove_services() {
  local svc
  for svc in "${SERVICES[@]}"; do
    systemctl --user stop    "$svc" 2>/dev/null || true
    systemctl --user disable "$svc" 2>/dev/null || true
    rm -f "$SYSTEMD_USER_DIR/$svc"
    log "removed $svc"
  done
  systemctl --user daemon-reload
  log "data preserved at $DATA_ROOT"
}

if [ "$ACTION" = "remove" ]; then
  remove_services
  exit 0
fi

# 1. Data + config -----------------------------------------------------------
log "ensuring data dirs under $DATA_ROOT"
mkdir -p "$DATA_ROOT"/{run,workspaces}
chmod 0750 "$DATA_ROOT/run"

if [ ! -f "$CONFIG_FILE" ]; then
  log "writing default config from conf/app.example.toml → $CONFIG_FILE"
  install -D -m 0600 "$PROJECT_ROOT/conf/app.example.toml" "$CONFIG_FILE"
  log "⚠  edit $CONFIG_FILE before starting:"
  log "    - [admin] username/password"
  log "    - [auth].jwt_secret (generate: openssl rand -hex 32)"
  log "    - [postgres]/[qdrant]/[sparse] credentials matching docker-compose.yml"
  log "    - [container].default_image (recommended: memoh/workspace:debian)"
else
  log "config already present, leaving $CONFIG_FILE untouched"
fi

# 2. Sanity: required artifacts ----------------------------------------------
for f in "$PROJECT_ROOT/bin/memoh-server" "$PROJECT_ROOT/data/runtime/bridge"; do
  if [ ! -x "$f" ]; then
    log "ERROR: $f missing — run scripts/build.sh first"
    exit 1
  fi
done

# 3. Install systemd units ---------------------------------------------------
mkdir -p "$SYSTEMD_USER_DIR"
for svc in "${SERVICES[@]}"; do
  src="$PROJECT_ROOT/systemd/$svc"
  if [ ! -f "$src" ]; then
    log "ERROR: $src not found"
    exit 1
  fi
  # Substitute the canonical placeholder path with the real repo path so
  # users can clone the repo anywhere.
  sed "s|%h/workspace/memoh/Memoh|$PROJECT_ROOT|g" "$src" \
    > "$SYSTEMD_USER_DIR/$svc"
  log "installed $svc → $SYSTEMD_USER_DIR/$svc"
done
systemctl --user daemon-reload
log "systemctl --user daemon-reload done"

# 4. Enable + start ----------------------------------------------------------
if [ "$ACTION" = "enable" ]; then
  for svc in "${SERVICES[@]}"; do
    systemctl --user enable "$svc"
    systemctl --user restart "$svc"
    log "enabled + (re)started $svc"
  done
  echo
  log "all services running. Check with:"
  log "  systemctl --user status 'memoh-*.service'"
  log "  journalctl --user -u memoh-server -f"
  log ""
  log "Web UI: http://localhost:18082 (or LAN IP)"
  log "API:    http://localhost:18731"
else
  echo
  log "units installed. To enable + start:"
  log "  $0 --enable"
  log "Or manually:"
  log "  systemctl --user enable --now memoh-infra.service memoh-server.service memoh-web.service"
fi
