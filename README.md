[![Go Reference](https://pkg.go.dev/badge/github.com/amimof/blipblop.svg)](https://pkg.go.dev/github.com/amimof/blipblop) [![Release](https://github.com/amimof/blipblop/actions/workflows/release.yaml/badge.svg)](https://github.com/amimof/blipblop/actions/workflows/release.yaml) [![Go](https://github.com/amimof/blipblop/actions/workflows/go.yaml/badge.svg)](https://github.com/amimof/blipblop/actions/workflows/go.yaml)

# blipblop

Blipblop is a lightweight container orchestration platform with a central server and agent nodes. It lets you schedule and manage containers across many number of arbitrary Linux hosts using a simple CLI (voiydctl) and a gRPC/HTTP API. It’s designed to be small, understandable, and easy to run on your own infrastructure.

## Features

- **Central control plane**: blipblop-server provides the API and manages cluster state.
- **Node agent**: blipblop-node runs on each worker node and integrates with containerd. More runtimes are beeing added.
- **Container management**: Create, run, start, stop, update, and delete containers. Manage “containersets” as grouped workloads.
- **Volume management**: Create and attach host-local volumes. Snapshot and template support (where configured).
- **Scheduling**: Built-in scheduler for placing workloads on nodes. Horizontal scheduling utilities for multi-node clusters.
- **Event and log streaming**: Event service for cluster events (create/update/delete, etc). Log service for streaming container logs.
- **Node management and upgrades**: Register and list nodes. Node upgrade support and associated controllers.
- **Pluggable storage backends**: BadgerDB-based repository implementation. In-memory repositories for testing.
- **CLI-focused**: Use voiydctl to manage clusters.
- **Instrumentation**: Metrics and tracing hooks in pkg/instrumentation.

## Architecture Overview

Blipblop is an event-driven system which means that any interactions made with the server emits an event that clients within the cluster may react to. For example using `voiydctl` you can `run` a container. The run command will publish a `ContainerCreate` event to the server. All nodes in the cluser may choose to react to that event, based on labels etc, to start the container on the host. The system consists of three main pieces:

- **Control plane**: `blipblop-server`
  - Exposes gRPC/HTTP APIs defined in api/.
  - Persists containers, nodes, volumes, and events via the repository layer.
- **Node agent**: `blipblop-node`
  - Runs on each node.
  - Talks to containerd (pkg/runtime, pkg/containerd) to manage containers.
  - Reports status and metrics back to the server.
- **CLI**: `voiydctl`
  - Used by operators to interact with the server and nodes.
  - Provides subcommands for create/get/apply/delete, logs, upgrade, etc.

## Prerequisites

blipblop-server and voiydctl can run on pretty much any platform whereas blipblop-node requires Linux with the following requirements:

- Linux
- [Containerd](https://containerd.io/downloads/) >= 1.6
- Iptables
- [CNI plugins](https://github.com/containernetworking/plugins)

## Getting Started

You can install Blipblop either from source or using pre-built binaries from [releases](https://github.com/amimof/blipblop/releases).

### Install From Releases

1. Go to [releases](https://github.com/amimof/blipblop/releases).
2. Download the binary for your platform.  
3. Make it executable and move it into your `PATH`

   ```bash
   chmod +x voiydctl blipblop-server blipblop-node
   sudo mv voiydctl blipblop-server blipblop-node /usr/local/bin/
    ```

### Build From Source

```bash
git clone https://github.com/amimof/blipblop.git
cd blipblop
make
```

To build a specific binary you may run `make voiydctl`, `make node` or `make server`. Use env variables to specify target os, architecture and binary name with `GOOS`, `GOARCH` and `BINARY_NAME`. For example:

```bash
GOOS=windows GOARCH=amd64 BINARY_NAME=blipblop-server-windows-amd64.exe make server
```

Alternatively, use `go install` directly:

```bash
go install github.com/amimof/blipblop/cmd/blipblop-node
go install github.com/amimof/blipblop/cmd/blipblop-server
go install github.com/amimof/blipblop
```

NOTE: Run make help for more information on all potential make targets

## Quick Start

1. Generate certificates. See [instruction here](/certs/README.md). Alternatively you may use pre-generated development certificates under `./certs`. These certificates are for testing purposes only!

2. Start the server

    ```bash
    blipblop-server \
        --tls-key ./certs/server.key \
        --tls-certificate ./certs/server.crt \
        --tls-ca ./certs/ca.crt \
        --tls-host 0.0.0.0 \
        --tcp-tls-host 0.0.0.0
    ```

3. Run any number of node instances

    ```bash
    blipblop-node \
        --tls-ca ./certs/ca.crt \
        --port 5743
    ```

4. Create a `voiydctl` configuration

    ```bash
    voiydctl config init
    voiydctl config create-server dev --address localhost:5743 --tls --ca ./certs/ca.crt
    ```

5. Run a container

    ```bash
    voiydctl run victoria-metrics --image docker.io/victoriametrics/victoria-metrics:v1.130.0
    ```

## License

blipblop is licensed under the Apache License, Version 2.0.
See the [`LICENSE`](./LICENSE) file for details.

## Contributing

You are welcome to contribute to this project by opening PR's. Create an Issue if you have feedback
