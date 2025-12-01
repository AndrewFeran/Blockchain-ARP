#!/bin/bash
# Cleanup Demo - Stop all services and clean up

echo "============================================================"
echo "  🧹 Cleaning up Blockchain-ARP Demo"
echo "============================================================"
echo ""

# Stop monitoring containers
echo "⏹️  Stopping monitoring containers..."
cd ~/fabric/arp-chaincode
docker-compose -f docker-compose-monitors.yaml down
docker rm -f monitor-org1 monitor-org2 monitor-org3 2>/dev/null
docker rm -f traffic-org1 traffic-org2 traffic-org3 2>/dev/null
docker rm -f arp-attacker 2>/dev/null

# Stop event listener and dashboard
echo "⏹️  Stopping event listener and dashboard..."
pkill -f "event-listener.go" 2>/dev/null
pkill -f "dashboard/app.py" 2>/dev/null

# Remove ARP test LAN network
echo "🌐 Removing ARP test LAN network..."
docker network rm arp-test-lan 2>/dev/null

# Stop Fabric network
echo "⏹️  Stopping Fabric network..."
cd ~/fabric/fabric-samples/test-network
./network.sh down

# Clean up volumes
echo "🧹 Cleaning up Docker volumes..."
docker volume prune -f

echo ""
echo "============================================================"
echo "✅ Cleanup complete!"
echo "============================================================"
echo ""
echo "To restart the demo, run: bash scripts/demo.sh"
echo ""
