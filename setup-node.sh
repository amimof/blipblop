#!/usr/bin/env bash

# Voiyd Node Installation Script
# This script installs voiyd-node and its dependencies on Linux systems
# Usage: curl -sfL https://raw.githubusercontent.com/amimof/voiyd/master/setup-node.sh | sh -
# Or: curl -sfL https://raw.githubusercontent.com/amimof/voiyd/master/setup-node.sh | sh -s -- [options]

set -e
set -o pipefail

# ============================================================================
# Configuration Variables
# ============================================================================

# Default values
VOIYD_VERSION="${VOIYD_VERSION:-latest}"
CNI_VERSION="${CNI_VERSION:-v1.9.0}"
USE_NIGHTLY="${USE_NIGHTLY:-false}"
PREFIX="${PREFIX:-/usr/local}"
CNI_BIN_DIR="${CNI_BIN_DIR:-/opt/cni/bin}"
CNI_CONF_DIR="${CNI_CONF_DIR:-/etc/cni/net.d}"
SYSTEMD_DIR="${SYSTEMD_DIR:-/etc/systemd/system}"
VOIYD_CONFIG_DIR="${VOIYD_CONFIG_DIR:-/etc/voiyd}"
VOIYD_TLS_DIR="${VOIYD_TLS_DIR:-/etc/voiyd/tls}"
VOIYD_DATA_DIR="${VOIYD_DATA_DIR:-/var/lib/voiyd}"

# Server configuration
SERVER_ADDRESS="${SERVER_ADDRESS:-localhost:5743}"
INSECURE_SKIP_VERIFY="${INSECURE_SKIP_VERIFY:-true}"
METRICS_HOST="${METRICS_HOST:-0.0.0.0}"

# Installation options
INSTALL_SYSTEMD="${INSTALL_SYSTEMD:-true}"
START_SERVICE="${START_SERVICE:-true}"
AUTO_INSTALL_DEPS="${AUTO_INSTALL_DEPS:-true}"
SKIP_DEPS="${SKIP_DEPS:-false}"
DRY_RUN="${DRY_RUN:-false}"
VERBOSE="${VERBOSE:-false}"

# GitHub repository
GITHUB_REPO="amimof/voiyd"
CNI_REPO="containernetworking/plugins"
GITHUB_API_URL="https://api.github.com/repos"
GITHUB_DOWNLOAD_URL="https://github.com"
# Nightly builds: single zip containing all architectures (amd64, arm64, arm)
NIGHTLY_LINK_URL="https://nightly.link/amimof/voiyd/workflows/upload.yaml/master/voiyd-node-linux-master.zip"

# Temporary directory for downloads
TMP_DIR="$(mktemp -d -t voiyd-install.XXXXXXXXXX)"
trap cleanup EXIT

# Color codes for output
if [ -t 1 ]; then
  RED='\033[0;31m'
  GREEN='\033[0;32m'
  YELLOW='\033[1;33m'
  BLUE='\033[0;34m'
  BOLD='\033[1m'
  NC='\033[0m' # No Color
else
  RED=''
  GREEN=''
  YELLOW=''
  BLUE=''
  BOLD=''
  NC=''
fi

# ============================================================================
# Helper Functions
# ============================================================================

log() {
  echo -e "${GREEN}[INFO]${NC} $*"
}

log_verbose() {
  if [ "$VERBOSE" = "true" ]; then
    echo -e "${BLUE}[DEBUG]${NC} $*"
  fi
}

warn() {
  echo -e "${YELLOW}[WARN]${NC} $*" >&2
}

error() {
  echo -e "${RED}[ERROR]${NC} $*" >&2
}

fatal() {
  error "$*"
  exit 1
}

success() {
  echo -e "${GREEN}[SUCCESS]${NC} $*"
}

cleanup() {
  if [ -d "$TMP_DIR" ]; then
    log_verbose "Cleaning up temporary directory: $TMP_DIR"
    rm -rf "$TMP_DIR"
  fi
}

check_root() {
  if [ "$(id -u)" -ne 0 ]; then
    fatal "This script must be run as root or with sudo"
  fi
}

command_exists() {
  command -v "$1" >/dev/null 2>&1
}

