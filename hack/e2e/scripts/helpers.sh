#!/usr/bin/env bash

# E2E Test Helper Functions
# This file contains common functions used across all e2e tests

set -euo pipefail

# ============================================================================
# Color codes for output
# ============================================================================
if [ -t 1 ]; then
  RED='\033[0;31m'
  GREEN='\033[0;32m'
  YELLOW='\033[1;33m'
  BLUE='\033[0;34m'
  CYAN='\033[0;36m'
  BOLD='\033[1m'
  NC='\033[0m' # No Color
else
  RED=''
  GREEN=''
  YELLOW=''
  BLUE=''
  CYAN=''
  BOLD=''
  NC=''
fi

# ============================================================================
# Logging Functions
# ============================================================================

log_info() {
  echo -e "${GREEN}[INFO]${NC} $(date '+%Y-%m-%d %H:%M:%S') - $*"
}

log_debug() {
  if [ "${DEBUG:-false}" = "true" ]; then
    echo -e "${BLUE}[DEBUG]${NC} $(date '+%Y-%m-%d %H:%M:%S') - $*"
  fi
}

log_warn() {
  echo -e "${YELLOW}[WARN]${NC} $(date '+%Y-%m-%d %H:%M:%S') - $*" >&2
}

log_error() {
  echo -e "${RED}[ERROR]${NC} $(date '+%Y-%m-%d %H:%M:%S') - $*" >&2
}

log_success() {
  echo -e "${GREEN}[SUCCESS]${NC} $(date '+%Y-%m-%d %H:%M:%S') - $*"
}

log_test_start() {
  echo ""
  echo -e "${CYAN}${BOLD}========================================${NC}"
  echo -e "${CYAN}${BOLD} TEST: $*${NC}"
  echo -e "${CYAN}${BOLD}========================================${NC}"
  echo ""
}

log_test_end() {
  echo ""
  echo -e "${GREEN}${BOLD}========================================${NC}"
  echo -e "${GREEN}${BOLD} TEST PASSED: $*${NC}"
  echo -e "${GREEN}${BOLD}========================================${NC}"
  echo ""
}

log_test_fail() {
  echo ""
  echo -e "${RED}${BOLD}========================================${NC}"
  echo -e "${RED}${BOLD} TEST FAILED: $*${NC}"
  echo -e "${RED}${BOLD}========================================${NC}"
  echo ""
}

# ============================================================================
# Assertion Functions
# ============================================================================

assert_equals() {
  local actual="$1"
  local expected="$2"
  local message="${3:-}"
  
  if [ "$actual" != "$expected" ]; then
    log_error "Assertion failed: $message"
    log_error "Expected: '$expected'"
    log_error "Actual:   '$actual'"
    return 1
  fi
  log_debug "Assertion passed: $message"
  return 0
}

assert_not_equals() {
  local actual="$1"
  local not_expected="$2"
  local message="${3:-}"
  
  if [ "$actual" == "$not_expected" ]; then
    log_error "Assertion failed: $message"
    log_error "Should not equal: '$not_expected'"
    log_error "Actual:           '$actual'"
    return 1
  fi
  log_debug "Assertion passed: $message"
  return 0
}

assert_contains() {
  local haystack="$1"
  local needle="$2"
  local message="${3:-}"
  
  if [[ ! "$haystack" =~ $needle ]]; then
    log_error "Assertion failed: $message"
    log_error "Expected to contain: '$needle'"
    log_error "Actual:              '$haystack'"
    return 1
  fi
  log_debug "Assertion passed: $message"
  return 0
}

assert_not_empty() {
  local value="$1"
  local message="${2:-}"
  
  if [ -z "$value" ]; then
    log_error "Assertion failed: $message"
    log_error "Expected non-empty value, got empty string"
    return 1
  fi
  log_debug "Assertion passed: $message"
  return 0
}

