#!/bin/bash
# Cleanup Demo - Stop all services and clean up THOROUGHLY

echo "============================================================"
echo "  🧹 Cleaning up Blockchain-ARP Demo (DEEP CLEAN)"
echo "============================================================"
echo ""

# Stop monitoring containers
echo "⏹️  Stopping monitoring containers..."
cd ~/fabric/arp-chaincode
docker-compose -f docker-compose-monitors.yaml down 2>/dev/null
docker rm -f monitor-org1 monitor-org2 monitor-org3 2>/dev/null
docker rm -f traffic-org1 traffic-org2 traffic-org3 2>/dev/null
docker rm -f arp-attacker 2>/dev/null

# Stop event listener and dashboard
echo "⏹️  Stopping event listener and dashboard..."
pkill -f "event-listener.go" 2>/dev/null
pkill -f "dashboard/app.py" 2>/dev/null
pkill -f "app.py" 2>/dev/null

# Remove log files
rm -f /tmp/event-listener.log 2>/dev/null
rm -f /tmp/dashboard.log 2>/dev/null

# Remove ARP test LAN network
echo "🌐 Removing ARP test LAN network..."
docker network disconnect arp-test-lan peer0.org1.example.com 2>/dev/null
docker network disconnect arp-test-lan peer0.org2.example.com 2>/dev/null
docker network disconnect arp-test-lan peer0.org3.example.com 2>/dev/null
docker network rm arp-test-lan 2>/dev/null

# Stop Fabric network
echo "⏹️  Stopping Fabric network..."
cd ~/fabric/fabric-samples/test-network
./network.sh down

# Stop and remove ALL Docker containers (including orphans)
echo "🧹 Removing all Fabric containers..."
# First stop all containers
docker stop $(docker ps -aq) 2>/dev/null

# Remove by name filter
docker rm -f $(docker ps -aq --filter "name=peer") 2>/dev/null
docker rm -f $(docker ps -aq --filter "name=orderer") 2>/dev/null
docker rm -f $(docker ps -aq --filter "name=arptracker") 2>/dev/null
docker rm -f $(docker ps -aq --filter "name=ca_") 2>/dev/null
docker rm -f $(docker ps -aq --filter "name=fabric") 2>/dev/null

# Remove orphan containers from the fabric_test network
docker ps -a --filter "network=fabric_test" -q | xargs docker rm -f 2>/dev/null

# Clean up crypto material and channel artifacts
echo "🧹 Removing crypto material and channel artifacts..."
sudo rm -rf organizations/ordererOrganizations 2>/dev/null
sudo rm -rf organizations/peerOrganizations 2>/dev/null
sudo rm -rf channel-artifacts 2>/dev/null
sudo rm -rf system-genesis-block 2>/dev/null
sudo rm -rf addOrg3/organizations 2>/dev/null
sudo rm -rf addOrg3/channel-artifacts 2>/dev/null
sudo rm -rf organizations/fabric-ca/org1/msp 2>/dev/null
sudo rm -rf organizations/fabric-ca/org2/msp 2>/dev/null
sudo rm -rf organizations/fabric-ca/ordererOrg/msp 2>/dev/null

# Clean up volumes and networks
echo "🧹 Cleaning up Docker volumes and networks..."
docker volume prune -f
docker network prune -f

# Return to project directory
cd ~/fabric/arp-chaincode

echo ""
echo "✅ Cleanup complete!"
echo ""