# ============================================================================
# Platform Detection
# ============================================================================

detect_platform() {
  local os arch

  os="$(uname -s | tr '[:upper:]' '[:lower:]')"
  if [ "$os" != "linux" ]; then
    fatal "Unsupported operating system: $os. voiyd-node requires Linux."
  fi

  arch="$(uname -m)"
  case "$arch" in
  x86_64 | amd64)
    arch="amd64"
    ;;
  aarch64 | arm64)
    arch="arm64"
    ;;
  armv7l | armhf)
    arch="arm"
    ;;
  *)
    fatal "Unsupported architecture: $arch"
    ;;
  esac

  echo "$arch"
}

detect_distro() {
  if [ -f /etc/os-release ]; then
    # shellcheck disable=SC1091
    . /etc/os-release
    echo "$ID"
  elif [ -f /etc/redhat-release ]; then
    echo "rhel"
  elif [ -f /etc/debian_version ]; then
    echo "debian"
  else
    echo "unknown"
  fi
}

# ============================================================================
# Version Resolution
# ============================================================================

get_latest_release() {
  local repo="$1"
  local url="${GITHUB_API_URL}/${repo}/releases/latest"

  log_verbose "Fetching latest release from: $url"
  set +e
  if command_exists curl; then
    curl -sL "$url" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/'
  elif command_exists wget; then
    wget -qO- "$url" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/'
  else
    fatal "Neither curl nor wget is available. Please install one of them."
  fi
  set -e
}

resolve_version() {
  if [ "$USE_NIGHTLY" = "true" ]; then
    log "Using nightly build from master branch"
    VOIYD_VERSION="nightly"
  elif [ "$VOIYD_VERSION" = "latest" ]; then
    log "Resolving latest voiyd version..."
    VOIYD_VERSION=$(get_latest_release "$GITHUB_REPO")
    if [ -z "$VOIYD_VERSION" ]; then
      fatal "Failed to fetch latest version"
    fi
    log "Latest version: $VOIYD_VERSION"
  fi
}

# ============================================================================
# Dependency Management
# ============================================================================

check_unzip() {
  log "Checking for unzip..."
  if command_exists unzip; then
    local version
    version=$(unzip -v 2>&1 | head -n1)
    log "Found: $version"
    return 0
  else
    warn "unzip not found"
    return 1
  fi
}

check_iptables() {
  log "Checking for iptables..."
  if command_exists iptables; then
    local version
    version=$(iptables --version 2>&1 | head -n1)
    log "Found: $version"
    return 0
  else
    warn "iptables not found"
    return 1
  fi
}

check_containerd() {
  log "Checking for containerd..."
  if command_exists containerd; then
    local version
    version=$(containerd --version 2>&1 | head -n1)
    log "Found: $version"

    # Extract version number and check if >= 1.6
    local ver_num
    ver_num=$(echo "$version" | grep -oP 'v?\K[0-9]+\.[0-9]+' | head -n1)
    if [ -n "$ver_num" ]; then
      local major minor
      major=$(echo "$ver_num" | cut -d. -f1)
      minor=$(echo "$ver_num" | cut -d. -f2)

      if [ "$major" -gt 1 ] || ([ "$major" -eq 1 ] && [ "$minor" -ge 6 ]); then
        return 0
      else
        warn "containerd version $ver_num is less than 1.6"
        return 1
      fi
    fi
    return 0
  else
    warn "containerd not found"
    return 1
  fi
}

install_unzip() {
  local distro
  distro=$(detect_distro)

  log "Installing unzip for $distro..."

  case "$distro" in
  ubuntu | debian)
    apt-get update -qq
    apt-get install -y unzip
    ;;
  centos | rhel | fedora)
    if command_exists dnf; then
      dnf install -y unzip
    else
      yum install -y unzip
    fi
    ;;
  arch)
    pacman -S --noconfirm unzip
    ;;
  *)
    warn "Unknown distro: $distro. Please install unzip manually."
    return 1
    ;;
  esac
}

