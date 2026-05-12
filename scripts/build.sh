#!/usr/bin/env bash
# Memoh single-source-of-truth build script.
#
# Builds (in order):
#   1. workspace bridge binary       → data/runtime/bridge (+ templates)
#   2. workspace toolkit (idempotent)→ data/runtime/toolkit/
#   3. memoh-server binary           → bin/memoh-server
#   4. workspace container image     → memoh/workspace:debian (Chromium baked in)
#   5. (optional) memoh CLI          → bin/memoh
#
# Usage:
#   scripts/build.sh                       # all of (1..4)
#   scripts/build.sh --with-cli            # also build bin/memoh
#   scripts/build.sh --no-image            # skip docker image
#   scripts/build.sh --no-server           # skip server binary
#   scripts/build.sh --no-bridge           # skip bridge (and toolkit)
#   scripts/build.sh --image-tag <tag>     # custom image tag (default memoh/workspace:debian)
#   scripts/build.sh --proxy <http_proxy>  # forward proxy to docker build (e.g. http://127.0.0.1:20171)
#   scripts/build.sh --no-chromium         # build slim image without Chromium
#
# Environment overrides:
#   VERSION, COMMIT_HASH, BUILD_TIME — injected into version ldflags.
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT_DIR"

WITH_CLI=false
DO_BRIDGE=true
DO_SERVER=true
DO_IMAGE=true
WITH_CHROMIUM=true
IMAGE_TAG="memoh/workspace:debian"
PROXY="${HTTP_PROXY:-${http_proxy:-}}"

while [ $# -gt 0 ]; do
  case "$1" in
    --with-cli)     WITH_CLI=true ;;
    --no-bridge)    DO_BRIDGE=false ;;
    --no-server)    DO_SERVER=false ;;
    --no-image)     DO_IMAGE=false ;;
    --no-chromium)  WITH_CHROMIUM=false ;;
    --image-tag)    IMAGE_TAG="$2"; shift ;;
    --proxy)        PROXY="$2"; shift ;;
    -h|--help)      sed -n '2,21p' "$0"; exit 0 ;;
    *) echo "[build] unknown option: $1" >&2; exit 2 ;;
  esac
  shift
done

VERSION="${VERSION:-$(git describe --tags --always --dirty 2>/dev/null || echo dev)}"
COMMIT_HASH="${COMMIT_HASH:-$(git rev-parse --short HEAD 2>/dev/null || echo unknown)}"
BUILD_TIME="${BUILD_TIME:-$(date -u +%Y-%m-%dT%H:%M:%SZ)}"
LDFLAGS="-X github.com/mestarz/Memoh/internal/version.Version=${VERSION} \
         -X github.com/mestarz/Memoh/internal/version.Commit=${COMMIT_HASH} \
         -X github.com/mestarz/Memoh/internal/version.BuildTime=${BUILD_TIME}"

log()  { printf '\033[1;34m[build]\033[0m %s\n' "$*"; }
step() { printf '\n\033[1;32m[build] ▶ %s\033[0m\n' "$*"; }

# 1. bridge + toolkit ---------------------------------------------------------
if $DO_BRIDGE; then
  step "(1/4) building workspace bridge → data/runtime/bridge"
  mkdir -p data/runtime
  go build -ldflags "$LDFLAGS" -o data/runtime/bridge ./cmd/bridge
  rm -rf data/runtime/templates
  cp -a cmd/bridge/template data/runtime/templates
  log "bridge ready: $(data/runtime/bridge --help 2>&1 | head -1 || true)"

  if [ ! -d data/runtime/toolkit/uv ]; then
    step "(1b) installing workspace toolkit → data/runtime/toolkit/"
    ./docker/toolkit/install.sh data/runtime/toolkit
  else
    log "toolkit already present, skipping (delete data/runtime/toolkit to force reinstall)"
  fi
fi

# 2. server binary ------------------------------------------------------------
if $DO_SERVER; then
  step "(2/4) building memoh-server → bin/memoh-server"
  mkdir -p bin
  go build -ldflags "$LDFLAGS" -o bin/memoh-server ./cmd/memoh-server
  log "server ready: $(./bin/memoh-server version 2>&1 | head -1 || true)"
fi

# 3. workspace docker image ---------------------------------------------------
if $DO_IMAGE; then
  step "(3/4) building workspace image → ${IMAGE_TAG}"
  if ! command -v docker >/dev/null 2>&1; then
    log "ERROR: docker not installed; install docker or pass --no-image"
    exit 1
  fi
  build_args=()
  if [ -n "$PROXY" ]; then
    log "using proxy: $PROXY"
    build_args+=(--build-arg "http_proxy=$PROXY" --build-arg "https_proxy=$PROXY")
  fi
  if $WITH_CHROMIUM; then
    build_args+=(--build-arg CHROMIUM=1)
  else
    build_args+=(--build-arg CHROMIUM=0)
  fi
  docker build \
    --network=host \
    "${build_args[@]}" \
    -f docker/Dockerfile.workspace \
    -t "$IMAGE_TAG" \
    .
  size=$(docker images "$IMAGE_TAG" --format '{{.Size}}' | head -1)
  log "image ready: $IMAGE_TAG ($size)"
fi

# 4. (optional) CLI -----------------------------------------------------------
if $WITH_CLI; then
  step "(4/4) building memoh CLI → bin/memoh"
  # CLI embeds the web assets; build them first.
  ./scripts/release.sh --prepare-assets
  go build -ldflags "$LDFLAGS" -o bin/memoh ./cmd/memoh
  log "CLI ready: $(./bin/memoh version 2>&1 | head -1 || true)"
fi

echo
log "build complete:"
[ -f bin/memoh-server ]      && echo "  bin/memoh-server      ($(du -h bin/memoh-server      | cut -f1))"
[ -f bin/memoh ]             && echo "  bin/memoh             ($(du -h bin/memoh             | cut -f1))"
[ -f data/runtime/bridge ]   && echo "  data/runtime/bridge   ($(du -h data/runtime/bridge   | cut -f1))"
$DO_IMAGE && docker images "$IMAGE_TAG" --format '  docker image          {{.Repository}}:{{.Tag}} ({{.Size}})'
echo
log "next: ./scripts/check.sh && ./scripts/install.sh --enable"
