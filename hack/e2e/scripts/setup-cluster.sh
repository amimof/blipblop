#!/usr/bin/env bash

# Setup Kind cluster for e2e testing
# This script creates a Kind cluster and loads necessary images

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
E2E_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
PROJECT_ROOT="$(cd "$E2E_DIR/../.." && pwd)"

# shellcheck source=hack/e2e/scripts/helpers.sh
source "$SCRIPT_DIR/helpers.sh"

CLUSTER_NAME="${CLUSTER_NAME:-voiyd-e2e}"
IMAGE_TAG="${IMAGE_TAG:-e2e-test}"
IMAGE_NAME="ghcr.io/amimof/voiyd:${IMAGE_TAG}"

log_info "Setting up Kind cluster for e2e tests..."

# Check if kind is installed
if ! command -v kind &> /dev/null; then
  log_error "kind is not installed. Please install it from https://kind.sigs.k8s.io/docs/user/quick-start/#installation"
  exit 1
fi

# Check if kubectl is installed
if ! command -v kubectl &> /dev/null; then
  log_error "kubectl is not installed. Please install it first"
  exit 1
fi

# Check if cluster already exists
if kind get clusters 2>/dev/null | grep -q "^${CLUSTER_NAME}$"; then
  log_warn "Cluster '$CLUSTER_NAME' already exists. Deleting it..."
  kind delete cluster --name "$CLUSTER_NAME"
fi

# Create Kind cluster
log_info "Creating Kind cluster '$CLUSTER_NAME'..."
if ! kind create cluster --config "$E2E_DIR/kind-config.yaml" --name "$CLUSTER_NAME"; then
  log_error "Failed to create Kind cluster"
  exit 1
fi

log_success "Kind cluster created successfully"

# Wait for cluster to be ready
log_info "Waiting for cluster to be ready..."
if ! kubectl wait --for=condition=ready nodes --all --timeout=300s; then
  log_error "Cluster nodes are not ready"
  exit 1
fi

log_success "Cluster is ready"

# Build voiyd image if not exists
log_info "Checking if voiyd image exists..."
if ! docker images | grep -q "$IMAGE_NAME"; then
  log_info "Building voiyd image..."
  cd "$PROJECT_ROOT"
  if ! docker build -t "$IMAGE_NAME" .; then
    log_error "Failed to build voiyd image"
    exit 1
  fi
  log_success "Voiyd image built successfully"
else
  log_info "Voiyd image already exists"
fi

# Load image into Kind cluster
log_info "Loading voiyd image into Kind cluster..."
if ! kind load docker-image "$IMAGE_NAME" --name "$CLUSTER_NAME"; then
  log_error "Failed to load image into Kind cluster"
  exit 1
fi

log_success "Image loaded successfully"

# Verify cluster nodes
log_info "Cluster nodes:"
kubectl get nodes -o wide

log_success "Kind cluster setup completed successfully!"
log_info "Cluster name: $CLUSTER_NAME"
log_info "Image: $IMAGE_NAME"
