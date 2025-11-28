#!/bin/bash
# Stop the entire ARP Detection System

echo "============================================================"
echo "  ARP Tracker - Shutdown Script"
echo "============================================================"
echo ""

echo "🛑 Stopping Fabric network..."
cd ~/fabric/fabric-samples/test-network
./network.sh down

echo ""
echo "✅ All components stopped!"
echo ""