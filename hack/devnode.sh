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
    'all')
      __run_master $REMOTE_HOST
      __run_node $REMOTE_HOST
      ;;
    *)
      echo "role must be one of [all,master,node]"
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

__debug() {
  local REMOTE_HOST=$1
  local END=$2
  local CURRENT_SESSION=$(tmux display-message -p '#S')
  tmux new-window -c "#{pane_current_path}" -n devnode-debug \; split-window -h
  # for i in $(seq 1 $END); do
    tmux send-keys -t $CURRENT_SESSION:devnode-debug.0 "ssh -t $REMOTE_HOST 'cd go/blipblop; sudo -s; exec \$SHELL'" C-m
    tmux send-keys -t $CURRENT_SESSION:devnode-debug.1 "ssh -t $REMOTE_HOST 'cd go/blipblop; sudo -s; exec \$SHELL'" C-m
    # tmux new-window -c "#{pane_current_path}" -n devnode-debug-$i ssh -t $REMOTE_HOST "cd go/blipblop; sudo -s; exec \$SHELL"
  # done
}

__sync() {
  local REMOTE_HOST=$1
  rsync -avr --exclude .git*  ../blipblop $REMOTE_HOST:/home/amir/go
}

__killall() {
  local REMOTE_HOST=$1
  ssh $REMOTE_HOST "sudo killall go; sudo killall main"
}

__build() {
  local REMOTE_HOST=$1
  ssh $REMOTE_HOST "cd go/blipblop; make; sudo mv bin/* /usr/local/bin/"
}

__usage() {
    p="$(basename $0)"
    echo "usage:  $p HOST [run|sync|killall]"
}
case "$2" in
  'run')
    __run_role $1 $3
    ;;
  'sync')
    __sync $1
    ;;
  'killall')
    __killall $1
    ;;
  'debug')
    __debug $1
    ;;
  'build')
    __build $1
    ;;
  *) 
  __usage
esac

