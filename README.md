[![Release](https://github.com/amimof/blipblop/actions/workflows/release.yaml/badge.svg)](https://github.com/amimof/blipblop/actions/workflows/release.yaml) [![Go](https://github.com/amimof/blipblop/actions/workflows/go.yaml/badge.svg)](https://github.com/amimof/blipblop/actions/workflows/go.yaml)

# blipblop

Distributed `containerd` workloads - an alternative to modern full-featured container orchestrators.

**Work in progress** _blipblop is still under active development and most features are still in an idéa phase. Please check in from time to time to follow the progress_ 🧡

## What is it?

Code name `blipblop` is a very simple container distribution plattform that allows you to run containerized workload on arbitrary Linux hosts. The architecture is very simple by design and contains only two components. Nodes, which are run on the Linux hosts that run the containers. And the server that clients, including the Nodes interact with.

Blipblop is an event-driven system which means that any interactions made with the server emits an event that clients within the cluster may react to. For example using `bbctl` you can `run` a container. The run command will publish a `ContainerCreate` event to the server. All nodes in the cluser may choose to react to that event, based on labels etc, to start the container on the host.

## System Requirements

- Linux
- [Containerd](https://containerd.io/downloads/) >= 1.4.13
- Iptables
- [CNI plugins](https://github.com/containernetworking/plugins)

## Installing

Download the latest binaries under [relases](https://github.com/amimof/blipblop/releases)

## Try it out

Run the server.

```shell
blipblop-server \
    --tls-key ./certs/server-key.pem \
    --tls-certificate ./certs/server.pem \
    --tls-host 0.0.0.0 \
    --tcp-tls-host 0.0.0.0 \
    --tls-port 8443
```

Run any number of node instances

```
blipblop-node \
    --node-name node-01 \
    --tls-host server-hostname.foo.com
```

Use `bbctl` to interact with the cluster

```
bbctl -s server-hostname.foo.com:5700 get nodes
```

## Contributing

You are welcome to contribute to this project by opening PR's. Create an Issue if you have feedback

NOTE: Run make help for more information on all potential make targets
