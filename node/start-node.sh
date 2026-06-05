#!/bin/bash

echo "============================================================"
echo "  Starting LAN Node: ${NODE_NAME} (${ROLE})"
echo "============================================================"
echo ""
echo "Network Configuration:"
ip addr show eth0
echo ""
echo "ARP Table (initial):"
ip neigh show
echo ""

exec /app/node-sync
