Thank you for your interest in contributing to voiyd! This document provides guidelines and instructions for contributing to the project.

<!--toc:start-->
- [Getting Started](#getting-started)
  - [Prerequisites](#prerequisites)
  - [Fork and Clone](#fork-and-clone)
  - [Build the Project](#build-the-project)
  - [Run Locally](#run-locally)
- [Go Code Style](#go-code-style)
- [Testing](#testing)
- [Questions?](#questions)
<!--toc:end-->

## Getting Started

### Prerequisites

Before you begin, ensure you have the following installed:

- Go 1.21 or higher
- Git
- Make

Because `voiyd-node` requires Linux, this is generally the best platform to develop voiyd on. However other platforms work but might have some limitations. Mainly because you have to run the node in a VM.  I'm on MacOS myself and use [lima-vm](https://lima-vm.io/) which works verry well.

### Fork and Clone

1. Fork the repository on GitHub
2. Clone your fork locally:

```bash
git clone https://github.com/YOUR_USERNAME/voiyd.git
cd voiyd
```

1. Add the upstream repository as a remote:

```bash
git remote add upstream https://github.com/amimof/voiyd.git
```

1. Verify your remotes:

```bash
git remote -v
```

### Build the Project

```bash
# Build all components
make all

# Or build specific components
make server    # Build voiyd-server
make node      # Build voiyd-node
make voiydctl  # Build voiydctl
```

Binaries will be created in the `bin/` directory.

### Run Locally

```bash
# Start the server
go run cmd/voiyd-server/main.go \
  --tls-key ./certs/server.key \
  --tls-certificate ./certs/server.crt \
  --tls-ca ./certs/ca.crt \
  --log-level debug

# Start a node
go run cmd/voiyd-node/main.go \ 
  --tls-ca ./certs/ca.crt \
  --port 5743 \
  --log-level debug

# Use voiydctl
go run main.go config init
go run main.go config create-server dev --address localhost:5743 --tls --ca ./certs/ca.crt
go run main.go get nodes
```

## Go Code Style

- Follow standard Go conventions and idioms
- Use `gofmt` for code formatting (automatically done with `make fmt`)
- Run `golangci-lint` before submitting (automatically done with `make lint`)

## Testing

```bash
# Run all tests
make test

# Run tests for specific package
go test ./pkg/scheduler/...

# Run tests with coverage
go test -cover ./...

# Run tests with verbose output
go test -v ./...
```

## Questions?

- General questions: Open a [GitHub Discussion](https://github.com/amimof/voiyd/discussions)
- Bug reports & Feature requests: Open a [GitHub Issue](https://github.com/amimof/voiyd/issues)

---

Thank you for contributing to voiyd! 🚀