install_iptables() {
  local distro
  distro=$(detect_distro)

  log "Installing iptables for $distro..."

  case "$distro" in
  ubuntu | debian)
    apt-get update -qq
    apt-get install -y iptables
    ;;
  centos | rhel | fedora)
    if command_exists dnf; then
      dnf install -y iptables
    else
      yum install -y iptables
    fi
    ;;
  arch)
    pacman -S --noconfirm iptables
    ;;
  *)
    warn "Unknown distro: $distro. Please install iptables manually."
    return 1
    ;;
  esac
}

install_containerd() {
  local distro
  distro=$(detect_distro)

  log "Installing containerd for $distro..."

  case "$distro" in
  ubuntu | debian)
    apt-get update -qq
    apt-get install -y containerd
    systemctl enable containerd
    systemctl start containerd
    ;;
  centos | rhel | fedora)
    if command_exists dnf; then
      dnf install -y containerd
    else
      yum install -y containerd
    fi
    systemctl enable containerd
    systemctl start containerd
    ;;
  arch)
    pacman -S --noconfirm containerd
    systemctl enable containerd
    systemctl start containerd
    ;;
  *)
    warn "Unknown distro: $distro. Please install containerd manually."
    warn "See: https://containerd.io/downloads/"
    return 1
    ;;
  esac
}

check_and_install_dependencies() {
  if [ "$SKIP_DEPS" = "true" ]; then
    log "Skipping dependency checks (--skip-deps)"
    return 0
  fi

  log "Checking dependencies..."

  local missing_deps=()

  # Check unzip (required for nightly builds)
  if [ "$USE_NIGHTLY" = "true" ]; then
    if ! check_unzip; then
      missing_deps+=("unzip")
    fi
  fi

  # Check iptables
  if ! check_iptables; then
    missing_deps+=("iptables")
  fi

  # Check containerd
  if ! check_containerd; then
    missing_deps+=("containerd")
  fi

  if [ ${#missing_deps[@]} -eq 0 ]; then
    log "All dependencies are satisfied"
    return 0
  fi

  # Handle missing dependencies
  if [ "$AUTO_INSTALL_DEPS" = "true" ]; then
    log "Installing missing dependencies: ${missing_deps[*]}"
    for dep in "${missing_deps[@]}"; do
      case "$dep" in
      unzip)
        install_unzip || fatal "Failed to install unzip"
        ;;
      iptables)
        install_iptables || fatal "Failed to install iptables"
        ;;
      containerd)
        install_containerd || fatal "Failed to install containerd"
        ;;
      esac
    done
  else
    error "Missing dependencies: ${missing_deps[*]}"
    error "Please install them manually or run with --auto-install-deps"
    fatal "Installation cannot continue without dependencies"
  fi
}

# ============================================================================
# Download Functions
# ============================================================================

download_file() {
  local url="$1"
  local dest="$2"

  log_verbose "Downloading: $url"
  log_verbose "Destination: $dest"

  if [ "$DRY_RUN" = "true" ]; then
    log "[DRY RUN] Would download: $url"
    return 0
  fi

  if command_exists curl; then
    curl -fsSL -o "$dest" "$url" || fatal "Download failed: $url"
  elif command_exists wget; then
    wget -q -O "$dest" "$url" || fatal "Download failed: $url"
  else
    fatal "Neither curl nor wget is available"
  fi
}

verify_checksum() {
  local file="$1"
  local checksum_url="$2"

  if [ -z "$checksum_url" ]; then
    log_verbose "No checksum URL provided, skipping verification"
    return 0
  fi

  log "Verifying checksum..."

  local checksum_file="${TMP_DIR}/checksum.txt"
  download_file "$checksum_url" "$checksum_file"

  if command_exists sha256sum; then
    (cd "$(dirname "$file")" && sha256sum -c "$checksum_file") || fatal "Checksum verification failed"
  else
    warn "sha256sum not available, skipping checksum verification"
  fi
}

# ============================================================================
# Installation Functions
# ============================================================================

