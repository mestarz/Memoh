#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"

VERSION="${VERSION:-dev}"
COMMIT_HASH="${COMMIT_HASH:-$(git -C "$ROOT_DIR" rev-parse --short HEAD 2>/dev/null || echo unknown)}"
BUILD_TIME="${BUILD_TIME:-$(date -u +"%Y-%m-%dT%H:%M:%SZ")}"
IMAGE_TAG="${IMAGE_TAG:-memohai/server:${VERSION}}"
PLATFORM="${PLATFORM:-linux/$(uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/')}"

log() {
  echo "[build-server] $*"
}

usage() {
  cat <<'EOF'
Usage: scripts/build-server-image.sh [OPTIONS]

Build the Memoh server Docker image for the current platform.

Options:
  -t, --tag TAG       Image tag (default: memohai/server:<VERSION>)
  -v, --version VER   Version string injected into binary (default: dev)
  -p, --platform PLT  Target platform (default: auto-detected host platform)
  --no-cache          Disable Docker layer cache
  -h, --help          Show this help message

Environment variables (all overridable):
  IMAGE_TAG      Full image tag (overrides -t)
  VERSION        Version string (overrides -v)
  COMMIT_HASH    Git commit hash (auto-detected)
  BUILD_TIME     Build timestamp (auto-detected)
  PLATFORM       Target platform (overrides -p)

Examples:
  scripts/build-server-image.sh
  scripts/build-server-image.sh -t myrepo/server:1.0.0 -v 1.0.0
  VERSION=1.2.3 IMAGE_TAG=myrepo/server:1.2.3 scripts/build-server-image.sh
EOF
}

NO_CACHE=""

while [[ $# -gt 0 ]]; do
  case "$1" in
    -t|--tag)      IMAGE_TAG="$2";    shift 2 ;;
    -v|--version)  VERSION="$2";      shift 2 ;;
    -p|--platform) PLATFORM="$2";     shift 2 ;;
    --no-cache)    NO_CACHE="--no-cache"; shift ;;
    -h|--help)     usage; exit 0 ;;
    *) echo "Unknown option: $1"; usage; exit 1 ;;
  esac
done

log "Building server image"
log "  Image tag   : ${IMAGE_TAG}"
log "  Version     : ${VERSION}"
log "  Commit hash : ${COMMIT_HASH}"
log "  Build time  : ${BUILD_TIME}"
log "  Platform    : ${PLATFORM}"

DOCKER_BUILDKIT=1 docker build \
  --platform "${PLATFORM}" \
  -f "${ROOT_DIR}/docker/Dockerfile.server" \
  --build-arg VERSION="${VERSION}" \
  --build-arg COMMIT_HASH="${COMMIT_HASH}" \
  --build-arg BUILD_TIME="${BUILD_TIME}" \
  ${NO_CACHE} \
  -t "${IMAGE_TAG}" \
  "${ROOT_DIR}"

log "Done. Image: ${IMAGE_TAG}"
log "To use with docker compose: docker tag ${IMAGE_TAG} memohai/server:latest"
