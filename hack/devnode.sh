#!/usr/bin/env bash

# SSH_HOST=$1
# HOST_ROLE=$2

__run_role() {
  if [[ $# < 2 ]]; then {
    __usage
    return
  } >&2
  fi
  local REMOTE_HOST=$1
  local REMOTE_ROLE=$2
  case "$REMOTE_ROLE" in
    'master')
      __run_master $REMOTE_HOST
      ;;
    'node')
      __run_node $REMOTE_HOST
      ;;
    'debug')
      __run_debug $REMOTE_HOST
      ;;
    'all')
      __run_master $REMOTE_HOST
      __run_node $REMOTE_HOST
      ;;
    *)
      echo "role must be one of [all,master,node,debug]"
  esac
}

__run_master() {
  local REMOTE_HOST=$1
  tmux new-window -c "#{pane_curent_path}" -n devnode-master ssh $REMOTE_HOST "cd go/blipblop; sudo /usr/local/go/bin/go run /home/amir/go/blipblop/cmd/blipblop-server/main.go --tls-key ./certs/server-key.pem --tls-certificate ./certs/server.pem --tls-host 0.0.0.0 --tcp-tls-host 0.0.0.0"
}

__run_node() {
  local REMOTE_HOST=$1
  tmux new-window -c "#{pane_curent_path}" -n devnode-node ssh $REMOTE_HOST "cd go/blipblop; sudo /usr/local/go/bin/go run /home/amir/go/blipblop/cmd/blipblop-node/main.go --node-name devnode"
}

__run_debug() {
  local REMOTE_HOST=$1
  tmux new-window -c "#{pane_curent_path}" -n devnode-debug ssh $REMOTE_HOST "cd go/blipblop; sudo -i"
}

__sync() {
  local REMOTE_HOST=$1
  rsync -avr --exclude .git*  ../blipblop 192.168.13.123:/home/amir/go
}

__killall() {
  local REMOTE_HOST=$1
  ssh $REMOTE_HOST "sudo killall go; sudo killall main"
}

__usage() {
    p="$(basename $0)"
    echo "usage:  $p [run|sync]"
}
case "$1" in
  'run')
    __run_role $2 $3
    ;;
  'sync')
    __sync $2
    ;;
  'killall')
    __killall $2
    ;;
  *) 
  __usage
esac

