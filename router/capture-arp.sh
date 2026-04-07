#!/bin/bash
# Helper script to manually capture ARP traffic for debugging

echo "Capturing ARP traffic on eth0..."
echo "Press Ctrl+C to stop"
echo ""

tcpdump -i eth0 -e -n arp
