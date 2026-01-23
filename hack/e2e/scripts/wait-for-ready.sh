#!/usr/bin/env bash

# Wait for voiyd components to be ready
# This script waits for all voiyd components to be healthy before running tests

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# shellcheck source=hack/e2e/scripts/helpers.sh
source "$SCRIPT_DIR/helpers.sh"

NAMESPACE="voiyd"
TIMEOUT="${TIMEOUT:-600}"

log_info "Waiting for voiyd components to be ready..."

# Wait for namespace
log_info "Checking namespace..."
if ! kubectl get namespace "$NAMESPACE" &> /dev/null; then
  log_error "Namespace '$NAMESPACE' does not exist"
  exit 1
fi

# Wait for voiyd-server deployment
log_info "Waiting for voiyd-server deployment..."
if ! wait_for_deployment "$NAMESPACE" "voiyd-server" "$TIMEOUT"; then
  log_error "voiyd-server deployment is not ready"
  kubectl describe deployment voiyd-server -n "$NAMESPACE"
  kubectl logs -n "$NAMESPACE" -l app=voiyd-server --tail=50
  exit 1
fi

# Wait for voiyd-server pod to be running
log_info "Waiting for voiyd-server pod..."
if ! wait_for_pod "$NAMESPACE" "app=voiyd-server" "$TIMEOUT"; then
  log_error "voiyd-server pod is not ready"
  exit 1
fi

# Wait for voiyd-node statefulset (3 replicas)
log_info "Waiting for voiyd-node statefulset..."
if ! kubectl wait --for=jsonpath='{.status.readyReplicas}'=3 statefulset/voiyd-node \
  -n "$NAMESPACE" --timeout="${TIMEOUT}s" 2>/dev/null; then
  log_error "voiyd-node statefulset is not ready"
  kubectl describe statefulset voiyd-node -n "$NAMESPACE"
  kubectl logs -n "$NAMESPACE" -l app=voiyd-node -c node --tail=50
  exit 1
fi

log_success "All 3 voiyd-node pods are ready"

# Wait for voiydctl pod
log_info "Waiting for voiydctl pod..."
if ! wait_for_pod "$NAMESPACE" "app=voiydctl" "$TIMEOUT"; then
  log_error "voiydctl pod is not ready"
  exit 1
fi

# Configure voiydctl
log_info "Configuring voiydctl..."
if ! kubectl exec -n "$NAMESPACE" voiydctl -- voiydctl config init 2>/dev/null; then
  log_warn "voiydctl config already initialized or failed to initialize"
fi

if ! kubectl exec -n "$NAMESPACE" voiydctl -- voiydctl config create-server e2e \
  --address voiyd-server:5743 --tls --insecure 2>/dev/null; then
  log_warn "voiydctl server config already exists or failed to create"
fi

# Wait for voiyd nodes to register with server
log_info "Waiting for voiyd nodes to register with server..."
if ! wait_for_node_count 3 120; then
  log_error "Not all voiyd nodes registered with server"
  kubectl exec -n "$NAMESPACE" voiydctl -- voiydctl get nodes || true
  exit 1
fi

# Display cluster status
log_success "All components are ready!"
echo ""
log_info "Cluster status:"
echo ""
kubectl get pods -n "$NAMESPACE" -o wide
echo ""
log_info "Voiyd nodes:"
kubectl exec -n "$NAMESPACE" voiydctl -- voiydctl get nodes || true
echo ""

log_success "Voiyd cluster is ready for e2e testing!"