install_voiyd_node() {
  local arch
  arch=$(detect_platform)

  local binary_name="voiyd-node-linux-${arch}"
  local download_url
  local dest_path="${PREFIX}/bin/voiyd-node"

  if [ "$USE_NIGHTLY" = "true" ]; then
    log "Installing voiyd-node nightly build for linux-${arch}..."

    # Nightly.link provides a single zip file containing all architectures
    local nightly_zip="${TMP_DIR}/voiyd-node-nightly.zip"
    download_url="${NIGHTLY_LINK_URL}"

    # Create bin directory if it doesn't exist
    if [ "$DRY_RUN" = "false" ]; then
      mkdir -p "${PREFIX}/bin"
    fi

    log_verbose "Downloading nightly build from: $download_url"
    download_file "$download_url" "$nightly_zip"

    if [ "$DRY_RUN" = "true" ]; then
      log "[DRY RUN] Would extract and install binary for ${arch} to: $dest_path"
      return 0
    fi

    # Extract the zip file
    log "Extracting nightly build..."
    if command_exists unzip; then
      unzip -q -o "$nightly_zip" -d "${TMP_DIR}/nightly" || fatal "Failed to extract nightly build"
    else
      error "unzip is required to extract nightly builds but it's not installed."
      error "This should have been detected earlier. Please install unzip manually:"
      error "  Ubuntu/Debian: apt-get install unzip"
      error "  RHEL/CentOS/Fedora: dnf install unzip (or yum install unzip)"
      error "  Arch: pacman -S unzip"
      fatal "Cannot continue without unzip"
    fi

    # Find the binary for the detected architecture
    log "Looking for ${binary_name} in extracted files..."
    local tmp_binary
    tmp_binary=$(find "${TMP_DIR}/nightly" -name "$binary_name" -type f | head -n1)

    if [ -z "$tmp_binary" ] || [ ! -f "$tmp_binary" ]; then
      error "Binary '${binary_name}' not found in nightly build archive"
      log "Available binaries in archive:"
      find "${TMP_DIR}/nightly" -type f -name "voiyd-node-*" | sed 's/^/  /'
      fatal "Could not find binary for architecture: ${arch}"
    fi

    log_verbose "Found binary: $tmp_binary"
  else
    log "Installing voiyd-node ${VOIYD_VERSION} for linux-${arch}..."
    download_url="${GITHUB_DOWNLOAD_URL}/${GITHUB_REPO}/releases/download/${VOIYD_VERSION}/${binary_name}"

    # Create bin directory if it doesn't exist
    if [ "$DRY_RUN" = "false" ]; then
      mkdir -p "${PREFIX}/bin"
    fi

    # Download binary
    local tmp_binary="${TMP_DIR}/${binary_name}"
    download_file "$download_url" "$tmp_binary"

    if [ "$DRY_RUN" = "true" ]; then
      log "[DRY RUN] Would install binary to: $dest_path"
      return 0
    fi
  fi

  # Backup existing binary if present
  if [ -f "$dest_path" ]; then
    log "Backing up existing binary to ${dest_path}.backup"
    mv "$dest_path" "${dest_path}.backup"
  fi

  # Install binary
  log "Installing binary to: $dest_path"
  install -m 755 "$tmp_binary" "$dest_path"

  # Verify installation
  if [ -x "$dest_path" ]; then
    success "voiyd-node installed successfully"
    if [ "$USE_NIGHTLY" = "true" ]; then
      log "Version: nightly build from master"
    else
      log "Version: $("$dest_path" --version 2>&1 || echo 'version check not available')"
    fi
  else
    fatal "Installation failed: binary not executable"
  fi
}

install_cni_plugins() {
  local arch
  arch=$(detect_platform)

  local archive_name="cni-plugins-linux-${arch}-${CNI_VERSION}.tgz"
  local download_url="${GITHUB_DOWNLOAD_URL}/${CNI_REPO}/releases/download/${CNI_VERSION}/${archive_name}"
  local checksum_url="${download_url}.sha256"

  log "Installing CNI plugins ${CNI_VERSION} for linux-${arch}..."

  # Create CNI directories
  if [ "$DRY_RUN" = "false" ]; then
    mkdir -p "$CNI_BIN_DIR"
    mkdir -p "$CNI_CONF_DIR"
  fi

  # Download CNI plugins archive
  local tmp_archive="${TMP_DIR}/${archive_name}"
  download_file "$download_url" "$tmp_archive"

  # Verify checksum
  local tmp_checksum="${TMP_DIR}/${archive_name}.sha256"
  download_file "$checksum_url" "$tmp_checksum"

  if [ "$DRY_RUN" = "false" ]; then
    log "Verifying checksum..."
    (cd "$TMP_DIR" && sha256sum -c "${archive_name}.sha256") || fatal "Checksum verification failed"

    # Extract plugins
    log "Extracting CNI plugins to: $CNI_BIN_DIR"
    tar -xzf "$tmp_archive" -C "$CNI_BIN_DIR"

    success "CNI plugins installed successfully to $CNI_BIN_DIR"
    log "Installed plugins:"
    ls -1 "$CNI_BIN_DIR" | sed 's/^/  - /'
  else
    log "[DRY RUN] Would extract CNI plugins to: $CNI_BIN_DIR"
  fi
}

