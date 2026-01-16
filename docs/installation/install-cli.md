[🏠 Home](/docs/README.md)

# CLI Installation Guide

This guide explains how to install `voiydctl`

<!--toc:start-->
- [CLI Installation Guide](#cli-installation-guide)
  - [Installation](#installation)
    - [MacOS](#macos)
    - [Manual Installation](#manual-installation)
    - [Build From Source](#build-from-source)
    - [Configure TLS Certificates](#configure-tls-certificates)
  - [Contributing](#contributing)
<!--toc:end-->

## Installation

### MacOS

```bash
brew tap amimof/voiyd
brew install voiydctl
```

Nightly

```
brew install voiyd --nightly
```

### Manual Installation

1. Download the latest binary from [Releases](<https://github.com/amimof/voiyd/releases>.

   ```bash
   curl -LO https://github.com/amimof/voiyd/releases/download/v0.0.11/voiydctl-linux-amd64
   ```

   > Use this [nighly.link](https://nightly.link/amimof/voiyd/workflows/upload.yaml/master?preview) do download latest unstable releases

2. Install binary

   ```bash
   sudo install -m 755 voiydctl-linux-amd64 /usr/local/bin/voiydctl
   ```

3. Verify

   ```bash
   voiydctl --version
   ```

### Build From Source

Building from source requires `Go 1.21` or higher, Git and Make. Make sure these are installed on your system before building.

```bash
git clone https://github.com/amimof/voiyd.git
cd voiyd
make voiydctl
```

> `make` will build binaries to `bin/`

To build for a specific architecture you may use the `GARCH` environment variable. To build for a specific platform use `GOOS`. Customize the binary file name with `BINARY_NAME`. For example:

```bash
GOOS=windows GOARCH=amd64 BINARY_NAME=voiyd.exe make voiydctl
```

> Run `make help` for more information

## Post-Installation

### Initialize Configuration

Once `voiydctl` is installed, a client configuration must be initialized and servers added to it. See the [usage guide](/docs/README.md) for more information.

### Configure TLS Certificates

For production deployments, ensure proper TLS certificates are configured on the server. See [this guide](/docs/configuration/tls.md) for more infrmation.

## Contributing

If you encounter issues with the installation script, please [open an issue](https://github.com/amimof/voiyd/issues) with:

- Your Linux distribution and version
- Output from `--dry-run --verbose`
- Any error messages received
