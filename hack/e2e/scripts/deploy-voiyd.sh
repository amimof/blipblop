#!/usr/bin/env bash

# Deploy voiyd components to Kind cluster
# This script applies all Kubernetes manifests for voiyd

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
E2E_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
MANIFESTS_DIR="$E2E_DIR/manifests"

# shellcheck source=hack/e2e/scripts/helpers.sh
source "$SCRIPT_DIR/helpers.sh"

log_info "Deploying voiyd components to cluster..."

# Generate self-signed certificates for e2e testing
generate_e2e_certs() {
  local temp_dir
  temp_dir=$(mktemp -d)
  
  log_info "Generating self-signed certificates for e2e testing..."
  
  # Generate CA key and certificate
  openssl genrsa -out "$temp_dir/ca.key" 4096 2>/dev/null
  openssl req -x509 -new -nodes -key "$temp_dir/ca.key" -sha256 -days 365 \
    -out "$temp_dir/ca.crt" \
    -subj "/C=SE/ST=Europe/L=Stockholm/O=voiyd/OU=e2e-testing/CN=CA" 2>/dev/null
  
  # Generate server key and certificate
  openssl genrsa -out "$temp_dir/server.key" 4096 2>/dev/null
  openssl req -new -key "$temp_dir/server.key" \
    -out "$temp_dir/server.csr" \
    -subj "/C=SE/ST=Europe/L=Stockholm/O=voiyd/OU=e2e-testing/CN=voiyd-server" 2>/dev/null
  
  # Create certificate extensions file
  cat > "$temp_dir/server.ext" << EXTEOF
authorityKeyIdentifier=keyid,issuer
basicConstraints=CA:FALSE
keyUsage = digitalSignature, nonRepudiation, keyEncipherment, dataEncipherment
subjectAltName = @alt_names

[alt_names]
DNS.1 = voiyd-server
DNS.2 = voiyd-server.voiyd
DNS.3 = voiyd-server.voiyd.svc
DNS.4 = voiyd-server.voiyd.svc.cluster.local
DNS.5 = localhost
IP.1 = 127.0.0.1
EXTEOF
  
  openssl x509 -req -in "$temp_dir/server.csr" \
    -CA "$temp_dir/ca.crt" -CAkey "$temp_dir/ca.key" -CAcreateserial \
    -out "$temp_dir/server.crt" -days 365 -sha256 \
    -extfile "$temp_dir/server.ext" 2>/dev/null
  
  # Update the certificates manifest
  cat > "$MANIFESTS_DIR/02-certs.yaml" << CERTEOF
---
apiVersion: v1
kind: Secret
metadata:
  name: voiyd-ca-cert
  namespace: voiyd
type: Opaque
stringData:
  ca.crt: |
$(sed 's/^/    /' "$temp_dir/ca.crt")
---
apiVersion: v1
kind: Secret
metadata:
  name: voiyd-server-tls
  namespace: voiyd
type: kubernetes.io/tls
stringData:
  tls.crt: |
$(sed 's/^/    /' "$temp_dir/server.crt")
  tls.key: |
$(sed 's/^/    /' "$temp_dir/server.key")
CERTEOF
  
  rm -rf "$temp_dir"
  log_success "Certificates generated successfully"
}

# Check if openssl is installed
if ! command -v openssl &> /dev/null; then
  log_warn "openssl is not installed. Using placeholder certificates..."
else
  generate_e2e_certs
fi

# Apply manifests in order
log_info "Applying Kubernetes manifests..."

for manifest in "$MANIFESTS_DIR"/*.yaml; do
  log_info "Applying $(basename "$manifest")..."
  if ! kubectl apply -f "$manifest"; then
    log_error "Failed to apply $manifest"
    exit 1
  fi
done

log_success "All manifests applied successfully"

log_info "Deployment initiated. Use wait-for-ready.sh to wait for components to be ready."
