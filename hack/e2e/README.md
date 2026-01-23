# Voiyd End-to-End (E2E) Tests

Comprehensive end-to-end testing suite for the Voiyd distributed container orchestration system.

## Overview

This directory contains the complete E2E testing framework for Voiyd, designed to test distributed system behaviors including:

- Multi-node task scheduling and placement
- Node failure and recovery scenarios
- Network partitions and split-brain scenarios
- Concurrent operations and race conditions
- Full workflow end-to-end testing
- Volume management
- Event and log streaming

## Architecture

The E2E tests run on a **Kind (Kubernetes in Docker)** cluster with:
- 1 Kubernetes control plane node
- 3 worker nodes (each running voiyd-server and voiyd-node components)
- Isolated Docker containers simulating separate machines

## Directory Structure

```
hack/e2e/
├── README.md                  # This file
├── kind-config.yaml          # Kind cluster configuration
├── manifests/                # Kubernetes manifests for voiyd components
│   ├── 01-namespace.yaml
│   ├── 02-certs.yaml
│   ├── 03-server.yaml
│   ├── 04-nodes.yaml
│   └── 05-voiydctl.yaml
├── scripts/                  # Helper and orchestration scripts
│   ├── helpers.sh           # Common functions and utilities
│   ├── setup-cluster.sh     # Create and configure Kind cluster
│   ├── deploy-voiyd.sh      # Deploy voiyd components
│   ├── wait-for-ready.sh    # Wait for components to be ready
│   ├── run-all-tests.sh     # Execute all test suites
│   └── teardown-cluster.sh  # Cleanup cluster
├── tests/                    # Individual test scripts
│   ├── 01-basic-workflow.sh
│   ├── 02-multi-node-scheduling.sh
│   ├── 03-node-failure.sh
│   ├── 04-node-recovery.sh
│   ├── 05-network-partition.sh
│   ├── 06-concurrent-operations.sh
│   ├── 07-volume-management.sh
│   ├── 08-event-streaming.sh
│   └── 09-log-streaming.sh
└── results/                  # Test results and logs (created at runtime)
```

## Prerequisites

### Required Tools

