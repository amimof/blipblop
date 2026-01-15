Welcome to the voiyd documentation! This guide will help you get started with voiyd, a lightweight container orchestration platform designed for simplicity and ease of use.

<!--toc:start-->
- [Architecture](#architecture)
  - [Components](#components)
    - [voiyd-server](#voiyd-server)
    - [voiyd-node](#voiyd-node)
    - [voiydctl](#voiydctl)
- [Installation](#installation)
- [Configuration](#configuration)
<!--toc:end-->
---

## Architecture

### Components

#### voiyd-server

The central control plane that:

- Exposes gRPC/HTTP APIs for cluster management
- Stores cluster state using pluggable storage backends (BadgerDB, in-memory)
- Handles task scheduling and placement decisions
- Manages cluster events and log streaming
- Supports state replication for redundancy (in development)

**Supported Platforms**: Linux, macOS, Windows

#### voiyd-node

The agent that runs on each worker node:

- Establishes outbound connection to the server
- Subscribes to events and executes tasks
- Integrates with container runtimes (containerd, with more coming)
- Reports node status and metrics to the server
- Works behind NAT/firewalls (only requires outbound connectivity)

**Supported Platforms**: Linux (amd64, arm64, arm)

**Requirements**:

- Containerd >= 1.6
- iptables
- CNI plugins

#### voiydctl

The command-line interface for cluster management:

- Communicates only with the server (never directly with nodes)
- Supports multiple cluster contexts
- Provides intuitive commands for managing tasks, nodes, and volumes
- Streams logs and events in real-time

**Supported Platforms**: Linux, macOS, Windows

## Installation

Guides on how to install each of voiyds components

- Installing the control plane
- [Installing the nodes agents](/docs/install-node.md)
- Installing the CLI

## Configuration

Guides on how to configure voiyd clusters

- [TLS](/certs/README.md)
