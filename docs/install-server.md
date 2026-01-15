# Server Installation Guide

This guide explains how to install and run `voiyd-server`

<!--toc:start-->
- [Server Installation Guide](#server-installation-guide)
  - [Dependencies](#dependencies)
  - [Installation](#installation)
    - [Automated Install](#automated-install)
      - [Install Specific Version](#install-specific-version)
      - [Install Nightly Build](#install-nightly-build)
      - [Certificate Configuration](#certificate-configuration)
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

`voiyd-server` does not have any external dependencies and should be supported on most platforms. Pre-built binaries are available for widely used operating systems and architectures. Let me know if your OS is missing from the list. To run `voiyd-server` securely, `openssl` or a similar tool is required.

## Installation

`voiyd-server` can be installed either with the script [`setup-server.sh`](/setup-server.sh) - an automated installation script, or manually by downloading binaries/dependencies individually and moving them into place by hand.

### Automated Install

The automated installation script aims to make server provisioning as simple as possible. Allowing you to bootstrap clusters quickly. It provides command line options so you can customize the installation to fit your needs. By default, the script will install the latest tagged version of `voiyd-server`.

```bash
curl -sfL https://raw.githubusercontent.com/amimof/voiyd/master/setup-server.sh | sudo sh 
```

#### Install Specific Version

```bash
curl -sfL https://raw.githubusercontent.com/amimof/voiyd/master/setup-server.sh | \
  sudo sh -s -- --version v0.0.11 
```

#### Install Nightly Build

Install the latest development version from the master branch:

```bash
curl -sfL https://raw.githubusercontent.com/amimof/voiyd/master/setup-server.sh | \
  sudo sh -s -- --nightly 
```

#### Certificate Configuration

```bash
curl -sfL https://raw.githubusercontent.com/amimof/voiyd/master/setup-server.sh | \
  sudo sh -s -- \
    --cert-org "MyCompany" \
    --cert-cn "voiyd.example.com" \
    --cert-days 730 
```

#### Dry Run

Preview what will be installed without making any changes:

```bash
curl -sfL https://raw.githubusercontent.com/amimof/voiyd/master/setup-server.sh | \
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
| --version <version> | Install specific version | `latest` |
| --nightly | Install nightly build | `false` |
| --prefix <path> | Binary installation prefix | `/usr/local` |
| --tls-dir <path> | Certificate directory | `/etc/voiyd/tls` |
| --port <port> | Server port | `5743` |
| --tls-host <host> | HTTPS listen address |  `0.0.0.0` |
| --tcp-tls-host <host> | gRPC TLS listen address |  `0.0.0.0` |
| --metrics-host <host> | Metrics host |  `0.0.0.0` |
| --skip-cert-generation | Don't generate certificate | `false` |
| --cert-days <days> | Certificate validity |  `365` |
| --cert-country <code> | Country code |  `US` |
| --cert-state <state> | State |  `State` |
| --cert-city <city> | City |  `City` |
| --cert-org <org> | Organization |  voiyd |
| --cert-cn <cn> | Common Name | voiyd-server |
| --auto-install-deps | Auto-install dependencies | `true` |
| --no-systemd | Skip systemd service creation | `false` |
| --start | Start service after installation | `true` |
| --dry-run | Preview without making changes | |
| --verbose | Enable debug output | |

### Manual Installation

Users that don't run any of the supported distributions to run an automated install script can still install `voiyd-server` manually. Here's how:

1. Install required [dependencies](#prerequisites)

2. Download the [latest](https://github.com/amimof/voiyd/releases) binary. Binaries are build for most architectures. Choose the one that suits you.

   ```bash
   curl -LO https://github.com/amimof/voiyd/releases/download/v0.0.11/voiyd-server-linux-amd64
   ```

   > Use this [nighly.link](https://nightly.link/amimof/voiyd/workflows/upload.yaml/master?preview) do download latest unstable releases

3. Install binary

   ```bash
   sudo install -m 755 voiyd-server-linux-amd64 /usr/local/bin/voiyd-server
   ```

4. Generate certificates following [this guide](/certs/README.md)

5. Create systemd unit files (optional)

   ```bash
   curl -LO https://raw.githubusercontent.com/amimof/voiyd/refs/heads/master/systemd/voiyd-server.service
   sudo mv voiyd-server.service /etc/systemd/system
   sudo systemctl daemon-reload
   ```

6. Enable & start service (optional)

   ```bash
   sudo systemctl enable --now voiyd-server.service
   ```

### Build From Source

Building from source requires `Go 1.21` or higher, Git and Make. Make sure these are installed on your system before building.

```bash
git clone https://github.com/amimof/voiyd.git
cd voiyd
make server
```

> `make` will build binaries to `bin/`

To build for a specific architecture you may use the `GARCH` environment variable. To build for a specific platform use `GOOS`. Customize the binary file name with `BINARY_NAME`. For example:

```bash
GOOS=windows GOARCH=amd64 BINARY_NAME=voiyd-server-windows-amd64.exe make server
```

Alternatively, use `go install` directly:

```bash
go install github.com/amimof/voiyd/cmd/voiyd-server
```

> Run `make help` for more information

## Post-Installation

### Verify Installation

Check that the binary is installed:

```bash
which voiyd-server
voiyd-server --version
```

### Configure TLS Certificates

For production deployments, ensure proper TLS certificates are configured on the server. See [this guide](/certs/README.md) on how to configure TLS for both server and nodes after installation.

### Manage the Service

Start the service:

```bash
sudo systemctl start voiyd-server
```

Stop the service:

```bash
sudo systemctl stop voiyd-server
```

Check service status:

```bash
sudo systemctl status voiyd-server
```

View logs:

```bash
sudo journalctl -ef -u voiyd-server
```

## Uninstall

To remove voiyd-server:

```bash
# Stop and disable service
sudo systemctl stop voiyd-server
sudo systemctl disable voiyd-server

# Remove binary
sudo rm /usr/local/bin/voiyd-server

# Remove systemd service
sudo rm /etc/systemd/system/voiyd-server.service
sudo systemctl daemon-reload

# Optional: Remove data directory
sudo rm -rf /var/lib/voiyd
```

## Contributing

If you encounter issues with the installation script, please [open an issue](https://github.com/amimof/voiyd/issues) with:

- Your Linux distribution and version
- Output from `--dry-run --verbose`
- Any error messages received