setup_systemd_service() {
  if [ "$INSTALL_SYSTEMD" != "true" ]; then
    log "Skipping systemd service installation (--no-systemd)"
    return 0
  fi

  log "Setting up systemd service..."

  # Create voiyd directories
  if [ "$DRY_RUN" = "false" ]; then
    mkdir -p "$VOIYD_CONFIG_DIR"
    mkdir -p "$VOIYD_TLS_DIR"
    mkdir -p "$VOIYD_DATA_DIR"
  fi

  # Determine TLS configuration
  local tls_flag=""
  if [ "$INSECURE_SKIP_VERIFY" = "true" ]; then
    tls_flag="--insecure-skip-verify"
    warn "TLS verification disabled. This is insecure and should only be used for development/testing."
  else
    tls_flag=""
    log "TLS verification enabled (secure mode)"
  fi

  # Create systemd service file
  local service_file="${SYSTEMD_DIR}/voiyd-node.service"

  if [ "$DRY_RUN" = "true" ]; then
    log "[DRY RUN] Would create systemd service at: $service_file"
    log "[DRY RUN] Server address: $SERVER_ADDRESS"
    return 0
  fi

  log "Creating systemd service file: $service_file"

  cat >"$service_file" <<EOF
[Unit]
Description=voiyd Node Agent
Documentation=https://github.com/amimof/voiyd
After=network-online.target containerd.service
Wants=network-online.target containerd.service

[Service]
Type=simple

# Node connects to the server over gRPC with TLS
ExecStart=${PREFIX}/bin/voiyd-node \\
  --port=${SERVER_ADDRESS##*:} \\
  --host=${SERVER_ADDRESS%%:*} \\
  ${tls_flag} \\
  --metrics-host=${METRICS_HOST}

Restart=on-failure
RestartSec=5s

# Hardening
NoNewPrivileges=true
LimitNOFILE=65535

[Install]
WantedBy=multi-user.target
EOF

  # Reload systemd
  log "Reloading systemd daemon..."
  systemctl daemon-reload

  # Enable service
  log "Enabling voiyd-node service..."
  systemctl enable voiyd-node.service

  success "Systemd service configured"

  # Start service if requested
  if [ "$START_SERVICE" = "true" ]; then
    log "Starting voiyd-node service..."
    systemctl start voiyd-node.service

    # Check status
    sleep 2
    if systemctl is-active --quiet voiyd-node.service; then
      success "voiyd-node service is running"
    else
      error "Service failed to start. Check status with: systemctl status voiyd-node"
    fi
  else
    log "Service not started. Start it with: systemctl start voiyd-node"
  fi
}

# ============================================================================
# Verification
# ============================================================================

verify_installation() {
  log "Verifying installation..."

  local binary_path="${PREFIX}/bin/voiyd-node"

  # Check binary
  if [ -x "$binary_path" ]; then
    success "Binary installed: $binary_path"
  else
    error "Binary not found or not executable: $binary_path"
    return 1
  fi

  # Check CNI plugins
  if [ -d "$CNI_BIN_DIR" ] && [ "$(ls -A "$CNI_BIN_DIR" 2>/dev/null)" ]; then
    success "CNI plugins installed: $CNI_BIN_DIR"
  else
    warn "CNI plugins directory empty or not found: $CNI_BIN_DIR"
  fi

  # Check systemd service
  if [ "$INSTALL_SYSTEMD" = "true" ] && [ -f "${SYSTEMD_DIR}/voiyd-node.service" ]; then
    success "Systemd service configured: voiyd-node.service"
  fi

  # Check dependencies
  if command_exists iptables; then
    success "iptables is available"
  else
    warn "iptables not found"
  fi

  if command_exists containerd; then
    success "containerd is available"
  else
    warn "containerd not found"
  fi
}

# ============================================================================
# Usage and Help
# ============================================================================

show_usage() {
  cat <<EOF
${BOLD}Voiyd Node Installation Script${NC}

Installs voiyd-node and its dependencies on Linux systems.

${BOLD}USAGE:${NC}
    curl -sfL https://raw.githubusercontent.com/amimof/voiyd/master/setup-node.sh | sh -
    curl -sfL https://raw.githubusercontent.com/amimof/voiyd/master/setup-node.sh | sh -s -- [OPTIONS]

${BOLD}OPTIONS:${NC}
    ${BOLD}Version Options:${NC}
    --version <version>         voiyd version to install (default: latest)
                                Example: --version v0.0.11
    --nightly                   Install latest nightly build from master branch
                                (overrides --version)
    --cni-version <version>     CNI plugins version (default: v1.9.0)

    ${BOLD}Installation Paths:${NC}
    --prefix <path>             Installation prefix (default: /usr/local)
                                Binary will be installed to <prefix>/bin/voiyd-node
    --cni-bin-dir <path>        CNI plugins directory (default: /opt/cni/bin)
    --cni-conf-dir <path>       CNI config directory (default: /etc/cni/net.d)

    ${BOLD}Server Configuration:${NC}
    --server-address <addr>     Server address (default: localhost:5743)
                                Example: --server-address server.example.com:5743
    --insecure-skip-verify      Skip TLS certificate verification (insecure, dev only)
    --metrics-host <host>       Metrics host (default: 0.0.0.0)

    ${BOLD}Systemd Options:${NC}
    --no-systemd                Don't install systemd service
    --start                     Start the service after installation

    ${BOLD}Dependency Options:${NC}
    --auto-install-deps         Automatically install missing dependencies
    --skip-deps                 Skip dependency checks (not recommended)

    ${BOLD}Other Options:${NC}
    --dry-run                   Show what would be installed without making changes
    --verbose, -v               Enable verbose output
    --help, -h                  Show this help message

${BOLD}EXAMPLES:${NC}
    # Install latest version with auto-install dependencies
    curl -sfL https://raw.githubusercontent.com/amimof/voiyd/master/setup-node.sh | \\
      sh -s -- --auto-install-deps

    # Install specific version with custom prefix
    curl -sfL https://raw.githubusercontent.com/amimof/voiyd/master/setup-node.sh | \\
      sh -s -- --version v0.0.11 --prefix /usr

    # Install latest nightly build
    curl -sfL https://raw.githubusercontent.com/amimof/voiyd/master/setup-node.sh | \\
      sh -s -- --nightly --auto-install-deps

    # Install and start service with server configuration
    curl -sfL https://raw.githubusercontent.com/amimof/voiyd/master/setup-node.sh | \\
      sh -s -- \\
        --server-address voiyd.example.com:5743 \\
        --insecure-skip-verify \\
        --auto-install-deps \\
        --start

    # Dry run to see what would be installed
    curl -sfL https://raw.githubusercontent.com/amimof/voiyd/master/setup-node.sh | \\
      sh -s -- --dry-run --verbose

${BOLD}REQUIREMENTS:${NC}
    - Linux operating system
    - Root privileges (sudo)
    - curl or wget
    - tar
    - unzip (for nightly builds, auto-installed with --auto-install-deps)
    - containerd >= 1.6
    - iptables

${BOLD}MORE INFO:${NC}
    GitHub: https://github.com/amimof/voiyd
    Documentation: https://voiyd.io

EOF
}

# ============================================================================
# Argument Parsing
# ============================================================================

parse_args() {
  while [ $# -gt 0 ]; do
    case "$1" in
    --version)
      VOIYD_VERSION="$2"
      shift 2
      ;;
    --nightly)
      USE_NIGHTLY="true"
      shift
      ;;
    --cni-version)
      CNI_VERSION="$2"
      shift 2
      ;;
    --prefix)
      PREFIX="$2"
      shift 2
      ;;
    --cni-bin-dir)
      CNI_BIN_DIR="$2"
      shift 2
      ;;
    --cni-conf-dir)
      CNI_CONF_DIR="$2"
      shift 2
      ;;
    --server-address)
      SERVER_ADDRESS="$2"
      shift 2
      ;;
    --insecure-skip-verify)
      INSECURE_SKIP_VERIFY="true"
      shift
      ;;
    --metrics-host)
      METRICS_HOST="$2"
      shift 2
      ;;
    --no-systemd)
      INSTALL_SYSTEMD="false"
      shift
      ;;
    --start)
      START_SERVICE="true"
      shift
      ;;
    --auto-install-deps)
      AUTO_INSTALL_DEPS="true"
      shift
      ;;
    --skip-deps)
      SKIP_DEPS="true"
      shift
      ;;
    --dry-run)
      DRY_RUN="true"
      shift
      ;;
    --verbose | -v)
      VERBOSE="true"
      shift
      ;;
    --help | -h)
      show_usage
      exit 0
      ;;
    *)
      error "Unknown option: $1"
      show_usage
      exit 1
      ;;
    esac
  done
}

