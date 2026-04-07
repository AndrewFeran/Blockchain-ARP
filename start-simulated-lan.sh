#!/bin/bash

echo "============================================================"
echo "  🚀 Starting Blockchain-Secured Simulated LAN"
echo "============================================================"
echo ""

# Check if Fabric network is running
echo "📋 Checking prerequisites..."
echo ""

if ! docker ps | grep -q "peer0.org1.example.com"; then
    echo "❌ Fabric network not running!"
    echo ""
    echo "Please start the Fabric network first:"
    echo "  cd ~/fabric/fabric-samples/test-network"
    echo "  ./network.sh up createChannel -c mychannel"
    echo "  ./network.sh deployCC -ccn arptracker -ccp ~/fabric/arp-chaincode/chaincode -ccl go"
    echo ""
    exit 1
fi

echo "✅ Fabric network is running"
echo ""

# Check if chaincode is deployed
echo "📦 Checking if chaincode is deployed..."
if ! docker ps | grep -q "arptracker"; then
    echo "❌ ARP chaincode not deployed!"
    echo ""
    echo "Please deploy the chaincode:"
    echo "  cd ~/fabric/fabric-samples/test-network"
    echo "  ./network.sh deployCC -ccn arptracker -ccp ~/fabric/arp-chaincode/chaincode -ccl go"
    echo ""
    exit 1
fi

echo "✅ Chaincode is deployed"
echo ""

# Start the simulated LAN
echo "🏗️  Building Docker images..."
docker-compose build

echo ""
echo "🚀 Starting containers..."
docker-compose up -d

echo ""
echo "⏳ Waiting for services to start..."
sleep 5

echo ""
echo "============================================================"
echo "  ✅ Simulated LAN is running!"
echo "============================================================"
echo ""
echo "Services:"
echo "  🌐 Dashboard:  http://localhost:5000"
echo "  🛡️  Router:     blockchain-router (10.5.0.1)"
echo "  🖥️  Node 1:     lan-node-1 (10.5.0.10)"
echo "  🖥️  Node 2:     lan-node-2 (10.5.0.11)"
echo "  🖥️  Node 3:     lan-node-3 (10.5.0.12)"
echo ""
echo "Commands:"
echo "  View logs:     docker-compose logs -f [service]"
echo "  Stop all:      docker-compose down"
echo "  Run attack:    docker exec lan-attacker /app/spoof-attack.sh"
echo ""
echo "View live ARP activity at: http://localhost:5000"
echo ""
