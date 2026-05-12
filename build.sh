#!/usr/bin/env bash
# Build all artifacts needed for host-mode Memoh:
#   bin/memoh-server   - HTTP/API server (cmd/agent)
#   bin/memoh          - CLI wrapper (cmd/memoh)
#   data/runtime/bridge          - in-container gRPC bridge
#   data/runtime/toolkit/...     - workspace toolkit (node, uv, Xvnc, ...)
#   apps/web/dist                - prebuilt web assets (optional, skip with SKIP_WEB=1)
#
# Docker images are no longer built here — the legacy Dockerfile.server is gone
# and bot workspaces use plain debian:bookworm-slim.
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$ROOT_DIR"

COMMIT_HASH="${COMMIT_HASH:-$(git rev-parse --short HEAD 2>/dev/null || echo unknown)}"
BUILD_TIME="${BUILD_TIME:-$(date -u +"%Y-%m-%dT%H:%M:%SZ")}"
VERSION="${VERSION:-dev}"

SKIP_TOOLKIT="${SKIP_TOOLKIT:-0}"
SKIP_BRIDGE="${SKIP_BRIDGE:-0}"
SKIP_WEB="${SKIP_WEB:-0}"
SKIP_SERVER="${SKIP_SERVER:-0}"
SKIP_CLI="${SKIP_CLI:-0}"

log() { echo "[build] $*"; }

# Make mise tasks available even when invoked from cron / one-shot shells.
export PATH="$HOME/.local/share/mise/shims:$HOME/.local/bin:$PATH"

usage() {
  cat <<EOF
Usage: build.sh [--no-toolkit] [--no-bridge] [--no-web] [--no-server] [--no-cli]

Environment variables:
  VERSION         Override version string (default: dev)
  COMMIT_HASH     Override commit hash (default: git short SHA)
  SKIP_TOOLKIT=1  Skip workspace toolkit (long; needs docker)
  SKIP_BRIDGE=1   Skip bridge binary
  SKIP_WEB=1      Skip web assets build
  SKIP_SERVER=1   Skip memoh-server build
  SKIP_CLI=1      Skip memoh CLI build
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --no-toolkit) SKIP_TOOLKIT=1; shift ;;
    --no-bridge)  SKIP_BRIDGE=1;  shift ;;
    --no-web)     SKIP_WEB=1;     shift ;;
    --no-server)  SKIP_SERVER=1;  shift ;;
    --no-cli)     SKIP_CLI=1;     shift ;;
    -h|--help)    usage; exit 0 ;;
    *) echo "Unknown option: $1"; usage; exit 1 ;;
  esac
done

log "Version    : $VERSION"
log "Commit     : $COMMIT_HASH"
log "Build time : $BUILD_TIME"

mkdir -p bin data/runtime

# ---------------------------------------------------------------------------
# 1. Workspace toolkit (node, uv, Xvnc) — bind-mounted into bot containers.
#    Slow (pulls + extracts apk packages), so allow skipping if already done.
# ---------------------------------------------------------------------------
if [ "$SKIP_TOOLKIT" != "1" ]; then
  if [ -d data/runtime/toolkit/node-glibc ] && [ -d data/runtime/toolkit/uv ]; then
    log "Toolkit already present in data/runtime/toolkit (use --no-toolkit to silence; rm -rf to rebuild)"
  else
    log "Building workspace toolkit → data/runtime/toolkit/ (this can take a few minutes)"
    mise run install-workspace-toolkit
  fi
else
  log "Skipping workspace toolkit"
fi

# ---------------------------------------------------------------------------
# 2. Bridge (runs inside every bot workspace container).
# ---------------------------------------------------------------------------
if [ "$SKIP_BRIDGE" != "1" ]; then
  log "Building bridge → data/runtime/bridge"
  mise run bridge:build
else
  log "Skipping bridge"
fi

# ---------------------------------------------------------------------------
# 3. Web assets (apps/web/dist) — used when serving from a built bundle
#    instead of the vite dev server.
# ---------------------------------------------------------------------------
if [ "$SKIP_WEB" != "1" ]; then
  log "Building web assets → apps/web/dist"
  pnpm --filter @memohai/web build
else
  log "Skipping web build"
fi

# ---------------------------------------------------------------------------
# 4. memoh-server — the HTTP/API server (cmd/agent).
# ---------------------------------------------------------------------------
if [ "$SKIP_SERVER" != "1" ]; then
  log "Building memoh-server → bin/memoh-server"
  go build \
    -ldflags="-X 'github.com/memohai/memoh/internal/version.Version=$VERSION' \
              -X 'github.com/memohai/memoh/internal/version.CommitHash=$COMMIT_HASH' \
              -X 'github.com/memohai/memoh/internal/version.BuildTime=$BUILD_TIME'" \
    -o bin/memoh-server ./cmd/agent
else
  log "Skipping memoh-server"
fi

# ---------------------------------------------------------------------------
# 5. memoh CLI (cmd/memoh) — desktop companion / status / chat.
# ---------------------------------------------------------------------------
if [ "$SKIP_CLI" != "1" ]; then
  log "Building memoh CLI → bin/memoh"
  go build \
    -ldflags="-X 'github.com/memohai/memoh/internal/version.Version=$VERSION' \
              -X 'github.com/memohai/memoh/internal/version.CommitHash=$COMMIT_HASH' \
              -X 'github.com/memohai/memoh/internal/version.BuildTime=$BUILD_TIME'" \
    -o bin/memoh ./cmd/memoh
else
  log "Skipping memoh CLI"
fi

log "Build complete."
log "Outputs:"
ls -lh bin/memoh-server bin/memoh data/runtime/bridge 2>/dev/null | awk '{print "  ", $0}'
