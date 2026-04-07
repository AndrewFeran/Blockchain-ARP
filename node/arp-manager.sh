#!/bin/bash
# Helper script to manually manage ARP entries

ACTION=$1
IP=$2
MAC=$3

case "$ACTION" in
  "add")
    echo "Adding ARP entry: $IP -> $MAC"
    ip neigh replace "$IP" lladdr "$MAC" dev eth0 nud permanent
    ;;
  "del")
    echo "Deleting ARP entry: $IP"
    ip neigh del "$IP" dev eth0
    ;;
  "show")
    echo "Current ARP table:"
    ip neigh show
    ;;
  "flush")
    echo "Flushing ARP table..."
    ip neigh flush all
    ;;
  *)
    echo "Usage: $0 {add|del|show|flush} [IP] [MAC]"
    echo ""
    echo "Examples:"
    echo "  $0 add 10.5.0.10 aa:bb:cc:dd:ee:01"
    echo "  $0 del 10.5.0.10"
    echo "  $0 show"
    echo "  $0 flush"
    exit 1
    ;;
esac
