#!/bin/bash

echo "============================================================"
echo "  Starting Blockchain Router (ARP Authority)"
echo "============================================================"
echo ""
echo "Network Configuration:"
ip addr show eth0
echo ""
echo "ARP Table (initial):"
ip neigh show
echo ""

# Start the router monitor
exec /app/router-monitor
