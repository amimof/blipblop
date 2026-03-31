#!/usr/bin/env bash

# Run all e2e tests
# This script orchestrates the execution of all e2e test suites

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
E2E_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
TESTS_DIR="$E2E_DIR/tests"

# shellcheck source=hack/e2e/scripts/helpers.sh
source "$SCRIPT_DIR/helpers.sh"

# Test configuration
CONTINUE_ON_FAILURE="${CONTINUE_ON_FAILURE:-true}"
CLEANUP_BETWEEN_TESTS="${CLEANUP_BETWEEN_TESTS:-true}"

# Initialize test results
TESTS_PASSED=0
TESTS_FAILED=0
FAILED_TESTS=()

log_info "Starting e2e test suite..."
log_info "Continue on failure: $CONTINUE_ON_FAILURE"
log_info "Cleanup between tests: $CLEANUP_BETWEEN_TESTS"
echo ""

# Find all test scripts
TEST_SCRIPTS=($(find "$TESTS_DIR" -name "*.sh" -type f | sort))

if [ ${#TEST_SCRIPTS[@]} -eq 0 ]; then
  log_error "No test scripts found in $TESTS_DIR"
  exit 1
fi

log_info "Found ${#TEST_SCRIPTS[@]} test(s) to run"
echo ""

# Run each test
for test_script in "${TEST_SCRIPTS[@]}"; do
  test_name=$(basename "$test_script" .sh)
  
  log_info "=========================================="
  log_info "Running test: $test_name"
  log_info "=========================================="
  
  start_time=$(date +%s)
  
  if bash "$test_script"; then
    end_time=$(date +%s)
    duration=$((end_time - start_time))
    
    TESTS_PASSED=$((TESTS_PASSED + 1))
    log_success "Test PASSED: $test_name (${duration}s)"
    record_test_result "$test_name" "PASS" "$duration"
    
    # Cleanup after successful test
    if [ "$CLEANUP_BETWEEN_TESTS" = "true" ]; then
      log_info "Cleaning up after test..."
      cleanup_all_tasks || true
    fi
  else
    end_time=$(date +%s)
    duration=$((end_time - start_time))
    
    TESTS_FAILED=$((TESTS_FAILED + 1))
    FAILED_TESTS+=("$test_name")
    log_error "Test FAILED: $test_name (${duration}s)"
    record_test_result "$test_name" "FAIL" "$duration"
    
    # Collect diagnostics on failure
    log_info "Collecting diagnostics..."
    {
      echo "=== Pods ==="
      kubectl get pods -n voiyd -o wide
      echo ""
      echo "=== Server Logs ==="
      kubectl logs -n voiyd -l app=voiyd-server --tail=100
      echo ""
      echo "=== Node Logs ==="
      kubectl logs -n voiyd -l app=voiyd-node -c node --tail=100
      echo ""
      echo "=== Tasks ==="
      kubectl exec -n voiyd voiydctl -- voiydctl get tasks || true
      echo ""
      echo "=== Nodes ==="
      kubectl exec -n voiyd voiydctl -- voiydctl get nodes || true
    } > "$E2E_DIR/results/${test_name}-failure.log" 2>&1
    
    if [ "$CONTINUE_ON_FAILURE" != "true" ]; then
      log_error "Stopping test execution due to failure"
      break
    fi
    
    # Cleanup after failed test
    if [ "$CLEANUP_BETWEEN_TESTS" = "true" ]; then
      log_info "Cleaning up after failed test..."
      cleanup_all_tasks || true
    fi
  fi
  
  echo ""
done

# Print summary
echo ""
echo "=========================================="
echo "E2E Test Summary"
echo "=========================================="
echo "Total tests:  $((TESTS_PASSED + TESTS_FAILED))"
echo "Passed:       $TESTS_PASSED"
echo "Failed:       $TESTS_FAILED"
echo ""

if [ $TESTS_FAILED -gt 0 ]; then
  echo "Failed tests:"
  for failed_test in "${FAILED_TESTS[@]}"; do
    echo "  - $failed_test"
  done
  echo ""
fi

# Generate results summary
{
  echo "E2E Test Results - $(date)"
  echo "================================"
  echo "Total: $((TESTS_PASSED + TESTS_FAILED))"
  echo "Passed: $TESTS_PASSED"
  echo "Failed: $TESTS_FAILED"
  echo ""
  echo "Detailed Results:"
  cat "$E2E_DIR/results/results.log" 2>/dev/null || echo "No results log found"
} > "$E2E_DIR/results/summary.txt"

if [ $TESTS_FAILED -eq 0 ]; then
  log_success "All tests passed!"
  exit 0
else
  log_error "Some tests failed"
  exit 1
fi
