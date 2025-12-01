#!/bin/bash
# Setup 3-Organization Fabric Network for ARP Detection
# This script extends the standard 2-org test-network to include Org3

echo "============================================================"
echo "  ARP Tracker - 3-Organization Network Setup"
echo "============================================================"
echo ""

# Navigate to test-network directory
cd ~/fabric/fabric-samples/test-network || { echo "❌ test-network directory not found"; exit 1; }

# Step 1: Start base network with Org1 and Org2
echo "🚀 Step 1/5: Starting base Fabric network (Org1 + Org2)..."
./network.sh up createChannel -ca -c mychannel
if [ $? -ne 0 ]; then
    echo "❌ Failed to start base network!"
    exit 1
fi
echo "✅ Base network started"
echo ""

# Step 2: Add Org3 to the network
echo "🔗 Step 2/5: Adding Org3 to the network..."
cd addOrg3
./addOrg3.sh up -c mychannel
if [ $? -ne 0 ]; then
    echo "❌ Failed to add Org3!"
    exit 1
fi
cd ..
echo "✅ Org3 added successfully"
echo ""

# Step 3: Deploy chaincode to all 3 organizations
echo "📦 Step 3/5: Building chaincode Docker image..."
cd ~/fabric/arp-chaincode/chaincode || { echo "❌ chaincode directory not found"; exit 1; }
go mod vendor
docker build -t arptracker_ccaas_image:latest .
if [ $? -ne 0 ]; then
    echo "❌ Failed to build chaincode!"
    exit 1
fi
echo "✅ Chaincode built"
echo ""

echo "🔗 Step 4/5: Deploying chaincode to all 3 organizations..."
cd ~/fabric/fabric-samples/test-network
./network.sh deployCCAAS -ccn arptracker -ccp ~/fabric/arp-chaincode/chaincode
if [ $? -ne 0 ]; then
    echo "❌ Failed to deploy chaincode!"
    exit 1
fi
echo "✅ Chaincode deployed to Org1 and Org2"
echo ""

# Step 5: Install and approve chaincode for Org3
echo "🔗 Step 5/5: Installing chaincode on Org3..."

# Set environment for Org3
export PATH=${PWD}/../bin:$PATH
export FABRIC_CFG_PATH=${PWD}/../config
export CORE_PEER_TLS_ENABLED=true
export CORE_PEER_LOCALMSPID="Org3MSP"
export CORE_PEER_TLS_ROOTCERT_FILE=${PWD}/organizations/peerOrganizations/org3.example.com/tlsca/tlsca.org3.example.com-cert.pem
export CORE_PEER_MSPCONFIGPATH=${PWD}/organizations/peerOrganizations/org3.example.com/users/Admin@org3.example.com/msp
export CORE_PEER_ADDRESS=localhost:11051

# Package chaincode (if not already packaged)
PACKAGE_FILE="arptracker.tar.gz"
if [ ! -f "$PACKAGE_FILE" ]; then
    echo "Packaging chaincode..."
    peer lifecycle chaincode package $PACKAGE_FILE --path ~/fabric/arp-chaincode/chaincode --lang golang --label arptracker_1.0
fi

# Install chaincode on Org3
echo "Installing chaincode on Org3 peer..."
peer lifecycle chaincode install $PACKAGE_FILE

# Get package ID
PACKAGE_ID=$(peer lifecycle chaincode queryinstalled | grep arptracker_1.0 | awk '{print $3}' | sed 's/,$//')
echo "Package ID: $PACKAGE_ID"

# Approve chaincode for Org3
echo "Approving chaincode for Org3..."
peer lifecycle chaincode approveformyorg -o localhost:7050 --ordererTLSHostnameOverride orderer.example.com \
    --channelID mychannel --name arptracker --version 1.0 --package-id $PACKAGE_ID \
    --sequence 1 --tls --cafile ${PWD}/organizations/ordererOrganizations/example.com/orderers/orderer.example.com/msp/tlscacerts/tlsca.example.com-cert.pem

# Check commit readiness
echo "Checking commit readiness..."
peer lifecycle chaincode checkcommitreadiness --channelID mychannel --name arptracker --version 1.0 --sequence 1 --tls \
    --cafile ${PWD}/organizations/ordererOrganizations/example.com/orderers/orderer.example.com/msp/tlscacerts/tlsca.example.com-cert.pem \
    --output json

echo "✅ Chaincode installed and approved for Org3"
echo ""

echo "============================================================"
echo "🎉 SUCCESS! 3-Organization Network is Ready!"
echo "============================================================"
echo ""
echo "Organizations:"
echo "  - Org1 (Gateway): peer0.org1.example.com:7051"
echo "  - Org2 (Server):  peer0.org2.example.com:9051"
echo "  - Org3 (Laptop):  peer0.org3.example.com:11051"
echo ""
echo "Next steps:"
echo "  1. Run './create-arp-network.sh' to create the simulated LAN"
echo "  2. Start monitoring agents with './start-monitors.sh'"
echo ""