- **Docker** - For building images and running Kind
- **kind** - Kubernetes in Docker ([installation guide](https://kind.sigs.k8s.io/docs/user/quick-start/#installation))
- **kubectl** - Kubernetes CLI ([installation guide](https://kubernetes.io/docs/tasks/tools/))
- **Go 1.24.3+** - For building voiyd binaries
- **jq** - For JSON processing
- **openssl** - For generating TLS certificates (optional, uses placeholders if not available)

### Install Kind

```bash
# On macOS
brew install kind

# On Linux
curl -Lo ./kind https://kind.sigs.k8s.io/dl/v0.20.0/kind-linux-amd64
chmod +x ./kind
sudo mv ./kind /usr/local/bin/kind
```

### Install kubectl

```bash
# On macOS
brew install kubectl

# On Linux
curl -LO "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl"
chmod +x kubectl
sudo mv kubectl /usr/local/bin/
```

## Running E2E Tests

### Quick Start (All-in-One)

Run the complete E2E test suite:

```bash
# From project root
cd hack/e2e

# Setup cluster, deploy voiyd, and run all tests
bash scripts/setup-cluster.sh && \
bash scripts/deploy-voiyd.sh && \
bash scripts/wait-for-ready.sh && \
bash scripts/run-all-tests.sh

# Cleanup when done
bash scripts/teardown-cluster.sh
```

### Step-by-Step Execution

For more control, run each step individually:

```bash
# 1. Create Kind cluster
bash scripts/setup-cluster.sh

# 2. Deploy voiyd components
bash scripts/deploy-voiyd.sh

# 3. Wait for components to be ready
bash scripts/wait-for-ready.sh

# 4. Run all tests
bash scripts/run-all-tests.sh

# 5. Cleanup
bash scripts/teardown-cluster.sh
```

### Running Individual Tests

Execute a specific test:

```bash
# Ensure cluster is ready first
bash scripts/wait-for-ready.sh

# Run a single test
bash tests/01-basic-workflow.sh

# Or run multiple specific tests
bash tests/02-multi-node-scheduling.sh
bash tests/03-node-failure.sh
```

### Running in GitHub Actions

The E2E tests run automatically in GitHub Actions on:
- **Push to master branch**
- **Pull requests to master**
- **Nightly schedule** (2 AM UTC)
- **Manual trigger** (workflow_dispatch)

See `.github/workflows/e2e.yaml` for the complete workflow.

## Test Descriptions

### 01-basic-workflow.sh
Tests the complete task lifecycle:
- Create task
- Verify task is running
- Verify node assignment
- Stop task
- Delete task
- Verify cleanup

### 02-multi-node-scheduling.sh
Tests task distribution across multiple nodes:
- Deploy 6 tasks
- Verify tasks are distributed across 3 nodes
- Ensure basic load balancing

### 03-node-failure.sh
Tests system behavior during node failures:
- Deploy task
- Identify node running the task
- Simulate node failure by deleting pod
- Verify cluster stability

### 04-node-recovery.sh
Tests node recovery after restart:
- Restart a node pod
- Verify pod is recreated
- Verify node re-registers with server
- Verify cluster health

### 05-network-partition.sh
Tests behavior during network partitions:
- Create network partition using iptables
- Observe system behavior
- Restore connectivity
- Verify recovery

### 06-concurrent-operations.sh
Tests concurrent operations and race conditions:
- Deploy 10 tasks simultaneously
- Stop all tasks concurrently
- Delete all tasks concurrently
- Verify cluster stability

### 07-volume-management.sh
Tests volume creation and attachment:
- Create volume
- Attach volume to task
- Verify task runs with volume
- Cleanup

### 08-event-streaming.sh
Tests event streaming functionality:
- Start event stream
- Generate events by creating/stopping tasks
- Verify events are captured

### 09-log-streaming.sh
Tests log streaming from tasks:
- Create task
- Wait for logs to be generated
- Retrieve and verify logs

## Configuration

### Environment Variables

```bash
# Cluster name (default: voiyd-e2e)
export CLUSTER_NAME="my-e2e-cluster"

# Image tag (default: e2e-test)
export IMAGE_TAG="custom-tag"

# Continue running tests even if one fails (default: true)
export CONTINUE_ON_FAILURE=true

# Cleanup tasks between tests (default: true)
export CLEANUP_BETWEEN_TESTS=true

# Enable debug logging (default: false)
export DEBUG=true

# Timeout for waiting (default: 600 seconds)
export TIMEOUT=900
```

### Customizing Tests

To add a new test:

1. Create a new script in `tests/` with a numbered prefix (e.g., `10-my-new-test.sh`)
2. Make it executable: `chmod +x tests/10-my-new-test.sh`
3. Use the helper functions from `scripts/helpers.sh`
4. Follow the pattern of existing tests

Example test template:

```bash
#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/../scripts/helpers.sh"

TEST_NAME="My New Test"

log_test_start "$TEST_NAME"

# Your test logic here

log_test_end "$TEST_NAME"
exit 0
```

## Helper Functions

The `scripts/helpers.sh` file provides many useful functions:

### Logging
- `log_info "message"` - Info level logging
- `log_error "message"` - Error level logging
- `log_success "message"` - Success message
- `log_warn "message"` - Warning message
- `log_debug "message"` - Debug logging (only if DEBUG=true)

### Assertions
- `assert_equals actual expected "message"`
- `assert_not_equals actual not_expected "message"`
- `assert_contains haystack needle "message"`
- `assert_not_empty value "message"`
- `assert_greater_than actual threshold "message"`

### Kubernetes
- `wait_for_pod namespace selector timeout`
- `wait_for_deployment namespace deployment timeout`
- `get_pod_name namespace selector`
- `get_pod_count namespace selector`

### Voiyd Operations
- `voiydctl_exec command args...`
- `voiydctl_get_tasks`
- `voiydctl_get_task task_name`
- `voiydctl_get_nodes`
- `voiydctl_run_task task_name image [args...]`
- `voiydctl_stop_task task_name`
- `voiydctl_delete_task task_name`
- `wait_for_task_state task_name expected_state timeout`
- `wait_for_node_count expected_count timeout`

### Cleanup
- `cleanup_task task_name`
- `cleanup_all_tasks`

## Troubleshooting

### Cluster Creation Fails

```bash
# Check if Kind is installed
kind version

# Check if Docker is running
docker ps

# Delete any existing cluster
kind delete cluster --name voiyd-e2e
```

### Tests Hang or Timeout

```bash
# Check cluster status
kubectl get nodes
kubectl get pods -n voiyd

# Check voiyd-server logs
kubectl logs -n voiyd -l app=voiyd-server

# Check voiyd-node logs
kubectl logs -n voiyd -l app=voiyd-node -c node

# Increase timeout
export TIMEOUT=1200
```

### Permission Errors in Network Partition Test

The network partition test requires privileged containers. If it fails:

```bash
# Check if pods are running with privilege
kubectl get pods -n voiyd -o jsonpath='{.items[*].spec.containers[*].securityContext}'

# The test will skip if iptables cannot be modified
```

### Accessing the Cluster Manually

```bash
# Get cluster info
kubectl cluster-info --context kind-voiyd-e2e

# Access voiydctl pod
kubectl exec -it -n voiyd voiydctl -- /bin/bash

# Inside the pod, run voiydctl commands
voiydctl get nodes
voiydctl get tasks
```

## CI/CD Integration

The tests are integrated with GitHub Actions. See `.github/workflows/e2e.yaml`.

### Viewing Results

After a workflow run:
1. Go to GitHub Actions tab
2. Select the E2E Tests workflow
3. Click on a run
4. Download the `e2e-test-results` artifact for detailed logs

## Performance Considerations

- **Full test suite**: ~15-30 minutes
- **Cluster setup**: ~3-5 minutes
- **Individual test**: ~1-3 minutes

## Contributing

When adding new tests:
1. Follow the naming convention: `NN-descriptive-name.sh`
2. Use helper functions from `helpers.sh`
3. Include proper logging and assertions
4. Clean up resources after test completion
5. Handle both success and failure cases gracefully
6. Update this README with test description

## License

Apache License 2.0 - See LICENSE file for details
