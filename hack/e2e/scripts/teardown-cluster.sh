#!/usr/bin/env bash

# Teardown Kind cluster
# This script deletes the Kind cluster used for e2e testing

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# shellcheck source=hack/e2e/scripts/helpers.sh
source "$SCRIPT_DIR/helpers.sh"

CLUSTER_NAME="${CLUSTER_NAME:-voiyd-e2e}"

log_info "Tearing down Kind cluster '$CLUSTER_NAME'..."

# Check if kind is installed
if ! command -v kind &> /dev/null; then
  log_error "kind is not installed"
  exit 1
fi

# Check if cluster exists
if ! kind get clusters 2>/dev/null | grep -q "^${CLUSTER_NAME}$"; then
  log_warn "Cluster '$CLUSTER_NAME' does not exist"
  exit 0
fi

# Delete cluster
log_info "Deleting cluster..."
if ! kind delete cluster --name "$CLUSTER_NAME"; then
  log_error "Failed to delete cluster"
  exit 1
fi

log_success "Cluster '$CLUSTER_NAME' deleted successfully"
