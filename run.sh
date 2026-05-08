#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")" && pwd)"

log() {
  echo "[run] $*"
}

usage() {
  cat <<'EOF'
Usage: run.sh [OPTIONS]

Manage Memoh services via docker compose.

Options:
  (no flags)   Start all services
  -s           Stop all services
  -r           Restart all services
  -h           Show this help message
EOF
}

# Detect host's outbound IP for WebRTC NAT candidate announcement
setup_host_ip() {
  local host_ip
  host_ip="$(ip route get 8.8.8.8 2>/dev/null | grep -oP '(?<=src )\S+' | head -1 || true)"
  if [ -z "$host_ip" ]; then
    host_ip="$(hostname -I 2>/dev/null | awk '{print $1}' || true)"
  fi
  if [ -n "$host_ip" ]; then
    export HOST_IP="$host_ip"
    log "Host IP for WebRTC NAT: $host_ip"
  else
    log "Warning: could not detect host IP; WebRTC NAT IPs will be inferred from request"
  fi
}

# Convert 127.0.0.1/localhost to Docker bridge IP so containers can reach host proxy
setup_proxy() {
  # Use MEMOH_PULL_PROXY if explicitly set; otherwise fall back to system proxy env vars
  local raw_proxy="${MEMOH_PULL_PROXY:-${https_proxy:-${http_proxy:-http://127.0.0.1:20171}}}"
  local bridge_ip
  bridge_ip="$(docker network inspect bridge --format '{{range .IPAM.Config}}{{.Gateway}}{{end}}' 2>/dev/null || echo "172.17.0.1")"
  local container_proxy
  container_proxy="$(echo "$raw_proxy" | sed "s|127\.0\.0\.1|${bridge_ip}|g; s|localhost|${bridge_ip}|g")"
  export CONTAINER_HTTP_PROXY="$container_proxy"
  log "Pull proxy: $raw_proxy → $container_proxy (for containerd image pulls only)"
}

ACTION="start"

while [[ $# -gt 0 ]]; do
  case "$1" in
    -s) ACTION="stop";    shift ;;
    -r) ACTION="restart"; shift ;;
    -h) usage; exit 0 ;;
    *)  echo "Unknown option: $1"; usage; exit 1 ;;
  esac
done

cd "$ROOT_DIR"

PROFILES="--profile qdrant --profile sparse --profile browser"

case "$ACTION" in
  start)
    setup_proxy
    setup_host_ip
    log "Starting services..."
    docker compose $PROFILES up -d
    log "Services started. Server: http://localhost:8080  Web: http://localhost:8082  Browser: http://localhost:8083"
    ;;
  stop)
    log "Stopping services..."
    docker compose $PROFILES down
    log "Services stopped."
    ;;
  restart)
    setup_proxy
    setup_host_ip
    log "Recreating server and web with latest images..."
    docker compose up -d --force-recreate --no-deps server web
    log "Services restarted. Server: http://localhost:8080  Web: http://localhost:8082"
    ;;
esac