assert_greater_than() {
  local actual="$1"
  local threshold="$2"
  local message="${3:-}"
  
  if [ "$actual" -le "$threshold" ]; then
    log_error "Assertion failed: $message"
    log_error "Expected greater than: $threshold"
    log_error "Actual:                $actual"
    return 1
  fi
  log_debug "Assertion passed: $message"
  return 0
}

assert_exit_code() {
  local exit_code=$?
  local expected="${1:-0}"
  local message="${2:-Command should succeed}"
  
  if [ "$exit_code" != "$expected" ]; then
    log_error "Assertion failed: $message"
    log_error "Expected exit code: $expected"
    log_error "Actual exit code:   $exit_code"
    return 1
  fi
  log_debug "Assertion passed: $message"
  return 0
}

# ============================================================================
# Kubernetes Helper Functions
# ============================================================================

wait_for_pod() {
  local namespace="$1"
  local pod_selector="$2"
  local timeout="${3:-300}"
  
  log_info "Waiting for pod '$pod_selector' in namespace '$namespace' to be ready..."
  
  if kubectl wait --for=condition=ready pod -l "$pod_selector" \
    -n "$namespace" --timeout="${timeout}s" 2>/dev/null; then
    log_success "Pod '$pod_selector' is ready"
    return 0
  else
    log_error "Timeout waiting for pod '$pod_selector'"
    return 1
  fi
}

wait_for_deployment() {
  local namespace="$1"
  local deployment="$2"
  local timeout="${3:-300}"
  
  log_info "Waiting for deployment '$deployment' in namespace '$namespace' to be ready..."
  
  if kubectl wait --for=condition=available deployment/"$deployment" \
    -n "$namespace" --timeout="${timeout}s" 2>/dev/null; then
    log_success "Deployment '$deployment' is ready"
    return 0
  else
    log_error "Timeout waiting for deployment '$deployment'"
    return 1
  fi
}

get_pod_name() {
  local namespace="$1"
  local selector="$2"
  
  kubectl get pods -n "$namespace" -l "$selector" \
    -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || echo ""
}

get_pod_count() {
  local namespace="$1"
  local selector="$2"
  
  kubectl get pods -n "$namespace" -l "$selector" \
    --field-selector=status.phase=Running \
    -o json 2>/dev/null | jq '.items | length' || echo "0"
}

exec_in_pod() {
  local namespace="$1"
  local pod="$2"
  local container="${3:-}"
  shift 3
  local cmd=("$@")
  
  if [ -n "$container" ]; then
    kubectl exec -n "$namespace" "$pod" -c "$container" -- "${cmd[@]}" 2>/dev/null
  else
    kubectl exec -n "$namespace" "$pod" -- "${cmd[@]}" 2>/dev/null
  fi
}

# ============================================================================
# Voiyd Helper Functions
# ============================================================================

voiydctl_exec() {
  local cmd=("$@")
  kubectl exec -n voiyd voiydctl -- voiydctl "${cmd[@]}" 2>/dev/null
}

voiydctl_get_tasks() {
  voiydctl_exec get tasks -o json 2>/dev/null || echo "[]"
}

voiydctl_get_task() {
  local task_name="$1"
  voiydctl_exec get tasks "$task_name" -o json 2>/dev/null || echo "{}"
}

voiydctl_get_nodes() {
  voiydctl_exec get nodes -o json 2>/dev/null || echo "[]"
}

voiydctl_run_task() {
  local task_name="$1"
  local image="$2"
  shift 2
  local extra_args=("$@")
  
  log_info "Creating task '$task_name' with image '$image'..."
  voiydctl_exec run "$task_name" --image "$image" "${extra_args[@]}"
}

voiydctl_stop_task() {
  local task_name="$1"
  
  log_info "Stopping task '$task_name'..."
  voiydctl_exec stop task "$task_name"
}

voiydctl_delete_task() {
  local task_name="$1"
  
  log_info "Deleting task '$task_name'..."
  voiydctl_exec delete task "$task_name" 2>/dev/null || true
}

