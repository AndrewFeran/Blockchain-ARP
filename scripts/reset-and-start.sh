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
cd ~/fabric/arp-chaincode
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
./network.sh deployCCAAS -ccn arptracker -ccp ~/fabric/arp-chaincode
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
echo "📋 Running containers:"
docker ps --format "table {{.Names}}\t{{.Status}}" | grep -E "peer|orderer|arptracker"
echo ""
echo "🧪 Test Commands (copy and paste):"
echo ""
echo "# 1. Record a new device:"
echo "peer chaincode invoke -o localhost:7050 --ordererTLSHostnameOverride orderer.example.com --tls --cafile \"\${PWD}/organizations/ordererOrganizations/example.com/orderers/orderer.example.com/msp/tlscacerts/tlsca.example.com-cert.pem\" -C mychannel -n arptracker --peerAddresses localhost:7051 --tlsRootCertFiles \"\${PWD}/organizations/peerOrganizations/org1.example.com/peers/peer0.org1.example.com/tls/ca.crt\" --peerAddresses localhost:9051 --tlsRootCertFiles \"\${PWD}/organizations/peerOrganizations/org2.example.com/peers/peer0.org2.example.com/tls/ca.crt\" -c '{\"function\":\"RecordARPEntry\",\"Args\":[\"192.168.1.100\",\"AA:BB:CC:DD:EE:FF\",\"eth0\",\"laptop\",\"dynamic\",\"reachable\",\"gateway1\"]}'"
echo ""
echo "# 2. Trigger spoofing detection (same IP, different MAC):"
echo "peer chaincode invoke -o localhost:7050 --ordererTLSHostnameOverride orderer.example.com --tls --cafile \"\${PWD}/organizations/ordererOrganizations/example.com/orderers/orderer.example.com/msp/tlscacerts/tlsca.example.com-cert.pem\" -C mychannel -n arptracker --peerAddresses localhost:7051 --tlsRootCertFiles \"\${PWD}/organizations/peerOrganizations/org1.example.com/peers/peer0.org1.example.com/tls/ca.crt\" --peerAddresses localhost:9051 --tlsRootCertFiles \"\${PWD}/organizations/peerOrganizations/org2.example.com/peers/peer0.org2.example.com/tls/ca.crt\" -c '{\"function\":\"RecordARPEntry\",\"Args\":[\"192.168.1.100\",\"11:22:33:44:55:66\",\"eth0\",\"laptop\",\"dynamic\",\"reachable\",\"gateway1\"]}'"
echo ""
echo "# 3. Valid update (same IP, same MAC):"
echo "peer chaincode invoke -o localhost:7050 --ordererTLSHostnameOverride orderer.example.com --tls --cafile \"\${PWD}/organizations/ordererOrganizations/example.com/orderers/orderer.example.com/msp/tlscacerts/tlsca.example.com-cert.pem\" -C mychannel -n arptracker --peerAddresses localhost:7051 --tlsRootCertFiles \"\${PWD}/organizations/peerOrganizations/org1.example.com/peers/peer0.org1.example.com/tls/ca.crt\" --peerAddresses localhost:9051 --tlsRootCertFiles \"\${PWD}/organizations/peerOrganizations/org2.example.com/peers/peer0.org2.example.com/tls/ca.crt\" -c '{\"function\":\"RecordARPEntry\",\"Args\":[\"192.168.1.100\",\"11:22:33:44:55:66\",\"eth0\",\"laptop\",\"dynamic\",\"reachable\",\"gateway1\"]}'"
echo ""
echo "# 4. Query all entries:"
echo "peer chaincode query -C mychannel -n arptracker -c '{\"function\":\"GetAllARPEntries\",\"Args\":[]}'"
echo ""
echo "📊 Next Steps:"
echo "  1. Start Flask dashboard: cd ~/fabric/arp-chaincode/dashboard && python3 app.py"
echo "  2. Start event listener: cd ~/fabric/arp-chaincode && python3 event-listener-sdk.py"
echo "  3. Access dashboard: http://localhost:5000"
echo ""