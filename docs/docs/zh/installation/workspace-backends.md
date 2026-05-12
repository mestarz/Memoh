# Workspace 后端

每个 Memoh 机器人都运行在独立的 Docker workspace 容器中。Workspace 后端通过宿主机的 Docker Engine 创建、启动和停止这些容器。

在 `config.toml` 中配置：

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
# 留空使用 Docker 标准环境发现：DOCKER_HOST、DOCKER_TLS_VERIFY、
# DOCKER_CERT_PATH 或平台默认 socket。
host = ""
```

Docker 后端通过标准 Docker 环境与 Docker Engine 通信。它要求 bind-mount 源路径（`runtime_dir`、`data_root`）指向 server 进程可读写的真实宿主机路径。

## 网络与 overlay

机器人网络分为两层：

- 运行时网络：把 workspace 接入基础 Docker 容器网络。
- Overlay provider（如 Tailscale、NetBird）：为每个机器人附加可选的私有网络。

Overlay provider 设置在 web UI 中按机器人配置，而不是在全局 TOML 文件中。
