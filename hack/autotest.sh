#!/bin/env bash
count=5

__up() {

  # Start server
  nerdctl run \
    -d \
    --name blipblop-server \
    -v /etc/blipblop/tls:/etc/blipblop/tls \
    -p 5743:5743 \
    ghcr.io/amimof/blipblop:latest \
    blipblop-server \
    --tls-key /etc/blipblop/tls/server.key \
    --tls-certificate /etc/blipblop/tls/server.crt \
    --tls-host 0.0.0.0 \
    --tcp-tls-host 0.0.0.0

  # Start nodes
  for i in $(seq $count); do
    nerdctl run \
      -d \
      --name blipblop-node-$i \
      --privileged \
      -v /run/containerd/containerd.sock:/run/containerd/containerd.sock \
      -v /etc/blipblop/tls:/etc/blipblop/tls \
      -v /tmp:/tmp \
      -v /run/containerd:/run/containerd \
      -v /var/lib/containerd:/var/lib/containerd \
      ghcr.io/amimof/blipblop:latest \
      blipblop-node \
      --tls-ca /etc/blipblop/tls/ca.crt \
      --port 5743 \
      --host blipblop-server \
      --insecure-skip-verify
  done

}

__down() {
  # Kill & remove the server
  nerdctl kill blipblop-server
  nerdctl rm blipblop-server

  # Kill & remove nodes
  for i in $(seq $count); do 
    nerdctl kill blipblop-node-$i
    nerdctl rm blipblop-node-$i
  done

}

__usage() {
    p="$(basename $0)"
    echo "usage: $p [up|down]"
}

case "$1" in
  'up')
    __up
    ;;
  'down')
    __down
    ;;
  *) 
  __usage
esac
