#!/bin/env bash
node_count=4

__up() {

  # Start server
  nerdctl run \
    -d \
    --name voiyd-server \
    --hostname voiyd-server \
    -v $PWD/certs:/etc/voiyd/tls \
    -p 5743:5743 \
    -p 8443:8443 \
    ghcr.io/amimof/voiyd:latest \
    voiyd-server \
    --tls-key /etc/voiyd/tls/server.key \
    --tls-certificate /etc/voiyd/tls/server.crt \
    --tls-ca /etc/voiyd/tls/ca.crt \
    --tls-host 0.0.0.0 \
    --tcp-tls-host 0.0.0.0

  # Start nodes
  for i in $(seq $node_count); do
    nerdctl run \
      -d \
      --name voiyd-node-$i \
      --hostname voiyd-node-$i \
      --privileged \
      -v /run/containerd/containerd.sock:/run/containerd/containerd.sock \
      -v $PWD/certs:/etc/voiyd/tls \
      -v /tmp:/tmp \
      -v /run/containerd:/run/containerd \
      -v /var/lib/containerd:/var/lib/containerd \
      ghcr.io/amimof/voiyd:latest \
      voiyd-node \
      --tls-ca /etc/voiyd/tls/ca.crt \
      --port 5743 \
      --host voiyd-server \
      --insecure-skip-verify
  done

}

__down() {
  # Kill & remove the server
  nerdctl kill voiyd-server
  nerdctl rm voiyd-server

  # Kill & remove nodes
  for i in $(seq $node_count); do
    nerdctl kill voiyd-node-$i
    nerdctl rm voiyd-node-$i
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
  ;;
esac
