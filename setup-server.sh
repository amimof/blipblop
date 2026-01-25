#!/usr/bin/env bash

# Voiyd Server Installation Script
# This script installs voiyd-server and generates self-signed certificates on Linux systems
# Usage: curl -sfL https://raw.githubusercontent.com/amimof/voiyd/master/setup-server.sh | sh -
# Or: curl -sfL https://raw.githubusercontent.com/amimof/voiyd/master/setup-server.sh | sh -s -- [options]

set -e
set -o pipefail

# ============================================================================
# Configuration Variables
# ============================================================================

# Default values
VOIYD_VERSION="${VOIYD_VERSION:-latest}"
USE_NIGHTLY="${USE_NIGHTLY:-false}"
PREFIX="${PREFIX:-/usr/local}"
SYSTEMD_DIR="${SYSTEMD_DIR:-/etc/systemd/system}"
VOIYD_CONFIG_DIR="${VOIYD_CONFIG_DIR:-/etc/voiyd}"
VOIYD_TLS_DIR="${VOIYD_TLS_DIR:-/etc/voiyd/tls}"

# Server configuration
SERVER_ADDRESS="${SERVER_ADDRESS:-0.0.0.0:5743}"
GATEWAY_ADDRESS="${GATEWAY_ADDRESS:-0.0.0.0:8443}"
METRICS_ADDRESS="${METRICS_ADDRESS:-0.0.0.0:8888}"

# TLS Certificate configuration
CERT_COUNTRY="${CERT_COUNTRY:-SE}"
CERT_STATE="${CERT_STATE:-Halland}"
CERT_CITY="${CERT_CITY:-Varberg}"
CERT_ORG="${CERT_ORG:-voiyd}"
CERT_OU="${CERT_OU:-server}"
CERT_CN="${CERT_CN:-voiyd-server}"
CERT_DAYS="${CERT_DAYS:-365}"
GENERATE_CERTS="${GENERATE_CERTS:-true}"

# Installation options
INSTALL_SYSTEMD="${INSTALL_SYSTEMD:-true}"
START_SERVICE="${START_SERVICE:-true}"
AUTO_INSTALL_DEPS="${AUTO_INSTALL_DEPS:-false}"
SKIP_DEPS="${SKIP_DEPS:-false}"
DRY_RUN="${DRY_RUN:-false}"
VERBOSE="${VERBOSE:-false}"

# GitHub repository
GITHUB_REPO="amimof/voiyd"
GITHUB_API_URL="https://api.github.com/repos"
GITHUB_DOWNLOAD_URL="https://github.com"
# Nightly builds: single zip containing all architectures (amd64, arm64, arm)
NIGHTLY_LINK_URL="https://nightly.link/amimof/voiyd/workflows/upload.yaml/master/voiyd-server-linux-master.zip"

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
    fatal "Unsupported operating system: $os. voiyd-server installation script requires Linux."
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

  if command_exists curl; then
    curl -sL "$url" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/'
  elif command_exists wget; then
    wget -qO- "$url" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/'
  else
    fatal "Neither curl nor wget is available. Please install one of them."
  fi
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

check_openssl() {
  log "Checking for openssl..."
  if command_exists openssl; then
    local version
    version=$(openssl version 2>&1)
    log "Found: $version"
    return 0
  else
    warn "openssl not found"
    return 1
  fi
}

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

install_openssl() {
  local distro
  distro=$(detect_distro)

  log "Installing openssl for $distro..."

  case "$distro" in
  ubuntu | debian)
    apt-get update -qq
    apt-get install -y openssl
    ;;
  centos | rhel | fedora)
    if command_exists dnf; then
      dnf install -y openssl
    else
      yum install -y openssl
    fi
    ;;
  arch)
    pacman -S --noconfirm openssl
    ;;
  *)
    warn "Unknown distro: $distro. Please install openssl manually."
    return 1
    ;;
  esac
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

  # Check openssl (required for certificate generation)
  if [ "$GENERATE_CERTS" = "true" ]; then
    if ! check_openssl; then
      missing_deps+=("openssl")
    fi
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
      openssl)
        install_openssl || fatal "Failed to install openssl"
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

# ============================================================================
# TLS Certificate Generation
# ============================================================================

