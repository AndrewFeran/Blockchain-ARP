#!/bin/bash
# Stop the entire ARP Detection System

echo "============================================================"
echo "  ARP Tracker - Shutdown Script"
echo "============================================================"
echo ""

echo "🛑 Stopping tmux session..."
tmux kill-session -t arp-detection 2>/dev/null
if [ $? -eq 0 ]; then
    echo "✅ Tmux session stopped"
else
    echo "ℹ️  No tmux session running"
fi

echo ""
echo "🛑 Stopping Fabric network..."
cd ~/fabric/fabric-samples/test-network
./network.sh down

echo ""
echo "✅ All components stopped!"
echo ""