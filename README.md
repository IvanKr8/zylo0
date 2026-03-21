
# Zylo

Zylo is a lightweight container runtime built from scratch in Go. It provides full container isolation using Linux namespaces, overlayfs, and includes a built-in DNS server with external forwarding. Designed for developers and system engineers who want to understand container internals or need a minimal, self-contained container solution without Docker's complexity.


## How to install
```bash
curl -L https://github.com/IvanKr8/zylo0/releases/download/1.0/zylo-1.0-linux-amd64.tar.gz | tar xz && sudo ./scripts/install.sh
```

System Requirements

    OS: Linux (x86_64 only)

    Kernel: 3.10+ (for overlayfs support)

    Root privileges: Required for daemon

    Disk space: Minimum 1024 MB for base images

    RAM: 256 MB minimum (512 MB recommended for multiple containers)

Dependencies (usually preinstalled)

    bash
    iptables
    iproute2
    nsenter
    overlayfs
## Features

- **Full container isolation** — Linux namespaces (mount, pid, net, ipc, uts)
- **OverlayFS** — efficient image layering with copy-on-write
- **Built-in DNS** — DNS server per network with external forwarding (8.8.8.8)
- **Network management** — bridge networks, IPAM, veth pairs, port forwarding
- **Volume management** — persistent storage with usage tracking
- **Container exec** — run commands inside running containers
- **Hosts file sync** — automatic /etc/hosts updates for container discovery
- **ZyloFile** — simple declarative configuration (like Dockerfile)
- **CLI + Daemon** — clean client-server architecture over Unix socket
## How to use

First, start the daemon in a separate terminal (requires root):

```bash
sudo zylod up
```

Create a ZyloFile in your project root and define your container configuration

Start a container:

```bash
zylo up
```

Stop and remove a container:

```bash
zylo down
```

List running containers:
```bash
zylo ps
```
# Zylo CLI Reference

## zylo commands

| Command        | Description                                   |
|----------------|-----------------------------------------------|
| zylo up        | Creates and starts a new isolated container   |
| zylo down      | Stops and removes a container                 |
| zylo ps        | Lists all running containers                  |
| zylo logs      | Shows container logs                          |
| zylo volume    | Volume management                            |
| zylo image     | Image management                             |

---

## zylo up

Creates a new container from a ZyloFile in the current directory.


### ZyloFile Options

| Option        | Description                                      | Example                 |
|---------------|--------------------------------------------------|-------------------------|
| USE_IMAGE     | Base image name                                  | postgres                |
| NAME          | Container name                                   | my_app                  |
| SET_WORKDIR   | Working directory inside container               | /app                    |
| COPY          | Copy local directory into container              | .                       |
| NETWORK       | Custom network name                              | my_net                  |
| VOLUME        | Volume mount (host:container or named volume)    | /data:/var/lib/data     |
| SET_ENV       | Environment variable                             | ENV:my_env              |
| OPEN_PORT     | Port mapping host:container                      | 8080:8080               |
| START_WITH    | Override container start command                 | ["python", "app.py"]    |

### Example ZyloFile

```yaml
USE_IMAGE python
NAME my_app
SET_WORKDIR /app
COPY .
OPEN_PORT 5000:5000
SET_ENV ENV:my_env
START_WITH ["python", "app.py"]
```

### zylo down

Stops and removes a container.

| Flag | Description | Example |
|------|-------------|---------|
| `-hash` | Remove container by hash | `1774124071-4` |
| `-name` | Remove container by name | `my_app` |

**Example:**
```bash
zylo down -hash=1774124071-4
zylo down -name=my_app
```

### zylo logs

Shows container logs.

| Flag | Description | Example |
|------|-------------|---------|
| `-hash` | Show logs by container hash | `1774124071-4` |
| `-name` | Show logs by container name | `my_app` |
| `-type` | Log type: `output` (stdout/stderr) or `daemon` (full logs) | `daemon` |

**Example:**
```bash
zylo logs -name=my_app
zylo logs -hash=1774124071-4 -type daemon
```

### zylo volume

Volume management.

**Commands:**

| Command | Description |
|---------|-------------|
| `list` | List all registered volumes |
| `delete` | Delete a volume |

**Volume delete flags:**

| Flag | Description | Example |
|------|-------------|---------|
| `-name` | Delete by volume name | `postgresql_volume4` |
| `-path` | Delete by volume path | `/var/lib/zylo/volumes/postgresql_volume4` |

**Example:**
```bash
zylo volume list
zylo volume delete -name=postgresql_volume4
```

### zylo image

Image management.

**Commands:**

| Command | Description |
|---------|-------------|
| `list` | List all downloaded images |
| `pull` | Download an image from registry |
| `delete` | Delete an image from host |

**Image pull flags:**

| Flag | Description | Example |
|------|-------------|---------|
| `-name` | Image name to download | `python` |

**Image delete flags:**

| Flag | Description | Example |
|------|-------------|---------|
| `-name` | Image name to delete | `python` |

**Example:**
```bash
zylo image list
zylo image pull -name=python
zylo image delete -name=python
```

### zylod commands (daemon)

| Command | Description |
|---------|-------------|
| `sudo zylod up` | Start the daemon in background |
| `sudo zylod down` | Stop the daemon |
| `sudo zylod status` | Check daemon status (OK / FAIL) |

**Example:**
```bash
sudo zylod up
sudo zylod status
sudo zylod down
```
## Available Images

Golang:

```bash
zylo image pull --name=golang
```

Python:

```bash
zylo image pull --name=python
```

Postgres:

```bash
zylo image pull --name=postgres
```