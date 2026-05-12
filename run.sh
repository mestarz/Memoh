#!/usr/bin/env bash
# Manage host-mode Memoh services:
#   docker compose infra (postgres, qdrant, sparse, browser)
#   memoh-server (host process, listening on 127.0.0.1:18731)
#   vite dev server (host process, serving web on :8082)
#
# Usage:
#   run.sh           # start everything (alias: -u / start)
#   run.sh -s        # stop everything (also: stop)
#   run.sh -r        # restart only memoh-server + web (for new build)
#   run.sh status    # show what is running
#   run.sh logs      # tail memoh-server log
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$ROOT_DIR"

CONFIG_PATH="${CONFIG_PATH:-$HOME/.config/Memoh/config.toml}"
USERDATA_DIR="$(dirname "$CONFIG_PATH")"

SERVER_BIN="$ROOT_DIR/bin/memoh-server"
SERVER_PID_FILE="$USERDATA_DIR/local-server.pid"
SERVER_LOG_FILE="$USERDATA_DIR/local-server.log"

WEB_PID_FILE="$USERDATA_DIR/web.pid"
WEB_LOG_FILE="$USERDATA_DIR/web.log"
WEB_HOST="${WEB_HOST:-0.0.0.0}"
WEB_PORT="${WEB_PORT:-8082}"

SERVER_URL="http://127.0.0.1:18731"

export PATH="$HOME/.local/share/mise/shims:$HOME/.local/bin:$PATH"

log() { echo "[run] $*"; }

usage() {
  cat <<'EOF'
Usage: run.sh [start|stop|restart|status|logs]

  start (default)  Bring up infra (docker compose) + memoh-server + web (vite)
  stop  | -s       Stop memoh-server + web + docker compose
  restart | -r     Restart only memoh-server and web (keep infra running) -
                   use after `./build.sh` to pick up new binaries
  status           Show what is running
  logs             Tail memoh-server log (Ctrl-C to exit)
  -h | --help      Show this help
EOF
}

# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------
ensure_config() {
  if [ ! -f "$CONFIG_PATH" ]; then
    log "ERROR: config.toml not found at $CONFIG_PATH"
    log "Copy conf/app.example.toml and adjust [postgres]/[qdrant]/[sparse]/[container] for host mode."
    exit 1
  fi
}

ensure_binaries() {
  if [ ! -x "$SERVER_BIN" ]; then
    log "ERROR: $SERVER_BIN not found. Run ./build.sh first."
    exit 1
  fi
}

is_pid_alive() {
  local pid_file="$1"
  [ -f "$pid_file" ] || return 1
  local pid
  pid="$(cat "$pid_file" 2>/dev/null || true)"
  [ -n "$pid" ] && kill -0 "$pid" 2>/dev/null
}

kill_pid_file() {
  local pid_file="$1" name="$2"
  if [ -f "$pid_file" ]; then
    local pid
    pid="$(cat "$pid_file" 2>/dev/null || true)"
    if [ -n "$pid" ] && kill -0 "$pid" 2>/dev/null; then
      log "Stopping $name (pid $pid)"
      kill "$pid" 2>/dev/null || true
      for _ in 1 2 3 4 5; do
        kill -0 "$pid" 2>/dev/null || break
        sleep 1
      done
      kill -0 "$pid" 2>/dev/null && { log "Force-killing $name (pid $pid)"; kill -9 "$pid" 2>/dev/null || true; }
    fi
    rm -f "$pid_file"
  fi
}

# ---------------------------------------------------------------------------
# Service actions
# ---------------------------------------------------------------------------
start_infra() {
  log "Starting infra (docker compose: postgres, qdrant, sparse, browser)"
  docker compose up -d
}

stop_infra() {
  log "Stopping infra (docker compose down)"
  docker compose down
}

run_migrations() {
  log "Running database migrations"
  CONFIG_PATH="$CONFIG_PATH" "$SERVER_BIN" migrate up
}

