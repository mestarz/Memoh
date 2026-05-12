#!/usr/bin/env bash
# Memoh environment check.
#
# Verifies the host has everything required to build & run a Memoh deployment.
# Prints a green ✓ for each satisfied requirement and a red ✗ for missing ones,
# then exits 0 if all hard requirements are met, 1 otherwise.
#
# Usage:
#   scripts/check.sh           # check everything
#   scripts/check.sh --quiet   # only print failures
set -euo pipefail

QUIET=false
[ "${1:-}" = "--quiet" ] && QUIET=true

ok=0; fail=0; warn=0
G=$'\033[1;32m'; R=$'\033[1;31m'; Y=$'\033[1;33m'; N=$'\033[0m'

pass() { ok=$((ok+1));   $QUIET || printf '  %s✓%s %s\n' "$G" "$N" "$*"; }
miss() { fail=$((fail+1));        printf '  %s✗%s %s\n' "$R" "$N" "$*"; }
note() { warn=$((warn+1));        printf '  %s!%s %s\n' "$Y" "$N" "$*"; }
sect() { printf '\n%s== %s ==%s\n' "$G" "$*" "$N"; }

# 1. Required binaries -------------------------------------------------------
sect "required binaries"
for bin in go docker git curl tar; do
  if command -v "$bin" >/dev/null 2>&1; then
    pass "$bin   ($($bin --version 2>&1 | head -1))"
  else
    miss "$bin not found in PATH"
  fi
done
# go version >= 1.22
if command -v go >/dev/null 2>&1; then
  goversion=$(go env GOVERSION | sed 's/go//')
  major=$(echo "$goversion" | cut -d. -f1)
  minor=$(echo "$goversion" | cut -d. -f2)
  if [ "$major" -gt 1 ] || { [ "$major" -eq 1 ] && [ "$minor" -ge 22 ]; }; then
    pass "go version >= 1.22 (found $goversion)"
  else
    miss "go version >= 1.22 required (found $goversion)"
  fi
fi

# 2. Optional but recommended ------------------------------------------------
sect "optional / dev tooling"
for bin in mise pnpm node sqlc; do
  if command -v "$bin" >/dev/null 2>&1; then
    pass "$bin   ($($bin --version 2>&1 | head -1))"
  else
    note "$bin missing (only needed for dev workflow / regenerating sqlc/sdk)"
  fi
done

# 3. gst-launch-1.0 (server-side WebRTC encoder) -----------------------------
if command -v gst-launch-1.0 >/dev/null 2>&1; then
  pass "gst-launch-1.0   ($(gst-launch-1.0 --version 2>&1 | head -1))"
  if gst-inspect-1.0 x264enc >/dev/null 2>&1; then
    pass "gstreamer x264enc element present"
  else
    miss "gstreamer x264enc missing — H264 streaming will fail (Arch: pacman -S gst-plugins-ugly; Debian/Ubuntu: apt install gstreamer1.0-plugins-ugly)"
  fi
else
  miss "gst-launch-1.0 not found — remote-desktop WebRTC streaming will fail (apt install gstreamer1.0-tools gstreamer1.0-plugins-{base,good,bad,ugly} / pacman -S gst-plugins-{base,good,bad,ugly})"
fi

# 4. Docker daemon reachable -------------------------------------------------
sect "docker daemon"
if command -v docker >/dev/null 2>&1; then
  if docker info >/dev/null 2>&1; then
    pass "docker daemon reachable"
    if id -nG "$USER" 2>/dev/null | grep -qw docker; then
      pass "user '$USER' is in docker group"
    else
      note "user '$USER' NOT in docker group — you may need 'sudo' for docker commands (sudo usermod -aG docker $USER && newgrp docker)"
    fi
  else
    miss "docker installed but daemon unreachable (try: sudo systemctl start docker)"
  fi
fi

# 5. systemd --user available ------------------------------------------------
sect "systemd --user"
if systemctl --user --version >/dev/null 2>&1 && systemctl --user list-units >/dev/null 2>&1; then
  pass "systemd --user usable"
  # XDG_RUNTIME_DIR present?
  if [ -n "${XDG_RUNTIME_DIR:-}" ] && [ -d "${XDG_RUNTIME_DIR}" ]; then
    pass "XDG_RUNTIME_DIR=$XDG_RUNTIME_DIR"
  else
    note "XDG_RUNTIME_DIR unset or missing — systemd --user may be flaky on console logins (loginctl enable-linger \$USER fixes this for headless servers)"
  fi
  # linger enabled?
  if loginctl show-user "$USER" 2>/dev/null | grep -q '^Linger=yes'; then
    pass "user-linger enabled (services survive logout)"
  else
    note "user-linger disabled — services stop on logout (loginctl enable-linger $USER to fix)"
  fi
else
  miss "systemd --user not usable on this host"
fi

# 6. Config file -------------------------------------------------------------
sect "configuration"
CFG="${MEMOH_CONFIG:-$HOME/.config/Memoh/config.toml}"
if [ -f "$CFG" ]; then
  pass "config file present: $CFG"
else
  note "config file missing: $CFG (install.sh will copy conf/app.example.toml on first install)"
fi

# 7. Data directory ----------------------------------------------------------
DATAROOT="${MEMOH_DATA_ROOT:-$HOME/.config/Memoh/data}"
if [ -d "$DATAROOT" ]; then
  pass "data root exists: $DATAROOT"
else
  note "data root absent: $DATAROOT (install.sh will create it)"
fi

# 8. Build artifacts ---------------------------------------------------------
sect "build artifacts (run scripts/build.sh first)"
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
[ -x "$ROOT/bin/memoh-server" ]      && pass "bin/memoh-server"      || note "bin/memoh-server missing (scripts/build.sh)"
[ -x "$ROOT/data/runtime/bridge" ]   && pass "data/runtime/bridge"   || note "data/runtime/bridge missing (scripts/build.sh)"
[ -d "$ROOT/data/runtime/toolkit" ]  && pass "data/runtime/toolkit/" || note "data/runtime/toolkit missing (scripts/build.sh)"
if command -v docker >/dev/null 2>&1; then
  if docker image inspect memoh/workspace:debian >/dev/null 2>&1; then
    pass "docker image memoh/workspace:debian present"
  else
    note "docker image memoh/workspace:debian missing (scripts/build.sh)"
  fi
fi

# Summary --------------------------------------------------------------------
echo
printf '%s%d ok%s, %s%d warning%s, %s%d failed%s\n' \
  "$G" "$ok" "$N" "$Y" "$warn" "$N" "$R" "$fail" "$N"
[ "$fail" -eq 0 ] || exit 1