# ============================================================================
# Main Execution
# ============================================================================

main() {
  echo ""
  echo -e "${BOLD}╔═══════════════════════════════════════════════════════════╗${NC}"
  echo -e "${BOLD}║          Voiyd Node Installation Script                  ║${NC}"
  echo -e "${BOLD}╚═══════════════════════════════════════════════════════════╝${NC}"
  echo ""

  parse_args "$@"

  if [ "$DRY_RUN" = "true" ]; then
    warn "DRY RUN MODE - No changes will be made"
    echo ""
  fi

  # Pre-flight checks
  check_root

  # Resolve versions
  resolve_version

  # Show configuration
  log "Installation Configuration:"
  if [ "$USE_NIGHTLY" = "true" ]; then
    log "  voiyd version:    ${VOIYD_VERSION} (nightly build)"
  else
    log "  voiyd version:    ${VOIYD_VERSION}"
  fi
  log "  CNI version:      ${CNI_VERSION}"
  log "  Install prefix:   ${PREFIX}"
  log "  CNI bin dir:      ${CNI_BIN_DIR}"
  log "  Server address:   ${SERVER_ADDRESS}"
  log "  Auto-install deps: ${AUTO_INSTALL_DEPS}"
  log "  Install systemd:  ${INSTALL_SYSTEMD}"
  echo ""

  # Check and install dependencies
  check_and_install_dependencies
  echo ""

  # Install voiyd-node
  install_voiyd_node
  echo ""

  # Install CNI plugins
  install_cni_plugins
  echo ""

  # Setup systemd service
  setup_systemd_service
  echo ""

  # Verify installation
  verify_installation
  echo ""

  # Final message
  echo -e "${BOLD}╔═══════════════════════════════════════════════════════════╗${NC}"
  echo -e "${BOLD}║              Installation Complete!                       ║${NC}"
  echo -e "${BOLD}╚═══════════════════════════════════════════════════════════╝${NC}"
  echo ""

  if [ "$DRY_RUN" = "false" ]; then
    success "voiyd-node ${VOIYD_VERSION} has been installed successfully!"
    echo ""
    log "Binary location: ${PREFIX}/bin/voiyd-node"
    log "CNI plugins: ${CNI_BIN_DIR}"

    if [ "$INSTALL_SYSTEMD" = "true" ]; then
      echo ""
      log "Systemd service commands:"
      log "  Start:   systemctl start voiyd-node"
      log "  Stop:    systemctl stop voiyd-node"
      log "  Status:  systemctl status voiyd-node"
      log "  Logs:    journalctl -u voiyd-node -f"

      if [ "$INSECURE_SKIP_VERIFY" = "true" ]; then
        echo ""
        warn "TLS verification is disabled (--insecure-skip-verify)."
        warn "This is NOT recommended for production environments."
      fi
    fi

    echo ""
    log "For more information, visit: https://github.com/amimof/voiyd"
  else
    log "Dry run completed. No changes were made."
  fi

  echo ""
}

# Run main function
main "$@"
