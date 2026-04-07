#!/usr/bin/env bash
# Single-session script: start Docker, start Fabric, deploy chaincode, run benchmark
set -euo pipefail

FABRIC_DIR="/mnt/c/Users/Perky/OneDrive/Desktop/barp/fabric-samples"
CHAINCODE_DIR="/mnt/c/Users/Perky/OneDrive/Desktop/barp/Blockchain-ARP/chaincode"
BENCH_DIR="/mnt/c/Users/Perky/OneDrive/Desktop/barp/Blockchain-ARP/benchmark"
RESULTS_DIR="$BENCH_DIR/benchmark-results"

export PATH=$PATH:/usr/local/go/bin:$FABRIC_DIR/bin
export FABRIC_CFG_PATH=$FABRIC_DIR/config
export GOFLAGS=-buildvcs=false
export IMAGE_TAG=2.5.0
export CA_IMAGE_TAG=1.5.7

# ── Docker ────────────────────────────────────────────────────────────────────
echo "=== Starting Docker ==="
service docker start 2>/dev/null || true
sleep 4
docker ps > /dev/null
echo "✅ Docker ready"

# ── Fabric network ────────────────────────────────────────────────────────────
echo ""
echo "=== Starting Fabric test-network (IMAGE_TAG=2.5.0) ==="
cd "$FABRIC_DIR/test-network"
./network.sh down 2>/dev/null || true
IMAGE_TAG=2.5.0 CA_IMAGE_TAG=1.5.7 ./network.sh up createChannel -c mychannel -ca
echo "✅ Network up"
docker ps --format "table {{.Names}}\t{{.Image}}\t{{.Status}}"

# ── Chaincode go.sum fix ───────────────────────────────────────────────────────
echo ""
echo "=== Fix chaincode go.sum ==="
cd "$CHAINCODE_DIR"
go mod tidy
echo "✅ go.sum fixed"

# ── Package ───────────────────────────────────────────────────────────────────
echo ""
echo "=== Package chaincode ==="
mkdir -p /tmp/ccpkg && cd /tmp/ccpkg
rm -f arptracker.tar.gz
peer lifecycle chaincode package arptracker.tar.gz \
    --path "$CHAINCODE_DIR" --lang golang --label arptracker_1.0
echo "✅ Packaged"

# ── Install org1 ──────────────────────────────────────────────────────────────
echo ""
echo "=== Install on peer0.org1 ==="
export CORE_PEER_TLS_ENABLED=true
export CORE_PEER_LOCALMSPID=Org1MSP
export CORE_PEER_TLS_ROOTCERT_FILE=$FABRIC_DIR/test-network/organizations/peerOrganizations/org1.example.com/peers/peer0.org1.example.com/tls/ca.crt
export CORE_PEER_MSPCONFIGPATH=$FABRIC_DIR/test-network/organizations/peerOrganizations/org1.example.com/users/Admin@org1.example.com/msp
export CORE_PEER_ADDRESS=localhost:7051
export ORDERER_CA=$FABRIC_DIR/test-network/organizations/ordererOrganizations/example.com/orderers/orderer.example.com/msp/tlscacerts/tlsca.example.com-cert.pem
peer lifecycle chaincode install arptracker.tar.gz
echo "✅ Installed org1"

# ── Install org2 ──────────────────────────────────────────────────────────────
echo ""
echo "=== Install on peer0.org2 ==="
export CORE_PEER_LOCALMSPID=Org2MSP
export CORE_PEER_TLS_ROOTCERT_FILE=$FABRIC_DIR/test-network/organizations/peerOrganizations/org2.example.com/peers/peer0.org2.example.com/tls/ca.crt
export CORE_PEER_MSPCONFIGPATH=$FABRIC_DIR/test-network/organizations/peerOrganizations/org2.example.com/users/Admin@org2.example.com/msp
export CORE_PEER_ADDRESS=localhost:9051
peer lifecycle chaincode install arptracker.tar.gz
echo "✅ Installed org2"

