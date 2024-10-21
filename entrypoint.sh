#!/usr/bin/env bash

term_handler() {
  echo "Stopping process..."
  kill -TERM "$!"
  wait
  exit 0
}

trap 'term_handler' SIGINT SIGTERM

# Start Containerd
/usr/bin/containerd &
CONTAINERD_PID=$!

# Start server instance
/usr/local/bin/blipblop-server \
  --tls-key /blipblop/src/certs/server.key \
  --tls-certificate /blipblop/src/certs/server.crt \
  --tls-host 0.0.0.0 \
  --tcp-tls-host 0.0.0.0 &
SERVER_PID=$!

# Wait for server to start
sleep 2

# Start node service
/usr/local/bin/blipblop-node \
  --tls-ca /blipblop/src/certs/ca.crt \
  --port 5743 &
NODE_PID=$!

wait $CONTAINERD_PID
wait $SERVER_PID
wait $NODE_PID
