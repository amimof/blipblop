[🏠 Home](/docs/README.md)

# Quick Start

<!--toc:start-->
- [Quick Start](#quick-start)
  - [Prerequisites](#prerequisites)
  - [Your First Cluster](#your-first-cluster)
  - [Running Your First Task](#running-your-first-task)
  - [Next Steps](#next-steps)
<!--toc:end-->

Get your first voiyd cluster up and running in minutes. This guide walks you through installing the control plane, running a node, configuring the CLI, and deploying your first task.

## Prerequisites

- Linux system (Ubuntu/Debian, CentOS/RHEL/Fedora, or Arch Linux)
- Root or sudo access
- Internet connection

## Your First Cluster

1. **Install `voiyd-server`**

   ```bash
   curl -sfL https://raw.githubusercontent.com/amimof/voiyd/master/setup-server.sh | sudo bash -
   ```

2. **Install as many `voiyd-nodes` as you wish**

   ```bash
   curl -sfL https://raw.githubusercontent.com/amimof/voiyd/master/setup-node.sh | sudo bash -
   ```

   If installing on a different machine, specify the server address:

   ```bash
   curl -sfL https://raw.githubusercontent.com/amimof/voiyd/master/setup-server.sh | \
    sudo bash -s -- --server-address YOUR_SERVER_IP:5743
   ```

3. **Install `voiydctl`**

   ```bash
   # MacOS
   brew tap amimof/voiyd
   brew install voiydctl

   # Manual
   curl -LO https://github.com/amimof/voiyd/releases/download/v0.0.11/voiydctl-linux-amd64
   sudo install -m 755 voiydctl-linux-amd64 /usr/local/bin/voiydctl
   ```

   See [Releases](https://github.com/amimof/voiyd/releases) for all available versions

   > Use this [nighly.link](https://nightly.link/amimof/voiyd/workflows/upload.yaml/master?preview) do download latest unstable releases

4. **Initialize `voiydctl` config**

   ```bash
   voiydctl config init
   voiydctl config create-server my-cluster --address YOUR_SERVER_IP:5743 --tls --insecure
   ```

5. **List the nodes**

   ```bash
   voiydctl get nodes
   ```

   You should see something similar to

   ```
   NAME                                    REVISION        STATE           VERSION         AGE
   lima-lima-debian-12-secondary           1               ready           v0.0.11         32m
   ```

🎉 You have successfully created a voiyd cluster! The next step is to deploy some workload.

## Running Your First Task

Now that you have a cluster up and running you can start running `tasks` on it

1. **Run a `victoria-metrics` task**

   ```bash
   # Provision a victoria-metrics task
   voiydctl run victoria-metrics \
    --image docker.io/victoriametrics/victoria-metrics:v1.130.0 \
    --port 9090:9090
   
   # List tasks
   voiydctl get tasks

   # View task details
   voiydctl get tasks victoria-metrics

   # victoria-metrics is available using the IP address 
   # of the node running task.
   curl http://YOUR_NODE_IP:9090
   ```

2. **Stop the task**

   ```bash
   voiydctl stop task victoria-metrics
   ```

3. **View some logs**

   ```bash
   voiydctl logs victoria-metrics
   ```

## Next Steps

- For a more detailed installation guide see the [Installation Guide](/docs/installation/README.md).
- To learn more read the [Documentation](/docs/README.md)