generate_certificates() {
  if [ "$GENERATE_CERTS" != "true" ]; then
    log "Skipping certificate generation (--skip-cert-generation)"
    return 0
  fi

  log "Generating self-signed TLS certificates..."

  if [ "$DRY_RUN" = "true" ]; then
    log "[DRY RUN] Would generate certificates in: $VOIYD_TLS_DIR"
    return 0
  fi

  # Create TLS directory
  mkdir -p "$VOIYD_TLS_DIR"

  # Get system hostname for certificate
  local hostname
  hostname=$(hostname -f 2>/dev/null || hostname)

  log "Generating CA certificate..."

  # Generate CA private key
  openssl genrsa -out "${VOIYD_TLS_DIR}/ca.key" 4096 2>/dev/null

  # Generate CA certificate
  openssl req -x509 -new -nodes -sha256 -days "$CERT_DAYS" \
    -key "${VOIYD_TLS_DIR}/ca.key" \
    -out "${VOIYD_TLS_DIR}/ca.crt" \
    -subj "/C=${CERT_COUNTRY}/ST=${CERT_STATE}/L=${CERT_CITY}/O=${CERT_ORG}/OU=${CERT_OU}/CN=voiyd-ca" \
    2>/dev/null || fatal "Failed to generate CA certificate"

  success "CA certificate generated"

  log "Generating server certificate..."

  # Generate server private key
  openssl genrsa -out "${VOIYD_TLS_DIR}/server.key" 2048 2>/dev/null

  # Create OpenSSL config for server certificate with SANs
  cat >"${TMP_DIR}/server-openssl.conf" <<EOF
[ req ]
default_bits       = 2048
prompt             = no
default_md         = sha256
distinguished_name = req_distinguished_name
req_extensions     = v3_req

[ req_distinguished_name ]
C  = ${CERT_COUNTRY}
ST = ${CERT_STATE}
L  = ${CERT_CITY}
O  = ${CERT_ORG}
OU = ${CERT_OU}
CN = ${CERT_CN}

[ v3_req ]
basicConstraints = CA:FALSE
keyUsage = digitalSignature, keyEncipherment
extendedKeyUsage = serverAuth
subjectAltName = @alt_names

[ alt_names ]
DNS.1 = localhost
DNS.2 = ${hostname}
DNS.3 = ${CERT_CN}
IP.1  = 127.0.0.1
IP.2  = ::1
EOF

  # Generate server CSR
  openssl req -new -sha256 \
    -key "${VOIYD_TLS_DIR}/server.key" \
    -out "${TMP_DIR}/server.csr" \
    -config "${TMP_DIR}/server-openssl.conf" \
    2>/dev/null || fatal "Failed to generate server CSR"

  # Sign server certificate with CA
  openssl x509 -req -sha256 -days "$CERT_DAYS" \
    -in "${TMP_DIR}/server.csr" \
    -CA "${VOIYD_TLS_DIR}/ca.crt" \
    -CAkey "${VOIYD_TLS_DIR}/ca.key" \
    -CAcreateserial \
    -out "${VOIYD_TLS_DIR}/server.crt" \
    -extensions v3_req \
    -extfile "${TMP_DIR}/server-openssl.conf" \
    2>/dev/null || fatal "Failed to sign server certificate"

  # Set proper permissions
  chmod 600 "${VOIYD_TLS_DIR}/ca.key" "${VOIYD_TLS_DIR}/server.key"
  chmod 644 "${VOIYD_TLS_DIR}/ca.crt" "${VOIYD_TLS_DIR}/server.crt"

  success "Server certificate generated"
  log "Certificates saved to: $VOIYD_TLS_DIR"
  log "  CA cert:     ${VOIYD_TLS_DIR}/ca.crt"
  log "  CA key:      ${VOIYD_TLS_DIR}/ca.key"
  log "  Server cert: ${VOIYD_TLS_DIR}/server.crt"
  log "  Server key:  ${VOIYD_TLS_DIR}/server.key"
  echo ""
  warn "These are self-signed certificates for development/testing only!"
  warn "For production, use properly signed certificates from a trusted CA."
}

# ============================================================================
# Installation Functions
# ============================================================================

