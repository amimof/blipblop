#!/bin/env bash

__flush() {
  iptables -F -t nat
  iptables -X -t nat
  rm -rf /var/lib/cni/
}

__list() {
  iptables -vL -t nat
}

__usage() {
  p="$(basename $0)"
  echo "usage: $p [flush|list]"
}

case "$1" in
'flush')
  __flush
  ;;
'list')
  __list
  ;;
*)
  __usage
  ;;
esac