wait_for_task_state() {
  local task_name="$1"
  local expected_state="$2"
  local timeout="${3:-60}"
  local elapsed=0
  
  log_info "Waiting for task '$task_name' to reach state '$expected_state'..."
  
  while [ $elapsed -lt $timeout ]; do
    local current_state
    current_state=$(voiydctl_get_task "$task_name" | jq -r '.status // "unknown"')
    
    if [ "$current_state" = "$expected_state" ]; then
      log_success "Task '$task_name' reached state '$expected_state'"
      return 0
    fi
    
    log_debug "Task state: $current_state (waiting for $expected_state)"
    sleep 2
    elapsed=$((elapsed + 2))
  done
  
  log_error "Timeout waiting for task '$task_name' to reach state '$expected_state'"
  return 1
}

wait_for_node_count() {
  local expected_count="$1"
  local timeout="${2:-120}"
  local elapsed=0
  
  log_info "Waiting for $expected_count voiyd nodes to be ready..."
  
  while [ $elapsed -lt $timeout ]; do
    local node_count
    node_count=$(voiydctl_get_nodes | jq '. | length')
    
    if [ "$node_count" -ge "$expected_count" ]; then
      log_success "$node_count voiyd nodes are ready"
      return 0
    fi
    
    log_debug "Node count: $node_count (waiting for $expected_count)"
    sleep 5
    elapsed=$((elapsed + 5))
  done
  
  log_error "Timeout waiting for $expected_count nodes"
  return 1
}

# ============================================================================
# Cleanup Functions
# ============================================================================

cleanup_task() {
  local task_name="$1"
  log_info "Cleaning up task '$task_name'..."
  voiydctl_stop_task "$task_name" 2>/dev/null || true
  voiydctl_delete_task "$task_name" 2>/dev/null || true
}

cleanup_all_tasks() {
  log_info "Cleaning up all tasks..."
  local tasks
  tasks=$(voiydctl_get_tasks | jq -r '.[].name // empty')
  
  for task in $tasks; do
    cleanup_task "$task"
  done
}

# ============================================================================
# Utility Functions
# ============================================================================

retry() {
  local max_attempts="$1"
  local delay="$2"
  shift 2
  local cmd=("$@")
  local attempt=1
  
  while [ $attempt -le $max_attempts ]; do
    log_debug "Attempt $attempt/$max_attempts: ${cmd[*]}"
    
    if "${cmd[@]}"; then
      return 0
    fi
    
    if [ $attempt -lt $max_attempts ]; then
      log_debug "Command failed, retrying in ${delay}s..."
      sleep "$delay"
    fi
    
    attempt=$((attempt + 1))
  done
  
  log_error "Command failed after $max_attempts attempts"
  return 1
}

random_string() {
  local length="${1:-8}"
  LC_ALL=C tr -dc 'a-z0-9' < /dev/urandom | head -c "$length"
}

# ============================================================================
# Test Result Tracking
# ============================================================================

E2E_RESULTS_DIR="${E2E_RESULTS_DIR:-hack/e2e/results}"
mkdir -p "$E2E_RESULTS_DIR"

record_test_result() {
  local test_name="$1"
  local result="$2"  # "PASS" or "FAIL"
  local duration="$3"
  local message="${4:-}"
  
  local timestamp
  timestamp=$(date '+%Y-%m-%d %H:%M:%S')
  
  echo "$timestamp|$test_name|$result|${duration}s|$message" >> "$E2E_RESULTS_DIR/results.log"
}

export -f log_info log_debug log_warn log_error log_success
export -f log_test_start log_test_end log_test_fail
export -f assert_equals assert_not_equals assert_contains assert_not_empty
export -f assert_greater_than assert_exit_code
export -f wait_for_pod wait_for_deployment get_pod_name get_pod_count exec_in_pod
export -f voiydctl_exec voiydctl_get_tasks voiydctl_get_task voiydctl_get_nodes
export -f voiydctl_run_task voiydctl_stop_task voiydctl_delete_task
export -f wait_for_task_state wait_for_node_count
export -f cleanup_task cleanup_all_tasks
export -f retry random_string record_test_result
