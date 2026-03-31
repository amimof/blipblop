#!/usr/bin/env bash

# Quick start script for running e2e tests
# This script sets up, runs, and tears down the e2e test environment

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m'

log_info() {
  echo -e "${GREEN}[INFO]${NC} $*"
}

log_error() {
  echo -e "${RED}[ERROR]${NC} $*" >&2
}

log_warn() {
  echo -e "${YELLOW}[WARN]${NC} $*"
}

# Cleanup function
cleanup() {
  local exit_code=$?
  
  if [ $exit_code -ne 0 ]; then
    log_error "E2E tests failed!"
  fi
  
  if [ "${SKIP_CLEANUP:-false}" != "true" ]; then
    log_info "Cleaning up..."
    bash "$SCRIPT_DIR/scripts/teardown-cluster.sh" || true
  else
    log_warn "Skipping cleanup (SKIP_CLEANUP=true)"
  fi
  
  exit $exit_code
}

trap cleanup EXIT

# Main execution
main() {
  log_info "Starting Voiyd E2E Test Suite"
  echo ""
  
  # Check prerequisites
  log_info "Checking prerequisites..."
  
  for cmd in kind kubectl docker jq; do
    if ! command -v $cmd &> /dev/null; then
      log_error "$cmd is not installed. Please install it first."
      log_info "See hack/e2e/README.md for installation instructions"
      exit 1
    fi
  done
  
  log_info "All prerequisites satisfied"
  echo ""
  
  # Step 1: Setup cluster
  log_info "Step 1/4: Setting up Kind cluster..."
  if ! bash "$SCRIPT_DIR/scripts/setup-cluster.sh"; then
    log_error "Failed to setup cluster"
    exit 1
  fi
  echo ""
  
  # Step 2: Deploy voiyd
  log_info "Step 2/4: Deploying voiyd components..."
  if ! bash "$SCRIPT_DIR/scripts/deploy-voiyd.sh"; then
    log_error "Failed to deploy voiyd"
    exit 1
  fi
  echo ""
  
  # Step 3: Wait for ready
  log_info "Step 3/4: Waiting for components to be ready..."
  if ! bash "$SCRIPT_DIR/scripts/wait-for-ready.sh"; then
    log_error "Components did not become ready"
    exit 1
  fi
  echo ""
  
  # Step 4: Run tests
  log_info "Step 4/4: Running e2e tests..."
  if ! bash "$SCRIPT_DIR/scripts/run-all-tests.sh"; then
    log_error "Tests failed"
    exit 1
  fi
  echo ""
  
  log_info "E2E test suite completed successfully!"
}

# Parse arguments
while [[ $# -gt 0 ]]; do
  case $1 in
    --skip-cleanup)
      export SKIP_CLEANUP=true
      shift
      ;;
    --help|-h)
      cat << HELP
Usage: $0 [OPTIONS]

Run the complete Voiyd e2e test suite.

Options:
  --skip-cleanup    Don't delete the cluster after tests (for debugging)
  --help, -h        Show this help message

Environment Variables:
  CLUSTER_NAME            Cluster name (default: voiyd-e2e)
  IMAGE_TAG               Image tag (default: e2e-test)
  CONTINUE_ON_FAILURE     Continue if a test fails (default: true)
  CLEANUP_BETWEEN_TESTS   Clean up between tests (default: true)
  DEBUG                   Enable debug logging (default: false)
  SKIP_CLEANUP            Skip cluster cleanup (default: false)

Examples:
  # Run all tests
  $0

  # Run tests and keep cluster for debugging
  $0 --skip-cleanup

  # Enable debug logging
  DEBUG=true $0

For more information, see hack/e2e/README.md
HELP
      exit 0
      ;;
    *)
      log_error "Unknown option: $1"
      log_info "Use --help for usage information"
      exit 1
      ;;
  esac
done

main
