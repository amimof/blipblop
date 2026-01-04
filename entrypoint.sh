#!/usr/bin/env bash

term_handler() {
  echo "Stopping process..."
  kill -TERM "$!"
  wait
  exit 0
}

trap 'term_handler' SIGINT SIGTERM

# Start server instance
__run_server() {
  /usr/local/bin/voiyd-server \
    --tls-key /etc/voiyd/server.key \
    --tls-certificate /etc/voiyd/server.crt \
    --tls-host 0.0.0.0 \
    --tcp-tls-host 0.0.0.0 &
  SERVER_PID=$!

}

__run_node() {
  # Start node service
  /usr/local/bin/voiyd-node \
    --tls-ca /etc/voiyd/ca.crt \
    --port 5743 &
  NODE_PID=$!
}

__usage() {
  p="$(basename $0)"
  echo "usage: $p [server|node]"
}

case "$1" in
'server')
  __run_server
  wait $SERVER_PID
  ;;
'node')
  __run_node $1
  wait $NODE_PID
  ;;
*)
  __usage
  ;;
esac