install_voiyd_server() {
  local arch
  arch=$(detect_platform)

  local binary_name="voiyd-server-linux-${arch}"
  local download_url
  local dest_path="${PREFIX}/bin/voiyd-server"

  if [ "$USE_NIGHTLY" = "true" ]; then
    log "Installing voiyd-server nightly build for linux-${arch}..."

    # Nightly.link provides a single zip file containing all architectures
    local nightly_zip="${TMP_DIR}/voiyd-server-nightly.zip"
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
      find "${TMP_DIR}/nightly" -type f -name "voiyd-server-*" | sed 's/^/  /'
      fatal "Could not find binary for architecture: ${arch}"
    fi

    log_verbose "Found binary: $tmp_binary"
  else
    log "Installing voiyd-server ${VOIYD_VERSION} for linux-${arch}..."
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
    success "voiyd-server installed successfully"
    if [ "$USE_NIGHTLY" = "true" ]; then
      log "Version: nightly build from master"
    else
      log "Version: $("$dest_path" --version 2>&1 || echo 'version check not available')"
    fi
  else
    fatal "Installation failed: binary not executable"
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
  fi

  # Create systemd service file
  local service_file="${SYSTEMD_DIR}/voiyd-server.service"

  if [ "$DRY_RUN" = "true" ]; then
    log "[DRY RUN] Would create systemd service at: $service_file"
    log "[DRY RUN] Server address: $SERVER_ADDRESS"
    return 0
  fi

  log "Creating systemd service file: $service_file"

  cat >"$service_file" <<EOF
[Unit]
Description=voiyd Server
Documentation=https://github.com/amimof/voiyd
After=network-online.target
Wants=network-online.target

[Service]
Type=simple

# Server configuration
ExecStart=${PREFIX}/bin/voiyd-server \\
  --server-address=${SERVER_ADDRESS} \\
  --gateway-address=${GATEWAY_ADDRESS} \\
  --metrics-address=${METRICS_ADDRESS} \\
  --tls-key=${VOIYD_TLS_DIR}/server.key \\
  --tls-certificate=${VOIYD_TLS_DIR}/server.crt

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
  log "Enabling voiyd-server service..."
  systemctl enable voiyd-server.service

  success "Systemd service configured"

  # Start service if requested
  if [ "$START_SERVICE" = "true" ]; then
    log "Starting voiyd-server service..."
    systemctl start voiyd-server.service

    # Check status
    sleep 2
    if systemctl is-active --quiet voiyd-server.service; then
      success "voiyd-server service is running"
    else
      error "Service failed to start. Check status with: systemctl status voiyd-server"
    fi
  else
    log "Service not started. Start it with: systemctl start voiyd-server"
  fi
}

# ============================================================================
# Verification
# ============================================================================

verify_installation() {
  log "Verifying installation..."

  local binary_path="${PREFIX}/bin/voiyd-server"

  # Check binary
  if [ -x "$binary_path" ]; then
    success "Binary installed: $binary_path"
  else
    error "Binary not found or not executable: $binary_path"
    return 1
  fi

  # Check certificates
  if [ "$GENERATE_CERTS" = "true" ]; then
    if [ -f "${VOIYD_TLS_DIR}/ca.crt" ] && [ -f "${VOIYD_TLS_DIR}/server.crt" ] && [ -f "${VOIYD_TLS_DIR}/server.key" ]; then
      success "TLS certificates generated: $VOIYD_TLS_DIR"
    else
      warn "TLS certificates not found in: $VOIYD_TLS_DIR"
    fi
  fi

  # Check systemd service
  if [ "$INSTALL_SYSTEMD" = "true" ] && [ -f "${SYSTEMD_DIR}/voiyd-server.service" ]; then
    success "Systemd service configured: voiyd-server.service"
  fi

  # Check dependencies
  if command_exists openssl; then
    success "openssl is available"
  else
    warn "openssl not found"
  fi
}

# ============================================================================
# Usage and Help
# ============================================================================

