#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")" && pwd)"

PLATFORM="${PLATFORM:-linux/$(uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/')}"
COMMIT_HASH="${COMMIT_HASH:-$(git -C "$ROOT_DIR" rev-parse --short HEAD 2>/dev/null || echo unknown)}"
BUILD_TIME="${BUILD_TIME:-$(date -u +"%Y-%m-%dT%H:%M:%SZ")}"
VERSION="${VERSION:-dev}"

log() {
  echo "[build] $*"
}

# Convert local proxy (127.0.0.1/localhost) to Docker bridge IP so build containers can reach the host proxy
resolve_proxy() {
  local proxy="$1"
  local bridge_ip
  bridge_ip="$(docker network inspect bridge --format '{{range .IPAM.Config}}{{.Gateway}}{{end}}' 2>/dev/null || echo "172.17.0.1")"
  echo "$proxy" | sed "s|127\.0\.0\.1|${bridge_ip}|g; s|localhost|${bridge_ip}|g"
}

PROXY_ARGS=()
RAW_PROXY="${https_proxy:-${http_proxy:-}}"
if [ -n "$RAW_PROXY" ]; then
  DOCKER_PROXY="$(resolve_proxy "$RAW_PROXY")"
  log "Proxy detected: $RAW_PROXY → $DOCKER_PROXY (for build containers)"
  PROXY_ARGS+=(
    --build-arg "http_proxy=$DOCKER_PROXY"
    --build-arg "https_proxy=$DOCKER_PROXY"
    --build-arg "HTTP_PROXY=$DOCKER_PROXY"
    --build-arg "HTTPS_PROXY=$DOCKER_PROXY"
  )
fi

log "Platform : $PLATFORM"

log "Building server image → memohai/server:local"
DOCKER_BUILDKIT=1 docker build \
  --platform "$PLATFORM" \
  -f "$ROOT_DIR/docker/Dockerfile.server" \
  --build-arg VERSION="$VERSION" \
  --build-arg COMMIT_HASH="$COMMIT_HASH" \
  --build-arg BUILD_TIME="$BUILD_TIME" \
  "${PROXY_ARGS[@]}" \
  -t memohai/server:local \
  "$ROOT_DIR"

log "Building web image → memohai/web:local"
DOCKER_BUILDKIT=1 docker build \
  --platform "$PLATFORM" \
  -f "$ROOT_DIR/docker/Dockerfile.web" \
  --build-arg VERSION="$VERSION" \
  "${PROXY_ARGS[@]}" \
  -t memohai/web:local \
  "$ROOT_DIR"

log "Build complete."
