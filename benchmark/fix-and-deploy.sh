#!/usr/bin/env bash
set -euo pipefail

FABRIC_DIR="/mnt/c/Users/Perky/OneDrive/Desktop/barp/fabric-samples"
CHAINCODE_DIR="/mnt/c/Users/Perky/OneDrive/Desktop/barp/Blockchain-ARP/chaincode"
export PATH=$PATH:/usr/local/go/bin:$FABRIC_DIR/bin
export IMAGE_TAG=2.5.0
export CA_IMAGE_TAG=1.5.7
export FABRIC_CFG_PATH=$FABRIC_DIR/config
export GOFLAGS=-buildvcs=false
export GONOSUMCHECK=*

# Org1 peer env
export CORE_PEER_TLS_ENABLED=true
export CORE_PEER_LOCALMSPID=Org1MSP
export CORE_PEER_TLS_ROOTCERT_FILE=$FABRIC_DIR/test-network/organizations/peerOrganizations/org1.example.com/peers/peer0.org1.example.com/tls/ca.crt
export CORE_PEER_MSPCONFIGPATH=$FABRIC_DIR/test-network/organizations/peerOrganizations/org1.example.com/users/Admin@org1.example.com/msp
export CORE_PEER_ADDRESS=localhost:7051
export ORDERER_CA=$FABRIC_DIR/test-network/organizations/ordererOrganizations/example.com/orderers/orderer.example.com/msp/tlscacerts/tlsca.example.com-cert.pem

echo "============================================================"
echo "  Fix chaincode go.sum"
echo "============================================================"
cd "$CHAINCODE_DIR"
go mod tidy
echo "✅ go.sum fixed"

echo ""
echo "============================================================"
echo "  Package chaincode"
echo "============================================================"
cd /tmp
rm -f arptracker.tar.gz
peer lifecycle chaincode package arptracker.tar.gz \
    --path "$CHAINCODE_DIR" --lang golang --label arptracker_1.0
echo "✅ Packaged"

echo ""
echo "============================================================"
echo "  Install on peer0.org1"
echo "============================================================"
peer lifecycle chaincode install arptracker.tar.gz
echo "✅ Installed on org1"

echo ""
echo "============================================================"
echo "  Install on peer0.org2"
echo "============================================================"
export CORE_PEER_LOCALMSPID=Org2MSP
export CORE_PEER_TLS_ROOTCERT_FILE=$FABRIC_DIR/test-network/organizations/peerOrganizations/org2.example.com/peers/peer0.org2.example.com/tls/ca.crt
export CORE_PEER_MSPCONFIGPATH=$FABRIC_DIR/test-network/organizations/peerOrganizations/org2.example.com/users/Admin@org2.example.com/msp
export CORE_PEER_ADDRESS=localhost:9051
peer lifecycle chaincode install arptracker.tar.gz
echo "✅ Installed on org2"

echo ""
echo "============================================================"
echo "  Get package ID"
echo "============================================================"
export CORE_PEER_LOCALMSPID=Org1MSP
export CORE_PEER_TLS_ROOTCERT_FILE=$FABRIC_DIR/test-network/organizations/peerOrganizations/org1.example.com/peers/peer0.org1.example.com/tls/ca.crt
export CORE_PEER_MSPCONFIGPATH=$FABRIC_DIR/test-network/organizations/peerOrganizations/org1.example.com/users/Admin@org1.example.com/msp
export CORE_PEER_ADDRESS=localhost:7051

CC_PACKAGE_ID=$(peer lifecycle chaincode queryinstalled --output json | \
    jq -r '.installed_chaincodes[] | select(.label=="arptracker_1.0") | .package_id')
echo "Package ID: $CC_PACKAGE_ID"

echo ""
echo "============================================================"
echo "  Approve for org1"
echo "============================================================"
peer lifecycle chaincode approveformyorg \
    -o localhost:7050 --ordererTLSHostnameOverride orderer.example.com \
    --channelID mychannel --name arptracker --version 1.0 \
    --package-id "$CC_PACKAGE_ID" --sequence 1 \
    --tls --cafile "$ORDERER_CA"
echo "✅ Approved org1"

echo ""
echo "============================================================"
echo "  Approve for org2"
echo "============================================================"
export CORE_PEER_LOCALMSPID=Org2MSP
export CORE_PEER_TLS_ROOTCERT_FILE=$FABRIC_DIR/test-network/organizations/peerOrganizations/org2.example.com/peers/peer0.org2.example.com/tls/ca.crt
export CORE_PEER_MSPCONFIGPATH=$FABRIC_DIR/test-network/organizations/peerOrganizations/org2.example.com/users/Admin@org2.example.com/msp
export CORE_PEER_ADDRESS=localhost:9051
peer lifecycle chaincode approveformyorg \
    -o localhost:7050 --ordererTLSHostnameOverride orderer.example.com \
    --channelID mychannel --name arptracker --version 1.0 \
    --package-id "$CC_PACKAGE_ID" --sequence 1 \
    --tls --cafile "$ORDERER_CA"
echo "✅ Approved org2"

echo ""
echo "============================================================"
echo "  Commit chaincode"
echo "============================================================"
export CORE_PEER_LOCALMSPID=Org1MSP
export CORE_PEER_TLS_ROOTCERT_FILE=$FABRIC_DIR/test-network/organizations/peerOrganizations/org1.example.com/peers/peer0.org1.example.com/tls/ca.crt
export CORE_PEER_MSPCONFIGPATH=$FABRIC_DIR/test-network/organizations/peerOrganizations/org1.example.com/users/Admin@org1.example.com/msp
export CORE_PEER_ADDRESS=localhost:7051
ORG2_CA=$FABRIC_DIR/test-network/organizations/peerOrganizations/org2.example.com/peers/peer0.org2.example.com/tls/ca.crt
peer lifecycle chaincode commit \
    -o localhost:7050 --ordererTLSHostnameOverride orderer.example.com \
    --channelID mychannel --name arptracker --version 1.0 --sequence 1 \
    --tls --cafile "$ORDERER_CA" \
    --peerAddresses localhost:7051 --tlsRootCertFiles "$CORE_PEER_TLS_ROOTCERT_FILE" \
    --peerAddresses localhost:9051 --tlsRootCertFiles "$ORG2_CA"
echo "✅ Chaincode committed"

echo ""
echo "============================================================"
echo "  Verify"
echo "============================================================"
peer lifecycle chaincode querycommitted --channelID mychannel --name arptracker

echo ""
echo "CHAINCODE_READY"