show_usage() {
  cat <<EOF
${BOLD}Voiyd Server Installation Script${NC}

Installs voiyd-server and generates self-signed TLS certificates on Linux systems.

${BOLD}USAGE:${NC}
    curl -sfL https://raw.githubusercontent.com/amimof/voiyd/master/setup-server.sh | sh -
    curl -sfL https://raw.githubusercontent.com/amimof/voiyd/master/setup-server.sh | sh -s -- [OPTIONS]

${BOLD}OPTIONS:${NC}
    ${BOLD}Version Options:${NC}
    --version <version>         voiyd version to install (default: latest)
                                Example: --version v0.0.11
    --nightly                   Install latest nightly build from master branch
                                (overrides --version)

    ${BOLD}Installation Paths:${NC}
    --prefix <path>             Installation prefix (default: /usr/local)
                                Binary will be installed to <prefix>/bin/voiyd-server
    --tls-dir <path>            TLS certificate directory (default: /etc/voiyd/tls)
    --data-dir <path>           Data directory (default: /var/lib/voiyd)

    ${BOLD}Server Configuration:${NC}
    --server-address <addr>     Server address (default: 0.0.0.0:5743)
    --gateway-address <addr>    HTTP Gateway address j(default: 0.0.0.0:8443)
    --metrics-address <addr>    Metics server address (default: 0.0.0.0:8889)

    ${BOLD}TLS Certificate Options:${NC}
    --skip-cert-generation      Skip automatic certificate generation
    --cert-days <days>          Certificate validity in days (default: 365)
    --cert-country <code>       Certificate country code (default: US)
    --cert-state <state>        Certificate state (default: State)
    --cert-city <city>          Certificate city (default: City)
    --cert-org <org>            Certificate organization (default: voiyd)
    --cert-cn <cn>              Certificate common name (default: voiyd-server)

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
    curl -sfL https://raw.githubusercontent.com/amimof/voiyd/master/setup-server.sh | \\
      sh -s -- --auto-install-deps

    # Install specific version with custom prefix
    curl -sfL https://raw.githubusercontent.com/amimof/voiyd/master/setup-server.sh | \\
      sh -s -- --version v0.0.11 --prefix /usr

    # Install latest nightly build
    curl -sfL https://raw.githubusercontent.com/amimof/voiyd/master/setup-server.sh | \\
      sh -s -- --nightly --auto-install-deps

    # Install and start service with custom certificate settings
    curl -sfL https://raw.githubusercontent.com/amimof/voiyd/master/setup-server.sh | \\
      sh -s -- \\
        --cert-org "MyCompany" \\
        --cert-cn "voiyd.example.com" \\
        --cert-days 730 \\
        --auto-install-deps \\
        --start

    # Dry run to see what would be installed
    curl -sfL https://raw.githubusercontent.com/amimof/voiyd/master/setup-server.sh | \\
      sh -s -- --dry-run --verbose

${BOLD}REQUIREMENTS:${NC}
    - Linux operating system
    - Root privileges (sudo)
    - curl or wget
    - openssl (for certificate generation, auto-installed with --auto-install-deps)
    - unzip (for nightly builds, auto-installed with --auto-install-deps)

${BOLD}TLS CERTIFICATES:${NC}
    By default, the script generates self-signed certificates for development/testing.
    These certificates are placed in ${VOIYD_TLS_DIR}:
      - ca.crt, ca.key    - Certificate Authority
      - server.crt, server.key - Server certificate

    ${BOLD}WARNING:${NC} Self-signed certificates are for development only!
    For production, use certificates from a trusted CA.

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
    --prefix)
      PREFIX="$2"
      shift 2
      ;;
    --tls-dir)
      VOIYD_TLS_DIR="$2"
      shift 2
      ;;
    --server-address)
      SERVER_ADDRESS="$2"
      shift 2
      ;;
    --gateway-address)
      GATEWAY_ADDRESS="$2"
      shift 2
      ;;
    --metrics-address)
      METRICS_ADDRESS="$2"
      shift 2
      ;;
    --skip-cert-generation)
      GENERATE_CERTS="false"
      shift
      ;;
    --cert-days)
      CERT_DAYS="$2"
      shift 2
      ;;
    --cert-country)
      CERT_COUNTRY="$2"
      shift 2
      ;;
    --cert-state)
      CERT_STATE="$2"
      shift 2
      ;;
    --cert-city)
      CERT_CITY="$2"
      shift 2
      ;;
    --cert-org)
      CERT_ORG="$2"
      shift 2
      ;;
    --cert-cn)
      CERT_CN="$2"
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
  echo -e "${BOLD}║          Voiyd Server Installation Script                ║${NC}"
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
  log "  Install prefix:   ${PREFIX}"
  log "  TLS directory:    ${VOIYD_TLS_DIR}"
  log "  Server address:   ${SERVER_ADDRESS}"
  log "  Generate certs:   ${GENERATE_CERTS}"
  log "  Auto-install deps: ${AUTO_INSTALL_DEPS}"
  log "  Install systemd:  ${INSTALL_SYSTEMD}"
  echo ""

  # Check and install dependencies
  check_and_install_dependencies
  echo ""

  # Install voiyd-server
  install_voiyd_server
  echo ""

  # Generate TLS certificates
  generate_certificates
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
    success "voiyd-server ${VOIYD_VERSION} has been installed successfully!"
    echo ""
    log "Binary location: ${PREFIX}/bin/voiyd-server"
    log "TLS certificates: ${VOIYD_TLS_DIR}"

    if [ "$INSTALL_SYSTEMD" = "true" ]; then
      echo ""
      log "Systemd service commands:"
      log "  Start:   systemctl start voiyd-server"
      log "  Stop:    systemctl stop voiyd-server"
      log "  Status:  systemctl status voiyd-server"
      log "  Logs:    journalctl -u voiyd-server -f"
    fi

    if [ "$GENERATE_CERTS" = "true" ]; then
      echo ""
      warn "Self-signed certificates were generated for development/testing."
      warn "Certificate files:"
      warn "  CA certificate:     ${VOIYD_TLS_DIR}/ca.crt"
      warn "  Server certificate: ${VOIYD_TLS_DIR}/server.crt"
      warn "  Server key:         ${VOIYD_TLS_DIR}/server.key"
      echo ""
      warn "For production deployments, replace these with certificates from a trusted CA."
      echo ""
      log "To use these certificates with voiyd-node, copy ca.crt to the node and use:"
      log "  voiyd-node --tls-ca ${VOIYD_TLS_DIR}/ca.crt --server-address ${SERVER_ADDRESS}"
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