# ── Get package ID ────────────────────────────────────────────────────────────
export CORE_PEER_LOCALMSPID=Org1MSP
export CORE_PEER_TLS_ROOTCERT_FILE=$FABRIC_DIR/test-network/organizations/peerOrganizations/org1.example.com/peers/peer0.org1.example.com/tls/ca.crt
export CORE_PEER_MSPCONFIGPATH=$FABRIC_DIR/test-network/organizations/peerOrganizations/org1.example.com/users/Admin@org1.example.com/msp
export CORE_PEER_ADDRESS=localhost:7051
CC_PACKAGE_ID=$(peer lifecycle chaincode queryinstalled --output json | \
    jq -r '.installed_chaincodes[] | select(.label=="arptracker_1.0") | .package_id')
echo "Package ID: $CC_PACKAGE_ID"

# ── Approve org1 ──────────────────────────────────────────────────────────────
echo ""
echo "=== Approve org1 ==="
peer lifecycle chaincode approveformyorg \
    -o localhost:7050 --ordererTLSHostnameOverride orderer.example.com \
    --channelID mychannel --name arptracker --version 1.0 \
    --package-id "$CC_PACKAGE_ID" --sequence 1 \
    --tls --cafile "$ORDERER_CA"
echo "✅ Approved org1"

# ── Approve org2 ──────────────────────────────────────────────────────────────
echo ""
echo "=== Approve org2 ==="
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

# ── Commit ────────────────────────────────────────────────────────────────────
echo ""
echo "=== Commit chaincode ==="
export CORE_PEER_LOCALMSPID=Org1MSP
export CORE_PEER_TLS_ROOTCERT_FILE=$FABRIC_DIR/test-network/organizations/peerOrganizations/org1.example.com/peers/peer0.org1.example.com/tls/ca.crt
export CORE_PEER_MSPCONFIGPATH=$FABRIC_DIR/test-network/organizations/peerOrganizations/org1.example.com/users/Admin@org1.example.com/msp
export CORE_PEER_ADDRESS=localhost:7051
ORG2_TLS=$FABRIC_DIR/test-network/organizations/peerOrganizations/org2.example.com/peers/peer0.org2.example.com/tls/ca.crt
peer lifecycle chaincode commit \
    -o localhost:7050 --ordererTLSHostnameOverride orderer.example.com \
    --channelID mychannel --name arptracker --version 1.0 --sequence 1 \
    --tls --cafile "$ORDERER_CA" \
    --peerAddresses localhost:7051 --tlsRootCertFiles "$CORE_PEER_TLS_ROOTCERT_FILE" \
    --peerAddresses localhost:9051 --tlsRootCertFiles "$ORG2_TLS"
echo "✅ Chaincode committed"

peer lifecycle chaincode querycommitted --channelID mychannel --name arptracker
echo ""
echo "=== CHAINCODE READY - Starting benchmark ==="

# ── Build benchmark ───────────────────────────────────────────────────────────
cd "$BENCH_DIR"
go build -o benchmark-bin .
echo "✅ Benchmark built"

# ── Run benchmark ─────────────────────────────────────────────────────────────
mkdir -p "$RESULTS_DIR"
export CRYPTO_PATH="$FABRIC_DIR/test-network/organizations/peerOrganizations/org1.example.com"
export PEER_ENDPOINT=localhost:7051
export GATEWAY_PEER=peer0.org1.example.com
export MSP_ID=Org1MSP
export CHANNEL_NAME=mychannel
export CHAINCODE_NAME=arptracker
export RESULTS_DIR
export BENCH_WRITE_TRIALS=50
export BENCH_READ_TRIALS=100
export BENCH_READ_ALL_TRIALS=20
export BENCH_THROUGHPUT_COUNT=100
export BENCH_CONCURRENCY=10
export BENCH_WARMUP=5

./benchmark-bin

echo ""
echo "=== DONE ==="
ls -lh "$RESULTS_DIR/"
