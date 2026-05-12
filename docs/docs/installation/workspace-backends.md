# Workspace backend

Each Memoh bot runs in an isolated Docker workspace container. The workspace backend talks to the host Docker Engine to create, start, and stop those containers.

Configure it in `config.toml`:

```toml
[container]
backend = "docker"
default_image = "debian:bookworm-slim"
image_pull_policy = "if_not_present"
data_root = "/opt/memoh/data"
runtime_dir = "/opt/memoh/runtime"
cni_bin_dir = "/opt/cni/bin"
cni_conf_dir = "/etc/cni/net.d"

[docker]
# Empty means Docker's standard environment discovery: DOCKER_HOST,
# DOCKER_TLS_VERIFY, DOCKER_CERT_PATH, or the platform default socket.
host = ""
```

The Docker backend talks to Docker Engine through the standard Docker environment. It expects the bind-mount source paths (`runtime_dir`, `data_root`) to refer to real host paths that the server process can read and write.

## Networking and overlays

Bot networking has two layers:

- Runtime networking connects the workspace to the base Docker container network.
- Overlay providers such as Tailscale and NetBird attach optional per-bot private networking.

Overlay provider settings are configured per bot in the web UI, not in the global TOML file.