start_server() {
  if is_pid_alive "$SERVER_PID_FILE"; then
    log "memoh-server already running (pid $(cat "$SERVER_PID_FILE"))"
    return 0
  fi
  mkdir -p "$USERDATA_DIR"
  log "Starting memoh-server → $SERVER_URL  (log: $SERVER_LOG_FILE)"
  setsid env CONFIG_PATH="$CONFIG_PATH" "$SERVER_BIN" serve \
    > "$SERVER_LOG_FILE" 2>&1 < /dev/null &
  local pid=$!
  echo "$pid" > "$SERVER_PID_FILE"
  # Wait for /ping
  for i in $(seq 1 30); do
    if curl -sf "$SERVER_URL/ping" > /dev/null 2>&1; then
      log "memoh-server ready (pid $pid, ${i}s)"
      return 0
    fi
    sleep 1
  done
  log "WARN: memoh-server did not respond on /ping within 30s — see $SERVER_LOG_FILE"
}

stop_server() {
  kill_pid_file "$SERVER_PID_FILE" "memoh-server"
}

start_web() {
  if is_pid_alive "$WEB_PID_FILE"; then
    log "web already running (pid $(cat "$WEB_PID_FILE"))"
    return 0
  fi
  mkdir -p "$USERDATA_DIR"
  log "Starting web (vite) → http://$WEB_HOST:$WEB_PORT  (log: $WEB_LOG_FILE)"
  setsid env MEMOH_WEB_PROXY_TARGET="$SERVER_URL" \
    pnpm --filter @memohai/web exec vite \
      --host "$WEB_HOST" --port "$WEB_PORT" \
    > "$WEB_LOG_FILE" 2>&1 < /dev/null &
  local pid=$!
  echo "$pid" > "$WEB_PID_FILE"
  sleep 3
  if kill -0 "$pid" 2>/dev/null; then
    log "web started (pid $pid)"
  else
    log "WARN: web exited immediately — see $WEB_LOG_FILE"
  fi
}

stop_web() {
  kill_pid_file "$WEB_PID_FILE" "web"
  # vite spawns child node process; clean leftovers bound to our port.
  local stragglers
  stragglers="$(ss -tnlp 2>/dev/null | awk -v p=":$WEB_PORT" '$4 ~ p {print $0}' | grep -oP 'pid=\K[0-9]+' | sort -u || true)"
  if [ -n "$stragglers" ]; then
    log "Killing leftover web pids: $stragglers"
    echo "$stragglers" | xargs -r kill 2>/dev/null || true
  fi
}

show_status() {
  echo "=== infra (docker) ==="
  docker compose ps 2>/dev/null || true
  echo ""
  echo "=== memoh-server ==="
  if is_pid_alive "$SERVER_PID_FILE"; then
    local pid; pid="$(cat "$SERVER_PID_FILE")"
    ps -p "$pid" -o pid,etime,cmd
    curl -sf "$SERVER_URL/ping" && echo
  else
    echo "(not running)"
  fi
  echo ""
  echo "=== web (vite) ==="
  if is_pid_alive "$WEB_PID_FILE"; then
    ps -p "$(cat "$WEB_PID_FILE")" -o pid,etime,cmd
  else
    echo "(not running)"
  fi
  echo ""
  echo "=== bot workspaces (docker) ==="
  docker ps --filter 'name=workspace-' --format 'table {{.Names}}\t{{.Status}}'
}

# ---------------------------------------------------------------------------
# Dispatch
# ---------------------------------------------------------------------------
ACTION="start"
if [ $# -gt 0 ]; then
  case "$1" in
    start|-u|"") ACTION="start" ;;
    stop|-s)     ACTION="stop" ;;
    restart|-r)  ACTION="restart" ;;
    status)      ACTION="status" ;;
    logs)        ACTION="logs" ;;
    -h|--help)   usage; exit 0 ;;
    *) log "Unknown command: $1"; usage; exit 1 ;;
  esac
fi

case "$ACTION" in
  start)
    ensure_config
    ensure_binaries
    start_infra
    sleep 3   # give postgres a moment for healthcheck
    run_migrations
    start_server
    start_web
    log "All services started."
    log "  Server: $SERVER_URL"
    log "  Web   : http://$WEB_HOST:$WEB_PORT"
    ;;
  stop)
    stop_web
    stop_server
    stop_infra
    log "All services stopped."
    ;;
  restart)
    ensure_config
    ensure_binaries
    log "Restarting memoh-server + web (infra untouched)"
    stop_web
    stop_server
    run_migrations
    start_server
    start_web
    log "Restart complete."
    ;;
  status)
    show_status
    ;;
  logs)
    [ -f "$SERVER_LOG_FILE" ] || { log "no log at $SERVER_LOG_FILE"; exit 1; }
    exec tail -f "$SERVER_LOG_FILE"
    ;;
esac
