#!/bin/bash
# Master Script - Complete Reset and Start for ARP Detection System
# This script does everything needed to get the system running from scratch

echo "============================================================"
echo "  ARP Tracker - Master Reset & Start Script"
echo "============================================================"
echo ""

# Step 1: Complete cleanup
echo "🧹 Step 1/5: Deep cleaning old network..."
cd ~/fabric/fabric-samples/test-network
./network.sh down
docker volume rm $(docker volume ls -q | grep peer) 2>/dev/null
docker volume rm $(docker volume ls -q | grep orderer) 2>/dev/null
docker volume prune -f
sudo rm -rf organizations/fabric-ca/org1/msp organizations/fabric-ca/org2/msp organizations/fabric-ca/ordererOrg/msp channel-artifacts/* system-genesis-block/* 2>/dev/null
echo "✅ Cleanup complete"
echo ""

# Step 2: Start Fabric network and create channel
echo "🚀 Step 2/5: Starting Fabric network and creating channel..."
./network.sh up createChannel -ca -c mychannel
if [ $? -ne 0 ]; then
    echo "❌ Failed to start network!"
    exit 1
fi
echo "✅ Network started and channel created"
echo ""

# Step 3: Build chaincode image
echo "📦 Step 3/5: Building chaincode Docker image..."
cd ~/fabric/arp-chaincode/chaincode
go mod vendor
docker build -t arptracker_ccaas_image:latest .
if [ $? -ne 0 ]; then
    echo "❌ Failed to build chaincode!"
    exit 1
fi
echo "✅ Chaincode built"
echo ""

# Step 4: Deploy chaincode as external service
echo "🔗 Step 4/5: Deploying chaincode to network..."
cd ~/fabric/fabric-samples/test-network
./network.sh deployCCAAS -ccn arptracker -ccp ~/fabric/arp-chaincode/chaincode
if [ $? -ne 0 ]; then
    echo "❌ Failed to deploy chaincode!"
    exit 1
fi
echo "✅ Chaincode deployed"
echo ""

# Step 5: Set up environment and show test commands
echo "✅ Step 5/5: Setting up environment..."
export PATH=${PWD}/../bin:$PATH
export FABRIC_CFG_PATH=${PWD}/../config
export CORE_PEER_TLS_ENABLED=true
export CORE_PEER_LOCALMSPID="Org1MSP"
export CORE_PEER_TLS_ROOTCERT_FILE=${PWD}/organizations/peerOrganizations/org1.example.com/tlsca/tlsca.org1.example.com-cert.pem
export CORE_PEER_MSPCONFIGPATH=${PWD}/organizations/peerOrganizations/org1.example.com/users/Admin@org1.example.com/msp
export CORE_PEER_ADDRESS=localhost:7051

echo ""
echo "============================================================"
echo "🎉 SUCCESS! ARP Detection System is Ready!"
echo "============================================================"
echo ""