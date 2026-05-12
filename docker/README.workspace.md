# Bot workspace base images

Memoh runs every bot inside a long-lived container. The Memoh agent
injects its own runtime (the `bridge` binary plus a self-contained
toolkit) into each workspace via a read-only bind mount under
`/opt/memoh/toolkit`, so a workspace base image only needs to provide
the surrounding OS userland.

## What Memoh injects (do **not** put these in your image)

Mounted at `/opt/memoh/toolkit`:

| Path | What |
| --- | --- |
| `bin/node`, `bin/npm`, `bin/npx` | Node.js 24 (glibc + musl variants auto-selected) |
| `bin/uv`, `bin/uvx` | Astral `uv` Python toolchain |
| `display/bin/Xvnc` | TigerVNC server |
| `display/bin/xkbcomp`, `xsetroot`, `twm`, `xterm` | X.Org companions for the desktop session |
| `bridge` | The Memoh in-container runtime |

The bridge appends `/opt/memoh/toolkit/bin` to `PATH` automatically; the
sample images below also export it in the image so it shows up in
interactive shells.

## What your image **must** provide

Required:

- A POSIX shell (`/bin/sh` / `bash`) and standard userland (`coreutils`,
  `grep`, `sed`, `awk`, `procps` for `ps`, `findutils`)
- `ca-certificates` for HTTPS egress
- A writable `/tmp` and `/run`, with `/tmp/.X11-unix` mode 1777
- glibc (the injected toolkit auto-selects the glibc Node build)

Strongly recommended:

- `tini` or `dumb-init` as PID 1 — bridge is long-lived and forks
  helpers; without an init you will leak zombies
- `git`, `curl`, `wget`, `tar`, `xz-utils`, `unzip`, `jq`
- `tzdata`, `locales` (with `en_US.UTF-8` and `zh_CN.UTF-8`),
  `fontconfig`, CJK fonts (`fonts-noto-cjk` / `font-noto-cjk`) — needed
  for screenshot legibility and Chromium UI rendering

For the workspace remote desktop:

- `xauth`, `xdg-utils`
- Chromium runtime libs: `libnss3`, `libxss1`, `libgbm1`, `libasound2`,
  `libdrm2`, `libatk1.0-0`, `libatk-bridge2.0-0`, `libxcomposite1`,
  `libxdamage1`, `libxrandr2`, `libxkbcommon0`, `libpango-1.0-0`,
  `libpangocairo-1.0-0`, `libcups2`, `libxshmfence1`
- Chromium itself — either install it in the image (`--build-arg
  CHROMIUM=1` in the supplied Dockerfile) or let Memoh's display prepare
  script `apt install` it on first launch
- A window manager: Memoh falls back to the injected `twm` if no
  `xfce4-session` / `xfwm4` is present, so a WM in the image is
  optional

Optional, for native dependency builds:

- `python3`, `build-essential`/`gcc`, `pkg-config`

## Reference Dockerfile

`Dockerfile.workspace` — `debian:bookworm-slim` based, matches Memoh's
default `default_image`. The display prepare-script fallback (`apt
install`) targets this distro, so Chromium can be installed lazily on
first remote-desktop launch.

Build and use:

```bash
docker build -f docker/Dockerfile.workspace -t memoh/workspace:debian .
# Optionally bake Chromium in at build time (skips the lazy install):
# docker build --build-arg CHROMIUM=1 -f docker/Dockerfile.workspace \
#   -t memoh/workspace:debian-chromium .

# In config.toml:
#   [container]
#   default_image = "memoh/workspace:debian"
# Or per-bot via the workspace image preference UI / API.
```
