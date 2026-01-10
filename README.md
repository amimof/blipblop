
<p align="center">
  <img src="./logo/voiyd_logo_white_with_text.png" width="450"/>
</p>

<p align="center">
  Lightweight, event‑driven orchestration for container workloads
  <br/>
  <a href="https://voiyd.io">voiyd.io</a>
</p>

---

[![Go Reference](https://pkg.go.dev/badge/github.com/amimof/voiyd.svg)](https://pkg.go.dev/github.com/amimof/voiyd)
[![Release](https://github.com/amimof/voiyd/actions/workflows/release.yaml/badge.svg)](https://github.com/amimof/voiyd/actions/workflows/release.yaml)
[![Go](https://github.com/amimof/voiyd/actions/workflows/go.yaml/badge.svg)](https://github.com/amimof/voiyd/actions/workflows/go.yaml)
[![Go Report Card](https://goreportcard.com/badge/github.com/amimof/voiyd)](https://goreportcard.com/report/github.com/amimof/voiyd)
![License: Apache-2.0](https://img.shields.io/github/license/kyverno/kyverno?color=blue)

Voiyd is a lightweight container orchestration platform with a central server and agent nodes. It lets you schedule and manage containers across many number of arbitrary Linux hosts using a simple CLI. It’s designed to be small, understandable, and easy to run on your own infrastructure.

> **Note**: This project is under active early development and unstable. Features, APIs, and behavior are subject to change at any time and may not be backwards compatible between versions. Expect breaking changes.

## Features

- **Central control plane**: voiyd-server provides the API and manages cluster state.
- **Node agent**: voiyd-node runs on each worker node and integrates with a *Runtime* such as [containerd](https://containerd.io). More runtimes are beeing added for example the *Exec* runtime for legacy applications.
- **Task orchestration**: Deploy workload with *Tasks* - the unit of scheduling.
- **Volume management**: Create and attach host-local volumes. Snapshot and template support (where configured).
- **Networking**: Expose services with built-in [CNI](https://www.cni.dev) support.
- **Scheduling**: Built-in scheduler for placing *Tasks* on nodes. Horizontal scheduling utilities for multi-node clusters.
- **Event and log streaming**: Event service for cluster events. Log service for streaming container logs.
- **Node management and upgrades**: Node upgrade support and associated controllers.
- **Pluggable storage backends**: BadgerDB-based repository implementation. In-memory repositories for testing.
- **CLI-focused**: Use voiydctl to manage clusters.
- **Instrumentation**: Metrics and tracing hooks in pkg/instrumentation.

## Components

- `voiyd-server`  
  - Exposes gRPC/HTTP APIs defined in `api/`.  
  - Stores cluster resources via the repository layer.  
  - State replication for redundancy is in development.
- `voiyd-node`  
  - Runs on each node.
  - Establishes an outbound connection to the server and subscribes to events.  
  - Manages tasks using a runtime and reports status and metrics back to the server.  
  - Can operate behind NAT/firewalls as long as it can reach the server.
- `voiydctl`  
  - Talks only to the server and never connects directly to nodes.
  - Multi-cluster support with the use of contexts.

## Prerequisites

voiyd-server and voiydctl can run on pretty much any platform whereas voiyd-node requires Linux with the following requirements:

- [Containerd](https://containerd.io/downloads/) >= 1.6
- Iptables
- [CNI plugins](https://github.com/containernetworking/plugins)

## Getting Started

You can install voiyd either from source or using pre-built binaries from [releases](https://github.com/amimof/voiyd/releases).

### Install From Releases

1. Go to [releases](https://github.com/amimof/voiyd/releases).
2. Download the binary for your platform.  
3. Make it executable and move it into your `PATH`

   ```bash
   chmod +x voiydctl voiyd-server voiyd-node
   sudo mv voiydctl voiyd-server voiyd-node /usr/local/bin/
    ```

To download untagged `latest` binaries: [nightly.link](https://nightly.link/amimof/voiyd/workflows/upload.yaml/master?preview)  

### Build From Source

```bash
git clone https://github.com/amimof/voiyd.git
cd voiyd
make
```

To build a specific binary you may run `make voiydctl`, `make node` or `make server`. Use env variables to specify target os, architecture and binary name with `GOOS`, `GOARCH` and `BINARY_NAME`. For example:

```bash
GOOS=windows GOARCH=amd64 BINARY_NAME=voiyd-server-windows-amd64.exe make server
```

Alternatively, use `go install` directly:

```bash
go install github.com/amimof/voiyd/cmd/voiyd-node
go install github.com/amimof/voiyd/cmd/voiyd-server
go install github.com/amimof/voiyd
```

NOTE: Run make help for more information on all potential make targets

## Quick Start

1. Generate certificates. See [instruction here](/certs/README.md). Alternatively you may use pre-generated development certificates under `./certs`. These certificates are for testing purposes only!

2. Start the server

    ```bash
    voiyd-server \
        --tls-key ./certs/server.key \
        --tls-certificate ./certs/server.crt \
        --tls-ca ./certs/ca.crt \
        --tls-host 0.0.0.0 \
        --tcp-tls-host 0.0.0.0
    ```

3. Run any number of node instances

    ```bash
    voiyd-node \
        --tls-ca ./certs/ca.crt \
        --port 5743
    ```

4. Create a `voiydctl` configuration

    ```bash
    voiydctl config init
    voiydctl config create-server dev --address localhost:5743 --tls --ca ./certs/ca.crt
    ```

5. Run a `Task`

    ```bash
    voiydctl run victoria-metrics --image docker.io/victoriametrics/victoria-metrics:v1.130.0
    ```

## License

voiyd is licensed under the Apache License, Version 2.0.
See the [`LICENSE`](./LICENSE) file for details.

## Contributing

You are welcome to contribute to this project by opening PR's. Create an Issue if you have feedback
