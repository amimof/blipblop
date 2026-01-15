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

<!--toc:start-->
- [Features](#features)
- [Architecture](#architecture)
- [Components](#components)
- [Getting Started](#getting-started)
- [License](#license)
- [Contributing](#contributing)
<!--toc:end-->
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

## Architecture

```
┌─────────────┐
│  voiydctl   │
│   (CLI)     │
└──────┬──────┘
       │ gRPC/HTTPS
       ▼
┌─────────────────┐
│  voiyd-server   │
│ (Control Plane) │
└─────────────────┘
       ▲
       │ 
       │ Outbound gRPC
       │
  ┌────┴───┬────────┬────────┐
  │        │        │        │
┌─┴──┐   ┌─┴──┐   ┌─┴──┐   ┌─┴──┐
│Node│   │Node│   │Node│   │Node│
└────┘   └────┘   └────┘   └────┘
```

## Components

- `voiyd-server`  
  - Exposes gRPC/HTTP APIs for cluster management
  - Stores cluster state using pluggable storage backends (BadgerDB, in-memory)
  - Handles task scheduling and placement decisions
  - Manages cluster events and log streaming
  - Supports state replication for redundancy (in development)
- `voiyd-node`  
  - Runs on each node.
  - Subscribes to events and executes tasks
  - Establishes an outbound connection to the server and subscribes to events.  
  - Manages tasks using a runtime and reports status and metrics back to the server.  
  - Can operate behind NAT/firewalls as long as it can reach the server.
- `voiydctl`  
  - Communicates only with the server
  - Supports multiple clusters
  - Provides intuitive commands for managing tasks, nodes, and volumes
  - Streams logs and events in real-time

## Getting Started

See the [Installation Guide](/docs/installation.md) on how to setup and run voiyd clusters.

## License

voiyd is licensed under the Apache License, Version 2.0.
See the [`LICENSE`](./LICENSE) file for details.

## Contributing

See the [Contribution Guide](/CONTRIBUTING.md)
