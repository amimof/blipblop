[🏠 Home](/docs/README.md)

# Node Installation Guide

This guide explains how to install `voiyd-node` on Linux.

<!--toc:start-->
- [Node Installation Guide](#node-installation-guide)
  - [Dependencies](#dependencies)
  - [Installation](#installation)
    - [Automated Install](#automated-install)
      - [Install Specific Version](#install-specific-version)
      - [Install Nightly Build](#install-nightly-build)
      - [Dry Run](#dry-run)
      - [Supported Platforms](#supported-platforms)
      - [Command-Line Options](#command-line-options)
    - [Manual Installation](#manual-installation)
    - [Build From Source](#build-from-source)
  - [Post-Installation](#post-installation)
    - [Verify Installation](#verify-installation)
    - [Configure TLS Certificates](#configure-tls-certificates)
    - [Manage the Service](#manage-the-service)
  - [Uninstall](#uninstall)
  - [Contributing](#contributing)
<!--toc:end-->

## Dependencies

To run `voiyd-node` following dependencies must be installed. This document will guide you through how to setup everything up correctly.

- [containerd](https://containerd.io/) >= 1.6
- [CNI plugins](https://www.cni.dev/plugins/current/)
- Iptables

## Installation

`voiyd-node` can be installed either with the script [`setup-node.sh`](/setup-node.sh) - an automated installation script, or manually by downloading binaries/dependencies individually and moving them into place by hand.

### Automated Install

The automated installation script aims to make node provisioning as simple as possible. Allowing you to bootstrap worker nodes with only one command. It provides command line options so you can customize the installation to fit your needs. By default, the script will install the latest tagged version of `voiyd-node`. Any dependencies that are missing will be installed using the systems package manager.

```bash
curl -sfL https://raw.githubusercontent.com/amimof/voiyd/master/setup-node.sh | \
  sudo sh -s -- --host VOIYD_SERVER_HOST:5743
```

#### Install Specific Version

```bash
curl -sfL https://raw.githubusercontent.com/amimof/voiyd/master/setup-node.sh | \
  sudo sh -s -- --version v0.0.11
```

#### Install Nightly Build

Install the latest development version from the master branch:

```bash
curl -sfL https://raw.githubusercontent.com/amimof/voiyd/master/setup-node.sh | \
  sudo sh -s -- --nightly 
```

#### Dry Run

Preview what will be installed without making any changes:

```bash
curl -sfL https://raw.githubusercontent.com/amimof/voiyd/master/setup-node.sh | \
  sudo sh -s -- --dry-run --verbose
```

#### Supported Platforms

The installation script automatically detects your platform and downloads the appropriate binary as well as installs dependencies. Currently the supported distributions are:

- Ubuntu / Debian
- CentOS / RHEL / Fedora
- Arch Linux

#### Command-Line Options

Following is the complete set of flags that can be passed in to the installation script allowing you to customize the installation.

| Option | Description | Default |
|--------|-------------|---------|
| `--version <version>` | voiyd version to install | `latest` |
| `--nightly` | Install latest nightly build from master | `false` |
| `--cni-version <version>` | CNI plugins version | `v1.9.0` |
| `--prefix <path>` | Installation prefix | `/usr/local` |
| `--cni-bin-dir <path>` | CNI plugins directory | `/opt/cni/bin` |
| `--cni-conf-dir <path>` | CNI config directory | `/etc/cni/net.d` |
| `--server-address <addr>` | Server address and port | `localhost:5743` |
| `--insecure-skip-verify` | Skip TLS certificate verification (dev only) | `true` |
| `--metrics-host <host>` | Metrics host address | `0.0.0.0` |
| `--no-systemd` | Don't install systemd service | `false` |
| `--start` | Start service after installation | `true` |
| `--auto-install-deps` | Auto-install missing dependencies | `true` |
| `--skip-deps` | Skip dependency checks | `false` |
| `--dry-run` | Show what would be installed |  |
| `--verbose` `-v` | Enable verbose output |  |
| `--help` `-h` | Show help message |  |

### Manual Installation

Users that don't run any of the supported distributions to run an automated install script can still install `voiyd-node` manually. Here's how:

1. Install required [dependencies](#prerequisites)

2. Download the [latest](https://github.com/amimof/voiyd/releases) binary. Binaries are build for most architectures. Choose the one that suits you.

   ```bash
   curl -LO https://github.com/amimof/voiyd/releases/download/v0.0.11/voiyd-node-linux-amd64
   ```

   > Use this [nighly.link](https://nightly.link/amimof/voiyd/workflows/upload.yaml/master?preview) do download latest unstable releases

3. Install binary

   ```bash
   sudo install -m 755 voiyd-node-linux-amd64 /usr/local/bin/voiyd-node
   ```

4. Create systemd unit files (optional)

   ```bash
   curl -LO https://raw.githubusercontent.com/amimof/voiyd/refs/heads/master/systemd/voiyd-node.service
   sudo mv voiyd-node.service /etc/systemd/system
   sudo systemctl daemon-reload
   ```

5. Download and install CNI plugins

   ```bash
   curl -LO https://github.com/containernetworking/plugins/releases/download/v1.9.0/cni-plugins-linux-amd64-v1.9.0.tgz
   mkdir -p /opt/cni/bin
   tar xvfz cni-plugins-linux-arm64-v1.9.0.tgz -C /opt/cni/bin/
   ```

6. Enable & start services (optional)

   ```bash
   sudo systemctl enable --now voiyd-node.service
   ```

### Build From Source

Building from source requires `Go 1.21` or higher, Git and Make. Make sure these are installed on your system before building.

```bash
git clone https://github.com/amimof/voiyd.git
cd voiyd
make node
```

> `make` will build binaries to `bin/`

To build for a specific architecture you may use the `GARCH` environment variable. Customize the binary file name with `BINARY_NAME`. For example:

```bash
GOARCH=amd64 BINARY_NAME=voiyd-node-linux-amd64 make node
```

Alternatively, use `go install` directly:

```bash
go install github.com/amimof/voiyd/cmd/voiyd-node
```

> Run `make help` for more information

## Post-Installation

### Verify Installation

Check that the binary is installed:

```bash
which voiyd-node
voiyd-node --version
```

### Configure TLS Certificates

For production deployments, ensure proper TLS certificates are configured on the server. See [this guide](/docs/tls.md) on how to configure TLS for both server and nodes after installation.

### Manage the Service

Start the service:

```bash
sudo systemctl start voiyd-node
```

Stop the service:

```bash
sudo systemctl stop voiyd-node
```

Check service status:

```bash
sudo systemctl status voiyd-node
```

View logs:

```bash
sudo journalctl -ef -u voiyd-node
```

## Uninstall

To remove voiyd-node:

```bash
# Stop and disable service
sudo systemctl stop voiyd-node
sudo systemctl disable voiyd-node

# Remove binary
sudo rm /usr/local/bin/voiyd-node

# Remove systemd service
sudo rm /etc/systemd/system/voiyd-node.service
sudo systemctl daemon-reload

# Optional: Remove CNI plugins
sudo rm -rf /opt/cni/bin/*

# Optional: Remove configuration
sudo rm -rf /etc/voiyd

# Optional: Remove data directory
sudo rm -rf /var/lib/voiyd
```

## Contributing

If you encounter issues with the installation script, please [open an issue](https://github.com/amimof/voiyd/issues) with:

- Your Linux distribution and version
- Output from `--dry-run --verbose`
- Any error messages received
